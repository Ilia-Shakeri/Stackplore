package docker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type collectorFunc func(context.Context) (Snapshot, error)

func (collect collectorFunc) Snapshot(ctx context.Context) (Snapshot, error) { return collect(ctx) }

type metricFixtureCollector struct {
	snapshot Snapshot
	stats    []ContainerStats
}

type logFixtureCollector struct {
	snapshot Snapshot
	lines    []ContainerLogLine
}

type eventFixtureCollector struct {
	snapshot Snapshot
	batch    DockerEventBatch
}

type operationFixtureCollector struct {
	snapshot Snapshot
	restart  []string
}

func (collector *operationFixtureCollector) Snapshot(context.Context) (Snapshot, error) {
	return collector.snapshot, nil
}

func (collector *operationFixtureCollector) RestartContainer(_ context.Context, identifier, version, key string) (string, error) {
	collector.restart = []string{identifier, version, key}
	return "dv_after", nil
}

func (collector eventFixtureCollector) Snapshot(context.Context) (Snapshot, error) {
	return collector.snapshot, nil
}

func (collector eventFixtureCollector) EngineEvents(context.Context, time.Time, time.Time) (DockerEventBatch, error) {
	return collector.batch, nil
}

func (collector logFixtureCollector) Snapshot(context.Context) (Snapshot, error) {
	return collector.snapshot, nil
}

func (collector logFixtureCollector) ReadContainerLogs(context.Context, ContainerLogRequest) ([]ContainerLogLine, error) {
	return append([]ContainerLogLine(nil), collector.lines...), nil
}

func (collector metricFixtureCollector) Snapshot(context.Context) (Snapshot, error) {
	return collector.snapshot, nil
}

func (collector metricFixtureCollector) CurrentContainerStats(context.Context, []string) ([]ContainerStats, error) {
	return append([]ContainerStats(nil), collector.stats...), nil
}

func TestProviderNormalizesComposeInventoryHealthAndTopology(t *testing.T) {
	provider := fixtureProvider(t, nil)
	ref := domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: "source_docker"}
	query := domain.InventoryQuery{
		Source: ref, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending,
		Page: domain.PageRequest{Limit: 20},
	}
	page, err := provider.ListResources(context.Background(), query)
	if err != nil || len(page.Resources) != 7 {
		t.Fatalf("resources = %d, error = %v; want 7", len(page.Resources), err)
	}
	var container domain.ResourceRef
	for _, item := range page.Resources {
		if item.Resource.Ref.Identity.Kind == "docker.container" {
			container = item.Resource.Ref
		}
	}
	if container.ID == "" {
		t.Fatal("container resource missing")
	}
	healthQuery := domain.HealthQuery{Source: ref, Resources: []domain.ResourceRef{container}}
	health, err := provider.GetHealth(context.Background(), healthQuery)
	if err != nil || len(health.Snapshots) != 1 || health.Snapshots[0].State != domain.HealthUnhealthy {
		t.Fatalf("container health = %#v, error = %v", health, err)
	}
	topologyQuery := domain.TopologyQuery{Source: ref, Page: domain.PageRequest{Limit: 20}}
	topology, err := provider.GetRelationships(context.Background(), topologyQuery)
	if err != nil || len(topology.Relationships) != 5 {
		t.Fatalf("relationships = %d, error = %v; want 5", len(topology.Relationships), err)
	}
	if err := topology.ValidateFor(topologyQuery); err != nil {
		t.Fatalf("topology invalid: %v", err)
	}
	capabilities, err := provider.GetCapabilities(context.Background(), ref)
	if err != nil || capabilities.ValidateFor(ref) != nil {
		t.Fatalf("capabilities invalid: %v", err)
	}
	for index, capability := range capabilities.Capabilities {
		want := domain.AvailabilityUnsupported
		if index < 4 {
			want = domain.AvailabilityAvailable
		}
		if capability.Status.State != want {
			t.Fatalf("capability %s = %s, want %s", capability.Name, capability.Status.State, want)
		}
	}
}

