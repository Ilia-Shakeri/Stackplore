package kubernetes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	appsv1 "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsclient "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacclient "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func TestGuardedSDKWriterOnRealKubernetes(t *testing.T) {
	if os.Getenv("STACKPLORE_TEST_KUBE_WRITES") != "1" || os.Getenv("STACKPLORE_TEST_KUBECONFIG") == "" {
		if os.Getenv("KUBERNETES_REQUIRED") == "1" {
			t.Fatal("required guarded Kubernetes fixture is not configured")
		}
		t.Skip("guarded disposable Kubernetes fixture not selected")
	}
	if os.Getenv("STACKPLORE_TEST_KUBE_DISPOSABLE") != "1" {
		t.Fatal("guarded fixture requires disposable target")
	}
	raw, err := clientcmd.LoadFromFile(os.Getenv("STACKPLORE_TEST_KUBECONFIG"))
	if err != nil {
		t.Fatal("fixture configuration unreadable")
	}
	if !strings.HasPrefix(raw.CurrentContext, "kind-stackplore-proof-") {
		t.Fatal("fixture requires owned named kind context")
	}
	for _, auth := range raw.AuthInfos {
		if auth.Exec != nil || auth.AuthProvider != nil {
			t.Fatal("credential plugins forbidden")
		}
	}
	config, err := clientcmd.NewNonInteractiveClientConfig(*raw, raw.CurrentContext, &clientcmd.ConfigOverrides{}, nil).ClientConfig()
	if err != nil {
		t.Fatal("fixture configuration invalid")
	}
	endpoint, err := url.Parse(config.Host)
	if err != nil || endpoint.Scheme != "https" || endpoint.User != nil || endpoint.Path != "" || net.ParseIP(endpoint.Hostname()) == nil || !net.ParseIP(endpoint.Hostname()).IsLoopback() || config.Insecure {
		t.Fatal("fixture requires loopback trusted TLS")
	}
	config.Timeout = 10 * time.Second
	config.Proxy = func(*http.Request) (*url.URL, error) { return nil, nil }
	core, err := coreclient.NewForConfig(config)
	if err != nil {
		t.Fatal("fixture core client failed")
	}
	apps, err := appsclient.NewForConfig(config)
	if err != nil {
		t.Fatal("fixture workload client failed")
	}
	rbac, err := rbacclient.NewForConfig(config)
	if err != nil {
		t.Fatal("fixture permission client failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var nonce [6]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal("fixture identity failed")
	}
	namespace := "stackplore-proof-write-" + hex.EncodeToString(nonce[:])
	denied := namespace + "-denied"
	for _, name := range []string{namespace, denied} {
		created, err := core.Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal("owned namespace creation failed")
		}
		uid := created.UID
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if core.Namespaces().Delete(cleanup, name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}) != nil {
				t.Error("owned namespace UID cleanup failed")
			}
		})
	}
	tokens := map[string]string{}
	for _, name := range []string{"observer", "writer"} {
		if _, err := core.ServiceAccounts(namespace).Create(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name}, AutomountServiceAccountToken: boolPointer(false)}, metav1.CreateOptions{}); err != nil {
			t.Fatal("fixture account creation failed")
		}
		verbs := []string{"get"}
		if name == "writer" {
			verbs = append(verbs, "patch")
		}
		if _, err := rbac.Roles(namespace).Create(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: name}, Rules: []rbacv1.PolicyRule{{APIGroups: []string{"apps"}, Resources: []string{"deployments", "statefulsets", "daemonsets"}, Verbs: verbs}}}, metav1.CreateOptions{}); err != nil {
			t.Fatal("fixture role creation failed")
		}
		if _, err := rbac.RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name}, RoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: name}, Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: name, Namespace: namespace}}}, metav1.CreateOptions{}); err != nil {
			t.Fatal("fixture binding creation failed")
		}
		expiration := int64(600)
		credential, err := core.ServiceAccounts(namespace).CreateToken(ctx, name, &authenticationv1.TokenRequest{Spec: authenticationv1.TokenRequestSpec{ExpirationSeconds: &expiration}}, metav1.CreateOptions{})
		if err != nil || credential.Status.Token == "" {
			t.Fatal("fixture scoped credential failed")
		}
		tokens[name] = credential.Status.Token
	}
	// No fixture Pods can schedule, so this proof establishes API acceptance only.
	labels := map[string]string{"app": "guarded-proof"}
	template := corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}, Spec: corev1.PodSpec{AutomountServiceAccountToken: boolPointer(false), NodeSelector: map[string]string{"stackplore-proof.invalid/disposable": "unscheduled"}, Containers: []corev1.Container{{Name: "probe", Image: "busybox:1.36.1@sha256:73aaf090f3d85aa34ee199857f03fa3a95c8ede2ffd4cc2cdb5b94e566b11662"}}}}
	zero := int32(0)
	selector := &metav1.LabelSelector{MatchLabels: labels}
	deployment, err := apps.Deployments(namespace).Create(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "deployment"}, Spec: appsv1.DeploymentSpec{Replicas: &zero, Selector: selector, Template: template}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal("fixture Deployment creation failed")
	}
	stateful, err := apps.StatefulSets(namespace).Create(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "stateful"}, Spec: appsv1.StatefulSetSpec{ServiceName: "proof", Replicas: &zero, Selector: selector, Template: template}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal("fixture StatefulSet creation failed")
	}
	daemon, err := apps.DaemonSets(namespace).Create(ctx, &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "daemon"}, Spec: appsv1.DaemonSetSpec{Selector: selector, Template: template}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal("fixture DaemonSet creation failed")
	}
	base, err := rest.TransportFor(&rest.Config{Host: config.Host, TLSClientConfig: rest.TLSClientConfig{CAData: config.CAData, CAFile: config.CAFile, ServerName: config.ServerName}, Timeout: 10 * time.Second, Proxy: config.Proxy})
	if err != nil {
		t.Fatal("fixture scoped TLS failed")
	}
	makeWriter := func(token string, drop bool) (*SDKWriter, *writeProofTransport) {
		transport := &writeProofTransport{kindObservationTransport: kindObservationTransport{base: base, endpoint: endpoint}, drop: drop}
		writer, err := NewSDKWriter(WriterOptions{Client: &http.Client{Transport: transport, Timeout: 10 * time.Second}, BearerToken: token, Namespaces: []string{namespace, denied}, MaximumResponse: 256 * 1024})
		if err != nil {
			t.Fatal("guarded SDK initialization failed")
		}
		t.Cleanup(func() { _ = writer.Close() })
		return writer, transport
	}
	writer, transport := makeWriter(tokens["writer"], false)
	objectFor := func(kind string) metav1.Object {
		var object metav1.Object
		switch kind {
		case "Deployment":
			item, err := apps.Deployments(namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal("fixture target read failed")
			}
			object = item
		case "StatefulSet":
			item, err := apps.StatefulSets(namespace).Get(ctx, stateful.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal("fixture target read failed")
			}
			object = item
		default:
			item, err := apps.DaemonSets(namespace).Get(ctx, daemon.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal("fixture target read failed")
			}
			object = item
		}
		return object
	}
	operationFor := func(kind, id string) WorkloadOperation {
		object := objectFor(kind)
		return WorkloadOperation{Kind: kind, Namespace: namespace, Name: object.GetName(), ExpectedUID: string(object.GetUID()), ResourceVersion: object.GetResourceVersion(), OperationID: id}
	}
	for index, kind := range []string{"Deployment", "StatefulSet", "DaemonSet"} {
		var op WorkloadOperation
		var version string
		for attempts := 0; attempts < 5; attempts++ {
			op = operationFor(kind, "operation_restart_"+string(rune('a'+index)))
			version, err = writer.RestartWorkload(ctx, op)
			if !errors.Is(err, ErrVersionConflict) {
				break
			}
		}
		if err != nil || version == op.ResourceVersion {
			t.Fatalf("real %s restart CAS failed", kind)
		}
		if got, found, err := writer.ReconcileWorkload(ctx, op); err != nil || !found || got == "" {
			t.Fatal("real restart receipt was not readable")
		}
	}
	for index, kind := range []string{"Deployment", "StatefulSet"} {
		op := operationFor(kind, "operation_scale_"+string(rune('a'+index)))
		replicas := 3
		op.RequestedReplicas = &replicas
		if _, err := writer.ScaleWorkload(ctx, op); err != nil {
			t.Fatalf("real %s scale failed", kind)
		}
		if _, found, err := writer.ReconcileWorkload(ctx, op); err != nil || !found {
			t.Fatal("real scale receipt missing from base object")
		}
	}
	before := transport.patchCount()
	op := operationFor("DaemonSet", "operation_no_scale")
	replicas := 3
	op.RequestedReplicas = &replicas
	if _, err := writer.ScaleWorkload(ctx, op); !errors.Is(err, ErrUnsupported) || transport.patchCount() != before {
		t.Fatal("real DaemonSet scale reached PATCH")
	}
	op = operationFor("Deployment", "operation_stale")
	op.ResourceVersion = "1"
	if _, err := writer.RestartWorkload(ctx, op); !errors.Is(err, ErrVersionConflict) || transport.patchCount() != before {
		t.Fatal("stale version reached real PATCH")
	}
	op = operationFor("Deployment", "operation_wrong_uid")
	op.ExpectedUID = "replacement-uid"
	if _, err := writer.RestartWorkload(ctx, op); !errors.Is(err, ErrVersionConflict) || transport.patchCount() != before {
		t.Fatal("wrong UID reached real PATCH")
	}
	op = operationFor("Deployment", "operation_denied_scope")
	op.Namespace = denied
	if _, err := writer.RestartWorkload(ctx, op); !errors.Is(err, ErrPermissionDenied) || transport.patchCount() != before {
		t.Fatal("writer escaped namespace permission")
	}
	observer, observerTransport := makeWriter(tokens["observer"], false)
	op = operationFor("Deployment", "operation_observer_forbidden")
	if _, err := observer.RestartWorkload(ctx, op); !errors.Is(err, ErrPermissionDenied) || observerTransport.patchCount() != 1 {
		t.Fatal("observation credential acquired write permission")
	}
	lost, lostTransport := makeWriter(tokens["writer"], true)
	op = operationFor("Deployment", "operation_lost_reply")
	if _, err := lost.RestartWorkload(ctx, op); !errors.Is(err, domain.ErrOperationOutcomeUncertain) || lostTransport.patchCount() != 1 {
		t.Fatal("lost real response was retried or treated as definite failure")
	}
	if _, found, err := lost.ReconcileWorkload(ctx, op); err != nil || !found || lostTransport.patchCount() != 1 {
		t.Fatal("real lost response not attributed through own receipt")
	}
	op.OperationID = "operation_coincidental"
	if _, found, err := lost.ReconcileWorkload(ctx, op); err != nil || found {
		t.Fatal("matching observed state falsely attributed to operation")
	}
	transport.assertWriterPaths(t, namespace, denied)
	observerTransport.assertWriterPaths(t, namespace, denied)
	lostTransport.assertWriterPaths(t, namespace, denied)
}

type writeProofTransport struct {
	kindObservationTransport
	lock    sync.Mutex
	patches int
	drop    bool
}

func (transport *writeProofTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.lock.Lock()
	drop := false
	if request.Method == http.MethodPatch {
		transport.patches++
		drop = transport.drop
		transport.drop = false
	}
	transport.lock.Unlock()
	response, err := transport.kindObservationTransport.RoundTrip(request)
	if drop && err == nil && response != nil {
		_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 256*1024))
		response.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		return nil, io.ErrUnexpectedEOF
	}
	return response, err
}
func (transport *writeProofTransport) patchCount() int {
	transport.lock.Lock()
	defer transport.lock.Unlock()
	return transport.patches
}
func (transport *writeProofTransport) assertWriterPaths(t *testing.T, allowed, denied string) {
	t.Helper()
	transport.mutex.Lock()
	defer transport.mutex.Unlock()
	guard := boundedTransport{writer: true, namespaces: []string{allowed, denied}}
	for _, request := range transport.requests {
		method, path, _ := strings.Cut(request, " ")
		if !guard.permits(method, path) {
			t.Fatal("real writer escaped closed method/path set")
		}
	}
}
