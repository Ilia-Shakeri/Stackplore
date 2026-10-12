package demo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestFixtureIsDeterministicAndValid(t *testing.T) {
	first, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	second, err := New()
	if err != nil {
		t.Fatalf("second New() error = %v", err)
	}
	query := inventoryQuery(20)
	firstPage, err := first.ListResources(context.Background(), query)
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	secondPage, err := second.ListResources(context.Background(), query)
	if err != nil {
		t.Fatalf("second ListResources() error = %v", err)
	}
	if len(firstPage.Resources) != 6 || len(secondPage.Resources) != 6 {
		t.Fatalf("resource counts = %d, %d", len(firstPage.Resources), len(secondPage.Resources))
	}
	for index := range firstPage.Resources {
		if firstPage.Resources[index].Resource.Ref.ID != secondPage.Resources[index].Resource.Ref.ID ||
			!firstPage.Resources[index].Resource.Synthetic {
			t.Fatalf("resource %d is not stable synthetic data", index)
		}
	}
	if err := firstPage.ValidateFor(query); err != nil {
		t.Fatalf("resource page invalid: %v", err)
	}
}

func TestResourcePaginationBindsCursorToQuery(t *testing.T) {
	provider := newProvider(t)
	query := inventoryQuery(2)
	first, err := provider.ListResources(context.Background(), query)
	if err != nil || !first.Page.HasMore || first.Page.NextCursor == "" {
		t.Fatalf("first page = %#v, %v", first.Page, err)
	}
	query.Page.Cursor = first.Page.NextCursor
	second, err := provider.ListResources(context.Background(), query)
	if err != nil || len(second.Resources) != 2 || second.Resources[0].Resource.Ref.ID == first.Resources[0].Resource.Ref.ID {
		t.Fatalf("second page = %#v, %v", second, err)
	}
	query.Filter.Search = "gateway"
	if _, err := provider.ListResources(context.Background(), query); err == nil {
		t.Fatal("cursor was accepted for a changed query")
	}
}

func TestResourceFilterDetailHealthAndCapabilities(t *testing.T) {
	provider := newProvider(t)
	query := inventoryQuery(10)
	query.Filter.Search = "checkout"
	page, err := provider.ListResources(context.Background(), query)
	if err != nil || len(page.Resources) != 1 {
		t.Fatalf("filtered page = %#v, %v", page, err)
	}
	resource := page.Resources[0]
	detail, err := provider.GetResource(context.Background(), domain.ResourceDetailQuery{
		Source: query.Source, ResourceID: resource.Resource.Ref.ID,
	})
	if err != nil || detail.Resource.Ref.ID != resource.Resource.Ref.ID {
		t.Fatalf("detail = %#v, %v", detail, err)
	}
	healthQuery := domain.HealthQuery{Source: query.Source, Resources: []domain.ResourceRef{resource.Resource.Ref}}
	health, err := provider.GetHealth(context.Background(), healthQuery)
	if err != nil || len(health.Snapshots) != 1 || health.Snapshots[0].State != domain.HealthUnhealthy {
		t.Fatalf("health = %#v, %v", health, err)
	}
	if err := health.ValidateFor(healthQuery); err != nil {
		t.Fatalf("health result invalid: %v", err)
	}
	capabilities, err := provider.GetCapabilities(context.Background(), query.Source)
	if err != nil || capabilities.ValidateFor(query.Source) != nil {
		t.Fatalf("capabilities = %#v, %v", capabilities, err)
	}
	for _, capability := range capabilities.Capabilities {
		want := domain.AvailabilityUnsupported
		switch capability.Name {
		case domain.CapabilityInventoryList, domain.CapabilityInventoryDetail, domain.CapabilityHealth,
			domain.CapabilityTopology, domain.CapabilityLogRead, domain.CapabilityMetricCurrent,
			domain.CapabilityMetricHistory, domain.CapabilityEventHistory:
			want = domain.AvailabilityAvailable
		}
		if capability.Status.State != want {
			t.Fatalf("capability %s state = %s, want %s", capability.Name, capability.Status.State, want)
		}
	}
}

