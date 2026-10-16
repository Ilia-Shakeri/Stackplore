package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

const collectionOutcomeColumns = `collected_at_us,availability,completeness,observed_at_us,reason_code,
metric_availability,metric_completeness,metric_observed_at_us,metric_reason_code,
event_availability,event_completeness,event_observed_at_us,event_reason_code`

func (store *Store) GetSourceCollectionStatus(ctx context.Context, source domain.ProviderRef) (domain.SourceCollectionOutcome, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.SourceCollectionOutcome, error) {
		return reader.GetSourceCollectionStatus(ctx, source)
	})
}

func (store *observationRead) GetSourceCollectionStatus(ctx context.Context, source domain.ProviderRef) (domain.SourceCollectionOutcome, error) {
	if !store.available() || ctx == nil || source.Validate() != nil {
		return domain.SourceCollectionOutcome{}, domain.ErrRepositoryInvalidInput
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	var enabled, createdAt, version int64
	err := store.queries.QueryRow(ctx, `SELECT enabled,created_at_us,row_version FROM infrastructure_sources WHERE id=$1 AND provider=$2 AND deleted_at_us IS NULL`, string(source.SourceID), string(source.Provider)).Scan(&enabled, &createdAt, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SourceCollectionOutcome{}, domain.ErrRepositoryNotFound
	}
	if err != nil {
		return domain.SourceCollectionOutcome{}, mapRepositoryReadError(ctx, err)
	}
	result, err := scanCollectionOutcome(store.queries.QueryRow(ctx, `SELECT `+collectionOutcomeColumns+` FROM source_collection_outcomes WHERE source_id=$1 AND configuration_version=$2 ORDER BY generation DESC LIMIT 1`, string(source.SourceID), version))
	if errors.Is(err, pgx.ErrNoRows) {
		status := domain.ObservationStatus{State: domain.AvailabilityUnavailable, Completeness: domain.CompletenessComplete, ObservedAt: time.UnixMicro(createdAt).UTC(), ReasonCode: "source.collection_pending"}
		result = domain.SourceCollectionOutcome{CollectedAt: status.ObservedAt, Status: status, MetricStatus: status, EventStatus: status}
	} else if err != nil {
		return domain.SourceCollectionOutcome{}, mapRepositoryReadError(ctx, err)
	}
	if enabled == 0 {
		result.Status.State = domain.AvailabilityUnavailable
		result.Status.Completeness = domain.CompletenessComplete
		result.Status.ReasonCode = "source.disabled"
		result.MetricStatus = result.Status
		result.EventStatus = result.Status
	}
	return result, nil
}

func (store *Store) ListSourceCollectionHistory(ctx context.Context, source domain.ProviderRef, period domain.TimeRange, limit int) (domain.SourceCollectionHistory, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.SourceCollectionHistory, error) {
		return reader.ListSourceCollectionHistory(ctx, source, period, limit)
	})
}

