package kubernetes

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func testAPICollector(t *testing.T, namespaces []string, transport roundTripFunc, modify ...func(*APICollectorOptions)) *APICollector {
	t.Helper()
	options := APICollectorOptions{Client: &http.Client{Transport: transport}, BearerToken: "unit-test-token", Namespaces: namespaces, MaximumItems: 100, MaximumResponse: 1024 * 1024, Now: func() time.Time { return time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC) }}
	for _, change := range modify {
		change(&options)
	}
	collector, err := NewAPICollector(options)
	if err != nil {
		t.Fatal(err)
	}
	return collector
}

func emptyListTransport(request *http.Request) (*http.Response, error) {
	return jsonResponse(http.StatusOK, `{"items":[]}`), nil
}

func TestSDKCollectorReadsOnlyConfiguredNamespaceAndSafeReferences(t *testing.T) {
	responses := map[string]string{
		"/api/v1/namespaces/shop/pods":              `{"items":[{"metadata":{"uid":"pod-one","name":"api-1","namespace":"shop","labels":{"app":"api"},"annotations":{"stackplore.io/depends-on":"Service/api"},"ownerReferences":[{"uid":"custom-owner","kind":"CustomWorkload","controller":true}]},"spec":{"containers":[{"name":"api","resources":{"requests":{"cpu":"100m","memory":"16Mi"},"limits":{"cpu":"250m","memory":"32Mi"}},"env":[{"name":"PRIVATE","value":"value-never-copied"}],"envFrom":[{"secretRef":{"name":"shop-token"}}]}],"initContainers":[{"name":"init","envFrom":[{"configMapRef":{"name":"init-config"}}]}],"volumes":[{"persistentVolumeClaim":{"claimName":"shop-data"}},{"projected":{"sources":[{"secret":{"name":"projected-token"}},{"configMap":{"name":"projected-config"}}]}}]},"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"},{"type":"ContainersReady","status":"True"}],"containerStatuses":[{"name":"api","ready":true,"restartCount":2,"state":{"running":{"startedAt":"2026-08-16T09:00:00Z"}}}],"initContainerStatuses":[{"name":"init","ready":false,"restartCount":0,"state":{"terminated":{"reason":"Completed","exitCode":0}}}]}}]}`,
		"/api/v1/namespaces/shop/services":          `{"items":[{"metadata":{"uid":"service-one","name":"api","namespace":"shop"},"spec":{"selector":{"app":"api"}}}]}`,
		"/apis/apps/v1/namespaces/shop/deployments": `{"items":[{"metadata":{"uid":"deployment-one","name":"api","namespace":"shop","resourceVersion":"7"},"spec":{"replicas":0,"template":{"spec":{"containers":[{"name":"api","envFrom":[{"configMapRef":{"name":"zero-replica-config"}}]}]}}}}]}`,
	}
	seen := []string{}
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != "kubernetes.default.svc" || request.Header.Get("Authorization") == "" {
			t.Fatal("unsafe SDK request")
		}
		if !strings.Contains(request.URL.Path, "/namespaces/shop/") {
			t.Fatalf("scope escaped: %s", request.URL.Path)
		}
		seen = append(seen, request.URL.Path)
		body, exists := responses[request.URL.Path]
		if !exists {
			body = `{"items":[]}`
		}
		return jsonResponse(http.StatusOK, body), nil
	})
	snapshot, err := collector.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ClusterObserved || len(snapshot.Nodes) != 0 || len(snapshot.Namespaces) != 0 {
		t.Fatal("namespace profile invented cluster observations")
	}
	if len(snapshot.Pods) != 1 || len(snapshot.Pods[0].Containers) != 2 || len(snapshot.ConfigMaps) != 3 || len(snapshot.Secrets) != 2 || len(snapshot.Workloads) != 1 {
		t.Fatalf("safe snapshot shape pods=%d configs=%d secrets=%d workloads=%d", len(snapshot.Pods), len(snapshot.ConfigMaps), len(snapshot.Secrets), len(snapshot.Workloads))
	}
	container := snapshot.Pods[0].Metadata.Containers[0]
	if container.CPURequestCores == nil || *container.CPURequestCores != 0.1 || container.MemoryLimitBytes == nil || *container.MemoryLimitBytes != 32<<20 || container.State != domain.KubernetesContainerRunning || container.RestartCount != 2 {
		t.Fatal("typed container facts missing")
	}
	if len(seen) != 11 {
		t.Fatalf("request count=%d; want 11", len(seen))
	}
	provider, err := New("source_cluster", "Scoped Kubernetes", collector)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if capture.Complete || capture.Status.Completeness != domain.CompletenessPartial || len(capture.Coverage) != 14 {
		t.Fatalf("coverage=%d, completeness=%s", len(capture.Coverage), capture.Status.Completeness)
	}
	var dependency, reference bool
	for _, edge := range capture.Relationships {
		dependency = dependency || edge.Kind == domain.RelationshipDependsOn
		reference = reference || edge.Kind == domain.RelationshipReferences
	}
	if !dependency || !reference {
		t.Fatal("safe explicit relationships missing")
	}
	for _, resource := range capture.Resources {
		if resource.Metadata != nil && resource.Metadata.ValidateFor(domain.ProviderKubernetes) != nil {
			t.Fatal("unsafe metadata escaped")
		}
	}
}

