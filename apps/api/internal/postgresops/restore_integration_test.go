package postgresops

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/testpostgres"
	"github.com/jackc/pgx/v5"
)

func TestRestoreAbortsNativeJobWhenMaintenanceSessionDies(t *testing.T) {
	connectionString := os.Getenv("STACKPLORE_TEST_POSTGRES_URL")
	if connectionString == "" {
		t.Skip("real PostgreSQL URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := testpostgres.OpenGate(ctx, connectionString)
	if err != nil {
		t.Fatal("open native restore fixture gate")
	}
	defer admin.Close(ctx)
	databaseName := fmt.Sprintf("stackplore_restore_test_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{databaseName}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal("create disposable restore database")
	}
	defer func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupContext, "DROP DATABASE "+identifier+" WITH (FORCE)")
	}()
	parsed, err := url.Parse(connectionString)
	if err != nil {
		t.Fatal("parse native restore fixture connection")
	}
	parsed.Path = "/" + databaseName
	settings, err := config.Load(mapGetter{"STACKPLORE_DATABASE_URL": parsed.String()})
	if err != nil {
		t.Fatal("load native restore fixture settings")
	}
	archive := filepath.Join(t.TempDir(), "restore.dump")
	if err := os.WriteFile(archive, []byte("fixture archive"), 0o600); err != nil {
		t.Fatal("create native restore fixture archive")
	}
	originalExecute := execute
	defer func() { execute = originalExecute }()
	invoked := false
	execute = func(jobContext context.Context, specification commandSpec) error {
		if slices.Contains(specification.args, "--list") {
			return nil
		}
		invoked = true
		if !slices.Contains(specification.args, "--schema=public") {
			t.Fatal("native restore scope exceeds public application schema")
		}
		var terminated int64
		if err := admin.QueryRow(ctx, `SELECT count(*) FROM (
    SELECT pg_terminate_backend(pid) FROM pg_stat_activity
    WHERE datname = $1 AND application_name = 'stackplore-maintenance'
) AS killed`, databaseName).Scan(&terminated); err != nil || terminated != 1 {
			t.Fatal("terminate native restore ownership session")
		}
		select {
		case <-jobContext.Done():
			return jobContext.Err()
		case <-time.After(3 * time.Second):
			t.Fatal("native restore job continued after ownership loss")
			return errors.New("fixture job did not cancel")
		}
	}
	if err := Restore(ctx, Options{Settings: settings, Path: archive}); !invoked || !errors.Is(err, ErrRestoreRejected) {
		t.Fatal("native restore did not abort after ownership loss")
	}
}
