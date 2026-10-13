package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestObservationLogicalReadKeepsStatusRowsAndEvidenceOneSnapshot(t *testing.T) {
	fixture := newObservationFixture(t)
	refs := []domain.ResourceRef{fixture.batch.Resources[0].Resource.Ref}
	healthQuery := domain.HealthQuery{Source: fixture.token.Source, Resources: refs}
	result, err := readObservation(fixture.ctx, fixture.store, func(ctx context.Context, reader *observationRead) (domain.HealthResult, error) {
		status, err := reader.GetObservationStatus(ctx, fixture.token.Source)
		if err != nil || status.State != domain.AvailabilityAvailable {
			t.Fatal("pin initial observation snapshot")
		}
		var originalHealthTime int64
		if err := reader.queries.QueryRow(ctx, `SELECT observed_at_us FROM resource_health_current WHERE resource_id=$1`, string(refs[0].ID)).Scan(&originalHealthTime); err != nil {
			t.Fatal("pin health row before concurrent evidence update")
		}
		token, err := fixture.store.BeginCapture(ctx, fixture.source.Source.ID, 1, fixture.at.Add(time.Second))
		if err != nil {
			t.Fatal("begin writer while read snapshot stays open")
		}
		health := fixture.batch.Health[0]
		health.State = domain.HealthDegraded
		health.Status.ObservedAt = token.ObservedAt
		health.Status.ReasonCode = "health.degraded"
		health.Evidence = append([]domain.HealthEvidence(nil), health.Evidence...)
		health.Evidence[0].ObservedAt = token.ObservedAt
		health.Evidence[0].Code = "health.failed"
		health.Evidence[0].Status = domain.EvidenceNegative
		relationship := fixture.batch.Relationships[0]
		relationship.ObservedAt = token.ObservedAt
		relationship.Evidence = append([]domain.RelationshipEvidence(nil), relationship.Evidence...)
		relationship.Evidence[0].ObservedAt = token.ObservedAt
		resource := fixture.batch.Resources[0]
		resource.Resource.Version = "version_two"
		resource.Status.ObservedAt = token.ObservedAt
		if err := fixture.store.SaveCapture(ctx, domain.CaptureBatch{Token: token, Resources: []domain.ResourceSnapshot{resource}, Health: []domain.HealthSnapshot{health}, Relationships: []domain.Relationship{relationship}}); err != nil {
			t.Fatal("commit changed health, evidence and version during read")
		}
		completion := fixture.completion
		completion.Token = token
		completion.Complete = false
		completion.Status.ObservedAt = token.ObservedAt
		completion.Capabilities.Capabilities = append([]domain.CapabilityStatus(nil), completion.Capabilities.Capabilities...)
		for index := range completion.Capabilities.Capabilities {
			completion.Capabilities.Capabilities[index].Status.ObservedAt = token.ObservedAt
		}
		if err := fixture.store.FinishCapture(ctx, completion); err != nil {
			t.Fatal("finish writer during read snapshot")
		}
		detail, err := reader.GetResource(ctx, domain.ResourceDetailQuery{Source: token.Source, ResourceID: refs[0].ID})
		if err != nil || detail.Resource.Version != "version_one" {
			t.Fatal("logical read mixed changed detail into old source status")
		}
		topology, err := reader.GetRelationships(ctx, domain.TopologyQuery{Source: token.Source, Page: domain.PageRequest{Limit: 10}})
		if err != nil || len(topology.Relationships) != 1 || topology.Relationships[0].ObservedAt.UnixMicro() != originalHealthTime || topology.Relationships[0].Evidence[0].ObservedAt.UnixMicro() != originalHealthTime {
			t.Fatal("logical read mixed new relationship evidence with old edge")
		}
		return reader.GetHealth(ctx, healthQuery)
	})
	if err != nil || len(result.Snapshots) != 1 || result.Snapshots[0].State != domain.HealthHealthy || result.Snapshots[0].Evidence[0].Code != "health.ready" {
		t.Fatalf("snapshot health read mixed concurrent evidence: %v", err)
	}
	fresh, err := fixture.store.GetHealth(fixture.ctx, healthQuery)
	if err != nil || len(fresh.Snapshots) != 1 || fresh.Snapshots[0].State != domain.HealthDegraded || fresh.Snapshots[0].Evidence[0].Code != "health.failed" {
		t.Fatalf("new read did not see committed health evidence: %v", err)
	}
}
