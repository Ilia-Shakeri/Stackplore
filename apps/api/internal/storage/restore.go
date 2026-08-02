package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/migrations"
)

const (
	restorePendingFilename        = ".stackplore.restore.pending"
	restoreInstalledFilename      = ".stackplore.restore.installed"
	restoreStageFilename          = ".stackplore.restore.stage.db"
	restoreRollbackFilename       = ".stackplore.restore.rollback.db"
	restorePendingFreshContents   = "stackplore-restore-v2:pending:fresh\n"
	restorePendingReplaceContents = "stackplore-restore-v2:pending:replace\n"
	restorePendingContents        = restorePendingReplaceContents
	restoreInstalledContents      = "stackplore-restore-v1:installed\n"
	restoreMarkerTempSuffix       = ".tmp"
)

// RestoreOptions contains the paths and timeout for an offline restore.
type RestoreOptions struct {
	DataDirectory string
	BusyTimeout   time.Duration
	Source        string
}

// Restore validates, upgrades, and installs a backup while holding exclusive ownership.
func Restore(ctx context.Context, options RestoreOptions) error {
	if ctx == nil || ctx.Err() != nil || !validOptions(Options{
		DataDirectory: options.DataDirectory,
		BusyTimeout:   options.BusyTimeout,
	}) {
		return ErrInvalidOptions
	}
	orderedMigrations, err := loadMigrations(migrations.FS)
	if err != nil {
		return ErrRestoreRejected
	}
	dataDirectory, err := prepareDataDirectory(options.DataDirectory)
	if err != nil {
		return ErrRestoreRejected
	}
	ownerLock, err := acquireOwnerLock(dataDirectory)
	if err != nil {
		return ErrRestoreRejected
	}
	defer ownerLock.Close()
	maintenanceLock, err := acquireMaintenanceLock(dataDirectory, true)
	if err != nil {
		return ErrRestoreRejected
	}
	defer maintenanceLock.Close()

	targetPath := filepath.Join(dataDirectory, databaseFilename)
	installedPending, installedPendingValid := restoreMarkerStatus(
		filepath.Join(dataDirectory, restoreInstalledFilename),
		restoreInstalledContents,
	)
	if err := recoverPendingRestore(ctx, dataDirectory, targetPath, options.BusyTimeout, orderedMigrations); err != nil {
		return ErrRestoreRejected
	}
	if installedPending && installedPendingValid {
		return nil
	}
	if err := ensureNoPendingRestore(dataDirectory); err != nil {
		return ErrRestoreRejected
	}
	targetExists, err := restoreTargetStatus(targetPath)
	if err != nil {
		return ErrRestoreRejected
	}
	sourcePath, err := validateRestoreSource(options.Source)
	if err != nil {
		return ErrRestoreRejected
	}
	if targetExists {
		sourceInfo, sourceErr := os.Lstat(sourcePath)
		targetInfo, targetErr := os.Lstat(targetPath)
		if sourceErr != nil || targetErr != nil || os.SameFile(sourceInfo, targetInfo) {
			return ErrRestoreRejected
		}
	}
	if targetExists && hasDatabaseSidecars(targetPath) {
		if err := normalizeCurrentRestoreTarget(ctx, targetPath, options.BusyTimeout, orderedMigrations); err != nil {
			return ErrRestoreRejected
		}
	}

	sourceDatabase, err := openReadOnlyDatabase(ctx, sourcePath, options.BusyTimeout)
	if err != nil {
		return ErrRestoreRejected
	}
	if validateRestorableMigrationLedger(ctx, sourceDatabase, orderedMigrations) != nil ||
		validateInstallationMetadata(ctx, sourceDatabase, false) != nil ||
		integrityCheck(ctx, sourceDatabase) != nil || foreignKeyCheck(ctx, sourceDatabase) != nil {
		_ = sourceDatabase.Close()
		return ErrRestoreRejected
	}

	stagePath := filepath.Join(dataDirectory, restoreStageFilename)
	pendingContents := restorePendingFreshContents
	if targetExists {
		pendingContents = restorePendingReplaceContents
	}
	if err := createRestoreMarker(dataDirectory, restorePendingFilename, pendingContents); err != nil {
		_ = sourceDatabase.Close()
		return ErrRestoreRejected
	}
	completed := false
	defer func() {
		if !completed {
			installed, valid := restoreMarkerStatus(
				filepath.Join(dataDirectory, restoreInstalledFilename),
				restoreInstalledContents,
			)
			if !installed || !valid {
				_ = recoverPendingRestore(ctx, dataDirectory, targetPath, options.BusyTimeout, orderedMigrations)
			}
		}
	}()
	if err := createPrivateFile(stagePath); err != nil {
		_ = sourceDatabase.Close()
		return ErrRestoreRejected
	}
	if err := runOnlineBackup(ctx, sourceDatabase, stagePath, options.BusyTimeout); err != nil {
		_ = sourceDatabase.Close()
		return ErrRestoreRejected
	}
	if err := sourceDatabase.Close(); err != nil {
		return ErrRestoreRejected
	}

	if err := prepareRestoreStage(ctx, stagePath, options.BusyTimeout, orderedMigrations); err != nil {
		return ErrRestoreRejected
	}
	if err := swapRestoreStage(ctx, stagePath, targetPath, dataDirectory, targetExists, options.BusyTimeout, orderedMigrations); err != nil {
		return ErrRestoreRejected
	}
	completed = true
	return nil
}