func TestProviderServesDeterministicObservabilityPages(t *testing.T) {
	provider := newProvider(t)
	inventory := inventoryQuery(10)
	page, err := provider.ListResources(context.Background(), inventory)
	if err != nil {
		t.Fatal(err)
	}
	refs := make([]domain.ResourceRef, len(page.Resources))
	for index := range page.Resources {
		refs[index] = page.Resources[index].Resource.Ref
	}

	topologyQuery := domain.TopologyQuery{Source: inventory.Source, Page: domain.PageRequest{Limit: 2}}
	topology, err := provider.GetRelationships(context.Background(), topologyQuery)
	if err != nil || len(topology.Relationships) != 2 || !topology.Page.HasMore {
		t.Fatalf("topology = %#v, %v", topology, err)
	}
	if err := topology.ValidateFor(topologyQuery); err != nil {
		t.Fatalf("topology invalid: %v", err)
	}
	topologyQuery.Page.Cursor = topology.Page.NextCursor
	if next, err := provider.GetRelationships(context.Background(), topologyQuery); err != nil || len(next.Relationships) == 0 {
		t.Fatalf("next topology = %#v, %v", next, err)
	}

	logQuery := domain.LogReadQuery{
		Source: inventory.Source, Targets: []domain.LogTarget{{Resource: refs[0]}, {Resource: refs[1]}},
		Range:     domain.TimeRange{Start: observedAt.Add(-time.Hour), End: observedAt.Add(time.Second)},
		LineLimit: 1, ByteLimit: 1024,
	}
	logs, err := provider.ReadLogs(context.Background(), logQuery)
	if err != nil || len(logs.Records) != 1 || !logs.Page.HasMore {
		t.Fatalf("logs = %#v, %v", logs, err)
	}
	if err := logs.ValidateFor(logQuery); err != nil {
		t.Fatalf("logs invalid: %v", err)
	}

	metricQuery := domain.CurrentMetricQuery{
		Source: inventory.Source, Resources: refs[:2],
		Names: []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes},
	}
	metrics, err := provider.GetCurrentMetrics(context.Background(), metricQuery)
	if err != nil || len(metrics.Sets) != 2 || len(metrics.Sets[0].Samples) != 2 {
		t.Fatalf("metrics = %#v, %v", metrics, err)
	}
	if err := metrics.ValidateFor(metricQuery); err != nil {
		t.Fatalf("metrics invalid: %v", err)
	}
	historyQuery := domain.MetricHistoryQuery{
		Source: inventory.Source, Resources: refs[:2], Names: metricQuery.Names,
		Range:      domain.TimeRange{Start: observedAt.Add(-time.Hour), End: observedAt.Add(time.Second)},
		PointLimit: 10,
	}
	history, err := provider.GetMetricHistory(context.Background(), historyQuery)
	if err != nil || len(history.Samples) != 10 || history.Status.Completeness != domain.CompletenessPartial {
		t.Fatalf("metric history = %#v, %v", history, err)
	}
	if err := history.ValidateFor(historyQuery); err != nil {
		t.Fatalf("metric history invalid: %v", err)
	}

	eventQuery := domain.EventQuery{
		Source: inventory.Source, Range: domain.TimeRange{Start: observedAt.Add(-time.Hour), End: observedAt.Add(time.Second)},
		Severities: []domain.Severity{domain.SeverityError, domain.SeverityWarning},
		Page:       domain.PageRequest{Limit: 10},
	}
	events, err := provider.ListEvents(context.Background(), eventQuery)
	if err != nil || len(events.Events) != 2 {
		t.Fatalf("events = %#v, %v", events, err)
	}
	if err := events.ValidateFor(eventQuery); err != nil {
		t.Fatalf("events invalid: %v", err)
	}
	alerts := provider.Alerts()
	if len(alerts) != 1 || alerts[0].State != domain.AlertFiring || !alerts[0].Synthetic {
		t.Fatalf("alerts = %#v", alerts)
	}
}

func TestProviderRejectsWrongSourceAndCanceledContext(t *testing.T) {
	provider := newProvider(t)
	query := inventoryQuery(10)
	query.Source.SourceID = "source_other"
	_, err := provider.ListResources(context.Background(), query)
	var providerError *domain.ProviderError
	if !errors.As(err, &providerError) || providerError.Code != domain.ProviderErrorInvalidRequest {
		t.Fatalf("wrong source error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	query = inventoryQuery(10)
	if _, err := provider.ListResources(ctx, query); err == nil {
		t.Fatal("canceled context was accepted")
	}
}

func newProvider(t *testing.T) *Provider {
	t.Helper()
	provider, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return provider
}

func inventoryQuery(limit int) domain.InventoryQuery {
	return domain.InventoryQuery{
		Source:    domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: SourceID},
		SortField: domain.ResourceSortName, SortDirection: domain.SortAscending,
		Page: domain.PageRequest{Limit: limit},
	}
}
