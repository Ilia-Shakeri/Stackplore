package kubernetes

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestAPICollectorReadsBoundedSafeObjectsAndDerivesNamedRefs(t *testing.T) {
	responses := map[string]string{
		"/api/v1/namespaces":                       `{"items":[{"metadata":{"uid":"ns-one","name":"shop"},"status":{"phase":"Active"}}]}`,
		"/api/v1/nodes":                            `{"items":[{"metadata":{"uid":"node-one","name":"worker-1"},"status":{"conditions":[{"type":"Ready","status":"True"}]}}]}`,
		"/api/v1/pods":                             `{"items":[{"metadata":{"uid":"pod-one","name":"api-1","namespace":"shop","labels":{"app":"api"},"ownerReferences":[{"uid":"rs-one","kind":"ReplicaSet","controller":true}]},"spec":{"containers":[{"name":"api","envFrom":[{"secretRef":{"name":"shop-token"}}],"env":[{"valueFrom":{"configMapKeyRef":{"name":"shop-config"}}}]}],"volumes":[{"persistentVolumeClaim":{"claimName":"shop-data"}},{"secret":{"secretName":"shop-token"}}]},"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}]}}]}`,
		"/api/v1/services":                         `{"items":[{"metadata":{"uid":"service-one","name":"api","namespace":"shop"},"spec":{"selector":{"app":"api"}}}]}`,
		"/api/v1/persistentvolumeclaims":           `{"items":[{"metadata":{"uid":"claim-one","name":"shop-data","namespace":"shop"},"status":{"phase":"Bound"}}]}`,
		"/apis/apps/v1/deployments":                `{"items":[{"metadata":{"uid":"deploy-one","name":"api","namespace":"shop"},"spec":{"replicas":1},"status":{"readyReplicas":1}}]}`,
		"/apis/apps/v1/statefulsets":               `{"items":[]}`,
		"/apis/apps/v1/daemonsets":                 `{"items":[]}`,
		"/apis/apps/v1/replicasets":                `{"items":[{"metadata":{"uid":"rs-one","name":"api-abc","namespace":"shop","ownerReferences":[{"uid":"deploy-one","kind":"Deployment","controller":true}]},"spec":{"replicas":1},"status":{"readyReplicas":1}}]}`,
		"/apis/batch/v1/jobs":                      `{"items":[]}`,
		"/apis/batch/v1/cronjobs":                  `{"items":[]}`,
		"/apis/networking.k8s.io/v1/ingresses":     `{"items":[{"metadata":{"uid":"ingress-one","name":"shop","namespace":"shop"},"spec":{"rules":[{"http":{"paths":[{"backend":{"service":{"name":"api"}}}]}}]}}]}`,
		"/apis/discovery.k8s.io/v1/endpointslices": `{"items":[{"metadata":{"uid":"endpoint-one","name":"api-abc","namespace":"shop","labels":{"kubernetes.io/service-name":"api"}},"endpoints":[{"targetRef":{"kind":"Pod","uid":"pod-one"}}]}]}`,
	}
	seen := map[string]int{}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != "kubernetes.default.svc" {
			t.Fatalf("unsafe request: %s %s", request.Method, request.URL)
		}
		if request.Header.Get("Authorization") != "Bearer unit-test-token" {
			t.Fatal("bearer header missing")
		}
		body, exists := responses[request.URL.Path]
		if !exists {
			t.Fatalf("unexpected API read %q", request.URL.Path)
		}
		seen[request.URL.Path]++
		return jsonResponse(http.StatusOK, body), nil
	})}
	collector, err := NewAPICollector(APICollectorOptions{
		Client: client, BearerToken: "unit-test-token", MaximumItems: 100,
		MaximumResponse: 1024 * 1024, Now: func() time.Time { return time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewAPICollector() error = %v", err)
	}
	snapshot, err := collector.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Pods) != 1 || len(snapshot.Secrets) != 1 || len(snapshot.ConfigMaps) != 1 || len(snapshot.Workloads) != 2 {
		t.Fatalf("snapshot shape = pods:%d secrets:%d config:%d workloads:%d", len(snapshot.Pods), len(snapshot.Secrets), len(snapshot.ConfigMaps), len(snapshot.Workloads))
	}
	if snapshot.Secrets[0].Name != "shop-token" || snapshot.ConfigMaps[0].Name != "shop-config" {
		t.Fatalf("safe refs = %#v %#v", snapshot.Secrets, snapshot.ConfigMaps)
	}
	if seen["/api/v1/secrets"] != 0 || seen["/api/v1/configmaps"] != 0 {
		t.Fatal("sensitive object body endpoint was read")
	}
}

