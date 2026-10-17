package kubernetes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacclient "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Fixture writes use a disposable-cluster administrator. The collector receives
// only a new namespace-bound service-account credential and a CA-only transport.
func TestScopedSDKOnRealKubernetes(t *testing.T) {
	testScopedSDKOnRealKubernetes(t, false)
}

func TestScopedSDKWithMetricsOnRealKubernetes(t *testing.T) {
	if os.Getenv("STACKPLORE_TEST_KUBE_METRICS_PRESENT") != "1" {
		if os.Getenv("KUBERNETES_REQUIRED") == "1" {
			t.Fatal("required positive Metrics API fixture not selected")
		}
		t.Skip("positive Metrics API fixture not selected")
	}
	testScopedSDKOnRealKubernetes(t, true)
}

func testScopedSDKOnRealKubernetes(t *testing.T, metricsPresent bool) {
	t.Helper()
	path := os.Getenv("STACKPLORE_TEST_KUBECONFIG")
	required := os.Getenv("KUBERNETES_REQUIRED") == "1"
	if path == "" {
		if required {
			t.Fatal("required Kubernetes target configuration missing")
		}
		t.Skip("disposable Kubernetes target not configured")
	}
	if os.Getenv("STACKPLORE_TEST_KUBE_DISPOSABLE") != "1" {
		t.Fatal("disposable Kubernetes fixture guard missing")
	}
	raw, err := clientcmd.LoadFromFile(path)
	if err != nil {
		t.Fatal("disposable Kubernetes configuration could not be read")
	}
	if !strings.HasPrefix(raw.CurrentContext, "kind-stackplore-proof-") {
		t.Fatal("fixture requires its own named kind context")
	}
	for _, auth := range raw.AuthInfos {
		if auth.Exec != nil || auth.AuthProvider != nil {
			t.Fatal("credential execution plugins are forbidden")
		}
	}
	config, err := clientcmd.NewNonInteractiveClientConfig(*raw, raw.CurrentContext, &clientcmd.ConfigOverrides{}, nil).ClientConfig()
	if err != nil {
		t.Fatal("disposable Kubernetes configuration invalid")
	}
	endpoint, err := url.Parse(config.Host)
	if err != nil || endpoint.Scheme != "https" || endpoint.User != nil || endpoint.Path != "" || net.ParseIP(endpoint.Hostname()) == nil || !net.ParseIP(endpoint.Hostname()).IsLoopback() || config.Insecure {
		t.Fatal("fixture requires trusted TLS on a loopback kind target")
	}
	config.Timeout = 10 * time.Second
	config.Proxy = func(*http.Request) (*url.URL, error) { return nil, nil }
	admin, err := coreclient.NewForConfig(config)
	if err != nil {
		t.Fatal("fixture core client failed")
	}
	rbac, err := rbacclient.NewForConfig(config)
	if err != nil {
		t.Fatal("fixture permission client failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	var nonce [6]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal("fixture identifier failed")
	}
	prefix := "stackplore-proof-" + hex.EncodeToString(nonce[:])
	allowed, denied := prefix+"-a", prefix+"-b"
	for _, namespace := range []string{allowed, denied} {
		created, err := admin.Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal("fixture namespace creation failed")
		}
		uid := created.UID
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if admin.Namespaces().Delete(cleanup, namespace, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}) != nil {
				t.Error("owned fixture namespace cleanup failed")
			}
		})
	}
	if _, err := admin.ServiceAccounts(allowed).Create(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "observer"}, AutomountServiceAccountToken: boolPointer(false)}, metav1.CreateOptions{}); err != nil {
		t.Fatal("fixture observer creation failed")
	}
	rules := []rbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"pods", "services", "persistentvolumeclaims", "events"}, Verbs: []string{"get", "list"}},
		{APIGroups: []string{""}, Resources: []string{"pods/log"}, Verbs: []string{"get"}},
		{APIGroups: []string{"apps"}, Resources: []string{"deployments", "statefulsets", "daemonsets", "replicasets"}, Verbs: []string{"get", "list"}},
		{APIGroups: []string{"batch"}, Resources: []string{"jobs", "cronjobs"}, Verbs: []string{"get", "list"}},
		{APIGroups: []string{"discovery.k8s.io"}, Resources: []string{"endpointslices"}, Verbs: []string{"get", "list"}},
		{APIGroups: []string{"metrics.k8s.io"}, Resources: []string{"pods"}, Verbs: []string{"get", "list"}},
	}
	if _, err := rbac.Roles(allowed).Create(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "observer"}, Rules: rules}, metav1.CreateOptions{}); err != nil {
		t.Fatal("fixture role creation failed")
	}
	if _, err := rbac.RoleBindings(allowed).Create(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "observer"}, RoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "observer"}, Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "observer", Namespace: allowed}}}, metav1.CreateOptions{}); err != nil {
		t.Fatal("fixture role binding failed")
	}
	probeImage := "busybox:1.36.1@sha256:73aaf090f3d85aa34ee199857f03fa3a95c8ede2ffd4cc2cdb5b94e566b11662"
	pod, err := admin.Pods(allowed).Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "probe", Labels: map[string]string{"app": "probe"}, Annotations: map[string]string{"stackplore.io/depends-on": "Service/probe"}}, Spec: corev1.PodSpec{
		AutomountServiceAccountToken: boolPointer(false), RestartPolicy: corev1.RestartPolicyAlways,
		Volumes: []corev1.Volume{{Name: "state", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
		Containers: []corev1.Container{{Name: "probe", Image: probeImage, ImagePullPolicy: corev1.PullIfNotPresent,
			Command:      []string{"sh", "-c", "if [ -f /state/restarted ]; then echo current-observation-proof; sleep 600; else touch /state/restarted; echo previous-observation-proof; exit 1; fi"},
			VolumeMounts: []corev1.VolumeMount{{Name: "state", MountPath: "/state"}},
			Resources:    corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("16Mi")}, Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m"), corev1.ResourceMemory: resource.MustParse("32Mi")}},
			EnvFrom:      []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "safe-reference"}, Optional: boolPointer(true)}}, {SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "safe-secret-reference"}, Optional: boolPointer(true)}}},
		}},
	}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal("fixture Pod creation failed")
	}
	if _, err := admin.Services(allowed).Create(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "probe"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "probe"}, Ports: []corev1.ServicePort{{Port: 8080}}}}, metav1.CreateOptions{}); err != nil {
		t.Fatal("fixture Service creation failed")
	}
	for {
		current, err := admin.Pods(allowed).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal("fixture Pod readiness read failed")
		}
		if len(current.Status.ContainerStatuses) == 1 && current.Status.ContainerStatuses[0].Ready && current.Status.ContainerStatuses[0].RestartCount >= 1 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("fixture Pod failed to reach restarted ready state")
		case <-time.After(time.Second):
		}
	}
	if _, err := admin.Events(allowed).Create(ctx, &corev1.Event{ObjectMeta: metav1.ObjectMeta{GenerateName: "proof-"}, InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: allowed, Name: pod.Name, UID: pod.UID}, Reason: "ObservationProof", Message: "safe observation event", Type: corev1.EventTypeNormal, FirstTimestamp: metav1.Now(), LastTimestamp: metav1.Now(), Count: 1, Source: corev1.EventSource{Component: "stackplore-fixture"}}, metav1.CreateOptions{}); err != nil {
		t.Fatal("fixture event creation failed")
	}
	expiration := int64(600)
	credential, err := admin.ServiceAccounts(allowed).CreateToken(ctx, "observer", &authenticationv1.TokenRequest{Spec: authenticationv1.TokenRequestSpec{ExpirationSeconds: &expiration}}, metav1.CreateOptions{})
	if err != nil || credential.Status.Token == "" {
		t.Fatal("fixture observer credential failed")
	}
	verifyMountedSDKOnOwnedKind(t, ctx, config.Host, config.CAData, config.CAFile, credential.Status.Token, allowed, denied)
	// Do not reuse the administrator client certificate for observation: it would
	// invalidate the service-account permission-denial proof.
	observerConfig := &rest.Config{Host: config.Host, TLSClientConfig: rest.TLSClientConfig{CAData: config.CAData, CAFile: config.CAFile, ServerName: config.ServerName}, Timeout: 10 * time.Second, Proxy: func(*http.Request) (*url.URL, error) { return nil, nil }}
	base, err := rest.TransportFor(observerConfig)
	if err != nil {
		t.Fatal("fixture observer TLS transport failed")
	}
	recorder := &kindObservationTransport{base: base, endpoint: endpoint}
	collector, err := NewAPICollector(APICollectorOptions{Client: &http.Client{Transport: recorder, Timeout: 10 * time.Second}, BearerToken: credential.Status.Token, Namespaces: []string{allowed, denied}, MaximumItems: 1000, MaximumResponse: 4 * 1024 * 1024})
	if err != nil {
		t.Fatal("official collector initialization failed")
	}
	defer collector.Close()
	provider, err := New("source_kind_proof", "Disposable kind proof", collector)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := provider.CaptureInventory(ctx)
	if err != nil {
		t.Fatalf("official collector capture failed: %v", err)
	}
	if capture.Status.State != domain.AvailabilityAvailable || capture.Status.Completeness != domain.CompletenessPartial {
		t.Fatal("denied optional partitions failed whole source")
	}
	var podRef, containerRef domain.ResourceRef
	for _, snapshot := range capture.Resources {
		if snapshot.Resource.Ref.Identity.Kind == "kubernetes.pod" && snapshot.Resource.Ref.Identity.NativeID() == domain.NativeResourceID(pod.UID) {
			podRef = snapshot.Resource.Ref
		}
		if snapshot.Resource.Ref.Identity.Kind == "kubernetes.container" {
			containerRef = snapshot.Resource.Ref
			if snapshot.Metadata == nil || snapshot.Metadata.Kubernetes.Containers[0].CPURequestCores == nil || *snapshot.Metadata.Kubernetes.Containers[0].CPURequestCores != 0.1 {
				t.Fatal("real container resource facts missing")
			}
		}
	}
	if podRef.ID == "" || containerRef.ID == "" {
		t.Fatal("real immutable Pod/container identity missing")
	}
	assertKindPartitions(t, capture, allowed, denied)
	metricsCollector, err := NewAPICollector(APICollectorOptions{Client: &http.Client{Transport: recorder, Timeout: 10 * time.Second}, BearerToken: credential.Status.Token, Namespaces: []string{allowed}, MaximumItems: 1000, MaximumResponse: 4 * 1024 * 1024})
	if err != nil {
		t.Fatal("allowed-only metrics collector initialization failed")
	}
	defer metricsCollector.Close()
	metricsProvider, err := New("source_kind_proof", "Disposable kind proof", metricsCollector)
	if err != nil {
		t.Fatal("allowed-only metrics provider initialization failed")
	}
	for _, previous := range []bool{false, true} {
		logs, err := provider.ReadLogs(ctx, domain.LogReadQuery{Source: capture.Source, Targets: []domain.LogTarget{{Resource: podRef, Container: "probe", Previous: previous}}, Range: domain.TimeRange{Start: time.Now().UTC().Add(-10 * time.Minute), End: time.Now().UTC().Add(time.Second)}, LineLimit: 20, ByteLimit: 4096})
		if err != nil {
			t.Fatalf("real bounded log read failed: %v", err)
		}
		marker := "current-observation-proof"
		if previous {
			marker = "previous-observation-proof"
		}
		found := false
		for _, record := range logs.Records {
			found = found || strings.Contains(record.Message, marker)
		}
		if !found {
			t.Fatal("real current/previous log proof missing")
		}
	}
	if metricsPresent {
		mixed, err := collector.CurrentPodMetrics(ctx)
		if err != nil || mixed.Status.State != domain.AvailabilityPermissionDenied || len(mixed.Metrics) != 0 || mixed.Namespaces[denied].State != domain.AvailabilityPermissionDenied {
			t.Fatal("mixed namespace usage denial did not block the metric port")
		}
		for {
			metrics, err := metricsCollector.CurrentPodMetrics(ctx)
			found := false
			for _, metric := range metrics.Metrics {
				if metric.Namespace == allowed && metric.PodName == pod.Name && !metric.SampledAt.Before(time.Now().Add(-5*time.Minute)) {
					for _, container := range metric.Containers {
						found = found || container.Name == "probe" && container.MemoryBytes > 0 && container.CPUCores >= 0
					}
				}
			}
			if err == nil && found {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("fresh live usage and denied namespace Metrics API proof missing")
			case <-time.After(2 * time.Second):
			}
		}
		usage, err := metricsProvider.GetCurrentMetrics(ctx, domain.CurrentMetricQuery{Source: capture.Source, Resources: []domain.ResourceRef{containerRef}, Names: []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes}})
		if err != nil || len(usage.Sets) != 1 || len(usage.Sets[0].Samples) != 2 || usage.Sets[0].Samples[0].Interval < time.Second {
			t.Fatal("live resource usage did not cross provider boundary")
		}
	} else {
		metrics, err := metricsCollector.CurrentPodMetrics(ctx)
		if err != nil || metrics.Namespaces[allowed].State != domain.AvailabilityUnsupported || len(metrics.Metrics) != 0 {
			t.Fatal("real absent Metrics API did not report unsupported without samples")
		}
	}
	metricResult, err := metricsProvider.GetCurrentMetrics(ctx, domain.CurrentMetricQuery{Source: capture.Source, Resources: []domain.ResourceRef{containerRef}, Names: []domain.MetricName{domain.MetricCPURequestCores, domain.MetricMemoryLimitBytes}})
	if err != nil || len(metricResult.Sets) != 1 || len(metricResult.Sets[0].Samples) != 2 {
		t.Fatal("configured resource metrics wrongly depend on Metrics API")
	}
	events, err := provider.ListEvents(ctx, domain.EventQuery{Source: capture.Source, Resources: []domain.ResourceRef{podRef}, Range: domain.TimeRange{Start: time.Now().UTC().Add(-10 * time.Minute), End: time.Now().UTC().Add(time.Second)}, Page: domain.PageRequest{Limit: 100}})
	if err != nil || len(events.Events) == 0 {
		t.Fatal("real scoped event proof missing")
	}
	uid := pod.UID
	zero := int64(0)
	if admin.Pods(allowed).Delete(ctx, pod.Name, metav1.DeleteOptions{GracePeriodSeconds: &zero, Preconditions: &metav1.Preconditions{UID: &uid}}) != nil {
		t.Fatal("owned fixture Pod delete failed")
	}
	for {
		after, err := provider.CaptureInventory(ctx)
		if err != nil {
			t.Fatal("real recollection after deletion failed")
		}
		present := false
		for _, resource := range after.Resources {
			present = present || resource.Resource.Ref.ID == podRef.ID
		}
		if !present {
			assertKindPartitions(t, after, allowed, denied)
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("deleted fixture Pod remained observed")
		case <-time.After(time.Second):
		}
	}
	recorder.assertReadOnly(t, allowed, denied)
}