func TestProviderBindsCursorAndMapsPermissionFailure(t *testing.T) {
	provider := fixtureProvider(t, nil)
	query := domain.InventoryQuery{
		Source:    domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: "source_docker"},
		SortField: domain.ResourceSortName, SortDirection: domain.SortAscending,
		Page: domain.PageRequest{Limit: 2},
	}
	first, err := provider.ListResources(context.Background(), query)
	if err != nil || !first.Page.HasMore {
		t.Fatalf("first page = %#v, error = %v", first.Page, err)
	}
	query.Page.Cursor = first.Page.NextCursor
	query.Filter.Search = "api"
	if _, err := provider.ListResources(context.Background(), query); err == nil {
		t.Fatal("cursor accepted for changed query")
	}

	denied := fixtureProvider(t, ErrPermissionDenied)
	query.Page.Cursor = ""
	query.Filter.Search = ""
	_, err = denied.ListResources(context.Background(), query)
	var providerError *domain.ProviderError
	if !errors.As(err, &providerError) || providerError.Code != domain.ProviderErrorPermissionDenied {
		t.Fatalf("denied error = %v", err)
	}
}

func TestProviderReturnsCurrentContainerMetrics(t *testing.T) {
	observedAt := time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC)
	collector := metricFixtureCollector{
		snapshot: Snapshot{
			ObservedAt: observedAt, EngineID: "engine-one", EngineName: "Local engine",
			Containers: []Container{{ID: "container-one", Name: "shop-api-1", State: "running"}},
		},
		stats: []ContainerStats{{
			ContainerID: "container-one", CPUUsageCores: 0.25, MemoryUsageBytes: 1024,
			RestartCount: 2, SampledAt: observedAt,
		}},
	}
	provider, err := New("source_docker", "Local Docker", collector)
	if err != nil {
		t.Fatal(err)
	}
	ref := domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: "source_docker"}
	resources, err := provider.ListResources(context.Background(), domain.InventoryQuery{
		Source: ref, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending,
		Page: domain.PageRequest{Limit: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	var container domain.ResourceRef
	for _, item := range resources.Resources {
		if item.Resource.Ref.Identity.Kind == "docker.container" {
			container = item.Resource.Ref
		}
	}
	query := domain.CurrentMetricQuery{
		Source: ref, Resources: []domain.ResourceRef{container},
		Names: []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes, domain.MetricRestartCount},
	}
	result, err := provider.GetCurrentMetrics(context.Background(), query)
	if err != nil || len(result.Sets) != 1 || len(result.Sets[0].Samples) != 3 || result.Sets[0].Samples[0].Value != 0.25 {
		t.Fatalf("GetCurrentMetrics() = %#v, error = %v", result, err)
	}
	capabilities, err := provider.GetCapabilities(context.Background(), ref)
	if err != nil || capabilities.Capabilities[6].Status.State != domain.AvailabilityAvailable {
		t.Fatalf("metric capability = %#v, error = %v", capabilities, err)
	}
}

func TestProviderReturnsRedactedContainerLogs(t *testing.T) {
	observedAt := time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC)
	collector := logFixtureCollector{
		snapshot: Snapshot{
			ObservedAt: observedAt, EngineID: "engine-one", EngineName: "Local engine",
			Containers: []Container{{ID: "container-one", Name: "shop-api-1", State: "running"}},
		},
		lines: []ContainerLogLine{{
			ContainerID: "container-one", Stream: domain.LogStreamStdout,
			Timestamp: observedAt.Add(-time.Minute), Message: "request token=sample-value done",
		}},
	}
	provider, err := New("source_docker", "Local Docker", collector)
	if err != nil {
		t.Fatal(err)
	}
	ref := domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: "source_docker"}
	resources, err := provider.ListResources(context.Background(), domain.InventoryQuery{
		Source: ref, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending,
		Page: domain.PageRequest{Limit: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	var container domain.ResourceRef
	for _, item := range resources.Resources {
		if item.Resource.Ref.Identity.Kind == "docker.container" {
			container = item.Resource.Ref
		}
	}
	query := domain.LogReadQuery{
		Source: ref, Targets: []domain.LogTarget{{Resource: container}},
		Range:     domain.TimeRange{Start: observedAt.Add(-time.Hour), End: observedAt.Add(time.Nanosecond)},
		LineLimit: 10, ByteLimit: 1024,
	}
	page, err := provider.ReadLogs(context.Background(), query)
	if err != nil || len(page.Records) != 1 || !page.Records[0].Redacted || strings.Contains(page.Records[0].Message, "sample-value") {
		t.Fatalf("ReadLogs() = %#v, error = %v", page, err)
	}
	capabilities, err := provider.GetCapabilities(context.Background(), ref)
	if err != nil || capabilities.Capabilities[4].Status.State != domain.AvailabilityAvailable {
		t.Fatalf("log capability = %#v, error = %v", capabilities, err)
	}
}

func TestSafeLogMessageKeepsRedactedLineBounded(t *testing.T) {
	value := strings.Repeat("x", domain.MaxLogLineBytes-len(" token=")) + " token="
	message, redacted, truncated := safeLogMessage(value, false)
	if !redacted || !truncated || len(message) > domain.MaxLogLineBytes || !utf8.ValidString(message) {
		t.Fatalf("safeLogMessage() = length %d, redacted %t, truncated %t", len(message), redacted, truncated)
	}
}

func TestProviderReturnsDockerEventHistory(t *testing.T) {
	observedAt := time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC)
	collector := eventFixtureCollector{
		snapshot: Snapshot{
			ObservedAt: observedAt, EngineID: "engine-one", EngineName: "Local engine",
			Containers: []Container{{ID: "container-one", Name: "shop-api-1", State: "running"}},
		},
		batch: DockerEventBatch{Events: []DockerEvent{{
			Type: "container", Action: "die", ActorID: "container-one", OccurredAt: observedAt.Add(-time.Minute),
		}}},
	}
	provider, err := New("source_docker", "Local Docker", collector)
	if err != nil {
		t.Fatal(err)
	}
	ref := domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: "source_docker"}
	resources, err := provider.ListResources(context.Background(), domain.InventoryQuery{
		Source: ref, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	var container domain.ResourceRef
	for _, item := range resources.Resources {
		if item.Resource.Ref.Identity.Kind == "docker.container" {
			container = item.Resource.Ref
		}
	}
	query := domain.EventQuery{
		Source: ref, Resources: []domain.ResourceRef{container},
		Range:      domain.TimeRange{Start: observedAt.Add(-time.Hour), End: observedAt.Add(time.Nanosecond)},
		Severities: []domain.Severity{domain.SeverityError}, Page: domain.PageRequest{Limit: 10},
	}
	page, err := provider.ListEvents(context.Background(), query)
	if err != nil || len(page.Events) != 1 || page.Events[0].ResourceID == nil || *page.Events[0].ResourceID != container.ID || page.Events[0].ReasonCode != "docker.event.die" {
		t.Fatalf("ListEvents() = %#v, error = %v", page, err)
	}
	capabilities, err := provider.GetCapabilities(context.Background(), ref)
	if err != nil || capabilities.Capabilities[8].Status.State != domain.AvailabilityAvailable {
		t.Fatalf("event capability = %#v, error = %v", capabilities, err)
	}
}

func TestDockerEventIdentityBindsSourceAndNormalizedResourceFacts(t *testing.T) {
	at := time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC)
	collector := &eventFixtureCollector{snapshot: Snapshot{ObservedAt: at, EngineID: "engine-one", EngineName: "Fixture engine",
		Containers: []Container{{ID: "container-one", Name: "fixture-api", State: "running"}}},
		batch: DockerEventBatch{Events: []DockerEvent{{Type: "container", Action: "die", ActorID: "container-one", OccurredAt: at.Add(-time.Minute)}}}}
	read := func(source domain.SourceID) domain.InfrastructureEvent {
		t.Helper()
		provider, err := New(source, "Fixture engine", collector)
		if err != nil {
			t.Fatal(err)
		}
		page, err := provider.ListEvents(context.Background(), domain.EventQuery{Source: domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: source},
			Range: domain.TimeRange{Start: at.Add(-time.Hour), End: at.Add(time.Minute)}, Page: domain.PageRequest{Limit: 10}})
		if err != nil || len(page.Events) != 1 {
			t.Fatal("fixture event failed")
		}
		return page.Events[0]
	}
	first := read("source_docker_one")
	other := read("source_docker_two")
	if first.ID == other.ID || first.ResourceID == nil {
		t.Fatal("event ID crossed source boundary")
	}
	collector.snapshot.ObservedAt = at.Add(time.Second)
	later := read("source_docker_one")
	if later.ID != first.ID || later.ObservedAt == first.ObservedAt {
		t.Fatal("poll receipt changed immutable event ID")
	}
	collector.snapshot.Containers = nil
	unscoped := read("source_docker_one")
	if unscoped.ResourceID != nil || unscoped.ID == first.ID {
		t.Fatal("changed resource fact kept old immutable ID")
	}
}