func TestSDKCollectorIsolatesDeniedKindsAndNamespaces(t *testing.T) {
	collector := testAPICollector(t, []string{"shop", "denied"}, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "/namespaces/denied/") || strings.HasSuffix(request.URL.Path, "/ingresses") {
			return jsonResponse(http.StatusForbidden, `{"kind":"Status","apiVersion":"v1","reason":"Forbidden","code":403}`), nil
		}
		if request.URL.Path == "/api/v1/namespaces/shop/pods" {
			return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"pod-one","name":"api","namespace":"shop"},"spec":{"containers":[{"name":"api"}]}}]}`), nil
		}
		return emptyListTransport(request)
	})
	provider, _ := New("source_cluster", "Scoped Kubernetes", collector)
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(capture.Resources) != 2 || capture.Status.State != domain.AvailabilityAvailable || capture.Status.Completeness != domain.CompletenessPartial {
		t.Fatalf("partial resources=%d status=%s", len(capture.Resources), capture.Status.State)
	}
	for _, resource := range capture.Resources {
		if resource.Status.State != domain.AvailabilityAvailable || resource.Status.Completeness != domain.CompletenessComplete {
			t.Fatal("independent successful resource lost complete evidence")
		}
	}
	var podComplete, ingressDenied, namespaceDenied bool
	for _, coverage := range capture.Coverage {
		if coverage.Kind == "kubernetes.pod" && coverage.Scope.Name == "shop" {
			podComplete = coverage.Complete()
		}
		if coverage.Kind == "kubernetes.ingress" && coverage.Scope.Name == "shop" {
			ingressDenied = coverage.Status.State == domain.AvailabilityPermissionDenied
		}
		if coverage.Kind == "kubernetes.pod" && coverage.Scope.Name == "denied" {
			namespaceDenied = coverage.Status.State == domain.AvailabilityPermissionDenied
		}
	}
	if !podComplete || !ingressDenied || !namespaceDenied {
		t.Fatal("partition authority or completion lost")
	}
}

func TestSDKCollectorOpaquePaginationAndLaterFailure(t *testing.T) {
	for _, mode := range []string{"complete", "repeat", "denied", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			requests := 0
			collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
				requests++
				token := request.URL.Query().Get("continue")
				if token == "" {
					return jsonResponse(http.StatusOK, `{"metadata":{"continue":"next page"},"items":[{"metadata":{"uid":"one","name":"one","namespace":"shop"}}]}`), nil
				}
				if token != "next page" {
					t.Fatal("opaque token changed")
				}
				switch mode {
				case "repeat":
					return jsonResponse(http.StatusOK, `{"metadata":{"continue":"next page"},"items":[]}`), nil
				case "denied":
					return jsonResponse(http.StatusForbidden, `{"reason":"Forbidden","code":403}`), nil
				case "timeout":
					return nil, context.DeadlineExceeded
				}
				return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"two","name":"two","namespace":"shop"}}]}`), nil
			})
			budget := 10
			items, err := readSDKList(context.Background(), "shop", &budget, func(ctx context.Context, options metav1.ListOptions) ([]corev1.Pod, string, error) {
				page, err := collector.core.Pods("shop").List(ctx, options)
				if err != nil {
					return nil, "", err
				}
				return page.Items, page.Continue, nil
			})
			if requests != 2 {
				t.Fatalf("SDK retried unexpected request %d", requests)
			}
			if mode == "complete" {
				if err != nil || len(items) != 2 || budget != 8 {
					t.Fatal("complete pagination failed")
				}
			} else if mode == "denied" {
				if !errors.Is(err, ErrPermissionDenied) || len(items) != 0 {
					t.Fatal("denied later page retained current authority")
				}
			} else if err == nil || len(items) != 1 {
				t.Fatal("partial earlier page lost or marked complete")
			}
		})
	}
}

