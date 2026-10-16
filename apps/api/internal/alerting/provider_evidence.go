package alerting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const defaultMaximumCollectionGap = 90 * time.Second

type EvidenceProvider interface {
	domain.InventoryProvider
	domain.HealthProvider
	Source() domain.InfrastructureSource
}

type ProviderEvidenceOptions struct {
	Providers            []EvidenceProvider
	Collections          domain.SourceCollectionReader
	Metrics              domain.AlertMetricHistoryProvider
	Replicas             domain.WorkloadReplicaHistoryProvider
	MaximumCollectionGap time.Duration
}

type ProviderEvidence struct {
	providers   map[domain.SourceID]EvidenceProvider
	collections domain.SourceCollectionReader
	metrics     domain.AlertMetricHistoryProvider
	replicas    domain.WorkloadReplicaHistoryProvider
	maximumGap  time.Duration
}

func NewProviderEvidence(providers []EvidenceProvider, collections ...domain.SourceCollectionReader) (*ProviderEvidence, error) {
	if len(collections) != 1 {
		return nil, fmt.Errorf("raw source collection evidence is required")
	}
	return NewProviderEvidenceWithOptions(ProviderEvidenceOptions{
		Providers: providers, Collections: collections[0], MaximumCollectionGap: defaultMaximumCollectionGap,
	})
}

func NewProviderEvidenceWithOptions(options ProviderEvidenceOptions) (*ProviderEvidence, error) {
	if len(options.Providers) == 0 || options.Collections == nil || options.MaximumCollectionGap < time.Second || options.MaximumCollectionGap > 5*time.Minute {
		return nil, fmt.Errorf("alert evidence options are invalid")
	}
	byID := make(map[domain.SourceID]EvidenceProvider, len(options.Providers))
	for _, provider := range options.Providers {
		if provider == nil || provider.Source().Validate() != nil {
			return nil, fmt.Errorf("alert evidence provider is invalid")
		}
		if _, exists := byID[provider.Source().ID]; exists {
			return nil, fmt.Errorf("alert evidence provider is duplicated")
		}
		byID[provider.Source().ID] = provider
	}
	if options.Metrics == nil {
		options.Metrics, _ = options.Collections.(domain.AlertMetricHistoryProvider)
	}
	if options.Replicas == nil {
		options.Replicas, _ = options.Collections.(domain.WorkloadReplicaHistoryProvider)
	}
	return &ProviderEvidence{providers: byID, collections: options.Collections, metrics: options.Metrics,
		replicas: options.Replicas, maximumGap: options.MaximumCollectionGap}, nil
}

