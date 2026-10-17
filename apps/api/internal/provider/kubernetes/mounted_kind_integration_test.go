package kubernetes

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

// This helper receives only the fixture's namespace-bound observation credential,
// never the administrator certificate used to create disposable test resources.
func verifyMountedSDKOnOwnedKind(t *testing.T, ctx context.Context, origin string, ca []byte, caFile, token, allowed, denied string) {
	t.Helper()
	directory := t.TempDir()
	if len(ca) == 0 {
		var err error
		ca, err = os.ReadFile(caFile)
		if err != nil {
			t.Fatal("fixture CA read failed")
		}
	}
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			t.Fatal("fixture mounted file write failed")
		}
	}
	write("ca.crt", ca)
	write("observer-token", []byte(token))
	configuration := clientcmdv1.Config{Kind: "Config", APIVersion: "v1", Clusters: []clientcmdv1.NamedCluster{{Name: "owned-kind", Cluster: clientcmdv1.Cluster{Server: origin, CertificateAuthority: "ca.crt"}}}, AuthInfos: []clientcmdv1.NamedAuthInfo{{Name: "observer", AuthInfo: clientcmdv1.AuthInfo{TokenFile: "observer-token"}}}, Contexts: []clientcmdv1.NamedContext{{Name: "owned-kind-observer", Context: clientcmdv1.Context{Cluster: "owned-kind", AuthInfo: "observer"}}}, CurrentContext: "owned-kind-observer"}
	encoded, err := json.Marshal(configuration)
	if err != nil {
		t.Fatal("fixture profile encode failed")
	}
	write("config.json", encoded)
	collector, err := NewMountedKubeconfigCollector(ctx, MountedKubeconfigOptions{File: filepath.Join(directory, "config.json"), Roots: []string{directory}, Collector: APICollectorOptions{Client: &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone(), Timeout: 10 * time.Second}, Namespaces: []string{allowed, denied}, MaximumItems: 1000, MaximumResponse: 4 << 20}})
	if err != nil {
		t.Fatal("real mounted profile initialization failed")
	}
	defer collector.Close()
	provider, err := New("source_kind_mounted_proof", "Disposable mounted profile", collector)
	if err != nil {
		t.Fatal("mounted provider initialization failed")
	}
	capture, err := provider.CaptureInventory(ctx)
	if err != nil || capture.ValidateFor(providerRefForMountedProof(provider)) != nil {
		t.Fatal("real mounted profile capture failed")
	}
	assertKindPartitions(t, capture, allowed, denied)
	found := false
	for _, resource := range capture.Resources {
		if resource.Resource.Ref.Identity.Kind == "kubernetes.pod" && resource.Resource.Ref.Identity.Scope.Name == allowed {
			found = true
		}
		if resource.Resource.Ref.Identity.Scope.Name == denied {
			t.Fatal("denied mounted-profile inventory leaked")
		}
	}
	if !found {
		t.Fatal("real mounted profile Pod absent")
	}
}

func providerRefForMountedProof(provider *Provider) domain.ProviderRef {
	return domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: provider.Source().ID}
}
