package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// EnsureObservationPolicy closes cached reads when operator authority changes.
// A failed attempt cannot clear this fence; fresh normalized coverage must publish.
func (store *Store) EnsureObservationPolicy(ctx context.Context, source domain.ProviderRef, digest string) error {
	decoded, err := hex.DecodeString(digest)
	if ctx == nil || source.Validate() != nil || err != nil || len(decoded) != 32 || len(digest) != 64 || hex.EncodeToString(decoded) != digest {
		return domain.ErrRepositoryInvalidInput
	}
	return mapRepositoryWriteError(ctx, store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		result, err := transaction.transaction.Exec(ctx, `UPDATE infrastructure_sources SET operator_policy_digest=$1,operator_policy_pending=true,row_version=row_version+1,updated_at_us=GREATEST(updated_at_us,$4)
WHERE id=$2 AND provider=$3 AND deleted_at_us IS NULL AND operator_policy_digest IS DISTINCT FROM $1`, digest, string(source.SourceID), string(source.Provider), time.Now().UTC().UnixMicro())
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			var exists bool
			if err := transaction.transaction.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM infrastructure_sources WHERE id=$1 AND provider=$2 AND deleted_at_us IS NULL)`, string(source.SourceID), string(source.Provider)).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return domain.ErrRepositoryNotFound
			}
			return nil
		}
		return appendSourceProfileAudit(ctx, transaction, source.SourceID, "source.profile_changed")
	}))
}

// ReconcileOperatorProfile is a startup-only change between the two reviewed
// Docker profiles. It preserves administrator enablement and closes old cache.
func (store *Store) ReconcileOperatorProfile(ctx context.Context, source domain.ProviderRef, profile domain.SourceProfileID) error {
	if ctx == nil || source.Validate() != nil || source.Provider != domain.ProviderDocker ||
		(profile != domain.SourceProfileDockerSocketDevelopment && profile != domain.SourceProfileDockerRestrictedProxy) {
		return domain.ErrRepositoryInvalidInput
	}
	return mapRepositoryWriteError(ctx, store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		result, err := transaction.transaction.Exec(ctx, `UPDATE infrastructure_sources SET operator_profile_id=$1,row_version=row_version+1,operator_policy_pending=true,updated_at_us=GREATEST(updated_at_us,$3)
WHERE id=$2 AND provider='docker' AND synthetic=0 AND deleted_at_us IS NULL
AND operator_profile_id IN ('docker_socket_dev','docker_restricted_proxy') AND operator_profile_id<>$1`, string(profile), string(source.SourceID), time.Now().UTC().UnixMicro())
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return nil
		}
		return appendSourceProfileAudit(ctx, transaction, source.SourceID, "source.profile_selected")
	}))
}

func appendSourceProfileAudit(ctx context.Context, transaction *transaction, source domain.SourceID, reason domain.ReasonCode) error {
	identity := make([]byte, 16)
	if _, err := rand.Read(identity); err != nil {
		return domain.ErrRepositoryUnavailable
	}
	suffix := hex.EncodeToString(identity)
	return (&transactionRepositories{transaction: transaction}).AppendAuditEntries(ctx, []domain.AuditEntry{{ID: domain.AuditEntryID("audit_policy_" + suffix), OccurredAt: time.Now().UTC().Truncate(time.Microsecond), Actor: domain.AuditActor{Kind: domain.AuditActorSystem}, Action: "source.profile_update", Target: domain.AuditTarget{Kind: "source.configuration", ID: string(source)}, Outcome: domain.AuditSuccess, RequestID: domain.RequestID("request_policy_" + suffix), ReasonCode: reason}})
}