func ensureNoPendingRestore(dataDirectory string) error {
	for _, path := range restoreStatePaths(dataDirectory) {
		if _, err := os.Lstat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
			return ErrRestoreRejected
		}
	}
	return nil
}

func restoreStatePaths(dataDirectory string) []string {
	paths := []string{
		filepath.Join(dataDirectory, restorePendingFilename),
		filepath.Join(dataDirectory, restoreInstalledFilename),
		filepath.Join(dataDirectory, restorePendingFilename+restoreMarkerTempSuffix),
		filepath.Join(dataDirectory, restoreInstalledFilename+restoreMarkerTempSuffix),
	}
	for _, filename := range []string{restoreStageFilename, restoreRollbackFilename} {
		base := filepath.Join(dataDirectory, filename)
		paths = append(paths, base)
		for _, suffix := range databaseSidecarSuffixes {
			paths = append(paths, base+suffix)
		}
	}
	return paths
}

func recoverPendingRestore(
	ctx context.Context,
	dataDirectory string,
	targetPath string,
	busyTimeout time.Duration,
	ordered []migration,
) error {
	if err := cleanupRestoreMarkerTemps(dataDirectory); err != nil {
		return ErrRestoreRejected
	}
	pendingPath := filepath.Join(dataDirectory, restorePendingFilename)
	installedPath := filepath.Join(dataDirectory, restoreInstalledFilename)
	stagePath := filepath.Join(dataDirectory, restoreStageFilename)
	rollbackPath := filepath.Join(dataDirectory, restoreRollbackFilename)
	pendingExists, pendingMode, pendingValid := restorePendingMarkerStatus(pendingPath)
	installedExists, installedValid := restoreMarkerStatus(installedPath, restoreInstalledContents)
	stageExists := databaseSetHasAnyFile(stagePath)
	rollbackExists := databaseSetHasAnyFile(rollbackPath)
	if !pendingExists && !installedExists {
		if stageExists || rollbackExists {
			return ErrRestoreRejected
		}
		return nil
	}

	if installedExists && installedValid {
		if pendingExists && !pendingValid {
			return ErrRestoreRejected
		}
		return finalizeInstalledRestore(ctx, dataDirectory, targetPath, busyTimeout, ordered)
	}

	if !pendingExists || !pendingValid || installedExists {
		return ErrRestoreRejected
	}
	rollbackMainExists, err := regularFileStatus(rollbackPath)
	if err != nil {
		return ErrRestoreRejected
	}
	if pendingMode == restorePendingFreshContents {
		if rollbackExists {
			return ErrRestoreRejected
		}
		if err := removeDatabaseSet(stagePath); err != nil {
			return ErrRestoreRejected
		}
		if err := removeDatabaseSet(targetPath); err != nil {
			return ErrRestoreRejected
		}
	} else if rollbackMainExists {
		if err := removeDatabaseSet(stagePath); err != nil {
			return ErrRestoreRejected
		}
		if err := restoreRollbackDatabaseSet(rollbackPath, targetPath); err != nil {
			return ErrRestoreRejected
		}
	} else if rollbackExists {
		return ErrRestoreRejected
	} else if err := removeDatabaseSet(stagePath); err != nil {
		return ErrRestoreRejected
	}
	if err := syncDirectory(dataDirectory); err != nil {
		return ErrRestoreRejected
	}
	if err := os.Remove(pendingPath); err != nil {
		return ErrRestoreRejected
	}
	return syncDirectory(dataDirectory)
}