func (resolver *ProviderEvidence) ResolveAlertEvidence(ctx context.Context, task EvaluationTask, evaluatedAt time.Time) (ConditionResult, error) {
	if resolver == nil || ctx == nil || ctx.Err() != nil || task.Validate() != nil || validateEvaluationTime(evaluatedAt) != nil {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	provider, exists := resolver.providers[task.Source.SourceID]
	if !exists || provider.Source().Provider != task.Source.Provider || provider.Source().Synthetic != task.Synthetic {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	collection, err := resolver.collections.GetSourceCollectionStatus(ctx, task.Source)
	if err != nil || !resolver.freshCollection(collection, evaluatedAt) {
		return ConditionResult{}, errors.Join(ErrEvaluationUnavailable, err)
	}
	if task.Source.Provider != domain.ProviderDemo && !resolver.freshStatus(collection.Status, evaluatedAt) {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	if task.Rule.Kind == domain.AlertRuleProviderConnectionLost {
		return connectionCondition(task.Rule, collection)
	}
	if !completeStatus(collection.Status) {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	evidenceAt := evaluatedAt
	if task.Synthetic {
		if clock, ok := provider.(interface{ ObservationTime() time.Time }); ok {
			evidenceAt = clock.ObservationTime()
		}
	}
	if validateEvaluationTime(evidenceAt) != nil || evidenceAt.After(evaluatedAt) {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	if !resolver.freshCompleteStatus(collection.Status, evidenceAt) {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	switch task.Rule.Kind {
	case domain.AlertRuleResourceUnavailable:
		query := domain.ResourceDetailQuery{Source: task.Source, ResourceID: task.Resource.ID}
		snapshot, err := provider.GetResource(ctx, query)
		if errors.Is(err, domain.ErrRepositoryNotFound) {
			availability := domain.AvailabilityUnavailable
			return Evaluate(task.Rule, EvaluationInput{
				SourceID: task.Source.SourceID, EnvironmentID: &task.Resource.Identity.EnvironmentID,
				ResourceID: &task.Resource.ID, EvidenceRange: statusRange(collection.CollectedAt), Availability: &availability,
			})
		}
		if err != nil || snapshot.ValidateFor(query) != nil || snapshot.Resource.Synthetic != task.Synthetic || !resolver.freshStatus(snapshot.Status, evidenceAt) ||
			snapshot.Status.Completeness != domain.CompletenessComplete {
			return ConditionResult{}, errors.Join(ErrEvaluationUnavailable, err)
		}
		availability := snapshot.Status.State
		return Evaluate(task.Rule, EvaluationInput{
			SourceID: task.Source.SourceID, EnvironmentID: &task.Resource.Identity.EnvironmentID,
			ResourceID: &task.Resource.ID, EvidenceRange: statusRange(snapshot.Status.ObservedAt), Availability: &availability,
		})
	case domain.AlertRuleResourceUnhealthy:
		query := domain.HealthQuery{Source: task.Source, Resources: []domain.ResourceRef{*task.Resource}}
		result, err := provider.GetHealth(ctx, query)
		if err != nil || result.ValidateFor(query) != nil || len(result.Snapshots) != 1 ||
			!resolver.freshCompleteStatus(result.Status, evidenceAt) || !resolver.freshCompleteStatus(result.Snapshots[0].Status, evidenceAt) {
			return ConditionResult{}, errors.Join(ErrEvaluationUnavailable, err)
		}
		for _, evidence := range result.Snapshots[0].Evidence {
			if evidence.Synthetic != task.Synthetic || evidenceAt.Sub(evidence.ObservedAt) > resolver.maximumGap {
				return ConditionResult{}, ErrEvaluationUnavailable
			}
		}
		health := result.Snapshots[0].State
		return Evaluate(task.Rule, EvaluationInput{
			SourceID: task.Source.SourceID, EnvironmentID: &task.Resource.Identity.EnvironmentID,
			ResourceID: &task.Resource.ID, EvidenceRange: statusRange(result.Snapshots[0].Status.ObservedAt), Health: &health,
		})
	case domain.AlertRuleCPUThreshold, domain.AlertRuleMemoryThreshold, domain.AlertRuleRestartThreshold:
		return resolver.metricCondition(ctx, provider, task, collection, evaluatedAt, evidenceAt)
	case domain.AlertRuleEventSeverityMatch:
		return resolver.eventCondition(ctx, provider, task, collection, evaluatedAt, evidenceAt)
	case domain.AlertRuleWorkloadUnavailableReplicas:
		return resolver.replicaCondition(ctx, provider, task, evaluatedAt, evidenceAt)
	default:
		return ConditionResult{}, ErrEvaluationUnavailable
	}
}

func connectionCondition(rule domain.AlertRule, collection domain.SourceCollectionOutcome) (ConditionResult, error) {
	status := collection.Status
	if status.Completeness != domain.CompletenessComplete || status.State != domain.AvailabilityAvailable &&
		(status.State != domain.AvailabilityUnavailable || status.ReasonCode != "observation.provider_unavailable" && status.ReasonCode != "observation.deadline_exceeded") {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	availability := status.State
	return Evaluate(rule, EvaluationInput{
		SourceID: rule.SourceID, EvidenceRange: statusRange(collection.CollectedAt), Availability: &availability,
	})
}

func completeStatus(status domain.ObservationStatus) bool {
	return status.Validate() == nil && status.State == domain.AvailabilityAvailable && status.Completeness == domain.CompletenessComplete
}

func (resolver *ProviderEvidence) freshStatus(status domain.ObservationStatus, at time.Time) bool {
	return status.Validate() == nil && !status.ObservedAt.After(at) && at.Sub(status.ObservedAt) <= resolver.maximumGap
}

func (resolver *ProviderEvidence) freshCompleteStatus(status domain.ObservationStatus, at time.Time) bool {
	return completeStatus(status) && resolver.freshStatus(status, at)
}

func (resolver *ProviderEvidence) freshCollection(collection domain.SourceCollectionOutcome, evaluatedAt time.Time) bool {
	return validateEvaluationTime(collection.CollectedAt) == nil && !collection.CollectedAt.After(evaluatedAt) &&
		evaluatedAt.Sub(collection.CollectedAt) <= resolver.maximumGap && collection.Status.Validate() == nil &&
		!collection.Status.ObservedAt.After(evaluatedAt)
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
