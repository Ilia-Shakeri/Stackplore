package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/postgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/testpostgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
	"github.com/jackc/pgx/v5"
)

func TestMountedKubernetesProfileStartsFullServerWithRealPostgreSQL(t *testing.T) {
	connectionString := os.Getenv("STACKPLORE_TEST_POSTGRES_URL")
	if connectionString == "" {
		t.Skip("real PostgreSQL URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal("create fixture identity")
	}
	var tokenBytes [32]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		t.Fatal("create isolated observer token")
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes[:])
	clear(tokenBytes[:])
	var unsafeRequest atomic.Bool
	readObserved := make(chan struct{})
	var readOnce sync.Once
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer "+token || !strings.Contains(r.URL.Path, "/namespaces/shop/") || strings.Contains(r.URL.Path, "/secrets") || strings.Contains(r.URL.Path, "/configmaps") {
			unsafeRequest.Store(true)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		readOnce.Do(func() { close(readObserved) })
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"List","metadata":{"resourceVersion":"1"},"items":[]}`))
	}))
	defer api.Close()
	root := t.TempDir()
	writeProfileFile := func(name string, content []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), content, 0600); err != nil {
			t.Fatal("write operator-owned fixture profile")
		}
	}
	writeProfileFile("ca.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: api.Certificate().Raw}))
	writeProfileFile("token", []byte(token))
	profile, err := json.Marshal(map[string]any{
		"apiVersion": "v1", "kind": "Config", "current-context": "owned",
		"clusters": []any{map[string]any{"name": "owned", "cluster": map[string]any{"server": api.URL, "certificate-authority": "ca.pem"}}},
		"users":    []any{map[string]any{"name": "observer", "user": map[string]any{"tokenFile": "token"}}},
		"contexts": []any{map[string]any{"name": "owned", "context": map[string]any{"cluster": "owned", "user": "observer"}}},
	})
	if err != nil {
		t.Fatal("encode operator-owned fixture profile")
	}
	writeProfileFile("config.json", profile)
	settings, err := config.Load(mapGetter{
		"STACKPLORE_DATA_DIRECTORY":             t.TempDir(),
		"STACKPLORE_KUBERNETES_ENABLED":         "true",
		"STACKPLORE_KUBERNETES_NAMESPACES":      "shop",
		"STACKPLORE_KUBERNETES_ACCESS_PROFILE":  config.KubernetesProfileKubeconfig,
		"STACKPLORE_KUBERNETES_KUBECONFIG_FILE": filepath.Join(root, "config.json"),
		"STACKPLORE_KUBERNETES_REFERENCE_ROOTS": root,
	})
	if err != nil {
		t.Fatalf("load mounted reader deployment configuration: %v", err)
	}
	admin, err := testpostgres.OpenGate(ctx, connectionString)
	if err != nil {
		t.Fatal("open isolated server PostgreSQL gate")
	}
	schema := "mounted_server_" + hex.EncodeToString(nonce[:])
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		_ = admin.Close(ctx)
		t.Fatal("create isolated server PostgreSQL schema")
	}
	defer func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupContext, "DROP SCHEMA "+identifier+" CASCADE")
		_ = admin.Close(cleanupContext)
	}()
	var digestKey [32]byte
	if _, err := rand.Read(digestKey[:]); err != nil {
		t.Fatal("create isolated server digest key")
	}
	defer clear(digestKey[:])
	store, err := postgres.Open(ctx, postgres.Options{ConnectionString: connectionString, Schema: schema, MinimumConnections: 2, MaximumConnections: 16, ConnectTimeout: 5 * time.Second, QueryTimeout: 10 * time.Second, MigrationTimeout: time.Minute, HealthTimeout: 2 * time.Second, ShutdownTimeout: 5 * time.Second, MaximumConnectionLifetime: 30 * time.Minute, MaximumConnectionIdleTime: 5 * time.Minute, AuthDigestKey: digestKey, AuthDigestKeyID: "test_mounted_server"}, postgresmigrations.FS)
	if err != nil {
		t.Fatal("open isolated server PostgreSQL store")
	}
	defer store.Close()
	if err := store.ConfigureDeployment(ctx, postgres.DeploymentProfile{DataRoot: settings.DataDirectory, FixtureVersion: demo.FixtureVersion}); err != nil {
		t.Fatal("configure isolated normal deployment")
	}
	serviceContext, stopService := context.WithCancel(ctx)
	defer stopService()
	listening := make(chan string, 1)
	done := make(chan struct{})
	var runtimeErr error
	go func() {
		defer close(done)
		runtimeErr = runWithStorage(serviceContext, settings, discardLogger(t), func(string, string) (net.Listener, error) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err == nil {
				listening <- listener.Addr().String()
			}
			return listener, err
		}, func(context.Context, config.Config) (databaseRuntime, error) { return store, nil })
	}()
	defer func() {
		stopService()
		select {
		case <-done:
			if runtimeErr != nil {
				t.Error("mounted profile server did not shut down cleanly")
			}
			if unsafeRequest.Load() {
				t.Error("mounted profile escaped its namespace or attempted forbidden reads")
			}
		case <-time.After(10 * time.Second):
			t.Error("mounted profile server did not release bounded work")
		}
	}()
	var address string
	select {
	case address = <-listening:
	case <-done:
		t.Fatal("mounted profile rejected complete server composition before listening")
	case <-ctx.Done():
		t.Fatal("mounted profile startup exceeded fixture deadline")
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/readyz", nil)
	if err != nil {
		t.Fatal("create readiness request")
	}
	request.Host = settings.PublicOrigin.Host
	response, err := client.Do(request)
	if err != nil {
		t.Fatal("mounted profile application did not answer readiness")
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal("mounted profile application was not ready")
	}
	configuration, err := store.GetSourceConfiguration(ctx, "source_kubernetes")
	if err != nil || configuration.ProfileID != domain.SourceProfileKubernetesKubeconfig || !configuration.Enabled {
		t.Fatal("mounted profile registration did not survive server composition")
	}
	select {
	case <-readObserved:
	case <-done:
		t.Fatal("mounted profile stopped before its first scoped TLS read")
	case <-time.After(10 * time.Second):
		t.Fatal("mounted profile collection did not reach its configured TLS origin")
	}
	if unsafeRequest.Load() {
		t.Fatal("mounted profile escaped its namespace or attempted forbidden reads")
	}
}