func restorePendingMarkerStatus(path string) (bool, string, bool) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, "", false
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
		!privatePermissions(info.Mode()) || info.Size() > 64 {
		return true, "", false
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return true, "", false
	}
	mode := string(contents)
	switch mode {
	case restorePendingFreshContents, restorePendingReplaceContents:
		return true, mode, true
	default:
		return true, "", false
	}
}

func restoreRollbackDatabaseSet(rollbackPath, targetPath string) error {
	rollbackMainExists, err := regularFileStatus(rollbackPath)
	if err != nil || !rollbackMainExists {
		return ErrRestoreRejected
	}
	for _, suffix := range databaseSidecarSuffixes {
		if _, err := regularFileStatus(rollbackPath + suffix); err != nil {
			return ErrRestoreRejected
		}
	}
	targetMainExists, err := regularFileStatus(targetPath)
	if err != nil {
		return ErrRestoreRejected
	}
	if !targetMainExists && hasDatabaseSidecars(targetPath) {
		return ErrRestoreRejected
	}
	if targetMainExists {
		if err := removeDatabaseSet(targetPath); err != nil {
			return ErrRestoreRejected
		}
	}
	return moveDatabaseSet(rollbackPath, targetPath)
}

func restoreMarkerStatus(path, expected string) (bool, bool) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, false
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
		!privatePermissions(info.Mode()) || info.Size() != int64(len(expected)) {
		return true, false
	}
	contents, err := os.ReadFile(path)
	return true, err == nil && string(contents) == expected
}

func createRestoreMarker(dataDirectory, filename, contents string) error {
	path := filepath.Join(dataDirectory, filename)
	temporaryPath := path + restoreMarkerTempSuffix
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return ErrRestoreRejected
	}
	file, err := os.OpenFile(temporaryPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return ErrRestoreRejected
	}
	failed := false
	if _, err := file.WriteString(contents); err != nil {
		failed = true
	}
	if err := file.Sync(); err != nil {
		failed = true
	}
	if err := file.Close(); err != nil {
		failed = true
	}
	if failed {
		_ = os.Remove(temporaryPath)
		return ErrRestoreRejected
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Remove(temporaryPath)
		return ErrRestoreRejected
	}
	if err := syncDirectory(dataDirectory); err != nil {
		return ErrRestoreRejected
	}
	return nil
}

func cleanupRestoreMarkerTemps(dataDirectory string) error {
	removed := false
	for _, filename := range []string{restorePendingFilename, restoreInstalledFilename} {
		path := filepath.Join(dataDirectory, filename+restoreMarkerTempSuffix)
		exists, err := regularFileStatus(path)
		if err != nil {
			return ErrRestoreRejected
		}
		if !exists {
			continue
		}
		if err := os.Remove(path); err != nil {
			return ErrRestoreRejected
		}
		removed = true
	}
	if removed {
		return syncDirectory(dataDirectory)
	}
	return nil
}

func createPrivateFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return ErrRestoreRejected
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return ErrRestoreRejected
	}
	return nil
}

func databaseSetHasAnyFile(path string) bool {
	for _, candidate := range append([]string{path}, sidecarPaths(path)...) {
		if _, err := os.Lstat(candidate); err == nil || !errors.Is(err, os.ErrNotExist) {
			return true
		}
	}
	return false
}

func sidecarPaths(path string) []string {
	paths := make([]string, 0, len(databaseSidecarSuffixes))
	for _, suffix := range databaseSidecarSuffixes {
		paths = append(paths, path+suffix)
	}
	return paths
}

func validateRestoreSource(value string) (string, error) {
	if value == "" || len(value) > maximumPathBytes || !utf8.ValidString(value) || !filepath.IsAbs(value) ||
		filepath.Clean(value) != value || isFilesystemRoot(value) || hasRemoteOrDevicePrefix(value) {
		return "", ErrRestoreRejected
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return "", ErrRestoreRejected
		}
	}
	if err := validateSecureDirectory(filepath.Dir(value), false); err != nil {
		return "", ErrRestoreRejected
	}
	exists, err := regularFileStatus(value)
	if err != nil || !exists || hasDatabaseSidecars(value) {
		return "", ErrRestoreRejected
	}
	return value, nil
}

