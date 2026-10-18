package postgresops

import (
	"context"
	"errors"
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

func TestNativeBackupRestorePreservesLiteralDatabaseNames(t *testing.T) {
	if os.Getenv("STACKPLORE_TEST_NATIVE_POSTGRES_TOOLS") != "true" {
		t.Skip("native PostgreSQL client proof is not enabled")
	}
	for _, tool := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatal("native PostgreSQL client proof requires matching installed tools")
		}
	}
	connectionString := os.Getenv("STACKPLORE_TEST_POSTGRES_URL")
	if connectionString == "" {
		t.Fatal("native PostgreSQL client proof requires an owned test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := testpostgres.OpenGate(ctx, connectionString)
	if err != nil {
		t.Fatal("open native database-name proof gate")
	}
	defer admin.Close(ctx)
	names := make([]string, 0, 2)
	t.Cleanup(func() {
		// DROP DATABASE may wait for a paced checkpoint in another fixture.
		// Release the fixture gate first and keep cleanup independent of that session.
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 6*time.Minute)
		defer cleanupCancel()
		for index := len(names) - 1; index >= 0; index-- {
			if err := removeOwnedNativeFixture(cleanupContext, connectionString, names[index]); err != nil {
				t.Error(err)
			}
		}
	})
	settings := make([]config.Config, 0, 2)
	for _, kind := range []string{"source", "restore"} {
		name := fmt.Sprintf("stackplore_native_%s_%d_%%='\\", kind, time.Now().UnixNano())
		identifier := pgx.Identifier{name}.Sanitize()
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
			t.Fatal("create owned literal-name native fixture database")
		}
		names = append(names, name)
		parsed, err := url.Parse(connectionString)
		if err != nil {
			t.Fatal("parse native fixture connection")
		}
		parsed.Path, parsed.RawPath = "/"+name, ""
		value, err := config.Load(mapGetter{"STACKPLORE_DATABASE_URL": parsed.String()})
		if err != nil {
			t.Fatal("load literal-name native fixture settings")
		}
		settings = append(settings, value)
	}
	source, err := postgres.Open(ctx, postgres.Options{
		ConnectionString: settings[0].DatabaseURL.Reveal(), Schema: "public",
		MinimumConnections: 1, MaximumConnections: 3, ConnectTimeout: 5 * time.Second,
		QueryTimeout: 5 * time.Second, MigrationTimeout: 30 * time.Second, HealthTimeout: time.Second,
		ShutdownTimeout: 5 * time.Second, MaximumConnectionLifetime: time.Minute, MaximumConnectionIdleTime: time.Minute,
		AuthDigestKey: [32]byte{1}, AuthDigestKeyID: "key_native_fixture",
	}, postgresmigrations.FS)
	if err != nil {
		t.Fatal("initialize literal-name source migrations")
	}
	defer source.Close()
	connection, err := pgx.Connect(ctx, settings[0].DatabaseURL.Reveal())
	if err != nil {
		t.Fatal("connect literal-name source fixture")
	}
	if _, err := connection.Exec(ctx, `INSERT INTO users(id,login_name,canonical_login_name,display_name,role,enabled,created_at_us,updated_at_us) VALUES('user_native_literal','native-literal','native-literal','Native literal fixture','administrator',1,1,1)`); err != nil {
		_ = connection.Close(ctx)
		t.Fatal("seed literal-name source marker")
	}
	_ = connection.Close(ctx)
	archive := filepath.Join(t.TempDir(), "literal-name.dump")
	if err := Backup(ctx, Options{Settings: settings[0], Path: archive}); err != nil {
		t.Fatal("native backup failed for the exact literal database name")
	}
	if err := Restore(ctx, Options{Settings: settings[1], Path: archive}); err != nil {
		t.Fatal("native restore failed for the exact literal database name")
	}
	restored, err := pgx.Connect(ctx, settings[1].DatabaseURL.Reveal())
	if err != nil {
		t.Fatal("connect exact literal-name restored fixture")
	}
	defer restored.Close(ctx)
	var marker string
	var generation int
	if err := restored.QueryRow(ctx, `SELECT display_name,(SELECT restore_generation FROM installation_metadata WHERE singleton_id=1) FROM users WHERE id='user_native_literal'`).Scan(&marker, &generation); err != nil || marker != "Native literal fixture" || generation != 1 {
		t.Fatal("native backup/restore changed database identity or lost the source marker")
	}
}

func removeOwnedNativeFixture(ctx context.Context, connectionString, name string) error {
	connection, err := pgx.Connect(ctx, connectionString)
	if err != nil {
		return fmt.Errorf("native fixture cleanup connection unavailable")
	}
	defer func() {
		closeContext, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer closeCancel()
		_ = connection.Close(closeContext)
	}()
	var owned bool
	err = connection.QueryRow(ctx, `SELECT datdba=(SELECT oid FROM pg_roles WHERE rolname=current_user) FROM pg_database WHERE datname=$1`, name).Scan(&owned)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("native fixture cleanup ownership query unavailable")
	}
	if !owned {
		return fmt.Errorf("native fixture database ownership changed; cleanup refused")
	}
	if _, err := connection.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("native fixture cleanup exceeded bounded checkpoint wait")
		}
		return fmt.Errorf("remove owned literal-name native fixture database failed")
	}
	var remains bool
	if err := connection.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, name).Scan(&remains); err != nil || remains {
		return fmt.Errorf("native fixture cleanup absence verification failed")
	}
	return nil
}