func TestSDKCollectorRejectsScopeAndWireBoundaryViolations(t *testing.T) {
	base := APICollectorOptions{Client: &http.Client{Transport: roundTripFunc(emptyListTransport)}, BearerToken: "unit-test-token", Namespaces: []string{"shop"}, MaximumItems: 10, MaximumResponse: minimumAPIResponseBytes}
	for _, namespaces := range [][]string{nil, {""}, {"shop", "shop"}, {"../shop"}, {"Shop"}, {strings.Repeat("a", 64)}} {
		options := base
		options.Namespaces = namespaces
		if _, err := NewAPICollector(options); err == nil {
			t.Fatal("invalid namespace profile accepted")
		}
	}
	for _, mode := range []string{"oversized", "gzip", "redirect", "retry", "wrong_namespace"} {
		t.Run(mode, func(t *testing.T) {
			requests := 0
			collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
				requests++
				switch mode {
				case "oversized":
					return jsonResponse(http.StatusOK, `{"items":[]}`+strings.Repeat(" ", int(minimumAPIResponseBytes))), nil
				case "gzip":
					response := jsonResponse(http.StatusOK, `{"items":[]}`)
					response.Header.Set("Content-Encoding", "gzip")
					return response, nil
				case "redirect":
					response := jsonResponse(http.StatusFound, `{}`)
					response.Header.Set("Location", "https://elsewhere.example/api")
					return response, nil
				case "retry":
					response := jsonResponse(http.StatusTooManyRequests, `{}`)
					response.Header.Set("Retry-After", "60")
					return response, nil
				default:
					return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"one","name":"one","namespace":"outside"}}]}`), nil
				}
			}, func(options *APICollectorOptions) { options.MaximumResponse = minimumAPIResponseBytes })
			budget := 10
			items, err := readSDKList(context.Background(), "shop", &budget, func(ctx context.Context, options metav1.ListOptions) ([]corev1.Pod, string, error) {
				page, err := collector.core.Pods("shop").List(ctx, options)
				if err != nil {
					return nil, "", err
				}
				return page.Items, page.Continue, nil
			})
			if err == nil || len(items) != 0 || requests != 1 {
				t.Fatalf("wire boundary accepted or retried: requests=%d", requests)
			}
		})
	}
	requests := 0
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) { requests++; return emptyListTransport(request) })
	for _, path := range []string{"/api/v1/pods", "/api/v1/nodes", "/api/v1/namespaces", "/api/v1/namespaces/shop/secrets", "/api/v1/namespaces/shop/configmaps", "/api/v1/namespaces/other/pods", "/api/v1/namespaces/shop/pods/api/exec", "/api/v1/namespaces/shop/pods/api/proxy"} {
		if _, err := collector.core.RESTClient().Get().AbsPath(path).DoRaw(context.Background()); err == nil {
			t.Fatalf("forbidden path passed: %s", path)
		}
	}
	if requests != 0 {
		t.Fatal("forbidden path reached transport")
	}
}

func TestSDKCollectorObservesClusterOnlyByExplicitProfile(t *testing.T) {
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v1/nodes":
			return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"node-one","name":"node"},"status":{"conditions":[{"type":"Ready","status":"True"}]}}]}`), nil
		case "/api/v1/namespaces/shop":
			return jsonResponse(http.StatusOK, `{"metadata":{"uid":"namespace-one","name":"shop"},"status":{"phase":"Active"}}`), nil
		}
		return emptyListTransport(request)
	}, func(options *APICollectorOptions) { options.ClusterScoped = true })
	provider, _ := New("source_cluster", "Explicit cluster", collector)
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !capture.Complete || len(capture.Resources) != 3 {
		t.Fatalf("cluster capture resources=%d complete=%v", len(capture.Resources), capture.Complete)
	}
	for _, resource := range capture.Resources {
		if resource.Resource.Ref.Identity.Kind == "kubernetes.namespace" && resource.Resource.Ref.Identity.NativeID() != "namespace-one" {
			t.Fatal("namespace UID identity lost")
		}
	}
}