func assertKindPartitions(t *testing.T, capture domain.InventoryCapture, allowed, denied string) {
	t.Helper()
	podComplete, optionalDenied, scopeDenied := false, false, false
	for _, partition := range capture.Coverage {
		if partition.Kind == "kubernetes.pod" && partition.Scope.Name == allowed {
			podComplete = partition.Complete()
		}
		if partition.Kind == "kubernetes.ingress" && partition.Scope.Name == allowed {
			optionalDenied = partition.Status.State == domain.AvailabilityPermissionDenied
		}
		if partition.Kind == "kubernetes.pod" && partition.Scope.Name == denied {
			scopeDenied = partition.Status.State == domain.AvailabilityPermissionDenied
		}
	}
	if !podComplete || !optionalDenied || !scopeDenied {
		t.Fatal("real complete/denied partition distinction missing")
	}
}

type kindObservationTransport struct {
	base     http.RoundTripper
	endpoint *url.URL
	mutex    sync.Mutex
	requests []string
}

func (transport *kindObservationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.mutex.Lock()
	if len(transport.requests) >= 4096 {
		transport.mutex.Unlock()
		return nil, ErrTruncated
	}
	transport.requests = append(transport.requests, request.Method+" "+request.URL.Path)
	transport.mutex.Unlock()
	copy := request.Clone(request.Context())
	urlCopy := *request.URL
	copy.URL = &urlCopy
	copy.URL.Scheme = transport.endpoint.Scheme
	copy.URL.Host = transport.endpoint.Host
	copy.Host = transport.endpoint.Host
	return transport.base.RoundTrip(copy)
}
func (transport *kindObservationTransport) CloseIdleConnections() {
	if closer, ok := transport.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}
func (transport *kindObservationTransport) assertReadOnly(t *testing.T, allowed, denied string) {
	t.Helper()
	transport.mutex.Lock()
	defer transport.mutex.Unlock()
	if len(transport.requests) == 0 {
		t.Fatal("real collector sent no requests")
	}
	for _, request := range transport.requests {
		if !strings.HasPrefix(request, "GET ") || !strings.Contains(request, "/namespaces/"+allowed+"/") && !strings.Contains(request, "/namespaces/"+denied+"/") {
			t.Fatal("real observation request escaped method or namespace")
		}
		for _, forbidden := range []string{"/secrets", "/configmaps", "/exec", "/proxy", "/nodes"} {
			if strings.Contains(request, forbidden) {
				t.Fatal("real observer touched forbidden endpoint")
			}
		}
	}
}
func boolPointer(value bool) *bool { return &value }
