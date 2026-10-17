package postgres

import (
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestPartialPartitionCannotSupplyCompleteReplicaEvidence(t *testing.T) {
	fixture := newObservationFixture(t)
	source, ref := fixture.token.Source, fixture.batch.Resources[0].Resource.Ref
	at := fixture.at.Add(time.Second)
	token, err := fixture.store.BeginCapture(fixture.ctx, source.SourceID, fixture.source.Version, at)
	if err != nil {
		t.Fatal(err)
	}
	completion := fixture.completion
	completion.Token, completion.Complete = token, false
	completion.Status.Completeness, completion.Status.ReasonCode = domain.CompletenessPartial, "source.collection_partial"
	partial := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessPartial, ObservedAt: at, ReasonCode: "source.partition_partial"}
	completion.Coverage = []domain.InventoryCoverage{{Source: source, EnvironmentID: ref.Identity.EnvironmentID, Kind: ref.Identity.Kind, ScopeMode: domain.InventoryAllScopes, Status: partial}}
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal(err)
	}
	current, err := fixture.store.GetWorkloadReplicas(fixture.ctx, domain.ResourceDetailQuery{Source: source, ResourceID: ref.ID})
	if err != nil || current.Status.Completeness != domain.CompletenessPartial || current.Status.ReasonCode != "source.partition_partial" {
		t.Fatalf("partial inventory claimed complete replicas: %#v, %v", current, err)
	}
	history, err := fixture.store.GetWorkloadReplicaHistory(fixture.ctx, domain.WorkloadReplicaHistoryQuery{Source: source, Resource: ref, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: at}, Limit: 100})
	if err != nil || history.Status.Completeness != domain.CompletenessPartial || len(history.Samples) != 0 {
		t.Fatalf("partial inventory exposed complete alert evidence: %#v, %v", history, err)
	}
}

func TestAlertMetricHistoryCannotReadRevokedPartitionEvidence(t *testing.T) {
	fixture := newObservationFixture(t)
	source, ref := fixture.token.Source, fixture.batch.Resources[0].Resource.Ref
	query := domain.AlertMetricHistoryQuery{Source: source, Resource: ref, Name: domain.MetricCPUUsageCores, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at}, Limit: 100}
	before, err := fixture.store.GetAlertMetricHistory(fixture.ctx, query)
	if err != nil || len(before.Samples) != 1 {
		t.Fatalf("missing baseline metric: %#v, %v", before, err)
	}
	at := fixture.at.Add(time.Second)
	token, err := fixture.store.BeginCapture(fixture.ctx, source.SourceID, fixture.source.Version, at)
	if err != nil {
		t.Fatal(err)
	}
	completion := fixture.completion
	completion.Token, completion.Complete = token, false
	completion.Status.Completeness, completion.Status.ReasonCode = domain.CompletenessPartial, "source.collection_partial"
	completion.Coverage = []domain.InventoryCoverage{{Source: source, EnvironmentID: ref.Identity.EnvironmentID, Kind: ref.Identity.Kind, ScopeMode: domain.InventoryAllScopes, Status: domain.ObservationStatus{State: domain.AvailabilityPermissionDenied, Completeness: domain.CompletenessComplete, ObservedAt: at, ReasonCode: "source.partition_denied"}}}
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal(err)
	}
	after, err := fixture.store.GetAlertMetricHistory(fixture.ctx, query)
	if err != nil || len(after.Samples) != 0 {
		t.Fatalf("revoked cached metric entered alert evidence: %#v, %v", after, err)
	}
}
