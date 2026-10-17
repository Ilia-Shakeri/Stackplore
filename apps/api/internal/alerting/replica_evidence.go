package alerting

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (resolver *ProviderEvidence) replicaCondition(ctx context.Context, provider EvidenceProvider, task EvaluationTask, evaluatedAt, evidenceAt time.Time) (ConditionResult, error) {
	reader := resolver.replicas
	if reader == nil {
		reader, _ = provider.(domain.WorkloadReplicaHistoryProvider)
	}
	if reader == nil {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	window := domain.TimeRange{Start: evidenceAt.Add(-task.Rule.Window), End: evidenceAt}
	if err := resolver.requireCollectionCoverage(ctx, task.Source, domain.TimeRange{Start: evaluatedAt.Add(-task.Rule.Window), End: evaluatedAt}, evaluatedAt, domain.CapabilityInventoryList); err != nil {
		return ConditionResult{}, err
	}
	query := domain.WorkloadReplicaHistoryQuery{Source: task.Source, Resource: *task.Resource,
		Range: domain.TimeRange{Start: window.Start.Add(-resolver.maximumGap), End: window.End.Add(time.Microsecond)}, Limit: domain.MaximumAlertMetricHistory}
	history, err := reader.GetWorkloadReplicaHistory(ctx, query)
	if err != nil || !resolver.freshCompleteStatus(history.Status, evidenceAt) || len(history.Samples) > query.Limit {
		return ConditionResult{}, errors.Join(ErrEvaluationUnavailable, err)
	}
	detailQuery := domain.ResourceDetailQuery{Source: task.Source, ResourceID: task.Resource.ID}
	samples := make([]sampledValue, 0, len(history.Samples))
	for _, snapshot := range history.Samples {
		if snapshot.ValidateFor(detailQuery) != nil || !completeStatus(snapshot.Status) ||
			snapshot.Status.ObservedAt.Before(query.Range.Start) || !snapshot.Status.ObservedAt.Before(query.Range.End) ||
			snapshot.Status.ObservedAt.After(history.Status.ObservedAt) {
			return ConditionResult{}, ErrEvaluationUnavailable
		}
		deficit := uint64(0)
		if snapshot.Desired > snapshot.Ready {
			deficit = snapshot.Desired - snapshot.Ready
		}
		samples = append(samples, sampledValue{value: float64(deficit), sampledAt: snapshot.Status.ObservedAt})
	}
	value, err := sampledWindowValue(samples, window, resolver.maximumGap, false)
	if err != nil {
		return ConditionResult{}, err
	}
	return Evaluate(task.Rule, EvaluationInput{SourceID: task.Source.SourceID, EnvironmentID: &task.Resource.Identity.EnvironmentID,
		ResourceID: &task.Resource.ID, EvidenceRange: window, WindowValue: &value})
}
