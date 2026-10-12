package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/testpostgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
	"github.com/jackc/pgx/v5"
)

func openIntegrationStore(t *testing.T) (*Store, Options, *pgx.Conn) {
	t.Helper()
	connectionString := os.Getenv(integrationDatabaseURLKey)
	if connectionString == "" {
		t.Skip("real PostgreSQL URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := testpostgres.OpenGate(ctx, connectionString)
	if err != nil {
		t.Fatal("open isolated PostgreSQL fixture gate")
	}
	schema := uniqueTestSchema(t)
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		_ = admin.Close(ctx)
		t.Fatal("create isolated PostgreSQL fixture schema")
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupContext, "DROP SCHEMA "+identifier+" CASCADE")
		_ = admin.Close(cleanupContext)
	})
	options := validTestOptions()
	options.ConnectionString = connectionString
	options.Schema = schema
	store, err := Open(ctx, options, postgresmigrations.FS)
	if err != nil {
		t.Fatal("open isolated PostgreSQL fixture")
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, options, admin
}
