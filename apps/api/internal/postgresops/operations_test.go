package postgresops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
)

type mapGetter map[string]string

func (values mapGetter) LookupEnv(key string) (string, bool) {
	value, ok := values[key]
	return value, ok
}

func (values mapGetter) Environ() []string {
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

func TestConnectionKeepsPasswordOutOfArguments(t *testing.T) {
	const secret = "hidden-password"
	connection, err := parseConnection("postgresql://operator:" + secret + "@db.example:5544/stackplore?sslmode=verify-full&application_name=ops")
	if err != nil {
		t.Fatalf("parseConnection() error = %v", err)
	}
	arguments := strings.Join(connection.args(), " ")
	if strings.Contains(arguments, secret) || strings.Contains(arguments, "postgresql://") {
		t.Fatalf("arguments disclose connection secret: %q", arguments)
	}
	if !slices.Contains(connection.environment(), "PGPASSWORD="+secret) ||
		!slices.Contains(connection.environment(), "PGSSLMODE=verify-full") {
		t.Fatal("child environment lacks exact protected connection settings")
	}
}

func TestBackupPublishesOnlyValidatedArchive(t *testing.T) {
	settings := testSettings(t)
	destination := filepath.Join(t.TempDir(), "snapshot.dump")
	originalExecute := execute
	t.Cleanup(func() { execute = originalExecute })
	var calls []string
	execute = func(_ context.Context, specification commandSpec) error {
		calls = append(calls, specification.name)
		if specification.name == "pg_dump" {
			index := slices.Index(specification.args, "--file")
			if index < 0 || index+1 >= len(specification.args) {
				t.Fatal("pg_dump file argument missing")
			}
			return os.WriteFile(specification.args[index+1], []byte("valid custom archive"), 0o600)
		}
		return nil
	}
	if err := Backup(context.Background(), Options{Settings: settings, Path: destination}); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if !slices.Equal(calls, []string{"pg_dump", "pg_restore"}) {
		t.Fatalf("tool calls = %v", calls)
	}
	contents, err := os.ReadFile(destination)
	if err != nil || string(contents) != "valid custom archive" {
		t.Fatalf("published archive = %q, %v", contents, err)
	}
}

func TestBackupFailureLeavesNoPublishedArtifact(t *testing.T) {
	settings := testSettings(t)
	destination := filepath.Join(t.TempDir(), "snapshot.dump")
	originalExecute := execute
	t.Cleanup(func() { execute = originalExecute })
	execute = func(context.Context, commandSpec) error { return errors.New("tool failed") }
	if err := Backup(context.Background(), Options{Settings: settings, Path: destination}); !errors.Is(err, ErrBackupRejected) {
		t.Fatalf("Backup() error = %v", err)
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed backup published an artifact: %v", err)
	}
}

func TestBackupHasBoundedJobAndCannotReplaceRacedDestination(t *testing.T) {
	settings := testSettings(t)
	destination := filepath.Join(t.TempDir(), "snapshot.dump")
	originalExecute := execute
	t.Cleanup(func() { execute = originalExecute })
	execute = func(ctx context.Context, specification commandSpec) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > operationTimeout {
			t.Fatal("native backup tool lacks job deadline")
		}
		if specification.name == "pg_dump" {
			index := slices.Index(specification.args, "--file")
			return os.WriteFile(specification.args[index+1], []byte("archive"), 0o600)
		}
		return os.WriteFile(destination, []byte("keep original"), 0o600)
	}
	if err := Backup(context.Background(), Options{Settings: settings, Path: destination}); !errors.Is(err, ErrBackupRejected) {
		t.Fatal("raced existing destination was replaced")
	}
	contents, err := os.ReadFile(destination)
	if err != nil || string(contents) != "keep original" {
		t.Fatal("backup overwrote a raced destination")
	}
}

func testSettings(t *testing.T) config.Config {
	t.Helper()
	settings, err := config.Load(mapGetter{
		"STACKPLORE_DATABASE_URL": "postgresql://operator:hidden-password@127.0.0.1:5432/stackplore?sslmode=disable",
	})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	return settings
}
