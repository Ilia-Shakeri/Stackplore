package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (store *Store) EnsureDemoAlerts(ctx context.Context, seed domain.DemoAlertSeed) error {
	if ctx == nil || seed.Validate() != nil {
		return domain.ErrRepositoryInvalidInput
	}
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if err := lockAlertChanges(ctx, transaction); err != nil {
			return err
		}
		var version string
		err := transaction.transaction.QueryRow(ctx, `SELECT fixture_version FROM demo_alert_seeds WHERE source_id=$1`, string(seed.Rule.SourceID)).Scan(&version)
		if err == nil {
			if version != seed.FixtureVersion {
				return domain.ErrRepositoryConflict
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var ready bool
		if err := transaction.transaction.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM infrastructure_sources WHERE id=$1 AND provider='demo' AND synthetic=1 AND enabled=1 AND deleted_at_us IS NULL AND completed_generation>0)`, string(seed.Rule.SourceID)).Scan(&ready); err != nil {
			return err
		}
		if !ready {
			return domain.ErrRepositoryNotFound
		}
		var userID domain.UserID
		if err := transaction.transaction.QueryRow(ctx, `SELECT id FROM users WHERE role='administrator' AND enabled=1 AND deleted_at_us IS NULL ORDER BY created_at_us,id LIMIT 1`).Scan(&userID); errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRepositoryNotFound
		} else if err != nil {
			return err
		}
		rule := seed.Rule
		// An existing fixture rule is preserved, including intentional soft deletion.
		var exists bool
		if err := transaction.transaction.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM alert_rules WHERE id=$1)`, string(rule.ID)).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			_, err := transaction.transaction.Exec(ctx, `INSERT INTO alert_rules(id,source_id,environment_id,resource_id,display_name,rule_kind,enabled,severity,comparison_operator,threshold_value,threshold_count,window_seconds,minimum_event_severity,evaluation_interval_seconds,pending_for_seconds,next_evaluation_at_us,created_by_user_id,updated_by_user_id,created_at_us,updated_at_us,row_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$17,$18,$19,1)`, string(rule.ID), string(rule.SourceID), optionalEnvironmentID(rule.EnvironmentID), optionalResourceID(rule.ResourceID), rule.DisplayName, string(rule.Kind), boolInteger(rule.Enabled), string(rule.Severity), optionalComparison(rule.Comparison), optionalFloat(rule.ThresholdValue), optionalCount(rule.ThresholdCount), optionalDuration(rule.Window), optionalSeverity(rule.MinimumSeverity), int64(rule.EvaluationInterval/time.Second), int64(rule.PendingFor/time.Second), seed.Mutation.Alert.UpdatedAt.UnixMicro(), string(userID), seed.Mutation.Alert.StartedAt.UnixMicro(), seed.Mutation.Alert.UpdatedAt.UnixMicro())
			if err != nil {
				return err
			}
			if err := appendAlertChange(ctx, transaction, rule.SourceID, rule.ID, nil, domain.AlertChangeRuleCreated, seed.Mutation.Alert.UpdatedAt); err != nil {
				return err
			}
			if err := (&transactionRepositories{transaction: transaction}).SaveAlertMutation(ctx, seed.Mutation); err != nil {
				return err
			}
		}
		digest := sha256.Sum256([]byte(string(rule.SourceID) + "\x00" + seed.FixtureVersion))
		suffix := hex.EncodeToString(digest[:16])
		audit := domain.AuditEntry{ID: domain.AuditEntryID("audit_demo_seed_" + suffix), OccurredAt: seed.Mutation.Alert.UpdatedAt, Actor: domain.AuditActor{Kind: domain.AuditActorSystem}, Action: "alert.demo_seed", Target: domain.AuditTarget{Kind: "source.configuration", ID: string(rule.SourceID)}, Outcome: domain.AuditSuccess, RequestID: domain.RequestID("request_demo_seed_" + suffix), ReasonCode: "alert.demo_seeded"}
		if err := (&transactionRepositories{transaction: transaction}).AppendAuditEntries(ctx, []domain.AuditEntry{audit}); err != nil {
			return err
		}
		_, err = transaction.transaction.Exec(ctx, `INSERT INTO demo_alert_seeds(source_id,fixture_version,seeded_at_us) VALUES($1,$2,$3)`, string(rule.SourceID), seed.FixtureVersion, seed.Mutation.Alert.UpdatedAt.UnixMicro())
		return err
	})
	return mapRepositoryWriteError(ctx, err)
}

var _ domain.DemoAlertSeedRepository = (*Store)(nil)
