package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestOperatorAuthorityFenceSurvivesOutageAndHidesRevokedCachedKinds(t *testing.T) {
	fixture := newObservationFixture(t)
	source := fixture.token.Source
	if err := fixture.store.EnsureObservationPolicy(fixture.ctx, source, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	configuration, err := fixture.store.GetSourceConfiguration(fixture.ctx, source.SourceID)
	if err != nil || configuration.Version != 2 {
		t.Fatal("authority change did not fence configuration version")
	}
	if err := fixture.store.EnsureObservationPolicy(fixture.ctx, source, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	status, err := fixture.store.GetObservationStatus(fixture.ctx, source)
	if err != nil || status.State != domain.AvailabilityPermissionDenied || status.ReasonCode != "source.profile_changed" {
		t.Fatal("changed authority served old data")
	}
	at := fixture.at.Add(time.Second)
	token, err := fixture.store.BeginCapture(fixture.ctx, source.SourceID, configuration.Version, at)
	if err != nil {
		t.Fatal(err)
	}
	completion := fixture.completion
	completion.Token, completion.Complete = token, false
	completion.Status = domain.ObservationStatus{State: domain.AvailabilityUnavailable, Completeness: domain.CompletenessComplete, ObservedAt: at, ReasonCode: "source.connection_lost"}
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal(err)
	}
	status, err = fixture.store.GetObservationStatus(fixture.ctx, source)
	if err != nil || status.State != domain.AvailabilityPermissionDenied {
		t.Fatal("outage cleared authority fence and exposed old cache")
	}
	// The newly permitted inventory kind differs from every old cached identity.
	at = at.Add(time.Second)
	token, err = fixture.store.BeginCapture(fixture.ctx, source.SourceID, configuration.Version, at)
	if err != nil {
		t.Fatal(err)
	}
	completion = fixture.completion
	completion.Token, completion.Complete = token, false
	completion.Coverage = []domain.InventoryCoverage{{Source: source, EnvironmentID: fixture.batch.Environments[0].ID, Kind: "demo.worker", ScopeMode: domain.InventoryAllScopes, Status: fixture.completion.Status}}
	completion.Status.Completeness, completion.Status.ReasonCode = domain.CompletenessPartial, "source.inventory_partial"
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal(err)
	}
	page, err := fixture.store.ListResources(fixture.ctx, domain.InventoryQuery{Source: source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 100}})
	if err != nil || len(page.Resources) != 0 {
		t.Fatal("new authority publication exposed omitted old kind")
	}
	var audits int
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM audit_entries WHERE action='source.profile_update'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatal("policy change audit missing or repeated unchanged policy")
	}
}
