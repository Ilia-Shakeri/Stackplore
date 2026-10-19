package postgresops

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/postgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/testpostgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
	"github.com/jackc/pgx/v5"
)

func TestAssistantNativeRestorePreservesUnknownUsageAndFencesPaidGeneration(t *testing.T) {
	if os.Getenv("STACKPLORE_TEST_NATIVE_POSTGRES_TOOLS") != "true" {
		t.Skip("native PostgreSQL client proof is not enabled")
	}
	for _, tool := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatal("matching native PostgreSQL tools required")
		}
	}
	connectionString := os.Getenv("STACKPLORE_TEST_POSTGRES_URL")
	if connectionString == "" {
		t.Fatal("owned PostgreSQL fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := testpostgres.OpenGate(ctx, connectionString)
	if err != nil {
		t.Fatal("open native assistant fixture gate")
	}
	defer admin.Close(ctx)
	names := make([]string, 0, 2)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 6*time.Minute)
		defer stop()
		for i := len(names) - 1; i >= 0; i-- {
			if err := removeOwnedNativeFixture(cleanup, connectionString, names[i]); err != nil {
				t.Error(err)
			}
		}
	})
	settings := make([]config.Config, 0, 2)
	for _, kind := range []string{"source", "restore"} {
		name := fmt.Sprintf("stackplore_assistant_native_%s_%d", kind, time.Now().UnixNano())
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Fatal("create owned assistant fixture database")
		}
		names = append(names, name)
		parsed, err := url.Parse(connectionString)
		if err != nil {
			t.Fatal("parse owned fixture")
		}
		parsed.Path = "/" + name
		parsed.RawPath = ""
		s, err := config.Load(mapGetter{"STACKPLORE_DATABASE_URL": parsed.String()})
		if err != nil {
			t.Fatal("load owned fixture settings")
		}
		settings = append(settings, s)
	}
	store, err := postgres.Open(ctx, postgres.Options{ConnectionString: settings[0].DatabaseURL.Reveal(), Schema: "public", MinimumConnections: 1, MaximumConnections: 3, ConnectTimeout: 5 * time.Second, QueryTimeout: 5 * time.Second, MigrationTimeout: 30 * time.Second, HealthTimeout: time.Second, ShutdownTimeout: 5 * time.Second, MaximumConnectionLifetime: time.Minute, MaximumConnectionIdleTime: time.Minute, AuthDigestKey: [32]byte{1}, AuthDigestKeyID: "assistant_native_key"}, postgresmigrations.FS)
	if err != nil {
		t.Fatal("initialize current-schema source")
	}
	defer store.Close()
	source, err := pgx.Connect(ctx, settings[0].DatabaseURL.Reveal())
	if err != nil {
		t.Fatal("connect owned source")
	}
	_, err = source.Exec(ctx, `UPDATE assistant_control SET enabled=TRUE,paid_enabled=TRUE,profile_paid=TRUE,paid_generation=0;
UPDATE assistant_usage_accounts SET reserved_units=100 WHERE account_key='installation';
INSERT INTO assistant_usage_ledger(reservation_id,reserved_units,state,restore_generation,created_at_us,updated_at_us)
VALUES('native_unknown',100,'unknown',0,1,1);`)
	_ = source.Close(ctx)
	if err != nil {
		t.Fatal("seed unknown usage fixture")
	}
	archive := filepath.Join(t.TempDir(), "assistant-runtime.dump")
	if err := Backup(ctx, Options{Settings: settings[0], Path: archive}); err != nil {
		t.Fatal("native current-schema backup failed")
	}
	if err := Restore(ctx, Options{Settings: settings[1], Path: archive}); err != nil {
		t.Fatal("native current-schema restore failed")
	}
	restored, err := pgx.Connect(ctx, settings[1].DatabaseURL.Reveal())
	if err != nil {
		t.Fatal("connect restored assistant fixture")
	}
	defer restored.Close(ctx)
	var schema, generation, paidGeneration, held int64
	var unknown int
	err = restored.QueryRow(ctx, `SELECT (SELECT max(version) FROM schema_migrations),
(SELECT restore_generation FROM installation_metadata WHERE singleton_id=1),
(SELECT paid_generation FROM assistant_control WHERE singleton_id=1),
(SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation'),
(SELECT count(*) FROM assistant_usage_ledger WHERE state='unknown')`).Scan(&schema, &generation, &paidGeneration, &held, &unknown)
	if err != nil || schema != 18 || generation != 1 || paidGeneration != 0 || held != 100 || unknown != 1 {
		t.Fatal("native restore lost unknown usage or paid-generation fence")
	}
}