func TestSDKCollectorEventsMetricsAndBoundedPreviousLogs(t *testing.T) {
	collector := testAPICollector(t, []string{"shop", "denied"}, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "/namespaces/denied/") {
			return jsonResponse(http.StatusForbidden, `{"reason":"Forbidden","code":403}`), nil
		}
		switch request.URL.Path {
		case "/api/v1/namespaces/shop/pods":
			return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"pod-one","name":"api","namespace":"shop","creationTimestamp":"2026-08-16T07:00:00Z"}}]}`), nil
		case "/api/v1/namespaces/shop/pods/api":
			return jsonResponse(http.StatusOK, `{"metadata":{"uid":"pod-one","name":"api","namespace":"shop","creationTimestamp":"2026-08-16T07:00:00Z"}}`), nil
		case "/api/v1/namespaces/shop/events":
			return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"event-one","name":"api.event","namespace":"shop"},"involvedObject":{"uid":"pod-one"},"reason":"BackOff","message":"restart backoff","type":"Warning","eventTime":"2026-08-16T08:30:00.000000Z","series":{"count":3,"lastObservedTime":"2026-08-16T08:31:00.000000Z"}}]}`), nil
		case "/apis/metrics.k8s.io/v1beta1/namespaces/shop/pods":
			return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"name":"api","namespace":"shop"},"timestamp":"2026-08-16T08:30:00Z","window":"30s","containers":[{"name":"api","usage":{"cpu":"250m","memory":"128Mi"}}]}]}`), nil
		case "/api/v1/namespaces/shop/pods/api/log":
			if request.URL.Query().Get("previous") != "true" || request.URL.Query().Get("timestamps") != "true" || request.URL.Query().Get("container") != "api" || request.URL.Query().Get("follow") == "true" {
				t.Fatal("bounded previous log options lost")
			}
			return jsonResponse(http.StatusOK, "2026-08-16T08:30:00Z previous line\n"), nil
		}
		t.Fatalf("unexpected request %s", request.URL.Path)
		return nil, ErrUnavailable
	})
	events, err := collector.ClusterEvents(context.Background())
	if err != nil || len(events.Events) != 1 || events.Events[0].Occurrences != 3 || events.Events[0].OccurredAt.Minute() != 31 || events.Status.Completeness != domain.CompletenessPartial {
		t.Fatalf("event scope evidence failed: %v", err)
	}
	metrics, err := collector.CurrentPodMetrics(context.Background())
	if err != nil || len(metrics.Metrics) != 0 || metrics.Status.State != domain.AvailabilityPermissionDenied || metrics.Namespaces["denied"].State != domain.AvailabilityPermissionDenied {
		t.Fatalf("metric scope evidence failed: %v", err)
	}
	logs, err := collector.ReadPodLogs(context.Background(), PodLogRequest{Targets: []PodLogTarget{{Key: "one", ExpectedUID: "pod-one", Namespace: "shop", PodName: "api", Container: "api", Previous: true}, {Key: "two", ExpectedUID: "pod-two", Namespace: "denied", PodName: "api", Container: "api"}}, Start: time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC), LineLimit: 10, ByteLimit: 1024})
	if err != nil || len(logs.Lines) != 1 || logs.Lines[0].Message != "previous line" || logs.Status.Completeness != domain.CompletenessPartial {
		t.Fatalf("log scope evidence failed: %v", err)
	}
	_, err = collector.ReadPodLogs(context.Background(), PodLogRequest{Targets: []PodLogTarget{{Key: "one", Namespace: "outside", PodName: "api", Container: "api"}}, Start: time.Now().UTC(), LineLimit: 10, ByteLimit: 1024})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatal("log target escaped namespace profile")
	}
}

func TestSDKCollectorMissingMetricsDoesNotInventZeroSamples(t *testing.T) {
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/v1/namespaces/shop/pods" {
			return jsonResponse(http.StatusOK, `{"items":[]}`), nil
		}
		return jsonResponse(http.StatusNotFound, `{"kind":"Status","apiVersion":"v1","reason":"NotFound","code":404}`), nil
	})
	metrics, err := collector.CurrentPodMetrics(context.Background())
	if err != nil || len(metrics.Metrics) != 0 || metrics.Status.State != domain.AvailabilityUnsupported {
		t.Fatalf("missing metrics state=%s error=%v", metrics.Status.State, err)
	}
}

func TestSDKCollectorNormalizedExpansionIsBoundedAndNotComplete(t *testing.T) {
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/pods") {
			return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"one","name":"api","namespace":"shop"},"spec":{"containers":[{"name":"one"},{"name":"two"},{"name":"three"}]}}]}`), nil
		}
		return emptyListTransport(request)
	}, func(options *APICollectorOptions) { options.MaximumItems = 2 })
	provider, _ := New("source_cluster", "Scoped", collector)
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(capture.Resources) > 2 || capture.Complete {
		t.Fatal("normalized container expansion escaped budget")
	}
	for _, coverage := range capture.Coverage {
		if coverage.Kind == "kubernetes.pod" && coverage.Complete() {
			t.Fatal("truncated normalized partition allows retirement")
		}
	}
}

func TestDependencyAnnotationHasFixedSameNamespaceGrammar(t *testing.T) {
	got := parseDependencies("Service/api,Deployment/worker,Service/api,https://outside.example,bad/name,Service/../../host")
	if len(got) != 2 || !slices.Contains(got, DependencyReference{Kind: "kubernetes.service", Name: "api"}) {
		t.Fatal("dependency grammar escaped fixed names")
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}
