package kubernetes

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProviderRetainsConfiguredMetricsWhenUsageAPIIsAbsent(t *testing.T) {
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "/metrics.k8s.io/") {
			return jsonResponse(http.StatusNotFound, `{"reason":"NotFound","code":404}`), nil
		}
		if request.URL.Path == "/api/v1/namespaces/shop/pods" {
			return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"pod-one","name":"api","namespace":"shop"},"spec":{"containers":[{"name":"api","resources":{"requests":{"cpu":"100m"},"limits":{"memory":"32Mi"}}}]},"status":{"containerStatuses":[{"name":"api","restartCount":3,"ready":true,"state":{"running":{}}}]}}]}`), nil
		}
		return emptyListTransport(request)
	})
	provider, _ := New("source_cluster", "Scoped", collector)
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var container, pod domain.ResourceRef
	for _, resource := range capture.Resources {
		switch resource.Resource.Ref.Identity.Kind {
		case "kubernetes.container":
			container = resource.Resource.Ref
		case "kubernetes.pod":
			pod = resource.Resource.Ref
		}
	}
	query := domain.CurrentMetricQuery{Source: capture.Source, Resources: []domain.ResourceRef{container}, Names: []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricCPURequestCores, domain.MetricMemoryLimitBytes, domain.MetricRestartCount}}
	result, err := provider.GetCurrentMetrics(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status.State != domain.AvailabilityAvailable || result.Status.Completeness != domain.CompletenessPartial || len(result.Sets) != 1 || len(result.Sets[0].Samples) != 3 {
		t.Fatal("known facts were lost with unavailable usage")
	}
	for _, sample := range result.Sets[0].Samples {
		if sample.Name == domain.MetricCPUUsageCores {
			t.Fatal("absent usage invented zero")
		}
		if sample.Name == domain.MetricCPURequestCores && sample.Value != 0.1 {
			t.Fatal("CPU request lost")
		}
		if sample.Interval != 0 {
			t.Fatal("configuration fact invented a sampling interval")
		}
	}
	query.Resources = []domain.ResourceRef{pod}
	query.Names = []domain.MetricName{domain.MetricCPURequestCores}
	result, err = provider.GetCurrentMetrics(context.Background(), query)
	if err != nil || len(result.Sets) != 0 || result.Status.State != domain.AvailabilityUnsupported {
		t.Fatal("container sum masqueraded as effective Pod scheduling request")
	}
}

func TestProviderSurgeReplicasAndFailedJobUseActualEvidence(t *testing.T) {
	snapshot := fixtureSnapshot()
	snapshot.Workloads[0].Desired = 1
	snapshot.Workloads[0].Ready = 2
	provider := newFixtureProvider(t, collectorFunc(func(context.Context) (Snapshot, error) { return snapshot, nil }))
	page, err := provider.ListResources(context.Background(), inventoryQuery())
	if err != nil {
		t.Fatal(err)
	}
	var workload domain.ResourceRef
	for _, resource := range page.Resources {
		if resource.Resource.Ref.Identity.Kind == "kubernetes.deployment" {
			workload = resource.Resource.Ref
		}
	}
	replicas, err := provider.GetWorkloadReplicas(context.Background(), domain.ResourceDetailQuery{Source: domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"}, ResourceID: workload.ID})
	if err != nil || replicas.Ready != 2 || replicas.Desired != 1 {
		t.Fatal("surge replica evidence disappeared")
	}
	if health, _ := workloadHealth(snapshot.Workloads[0]); health != domain.HealthHealthy {
		t.Fatal("surge made healthy workload unknown")
	}
	failed := Workload{Kind: "Job", Desired: 1, Ready: 0, ReadyKnown: true, Metadata: domain.KubernetesResourceMetadata{Conditions: []domain.KubernetesConditionMetadata{{Type: domain.KubernetesConditionFailed, Status: domain.KubernetesConditionTrue}}}}
	if health, _ := workloadHealth(failed); health != domain.HealthUnhealthy {
		t.Fatal("failed Job condition ignored")
	}
}

type paginationLogCollector struct {
	snapshot Snapshot
	last     PodLogRequest
	lines    []string
}

func (collector *paginationLogCollector) Snapshot(context.Context) (Snapshot, error) {
	return collector.snapshot, nil
}
func (collector *paginationLogCollector) ReadPodLogs(_ context.Context, request PodLogRequest) (LogObservation, error) {
	collector.last = request
	result := LogObservation{Status: observationStatus(collector.snapshot.ObservedAt, nil, false)}
	if len(request.Targets) == 0 {
		return result, nil
	}
	for index, message := range collector.lines {
		result.Lines = append(result.Lines, PodLogLine{TargetKey: request.Targets[0].Key, Timestamp: collector.snapshot.ObservedAt.Add(-time.Minute).Add(time.Duration(index) * time.Second), Message: message})
	}
	return result, nil
}

func TestProviderLogPaginationUsesFixedWindowAndContainerSelection(t *testing.T) {
	collector := &paginationLogCollector{snapshot: fixtureSnapshot(), lines: []string{"one", "two", "three"}}
	provider := newFixtureProvider(t, collector)
	resources, _ := provider.ListResources(context.Background(), inventoryQuery())
	var pod domain.ResourceRef
	for _, resource := range resources.Resources {
		if resource.Resource.Ref.Identity.Kind == "kubernetes.pod" {
			pod = resource.Resource.Ref
		}
	}
	query := domain.LogReadQuery{Source: domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"}, Targets: []domain.LogTarget{{Resource: pod, Container: "proxy"}}, Range: domain.TimeRange{Start: collector.snapshot.ObservedAt.Add(-time.Hour), End: collector.snapshot.ObservedAt.Add(time.Nanosecond)}, LineLimit: 1, ByteLimit: 1024}
	first, err := provider.ReadLogs(context.Background(), query)
	if err != nil || len(first.Records) != 1 || first.Records[0].Message != "one" || !first.Page.HasMore {
		t.Fatalf("first page failed: %v", err)
	}
	if len(collector.last.Targets) != 1 || collector.last.Targets[0].Container != "proxy" || collector.last.LineLimit != maximumCollectedLogLines {
		t.Fatal("selected container or fixed read window lost")
	}
	query.Cursor = first.Page.NextCursor
	second, err := provider.ReadLogs(context.Background(), query)
	if err != nil || len(second.Records) != 1 || second.Records[0].Message != "two" {
		t.Fatal("stable next page shifted with its limit")
	}
	query.Targets[0].Previous = true
	if _, err := provider.ReadLogs(context.Background(), query); err == nil {
		t.Fatal("current cursor reused for previous container")
	}
	query.Targets[0].Previous = false
	collector.lines = append(collector.lines, "window changed")
	if _, err := provider.ReadLogs(context.Background(), query); err == nil {
		t.Fatal("changed live window reused stale offset cursor")
	}
}

func TestProviderRedactionCannotExceedRequestedLogByteBudget(t *testing.T) {
	collector := &paginationLogCollector{snapshot: fixtureSnapshot(), lines: []string{"token=x"}}
	provider := newFixtureProvider(t, collector)
	resources, _ := provider.ListResources(context.Background(), inventoryQuery())
	var pod domain.ResourceRef
	for _, resource := range resources.Resources {
		if resource.Resource.Ref.Identity.Kind == "kubernetes.pod" {
			pod = resource.Resource.Ref
		}
	}
	query := domain.LogReadQuery{Source: domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"}, Targets: []domain.LogTarget{{Resource: pod}}, Range: domain.TimeRange{Start: collector.snapshot.ObservedAt.Add(-time.Hour), End: collector.snapshot.ObservedAt.Add(time.Nanosecond)}, LineLimit: 10, ByteLimit: 7}
	page, err := provider.ReadLogs(context.Background(), query)
	if err != nil || len(page.Records) != 1 || len(page.Records[0].Message) > 7 || !page.Records[0].Redacted || !page.Records[0].Truncated || strings.Contains(page.Records[0].Message, "x") {
		t.Fatalf("bounded redaction failed: %v", err)
	}
}

func TestSDKCollectionBudgetAndCancellationStopFurtherReads(t *testing.T) {
	requests := 0
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(http.StatusOK, `{"items":[]}`+strings.Repeat(" ", 40000)), nil
	}, func(options *APICollectorOptions) { options.MaximumResponse = minimumAPIResponseBytes })
	snapshot, err := collector.Snapshot(context.Background())
	if err != nil || requests != 2 || snapshot.Status.Completeness != domain.CompletenessPartial {
		t.Fatalf("aggregate response budget requests=%d error=%v", requests, err)
	}
	requests = 0
	collector = testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		requests++
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = collector.Snapshot(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || requests != 1 {
		t.Fatalf("cancellation escaped requests=%d", requests)
	}
}

func TestSDKPageWalkHasFixedBudgetEvenForEmptyContinuedPages(t *testing.T) {
	budget, pages := 10, 0
	_, err := readSDKList(context.Background(), "shop", &budget, func(context.Context, metav1.ListOptions) ([]metav1.PartialObjectMetadata, string, error) {
		pages++
		return nil, strings.Repeat("x", pages), nil
	})
	if !errors.Is(err, ErrTruncated) || pages != maximumAPIPages {
		t.Fatal("empty continuation pages escaped fixed page bound")
	}
}

func TestDerivedReferencesCannotRetainNamesFromDeniedParentKinds(t *testing.T) {
	denyPods := false
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/v1/namespaces/shop/pods" {
			if denyPods {
				return jsonResponse(http.StatusForbidden, `{"reason":"Forbidden","code":403}`), nil
			}
			return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"old-pod","name":"api","namespace":"shop"},"spec":{"containers":[{"name":"api","envFrom":[{"secretRef":{"name":"old-pod-only-ref"}}]}]}}]}`), nil
		}
		if request.URL.Path == "/apis/apps/v1/namespaces/shop/deployments" {
			return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"deploy","name":"worker","namespace":"shop"},"spec":{"replicas":0,"template":{"spec":{"containers":[{"name":"worker","envFrom":[{"secretRef":{"name":"new-allowed-ref"}}]}]}}}}]}`), nil
		}
		return emptyListTransport(request)
	})
	provider, _ := New("source_cluster", "Scoped", collector)
	before, err := provider.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, resource := range before.Resources {
		found = found || resource.Resource.DisplayName == "old-pod-only-ref"
	}
	if !found {
		t.Fatal("initial observed reference missing")
	}
	denyPods = true
	after, err := provider.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range after.Resources {
		if resource.Resource.Ref.Identity.Kind == "kubernetes.secret" || resource.Resource.Ref.Identity.Kind == "kubernetes.config_map" {
			t.Fatal("derived reference authority survived a denied parent")
		}
	}
	denied := false
	for _, partition := range after.Coverage {
		if partition.Kind == "kubernetes.secret" {
			denied = partition.Status.State == domain.AvailabilityPermissionDenied
		}
	}
	if !denied {
		t.Fatal("cached Pod-only names could survive as available partial references")
	}
}

func TestEventReasonCannotCopyArbitraryProviderText(t *testing.T) {
	if kubernetesEventReason("BackOff") != "kubernetes.event.backoff" || kubernetesEventReason("private-credential-material") != "kubernetes.event.unknown" {
		t.Fatal("event reason escaped reviewed code set")
	}
}

func TestSDKFutureUsageTimestampCannotExtendFreshness(t *testing.T) {
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		if response, matched := stableMetricIdentityResponse(request); matched {
			return response, nil
		}
		return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"name":"api","namespace":"shop"},"timestamp":"2099-08-16T10:00:00Z","window":"30s","containers":[{"name":"api","usage":{"cpu":"250m","memory":"128Mi"}}]}]}`), nil
	})
	metrics, err := collector.CurrentPodMetrics(context.Background())
	if err != nil || len(metrics.Metrics) != 0 || metrics.Status.State != domain.AvailabilityUnavailable {
		t.Fatal("future usage became current evidence")
	}
}

