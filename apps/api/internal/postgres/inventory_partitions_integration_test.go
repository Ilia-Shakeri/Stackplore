package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestInventoryPartitionsRetireCompleteKindWhileDeniedKindIsProtected(t *testing.T) {
	fixture := newObservationFixture(t)
	source := fixture.token.Source
	environment := fixture.batch.Environments[0]
	identity, err := domain.NewResourceIdentity(source.Provider, source.SourceID, environment.ID, "demo.worker", domain.Scope{Kind: domain.ScopeProject, Name: "gone-project"}, "worker_one")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	worker := domain.ResourceSnapshot{Resource: domain.Resource{Ref: ref, DisplayName: "worker", Synthetic: true}, Status: fixture.completion.Status}
	partition := func(kind domain.ResourceKind, state domain.AvailabilityState, complete domain.Completeness, at time.Time) domain.InventoryCoverage {
		status := domain.ObservationStatus{State: state, Completeness: complete, ObservedAt: at}
		if state != domain.AvailabilityAvailable || complete != domain.CompletenessComplete {
			status.ReasonCode = "source.partition_partial"
		}
		return domain.InventoryCoverage{Source: source, EnvironmentID: environment.ID, Kind: kind, ScopeMode: domain.InventoryAllScopes, Status: status}
	}
	capture := func(generation int, resources []domain.ResourceSnapshot, coverage []domain.InventoryCoverage, complete bool) domain.CaptureToken {
		t.Helper()
		at := fixture.at.Add(time.Duration(generation) * time.Second)
		token, err := fixture.store.BeginCapture(fixture.ctx, source.SourceID, 1, at)
		if err != nil {
			t.Fatal(err)
		}
		batch := domain.CaptureBatch{Token: token, Environments: []domain.Environment{environment}, Resources: resources}
		if err := fixture.store.SaveCapture(fixture.ctx, batch); err != nil {
			t.Fatal(err)
		}
		completion := fixture.completion
		completion.Token, completion.Complete, completion.Coverage = token, complete, coverage
		completion.Status.ObservedAt = at
		if !complete {
			completion.Status.Completeness, completion.Status.ReasonCode = domain.CompletenessPartial, "source.partition_partial"
		}
		if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
			t.Fatal(err)
		}
		return token
	}
	all := append(append([]domain.ResourceSnapshot{}, fixture.batch.Resources...), worker)
	at := fixture.at.Add(time.Second)
	capture(1, all, []domain.InventoryCoverage{partition("demo.service", domain.AvailabilityAvailable, domain.CompletenessComplete, at), partition("demo.worker", domain.AvailabilityAvailable, domain.CompletenessComplete, at)}, true)
	at = fixture.at.Add(2 * time.Second)
	token := capture(2, fixture.batch.Resources[:2], []domain.InventoryCoverage{partition("demo.service", domain.AvailabilityAvailable, domain.CompletenessComplete, at), partition("demo.worker", domain.AvailabilityPermissionDenied, domain.CompletenessComplete, at)}, false)
	page, err := fixture.store.ListResources(fixture.ctx, domain.InventoryQuery{Source: source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 100}})
	if err != nil || len(page.Resources) != 2 || page.Status.Completeness != domain.CompletenessPartial {
		t.Fatalf("independent allowed inventory: %d, %v", len(page.Resources), err)
	}
	if _, err := fixture.store.GetResource(fixture.ctx, domain.ResourceDetailQuery{Source: source, ResourceID: ref.ID}); !errors.Is(err, domain.ErrRepositoryNotFound) {
		t.Fatalf("denied cached worker exposed: %v", err)
	}
	retired, err := fixture.store.RetireUnseen(fixture.ctx, token, 100)
	if err != nil || retired != 1 {
		t.Fatalf("complete service partition retirement=%d, %v", retired, err)
	}
	var protected bool
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT retired_at_us IS NULL FROM resources WHERE id=$1`, string(ref.ID)).Scan(&protected); err != nil || !protected {
		t.Fatal("denied partition was retired")
	}
	// A truncated allowed partition cannot infer deletion of its missing member.
	at = fixture.at.Add(3 * time.Second)
	token = capture(3, fixture.batch.Resources[:1], []domain.InventoryCoverage{partition("demo.service", domain.AvailabilityAvailable, domain.CompletenessPartial, at), partition("demo.worker", domain.AvailabilityPermissionDenied, domain.CompletenessComplete, at)}, false)
	retired, err = fixture.store.RetireUnseen(fixture.ctx, token, 100)
	if err != nil || retired != 0 {
		t.Fatalf("partial partition retired data=%d, %v", retired, err)
	}
	page, err = fixture.store.ListResources(fixture.ctx, domain.InventoryQuery{Source: source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 100}})
	if err != nil || len(page.Resources) != 2 {
		t.Fatalf("partial partition dropped unseen row: %d, %v", len(page.Resources), err)
	}
	// Empty successful lists are deletion proof, including vanished project scopes.
	at = fixture.at.Add(4 * time.Second)
	token = capture(4, nil, []domain.InventoryCoverage{partition("demo.service", domain.AvailabilityAvailable, domain.CompletenessComplete, at), partition("demo.worker", domain.AvailabilityAvailable, domain.CompletenessComplete, at)}, true)
	for count := 0; count < 4; count++ {
		if _, err := fixture.store.RetireUnseen(fixture.ctx, token, 1); err != nil {
			t.Fatal(err)
		}
	}
	page, err = fixture.store.ListResources(fixture.ctx, domain.InventoryQuery{Source: source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 100}})
	if err != nil || len(page.Resources) != 0 {
		t.Fatal("empty complete partition did not retire vanished scopes")
	}
	if err := fixture.store.FinishCapture(fixture.ctx, fixture.completion); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("stale generation published: %v", err)
	}
}
