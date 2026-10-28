package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/postgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/testpostgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
	"github.com/jackc/pgx/v5"
)

// This is the real application composition and database, with no admitted
// question. The loopback protocol endpoint is synthetic and must receive no I/O.
func TestAssistantProductRealSQLRestartKeepsControlAndIdlePrivacy(t *testing.T) {
	connectionString := os.Getenv("STACKPLORE_TEST_POSTGRES_URL")
	if connectionString == "" {
		t.Skip("real PostgreSQL URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := testpostgres.OpenGate(ctx, connectionString)
	if err != nil {
		t.Fatal("open isolated product fixture gate")
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		_ = admin.Close(ctx)
		t.Fatal("create owned fixture identity")
	}
	schema := "stackplore_product_test_" + hex.EncodeToString(suffix[:])
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		_ = admin.Close(ctx)
		t.Fatal("create isolated product schema")
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		_, _ = admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE")
		_ = admin.Close(cleanup)
	})
	var requests atomic.Int32
	processor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer processor.Close()
	var key [32]byte
	for index := range key {
		key[index] = byte(index + 1)
	}
	defer clear(key[:])
	options := postgres.Options{ConnectionString: connectionString, Schema: schema, MinimumConnections: 2, MaximumConnections: 16, ConnectTimeout: 5 * time.Second, QueryTimeout: 3 * time.Second, MigrationTimeout: time.Minute, HealthTimeout: time.Second, ShutdownTimeout: 5 * time.Second, MaximumConnectionLifetime: 30 * time.Minute, MaximumConnectionIdleTime: time.Minute, AuthDigestKey: key, AuthDigestKeyID: "product_fixture_key"}
	var profileID string
	for range 2 {
		func() {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal("bind owned application listener")
			}
			defer listener.Close()
			settings := assistantEnabledSettings(t, processor.URL)
			settings.HTTPAddress = listener.Addr().String()
			origin, err := url.Parse("http://" + listener.Addr().String())
			if err != nil {
				t.Fatal("construct owned application origin")
			}
			settings.PublicOrigin = *origin
			application, stop := context.WithCancel(ctx)
			defer stop()
			done := make(chan struct{})
			var applicationError error
			var store *postgres.Store
			go func() {
				defer close(done)
				applicationError = runWithStorage(application, settings, discardLogger(t), func(string, string) (net.Listener, error) { return listener, nil }, func(openContext context.Context, _ config.Config) (databaseRuntime, error) {
					var openError error
					store, openError = postgres.Open(openContext, options, postgresmigrations.FS)
					return store, openError
				})
			}()
			defer func() {
				stop()
				_ = listener.Close()
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					t.Error("owned application fixture did not join")
				}
			}()
			client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: time.Second}
			defer client.CloseIdleConnections()
			ready := false
			deadline := time.NewTimer(20 * time.Second)
			defer deadline.Stop()
			for !ready {
				select {
				case <-done:
					t.Fatal("enabled application startup failed")
				case <-deadline.C:
					t.Fatal("enabled application did not become ready")
				default:
				}
				response, requestError := client.Get(origin.String() + "/readyz")
				if requestError == nil {
					ready = response.StatusCode == http.StatusOK
					_ = response.Body.Close()
				}
				if !ready {
					select {
					case <-ctx.Done():
						t.Fatal("owned application context expired")
					case <-time.After(20 * time.Millisecond):
					}
				}
			}
			stop()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("enabled application did not join and close storage")
			}
			if applicationError != nil || store == nil || store.Health(ctx) == nil {
				t.Fatal("real application shutdown left open storage or a failure")
			}
		}()
		var version, userQuota, installationQuota int64
		var enabled bool
		var currentProfile string
		query := "SELECT row_version,enabled,profile_id,user_quota,installation_quota FROM " + pgx.Identifier{schema, "assistant_control"}.Sanitize() + " WHERE singleton_id=1"
		if err := admin.QueryRow(ctx, query).Scan(&version, &enabled, &currentProfile, &userQuota, &installationQuota); err != nil || version != 2 || !enabled || !strings.HasPrefix(currentProfile, "local_") || userQuota != 1000000 || installationQuota != 10000000 {
			t.Fatal("real startup changed control or quota semantics")
		}
		if profileID != "" && currentProfile != profileID {
			t.Fatal("restart changed pinned profile binding")
		}
		profileID = currentProfile
	}
	// A listener failure occurs after helper creation. It must still join the
	// new worker and close the real store without requiring HTTP shutdown.
	var failedStore *postgres.Store
	failed := runWithStorage(ctx, assistantEnabledSettings(t, processor.URL), discardLogger(t), func(string, string) (net.Listener, error) { return nil, errors.New("owned listener failure") }, func(openContext context.Context, _ config.Config) (databaseRuntime, error) {
		var openError error
		failedStore, openError = postgres.Open(openContext, options, postgresmigrations.FS)
		return failedStore, openError
	})
	if !errors.Is(failed, errListen) || failedStore == nil || failedStore.Health(ctx) == nil {
		t.Fatal("startup failure left the helper or real store open")
	}
	for _, table := range []string{"assistant_runs", "assistant_steps", "assistant_usage_ledger"} {
		var count int
		if err := admin.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{schema, table}.Sanitize()).Scan(&count); err != nil || count != 0 {
			t.Fatal("idle application created helper work or usage state")
		}
	}
	var accounts, installation int
	var reserved, used int64
	if err := admin.QueryRow(ctx, "SELECT count(*),count(*) FILTER (WHERE account_key='installation'),COALESCE(sum(reserved_units),0),COALESCE(sum(used_units),0) FROM "+pgx.Identifier{schema, "assistant_usage_accounts"}.Sanitize()).Scan(&accounts, &installation, &reserved, &used); err != nil || accounts != 1 || installation != 1 || reserved != 0 || used != 0 {
		t.Fatal("idle application changed the migration's empty installation account")
	}
	var auditCount int
	if err := admin.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{schema, "audit_entries"}.Sanitize()+" WHERE action='assistant.configure'").Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatal("restart repeated configuration audit or initial configuration was absent")
	}
	if requests.Load() != 0 {
		t.Fatal("idle enabled application contacted processor")
	}
}