func TestSDKNormalizesFractionalUsageWindowToTransportSeconds(t *testing.T) {
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		if response, matched := stableMetricIdentityResponse(request); matched {
			return response, nil
		}
		return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"name":"api","namespace":"shop"},"timestamp":"2026-08-16T10:00:00Z","window":"15.012s","containers":[{"name":"api","usage":{"cpu":"250m","memory":"128Mi"}}]}]}`), nil
	})
	metrics, err := collector.CurrentPodMetrics(context.Background())
	if err != nil || len(metrics.Metrics) != 1 || metrics.Metrics[0].Window != 15*time.Second || len(metrics.Metrics[0].Containers) != 1 || metrics.Metrics[0].Containers[0].CPUCores != 0.25 || metrics.Metrics[0].Containers[0].MemoryBytes != 128<<20 {
		t.Fatal("real fractional usage window lost at whole-second boundary")
	}
}

func stableMetricIdentityResponse(request *http.Request) (*http.Response, bool) {
	if request.URL.Path == "/api/v1/namespaces/shop/pods" {
		return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"pod-one","name":"api","namespace":"shop","creationTimestamp":"2026-08-16T07:00:00Z"}}]}`), true
	}
	if request.URL.Path == "/api/v1/namespaces/shop/pods/api" {
		return jsonResponse(http.StatusOK, `{"metadata":{"uid":"pod-one","name":"api","namespace":"shop","creationTimestamp":"2026-08-16T07:00:00Z"}}`), true
	}
	return nil, false
}