func TestAPICollectorFollowsOpaquePaginationAndMapsDenial(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v1/namespaces" {
			return jsonResponse(http.StatusForbidden, `{}`), nil
		}
		requests++
		if request.URL.Query().Get("continue") == "" {
			return jsonResponse(http.StatusOK, `{"metadata":{"continue":"next page"},"items":[{"metadata":{"uid":"one","name":"one"}}]}`), nil
		}
		if request.URL.Query().Get("continue") != "next page" {
			t.Fatalf("continue token = %q", request.URL.Query().Get("continue"))
		}
		return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"two","name":"two"}}]}`), nil
	})}
	collector, err := NewAPICollector(APICollectorOptions{
		Client: client, BearerToken: "unit-test-token", MaximumItems: 10, MaximumResponse: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewAPICollector() error = %v", err)
	}
	budget := 10
	items, err := getAll[namespaceObject](context.Background(), collector, "/api/v1/namespaces", &budget)
	if err != nil || len(items) != 2 || requests != 2 || budget != 8 {
		t.Fatalf("page result = %d items, %d requests, budget %d, error %v", len(items), requests, budget, err)
	}
	_, err = collector.Snapshot(context.Background())
	if !errorsIs(err, ErrPermissionDenied) {
		t.Fatalf("denied error = %v", err)
	}
}

func TestAPICollectorRejectsUnsafeOptionsAndOversizedPage(t *testing.T) {
	if _, err := NewAPICollector(APICollectorOptions{}); err == nil {
		t.Fatal("unsafe options accepted")
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"items":[{},{}]}`), nil
	})}
	collector, err := NewAPICollector(APICollectorOptions{
		Client: client, BearerToken: "unit-test-token", MaximumItems: 1, MaximumResponse: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewAPICollector() error = %v", err)
	}
	budget := 1
	if _, err := getAll[namespaceObject](context.Background(), collector, "/api/v1/namespaces", &budget); err == nil {
		t.Fatal("oversized page accepted")
	}
}

func TestAPICollectorReadsMetadataOnlyCoreEvents(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v1/events" || request.Header.Get("Authorization") != "Bearer unit-test-token" {
			t.Fatalf("unexpected event request: %s", request.URL)
		}
		return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"uid":"event-one"},"involvedObject":{"uid":"pod-one"},"reason":"BackOff","message":"Container restart backoff","type":"Warning","eventTime":"2026-08-16T08:30:00Z"}]}`), nil
	})}
	collector, err := NewAPICollector(APICollectorOptions{
		Client: client, BearerToken: "unit-test-token", MaximumItems: 10, MaximumResponse: 1024 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	events, err := collector.ClusterEvents(context.Background())
	if err != nil || len(events) != 1 || events[0].UID != "event-one" || events[0].InvolvedUID != "pod-one" || events[0].OccurredAt != time.Date(2026, 8, 16, 8, 30, 0, 0, time.UTC) {
		t.Fatalf("ClusterEvents() = %#v, error = %v", events, err)
	}
}

func TestAPICollectorReadsBoundedMetricsAPIValues(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/apis/metrics.k8s.io/v1beta1/pods" || request.Header.Get("Authorization") != "Bearer unit-test-token" {
			t.Fatalf("unexpected metric request: %s", request.URL)
		}
		return jsonResponse(http.StatusOK, `{"items":[{"metadata":{"name":"api-1","namespace":"shop"},"timestamp":"2026-08-16T08:30:00Z","window":"30s","containers":[{"name":"api","usage":{"cpu":"250m","memory":"128Mi"}}]}]}`), nil
	})}
	collector, err := NewAPICollector(APICollectorOptions{
		Client: client, BearerToken: "unit-test-token", MaximumItems: 10, MaximumResponse: 1024 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := collector.CurrentPodMetrics(context.Background())
	if err != nil || len(metrics) != 1 || len(metrics[0].Containers) != 1 || metrics[0].Containers[0].CPUCores != 0.25 || metrics[0].Containers[0].MemoryBytes != 128<<20 || metrics[0].Window != 30*time.Second {
		t.Fatalf("CurrentPodMetrics() = %#v, error = %v", metrics, err)
	}
}

func TestAPICollectorReadsBoundedPodLogs(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v1/namespaces/shop/pods/api-1/log" || request.Header.Get("Authorization") != "Bearer unit-test-token" ||
			request.URL.Query().Get("container") != "api" || request.URL.Query().Get("timestamps") != "true" {
			t.Fatalf("unexpected log request: %s", request.URL)
		}
		return jsonResponse(http.StatusOK, "2026-08-16T08:30:00Z request done\n"), nil
	})}
	collector, err := NewAPICollector(APICollectorOptions{
		Client: client, BearerToken: "unit-test-token", MaximumItems: 10, MaximumResponse: 1024 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines, err := collector.ReadPodLogs(context.Background(), PodLogRequest{
		Targets: []PodLogTarget{{Key: "target-one", Namespace: "shop", PodName: "api-1", Container: "api"}},
		Start:   time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC), LineLimit: 10, ByteLimit: 1024,
	})
	if err != nil || len(lines) != 1 || lines[0].TargetKey != "target-one" || lines[0].Message != "request done" {
		t.Fatalf("ReadPodLogs() = %#v, error = %v", lines, err)
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)),
	}
}

func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}
