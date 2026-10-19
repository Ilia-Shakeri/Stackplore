package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/jackc/pgx/v5"
)

var _ assistant.RuntimeRepository = (*Store)(nil)

const assistantRunColumns = `id,conversation_id,owner_id,source_id,COALESCE(resource_id,''),goal,state,
source_version,restore_generation,policy_version,profile_id,profile_version,volatile,paid,row_version,fence,
COALESCE(lease_owner,''),COALESCE(lease_until_us,0),tool_calls,inference_calls,reserved_units,used_units,
created_at_us,updated_at_us,deadline_us,session_digest,digest_key_id,auth_revision,fingerprint,COALESCE(memory_owner,''),from_us,until_us,configuration_version,claim_attempts,usage_unknown,usage_conservative`

type assistantRecord struct {
	assistant.Run
	Actor       assistant.Actor
	fingerprint []byte
	memoryOwner string
}
type assistantControl struct {
	assistant.Settings
	paidGeneration int64
}

func assistantMaterial(a assistant.Admission) [32]byte {
	// The supplied keyed text fingerprint is also bound to every material typed field.
	a.ID = ""
	a.MemoryOwner = ""
	a.At = time.Time{}
	a.Deadline = time.Time{}
	a.IdempotencyDigest = [32]byte{}
	a.From = a.From.UTC()
	a.Until = a.Until.UTC()
	encoded, _ := json.Marshal(a)
	return sha256.Sum256(encoded)
}

func assistantScan(row pgx.Row) (assistantRecord, error) {
	var record assistantRecord
	var lease, created, updated, deadline, from, until int64
	var digest []byte
	err := row.Scan(&record.ID, &record.ConversationID, &record.OwnerID, &record.SourceID, &record.ResourceID, &record.Goal, &record.State,
		&record.SourceVersion, &record.RestoreGeneration, &record.PolicyVersion, &record.ProfileID, &record.ProfileVersion, &record.Volatile, &record.Paid, &record.Version, &record.Fence,
		&record.LeaseOwner, &lease, &record.ToolCalls, &record.InferenceCalls, &record.ReservedUnits, &record.UsedUnits, &created, &updated, &deadline,
		&digest, &record.Actor.DigestKeyID, &record.Actor.AuthRevision, &record.fingerprint, &record.memoryOwner, &from, &until, &record.ConfigurationVersion, &record.Attempts, &record.UsageUncertain, &record.UsageConservative)
	if err != nil {
		return record, err
	}
	if len(digest) != 32 {
		return record, assistant.ErrUnavailable
	}
	copy(record.Actor.SessionDigest[:], digest)
	record.Actor.UserID = record.OwnerID
	if record.LeaseOwner != "" {
		record.LeaseUntil = time.UnixMicro(lease).UTC()
	}
	record.CreatedAt = time.UnixMicro(created).UTC()
	record.UpdatedAt = time.UnixMicro(updated).UTC()
	record.Deadline = time.UnixMicro(deadline).UTC()
	record.From = time.UnixMicro(from).UTC()
	record.Until = time.UnixMicro(until).UTC()
	return record, nil
}

func (store *Store) assistantTransaction(ctx context.Context, work func(context.Context, pgx.Tx) error) error {
	if ctx == nil || ctx.Err() != nil {
		return assistant.ErrInvalid
	}
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error { return work(ctx, transaction.transaction) })
	for _, owned := range []error{assistant.ErrInvalid, assistant.ErrDenied, assistant.ErrConflict, assistant.ErrQuota, assistant.ErrUnavailable} {
		if errors.Is(err, owned) {
			return owned
		}
	}
	if err != nil {
		return assistant.ErrUnavailable
	}
	return nil
}

func assistantLock(ctx context.Context, tx pgx.Tx) (assistantControl, error) {
	var c assistantControl
	err := tx.QueryRow(ctx, `SELECT row_version,enabled,paid_enabled,paid_generation,policy_version,profile_id,profile_version,user_quota,installation_quota,profile_paid FROM assistant_control WHERE singleton_id=1 FOR UPDATE`).Scan(
		&c.Version, &c.Enabled, &c.PaidEnabled, &c.paidGeneration, &c.PolicyVersion, &c.ProfileID, &c.ProfileVersion, &c.UserQuota, &c.InstallationQuota, &c.ProfilePaid)
	return c, err
}

func assistantNow(ctx context.Context, tx pgx.Tx) (time.Time, error) {
	var microseconds int64
	err := tx.QueryRow(ctx, `SELECT (extract(epoch FROM clock_timestamp())*1000000)::BIGINT`).Scan(&microseconds)
	return time.UnixMicro(microseconds).UTC(), err
}

