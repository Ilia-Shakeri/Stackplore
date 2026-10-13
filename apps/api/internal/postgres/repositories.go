package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

const insertMetricSampleStatement = `INSERT INTO metric_samples (
    id, resource_id, metric_name, unit, value, sampled_at_us, interval_seconds, synthetic
)
SELECT $1, $2, $3, $4, $5, $6, $7, $8
FROM resources AS resource
JOIN infrastructure_sources AS source ON source.id = resource.source_id
WHERE resource.id = $9
  AND resource.synthetic = $10
  AND source.synthetic = $11`

const insertInfrastructureEventStatement = `INSERT INTO infrastructure_events (
    id, origin_kind, provider, source_id, environment_id, resource_id, kind,
    severity, reason_code, summary, occurred_at_us, observed_at_us, synthetic
) VALUES ($1, 'provider', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

const insertAlertStatement = `INSERT INTO alerts (
    id, rule_id, source_id, environment_id, resource_id, dedupe_key, state, severity,
    evidence_start_at_us, evidence_end_at_us, started_at_us, updated_at_us,
    acknowledged_at_us, acknowledged_by_user_id, resolved_at_us, summary_code, synthetic
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`

const updateAlertStatement = `UPDATE alerts SET
    state = $1, severity = $2, evidence_start_at_us = $3, evidence_end_at_us = $4,
    updated_at_us = $5, acknowledged_at_us = $6, acknowledged_by_user_id = $7,
    resolved_at_us = $8, summary_code = $9, row_version = row_version + 1
WHERE id = $10 AND rule_id = $11 AND source_id = $12 AND dedupe_key = $13
  AND updated_at_us = $14 AND state = $15`

const insertAlertTransitionStatement = `INSERT INTO alert_transitions (
    id, alert_id, from_state, to_state, occurred_at_us, actor_kind, actor_user_id,
    reason_code, request_id, evidence_start_at_us, evidence_end_at_us
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

const insertNotificationStatement = `INSERT INTO notifications (
    id, source_id, alert_id, resource_id, notification_kind, severity,
    summary_code, created_at_us, synthetic
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

const notificationOutboxLockKey int64 = 0x535441434B4F5554

var _ domain.ProductRepositories = (*Store)(nil)
var _ domain.ProductRepositoryUnitOfWork = (*Store)(nil)
var _ domain.ProductRepositories = (*transactionRepositories)(nil)

type transactionRepositories struct {
	transaction *transaction
	writes      int
	failure     error
}

func (store *Store) InsertMetricSamples(ctx context.Context, samples []domain.MetricSample) error {
	return store.WithinProductUnitOfWork(ctx, func(repositories domain.ProductRepositories) error {
		return repositories.InsertMetricSamples(ctx, samples)
	})
}

func (store *Store) InsertInfrastructureEvents(ctx context.Context, events []domain.InfrastructureEvent) error {
	return store.WithinProductUnitOfWork(ctx, func(repositories domain.ProductRepositories) error {
		return repositories.InsertInfrastructureEvents(ctx, events)
	})
}

func (store *Store) AppendAuditEntries(ctx context.Context, entries []domain.AuditEntry) error {
	return store.WithinProductUnitOfWork(ctx, func(repositories domain.ProductRepositories) error {
		return repositories.AppendAuditEntries(ctx, entries)
	})
}

func (store *Store) SaveAlertMutation(ctx context.Context, mutation domain.AlertMutation) error {
	return store.WithinProductUnitOfWork(ctx, func(repositories domain.ProductRepositories) error {
		return repositories.SaveAlertMutation(ctx, mutation)
	})
}

func (store *Store) WithinProductUnitOfWork(ctx context.Context, work func(domain.ProductRepositories) error) error {
	if err := validateRepositoryContext(ctx); err != nil {
		return err
	}
	if work == nil {
		return domain.ErrRepositoryInvalidInput
	}
	var workErr error
	transactionErr := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		repositories := &transactionRepositories{transaction: transaction}
		workErr = work(repositories)
		if repositories.failure != nil {
			workErr = repositories.failure
		}
		return workErr
	})
	if workErr != nil {
		return workErr
	}
	return mapRepositoryWriteError(ctx, transactionErr)
}

func (repositories *transactionRepositories) InsertMetricSamples(ctx context.Context, samples []domain.MetricSample) error {
	ctx, cancel := repositories.queryContext(ctx)
	defer cancel()
	if err := repositories.beginWrite(ctx, len(samples), domain.ValidateMetricSampleWriteBatch(samples)); err != nil {
		return err
	}
	for _, sample := range samples {
		synthetic := boolInteger(sample.Synthetic)
		result, err := repositories.transaction.transaction.Exec(ctx, insertMetricSampleStatement,
			string(sample.ID), string(sample.ResourceID), string(sample.Name), string(sample.Unit),
			sample.Value, sample.SampledAt.UnixMicro(), int64(sample.Interval/time.Second), synthetic,
			string(sample.ResourceID), synthetic, synthetic,
		)
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return repositories.latchFailure(err)
		}
	}
	return nil
}

func (repositories *transactionRepositories) InsertInfrastructureEvents(ctx context.Context, events []domain.InfrastructureEvent) error {
	ctx, cancel := repositories.queryContext(ctx)
	defer cancel()
	if err := repositories.beginWrite(ctx, len(events), domain.ValidateInfrastructureEventWriteBatch(events)); err != nil {
		return err
	}
	for _, event := range events {
		result, err := repositories.transaction.transaction.Exec(ctx, insertInfrastructureEventStatement,
			string(event.ID), string(event.Provider), string(event.SourceID), optionalEnvironmentID(event.EnvironmentID),
			optionalResourceID(event.ResourceID), string(event.Kind), string(event.Severity), string(event.ReasonCode),
			event.Summary, event.OccurredAt.UnixMicro(), event.ObservedAt.UnixMicro(), boolInteger(event.Synthetic),
		)
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return repositories.latchFailure(err)
		}
	}
	return nil
}

func (repositories *transactionRepositories) AppendAuditEntries(ctx context.Context, entries []domain.AuditEntry) error {
	ctx, cancel := repositories.queryContext(ctx)
	defer cancel()
	if err := repositories.beginWrite(ctx, len(entries), domain.ValidateAuditEntryWriteBatch(entries)); err != nil {
		return err
	}
	for _, entry := range entries {
		actorUserID, operatorID := auditActorColumns(entry.Actor)
		result, err := repositories.transaction.transaction.Exec(ctx, insertAuditEntryStatement,
			string(entry.ID), entry.OccurredAt.UnixMicro(), string(entry.Actor.Kind), actorUserID, operatorID,
			string(entry.Action), string(entry.Target.Kind), optionalString(entry.Target.ID), string(entry.Outcome),
			string(entry.RequestID), string(entry.ReasonCode),
		)
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return repositories.latchFailure(err)
		}
	}
	return nil
}

func (repositories *transactionRepositories) SaveAlertMutation(ctx context.Context, mutation domain.AlertMutation) error {
	ctx, cancel := repositories.queryContext(ctx)
	defer cancel()
	if err := repositories.beginWrite(ctx, 1, mutation.Validate()); err != nil {
		return err
	}
	alert := mutation.Alert
	acknowledgedAt := optionalTime(alert.AcknowledgedAt)
	acknowledgedBy := optionalUserID(mutation.AcknowledgedByUserID)
	resolvedAt := optionalTime(alert.ResolvedAt)
	var result pgconn.CommandTag
	var err error
	if mutation.ExpectedUpdatedAt == nil {
		result, err = repositories.transaction.transaction.Exec(ctx, insertAlertStatement,
			string(alert.ID), string(alert.RuleID), string(alert.SourceID), optionalEnvironmentID(alert.EnvironmentID),
			optionalResourceID(alert.ResourceID), mutation.DedupeKey, string(alert.State), string(alert.Severity),
			alert.EvidenceRange.Start.UnixMicro(), alert.EvidenceRange.End.UnixMicro(), alert.StartedAt.UnixMicro(),
			alert.UpdatedAt.UnixMicro(), acknowledgedAt, acknowledgedBy, resolvedAt, string(alert.SummaryCode),
			boolInteger(alert.Synthetic),
		)
	} else {
		result, err = repositories.transaction.transaction.Exec(ctx, updateAlertStatement,
			string(alert.State), string(alert.Severity), alert.EvidenceRange.Start.UnixMicro(),
			alert.EvidenceRange.End.UnixMicro(), alert.UpdatedAt.UnixMicro(), acknowledgedAt, acknowledgedBy,
			resolvedAt, string(alert.SummaryCode), string(alert.ID), string(alert.RuleID), string(alert.SourceID),
			mutation.DedupeKey, mutation.ExpectedUpdatedAt.UnixMicro(), string(*mutation.ExpectedState),
		)
	}
	if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
		return repositories.latchFailure(err)
	}
	if mutation.Transition != nil {
		transition := mutation.Transition
		actorUserID := any(nil)
		if transition.Actor.Kind == domain.AuditActorUser {
			actorUserID = transition.Actor.ID
		}
		result, err := repositories.transaction.transaction.Exec(ctx, insertAlertTransitionStatement,
			string(transition.ID), string(transition.AlertID), optionalAlertState(transition.From), string(transition.To),
			transition.OccurredAt.UnixMicro(), string(transition.Actor.Kind), actorUserID, string(transition.ReasonCode),
			optionalRequestID(transition.RequestID), transition.EvidenceRange.Start.UnixMicro(),
			transition.EvidenceRange.End.UnixMicro(),
		)
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return repositories.latchFailure(err)
		}
	}
	if mutation.Notification != nil {
		if _, err := repositories.transaction.transaction.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, notificationOutboxLockKey); err != nil {
			return repositories.latchFailure(mapRepositoryWriteError(ctx, err))
		}
		notification := mutation.Notification
		result, err := repositories.transaction.transaction.Exec(ctx, insertNotificationStatement,
			string(notification.ID), optionalSourceID(notification.SourceID), optionalAlertID(notification.AlertID),
			optionalResourceID(notification.ResourceID), string(notification.Kind), string(notification.Severity),
			string(notification.SummaryCode), notification.CreatedAt.UnixMicro(), boolInteger(notification.Synthetic),
		)
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return repositories.latchFailure(err)
		}
	}
	return nil
}

func (repositories *transactionRepositories) queryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil || repositories == nil || repositories.transaction == nil || repositories.transaction.context == nil {
		return ctx, func() {}
	}
	transactionContext := repositories.transaction.context
	deadline, _ := transactionContext.Deadline()
	queryContext, cancel := boundedContext(ctx, time.Until(deadline))
	stop := context.AfterFunc(transactionContext, cancel)
	return queryContext, func() { stop(); cancel() }
}

func (repositories *transactionRepositories) beginWrite(ctx context.Context, count int, validation error) error {
	if err := repositories.previousFailure(); err != nil {
		return err
	}
	if err := validateRepositoryContext(ctx); err != nil {
		return repositories.latchFailure(err)
	}
	if validation != nil {
		return repositories.latchFailure(validation)
	}
	if repositories.transaction == nil || repositories.transaction.transaction == nil ||
		count < 1 || repositories.writes > domain.MaxRepositoryWriteBatch-count {
		return repositories.latchFailure(domain.ErrRepositoryInvalidInput)
	}
	repositories.writes += count
	return nil
}

func (repositories *transactionRepositories) previousFailure() error {
	if repositories == nil {
		return domain.ErrRepositoryUnavailable
	}
	return repositories.failure
}

func (repositories *transactionRepositories) latchFailure(err error) error {
	if err != nil && repositories != nil && repositories.failure == nil {
		repositories.failure = err
	}
	return err
}

func exactInsertResult(ctx context.Context, affected int64, err error) error {
	if err != nil {
		return mapRepositoryWriteError(ctx, err)
	}
	if affected != 1 {
		return domain.ErrRepositoryConflict
	}
	return nil
}

func validateRepositoryContext(ctx context.Context) error {
	if ctx == nil {
		return domain.ErrRepositoryInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(domain.ErrRepositoryUnavailable, err)
	}
	return nil
}

func mapRepositoryWriteError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return errors.Join(domain.ErrRepositoryUnavailable, ctx.Err())
	}
	for _, owned := range []error{domain.ErrRepositoryConflict, domain.ErrRepositoryInvalidInput, domain.ErrRepositoryNotFound, domain.ErrRepositoryUnavailable} {
		if errors.Is(err, owned) {
			return owned
		}
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && len(postgresError.Code) == 5 && postgresError.Code[:2] == "23" {
		return domain.ErrRepositoryConflict
	}
	return domain.ErrRepositoryUnavailable
}

func optionalEnvironmentID(value *domain.EnvironmentID) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func optionalResourceID(value *domain.ResourceID) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func optionalSourceID(value *domain.SourceID) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func optionalAlertID(value *domain.AlertID) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func optionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UnixMicro()
}

func optionalUserID(value *domain.UserID) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func optionalAlertState(value *domain.AlertState) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func optionalRequestID(value *domain.RequestID) any {
	if value == nil {
		return nil
	}
	return string(*value)
}
