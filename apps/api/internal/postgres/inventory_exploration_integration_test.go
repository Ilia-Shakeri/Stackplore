package postgres

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestInventorySummaryCountsBeyondTransportAndHealthTargetLimits(t *testing.T) {
	fixture := newObservationFixture(t)
	token, err := fixture.store.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(time.Second))
	if err != nil {
		t.Fatal("begin larger inventory")
	}
	batch := fixture.batch
	batch.Token = token
	batch.Events = nil
	batch.Metrics = nil
	batch.Relationships = nil
	batch.WorkloadReplicas = nil
	if err := fixture.store.SaveCapture(fixture.ctx, batch); err != nil {
		t.Fatal("retain baseline inventory")
	}
	for start := 0; start < 220; start += 100 {
		batch = domain.CaptureBatch{Token: token}
		for index := start; index < min(start+100, 220); index++ {
			item := fixture.batch.Resources[0]
			identity := item.Resource.Ref.Identity
			identity, err := domain.NewResourceIdentity(identity.Provider, identity.SourceID, identity.EnvironmentID, identity.Kind, identity.Scope, domain.NativeResourceID(fmt.Sprintf("extra_%03d", index)))
			if err != nil {
				t.Fatal("build inventory identity")
			}
			ref, err := domain.NewResourceRef(identity)
			if err != nil {
				t.Fatal("build inventory reference")
			}
			item.Resource.Ref = ref
			item.Resource.DisplayName = fmt.Sprintf("extra_%03d", index)
			batch.Resources = append(batch.Resources, item)
		}
		if err := fixture.store.SaveCapture(fixture.ctx, batch); err != nil {
			t.Fatal("save bounded inventory batch")
		}
	}
	completion := fixture.completion
	completion.Token = token
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal("complete larger inventory")
	}
	summary, err := fixture.store.GetInventorySummary(fixture.ctx, token.Source)
	if err != nil || summary.Resources != 223 || summary.Healthy != 3 || summary.Unknown != 220 || summary.Status.Completeness != domain.CompletenessPartial {
		t.Fatalf("full inventory summary: %+v %v", summary, err)
	}
	query := domain.InventoryQuery{Source: token.Source, Filter: domain.ResourceFilter{Provider: domain.ProviderDemo, EnvironmentID: fixture.batch.Environments[0].ID, ScopeKind: domain.ScopeGlobal, Health: domain.HealthUnknown, Availability: domain.AvailabilityAvailable}, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 200}}
	page, err := fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 200 || !page.Page.HasMore {
		t.Fatalf("full-set filter before pagination: %v", err)
	}
	query.Page.Cursor = page.Page.NextCursor
	last, err := fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(last.Resources) != 20 || last.Page.HasMore {
		t.Fatalf("filtered continuation: %v", err)
	}
	for _, change := range []func(*domain.ResourceFilter){
		func(filter *domain.ResourceFilter) { filter.Provider = domain.ProviderDocker },
		func(filter *domain.ResourceFilter) { filter.EnvironmentID = "other_environment" },
		func(filter *domain.ResourceFilter) { filter.ScopeKind = domain.ScopeNamespace },
		func(filter *domain.ResourceFilter) { filter.ScopeName = "another" },
		func(filter *domain.ResourceFilter) { filter.Health = domain.HealthHealthy },
		func(filter *domain.ResourceFilter) { filter.Availability = domain.AvailabilityStale },
	} {
		altered := query
		change(&altered.Filter)
		if _, err := fixture.store.ListResources(fixture.ctx, altered); !errors.Is(err, domain.ErrRepositoryInvalidInput) {
			t.Fatal("cursor accepted changed filter")
		}
	}
}

func TestInventoryHealthFiltersUseUsableEvidenceAndSourceAvailability(t *testing.T) {
	fixture := newObservationFixture(t)
	query := domain.InventoryQuery{Source: fixture.token.Source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 10}}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE resource_health_current SET health_state='unknown',availability='permission_denied',reason_code='provider.permission_denied' WHERE resource_id=$1`, string(fixture.batch.Resources[0].Resource.Ref.ID)); err != nil {
		t.Fatalf("seed denied health: %v", err)
	}
	query.Filter.Health = domain.HealthUnknown
	page, err := fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 1 || page.Resources[0].Health != domain.HealthUnknown {
		t.Fatalf("denied health must stay unknown: %v", err)
	}
	summary, err := fixture.store.GetInventorySummary(fixture.ctx, query.Source)
	if err != nil || summary.Healthy != 2 || summary.Unknown != 1 || summary.Resources != 3 || summary.Status.Completeness != domain.CompletenessPartial {
		t.Fatalf("denied summary health: %+v %v", summary, err)
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE infrastructure_sources SET enabled=0 WHERE id=$1`, string(query.Source.SourceID)); err != nil {
		t.Fatal("disable source fixture")
	}
	query.Filter.Availability = domain.AvailabilityStale
	page, err = fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 1 || page.Status.State != domain.AvailabilityStale {
		t.Fatalf("effective stale filter: %v", err)
	}
	query.Filter.Availability = domain.AvailabilityAvailable
	page, err = fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 0 {
		t.Fatalf("disabled source claimed available resource: %v", err)
	}
}

func TestInventoryFiltersHonorCapabilityStaleAndDeniedHealth(t *testing.T) {
	fixture := newObservationFixture(t)
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE source_capabilities SET availability='unavailable',reason_code='provider.disconnected' WHERE source_id=$1 AND capability_name='inventory.list'`, string(fixture.token.Source.SourceID)); err != nil {
		t.Fatal("seed stale inventory capability")
	}
	query := domain.InventoryQuery{Source: fixture.token.Source, Filter: domain.ResourceFilter{Availability: domain.AvailabilityStale}, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 10}}
	page, err := fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 3 || page.Status.State != domain.AvailabilityStale {
		t.Fatalf("capability stale applied after SQL filter: %v", err)
	}
	query.Filter.Availability = domain.AvailabilityAvailable
	page, err = fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 0 {
		t.Fatalf("unavailable inventory claimed available rows: %v", err)
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE source_capabilities SET availability='permission_denied',reason_code='provider.permission_denied' WHERE source_id=$1 AND capability_name='health.read'`, string(query.Source.SourceID)); err != nil {
		t.Fatal("seed denied health capability")
	}
	query.Filter = domain.ResourceFilter{Health: domain.HealthUnknown}
	page, err = fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 3 {
		t.Fatalf("health capability denial leaked old healthy classes: %v", err)
	}
	summary, err := fixture.store.GetInventorySummary(fixture.ctx, query.Source)
	if err != nil || summary.Unknown != 3 || summary.Healthy != 0 || summary.Status.Completeness != domain.CompletenessPartial {
		t.Fatalf("health capability denial summary: %+v %v", summary, err)
	}
}
