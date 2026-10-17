package postgres

import (
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestPartitionedEventsRequireCurrentResourceAuthority(t *testing.T) {
	fixture := newObservationFixture(t)
	ctx, source := fixture.ctx, fixture.token.Source
	at := fixture.at.Add(time.Second)
	token, err := fixture.store.BeginCapture(ctx, source.SourceID, 1, at)
	if err != nil {
		t.Fatal(err)
	}
	unscoped := fixture.batch.Events[0]
	unscoped.ID, unscoped.ResourceID = "event_unscoped_prior_authority", nil
	if err := fixture.store.SaveCapture(ctx, domain.CaptureBatch{Token: token, Events: []domain.InfrastructureEvent{unscoped}}); err != nil {
		t.Fatal(err)
	}
	completion := fixture.completion
	completion.Token = token
	if err := fixture.store.FinishCapture(ctx, completion); err != nil {
		t.Fatal(err)
	}
	query := domain.EventQuery{Source: source, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: at.Add(time.Second)}, Page: domain.PageRequest{Limit: 10}}
	before, err := fixture.store.ListEvents(ctx, query)
	if err != nil || len(before.Events) != 4 {
		t.Fatalf("legacy source event fixture: %v, %d", err, len(before.Events))
	}

	// A successful narrowed publication has no explicit denied partition for
	// omitted authority. Unscoped history cannot prove its original namespace.
	at = at.Add(time.Second)
	token, err = fixture.store.BeginCapture(ctx, source.SourceID, 1, at)
	if err != nil {
		t.Fatal(err)
	}
	status := fixture.completion.Status
	status.ObservedAt = at
	completion.Token, completion.Complete = token, false
	completion.Status = status
	completion.Status.Completeness, completion.Status.ReasonCode = domain.CompletenessPartial, "source.inventory_partial"
	completion.Coverage = []domain.InventoryCoverage{{Source: source, EnvironmentID: fixture.batch.Environments[0].ID, Kind: fixture.batch.Resources[0].Resource.Ref.Identity.Kind, ScopeMode: domain.InventoryAllScopes, Status: status}}
	if err := fixture.store.FinishCapture(ctx, completion); err != nil {
		t.Fatal(err)
	}
	query.Range.End = at.Add(time.Second)
	after, err := fixture.store.ListEvents(ctx, query)
	if err != nil || len(after.Events) != 3 {
		t.Fatalf("partition publication exposed unscoped history: %v, %d", err, len(after.Events))
	}
	for _, event := range after.Events {
		if event.ResourceID == nil {
			t.Fatal("event without current authority escaped")
		}
	}

	// Revoking the linked resource kind also closes its retained history.
	at = at.Add(time.Second)
	token, err = fixture.store.BeginCapture(ctx, source.SourceID, 1, at)
	if err != nil {
		t.Fatal(err)
	}
	completion.Token = token
	completion.Coverage[0].Status = domain.ObservationStatus{State: domain.AvailabilityPermissionDenied, Completeness: domain.CompletenessComplete, ObservedAt: at, ReasonCode: "source.permission_denied"}
	if err := fixture.store.FinishCapture(ctx, completion); err != nil {
		t.Fatal(err)
	}
	after, err = fixture.store.ListEvents(ctx, query)
	if err != nil || len(after.Events) != 0 {
		t.Fatalf("revoked resource event history escaped: %v, %d", err, len(after.Events))
	}
}