func TestProviderPreviewsAndRestartsOnlyExactContainerVersion(t *testing.T) {
	collector := &operationFixtureCollector{snapshot: fixtureSnapshotForOperation()}
	provider, err := New("source_docker", "Local Docker", collector)
	if err != nil {
		t.Fatal(err)
	}
	page, err := provider.ListResources(context.Background(), domain.InventoryQuery{
		Source:    domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: "source_docker"},
		SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	var container domain.Resource
	for _, item := range page.Resources {
		if item.Resource.Ref.Identity.Kind == "docker.container" {
			container = item.Resource
		}
	}
	request := domain.OperationRequest{
		Source:   domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: "source_docker"},
		Resource: container.Ref, ExpectedVersion: container.Version, Action: domain.OperationDockerRestart,
	}
	if impact, err := provider.PreviewOperation(context.Background(), request); err != nil || impact.DisplayName != "shop-api-1" {
		t.Fatalf("PreviewOperation() = (%#v, %v)", impact, err)
	}
	result, err := provider.ExecuteOperation(context.Background(), domain.OperationExecution{Request: request, IdempotencyKey: "request_one"})
	if err != nil || result.CurrentVersion != "dv_after" || len(collector.restart) != 3 || collector.restart[0] != "container-one" {
		t.Fatalf("ExecuteOperation() = (%#v, %v), call = %#v", result, err, collector.restart)
	}
	request.ExpectedVersion = "dv_stale"
	if _, err := provider.PreviewOperation(context.Background(), request); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale preview error = %v", err)
	}
}

