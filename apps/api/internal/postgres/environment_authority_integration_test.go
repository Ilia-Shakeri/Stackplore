package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestEnvironmentNamesRequireCurrentPartitionAuthorityBeforePagination(t *testing.T) {
	fixture := newObservationFixture(t)
	source := fixture.token.Source
	query := domain.EnvironmentQuery{Source: source, Page: domain.PageRequest{Limit: 10}}
	legacy, err := fixture.store.ListEnvironments(fixture.ctx, query)
	if err != nil || len(legacy.Environments) != 1 || legacy.Environments[0].ID != fixture.batch.Environments[0].ID {
		t.Fatal("legacy non-partitioned environment disappeared")
	}
	makeEnvironment := func(id domain.EnvironmentID) domain.Environment {
		environment := fixture.batch.Environments[0]
		environment.ID, environment.Name = id, string(id)
		return environment
	}
	current := makeEnvironment("environment_z_current")
	unsupported := makeEnvironment("environment_z_unsupported")
	empty := makeEnvironment("environment_z_empty")
	partial := makeEnvironment("environment_z_partial")
	coverage := func(environment domain.Environment, availability domain.AvailabilityState, completeness domain.Completeness) domain.InventoryCoverage {
		status := fixture.completion.Status
		status.State, status.Completeness = availability, completeness
		if availability != domain.AvailabilityAvailable || completeness != domain.CompletenessComplete {
			status.ReasonCode = "source.partition_partial"
		}
		return domain.InventoryCoverage{Source: source, EnvironmentID: environment.ID, Kind: "demo.service", ScopeMode: domain.InventoryAllScopes, Status: status}
	}
	capture := func(step int, environments []domain.Environment, partitions []domain.InventoryCoverage) {
		t.Helper()
		token, err := fixture.store.BeginCapture(fixture.ctx, source.SourceID, 1, fixture.at.Add(time.Duration(step)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.SaveCapture(fixture.ctx, domain.CaptureBatch{Token: token, Environments: environments}); err != nil {
			t.Fatal(err)
		}
		completion := fixture.completion
		completion.Token, completion.Complete, completion.Coverage = token, false, partitions
		completion.Status.Completeness, completion.Status.ReasonCode = domain.CompletenessPartial, "source.partition_partial"
		if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
			t.Fatal(err)
		}
	}
	initialPorts := func() map[domain.CapabilityName]bool {
		t.Helper()
		ports, err := readObservation(fixture.ctx, fixture.store, func(ctx context.Context, reader *observationRead) (map[domain.CapabilityName]bool, error) {
			return reader.observationInitialPorts(ctx, source)
		})
		if err != nil {
			t.Fatal(err)
		}
		return ports
	}
	// The legacy resource rows are hidden by the first partition publication;
	// their environment name must follow the same authority boundary.
	capture(1, []domain.Environment{current}, []domain.InventoryCoverage{coverage(current, domain.AvailabilityAvailable, domain.CompletenessComplete)})
	resources, err := fixture.store.ListResources(fixture.ctx, domain.InventoryQuery{Source: source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 100}})
	if err != nil || len(resources.Resources) != 0 {
		t.Fatal("omitted legacy resources remained visible")
	}
	page, err := fixture.store.ListEnvironments(fixture.ctx, query)
	if err != nil || len(page.Environments) != 1 || page.Environments[0].ID != current.ID {
		t.Fatalf("omitted legacy environment name leaked: environments=%#v error=%v", page.Environments, err)
	}
	capture(2, []domain.Environment{current, unsupported, empty, partial}, []domain.InventoryCoverage{
		coverage(current, domain.AvailabilityPermissionDenied, domain.CompletenessComplete),
		coverage(unsupported, domain.AvailabilityUnsupported, domain.CompletenessComplete),
		coverage(empty, domain.AvailabilityAvailable, domain.CompletenessComplete),
		coverage(partial, domain.AvailabilityAvailable, domain.CompletenessPartial),
	})
	query.Page.Limit = 1
	first, err := fixture.store.ListEnvironments(fixture.ctx, query)
	if err != nil || len(first.Environments) != 1 || first.Environments[0].ID != empty.ID || !first.Page.HasMore || first.Page.NextCursor == "" {
		t.Fatalf("first environment page counted forbidden rows: %#v error=%v", first, err)
	}
	query.Page.Cursor = first.Page.NextCursor
	second, err := fixture.store.ListEnvironments(fixture.ctx, query)
	if err != nil || len(second.Environments) != 1 || second.Environments[0].ID != partial.ID || second.Page.HasMore || second.Page.NextCursor != "" {
		t.Fatalf("last environment page counted forbidden rows: %#v error=%v", second, err)
	}
	if !initialPorts()[domain.CapabilityInventoryList] {
		t.Fatal("an authorized empty environment lost its inventory port")
	}
	// No resource is required for an empty permitted partition, while cached
	// environment-only rows cannot invent initial inventory authority.
	capture(3, []domain.Environment{current}, []domain.InventoryCoverage{coverage(current, domain.AvailabilityPermissionDenied, domain.CompletenessComplete)})
	query.Page = domain.PageRequest{Limit: 1}
	page, err = fixture.store.ListEnvironments(fixture.ctx, query)
	if err != nil || len(page.Environments) != 0 || page.Page.HasMore || page.Page.NextCursor != "" || initialPorts()[domain.CapabilityInventoryList] {
		t.Fatal("denied or omitted environment rows restored inventory authority")
	}
	// A pending operator policy blocks even a currently permitted empty partition.
	capture(4, []domain.Environment{empty}, []domain.InventoryCoverage{coverage(empty, domain.AvailabilityAvailable, domain.CompletenessComplete)})
	if err := fixture.store.EnsureObservationPolicy(fixture.ctx, source, strings.Repeat("e", 64)); err != nil {
		t.Fatal(err)
	}
	page, err = fixture.store.ListEnvironments(fixture.ctx, query)
	if err != nil || len(page.Environments) != 0 || initialPorts()[domain.CapabilityInventoryList] {
		t.Fatal("pending policy left environment name authority")
	}
}
