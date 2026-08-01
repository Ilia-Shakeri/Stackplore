package storage

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

const ownerLockChildEnvironment = "STACKPLORE_OWNER_LOCK_CHILD"

func TestOpenAppliesEmbeddedMigrationsAndConnectionPolicy(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	options := Options{
		DataDirectory: filepath.Join(t.TempDir(), "data #%"),
		BusyTimeout:   750 * time.Millisecond,
	}

	store, err := Open(ctx, options)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if err := store.Health(ctx); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if got := store.db.Stats().MaxOpenConnections; got != maximumOpenConnections {
		t.Fatalf("MaxOpenConnections = %d, want %d", got, maximumOpenConnections)
	}

	connections := make([]connectionPolicy, 0, maximumOpenConnections)
	for index := 0; index < maximumOpenConnections; index++ {
		connection, err := store.db.Conn(ctx)
		if err != nil {
			t.Fatalf("Conn(%d) error = %v", index, err)
		}
		connections = append(connections, connectionPolicy{connection: connection})
	}
	for index := range connections {
		connections[index].verify(t, ctx, options.BusyTimeout)
	}
	for index := range connections {
		if err := connections[index].connection.Close(); err != nil {
			t.Fatalf("connection Close(%d) error = %v", index, err)
		}
	}

	var appID int64
	if err := store.db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&appID); err != nil || appID != applicationID {
		t.Fatalf("application_id = %d, error = %v", appID, err)
	}
	var migrationCount int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&migrationCount); err != nil || migrationCount < 1 {
		t.Fatalf("migration count = %d, error = %v", migrationCount, err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !errors.Is(store.Health(ctx), ErrStoreClosed) {
		t.Fatalf("Health() after close = %v", store.Health(ctx))
	}
	if _, err := os.Lstat(filepath.Join(options.DataDirectory, ownerLockFilename)); err != nil {
		t.Fatalf("owner lock removed: %v", err)
	}

	reopened, err := Open(ctx, options)
	if err != nil {
		t.Fatalf("idempotent Open() error = %v", err)
	}
	if err := reopened.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&migrationCount); err != nil || migrationCount < 1 {
		t.Fatalf("idempotent migration count = %d, error = %v", migrationCount, err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("reopened Close() error = %v", err)
	}
}

type connectionPolicy struct {
	connection interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
		Close() error
	}
}

func (policy connectionPolicy) verify(t *testing.T, ctx context.Context, busyTimeout time.Duration) {
	t.Helper()
	var journalMode string
	if err := policy.connection.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil || journalMode != "wal" {
		t.Fatalf("journal_mode = %q, error = %v", journalMode, err)
	}
	var foreignKeys int
	if err := policy.connection.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, error = %v", foreignKeys, err)
	}
	var timeout int64
	if err := policy.connection.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeout); err != nil || timeout != busyTimeout.Milliseconds() {
		t.Fatalf("busy_timeout = %d, error = %v", timeout, err)
	}
	var synchronous int
	if err := policy.connection.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil || synchronous != 2 {
		t.Fatalf("synchronous = %d, error = %v", synchronous, err)
	}
}

func TestDatabaseDSNEscapesFixedPath(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "db #%", databaseFilename)
	dsn := databaseDSN(path, time.Second)
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	decodedPath := filepath.FromSlash(parsed.Path)
	if runtime.GOOS == "windows" {
		decodedPath = strings.TrimPrefix(decodedPath, string(os.PathSeparator))
	}
	if decodedPath != path {
		t.Fatalf("decoded path = %q, want %q", decodedPath, path)
	}
	if parsed.Query().Get("mode") != "rwc" || parsed.Query().Get("_foreign_keys") != "on" ||
		parsed.Query().Get("_synchronous") != "full" {
		t.Fatalf("DSN policy missing: %q", dsn)
	}
	if strings.Contains(parsed.RawPath, "#") || strings.Contains(parsed.RawQuery, "#") {
		t.Fatalf("DSN contains unescaped delimiter: %q", dsn)
	}
}

func TestHealthReadsDatabaseIdentityAndFullCurrentLedger(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	options := Options{DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: time.Second}
	store, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if len(store.expectedMigrations) < 2 {
		t.Fatal("test needs more than one migration")
	}
	if err := store.Health(ctx); err != nil {
		t.Fatalf("initial Health() error = %v", err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE schema_migrations SET checksum_sha256 = ? WHERE version = 1", strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	if err := store.Health(ctx); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("tampered Health() error = %v", err)
	}
}

func TestOpenRejectsUnsafePathsAndOptions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	files := oneTableMigration()
	root := filepath.VolumeName(t.TempDir()) + string(os.PathSeparator)
	for name, options := range map[string]Options{
		"empty":       {BusyTimeout: time.Second},
		"dot":         {DataDirectory: ".", BusyTimeout: time.Second},
		"traversal":   {DataDirectory: filepath.Join("..", "escape"), BusyTimeout: time.Second},
		"root":        {DataDirectory: root, BusyTimeout: time.Second},
		"short_busy":  {DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: time.Millisecond},
		"fraction_ms": {DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: 100*time.Millisecond + time.Nanosecond},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := openWithMigrations(ctx, options, files); err == nil {
				t.Fatal("Open() succeeded")
			}
		})
	}

	filePath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openWithMigrations(ctx, Options{DataDirectory: filePath, BusyTimeout: time.Second}, files); !errors.Is(err, ErrUnsafeDataDirectory) {
		t.Fatalf("file path error = %v", err)
	}

	orphanDirectory := filepath.Join(t.TempDir(), "orphan")
	if err := os.Mkdir(orphanDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphanDirectory, databaseFilename+"-wal"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openWithMigrations(ctx, Options{DataDirectory: orphanDirectory, BusyTimeout: time.Second}, files); !errors.Is(err, ErrUnsafeDataDirectory) {
		t.Fatalf("orphan sidecar error = %v", err)
	}
}

