package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const insertMetricSampleStatement = `INSERT INTO metric_samples (
    id,
    resource_id,
    metric_name,
    unit,
    value,
    sampled_at_us,
    interval_seconds,
    synthetic
)
SELECT ?, ?, ?, ?, ?, ?, ?, ?
FROM resources AS resource
JOIN infrastructure_sources AS source ON source.id = resource.source_id
WHERE resource.id = ?
  AND resource.synthetic = ?
  AND source.synthetic = ?`

const insertInfrastructureEventStatement = `INSERT INTO infrastructure_events (
    id,
    origin_kind,
    provider,
    source_id,
    environment_id,
    resource_id,
    kind,
    severity,
    reason_code,
    summary,
    occurred_at_us,
    observed_at_us,
    synthetic
) VALUES (?, 'provider', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertAuditEntryStatement = `INSERT INTO audit_entries (
    id,
    occurred_at_us,
    actor_kind,
    actor_user_id,
    operator_id,
    action,
    target_kind,
    target_id,
    outcome,
    request_id,
    reason_code,
    source_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`

var _ domain.Repositories = (*Store)(nil)
var _ domain.RepositoryUnitOfWork = (*Store)(nil)
var _ domain.Repositories = (*transactionRepositories)(nil)

type transactionRepositories struct {
	transaction *transaction
	writes      int
	failure     error
}

// InsertMetricSamples stores one bounded metric batch atomically.
func (store *Store) InsertMetricSamples(ctx context.Context, samples []domain.MetricSample) error {
	return store.WithinUnitOfWork(ctx, func(repositories domain.Repositories) error {
		return repositories.InsertMetricSamples(ctx, samples)
	})
}

// InsertInfrastructureEvents stores one bounded event batch atomically.
func (store *Store) InsertInfrastructureEvents(ctx context.Context, events []domain.InfrastructureEvent) error {
	return store.WithinUnitOfWork(ctx, func(repositories domain.Repositories) error {
		return repositories.InsertInfrastructureEvents(ctx, events)
	})
}

// AppendAuditEntries stores one bounded audit batch atomically.
func (store *Store) AppendAuditEntries(ctx context.Context, entries []domain.AuditEntry) error {
	return store.WithinUnitOfWork(ctx, func(repositories domain.Repositories) error {
		return repositories.AppendAuditEntries(ctx, entries)
	})
}

// WithinUnitOfWork commits all repository writes together or rolls them all back.
func (store *Store) WithinUnitOfWork(ctx context.Context, work func(domain.Repositories) error) error {
	if err := validateRepositoryContext(ctx); err != nil {
		return err
	}
	if work == nil {
		return domain.ErrRepositoryInvalidInput
	}

	var workErr error
	transactionErr := store.withinTransaction(ctx, func(transaction *transaction) error {
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
	if err := repositories.previousFailure(); err != nil {
		return err
	}
	if err := validateRepositoryContext(ctx); err != nil {
		return repositories.latchFailure(err)
	}
	if err := domain.ValidateMetricSampleWriteBatch(samples); err != nil {
		return repositories.latchFailure(err)
	}
	if err := repositories.reserveWrites(len(samples)); err != nil {
		return repositories.latchFailure(err)
	}
	statement, err := repositories.prepare(ctx, insertMetricSampleStatement)
	if err != nil {
		return repositories.latchFailure(err)
	}
	for _, sample := range samples {
		synthetic := boolInteger(sample.Synthetic)
		err = executeRepositoryInsert(ctx, statement,
			string(sample.ID),
			string(sample.ResourceID),
			string(sample.Name),
			string(sample.Unit),
			sample.Value,
			sample.SampledAt.UnixMicro(),
			int64(sample.Interval/time.Second),
			synthetic,
			string(sample.ResourceID),
			synthetic,
			synthetic,
		)
		if err != nil {
			_ = statement.Close()
			return repositories.latchFailure(err)
		}
	}
	return repositories.latchFailure(mapRepositoryWriteError(ctx, statement.Close()))
}

func (repositories *transactionRepositories) InsertInfrastructureEvents(ctx context.Context, events []domain.InfrastructureEvent) error {
	if err := repositories.previousFailure(); err != nil {
		return err
	}
	if err := validateRepositoryContext(ctx); err != nil {
		return repositories.latchFailure(err)
	}
	if err := domain.ValidateInfrastructureEventWriteBatch(events); err != nil {
		return repositories.latchFailure(err)
	}
	if err := repositories.reserveWrites(len(events)); err != nil {
		return repositories.latchFailure(err)
	}
	statement, err := repositories.prepare(ctx, insertInfrastructureEventStatement)
	if err != nil {
		return repositories.latchFailure(err)
	}
	for _, event := range events {
		err = executeRepositoryInsert(ctx, statement,
			string(event.ID),
			string(event.Provider),
			string(event.SourceID),
			optionalEnvironmentID(event.EnvironmentID),
			optionalResourceID(event.ResourceID),
			string(event.Kind),
			string(event.Severity),
			string(event.ReasonCode),
			event.Summary,
			event.OccurredAt.UnixMicro(),
			event.ObservedAt.UnixMicro(),
			boolInteger(event.Synthetic),
		)
		if err != nil {
			_ = statement.Close()
			return repositories.latchFailure(err)
		}
	}
	return repositories.latchFailure(mapRepositoryWriteError(ctx, statement.Close()))
}

func (repositories *transactionRepositories) AppendAuditEntries(ctx context.Context, entries []domain.AuditEntry) error {
	if err := repositories.previousFailure(); err != nil {
		return err
	}
	if err := validateRepositoryContext(ctx); err != nil {
		return repositories.latchFailure(err)
	}
	if err := domain.ValidateAuditEntryWriteBatch(entries); err != nil {
		return repositories.latchFailure(err)
	}
	if err := repositories.reserveWrites(len(entries)); err != nil {
		return repositories.latchFailure(err)
	}
	statement, err := repositories.prepare(ctx, insertAuditEntryStatement)
	if err != nil {
		return repositories.latchFailure(err)
	}
	for _, entry := range entries {
		actorUserID, operatorID := auditActorColumns(entry.Actor)
		err = executeRepositoryInsert(ctx, statement,
			string(entry.ID),
			entry.OccurredAt.UnixMicro(),
			string(entry.Actor.Kind),
			actorUserID,
			operatorID,
			string(entry.Action),
			string(entry.Target.Kind),
			optionalString(entry.Target.ID),
			string(entry.Outcome),
			string(entry.RequestID),
			string(entry.ReasonCode),
		)
		if err != nil {
			_ = statement.Close()
			return repositories.latchFailure(err)
		}
	}
	return repositories.latchFailure(mapRepositoryWriteError(ctx, statement.Close()))
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

func (repositories *transactionRepositories) reserveWrites(count int) error {
	if repositories == nil || count < 1 || repositories.writes > domain.MaxRepositoryWriteBatch-count {
		return domain.ErrRepositoryInvalidInput
	}
	repositories.writes += count
	return nil
}

func (repositories *transactionRepositories) prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	if repositories.transaction == nil || repositories.transaction.transaction == nil {
		return nil, domain.ErrRepositoryUnavailable
	}
	statement, err := repositories.transaction.transaction.PrepareContext(ctx, query)
	if err != nil {
		return nil, mapRepositoryWriteError(ctx, err)
	}
	return statement, nil
}

func executeRepositoryInsert(ctx context.Context, statement *sql.Stmt, arguments ...any) error {
	result, err := statement.ExecContext(ctx, arguments...)
	if err != nil {
		return mapRepositoryWriteError(ctx, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.ErrRepositoryUnavailable
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
	var sqliteError *sqlite.Error
	if errors.As(err, &sqliteError) && sqliteError.Code()&0xff == sqlite3.SQLITE_CONSTRAINT {
		return domain.ErrRepositoryConflict
	}
	return domain.ErrRepositoryUnavailable
}

func boolInteger(value bool) int {
	if value {
		return 1
	}
	return 0
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

func optionalString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func auditActorColumns(actor domain.AuditActor) (any, any) {
	switch actor.Kind {
	case domain.AuditActorUser:
		return actor.ID, nil
	case domain.AuditActorOperator:
		return nil, optionalString(actor.ID)
	default:
		return nil, nil
	}
}
