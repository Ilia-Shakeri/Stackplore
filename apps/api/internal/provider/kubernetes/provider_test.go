package kubernetes

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

type eventFixtureCollector struct {
	snapshot Snapshot
	events   []ClusterEvent
}

type metricFixtureCollector struct {
	snapshot Snapshot
	metrics  []PodMetric
}

type logFixtureCollector struct {
	snapshot  Snapshot
	timestamp time.Time
}

type operationFixtureCollector struct {
	snapshot Snapshot
	last     WorkloadOperation
	scale    bool
}

func (collector *operationFixtureCollector) Snapshot(context.Context) (Snapshot, error) {
	return collector.snapshot, nil
}

func (collector *operationFixtureCollector) RestartWorkload(_ context.Context, operation WorkloadOperation) (string, error) {
	collector.last = operation
	return "18", nil
}

func (collector *operationFixtureCollector) ScaleWorkload(_ context.Context, operation WorkloadOperation) (string, error) {
	collector.last, collector.scale = operation, true
	return "19", nil
}

func (collector logFixtureCollector) Snapshot(context.Context) (Snapshot, error) {
	return collector.snapshot, nil
}

func (collector logFixtureCollector) ReadPodLogs(_ context.Context, request PodLogRequest) ([]PodLogLine, error) {
	if len(request.Targets) == 0 {
		return nil, nil
	}
	return []PodLogLine{{
		TargetKey: request.Targets[0].Key, Timestamp: collector.timestamp,
		Message: "request token=sample-value done",
	}}, nil
}

func (collector metricFixtureCollector) Snapshot(context.Context) (Snapshot, error) {
	return collector.snapshot, nil
}

func (collector metricFixtureCollector) CurrentPodMetrics(context.Context) ([]PodMetric, error) {
	return append([]PodMetric(nil), collector.metrics...), nil
}

func (collector eventFixtureCollector) Snapshot(context.Context) (Snapshot, error) {
	return collector.snapshot, nil
}

func (collector eventFixtureCollector) ClusterEvents(context.Context) ([]ClusterEvent, error) {
	return append([]ClusterEvent(nil), collector.events...), nil
}

