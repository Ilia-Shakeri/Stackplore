package storage

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/migrations"
)

func TestOpenFailsClosedOnPendingRestoreState(t *testing.T) {
	t.Parallel()
	for name, createState := range map[string]func(string) error{
		"pending marker": func(directory string) error {
			return os.WriteFile(filepath.Join(directory, restorePendingFilename), []byte(restorePendingContents), 0o600)
		},
		"installed marker": func(directory string) error {
			return os.WriteFile(filepath.Join(directory, restoreInstalledFilename), []byte(restoreInstalledContents), 0o600)
		},
		"stage artifact": func(directory string) error {
			return os.WriteFile(filepath.Join(directory, restoreStageFilename), []byte("stage"), 0o600)
		},
		"rollback artifact": func(directory string) error {
			return os.WriteFile(filepath.Join(directory, restoreRollbackFilename), []byte("rollback"), 0o600)
		},
	} {
		t.Run(name, func(t *testing.T) {
			dataDirectory := filepath.Join(t.TempDir(), "data")
			if err := os.Mkdir(dataDirectory, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := createState(dataDirectory); err != nil {
				t.Fatal(err)
			}
			options := Options{DataDirectory: dataDirectory, BusyTimeout: time.Second}
			if _, err := openWithMigrations(context.Background(), options, oneTableMigration()); !errors.Is(err, ErrRestoreRejected) {
				t.Fatalf("Open() error = %v", err)
			}
			if _, err := os.Lstat(filepath.Join(dataDirectory, databaseFilename)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("database was created, error = %v", err)
			}
		})
	}
}

func TestRecoveryRemovesPartialMarkerTempFiles(t *testing.T) {
	t.Parallel()
	for _, filename := range []string{restorePendingFilename, restoreInstalledFilename} {
		filename := filename
		t.Run(filename, func(t *testing.T) {
			dataDirectory := filepath.Join(t.TempDir(), "data")
			if err := os.Mkdir(dataDirectory, 0o700); err != nil {
				t.Fatal(err)
			}
			temporaryPath := filepath.Join(dataDirectory, filename+restoreMarkerTempSuffix)
			if err := os.WriteFile(temporaryPath, []byte("partial"), 0o600); err != nil {
				t.Fatal(err)
			}
			options := Options{DataDirectory: dataDirectory, BusyTimeout: time.Second}
			if _, err := openWithMigrations(context.Background(), options, oneTableMigration()); !errors.Is(err, ErrRestoreRejected) {
				t.Fatalf("Open() error = %v", err)
			}
			if err := recoverPendingRestore(
				context.Background(),
				dataDirectory,
				filepath.Join(dataDirectory, databaseFilename),
				time.Second,
				nil,
			); err != nil {
				t.Fatalf("recoverPendingRestore() error = %v", err)
			}
			if err := ensureNoPendingRestore(dataDirectory); err != nil {
				t.Fatalf("restore state remains: %v", err)
			}
		})
	}
}

func TestPendingRestoreRecoveryReturnsOldDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	options := Options{DataDirectory: dataDirectory, BusyTimeout: time.Second}
	store, err := openWithMigrations(ctx, options, oneTableMigration())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "INSERT INTO probe (id, value) VALUES (1, 'old')"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dataDirectory, databaseFilename)
	rollbackPath := filepath.Join(dataDirectory, restoreRollbackFilename)
	if err := createRestoreMarker(dataDirectory, restorePendingFilename, restorePendingContents); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(targetPath, rollbackPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDirectory, restoreStageFilename), []byte("partial new database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openWithMigrations(ctx, options, oneTableMigration()); !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("crash-state Open() error = %v", err)
	}
	if err := recoverPendingRestore(ctx, dataDirectory, targetPath, time.Second, nil); err != nil {
		t.Fatalf("recoverPendingRestore() error = %v", err)
	}
	if err := ensureNoPendingRestore(dataDirectory); err != nil {
		t.Fatalf("restore state remains: %v", err)
	}
	store, err = openWithMigrations(ctx, options, oneTableMigration())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var value string
	if err := store.db.QueryRowContext(ctx, "SELECT value FROM probe WHERE id = 1").Scan(&value); err != nil || value != "old" {
		t.Fatalf("recovered value = %q, error = %v", value, err)
	}
}

