package postgres

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func metricNames(names []domain.MetricName) []string {
	values := make([]string, 0, len(names))
	for _, name := range names {
		values = append(values, string(name))
	}
	return values
}

func scanObservedMetric(row rowScanner) (domain.MetricSample, error) {
	var sample domain.MetricSample
	var name, unit string
	var sampledAt, interval, synthetic int64
	if err := row.Scan(&sample.ID, &sample.ResourceID, &name, &unit, &sample.Value, &sampledAt, &interval, &synthetic); err != nil {
		return domain.MetricSample{}, err
	}
	sample.Name, sample.Unit = domain.MetricName(name), domain.MetricUnit(unit)
	sample.SampledAt = time.UnixMicro(sampledAt).UTC()
	sample.Interval = time.Duration(interval) * time.Second
	sample.Synthetic = synthetic == 1
	if sample.Validate() != nil {
		return domain.MetricSample{}, domain.ErrRepositoryUnavailable
	}
	return sample, nil
}

func (store *observationRead) GetCurrentMetrics(ctx context.Context, query domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	if ctx == nil || query.Validate() != nil {
		return domain.CurrentMetricResult{}, domain.ErrRepositoryInvalidInput
	}
	status, err := store.GetObservationStatus(ctx, query.Source)
	if err != nil {
		return domain.CurrentMetricResult{}, err
	}
	result := domain.CurrentMetricResult{Status: status, Sets: []domain.CurrentMetricSet{}}
	if !status.AllowsData() {
		return result, nil
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	rows, err := store.queries.Query(ctx, `SELECT sample.id,sample.resource_id,sample.metric_name,sample.unit,
sample.value,sample.sampled_at_us,sample.interval_seconds,sample.synthetic
FROM resources AS resource JOIN infrastructure_sources AS source ON source.id=resource.source_id
CROSS JOIN unnest($3::text[]) AS name(metric_name)
JOIN LATERAL (SELECT id,resource_id,metric_name,unit,value,sampled_at_us,interval_seconds,synthetic
FROM metric_samples WHERE resource_id=resource.id AND metric_name=name.metric_name
ORDER BY sampled_at_us DESC,id COLLATE "C" LIMIT 1) AS sample ON TRUE
WHERE resource.source_id=$1 AND resource.provider=$2 AND resource.id=ANY($4::text[])
AND resource.retired_at_us IS NULL AND resource.capture_generation>=source.completed_generation
ORDER BY resource.id COLLATE "C",sample.metric_name COLLATE "C" LIMIT $5`, string(query.Source.SourceID), string(query.Source.Provider), metricNames(query.Names), resourceIDs(query.Resources), len(query.Resources)*len(query.Names))
	if err != nil {
		return domain.CurrentMetricResult{}, mapRepositoryReadError(ctx, err)
	}
	defer rows.Close()
	byResource := make(map[domain.ResourceID][]domain.MetricSample, len(query.Resources))
	for rows.Next() {
		sample, err := scanObservedMetric(rows)
		if err != nil {
			return domain.CurrentMetricResult{}, domain.ErrRepositoryUnavailable
		}
		byResource[sample.ResourceID] = append(byResource[sample.ResourceID], sample)
	}
	if err := rows.Err(); err != nil {
		return domain.CurrentMetricResult{}, mapRepositoryReadError(ctx, err)
	}
	rows.Close()
	status, err = store.observationCapabilityStatus(ctx, query.Source, domain.CapabilityMetricCurrent, status, len(byResource) > 0)
	if err != nil {
		return domain.CurrentMetricResult{}, err
	}
	result.Status = status
	if !status.AllowsData() {
		return result, nil
	}
	for _, resource := range query.Resources {
		samples := byResource[resource.ID]
		if samples == nil {
			samples = []domain.MetricSample{}
		}
		setStatus := status
		if len(samples) == 0 {
			setStatus.State = domain.AvailabilityUnavailable
			setStatus.Completeness = domain.CompletenessComplete
			setStatus.ReasonCode = "source.metric_unavailable"
			result.Status.Completeness = domain.CompletenessPartial
			result.Status.ReasonCode = "source.metric_partial"
		} else if len(samples) < len(query.Names) {
			setStatus.Completeness = domain.CompletenessPartial
			setStatus.ReasonCode = "source.metric_partial"
			result.Status.Completeness = domain.CompletenessPartial
			result.Status.ReasonCode = "source.metric_partial"
		}
		result.Sets = append(result.Sets, domain.CurrentMetricSet{ResourceID: resource.ID, Status: setStatus, Samples: samples})
	}
	if result.ValidateFor(query) != nil {
		return domain.CurrentMetricResult{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func (store *observationRead) GetMetricHistory(ctx context.Context, query domain.MetricHistoryQuery) (domain.MetricHistoryResult, error) {
	if ctx == nil || query.Validate() != nil {
		return domain.MetricHistoryResult{}, domain.ErrRepositoryInvalidInput
	}
	status, err := store.GetObservationStatus(ctx, query.Source)
	if err != nil {
		return domain.MetricHistoryResult{}, err
	}
	result := domain.MetricHistoryResult{Status: status, Samples: []domain.MetricSample{}}
	if !status.AllowsData() {
		return result, nil
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	rows, err := store.queries.Query(ctx, `SELECT sample.id,sample.resource_id,sample.metric_name,sample.unit,
sample.value,sample.sampled_at_us,sample.interval_seconds,sample.synthetic
FROM metric_samples AS sample JOIN resources AS resource ON resource.id=sample.resource_id
WHERE resource.source_id=$1 AND resource.provider=$2 AND sample.resource_id=ANY($3::text[])
AND sample.metric_name=ANY($4::text[]) AND sample.sampled_at_us>=$5 AND sample.sampled_at_us<$6
ORDER BY sample.sampled_at_us, sample.resource_id COLLATE "C",sample.metric_name COLLATE "C",sample.id COLLATE "C" LIMIT $7`, string(query.Source.SourceID), string(query.Source.Provider), resourceIDs(query.Resources), metricNames(query.Names), query.Range.Start.UnixMicro(), query.Range.End.UnixMicro(), query.PointLimit+1)
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
	status, err = store.observationCapabilityStatus(ctx, query.Source, domain.CapabilityMetricHistory, status, len(result.Samples) > 0)
	if err != nil {
		return domain.MetricHistoryResult{}, err
	}
	result.Status = status
	if !status.AllowsData() {
		result.Samples = []domain.MetricSample{}
		return result, nil
	}
	if len(result.Samples) > query.PointLimit {
		result.Samples = result.Samples[:query.PointLimit]
		result.Status.Completeness = domain.CompletenessPartial
		result.Status.ReasonCode = "source.metric_history_limited"
	}
	if result.ValidateFor(query) != nil {
		return domain.MetricHistoryResult{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func (store *observationRead) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	if ctx == nil || query.Validate() != nil {
		return domain.EventPage{}, domain.ErrRepositoryInvalidInput
	}
	status, err := store.GetObservationStatus(ctx, query.Source)
	if err != nil {
		return domain.EventPage{}, err
	}
	result := domain.EventPage{Status: status, Events: []domain.InfrastructureEvent{}, Page: domain.PageInfo{Limit: query.Page.Limit}}
	if !status.AllowsData() {
		return result, nil
	}
	queryBinding := query
	queryBinding.Page = domain.PageRequest{}
	queryHash := observationQueryHash(queryBinding)
	cursor, err := store.decodeObservationCursor(query.Page.Cursor, "events", queryHash)
	if err != nil {
		return domain.EventPage{}, err
	}
	afterID := ""
	afterTime := query.Range.End.UnixMicro()
	if cursor != nil {
		afterID = cursor.ID
		afterTime, err = strconv.ParseInt(cursor.Value, 10, 64)
		if err != nil || afterTime < 0 {
			return domain.EventPage{}, domain.ErrRepositoryInvalidInput
		}
	}
	severities := make([]string, 0, len(query.Severities))
	for _, severity := range query.Severities {
		severities = append(severities, string(severity))
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	rows, err := store.queries.Query(ctx, `SELECT id,provider,source_id,environment_id,resource_id,kind,severity,reason_code,summary,occurred_at_us,observed_at_us,synthetic
FROM infrastructure_events WHERE source_id=$1 AND provider=$2 AND occurred_at_us>=$3 AND occurred_at_us<$4
AND (cardinality($5::text[])=0 OR resource_id=ANY($5::text[]))
AND (cardinality($6::text[])=0 OR severity=ANY($6::text[]))
AND occurred_at_us<=$7 AND (occurred_at_us<$7 OR (occurred_at_us=$7 AND id COLLATE "C">$8 COLLATE "C"))
ORDER BY occurred_at_us DESC,id COLLATE "C" LIMIT $9`, string(query.Source.SourceID), string(query.Source.Provider), query.Range.Start.UnixMicro(), query.Range.End.UnixMicro(), resourceIDs(query.Resources), severities, afterTime, afterID, query.Page.Limit+1)
	if err != nil {
		return domain.EventPage{}, mapRepositoryReadError(ctx, err)
	}
	defer rows.Close()
	for rows.Next() {
		var event domain.InfrastructureEvent
		var provider, kind, severity, reason string
		var environmentID, resourceID sql.NullString
		var occurredAt, observedAt, synthetic int64
		if rows.Scan(&event.ID, &provider, &event.SourceID, &environmentID, &resourceID, &kind, &severity, &reason, &event.Summary, &occurredAt, &observedAt, &synthetic) != nil {
			return domain.EventPage{}, domain.ErrRepositoryUnavailable
		}
		event.Provider, event.Kind, event.Severity, event.ReasonCode = domain.ProviderKind(provider), domain.EventKind(kind), domain.Severity(severity), domain.ReasonCode(reason)
		if environmentID.Valid {
			value := domain.EnvironmentID(environmentID.String)
			event.EnvironmentID = &value
		}
		if resourceID.Valid {
			value := domain.ResourceID(resourceID.String)
			event.ResourceID = &value
		}
		event.OccurredAt, event.ObservedAt = time.UnixMicro(occurredAt).UTC(), time.UnixMicro(observedAt).UTC()
		event.Synthetic = synthetic == 1
		result.Events = append(result.Events, event)
	}
	if err := rows.Err(); err != nil {
		return domain.EventPage{}, mapRepositoryReadError(ctx, err)
	}
	rows.Close()
	status, err = store.observationCapabilityStatus(ctx, query.Source, domain.CapabilityEventHistory, status, len(result.Events) > 0)
	if err != nil {
		return domain.EventPage{}, err
	}
	result.Status = status
	if !status.AllowsData() {
		result.Events = []domain.InfrastructureEvent{}
		return result, nil
	}
	if len(result.Events) > query.Page.Limit {
		result.Events = result.Events[:query.Page.Limit]
		last := result.Events[len(result.Events)-1]
		result.Page.HasMore = true
		result.Page.NextCursor, err = store.encodeObservationCursor("events", queryHash, strconv.FormatInt(last.OccurredAt.UnixMicro(), 10), string(last.ID))
		if err != nil {
			return domain.EventPage{}, err
		}
	}
	if result.ValidateFor(query) != nil {
		return domain.EventPage{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

var _ domain.ObservationReader = (*Store)(nil)