func assistantActor(ctx context.Context, tx pgx.Tx, actor assistant.Actor, at time.Time) error {
	var found int
	err := tx.QueryRow(ctx, `SELECT 1 FROM users AS u JOIN sessions AS s ON s.user_id=u.id
WHERE u.id=$1 AND u.enabled=1 AND u.deleted_at_us IS NULL AND u.auth_revision=$2
 AND s.token_digest=$3 AND s.token_digest_key_id=$4 AND s.user_auth_revision=u.auth_revision
 AND s.revoked_at_us IS NULL
 AND s.created_at_us<=(extract(epoch FROM clock_timestamp())*1000000)::BIGINT
 AND s.idle_expires_at_us>(extract(epoch FROM clock_timestamp())*1000000)::BIGINT
 AND s.absolute_expires_at_us>(extract(epoch FROM clock_timestamp())*1000000)::BIGINT
FOR SHARE OF u,s`, actor.UserID, actor.AuthRevision, actor.SessionDigest[:], actor.DigestKeyID).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return assistant.ErrDenied
	}
	return err
}

func assistantAuthority(ctx context.Context, tx pgx.Tx, c assistantControl, r assistantRecord, at time.Time) error {
	if !c.Enabled || c.Version != r.ConfigurationVersion || c.ProfilePaid != r.Paid || c.PolicyVersion != r.PolicyVersion || c.ProfileID != r.ProfileID || c.ProfileVersion != r.ProfileVersion {
		return assistant.ErrDenied
	}
	if err := assistantActor(ctx, tx, r.Actor, at); err != nil {
		return err
	}
	var generation, version int64
	if err := tx.QueryRow(ctx, `SELECT restore_generation FROM installation_metadata WHERE singleton_id=1 FOR SHARE`).Scan(&generation); err != nil {
		return err
	}
	if generation != r.RestoreGeneration || (r.Paid && (!c.PaidEnabled || c.paidGeneration != generation)) {
		return assistant.ErrDenied
	}
	err := tx.QueryRow(ctx, `SELECT row_version FROM infrastructure_sources WHERE id=$1 AND enabled=1 AND deleted_at_us IS NULL FOR SHARE`, r.SourceID).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && version != r.SourceVersion {
		return assistant.ErrDenied
	}
	if err != nil {
		return err
	}
	return assistantActor(ctx, tx, r.Actor, at)
}

func assistantEvent(ctx context.Context, tx pgx.Tx, run assistant.Run, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO assistant_status_events(run_id,sequence,run_version,state,occurred_at_us) VALUES($1,$2,$2,$3,$4)`, run.ID, run.Version, run.State, at.UnixMicro())
	return err
}

func assistantAudit(ctx context.Context, tx pgx.Tx, a assistant.OperatorAudit, action, target string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_entries(id,occurred_at_us,actor_kind,operator_id,action,target_kind,target_id,outcome,request_id,reason_code)
VALUES($1,$2,'operator',$3,$4,'assistant.runtime',$5,'success',$6,$4)`, a.ID, a.At.UnixMicro(), a.OperatorID, action, target, a.RequestID)
	return err
}

func (store *Store) ConfigureAssistant(ctx context.Context, s assistant.Settings, a assistant.OperatorAudit) (assistant.Settings, error) {
	if !s.Valid() || !a.Valid() {
		return assistant.Settings{}, assistant.ErrInvalid
	}
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		c, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		if c.Version != s.Version {
			return assistant.ErrConflict
		}
		changed := c.ProfileID != s.ProfileID || c.ProfileVersion != s.ProfileVersion || c.PolicyVersion != s.PolicyVersion || c.ProfilePaid != s.ProfilePaid
		if _, err = tx.Exec(ctx, `UPDATE assistant_control SET row_version=row_version+1,enabled=$1,paid_enabled=$2,policy_version=$3,profile_id=$4,profile_version=$5,user_quota=$6,installation_quota=$7,paid_generation=CASE WHEN $8 THEN -1 ELSE paid_generation END,paid_receipt_digest=CASE WHEN $8 THEN NULL ELSE paid_receipt_digest END,profile_paid=$9 WHERE singleton_id=1`, s.Enabled, s.PaidEnabled, s.PolicyVersion, s.ProfileID, s.ProfileVersion, s.UserQuota, s.InstallationQuota, changed, s.ProfilePaid); err != nil {
			return err
		}
		return assistantAudit(ctx, tx, a, "assistant.configure", "installation")
	})
	if err == nil {
		s.Version++
	}
	return s, err
}