func fixtureSnapshotForOperation() Snapshot {
	return Snapshot{
		ObservedAt: time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC),
		EngineID:   "engine-one", EngineName: "Local engine",
		Containers: []Container{{ID: "container-one", Name: "shop-api-1", Version: "dv_before", State: "running"}},
	}
}

func TestCaptureInventoryCollectsExactlyOneSnapshot(t *testing.T) {
	calls := 0
	provider, err := New("source_docker", "Fixture engine", collectorFunc(func(context.Context) (Snapshot, error) { calls++; return fixtureSnapshotForOperation(), nil }))
	if err != nil {
		t.Fatal(err)
	}
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil || calls != 1 || capture.ValidateFor(domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: "source_docker"}) != nil || len(capture.Resources) != 2 || len(capture.Health) != 2 || !capture.Complete {
		t.Fatal("inventory capture repolled or failed normalized boundary")
	}
}

func fixtureProvider(t *testing.T, failure error) *Provider {
	t.Helper()
	observedAt := time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC)
	collector := collectorFunc(func(context.Context) (Snapshot, error) {
		if failure != nil {
			return Snapshot{}, failure
		}
		return Snapshot{
			ObservedAt: observedAt, EngineID: "engine-one", EngineName: "Local engine",
			Images:   []Image{{ID: "sha256:image-one", Name: "shop/api:1.0"}},
			Networks: []Network{{ID: "network-one", Name: "shop_default"}},
			Volumes:  []Volume{{Name: "shop_data"}},
			Containers: []Container{{
				ID: "container-one", Name: "shop-api-1", Version: "dv_before", ImageID: "sha256:image-one", State: "running",
				Health: "unhealthy", ComposeProject: "shop", ComposeService: "api",
				NetworkIDs: []string{"network-one"}, VolumeNames: []string{"shop_data"},
			}},
		}, nil
	})
	provider, err := New("source_docker", "Local Docker", collector)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return provider
}