func (store *observationRead) ListSourceCollectionHistory(ctx context.Context, source domain.ProviderRef, period domain.TimeRange, limit int) (domain.SourceCollectionHistory, error) {
	if !store.available() || ctx == nil || source.Validate() != nil || period.Validate() != nil || limit < 1 || limit > domain.MaximumCollectionHistory {
		return domain.SourceCollectionHistory{}, domain.ErrRepositoryInvalidInput
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	var exists bool
	if err := store.queries.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM infrastructure_sources WHERE id=$1 AND provider=$2 AND deleted_at_us IS NULL)`, string(source.SourceID), string(source.Provider)).Scan(&exists); err != nil {
		return domain.SourceCollectionHistory{}, mapRepositoryReadError(ctx, err)
	}
	if !exists {
		return domain.SourceCollectionHistory{}, domain.ErrRepositoryNotFound
	}
	rows, err := store.queries.Query(ctx, `SELECT `+collectionOutcomeColumns+` FROM source_collection_outcomes WHERE source_id=$1 AND configuration_version=(SELECT row_version FROM infrastructure_sources WHERE id=$1) AND collected_at_us>=$2 AND collected_at_us<$3 ORDER BY collected_at_us,generation LIMIT $4`, string(source.SourceID), period.Start.UnixMicro(), period.End.UnixMicro(), limit+1)
	if err != nil {
		return domain.SourceCollectionHistory{}, mapRepositoryReadError(ctx, err)
	}
	defer rows.Close()
	result := domain.SourceCollectionHistory{Observations: []domain.SourceCollectionOutcome{}, Complete: true}
	for rows.Next() {
		outcome, err := scanCollectionOutcome(rows)
		if err != nil {
			return domain.SourceCollectionHistory{}, domain.ErrRepositoryUnavailable
		}
		result.Observations = append(result.Observations, outcome)
	}
	if err := rows.Err(); err != nil {
		return domain.SourceCollectionHistory{}, mapRepositoryReadError(ctx, err)
	}
	if len(result.Observations) > limit {
		result.Observations = result.Observations[:limit]
		result.Complete = false
	}
	return result, nil
}

func scanCollectionOutcome(row rowScanner) (domain.SourceCollectionOutcome, error) {
	var result domain.SourceCollectionOutcome
	var at, observed, metricAt, eventAt int64
	var state, complete, metricState, metricComplete, eventState, eventComplete string
	var reason, metricReason, eventReason sql.NullString
	if err := row.Scan(&at, &state, &complete, &observed, &reason, &metricState, &metricComplete, &metricAt, &metricReason, &eventState, &eventComplete, &eventAt, &eventReason); err != nil {
		return result, err
	}
	result.CollectedAt = time.UnixMicro(at).UTC()
	result.Status = domain.ObservationStatus{State: domain.AvailabilityState(state), Completeness: domain.Completeness(complete), ObservedAt: time.UnixMicro(observed).UTC(), ReasonCode: domain.ReasonCode(reason.String)}
	result.MetricStatus = domain.ObservationStatus{State: domain.AvailabilityState(metricState), Completeness: domain.Completeness(metricComplete), ObservedAt: time.UnixMicro(metricAt).UTC(), ReasonCode: domain.ReasonCode(metricReason.String)}
	result.EventStatus = domain.ObservationStatus{State: domain.AvailabilityState(eventState), Completeness: domain.Completeness(eventComplete), ObservedAt: time.UnixMicro(eventAt).UTC(), ReasonCode: domain.ReasonCode(eventReason.String)}
	if result.Status.Validate() != nil || result.MetricStatus.Validate() != nil || result.EventStatus.Validate() != nil {
		return domain.SourceCollectionOutcome{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func saveCollectionOutcome(ctx context.Context, transaction *transaction, completion domain.CaptureCompletion, status domain.ObservationStatus) error {
	metric := domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: status.ObservedAt, ReasonCode: "source.metric_unsupported"}
	event := domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: status.ObservedAt, ReasonCode: "source.events_unsupported"}
	for _, capability := range completion.Capabilities.Capabilities {
		switch capability.Name {
		case domain.CapabilityMetricCurrent:
			metric = capability.Status
		case domain.CapabilityEventHistory:
			event = capability.Status
		}
	}
	_, err := transaction.transaction.Exec(ctx, `INSERT INTO source_collection_outcomes(source_id,generation,`+collectionOutcomeColumns+`,configuration_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, string(completion.Token.Source.SourceID), completion.Token.Generation, completion.Token.ObservedAt.UnixMicro(), string(status.State), string(status.Completeness), status.ObservedAt.UnixMicro(), optionalString(string(status.ReasonCode)), string(metric.State), string(metric.Completeness), metric.ObservedAt.UnixMicro(), optionalString(string(metric.ReasonCode)), string(event.State), string(event.Completeness), event.ObservedAt.UnixMicro(), optionalString(string(event.ReasonCode)), completion.Token.ConfigurationVersion)
	return err
}

var _ domain.SourceCollectionReader = (*Store)(nil)