func TestPendingRestoreRecoveryRejectsSplitSidecarSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	options := Options{DataDirectory: dataDirectory, BusyTimeout: time.Second}
	store, err := openWithMigrations(ctx, options, oneTableMigration())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dataDirectory, databaseFilename)
	rollbackSidecar := filepath.Join(dataDirectory, restoreRollbackFilename) + "-journal"
	if err := os.WriteFile(rollbackSidecar, []byte("old sidecar"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDirectory, restoreStageFilename), []byte("stage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := createRestoreMarker(dataDirectory, restorePendingFilename, restorePendingReplaceContents); err != nil {
		t.Fatal(err)
	}

	err = recoverPendingRestore(ctx, dataDirectory, targetPath, time.Second, nil)
	if !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("recoverPendingRestore() error = %v", err)
	}
	for _, path := range []string{
		rollbackSidecar,
		filepath.Join(dataDirectory, restorePendingFilename),
		filepath.Join(dataDirectory, restoreStageFilename),
	} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("recovery evidence missing at %q: %v", path, err)
		}
	}
}

func TestPendingRestoreRejectsOrphanRollbackSidecar(t *testing.T) {
	t.Parallel()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dataDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := createRestoreMarker(dataDirectory, restorePendingFilename, restorePendingContents); err != nil {
		t.Fatal(err)
	}
	rollbackSidecarPath := filepath.Join(dataDirectory, restoreRollbackFilename) + "-wal"
	if err := os.WriteFile(rollbackSidecarPath, []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := recoverPendingRestore(
		context.Background(),
		dataDirectory,
		filepath.Join(dataDirectory, databaseFilename),
		time.Second,
		nil,
	)
	if !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("recoverPendingRestore() error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dataDirectory, restorePendingFilename)); err != nil {
		t.Fatalf("pending marker was removed: %v", err)
	}
	if _, err := os.Lstat(rollbackSidecarPath); err != nil {
		t.Fatalf("rollback sidecar was removed: %v", err)
	}
}

func TestFreshPendingRestoreRecoveryRemovesUncommittedTarget(t *testing.T) {
	t.Parallel()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dataDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dataDirectory, databaseFilename)
	stagePath := filepath.Join(dataDirectory, restoreStageFilename)
	if err := os.WriteFile(targetPath, []byte("uncommitted target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagePath, []byte("partial stage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := createRestoreMarker(dataDirectory, restorePendingFilename, restorePendingFreshContents); err != nil {
		t.Fatal(err)
	}

	if err := recoverPendingRestore(
		context.Background(),
		dataDirectory,
		targetPath,
		time.Second,
		nil,
	); err != nil {
		t.Fatal(err)
	}
	if err := ensureNoPendingRestore(dataDirectory); err != nil {
		t.Fatalf("restore state remains: %v", err)
	}
	for _, path := range []string{targetPath, stagePath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("uncommitted file remains at %q: %v", path, err)
		}
	}
}

func TestInstalledRestoreRecoveryKeepsCheckedDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	options := Options{DataDirectory: dataDirectory, BusyTimeout: time.Second}
	store, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := seedInstallation(ctx, store.db, "new"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE installation_metadata
SET restored_at_us = 2, restore_generation = 1
WHERE singleton_id = 1`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	oldDirectory := filepath.Join(t.TempDir(), "old")
	oldOptions := Options{DataDirectory: oldDirectory, BusyTimeout: time.Second}
	oldStore, err := Open(ctx, oldOptions)
	if err != nil {
		t.Fatal(err)
	}
	if err := seedInstallation(ctx, oldStore.db, "old"); err != nil {
		t.Fatal(err)
	}
	if err := oldStore.Close(); err != nil {
		t.Fatal(err)
	}
	rollbackPath := filepath.Join(dataDirectory, restoreRollbackFilename)
	if err := os.Rename(filepath.Join(oldDirectory, databaseFilename), rollbackPath); err != nil {
		t.Fatal(err)
	}
	if err := createRestoreMarker(dataDirectory, restorePendingFilename, restorePendingContents); err != nil {
		t.Fatal(err)
	}
	if err := createRestoreMarker(dataDirectory, restoreInstalledFilename, restoreInstalledContents); err != nil {
		t.Fatal(err)
	}
	if _, err := openWithMigrations(ctx, options, oneTableMigration()); !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("installed-state Open() error = %v", err)
	}
	ordered, err := loadMigrations(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if err := recoverPendingRestore(
		ctx,
		dataDirectory,
		filepath.Join(dataDirectory, databaseFilename),
		time.Second,
		ordered,
	); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var installationID string
	if err := store.db.QueryRowContext(ctx, `SELECT installation_id
FROM installation_metadata WHERE singleton_id = 1`).Scan(&installationID); err != nil || installationID != "new" {
		t.Fatalf("recovered installation ID = %q, error = %v", installationID, err)
	}
	if err := ensureNoPendingRestore(dataDirectory); err != nil {
		t.Fatalf("restore state remains: %v", err)
	}
}

func TestInstalledRestoreRecoveryPreservesRollbackWhenTargetIsInvalid(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	store, err := Open(ctx, Options{DataDirectory: dataDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := seedInstallation(ctx, store.db, "old"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dataDirectory, databaseFilename)
	rollbackPath := filepath.Join(dataDirectory, restoreRollbackFilename)
	if err := os.Rename(targetPath, rollbackPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("invalid installed target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := createRestoreMarker(dataDirectory, restorePendingFilename, restorePendingReplaceContents); err != nil {
		t.Fatal(err)
	}
	if err := createRestoreMarker(dataDirectory, restoreInstalledFilename, restoreInstalledContents); err != nil {
		t.Fatal(err)
	}
	ordered, err := loadMigrations(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}

	err = recoverPendingRestore(ctx, dataDirectory, targetPath, time.Second, ordered)
	if !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("recoverPendingRestore() error = %v", err)
	}
	for _, path := range []string{
		rollbackPath,
		filepath.Join(dataDirectory, restorePendingFilename),
		filepath.Join(dataDirectory, restoreInstalledFilename),
	} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("recovery evidence missing at %q: %v", path, err)
		}
	}
}

func TestInstalledRestoreRecoveryRejectsLiveSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	store, err := Open(ctx, Options{DataDirectory: dataDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := seedInstallation(ctx, store.db, "installed-with-session"); err != nil {
		t.Fatal(err)
	}
	if err := seedCredentials(ctx, store.db); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE installation_metadata
SET restored_at_us = 2, restore_generation = 1
WHERE singleton_id = 1`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := createRestoreMarker(dataDirectory, restorePendingFilename, restorePendingFreshContents); err != nil {
		t.Fatal(err)
	}
	if err := createRestoreMarker(dataDirectory, restoreInstalledFilename, restoreInstalledContents); err != nil {
		t.Fatal(err)
	}
	ordered, err := loadMigrations(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}

	err = recoverPendingRestore(
		ctx,
		dataDirectory,
		filepath.Join(dataDirectory, databaseFilename),
		time.Second,
		ordered,
	)
	if !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("recoverPendingRestore() error = %v", err)
	}
	for _, filename := range []string{restorePendingFilename, restoreInstalledFilename} {
		if _, err := os.Lstat(filepath.Join(dataDirectory, filename)); err != nil {
			t.Fatalf("restore marker %q missing: %v", filename, err)
		}
	}
}

