package alerting

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (resolver *ProviderEvidence) metricCondition(ctx context.Context, provider EvidenceProvider, task EvaluationTask, collection domain.SourceCollectionOutcome, evaluatedAt, evidenceAt time.Time) (ConditionResult, error) {
	if !resolver.freshCompleteStatus(collection.MetricStatus, evidenceAt) {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	rangeValue := domain.TimeRange{Start: evidenceAt.Add(-task.Rule.Window), End: evidenceAt}
	if err := resolver.requireCollectionCoverage(ctx, task.Source, domain.TimeRange{Start: evaluatedAt.Add(-task.Rule.Window), End: evaluatedAt}, evaluatedAt, domain.CapabilityMetricCurrent); err != nil {
		return ConditionResult{}, err
	}
	query := domain.MetricHistoryQuery{
		Source: task.Source, Resources: []domain.ResourceRef{*task.Resource}, Names: []domain.MetricName{metricName(task.Rule.Kind)},
		Range:      domain.TimeRange{Start: rangeValue.Start.Add(-resolver.maximumGap), End: rangeValue.End.Add(time.Microsecond)},
		PointLimit: domain.MaxMetricHistoryPoints,
	}
	var history domain.MetricHistoryResult
	var err error
	limit := domain.MaxMetricHistoryPoints
	if resolver.metrics != nil {
		limit = domain.MaximumAlertMetricHistory
		history, err = resolver.metrics.GetAlertMetricHistory(ctx, domain.AlertMetricHistoryQuery{
			Source: task.Source, Resource: *task.Resource, Name: query.Names[0], Range: query.Range, Limit: limit,
		})
	} else if reader, ok := provider.(domain.MetricHistoryProvider); ok {
		history, err = reader.GetMetricHistory(ctx, query)
	} else {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	if err != nil || !resolver.freshCompleteStatus(history.Status, evidenceAt) || len(history.Samples) > limit {
		return ConditionResult{}, errors.Join(ErrEvaluationUnavailable, err)
	}
	seen := make(map[domain.MetricSampleID]struct{}, len(history.Samples))
	for _, sample := range history.Samples {
		if sample.ResourceID != task.Resource.ID || sample.Name != query.Names[0] || sample.Synthetic != task.Synthetic ||
			sample.SampledAt.Before(query.Range.Start) || !sample.SampledAt.Before(query.Range.End) || sample.SampledAt.After(history.Status.ObservedAt) {
			return ConditionResult{}, ErrEvaluationUnavailable
		}
		if _, duplicate := seen[sample.ID]; duplicate {
			return ConditionResult{}, ErrEvaluationUnavailable
		}
		seen[sample.ID] = struct{}{}
	}
	value, err := metricWindowValue(history.Samples, rangeValue, resolver.maximumGap, task.Rule.Kind == domain.AlertRuleRestartThreshold)
	if err != nil {
		return ConditionResult{}, err
	}
	return Evaluate(task.Rule, EvaluationInput{
		SourceID: task.Source.SourceID, EnvironmentID: &task.Resource.Identity.EnvironmentID,
		ResourceID: &task.Resource.ID, EvidenceRange: rangeValue, WindowValue: &value,
	})
}

// Gauges use the last sampled value between samples. Counters add observed
// increases, including the new value after a reset, within the requested window.
func metricWindowValue(samples []domain.MetricSample, window domain.TimeRange, maximumGap time.Duration, counter bool) (float64, error) {
	points := make([]sampledValue, 0, len(samples))
	for index, sample := range samples {
		if sample.Validate() != nil || index > 0 &&
			(sample.ResourceID != samples[index-1].ResourceID || sample.Name != samples[index-1].Name) {
			return 0, ErrEvaluationUnavailable
		}
		points = append(points, sampledValue{value: sample.Value, sampledAt: sample.SampledAt})
	}
	return sampledWindowValue(points, window, maximumGap, counter)
}

type sampledValue struct {
	value     float64
	sampledAt time.Time
}

func sampledWindowValue(samples []sampledValue, window domain.TimeRange, maximumGap time.Duration, counter bool) (float64, error) {
	if window.Validate() != nil || maximumGap <= 0 || len(samples) == 0 {
		return 0, ErrEvaluationUnavailable
	}
	baseline := -1
	for index, sample := range samples {
		if validateEvaluationTime(sample.sampledAt) != nil || math.IsNaN(sample.value) || math.IsInf(sample.value, 0) || sample.value < 0 ||
			sample.value > 1<<53-1 || sample.sampledAt.After(window.End) || index > 0 && !sample.sampledAt.After(samples[index-1].sampledAt) {
			return 0, ErrEvaluationUnavailable
		}
		if !sample.sampledAt.After(window.Start) {
			baseline = index
		}
	}
	if baseline < 0 || window.Start.Sub(samples[baseline].sampledAt) > maximumGap {
		return 0, ErrEvaluationUnavailable
	}
	previous := samples[baseline]
	cursor := window.Start
	value := 0.0
	for _, sample := range samples[baseline+1:] {
		if sample.sampledAt.Sub(previous.sampledAt) > maximumGap {
			return 0, ErrEvaluationUnavailable
		}
		if counter {
			increment := sample.value - previous.value
			if increment < 0 {
				increment = sample.value
			}
			value += increment
		} else {
			value += previous.value * (float64(sample.sampledAt.Sub(cursor)) / float64(window.End.Sub(window.Start)))
		}
		cursor, previous = sample.sampledAt, sample
	}
	if window.End.Sub(previous.sampledAt) > maximumGap {
		return 0, ErrEvaluationUnavailable
	}
	if !counter {
		value += previous.value * (float64(window.End.Sub(cursor)) / float64(window.End.Sub(window.Start)))
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || value > 1<<53-1 {
		return 0, ErrEvaluationUnavailable
	}
	return value, nil
}

func (resolver *ProviderEvidence) requireCollectionCoverage(ctx context.Context, source domain.ProviderRef, window domain.TimeRange, evaluatedAt time.Time, port domain.CapabilityName) error {
	queryRange := domain.TimeRange{Start: window.Start.Add(-resolver.maximumGap), End: window.End.Add(time.Microsecond)}
	history, err := resolver.collections.ListSourceCollectionHistory(ctx, source, queryRange, domain.MaximumCollectionHistory)
	if err != nil || !history.Complete || len(history.Observations) == 0 || len(history.Observations) > domain.MaximumCollectionHistory {
		return errors.Join(ErrEvaluationUnavailable, err)
	}
	baseline := -1
	for index, outcome := range history.Observations {
		if !resolver.freshCollection(outcome, outcome.CollectedAt) || outcome.CollectedAt.After(evaluatedAt) ||
			outcome.CollectedAt.Before(queryRange.Start) || !outcome.CollectedAt.Before(queryRange.End) ||
			index > 0 && !outcome.CollectedAt.After(history.Observations[index-1].CollectedAt) {
			return ErrEvaluationUnavailable
		}
		if !outcome.CollectedAt.After(window.Start) {
			baseline = index
		}
	}
	if baseline < 0 || window.Start.Sub(history.Observations[baseline].CollectedAt) > resolver.maximumGap {
		return ErrEvaluationUnavailable
	}
	previous := history.Observations[baseline].CollectedAt
	for _, outcome := range history.Observations[baseline:] {
		portStatus := outcome.Status
		if port == domain.CapabilityMetricCurrent {
			portStatus = outcome.MetricStatus
		} else if port == domain.CapabilityEventHistory {
			portStatus = outcome.EventStatus
		}
		if !completeStatus(outcome.Status) || !completeStatus(portStatus) || portStatus.ObservedAt.After(outcome.CollectedAt) ||
			outcome.CollectedAt.Sub(previous) > resolver.maximumGap {
			return ErrEvaluationUnavailable
		}
		if source.Provider != domain.ProviderDemo && (outcome.CollectedAt.Sub(portStatus.ObservedAt) > resolver.maximumGap ||
			outcome.CollectedAt.Sub(outcome.Status.ObservedAt) > resolver.maximumGap) {
			return ErrEvaluationUnavailable
		}
		previous = outcome.CollectedAt
	}
	if window.End.Sub(previous) > resolver.maximumGap {
		return ErrEvaluationUnavailable
	}
	return nil
}
