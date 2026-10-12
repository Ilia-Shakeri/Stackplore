package alerting

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type replicaEvidenceProvider struct {
	resource domain.ResourceRef
	status   domain.ObservationStatus
	desired  uint64
	ready    uint64
}

func (provider replicaEvidenceProvider) Source() domain.InfrastructureSource {
	return domain.InfrastructureSource{ID: "source_cluster", Provider: domain.ProviderKubernetes, Name: "Main cluster"}
}

func (provider replicaEvidenceProvider) ListEnvironments(context.Context, domain.EnvironmentQuery) (domain.EnvironmentPage, error) {
	return domain.EnvironmentPage{}, nil
}

func (provider replicaEvidenceProvider) ListResources(context.Context, domain.InventoryQuery) (domain.ResourcePage, error) {
	return domain.ResourcePage{}, nil
}

func (provider replicaEvidenceProvider) GetResource(context.Context, domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	return domain.ResourceSnapshot{}, nil
}

func (provider replicaEvidenceProvider) GetHealth(context.Context, domain.HealthQuery) (domain.HealthResult, error) {
	return domain.HealthResult{}, nil
}

func (provider replicaEvidenceProvider) GetWorkloadReplicas(_ context.Context, _ domain.ResourceDetailQuery) (domain.WorkloadReplicaSnapshot, error) {
	return domain.WorkloadReplicaSnapshot{
		Resource: provider.resource, Desired: provider.desired, Ready: provider.ready, Status: provider.status,
	}, nil
}

func TestProviderEvidenceEvaluatesUnavailableWorkloadReplicas(t *testing.T) {
	at := time.Date(2026, time.August, 21, 14, 0, 0, 0, time.UTC)
	identity, err := domain.NewResourceIdentity(
		domain.ProviderKubernetes, "source_cluster", "env_cluster", "kubernetes.deployment",
		domain.Scope{Kind: domain.ScopeNamespace, Name: "shop"}, "deployment-one",
	)
	if err != nil {
		t.Fatal(err)
	}
	resource, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	provider := replicaEvidenceProvider{
		resource: resource, desired: 3, ready: 1,
		status: domain.ObservationStatus{
			State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at.Add(-time.Second),
		},
	}
	resolver, err := NewProviderEvidence([]EvidenceProvider{provider})
	if err != nil {
		t.Fatal(err)
	}
	comparison := domain.ComparisonGreaterThanOrEqual
	threshold := uint64(2)
	rule := domain.AlertRule{
		ID: "rule_workload_replicas", SourceID: "source_cluster", ResourceID: &resource.ID,
		DisplayName: "Unavailable replicas", Kind: domain.AlertRuleWorkloadUnavailableReplicas,
		Enabled: true, Severity: domain.SeverityError, Comparison: &comparison, ThresholdCount: &threshold,
		Window: time.Minute, EvaluationInterval: time.Minute,
	}
	task := EvaluationTask{
		Rule: rule, Source: domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"},
		Resource: &resource, DedupeKey: "rule_workload_replicas", ClaimedAt: at, NextEvaluationAt: at.Add(time.Minute),
	}
	result, err := resolver.ResolveAlertEvidence(context.Background(), task, at)
	if err != nil || !result.Matched || result.SummaryCode != "alert.workload_unavailable_replicas.matched" {
		t.Fatalf("workload replica evidence = (%#v, %v)", result, err)
	}
}
