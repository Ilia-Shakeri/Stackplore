package storage

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/migrations"
)

func TestBackupCopiesLiveDatabaseAndNeverOverwrites(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	store, err := Open(ctx, Options{DataDirectory: dataDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := seedInstallation(ctx, store.db, "live-source"); err != nil {
		t.Fatal(err)
	}
	backupDirectory := filepath.Join(t.TempDir(), "backups")
	if err := os.Mkdir(backupDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(backupDirectory, "snapshot #%25.db")
	options := BackupOptions{DataDirectory: dataDirectory, BusyTimeout: time.Second, Destination: destination}
	if err := Backup(ctx, options); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	info, err := os.Lstat(destination)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("backup info = %v, error = %v", info, err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode = %o", info.Mode().Perm())
	}
	if hasDatabaseSidecars(destination) {
		t.Fatal("backup has sidecars")
	}
	if got := readInstallationID(t, destination, time.Second); got != "live-source" {
		t.Fatalf("backup installation ID = %q", got)
	}
	before, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := Backup(ctx, options); !errors.Is(err, ErrBackupRejected) {
		t.Fatalf("second Backup() error = %v", err)
	}
	after, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("existing backup changed")
	}
	temporaryFiles, err := filepath.Glob(filepath.Join(backupDirectory, ".stackplore-backup-*.tmp"))
	if err != nil || len(temporaryFiles) != 0 {
		t.Fatalf("temporary backup files = %v, error = %v", temporaryFiles, err)
	}
}

func TestFreshStoreBackupRestoresWithoutSetupData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sourceDirectory := filepath.Join(t.TempDir(), "source-data")
	sourceStore, err := Open(ctx, Options{DataDirectory: sourceDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var sourceInstallationID string
	if err := sourceStore.db.QueryRowContext(ctx, `SELECT installation_id
FROM installation_metadata
WHERE singleton_id = 1`).Scan(&sourceInstallationID); err != nil {
		t.Fatal(err)
	}
	backupDirectory := filepath.Join(t.TempDir(), "backup")
	if err := os.Mkdir(backupDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(backupDirectory, "fresh.db")
	if err := Backup(ctx, BackupOptions{
		DataDirectory: sourceDirectory,
		BusyTimeout:   time.Second,
		Destination:   backupPath,
	}); err != nil {
		t.Fatal(err)
	}
	if err := sourceStore.Close(); err != nil {
		t.Fatal(err)
	}

	targetDirectory := filepath.Join(t.TempDir(), "target-data")
	if err := Restore(ctx, RestoreOptions{
		DataDirectory: targetDirectory,
		BusyTimeout:   time.Second,
		Source:        backupPath,
	}); err != nil {
		t.Fatal(err)
	}
	if got := readInstallationID(t, filepath.Join(targetDirectory, databaseFilename), time.Second); got != sourceInstallationID {
		t.Fatalf("restored installation ID = %q; want %q", got, sourceInstallationID)
	}
	restoredStore, err := Open(ctx, Options{DataDirectory: targetDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := restoredStore.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBackupRejectsUnsafeDestinations(t *testing.T) {
	ctx := context.Background()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	store, err := Open(ctx, Options{DataDirectory: dataDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	backupDirectory := filepath.Join(t.TempDir(), "backups")
	if err := os.Mkdir(backupDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(backupDirectory, "existing.db")
	if err := os.WriteFile(existing, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, destination := range map[string]string{
		"relative":       "backup.db",
		"existing":       existing,
		"inside data":    filepath.Join(dataDirectory, "copy.db"),
		"reserved path":  filepath.Join(dataDirectory, databaseFilename+"-journal"),
		"missing parent": filepath.Join(backupDirectory, "missing", "backup.db"),
	} {
		t.Run(name, func(t *testing.T) {
			err := Backup(ctx, BackupOptions{DataDirectory: dataDirectory, BusyTimeout: time.Second, Destination: destination})
			if !errors.Is(err, ErrBackupRejected) {
				t.Fatalf("Backup() error = %v", err)
			}
		})
	}
}

func TestRestoreMigratesRevokesAndRemovesRollbackDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	backupPath := createRestorableBackup(t, ctx, "restored-source", true)

	targetDirectory := filepath.Join(t.TempDir(), "target-data")
	targetStore, err := Open(ctx, Options{DataDirectory: targetDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := seedInstallation(ctx, targetStore.db, "previous-target"); err != nil {
		t.Fatal(err)
	}
	if err := targetStore.Close(); err != nil {
		t.Fatal(err)
	}

	if err := Restore(ctx, RestoreOptions{
		DataDirectory: targetDirectory,
		BusyTimeout:   time.Second,
		Source:        backupPath,
	}); err != nil {
		entries, _ := os.ReadDir(targetDirectory)
		t.Fatalf("Restore() error = %v, entries = %v", err, entryNames(entries))
	}
	restoredStore, err := Open(ctx, Options{DataDirectory: targetDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer restoredStore.Close()
	var installationID string
	var restoredAt sql.NullInt64
	var generation int64
	if err := restoredStore.db.QueryRowContext(ctx, `SELECT installation_id, restored_at_us, restore_generation
FROM installation_metadata WHERE singleton_id = 1`).Scan(&installationID, &restoredAt, &generation); err != nil {
		t.Fatal(err)
	}
	if installationID != "restored-source" || !restoredAt.Valid || generation != 1 {
		t.Fatalf("restored metadata = %q, %v, %d", installationID, restoredAt, generation)
	}
	var revokedAt sql.NullInt64
	var reason sql.NullString
	if err := restoredStore.db.QueryRowContext(ctx, "SELECT revoked_at_us, revoke_reason_code FROM sessions WHERE id = 'session-1'").Scan(&revokedAt, &reason); err != nil {
		t.Fatal(err)
	}
	if !revokedAt.Valid || !reason.Valid || reason.String != "database_restore" {
		t.Fatalf("session revoke = %v, %v", revokedAt, reason)
	}
	for _, table := range []string{"preauth_challenges", "recent_auth_grants"} {
		var count int
		if err := restoredStore.db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s count = %d, error = %v", table, count, err)
		}
	}
	if err := restoredStore.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ensureNoPendingRestore(targetDirectory); err != nil {
		t.Fatalf("restore artifacts remain: %v", err)
	}
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestRestoreReplacesCorruptCurrentDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	backupPath := createRestorableBackup(t, ctx, "valid-source", false)
	targetDirectory := filepath.Join(t.TempDir(), "target-data")
	if err := os.Mkdir(targetDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(targetDirectory, databaseFilename)
	if err := os.WriteFile(targetPath, []byte("corrupt current database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Restore(ctx, RestoreOptions{
		DataDirectory: targetDirectory,
		BusyTimeout:   time.Second,
		Source:        backupPath,
	}); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if got := readInstallationID(t, targetPath, time.Second); got != "valid-source" {
		t.Fatalf("restored installation ID = %q", got)
	}
	if err := ensureNoPendingRestore(targetDirectory); err != nil {
		t.Fatalf("restore artifacts remain: %v", err)
	}
}

func TestRestorePreservesCorruptCurrentDatabaseWithSidecars(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	backupPath := createRestorableBackup(t, ctx, "valid-source", false)
	targetDirectory := filepath.Join(t.TempDir(), "target-data")
	if err := os.Mkdir(targetDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(targetDirectory, databaseFilename)
	targetContents := []byte("corrupt current database")
	sidecarPath := targetPath + "-wal"
	sidecarContents := []byte("preserve this sidecar")
	if err := os.WriteFile(targetPath, targetContents, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecarPath, sidecarContents, 0o600); err != nil {
		t.Fatal(err)
	}

	err := Restore(ctx, RestoreOptions{
		DataDirectory: targetDirectory,
		BusyTimeout:   time.Second,
		Source:        backupPath,
	})
	if !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("Restore() error = %v", err)
	}
	for path, want := range map[string][]byte{targetPath: targetContents, sidecarPath: sidecarContents} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("preserved file %q = %q, error = %v", path, got, err)
		}
	}
	if err := ensureNoPendingRestore(targetDirectory); err != nil {
		t.Fatalf("failed restore left state: %v", err)
	}
}

func TestNormalizeCurrentRestoreTargetCheckpointsValidWAL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sourceDirectory := filepath.Join(t.TempDir(), "source-data")
	store, err := Open(ctx, Options{DataDirectory: sourceDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := seedInstallation(ctx, store.db, "before-wal"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(sourceDirectory, databaseFilename)
	database, err := sql.Open("sqlite", databaseDSN(sourcePath, time.Second))
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	if _, err := database.ExecContext(ctx, "PRAGMA wal_autocheckpoint = 0"); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE installation_metadata
SET installation_id = 'after-wal'
WHERE singleton_id = 1`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	captured := make(map[string][]byte, 3)
	for _, suffix := range []string{"", "-wal", "-shm"} {
		captured[suffix], err = os.ReadFile(sourcePath + suffix)
		if err != nil {
			_ = database.Close()
			t.Fatalf("capture %q error = %v", suffix, err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	targetDirectory := filepath.Join(t.TempDir(), "target-data")
	if err := os.Mkdir(targetDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(targetDirectory, databaseFilename)
	for suffix, contents := range captured {
		if err := os.WriteFile(targetPath+suffix, contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ordered, err := loadMigrations(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if err := normalizeCurrentRestoreTarget(ctx, targetPath, time.Second, ordered); err != nil {
		t.Fatalf("normalizeCurrentRestoreTarget() error = %v", err)
	}
	if hasDatabaseSidecars(targetPath) {
		t.Fatal("normalized target has sidecars")
	}
	if got := readInstallationID(t, targetPath, time.Second); got != "after-wal" {
		t.Fatalf("normalized installation ID = %q", got)
	}
}

func TestRestoreRejectsBadSourceAndActiveOwner(t *testing.T) {
	ctx := context.Background()
	targetDirectory := filepath.Join(t.TempDir(), "target-data")
	targetStore, err := Open(ctx, Options{DataDirectory: targetDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := seedInstallation(ctx, targetStore.db, "target-stays"); err != nil {
		t.Fatal(err)
	}
	validBackup := createRestorableBackup(t, ctx, "source", false)
	if err := Restore(ctx, RestoreOptions{DataDirectory: targetDirectory, BusyTimeout: time.Second, Source: validBackup}); !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("active-owner Restore() error = %v", err)
	}
	if err := targetStore.Close(); err != nil {
		t.Fatal(err)
	}

	badDirectory := filepath.Join(t.TempDir(), "bad")
	if err := os.Mkdir(badDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	badSource := filepath.Join(badDirectory, "bad.db")
	if err := os.WriteFile(badSource, []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Restore(ctx, RestoreOptions{DataDirectory: targetDirectory, BusyTimeout: time.Second, Source: badSource}); !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("bad-source Restore() error = %v", err)
	}
	targetPath := filepath.Join(targetDirectory, databaseFilename)
	if got := readInstallationID(t, targetPath, time.Second); got != "target-stays" {
		t.Fatalf("target installation ID = %q", got)
	}
}

func TestRestoreRejectsBackupWithoutInstallationMetadata(t *testing.T) {
	ctx := context.Background()
	backupPath := createRestorableBackup(t, ctx, "source", false)
	backupDatabase, err := sql.Open("sqlite", backupTargetDSN(backupPath))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backupDatabase.ExecContext(ctx, "DELETE FROM installation_metadata"); err != nil {
		_ = backupDatabase.Close()
		t.Fatal(err)
	}
	if err := backupDatabase.Close(); err != nil {
		t.Fatal(err)
	}
	if hasDatabaseSidecars(backupPath) {
		t.Fatal("edited backup has sidecars")
	}

	targetDirectory := filepath.Join(t.TempDir(), "target-data")
	targetStore, err := Open(ctx, Options{DataDirectory: targetDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := seedInstallation(ctx, targetStore.db, "target-stays"); err != nil {
		t.Fatal(err)
	}
	if err := targetStore.Close(); err != nil {
		t.Fatal(err)
	}

	err = Restore(ctx, RestoreOptions{
		DataDirectory: targetDirectory,
		BusyTimeout:   time.Second,
		Source:        backupPath,
	})
	if !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("Restore() error = %v", err)
	}
	if got := readInstallationID(t, filepath.Join(targetDirectory, databaseFilename), time.Second); got != "target-stays" {
		t.Fatalf("target installation ID = %q", got)
	}
	if err := ensureNoPendingRestore(targetDirectory); err != nil {
		t.Fatalf("restore artifacts remain: %v", err)
	}
}

func createRestorableBackup(t *testing.T, ctx context.Context, installationID string, withCredentials bool) string {
	t.Helper()
	dataDirectory := filepath.Join(t.TempDir(), "source-data")
	store, err := Open(ctx, Options{DataDirectory: dataDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := seedInstallation(ctx, store.db, installationID); err != nil {
		t.Fatal(err)
	}
	if withCredentials {
		if err := seedCredentials(ctx, store.db); err != nil {
			t.Fatal(err)
		}
	}
	backupDirectory := filepath.Join(t.TempDir(), "backup")
	if err := os.Mkdir(backupDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(backupDirectory, "source.db")
	if err := Backup(ctx, BackupOptions{
		DataDirectory: dataDirectory,
		BusyTimeout:   time.Second,
		Destination:   backupPath,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return backupPath
}

func seedInstallation(ctx context.Context, database *sql.DB, installationID string) error {
	result, err := database.ExecContext(ctx, `UPDATE installation_metadata
SET installation_id = ?,
    data_mode = 'normal',
    demo_fixture_version = NULL,
    created_at_us = 1
WHERE singleton_id = 1`, installationID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return errors.New("installation metadata row is missing")
	}
	return nil
}

func seedCredentials(ctx context.Context, database *sql.DB) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	statements := []struct {
		query string
		args  []any
	}{
		{
			query: `INSERT INTO users
(id, login_name, canonical_login_name, display_name, role, created_at_us, updated_at_us)
VALUES ('user-1', 'Admin', 'admin', 'Admin', 'administrator', 1, 1)`,
		},
		{
			query: `INSERT INTO sessions
(id, user_id, token_digest, token_digest_key_id, csrf_digest, csrf_digest_key_id,
 user_auth_revision, created_at_us, last_seen_at_us, idle_expires_at_us, absolute_expires_at_us)
VALUES ('session-1', 'user-1', ?, 'key-1', ?, 'key-1', 1, 1, 1, 2000000000000000, 3000000000000000)`,
			args: []any{bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)},
		},
		{
			query: `INSERT INTO preauth_challenges
(id, purpose, cookie_digest, header_digest, digest_key_id, created_at_us, expires_at_us)
VALUES ('challenge-1', 'login', ?, ?, 'key-1', 1, 3000000000000000)`,
			args: []any{bytes.Repeat([]byte{3}, 32), bytes.Repeat([]byte{4}, 32)},
		},
		{
			query: `INSERT INTO recent_auth_grants
(id, user_id, session_id, purpose, grant_digest, digest_key_id, created_at_us, expires_at_us)
VALUES ('grant-1', 'user-1', 'session-1', 'admin', ?, 'key-1', 1, 3000000000000000)`,
			args: []any{bytes.Repeat([]byte{5}, 32)},
		},
	}
	for _, statement := range statements {
		if _, err := transaction.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

func readInstallationID(t *testing.T, path string, busyTimeout time.Duration) string {
	t.Helper()
	database, err := sql.Open("sqlite", immutableDatabaseDSN(path, busyTimeout))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var installationID string
	if err := database.QueryRow("SELECT installation_id FROM installation_metadata WHERE singleton_id = 1").Scan(&installationID); err != nil {
		t.Fatal(err)
	}
	return installationID
}