func TestOpenRejectsSymlinkedDataDirectory(t *testing.T) {
	t.Parallel()
	realDirectory := filepath.Join(t.TempDir(), "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(realDirectory, linkPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := openWithMigrations(context.Background(), Options{DataDirectory: linkPath, BusyTimeout: time.Second}, oneTableMigration())
	if !errors.Is(err, ErrUnsafeDataDirectory) {
		t.Fatalf("Open() error = %v", err)
	}
}

func TestStoreRejectsHardLinkedDatabaseFiles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	options := Options{DataDirectory: dataDirectory, BusyTimeout: time.Second}
	store, err := openWithMigrations(ctx, options, oneTableMigration())
	if err != nil {
		t.Fatal(err)
	}
	aliasDirectory := filepath.Join(t.TempDir(), "alias")
	if err := os.Mkdir(aliasDirectory, 0o700); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	aliasPath := filepath.Join(aliasDirectory, databaseFilename)
	if err := os.Link(filepath.Join(dataDirectory, databaseFilename), aliasPath); err != nil {
		_ = store.Close()
		t.Skipf("hard links unavailable: %v", err)
	}
	if err := store.Health(ctx); !errors.Is(err, ErrDatabaseUnavailable) {
		_ = store.Close()
		t.Fatalf("hard-linked Health() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := openWithMigrations(ctx, options, oneTableMigration()); !errors.Is(err, ErrUnsafeDataDirectory) {
		t.Fatalf("original hard-linked Open() error = %v", err)
	}
	aliasOptions := Options{DataDirectory: aliasDirectory, BusyTimeout: time.Second}
	if _, err := openWithMigrations(ctx, aliasOptions, oneTableMigration()); !errors.Is(err, ErrUnsafeDataDirectory) {
		t.Fatalf("alias hard-linked Open() error = %v", err)
	}
}

func TestSecureDirectoryTreatsBackslashAsPOSIXName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("backslash is a separator on Windows")
	}
	base := t.TempDir()
	realTarget := filepath.Join(base, `unsafe\name`)
	if err := os.Mkdir(realTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(realTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	decoyParent := filepath.Join(base, "unsafe")
	if err := os.Mkdir(decoyParent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(decoyParent, "name"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := validateSecureDirectory(realTarget, false); !errors.Is(err, ErrUnsafeDataDirectory) {
		t.Fatalf("validateSecureDirectory() error = %v; want unsafe directory", err)
	}
}

func TestOwnershipLockBlocksSameAndChildProcess(t *testing.T) {
	ctx := context.Background()
	options := Options{DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: time.Second}
	store, err := openWithMigrations(ctx, options, oneTableMigration())
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	if _, err := openWithMigrations(ctx, options, oneTableMigration()); !errors.Is(err, ErrOwnershipUnavailable) {
		t.Fatalf("second Open() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestOwnerLockChild$")
	command.Env = append(os.Environ(), ownerLockChildEnvironment+"="+options.DataDirectory)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "locked" {
		_ = stdin.Close()
		_ = command.Wait()
		t.Fatalf("child lock signal = %q, error = %v", line, err)
	}
	if _, err := acquireOwnerLock(options.DataDirectory); !errors.Is(err, ErrOwnershipUnavailable) {
		t.Fatalf("parent lock error = %v", err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("child Wait() error = %v", err)
	}
}

func TestOwnerLockChild(t *testing.T) {
	dataDirectory := os.Getenv(ownerLockChildEnvironment)
	if dataDirectory == "" {
		return
	}
	lock, err := acquireOwnerLock(dataDirectory)
	if err != nil {
		t.Fatalf("acquireOwnerLock() error = %v", err)
	}
	fmt.Fprintln(os.Stdout, "locked")
	_, _ = io.Copy(io.Discard, os.Stdin)
	if err := lock.Close(); err != nil {
		t.Fatalf("lock Close() error = %v", err)
	}
}

func oneTableMigration() fstest.MapFS {
	return fstest.MapFS{
		"000001_probe.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE probe (
    id INTEGER PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;`)},
	}
}