func TestInstalledRestoreCleanupRetriesWithoutRepeatingRestore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sourcePath := createRestorableBackup(t, ctx, "source", false)
	dataDirectory := filepath.Join(t.TempDir(), "data")
	store, err := Open(ctx, Options{DataDirectory: dataDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := seedInstallation(ctx, store.db, "installed"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE installation_metadata
SET restored_at_us = 2, restore_generation = 1
WHERE singleton_id = 1`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := createRestoreMarker(dataDirectory, restorePendingFilename, restorePendingReplaceContents); err != nil {
		t.Fatal(err)
	}
	if err := createRestoreMarker(dataDirectory, restoreInstalledFilename, restoreInstalledContents); err != nil {
		t.Fatal(err)
	}
	rollbackPath := filepath.Join(dataDirectory, restoreRollbackFilename)
	if err := os.Mkdir(rollbackPath, 0o700); err != nil {
		t.Fatal(err)
	}
	blockerPath := filepath.Join(rollbackPath, "blocker")
	if err := os.WriteFile(blockerPath, []byte("block"), 0o600); err != nil {
		t.Fatal(err)
	}
	ordered, err := loadMigrations(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dataDirectory, databaseFilename)
	if err := recoverPendingRestore(ctx, dataDirectory, targetPath, time.Second, ordered); !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("blocked recoverPendingRestore() error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dataDirectory, restoreInstalledFilename)); err != nil {
		t.Fatalf("installed marker missing after cleanup failure: %v", err)
	}
	if err := os.Remove(blockerPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(rollbackPath); err != nil {
		t.Fatal(err)
	}

	if err := Restore(ctx, RestoreOptions{
		DataDirectory: dataDirectory,
		BusyTimeout:   time.Second,
		Source:        sourcePath,
	}); err != nil {
		t.Fatalf("cleanup retry Restore() error = %v", err)
	}
	if err := ensureNoPendingRestore(dataDirectory); err != nil {
		t.Fatalf("restore state remains: %v", err)
	}
	reopened, err := Open(ctx, Options{DataDirectory: dataDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var installationID string
	var generation int64
	if err := reopened.db.QueryRowContext(ctx, `SELECT installation_id, restore_generation
FROM installation_metadata WHERE singleton_id = 1`).Scan(&installationID, &generation); err != nil {
		t.Fatal(err)
	}
	if installationID != "installed" || generation != 1 {
		t.Fatalf("cleanup retry metadata = %q, %d", installationID, generation)
	}
}

func TestMoveDatabaseSetPreflightsAndMovesWholeSet(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "source.db")
	destinationPath := filepath.Join(directory, "destination.db")
	want := map[string]string{"": "main", "-wal": "wal", "-shm": "shm"}
	for suffix, contents := range want {
		if err := os.WriteFile(sourcePath+suffix, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := moveDatabaseSet(sourcePath, destinationPath); err != nil {
		t.Fatal(err)
	}
	for suffix, contents := range want {
		got, err := os.ReadFile(destinationPath + suffix)
		if err != nil || string(got) != contents {
			t.Fatalf("moved %q = %q, error = %v", suffix, got, err)
		}
		if _, err := os.Lstat(sourcePath + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("source %q remains: %v", suffix, err)
		}
	}

	collisionSource := filepath.Join(directory, "collision-source.db")
	collisionDestination := filepath.Join(directory, "collision-destination.db")
	for suffix, contents := range map[string]string{"": "source-main", "-wal": "source-wal"} {
		if err := os.WriteFile(collisionSource+suffix, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(collisionDestination+"-wal", []byte("destination-wal"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := moveDatabaseSet(collisionSource, collisionDestination); !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("collision moveDatabaseSet() error = %v", err)
	}
	for path, contents := range map[string]string{
		collisionSource:               "source-main",
		collisionSource + "-wal":      "source-wal",
		collisionDestination + "-wal": "destination-wal",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != contents {
			t.Fatalf("collision file %q = %q, error = %v", path, got, err)
		}
	}
}

func TestMaintenanceLockSerializesBackupAndRestore(t *testing.T) {
	ctx := context.Background()
	sourceDirectory := filepath.Join(t.TempDir(), "source-data")
	sourceStore, err := Open(ctx, Options{DataDirectory: sourceDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := seedInstallation(ctx, sourceStore.db, "source"); err != nil {
		t.Fatal(err)
	}
	backupDirectory := filepath.Join(t.TempDir(), "backups")
	if err := os.Mkdir(backupDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(backupDirectory, "source.db")
	exclusive, err := acquireMaintenanceLock(sourceDirectory, true)
	if err != nil {
		t.Fatal(err)
	}
	backupOptions := BackupOptions{DataDirectory: sourceDirectory, BusyTimeout: time.Second, Destination: backupPath}
	if err := Backup(ctx, backupOptions); !errors.Is(err, ErrBackupRejected) {
		t.Fatalf("overlapping Backup() error = %v", err)
	}
	if err := exclusive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Backup(ctx, backupOptions); err != nil {
		t.Fatal(err)
	}
	if err := sourceStore.Close(); err != nil {
		t.Fatal(err)
	}

	targetDirectory := filepath.Join(t.TempDir(), "target-data")
	targetStore, err := Open(ctx, Options{DataDirectory: targetDirectory, BusyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := targetStore.Close(); err != nil {
		t.Fatal(err)
	}
	shared, err := acquireMaintenanceLock(targetDirectory, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Restore(ctx, RestoreOptions{DataDirectory: targetDirectory, BusyTimeout: time.Second, Source: backupPath}); !errors.Is(err, ErrRestoreRejected) {
		t.Fatalf("overlapping Restore() error = %v", err)
	}
	if err := shared.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ensureNoPendingRestore(targetDirectory); err != nil {
		t.Fatalf("failed overlap left restore state: %v", err)
	}
}

func TestWriteContentionHonorsContextAndRollsBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	options := Options{DataDirectory: filepath.Join(t.TempDir(), "data"), BusyTimeout: 2 * time.Second}
	store, err := openWithMigrations(ctx, options, oneTableMigration())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	blocker, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blocker.ExecContext(ctx, "INSERT INTO probe (id, value) VALUES (1, 'blocked')"); err != nil {
		t.Fatal(err)
	}
	blockedContext, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = store.withinTransaction(blockedContext, func(transaction *transaction) error {
		_, err := transaction.transaction.ExecContext(blockedContext, "INSERT INTO probe (id, value) VALUES (2, 'must not commit')")
		return err
	})
	elapsed := time.Since(started)
	if !errors.Is(err, ErrTransactionFailed) {
		t.Fatalf("contended write error = %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("contended write took %v", elapsed)
	}
	if err := blocker.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM probe").Scan(&count); err != nil || count != 0 {
		t.Fatalf("row count = %d, error = %v", count, err)
	}
}

func TestBackupCompletesDuringRepeatedWrites(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	dataDirectory := filepath.Join(t.TempDir(), "data")
	store, err := Open(ctx, Options{DataDirectory: dataDirectory, BusyTimeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := seedInstallation(ctx, store.db, "writer-a"); err != nil {
		t.Fatal(err)
	}
	writeContext, stopWrites := context.WithCancel(ctx)
	var writes atomic.Int64
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		for index := int64(0); ; index++ {
			select {
			case <-writeContext.Done():
				return
			default:
			}
			value := "writer-a"
			if index%2 == 1 {
				value = "writer-b"
			}
			if _, err := store.db.ExecContext(writeContext, "UPDATE installation_metadata SET installation_id = ? WHERE singleton_id = 1", value); err == nil {
				writes.Add(1)
			}
		}
	}()
	deadline := time.Now().Add(2 * time.Second)
	for writes.Load() < 10 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	backupDirectory := filepath.Join(t.TempDir(), "backups")
	if err := os.Mkdir(backupDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(backupDirectory, "busy.db")
	err = Backup(ctx, BackupOptions{DataDirectory: dataDirectory, BusyTimeout: 2 * time.Second, Destination: destination})
	stopWrites()
	wait.Wait()
	if err != nil {
		t.Fatalf("Backup() during writes error = %v", err)
	}
	if writes.Load() < 1 {
		t.Fatal("writer made no progress")
	}
	if got := readInstallationID(t, destination, time.Second); got != "writer-a" && got != "writer-b" {
		t.Fatalf("backup installation ID = %q", got)
	}
}

func TestOnlineBackupLockRetryHonorsCancellation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := openWithMigrations(ctx, Options{
		DataDirectory: filepath.Join(t.TempDir(), "source"),
		BusyTimeout:   2 * time.Second,
	}, oneTableMigration())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	destination := filepath.Join(t.TempDir(), "locked.db")
	parameters := url.Values{}
	parameters.Set("_txlock", "exclusive")
	parameters.Set("mode", "rwc")
	lockingDatabase, err := sql.Open("sqlite", sqliteFileURI(destination, parameters))
	if err != nil {
		t.Fatal(err)
	}
	defer lockingDatabase.Close()
	if _, err := lockingDatabase.ExecContext(ctx, "CREATE TABLE lock_probe (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	blockingTransaction, err := lockingDatabase.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer blockingTransaction.Rollback()

	backupContext, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = runOnlineBackup(backupContext, store.db, destination, 2*time.Second)
	if !errors.Is(err, ErrBackupRejected) {
		t.Fatalf("runOnlineBackup() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed < 100*time.Millisecond || elapsed > time.Second {
		t.Fatalf("canceled backup took %v", elapsed)
	}
}