func restoreTargetStatus(targetPath string) (bool, error) {
	exists, err := regularFileStatus(targetPath)
	if err != nil {
		return false, err
	}
	for _, suffix := range databaseSidecarSuffixes {
		sidecarExists, sidecarErr := regularFileStatus(targetPath + suffix)
		if sidecarErr != nil || !exists && sidecarExists {
			return false, ErrRestoreRejected
		}
	}
	return exists, nil
}

func openReadOnlyDatabase(ctx context.Context, path string, busyTimeout time.Duration) (*sql.DB, error) {
	database, err := sql.Open("sqlite", immutableDatabaseDSN(path, busyTimeout))
	if err != nil {
		return nil, ErrRestoreRejected
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, ErrRestoreRejected
	}
	return database, nil
}

func validateRestorableMigrationLedger(ctx context.Context, database *sql.DB, ordered []migration) error {
	claimed, err := verifyDatabaseIdentity(ctx, database)
	if err != nil || !claimed {
		return ErrRestoreRejected
	}
	applied, err := readAppliedMigrations(ctx, database)
	if err != nil || len(applied) == 0 || !migrationLedgerMatches(applied, ordered) {
		return ErrRestoreRejected
	}
	return nil
}

func normalizeCurrentRestoreTarget(
	ctx context.Context,
	targetPath string,
	busyTimeout time.Duration,
	ordered []migration,
) error {
	exists, err := restoreTargetStatus(targetPath)
	if err != nil || !exists || !hasDatabaseSidecars(targetPath) {
		return ErrRestoreRejected
	}
	if err := inspectDatabaseIdentity(ctx, targetPath, busyTimeout); err != nil {
		return ErrRestoreRejected
	}
	database, err := sql.Open("sqlite", databaseDSN(targetPath, busyTimeout))
	if err != nil {
		return ErrRestoreRejected
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	closed := false
	defer func() {
		if !closed {
			_ = database.Close()
		}
	}()
	if err := database.PingContext(ctx); err != nil ||
		verifyConnectionPolicy(ctx, database, busyTimeout) != nil ||
		validateRestorableMigrationLedger(ctx, database, ordered) != nil ||
		validateInstallationMetadata(ctx, database, false) != nil ||
		integrityCheck(ctx, database) != nil || foreignKeyCheck(ctx, database) != nil ||
		checkpointDatabase(ctx, database) != nil {
		return ErrRestoreRejected
	}
	if err := database.Close(); err != nil {
		return ErrRestoreRejected
	}
	closed = true
	if err := removeDatabaseSidecars(targetPath); err != nil || hasDatabaseSidecars(targetPath) {
		return ErrRestoreRejected
	}
	if err := syncPrivateFile(targetPath); err != nil || syncDirectory(filepath.Dir(targetPath)) != nil {
		return ErrRestoreRejected
	}
	if err := validateRestorableDatabaseFile(ctx, targetPath, busyTimeout, ordered); err != nil {
		return ErrRestoreRejected
	}
	return nil
}

func validateRestorableDatabaseFile(
	ctx context.Context,
	path string,
	busyTimeout time.Duration,
	ordered []migration,
) error {
	if hasDatabaseSidecars(path) {
		return ErrRestoreRejected
	}
	database, err := openReadOnlyDatabase(ctx, path, busyTimeout)
	if err != nil {
		return ErrRestoreRejected
	}
	if validateRestorableMigrationLedger(ctx, database, ordered) != nil ||
		validateInstallationMetadata(ctx, database, false) != nil ||
		integrityCheck(ctx, database) != nil || foreignKeyCheck(ctx, database) != nil {
		_ = database.Close()
		return ErrRestoreRejected
	}
	if err := database.Close(); err != nil {
		return ErrRestoreRejected
	}
	return nil
}

func validateInstalledRestore(
	ctx context.Context,
	targetPath string,
	busyTimeout time.Duration,
	ordered []migration,
) error {
	exists, err := restoreTargetStatus(targetPath)
	if err != nil || !exists || hasDatabaseSidecars(targetPath) {
		return ErrRestoreRejected
	}
	if err := validateBackupFile(ctx, targetPath, busyTimeout, ordered); err != nil {
		return ErrRestoreRejected
	}
	database, err := openReadOnlyDatabase(ctx, targetPath, busyTimeout)
	if err != nil {
		return ErrRestoreRejected
	}
	queries := []string{
		"SELECT count(*) FROM sessions WHERE revoked_at_us IS NULL",
		"SELECT count(*) FROM preauth_challenges",
		"SELECT count(*) FROM recent_auth_grants",
		`SELECT count(*) FROM installation_metadata
WHERE singleton_id = 1 AND restored_at_us IS NOT NULL AND restore_generation >= 1`,
	}
	for index, query := range queries {
		var count int
		if err := database.QueryRowContext(ctx, query).Scan(&count); err != nil || count != boolCount(index == len(queries)-1) {
			_ = database.Close()
			return ErrRestoreRejected
		}
	}
	if err := database.Close(); err != nil {
		return ErrRestoreRejected
	}
	return nil
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func finalizeInstalledRestore(
	ctx context.Context,
	dataDirectory string,
	targetPath string,
	busyTimeout time.Duration,
	ordered []migration,
) error {
	installedExists, installedValid := restoreMarkerStatus(
		filepath.Join(dataDirectory, restoreInstalledFilename),
		restoreInstalledContents,
	)
	if !installedExists || !installedValid {
		return ErrRestoreRejected
	}
	pendingExists, _, pendingValid := restorePendingMarkerStatus(
		filepath.Join(dataDirectory, restorePendingFilename),
	)
	if pendingExists && !pendingValid {
		return ErrRestoreRejected
	}
	if err := validateInstalledRestore(ctx, targetPath, busyTimeout, ordered); err != nil {
		return ErrRestoreRejected
	}
	stagePath := filepath.Join(dataDirectory, restoreStageFilename)
	rollbackPath := filepath.Join(dataDirectory, restoreRollbackFilename)
	if err := removeDatabaseSet(stagePath); err != nil {
		return ErrRestoreRejected
	}
	if err := removeDatabaseSet(rollbackPath); err != nil {
		return ErrRestoreRejected
	}
	if err := syncDirectory(dataDirectory); err != nil {
		return ErrRestoreRejected
	}
	pendingPath := filepath.Join(dataDirectory, restorePendingFilename)
	if err := os.Remove(pendingPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return ErrRestoreRejected
	}
	if err := syncDirectory(dataDirectory); err != nil {
		return ErrRestoreRejected
	}
	installedPath := filepath.Join(dataDirectory, restoreInstalledFilename)
	if err := os.Remove(installedPath); err != nil {
		return ErrRestoreRejected
	}
	_ = syncDirectory(dataDirectory)
	return nil
}

func prepareRestoreStage(ctx context.Context, stagePath string, busyTimeout time.Duration, ordered []migration) error {
	database, err := sql.Open("sqlite", databaseDSN(stagePath, busyTimeout))
	if err != nil {
		return ErrRestoreRejected
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	closed := false
	defer func() {
		if !closed {
			_ = database.Close()
		}
	}()
	if err := database.PingContext(ctx); err != nil || verifyConnectionPolicy(ctx, database, busyTimeout) != nil ||
		applyMigrations(ctx, database, ordered) != nil || revokeRestoredCredentials(ctx, database) != nil ||
		integrityCheck(ctx, database) != nil || foreignKeyCheck(ctx, database) != nil {
		return ErrRestoreRejected
	}
	if err := checkpointDatabase(ctx, database); err != nil {
		return ErrRestoreRejected
	}
	if err := database.Close(); err != nil {
		return ErrRestoreRejected
	}
	closed = true
	if removeDatabaseSidecars(stagePath) != nil || hasDatabaseSidecars(stagePath) || syncPrivateFile(stagePath) != nil {
		return ErrRestoreRejected
	}
	return validateBackupFile(ctx, stagePath, busyTimeout, ordered)
}

func revokeRestoredCredentials(ctx context.Context, database *sql.DB) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return ErrRestoreRejected
	}
	committed := false
	defer func() {
		if !committed {
			_ = transaction.Rollback()
		}
	}()
	restoredAt := time.Now().UTC().UnixMicro()
	statements := []struct {
		query        string
		arguments    []any
		requiredRows int64
	}{
		{query: "DELETE FROM recent_auth_grants"},
		{query: "DELETE FROM preauth_challenges"},
		{query: "DELETE FROM authentication_throttles"},
		{
			query: `UPDATE sessions
SET revoked_at_us = max(created_at_us, ?),
    revoke_reason_code = 'database_restore'
WHERE revoked_at_us IS NULL`,
			arguments: []any{restoredAt},
		},
		{
			query: `UPDATE installation_metadata
SET restored_at_us = max(created_at_us, ?),
    restore_generation = restore_generation + 1
WHERE singleton_id = 1`,
			arguments:    []any{restoredAt},
			requiredRows: 1,
		},
	}
	for _, statement := range statements {
		result, err := transaction.ExecContext(ctx, statement.query, statement.arguments...)
		if err != nil {
			return ErrRestoreRejected
		}
		if statement.requiredRows > 0 {
			rows, err := result.RowsAffected()
			if err != nil || rows != statement.requiredRows {
				return ErrRestoreRejected
			}
		}
	}
	if err := transaction.Commit(); err != nil {
		return ErrRestoreRejected
	}
	committed = true
	return nil
}

func checkpointDatabase(ctx context.Context, database *sql.DB) error {
	var busy, logFrames, checkpointedFrames int
	if err := database.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointedFrames); err != nil || busy != 0 {
		return ErrRestoreRejected
	}
	return nil
}

