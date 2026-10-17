package postgres

import (
	"context"
	"database/sql"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (store *Store) GetHealthHistory(ctx context.Context, query domain.HealthHistoryQuery) (domain.HealthHistoryResult, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.HealthHistoryResult, error) {
		return reader.GetHealthHistory(ctx, query)
	})
}
func (store *observationRead) GetHealthHistory(ctx context.Context, query domain.HealthHistoryQuery) (domain.HealthHistoryResult, error) {
	if ctx == nil || query.Validate() != nil {
		return domain.HealthHistoryResult{}, domain.ErrRepositoryInvalidInput
	}
	status, err := store.GetObservationStatus(ctx, query.Source)
	if err != nil {
		return domain.HealthHistoryResult{}, err
	}
	result := domain.HealthHistoryResult{Status: status, Samples: []domain.HealthHistorySample{}}
	if !status.AllowsData() {
		return result, nil
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	rows, err := store.queries.Query(ctx, `SELECT history.id,history.resource_id,history.health_state,history.availability,history.completeness,history.observed_at_us,history.reason_code,history.synthetic FROM health_history AS history JOIN resources AS resource ON resource.id=history.resource_id JOIN infrastructure_sources AS source ON source.id=resource.source_id WHERE resource.source_id=$1 AND resource.provider=$2 AND resource.id=$3 AND history.observed_at_us>=$4 AND history.observed_at_us<$5 AND history.observed_at_us<=$7 AND `+authorizedResourcePredicate+` ORDER BY history.observed_at_us DESC,history.id COLLATE "C" LIMIT $6`, string(query.Source.SourceID), string(query.Source.Provider), string(query.Resource.ID), query.Range.Start.UnixMicro(), query.Range.End.UnixMicro(), query.Limit+1, status.ObservedAt.UnixMicro())
	if err != nil {
		return domain.HealthHistoryResult{}, mapRepositoryReadError(ctx, err)
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.HealthHistorySample
		var state, availability, completeness string
		var at, synthetic int64
		var reason sql.NullString
		if rows.Scan(&item.ID, &item.ResourceID, &state, &availability, &completeness, &at, &reason, &synthetic) != nil {
			return domain.HealthHistoryResult{}, domain.ErrRepositoryUnavailable
		}
		item.State = domain.HealthState(state)
		item.Status = scanObservationStatus(availability, completeness, at, reason)
		item.Synthetic = synthetic == 1
		if item.Status.Completeness == domain.CompletenessPartial {
			item.State = domain.HealthUnknown
		}
		result.Samples = append(result.Samples, item)
	}
	if err := rows.Err(); err != nil {
		return domain.HealthHistoryResult{}, mapRepositoryReadError(ctx, err)
	}
	rows.Close()
	status, err = store.observationCapabilityStatus(ctx, query.Source, domain.CapabilityHealth, status, len(result.Samples) > 0)
	if err != nil {
		return domain.HealthHistoryResult{}, err
	}
	result.Status = status
	if !status.AllowsData() {
		result.Samples = []domain.HealthHistorySample{}
		return result, nil
	}
	if len(result.Samples) > query.Limit {
		result.Samples = result.Samples[:query.Limit]
		result.Status.Completeness = domain.CompletenessPartial
		result.Status.ReasonCode = "source.health_history_limited"
	}
	if result.ValidateFor(query) != nil {
		return domain.HealthHistoryResult{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}