func (store *Store) AdmitAssistant(ctx context.Context, a assistant.Admission) (assistant.Run, error) {
	if !a.Valid() {
		return assistant.Run{}, assistant.ErrInvalid
	}
	var result assistant.Run
	fingerprint := assistantMaterial(a)
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		c, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		duration := a.Deadline.Sub(a.At)
		a.At, err = assistantNow(ctx, tx)
		if err != nil {
			return err
		}
		a.Deadline = a.At.Add(duration)
		if err = assistantActor(ctx, tx, a.Actor, a.At); err != nil {
			return err
		}
		existing, err := assistantScan(tx.QueryRow(ctx, `SELECT `+assistantRunColumns+` FROM assistant_runs WHERE owner_id=$1 AND idempotency_digest=$2 FOR UPDATE`, a.Actor.UserID, a.IdempotencyDigest[:]))
		if err == nil {
			if !bytes.Equal(existing.fingerprint, fingerprint[:]) {
				return assistant.ErrConflict
			}
			if err = assistantAuthority(ctx, tx, c, existing, a.At); err != nil {
				return err
			}
			result = existing.Run
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var tombstone bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM assistant_usage_ledger WHERE owner_id=$1 AND idempotency_digest=$2)`, a.Actor.UserID, a.IdempotencyDigest[:]).Scan(&tombstone); err != nil {
			return err
		}
		if tombstone {
			return assistant.ErrConflict
		}
		record := assistantRecord{Run: assistant.Run{ID: a.ID, ConversationID: a.ConversationID, OwnerID: a.Actor.UserID, SourceID: a.SourceID, ResourceID: a.ResourceID, Goal: a.Goal, State: assistant.Queued, SourceVersion: a.SourceVersion, RestoreGeneration: a.RestoreGeneration, PolicyVersion: a.PolicyVersion, ProfileID: a.ProfileID, ProfileVersion: a.ProfileVersion, Volatile: a.Volatile, Paid: a.Paid, Version: 1, ReservedUnits: a.ReservedUnits, CreatedAt: a.At, UpdatedAt: a.At, Deadline: a.Deadline, From: a.From, Until: a.Until, ConfigurationVersion: c.Version}, Actor: a.Actor}
		if err = assistantAuthority(ctx, tx, c, record, a.At); err != nil {
			return err
		}
		var queued, userQueued int
		if err = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE owner_id=$1) FROM assistant_runs WHERE state='queued'`, a.Actor.UserID).Scan(&queued, &userQueued); err != nil {
			return err
		}
		if queued >= 16 || userQueued >= 2 {
			return assistant.ErrQuota
		}
		if _, err = tx.Exec(ctx, `INSERT INTO assistant_usage_accounts(account_key) VALUES($1) ON CONFLICT DO NOTHING`, "user:"+a.Actor.UserID); err != nil {
			return err
		}
		for _, account := range []struct {
			key   string
			limit int64
		}{{"installation", c.InstallationQuota}, {"user:" + a.Actor.UserID, c.UserQuota}} {
			command, err := tx.Exec(ctx, `UPDATE assistant_usage_accounts SET reserved_units=reserved_units+$1,used_units=CASE WHEN period_version=(SELECT quota_period FROM assistant_control WHERE singleton_id=1) THEN used_units ELSE 0 END,period_version=(SELECT quota_period FROM assistant_control WHERE singleton_id=1) WHERE account_key=$2 AND (CASE WHEN period_version=(SELECT quota_period FROM assistant_control WHERE singleton_id=1) THEN used_units ELSE 0 END)+reserved_units+$1<=$3`, a.ReservedUnits, account.key, account.limit)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return assistant.ErrQuota
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO assistant_runs(id,conversation_id,owner_id,session_digest,digest_key_id,auth_revision,idempotency_digest,fingerprint,goal,source_id,source_version,resource_id,from_us,until_us,restore_generation,policy_version,profile_id,profile_version,volatile,paid,state,reserved_units,created_at_us,updated_at_us,deadline_us,memory_owner,configuration_version)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULLIF($12,''),$13,$14,$15,$16,$17,$18,$19,$20,'queued',$21,$22,$22,$23,NULLIF($24,''),$25)`, a.ID, a.ConversationID, a.Actor.UserID, a.Actor.SessionDigest[:], a.Actor.DigestKeyID, a.Actor.AuthRevision, a.IdempotencyDigest[:], fingerprint[:], a.Goal, a.SourceID, a.SourceVersion, a.ResourceID, a.From.UnixMicro(), a.Until.UnixMicro(), a.RestoreGeneration, a.PolicyVersion, a.ProfileID, a.ProfileVersion, a.Volatile, a.Paid, a.ReservedUnits, a.At.UnixMicro(), a.Deadline.UnixMicro(), a.MemoryOwner, c.Version)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO assistant_usage_ledger(reservation_id,run_reference,owner_id,reserved_units,state,restore_generation,created_at_us,updated_at_us,idempotency_digest,fingerprint) VALUES($1,$1,$2,$3,'held',$4,$5,$5,$6,$7)`, a.ID, a.Actor.UserID, a.ReservedUnits, a.RestoreGeneration, a.At.UnixMicro(), a.IdempotencyDigest[:], fingerprint[:])
		if err != nil {
			return err
		}
		result = record.Run
		return assistantEvent(ctx, tx, result, a.At)
	})
	return result, err
}
