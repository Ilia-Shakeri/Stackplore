package kubernetes

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

func mountedFixture(t *testing.T, handler http.Handler) (MountedKubeconfigOptions, clientcmdv1.Config, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	directory := t.TempDir()
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		t.Fatal(err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	clear(tokenBytes)
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("token", []byte(token))
	write("ca.crt", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	configuration := clientcmdv1.Config{Kind: "Config", APIVersion: "v1", Clusters: []clientcmdv1.NamedCluster{{Name: "target", Cluster: clientcmdv1.Cluster{Server: server.URL, CertificateAuthority: "ca.crt"}}}, AuthInfos: []clientcmdv1.NamedAuthInfo{{Name: "observer", AuthInfo: clientcmdv1.AuthInfo{TokenFile: "token"}}}, Contexts: []clientcmdv1.NamedContext{{Name: "chosen", Context: clientcmdv1.Context{Cluster: "target", AuthInfo: "observer"}}}, CurrentContext: "chosen"}
	encoded, err := json.Marshal(configuration)
	if err != nil {
		t.Fatal(err)
	}
	write("config.json", encoded)
	options := MountedKubeconfigOptions{File: filepath.Join(directory, "config.json"), Roots: []string{directory}, Collector: APICollectorOptions{Client: &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone(), Timeout: time.Second}, Namespaces: []string{"shop"}, MaximumItems: 100, MaximumResponse: 1 << 20}}
	return options, configuration, server
}

func writeMountedConfig(t *testing.T, options MountedKubeconfigOptions, configuration clientcmdv1.Config) {
	t.Helper()
	data, err := json.Marshal(configuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(options.File, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestMountedKubeconfigReadsOnlySelectedScopedHTTPSOrigin(t *testing.T) {
	requests := 0
	options, configuration, server := mountedFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/api/v1/namespaces/shop/pods" || r.Header.Get("Authorization") == "" {
			t.Error("missing scoped request authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[]}`))
	}))
	configuration.CurrentContext = "unselected"
	writeMountedConfig(t, options, configuration)
	options.Context = "chosen"
	collector, err := NewMountedKubeconfigCollector(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	defer collector.Close()
	if collector.baseURL != server.URL || len(collector.OperatorPolicyDigest()) != 64 {
		t.Fatal("selected operator origin not retained safely")
	}
	if collector.clusterID == server.URL || len(collector.clusterID) != 72 {
		t.Fatal("raw operator endpoint leaked into normalized identity")
	}
	if _, err := collector.core.Pods("shop").List(context.Background(), metav1.ListOptions{}); err != nil {
		t.Fatal("verified scoped request failed")
	}
	if _, err := collector.core.Pods("other").List(context.Background(), metav1.ListOptions{}); err == nil {
		t.Fatal("namespace boundary bypass")
	}
	if requests != 1 {
		t.Fatalf("scoped request count=%d", requests)
	}
}

func TestMountedKubeconfigRejectsPluginsSecretsAndAuthorityOverrides(t *testing.T) {
	for name, change := range map[string]func(*clientcmdv1.Config){
		"exec": func(c *clientcmdv1.Config) {
			c.AuthInfos[0].AuthInfo.Exec = &clientcmdv1.ExecConfig{Command: "forbidden"}
		},
		"provider plugin": func(c *clientcmdv1.Config) {
			c.AuthInfos[0].AuthInfo.AuthProvider = &clientcmdv1.AuthProviderConfig{Name: "forbidden"}
		},
		"inline token": func(c *clientcmdv1.Config) { c.AuthInfos[0].AuthInfo.Token = "forbidden-inline" },
		"inline CA": func(c *clientcmdv1.Config) {
			c.Clusters[0].Cluster.CertificateAuthorityData = []byte("forbidden-inline")
		},
		"inline key":           func(c *clientcmdv1.Config) { c.AuthInfos[0].AuthInfo.ClientKeyData = []byte("forbidden-inline") },
		"proxy":                func(c *clientcmdv1.Config) { c.Clusters[0].Cluster.ProxyURL = "https://proxy.invalid" },
		"insecure TLS":         func(c *clientcmdv1.Config) { c.Clusters[0].Cluster.InsecureSkipTLSVerify = true },
		"TLS name override":    func(c *clientcmdv1.Config) { c.Clusters[0].Cluster.TLSServerName = "other.invalid" },
		"HTTP":                 func(c *clientcmdv1.Config) { c.Clusters[0].Cluster.Server = "http://cluster.invalid" },
		"path endpoint":        func(c *clientcmdv1.Config) { c.Clusters[0].Cluster.Server += "/proxy" },
		"impersonation":        func(c *clientcmdv1.Config) { c.AuthInfos[0].AuthInfo.Impersonate = "admin" },
		"basic authentication": func(c *clientcmdv1.Config) { c.AuthInfos[0].AuthInfo.Username = "forbidden" },
		"outside file":         func(c *clientcmdv1.Config) { c.AuthInfos[0].AuthInfo.TokenFile = "../outside" },
		"duplicate context":    func(c *clientcmdv1.Config) { c.Contexts = append(c.Contexts, c.Contexts[0]) },
		"missing context":      func(c *clientcmdv1.Config) { c.CurrentContext = "missing" },
		"unused plugin": func(c *clientcmdv1.Config) {
			c.AuthInfos = append(c.AuthInfos, clientcmdv1.NamedAuthInfo{Name: "unused", AuthInfo: clientcmdv1.AuthInfo{Exec: &clientcmdv1.ExecConfig{Command: "forbidden"}}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			options, configuration, _ := mountedFixture(t, http.NotFoundHandler())
			change(&configuration)
			writeMountedConfig(t, options, configuration)
			if collector, err := NewMountedKubeconfigCollector(context.Background(), options); !errors.Is(err, ErrMountedProfile) {
				if collector != nil {
					collector.Close()
				}
				t.Fatal("unsafe mounted profile accepted")
			}
		})
	}
}

func TestMountedKubeconfigRejectsSymlinkAndDuplicateDocument(t *testing.T) {
	options, _, _ := mountedFixture(t, http.NotFoundHandler())
	original, err := os.ReadFile(options.File)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(options.File, append(original, []byte("\n---\nkind: Config\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMountedKubeconfigCollector(context.Background(), options); !errors.Is(err, ErrMountedProfile) {
		t.Fatal("multiple documents accepted")
	}
	if err := os.WriteFile(options.File, original, 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(target, []byte("generated-placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(options.Roots[0], "token")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skip("symlink creation unavailable")
	}
	if _, err := NewMountedKubeconfigCollector(context.Background(), options); !errors.Is(err, ErrMountedProfile) {
		t.Fatal("linked credential accepted")
	}
}

func TestMountedKubeconfigClientCertificateUsesMutualTLSWithoutBearer(t *testing.T) {
	requests := 0
	options, configuration, server := mountedFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.TLS == nil || len(r.TLS.PeerCertificates) != 1 || r.Header.Get("Authorization") != "" {
			t.Error("certificate authentication boundary failed")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[]}`))
	}))
	server.TLS.ClientAuth = tls.RequireAnyClientCert
	pair := server.TLS.Certificates[0]
	// The server test certificate and generated key exist only in this owned fixture.
	pkcs, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(pkcs)
	directory := options.Roots[0]
	if err := os.WriteFile(filepath.Join(directory, "client.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "client.key"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs}), 0600); err != nil {
		t.Fatal(err)
	}
	configuration.AuthInfos[0].AuthInfo = clientcmdv1.AuthInfo{ClientCertificate: "client.crt", ClientKey: "client.key"}
	writeMountedConfig(t, options, configuration)
	collector, err := NewMountedKubeconfigCollector(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	defer collector.Close()
	transport := collector.client.Transport.(*boundedTransport)
	if !transport.certificateAuth || len(transport.base.(*http.Transport).TLSClientConfig.Certificates) != 1 || transport.base.(*http.Transport).TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatal("client-certificate boundary missing")
	}
	if _, err := collector.core.Pods("shop").List(context.Background(), metav1.ListOptions{}); err != nil {
		t.Fatal("mutual TLS request failed")
	}
	if requests != 1 {
		t.Fatal("mutual TLS request missing")
	}
}
