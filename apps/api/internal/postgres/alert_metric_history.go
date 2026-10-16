package postgres

import (
	"context"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (store *Store) GetAlertMetricHistory(ctx context.Context, query domain.AlertMetricHistoryQuery) (domain.MetricHistoryResult, error) {
	if ctx == nil || query.Validate() != nil {
		return domain.MetricHistoryResult{}, domain.ErrRepositoryInvalidInput
	}
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.MetricHistoryResult, error) {
		status, err := reader.GetObservationStatus(ctx, query.Source)
		if err != nil {
			return domain.MetricHistoryResult{}, err
		}
		result := domain.MetricHistoryResult{Status: status, Samples: []domain.MetricSample{}}
		if !status.AllowsData() {
			return result, nil
		}
		rows, err := reader.queries.Query(ctx, `SELECT sample.id,sample.resource_id,sample.metric_name,sample.unit,sample.value,sample.sampled_at_us,sample.interval_seconds,sample.synthetic FROM metric_samples AS sample JOIN resources AS resource ON resource.id=sample.resource_id WHERE resource.source_id=$1 AND resource.provider=$2 AND sample.resource_id=$3 AND sample.metric_name=$4 AND sample.sampled_at_us>=$5 AND sample.sampled_at_us<$6 ORDER BY sample.sampled_at_us,sample.id COLLATE "C" LIMIT $7`, string(query.Source.SourceID), string(query.Source.Provider), string(query.Resource.ID), string(query.Name), query.Range.Start.UnixMicro(), query.Range.End.UnixMicro(), query.Limit+1)
		if err != nil {
			return domain.MetricHistoryResult{}, mapRepositoryReadError(ctx, err)
		}
		defer rows.Close()
		for rows.Next() {
			sample, err := scanObservedMetric(rows)
			if err != nil {
				return domain.MetricHistoryResult{}, domain.ErrRepositoryUnavailable
			}
			result.Samples = append(result.Samples, sample)
		}
		if err := rows.Err(); err != nil {
			return domain.MetricHistoryResult{}, mapRepositoryReadError(ctx, err)
		}
		rows.Close()
		status, err = reader.observationCapabilityStatus(ctx, query.Source, domain.CapabilityMetricHistory, status, len(result.Samples) > 0)
		if err != nil {
			return domain.MetricHistoryResult{}, err
		}
		result.Status = status
		if !status.AllowsData() {
			result.Samples = []domain.MetricSample{}
			return result, nil
		}
		if len(result.Samples) > query.Limit {
			result.Samples = result.Samples[:query.Limit]
			result.Status.Completeness = domain.CompletenessPartial
			result.Status.ReasonCode = "source.metric_history_limited"
		}
		return result, nil
	})
}

var _ domain.AlertMetricHistoryProvider = (*Store)(nil)
