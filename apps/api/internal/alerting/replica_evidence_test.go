package alerting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type replicaHistoryStub struct {
	result domain.WorkloadReplicaHistoryResult
}

func (stub replicaHistoryStub) GetWorkloadReplicaHistory(context.Context, domain.WorkloadReplicaHistoryQuery) (domain.WorkloadReplicaHistoryResult, error) {
	return stub.result, nil
}

func TestReplicaRuleUsesWeightedWindowAndAcceptsFractionalMean(t *testing.T) {
	task, provider, collections := metricEvidenceFixture(t, domain.AlertRuleRestartThreshold)
	task.Rule.Kind = domain.AlertRuleWorkloadUnavailableReplicas
	comparison := domain.ComparisonLessThan
	task.Rule.Comparison = &comparison
	first, second := collectionOutcome(provider.at.Add(-time.Minute)).Status, collectionOutcome(provider.at.Add(-30*time.Second)).Status
	reader := replicaHistoryStub{result: domain.WorkloadReplicaHistoryResult{Status: collectionOutcome(provider.at).Status,
		Samples: []domain.WorkloadReplicaSnapshot{{Resource: *task.Resource, Desired: 2, Ready: 1, Status: first},
			{Resource: *task.Resource, Desired: 2, Ready: 2, Status: second}}}}
	resolver, err := NewProviderEvidenceWithOptions(ProviderEvidenceOptions{Providers: []EvidenceProvider{provider}, Collections: collections,
		Replicas: reader, MaximumCollectionGap: defaultMaximumCollectionGap})
	if err != nil {
		t.Fatal(err)
	}
	result, err := resolver.ResolveAlertEvidence(context.Background(), task, provider.at)
	if err != nil || !result.Matched || result.EvidenceRange.End.Sub(result.EvidenceRange.Start) != time.Minute {
		t.Fatalf("fractional deficit mean = %#v, %v; .5 <1 must match", result, err)
	}
}

func TestReplicaWindowRejectsMissingDeniedResetAndFuture(t *testing.T) {
	for _, name := range []string{"missing", "denied", "invalid_counts", "future", "capped", "other_resource"} {
		t.Run(name, func(t *testing.T) {
			task, provider, collections := metricEvidenceFixture(t, domain.AlertRuleRestartThreshold)
			task.Rule.Kind = domain.AlertRuleWorkloadUnavailableReplicas
			first, second := collectionOutcome(provider.at.Add(-time.Minute)).Status, collectionOutcome(provider.at.Add(-30*time.Second)).Status
			reader := replicaHistoryStub{result: domain.WorkloadReplicaHistoryResult{Status: collectionOutcome(provider.at).Status,
				Samples: []domain.WorkloadReplicaSnapshot{{Resource: *task.Resource, Desired: 2, Ready: 1, Status: first},
					{Resource: *task.Resource, Desired: 2, Ready: 2, Status: second}}}}
			switch name {
			case "missing":
				reader.result.Samples = reader.result.Samples[1:]
			case "denied":
				reader.result.Samples[1].Status.State, reader.result.Samples[1].Status.ReasonCode = domain.AvailabilityPermissionDenied, "source.denied"
			case "invalid_counts":
				reader.result.Samples[1].Ready = 3
			case "future":
				reader.result.Samples[1].Status.ObservedAt = provider.at.Add(time.Second)
			case "capped":
				reader.result.Status.Completeness, reader.result.Status.ReasonCode = domain.CompletenessPartial, "source.replica_history_limited"
			case "other_resource":
				reader.result.Samples[1].Resource.ID = *fixtureRule(0).ResourceID
			}
			resolver, err := NewProviderEvidenceWithOptions(ProviderEvidenceOptions{Providers: []EvidenceProvider{provider}, Collections: collections,
				Replicas: reader, MaximumCollectionGap: defaultMaximumCollectionGap})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := resolver.ResolveAlertEvidence(context.Background(), task, provider.at); !errors.Is(err, ErrEvaluationUnavailable) {
				t.Fatalf("%s replica window = %v; unknown must keep state", name, err)
			}
		})
	}
}