func swapRestoreStage(
	ctx context.Context,
	stagePath string,
	targetPath string,
	dataDirectory string,
	targetExists bool,
	busyTimeout time.Duration,
	ordered []migration,
) error {
	rollbackPath := filepath.Join(dataDirectory, restoreRollbackFilename)
	if targetExists {
		if err := moveDatabaseSet(targetPath, rollbackPath); err != nil {
			return ErrRestoreRejected
		}
		if err := syncDirectory(dataDirectory); err != nil {
			return ErrRestoreRejected
		}
	}
	if err := os.Rename(stagePath, targetPath); err != nil {
		return ErrRestoreRejected
	}
	if err := syncPrivateFile(targetPath); err != nil || syncDirectory(dataDirectory) != nil ||
		validateBackupFile(ctx, targetPath, busyTimeout, ordered) != nil {
		return ErrRestoreRejected
	}
	if err := createRestoreMarker(dataDirectory, restoreInstalledFilename, restoreInstalledContents); err != nil {
		return ErrRestoreRejected
	}
	return finalizeInstalledRestore(ctx, dataDirectory, targetPath, busyTimeout, ordered)
}

func moveDatabaseSet(sourcePath, destinationPath string) error {
	sourceMainExists, err := regularFileStatus(sourcePath)
	if err != nil || !sourceMainExists {
		return ErrRestoreRejected
	}
	for _, candidate := range append([]string{destinationPath}, sidecarPaths(destinationPath)...) {
		if _, err := os.Lstat(candidate); !errors.Is(err, os.ErrNotExist) {
			return ErrRestoreRejected
		}
	}
	suffixes := make([]string, 0, len(databaseSidecarSuffixes))
	for _, suffix := range databaseSidecarSuffixes {
		exists, err := regularFileStatus(sourcePath + suffix)
		if err != nil {
			return ErrRestoreRejected
		}
		if exists {
			suffixes = append(suffixes, suffix)
		}
	}
	movedSuffixes := make([]string, 0, len(suffixes))
	for _, suffix := range suffixes {
		if os.Rename(sourcePath+suffix, destinationPath+suffix) != nil {
			for index := len(movedSuffixes) - 1; index >= 0; index-- {
				movedSuffix := movedSuffixes[index]
				_ = os.Rename(destinationPath+movedSuffix, sourcePath+movedSuffix)
			}
			return ErrRestoreRejected
		}
		movedSuffixes = append(movedSuffixes, suffix)
	}
	if err := os.Rename(sourcePath, destinationPath); err != nil {
		for index := len(movedSuffixes) - 1; index >= 0; index-- {
			movedSuffix := movedSuffixes[index]
			_ = os.Rename(destinationPath+movedSuffix, sourcePath+movedSuffix)
		}
		return ErrRestoreRejected
	}
	return nil
}

func removeDatabaseSidecars(path string) error {
	for _, suffix := range databaseSidecarSuffixes {
		if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func removeDatabaseSet(path string) error {
	if err := removeDatabaseSidecars(path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