func TestProviderLogReadKeepsDeniedPodPartitionTruth(t *testing.T) {
	collector := &paginationLogCollector{snapshot: fixtureSnapshot()}
	provider := newFixtureProvider(t, collector)
	resources, err := provider.ListResources(context.Background(), inventoryQuery())
	if err != nil {
		t.Fatal(err)
	}
	var pod domain.ResourceRef
	for _, resource := range resources.Resources {
		if resource.Resource.Ref.Identity.Kind == "kubernetes.pod" {
			pod = resource.Resource.Ref
		}
	}
	collector.snapshot.Pods = nil
	collector.snapshot.Partitions = []InventoryPartition{{Kind: "kubernetes.pod", Scope: pod.Identity.Scope, Status: observationStatus(collector.snapshot.ObservedAt, ErrPermissionDenied, false)}}
	query := domain.LogReadQuery{Source: inventoryQuery().Source, Targets: []domain.LogTarget{{Resource: pod}}, Range: domain.TimeRange{Start: collector.snapshot.ObservedAt.Add(-time.Hour), End: collector.snapshot.ObservedAt.Add(time.Nanosecond)}, LineLimit: 10, ByteLimit: 4096}
	page, err := provider.ReadLogs(context.Background(), query)
	if err != nil || page.Status.State != domain.AvailabilityPermissionDenied || len(page.Records) != 0 {
		t.Fatal("denied observed Pod became unsupported log target")
	}
}
