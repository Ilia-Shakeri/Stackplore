package storage

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/migrations"
)

func TestMigrationsAreIdempotentAndUpgradeInOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	options := Options{DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: time.Second}
	first := oneTableMigration()
	store, err := openWithMigrations(ctx, options, first)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = openWithMigrations(ctx, options, first)
	if err != nil {
		t.Fatalf("idempotent Open() error = %v", err)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 1 {
		t.Fatalf("migration count = %d, error = %v", count, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	second := oneTableMigration()
	second["000002_upgrade.sql"] = &fstest.MapFile{Data: []byte(
		"ALTER TABLE probe ADD COLUMN extra INTEGER NOT NULL DEFAULT 0;",
	)}
	store, err = openWithMigrations(ctx, options, second)
	if err != nil {
		t.Fatalf("upgrade Open() error = %v", err)
	}
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 2 {
		t.Fatalf("upgraded migration count = %d, error = %v", count, err)
	}
	if !tableHasColumn(t, store.db, "probe", "extra") {
		t.Fatal("upgraded column missing")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestEmbeddedMigrationsUpgradeCommittedFirstVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	firstContents, err := fs.ReadFile(migrations.FS, "000001_core_identity.sql")
	if err != nil {
		t.Fatal(err)
	}
	firstOnly := fstest.MapFS{
		"000001_core_identity.sql": &fstest.MapFile{Data: firstContents},
	}
	options := Options{DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: time.Second}
	store, err := openWithMigrations(ctx, options, firstOnly)
	if err != nil {
		t.Fatalf("first-version Open() error = %v", err)
	}
	if err := seedInstallation(ctx, store.db, "upgrade-canary"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = openWithMigrations(ctx, options, migrations.FS)
	if err != nil {
		t.Fatalf("current-version Open() error = %v", err)
	}
	defer store.Close()
	var migrationCount int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&migrationCount); err != nil || migrationCount != 3 {
		t.Fatalf("migration count = %d, error = %v", migrationCount, err)
	}
	var installationID string
	if err := store.db.QueryRowContext(ctx, "SELECT installation_id FROM installation_metadata WHERE singleton_id = 1").Scan(&installationID); err != nil || installationID != "upgrade-canary" {
		t.Fatalf("canary = %q, error = %v", installationID, err)
	}
	wantedTables := []string{
		"resources",
		"metric_samples",
		"infrastructure_events",
		"alerts",
		"notification_deliveries",
		"audit_entries",
	}
	for _, table := range wantedTables {
		var count int
		if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("table %q count = %d, error = %v", table, count, err)
		}
	}
}

func TestEmbeddedSchemaRejectsCrossUserAuthenticationLinks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := Open(ctx, Options{
		DataDirectory: filepath.Join(t.TempDir(), "data"),
		BusyTimeout:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, user := range []string{"user_a", "user_b"} {
		if _, err := store.db.ExecContext(ctx, `INSERT INTO users
(id, login_name, canonical_login_name, display_name, role, created_at_us, updated_at_us)
VALUES (?, ?, ?, ?, 'viewer', 1, 1)`, user, user, user, user); err != nil {
			t.Fatal(err)
		}
	}
	insertSession := func(id, userID string, digestByte byte, rotatedFrom any) error {
		_, err := store.db.ExecContext(ctx, `INSERT INTO sessions
(id, user_id, token_digest, token_digest_key_id, csrf_digest, csrf_digest_key_id,
 user_auth_revision, created_at_us, last_seen_at_us, idle_expires_at_us,
 absolute_expires_at_us, rotated_from_session_id)
VALUES (?, ?, ?, 'key', ?, 'key', 1, 1, 1, 100, 200, ?)`,
			id,
			userID,
			bytes.Repeat([]byte{digestByte}, 32),
			bytes.Repeat([]byte{digestByte + 32}, 32),
			rotatedFrom,
		)
		return err
	}
	if err := insertSession("session_a", "user_a", 1, nil); err != nil {
		t.Fatal(err)
	}
	if err := insertSession("session_b", "user_b", 2, nil); err != nil {
		t.Fatal(err)
	}
	if err := insertSession("session_a_rotated", "user_a", 3, "session_a"); err != nil {
		t.Fatalf("same-user rotation error = %v", err)
	}
	if err := insertSession("session_cross_user", "user_b", 4, "session_a"); err == nil {
		t.Fatal("cross-user session rotation succeeded")
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO recent_auth_grants
(id, user_id, session_id, purpose, grant_digest, digest_key_id, created_at_us, expires_at_us)
VALUES ('grant_cross_user', 'user_b', 'session_a', 'admin', ?, 'key', 1, 100)`,
		bytes.Repeat([]byte{9}, 32),
	); err == nil {
		t.Fatal("cross-user recent authentication grant succeeded")
	}
}

func TestMigrationsRejectHashDriftGapFutureAndBadSQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("hash drift", func(t *testing.T) {
		options := Options{DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: time.Second}
		store, err := openWithMigrations(ctx, options, oneTableMigration())
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		drifted := fstest.MapFS{
			"000001_probe.sql": &fstest.MapFile{Data: []byte("CREATE TABLE changed (id INTEGER PRIMARY KEY) STRICT;")},
		}
		if _, err := openWithMigrations(ctx, options, drifted); !errors.Is(err, ErrMigrationRejected) {
			t.Fatalf("hash drift error = %v", err)
		}
	})

	t.Run("gap", func(t *testing.T) {
		files := fstest.MapFS{
			"000002_probe.sql": &fstest.MapFile{Data: []byte("CREATE TABLE probe (id INTEGER PRIMARY KEY) STRICT;")},
		}
		options := Options{DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: time.Second}
		if _, err := openWithMigrations(ctx, options, files); !errors.Is(err, ErrMigrationRejected) {
			t.Fatalf("gap error = %v", err)
		}
	})

	t.Run("future ledger", func(t *testing.T) {
		options := Options{DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: time.Second}
		store, err := openWithMigrations(ctx, options, oneTableMigration())
		if err != nil {
			t.Fatal(err)
		}
		_, err = store.db.ExecContext(ctx, `INSERT INTO schema_migrations
(version, filename, checksum_sha256, applied_at_us)
VALUES (2, '000002_future.sql', ?, 1)`, strings.Repeat("a", 64))
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := openWithMigrations(ctx, options, oneTableMigration()); !errors.Is(err, ErrMigrationRejected) {
			t.Fatalf("future ledger error = %v", err)
		}
	})

	t.Run("bad SQL rolls back", func(t *testing.T) {
		options := Options{DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: time.Second}
		bad := fstest.MapFS{
			"000001_probe.sql": &fstest.MapFile{Data: []byte("CREATE TABLE leaked (id INTEGER); invalid statement;")},
		}
		if _, err := openWithMigrations(ctx, options, bad); !errors.Is(err, ErrMigrationRejected) {
			t.Fatalf("bad SQL error = %v", err)
		}
		store, err := openWithMigrations(ctx, options, oneTableMigration())
		if err != nil {
			t.Fatalf("recovery Open() error = %v", err)
		}
		var leaked int
		if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE name = 'leaked'").Scan(&leaked); err != nil || leaked != 0 {
			t.Fatalf("leaked table count = %d, error = %v", leaked, err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestMigrationRejectsWrongApplicationID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dataDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dataDirectory, databaseFilename)
	database, err := sql.Open("sqlite", databaseDSN(path, time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, "PRAGMA application_id = 1"); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	options := Options{DataDirectory: dataDirectory, BusyTimeout: time.Second}
	if _, err := openWithMigrations(ctx, options, oneTableMigration()); !errors.Is(err, ErrMigrationRejected) {
		t.Fatalf("wrong application ID error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("rejected foreign database was modified")
	}
}

func TestTransactionHelperRollsBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	options := Options{DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: time.Second}
	store, err := openWithMigrations(ctx, options, oneTableMigration())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	workError := errors.New("work stopped")
	err = store.withinTransaction(ctx, func(transaction *transaction) error {
		if _, err := transaction.transaction.ExecContext(ctx, "INSERT INTO probe (id, value) VALUES (1, 'rolled back')"); err != nil {
			return err
		}
		return workError
	})
	if !errors.Is(err, workError) {
		t.Fatalf("withinTransaction() error = %v", err)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM probe").Scan(&count); err != nil || count != 0 {
		t.Fatalf("row count after rollback = %d, error = %v", count, err)
	}

	err = store.withinTransaction(ctx, func(transaction *transaction) error {
		if _, err := transaction.transaction.ExecContext(ctx, "INSERT INTO probe (id, value) VALUES (2, 'committed')"); err != nil {
			return err
		}
		var value string
		if err := transaction.transaction.QueryRowContext(ctx, "SELECT value FROM probe WHERE id = 2").Scan(&value); err != nil {
			return err
		}
		if value != "committed" {
			return errors.New("unexpected value")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("committed withinTransaction() error = %v", err)
	}
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM probe").Scan(&count); err != nil || count != 1 {
		t.Fatalf("row count after commit = %d, error = %v", count, err)
	}
}

func TestPruneUsesFixedClassesAndBoundedBatches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	files := fstest.MapFS{
		"000001_retention.sql": &fstest.MapFile{Data: []byte(`
CREATE TABLE metric_samples (id TEXT PRIMARY KEY, sampled_at_us INTEGER NOT NULL) STRICT;
CREATE TABLE health_history (id TEXT PRIMARY KEY, observed_at_us INTEGER NOT NULL) STRICT;
CREATE TABLE infrastructure_events (id TEXT PRIMARY KEY, occurred_at_us INTEGER NOT NULL) STRICT;
CREATE TABLE notification_deliveries (id TEXT PRIMARY KEY, terminal_at_us INTEGER) STRICT;
`)},
	}
	options := Options{DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: time.Second}
	store, err := openWithMigrations(ctx, options, files)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, tableAndColumn := range [][2]string{
		{"metric_samples", "sampled_at_us"},
		{"health_history", "observed_at_us"},
		{"infrastructure_events", "occurred_at_us"},
		{"notification_deliveries", "terminal_at_us"},
	} {
		for index, timestamp := range []int64{10, 20, 30, 200} {
			statement := "INSERT INTO " + tableAndColumn[0] + " (id, " + tableAndColumn[1] + ") VALUES (?, ?)"
			if _, err := store.db.ExecContext(ctx, statement, tableAndColumn[0]+string(rune('a'+index)), timestamp); err != nil {
				t.Fatal(err)
			}
		}
	}
	cutoff := time.UnixMicro(100)
	for _, testCase := range []struct {
		class RetentionClass
		table string
	}{
		{RetentionMetricSamples, "metric_samples"},
		{RetentionHealthHistory, "health_history"},
		{RetentionInfrastructureEvents, "infrastructure_events"},
		{RetentionTerminalNotificationDeliveries, "notification_deliveries"},
	} {
		removed, err := store.Prune(ctx, testCase.class, cutoff, 2)
		if err != nil || removed != 2 {
			t.Fatalf("Prune(%q) = %d, %v", testCase.class, removed, err)
		}
		var remaining int
		if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM "+testCase.table).Scan(&remaining); err != nil || remaining != 2 {
			t.Fatalf("remaining %s = %d, error = %v", testCase.table, remaining, err)
		}
	}
	for _, testCase := range []struct {
		class RetentionClass
		limit int
	}{
		{"unknown", 1},
		{RetentionMetricSamples, 0},
		{RetentionMetricSamples, MaximumRetentionBatch + 1},
	} {
		if _, err := store.Prune(ctx, testCase.class, cutoff, testCase.limit); !errors.Is(err, ErrRetentionRejected) {
			t.Fatalf("invalid Prune(%q, %d) error = %v", testCase.class, testCase.limit, err)
		}
	}
}

func tableHasColumn(t *testing.T, database *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := database.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return false
}
