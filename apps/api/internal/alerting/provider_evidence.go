package alerting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type EvidenceProvider interface {
	domain.InventoryProvider
	domain.HealthProvider
	Source() domain.InfrastructureSource
}

type ProviderEvidence struct {
	providers map[domain.SourceID]EvidenceProvider
}

func NewProviderEvidence(providers []EvidenceProvider) (*ProviderEvidence, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("alert evidence providers are required")
	}
	byID := make(map[domain.SourceID]EvidenceProvider, len(providers))
	for _, provider := range providers {
		if provider == nil || provider.Source().Validate() != nil {
			return nil, fmt.Errorf("alert evidence provider is invalid")
		}
		if _, exists := byID[provider.Source().ID]; exists {
			return nil, fmt.Errorf("alert evidence provider is duplicated")
		}
		byID[provider.Source().ID] = provider
	}
	return &ProviderEvidence{providers: byID}, nil
}

func (resolver *ProviderEvidence) ResolveAlertEvidence(ctx context.Context, task EvaluationTask, evaluatedAt time.Time) (ConditionResult, error) {
	if resolver == nil || ctx == nil || ctx.Err() != nil || task.Validate() != nil || validateEvaluationTime(evaluatedAt) != nil {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	provider, exists := resolver.providers[task.Source.SourceID]
	if !exists || provider.Source().Provider != task.Source.Provider {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	switch task.Rule.Kind {
	case domain.AlertRuleResourceUnavailable:
		snapshot, err := provider.GetResource(ctx, domain.ResourceDetailQuery{Source: task.Source, ResourceID: task.Resource.ID})
		if err != nil {
			return ConditionResult{}, err
		}
		availability := snapshot.Status.State
		return Evaluate(task.Rule, EvaluationInput{
			SourceID: task.Source.SourceID, EnvironmentID: &task.Resource.Identity.EnvironmentID,
			ResourceID: &task.Resource.ID, EvidenceRange: statusRange(snapshot.Status.ObservedAt),
			Availability: &availability,
		})
	case domain.AlertRuleResourceUnhealthy:
		result, err := provider.GetHealth(ctx, domain.HealthQuery{Source: task.Source, Resources: []domain.ResourceRef{*task.Resource}})
		if err != nil || len(result.Snapshots) != 1 {
			return ConditionResult{}, errors.Join(ErrEvaluationUnavailable, err)
		}
		health := result.Snapshots[0].State
		return Evaluate(task.Rule, EvaluationInput{
			SourceID: task.Source.SourceID, EnvironmentID: &task.Resource.Identity.EnvironmentID,
			ResourceID: &task.Resource.ID, EvidenceRange: statusRange(result.Snapshots[0].Status.ObservedAt),
			Health: &health,
		})
	case domain.AlertRuleProviderConnectionLost:
		page, err := provider.ListEnvironments(ctx, domain.EnvironmentQuery{
			Source: task.Source, Page: domain.PageRequest{Limit: 1},
		})
		if err != nil {
			return ConditionResult{}, err
		}
		availability := page.Status.State
		return Evaluate(task.Rule, EvaluationInput{
			SourceID: task.Source.SourceID, EvidenceRange: statusRange(page.Status.ObservedAt),
			Availability: &availability,
		})
	case domain.AlertRuleCPUThreshold, domain.AlertRuleMemoryThreshold, domain.AlertRuleRestartThreshold:
		metricProvider, ok := provider.(domain.CurrentMetricProvider)
		if !ok {
			return ConditionResult{}, ErrEvaluationUnavailable
		}
		name := metricName(task.Rule.Kind)
		result, err := metricProvider.GetCurrentMetrics(ctx, domain.CurrentMetricQuery{
			Source: task.Source, Resources: []domain.ResourceRef{*task.Resource}, Names: []domain.MetricName{name},
		})
		if err != nil || len(result.Sets) != 1 || len(result.Sets[0].Samples) != 1 {
			return ConditionResult{}, errors.Join(ErrEvaluationUnavailable, err)
		}
		sample := result.Sets[0].Samples[0]
		value := sample.Value
		return Evaluate(task.Rule, EvaluationInput{
			SourceID: task.Source.SourceID, EnvironmentID: &task.Resource.Identity.EnvironmentID,
			ResourceID: &task.Resource.ID, EvidenceRange: sampleRange(sample), WindowValue: &value,
		})
	case domain.AlertRuleEventSeverityMatch:
		eventProvider, ok := provider.(domain.EventHistoryProvider)
		if !ok {
			return ConditionResult{}, ErrEvaluationUnavailable
		}
		start := evaluatedAt.Add(-task.Rule.EvaluationInterval)
		query := domain.EventQuery{
			Source: task.Source, Range: domain.TimeRange{Start: start, End: evaluatedAt},
			Page: domain.PageRequest{Limit: 1},
		}
		if task.Resource != nil {
			query.Resources = []domain.ResourceRef{*task.Resource}
		}
		page, err := eventProvider.ListEvents(ctx, query)
		if err != nil {
			return ConditionResult{}, err
		}
		if len(page.Events) == 0 {
			return ConditionResult{
				Matched: false, EvidenceRange: domain.TimeRange{Start: start, End: evaluatedAt},
				SummaryCode: "alert.event_severity_match.clear",
			}, nil
		}
		severity := page.Events[0].Severity
		end := page.Events[0].ObservedAt
		if end.After(evaluatedAt) {
			end = evaluatedAt
		}
		evidenceStart := page.Events[0].OccurredAt
		if !evidenceStart.Before(end) {
			evidenceStart = end.Add(-time.Microsecond)
		}
		return Evaluate(task.Rule, EvaluationInput{
			SourceID: task.Source.SourceID, EnvironmentID: task.Rule.EnvironmentID,
			ResourceID:    task.Rule.ResourceID,
			EvidenceRange: domain.TimeRange{Start: evidenceStart, End: end},
			EventSeverity: &severity,
		})
	case domain.AlertRuleWorkloadUnavailableReplicas:
		replicaProvider, ok := provider.(domain.WorkloadReplicaProvider)
		if !ok {
			return ConditionResult{}, ErrEvaluationUnavailable
		}
		snapshot, err := replicaProvider.GetWorkloadReplicas(ctx, domain.ResourceDetailQuery{
			Source: task.Source, ResourceID: task.Resource.ID,
		})
		if err != nil {
			return ConditionResult{}, errors.Join(ErrEvaluationUnavailable, err)
		}
		unavailable := float64(snapshot.Desired - snapshot.Ready)
		return Evaluate(task.Rule, EvaluationInput{
			SourceID: task.Source.SourceID, EnvironmentID: &task.Resource.Identity.EnvironmentID,
			ResourceID: &task.Resource.ID, EvidenceRange: statusRange(snapshot.Status.ObservedAt),
			WindowValue: &unavailable,
		})
	default:
		return ConditionResult{}, ErrEvaluationUnavailable
	}
}

func metricName(kind domain.AlertRuleKind) domain.MetricName {
	switch kind {
	case domain.AlertRuleCPUThreshold:
		return domain.MetricCPUUsageCores
	case domain.AlertRuleMemoryThreshold:
		return domain.MetricMemoryUsageBytes
	default:
		return domain.MetricRestartCount
	}
}

func statusRange(observedAt time.Time) domain.TimeRange {
	return domain.TimeRange{Start: observedAt.Add(-time.Microsecond), End: observedAt}
}

func sampleRange(sample domain.MetricSample) domain.TimeRange {
	start := sample.SampledAt.Add(-sample.Interval)
	if !start.Before(sample.SampledAt) {
		start = sample.SampledAt.Add(-time.Microsecond)
	}
	return domain.TimeRange{Start: start, End: sample.SampledAt}
}