func TestProviderNormalizesClusterInventoryHealthAndTopology(t *testing.T) {
	provider := fixtureProvider(t, nil)
	ref := domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"}
	query := domain.InventoryQuery{
		Source: ref, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending,
		Page: domain.PageRequest{Limit: 30},
	}
	page, err := provider.ListResources(context.Background(), query)
	if err != nil || len(page.Resources) != 14 {
		t.Fatalf("resources = %d, error = %v; want 14", len(page.Resources), err)
	}
	var pod domain.ResourceRef
	var deployment domain.ResourceRef
	for _, item := range page.Resources {
		if item.Resource.Ref.Identity.Kind == "kubernetes.pod" {
			pod = item.Resource.Ref
		}
		if item.Resource.Ref.Identity.Kind == "kubernetes.deployment" {
			deployment = item.Resource.Ref
		}
		if item.Resource.Ref.Identity.Kind == "kubernetes.secret" && item.Resource.DisplayName != "shop-token" {
			t.Fatalf("secret exposed more than its safe name: %#v", item.Resource)
		}
	}
	if pod.ID == "" {
		t.Fatal("pod resource missing")
	}
	replicas, err := provider.GetWorkloadReplicas(context.Background(), domain.ResourceDetailQuery{Source: ref, ResourceID: deployment.ID})
	if err != nil || replicas.Desired != 1 || replicas.Ready != 1 || replicas.Resource.ID != deployment.ID {
		t.Fatalf("workload replicas = %#v, error = %v", replicas, err)
	}
	healthQuery := domain.HealthQuery{Source: ref, Resources: []domain.ResourceRef{pod}}
	health, err := provider.GetHealth(context.Background(), healthQuery)
	if err != nil || len(health.Snapshots) != 1 || health.Snapshots[0].State != domain.HealthHealthy {
		t.Fatalf("pod health = %#v, error = %v", health, err)
	}
	topologyQuery := domain.TopologyQuery{Source: ref, Page: domain.PageRequest{Limit: 30}}
	topology, err := provider.GetRelationships(context.Background(), topologyQuery)
	if err != nil || len(topology.Relationships) != 15 {
		t.Fatalf("relationships = %d, error = %v; want 15", len(topology.Relationships), err)
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

func TestProviderRejectsUnknownWorkloadReplicaEvidence(t *testing.T) {
	snapshot := fixtureSnapshot()
	snapshot.Workloads[0].ReadyKnown = false
	provider := newFixtureProvider(t, collectorFunc(func(context.Context) (Snapshot, error) { return snapshot, nil }))
	page, err := provider.ListResources(context.Background(), inventoryQuery())
	if err != nil {
		t.Fatal(err)
	}
	var deployment domain.ResourceRef
	for _, item := range page.Resources {
		if item.Resource.Ref.Identity.Kind == "kubernetes.deployment" {
			deployment = item.Resource.Ref
		}
	}
	_, err = provider.GetWorkloadReplicas(context.Background(), domain.ResourceDetailQuery{
		Source: domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"}, ResourceID: deployment.ID,
	})
	var providerFailure *domain.ProviderError
	if !errors.As(err, &providerFailure) || providerFailure.Code != domain.ProviderErrorUnavailable {
		t.Fatalf("unknown replica error = %v", err)
	}
}

func TestProviderBindsCursorAndMapsPermissionFailure(t *testing.T) {
	provider := fixtureProvider(t, nil)
	query := domain.InventoryQuery{
		Source:    domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"},
		SortField: domain.ResourceSortName, SortDirection: domain.SortAscending,
		Page: domain.PageRequest{Limit: 2},
	}
	first, err := provider.ListResources(context.Background(), query)
	if err != nil || !first.Page.HasMore {
		t.Fatalf("first page = %#v, error = %v", first.Page, err)
	}
	query.Page.Cursor = first.Page.NextCursor
	query.Filter.Search = "shop"
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

func TestProviderRejectsUnknownWorkloadKindAndDuplicateResource(t *testing.T) {
	base := fixtureSnapshot()
	base.Workloads[0].Kind = "Widget"
	provider := newFixtureProvider(t, collectorFunc(func(context.Context) (Snapshot, error) { return base, nil }))
	query := inventoryQuery()
	if _, err := provider.ListResources(context.Background(), query); err == nil {
		t.Fatal("unknown workload kind accepted")
	}

	base = fixtureSnapshot()
	base.ConfigMaps = append(base.ConfigMaps, base.ConfigMaps[0])
	provider = newFixtureProvider(t, collectorFunc(func(context.Context) (Snapshot, error) { return base, nil }))
	if _, err := provider.ListResources(context.Background(), query); err == nil {
		t.Fatal("duplicate resource accepted")
	}
}

func TestProviderKeepsSameNamedContainersInDifferentPodsDistinct(t *testing.T) {
	snapshot := fixtureSnapshot()
	second := snapshot.Pods[0]
	second.UID, second.Name = "pod-two", "api-abc-2"
	snapshot.Pods = append(snapshot.Pods, second)
	provider := newFixtureProvider(t, collectorFunc(func(context.Context) (Snapshot, error) { return snapshot, nil }))
	query := inventoryQuery()
	page, err := provider.ListResources(context.Background(), query)
	if err != nil {
		t.Fatalf("inventory rejected same-named containers: %v", err)
	}
	containers := make(map[domain.ResourceID]domain.ResourceRef)
	for _, item := range page.Resources {
		if item.Resource.Ref.Identity.Kind == "kubernetes.container" {
			containers[item.Resource.Ref.ID] = item.Resource.Ref
		}
	}
	if len(containers) != 4 {
		t.Fatalf("distinct containers = %d; want 4", len(containers))
	}
	topology, err := provider.GetRelationships(context.Background(), domain.TopologyQuery{Source: query.Source, Page: query.Page})
	if err != nil {
		t.Fatalf("topology rejected same-named containers: %v", err)
	}
	owned := make(map[domain.ResourceID]bool)
	for _, edge := range topology.Relationships {
		if _, exists := containers[edge.To]; exists && edge.Kind == domain.RelationshipOwns {
			owned[edge.To] = true
		}
	}
	if len(owned) != len(containers) {
		t.Fatalf("container owner edges = %d; want %d", len(owned), len(containers))
	}
}

func TestProviderReturnsBoundedKubernetesEvents(t *testing.T) {
	snapshot := fixtureSnapshot()
	collector := eventFixtureCollector{
		snapshot: snapshot,
		events: []ClusterEvent{{
			UID: "event-one", InvolvedUID: "pod-one", Reason: "BackOff", Type: "Warning",
			Message: "Container restart backoff", OccurredAt: snapshot.ObservedAt.Add(-30 * time.Minute),
		}},
	}
	provider := newFixtureProvider(t, collector)
	ref := domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"}
	resources, err := provider.ListResources(context.Background(), inventoryQuery())
	if err != nil {
		t.Fatal(err)
	}
	var pod domain.ResourceRef
	for _, item := range resources.Resources {
		if item.Resource.Ref.Identity.Kind == "kubernetes.pod" {
			pod = item.Resource.Ref
		}
	}
	query := domain.EventQuery{
		Source: ref, Resources: []domain.ResourceRef{pod},
		Range: domain.TimeRange{Start: snapshot.ObservedAt.Add(-time.Hour), End: snapshot.ObservedAt.Add(time.Nanosecond)},
		Page:  domain.PageRequest{Limit: 10},
	}
	page, err := provider.ListEvents(context.Background(), query)
	if err != nil || len(page.Events) != 1 || page.Events[0].Severity != domain.SeverityWarning || page.Events[0].ResourceID == nil || *page.Events[0].ResourceID != pod.ID {
		t.Fatalf("ListEvents() = %#v, error = %v", page, err)
	}
	capabilities, err := provider.GetCapabilities(context.Background(), ref)
	if err != nil || capabilities.Capabilities[8].Status.State != domain.AvailabilityAvailable {
		t.Fatalf("event capability = %#v, error = %v", capabilities, err)
	}
}

func TestProviderReturnsCurrentPodMetrics(t *testing.T) {
	snapshot := fixtureSnapshot()
	snapshot.Pods[0].ContainerRestarts = map[string]uint64{"api": 2, "proxy": 1}
	collector := metricFixtureCollector{
		snapshot: snapshot,
		metrics: []PodMetric{{
			Namespace: "shop", PodName: "api-abc-1", SampledAt: snapshot.ObservedAt.Add(-time.Minute), Window: 30 * time.Second,
			Containers: []ContainerMetric{{Name: "api", CPUCores: 0.25, MemoryBytes: 128 << 20}, {Name: "proxy", CPUCores: 0.05, MemoryBytes: 32 << 20}},
		}},
	}
	provider := newFixtureProvider(t, collector)
	ref := domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"}
	resources, err := provider.ListResources(context.Background(), inventoryQuery())
	if err != nil {
		t.Fatal(err)
	}
	var pod domain.ResourceRef
	for _, item := range resources.Resources {
		if item.Resource.Ref.Identity.Kind == "kubernetes.pod" {
			pod = item.Resource.Ref
		}
	}
	query := domain.CurrentMetricQuery{
		Source: ref, Resources: []domain.ResourceRef{pod},
		Names: []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes, domain.MetricRestartCount},
	}
	result, err := provider.GetCurrentMetrics(context.Background(), query)
	if err != nil || len(result.Sets) != 1 || len(result.Sets[0].Samples) != 3 || result.Sets[0].Samples[0].Value != 0.3 || result.Sets[0].Samples[2].Value != 3 {
		t.Fatalf("GetCurrentMetrics() = %#v, error = %v", result, err)
	}
	capabilities, err := provider.GetCapabilities(context.Background(), ref)
	if err != nil || capabilities.Capabilities[6].Status.State != domain.AvailabilityAvailable {
		t.Fatalf("metric capability = %#v, error = %v", capabilities, err)
	}
}

func TestProviderReturnsRedactedPodLogs(t *testing.T) {
	snapshot := fixtureSnapshot()
	collector := logFixtureCollector{snapshot: snapshot, timestamp: snapshot.ObservedAt.Add(-time.Minute)}
	provider := newFixtureProvider(t, collector)
	ref := domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"}
	resources, err := provider.ListResources(context.Background(), inventoryQuery())
	if err != nil {
		t.Fatal(err)
	}
	var pod domain.ResourceRef
	for _, item := range resources.Resources {
		if item.Resource.Ref.Identity.Kind == "kubernetes.pod" {
			pod = item.Resource.Ref
		}
	}
	query := domain.LogReadQuery{
		Source: ref, Targets: []domain.LogTarget{{Resource: pod}},
		Range:     domain.TimeRange{Start: snapshot.ObservedAt.Add(-time.Hour), End: snapshot.ObservedAt.Add(time.Nanosecond)},
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

func TestSafeKubernetesLogMessageKeepsRedactedLineBounded(t *testing.T) {
	value := strings.Repeat("ی", domain.MaxLogLineBytes/2-len(" token=")) + " token="
	message, redacted, truncated := safeKubernetesLogMessage(value, false)
	if !redacted || !truncated || len(message) > domain.MaxLogLineBytes || !utf8.ValidString(message) {
		t.Fatalf("safeKubernetesLogMessage() = length %d, redacted %t, truncated %t", len(message), redacted, truncated)
	}
}

func TestProviderPreviewsAndScalesExactSupportedWorkload(t *testing.T) {
	collector := &operationFixtureCollector{snapshot: fixtureSnapshot()}
	provider := newFixtureProvider(t, collector)
	page, err := provider.ListResources(context.Background(), inventoryQuery())
	if err != nil {
		t.Fatal(err)
	}
	var deployment domain.Resource
	for _, item := range page.Resources {
		if item.Resource.Ref.Identity.Kind == "kubernetes.deployment" {
			deployment = item.Resource
		}
	}
	replicas := 3
	request := domain.OperationRequest{
		Source: inventoryQuery().Source, Resource: deployment.Ref, ExpectedVersion: deployment.Version,
		Action: domain.OperationKubernetesScale, Parameters: domain.OperationParameters{Replicas: &replicas},
	}
	impact, err := provider.PreviewOperation(context.Background(), request)
	if err != nil || impact.CurrentReplicas == nil || *impact.CurrentReplicas != 1 {
		t.Fatalf("PreviewOperation() = (%#v, %v)", impact, err)
	}
	result, err := provider.ExecuteOperation(context.Background(), domain.OperationExecution{Request: request, IdempotencyKey: "request_scale_one"})
	if err != nil || !collector.scale || collector.last.Name != "api" || collector.last.ResourceVersion != "17" || result.CurrentVersion != kubernetesOperationVersion("19") {
		t.Fatalf("ExecuteOperation() = (%#v, %v), call = %#v", result, err, collector.last)
	}
	request.Action = domain.OperationDockerRestart
	request.Parameters.Replicas = nil
	if _, err := provider.PreviewOperation(context.Background(), request); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported preview error = %v", err)
	}
}

func fixtureProvider(t *testing.T, failure error) *Provider {
	t.Helper()
	collector := collectorFunc(func(context.Context) (Snapshot, error) {
		if failure != nil {
			return Snapshot{}, failure
		}
		return fixtureSnapshot(), nil
	})
	return newFixtureProvider(t, collector)
}

func newFixtureProvider(t *testing.T, collector Collector) *Provider {
	t.Helper()
	provider, err := New("source_cluster", "Main cluster", collector)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return provider
}

func inventoryQuery() domain.InventoryQuery {
	return domain.InventoryQuery{
		Source:    domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"},
		SortField: domain.ResourceSortName, SortDirection: domain.SortAscending,
		Page: domain.PageRequest{Limit: 30},
	}
}

func fixtureSnapshot() Snapshot {
	return Snapshot{
		ObservedAt: time.Date(2026, time.August, 16, 9, 0, 0, 0, time.UTC),
		ClusterID:  "cluster-one", ClusterName: "Main cluster", ContextName: "prod",
		Namespaces: []Namespace{{Name: "shop", Phase: "Active"}},
		Nodes:      []Node{{UID: "node-one", Name: "worker-1", Ready: true, ReadyKnown: true}},
		Workloads: []Workload{
			{UID: "deployment-one", Version: "17", Kind: "Deployment", Namespace: "shop", Name: "api", Desired: 1, Ready: 1, ReadyKnown: true},
			{UID: "replicaset-one", Kind: "ReplicaSet", Namespace: "shop", Name: "api-abc", Desired: 1, Ready: 1, ReadyKnown: true, OwnerKind: "Deployment", OwnerUID: "deployment-one"},
		},
		Pods: []Pod{{
			UID: "pod-one", Namespace: "shop", Name: "api-abc-1", Phase: "Running", Ready: true, ReadyKnown: true,
			OwnerKind: "ReplicaSet", OwnerUID: "replicaset-one", Labels: map[string]string{"app": "api"},
			Containers: []string{"api", "proxy"}, ClaimNames: []string{"shop-data"},
			ConfigMapNames: []string{"shop-config"}, SecretNames: []string{"shop-token"},
		}},
		Services:       []Service{{UID: "service-one", Namespace: "shop", Name: "api", Selector: map[string]string{"app": "api"}}},
		Ingresses:      []Ingress{{UID: "ingress-one", Namespace: "shop", Name: "shop", ServiceNames: []string{"api"}}},
		EndpointSlices: []EndpointSlice{{UID: "endpoint-one", Namespace: "shop", Name: "api-abc", ServiceName: "api", PodUIDs: []string{"pod-one"}}},
		Claims:         []PersistentVolumeClaim{{UID: "claim-one", Namespace: "shop", Name: "shop-data", Phase: "Bound"}},
		ConfigMaps:     []ObjectRef{{UID: "config-one", Namespace: "shop", Name: "shop-config"}},
		Secrets:        []ObjectRef{{UID: "secret-one", Namespace: "shop", Name: "shop-token"}},
	}
}

func TestCaptureInventoryCollectsOneSnapshotAndReplicaEvidence(t *testing.T) {
	calls := 0
	provider, err := New("source_cluster", "Fixture cluster", collectorFunc(func(context.Context) (Snapshot, error) { calls++; return fixtureSnapshot(), nil }))
	if err != nil {
		t.Fatal(err)
	}
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil || calls != 1 || capture.ValidateFor(domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"}) != nil || len(capture.Resources) != 14 || len(capture.Health) != 14 || len(capture.WorkloadReplicas) != 2 || !capture.Complete {
		t.Fatal("inventory capture repolled or lost workload evidence")
	}
}

func TestEventIdentityChangesOnlyWithEventFacts(t *testing.T) {
	snapshot := fixtureSnapshot()
	collector := &eventFixtureCollector{snapshot: snapshot, events: []ClusterEvent{{UID: "event-one", InvolvedUID: "pod-one", Reason: "BackOff", Message: "request token=synth-private-9 stopped", Type: "Warning", OccurredAt: snapshot.ObservedAt.Add(-time.Minute), Occurrences: 1}}}
	provider, err := New("source_cluster", "Fixture cluster", collector)
	if err != nil {
		t.Fatal(err)
	}
	query := domain.EventQuery{Source: domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"}, Range: domain.TimeRange{Start: snapshot.ObservedAt.Add(-time.Hour), End: snapshot.ObservedAt.Add(time.Minute)}, Page: domain.PageRequest{Limit: 10}}
	first, err := provider.ListEvents(context.Background(), query)
	if err != nil || len(first.Events) != 1 || strings.Contains(first.Events[0].Summary, "synth-private-9") {
		t.Fatal("event facts leaked or fixture failed")
	}
	collector.snapshot.ObservedAt = collector.snapshot.ObservedAt.Add(time.Second)
	second, err := provider.ListEvents(context.Background(), query)
	if err != nil || second.Events[0].ID != first.Events[0].ID || second.Events[0].ObservedAt == first.Events[0].ObservedAt {
		t.Fatal("poll receipt changed immutable event ID or lost receipt time")
	}
	collector.events[0].Occurrences++
	third, err := provider.ListEvents(context.Background(), query)
	if err != nil || third.Events[0].ID == first.Events[0].ID {
		t.Fatal("native event revision kept old immutable ID")
	}
	other, err := New("source_other_cluster", "Other fixture cluster", collector)
	if err != nil {
		t.Fatal(err)
	}
	otherQuery := query
	otherQuery.Source.SourceID = "source_other_cluster"
	otherPage, err := other.ListEvents(context.Background(), otherQuery)
	if err != nil || len(otherPage.Events) != 1 || otherPage.Events[0].ID == third.Events[0].ID {
		t.Fatal("event ID crossed source boundary")
	}
	collector.snapshot.Pods = nil
	unscoped, err := provider.ListEvents(context.Background(), query)
	if err != nil || len(unscoped.Events) != 1 || unscoped.Events[0].ResourceID != nil || unscoped.Events[0].ID == third.Events[0].ID {
		t.Fatal("changed resource fact kept old immutable ID")
	}
}
