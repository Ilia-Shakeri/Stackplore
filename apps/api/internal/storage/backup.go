package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/migrations"
	sqliteDriver "modernc.org/sqlite"
)

const backupStepPages int32 = 128

// BackupOptions contains the paths and timeout for a live database backup.
type BackupOptions struct {
	DataDirectory string
	BusyTimeout   time.Duration
	Destination   string
}

type onlineBackuper interface {
	NewBackup(destinationURI string) (*sqliteDriver.Backup, error)
}

// Backup writes a consistent single-file snapshot without taking server ownership.
func Backup(ctx context.Context, options BackupOptions) error {
	if ctx == nil || ctx.Err() != nil || !validOptions(Options{
		DataDirectory: options.DataDirectory,
		BusyTimeout:   options.BusyTimeout,
	}) {
		return ErrInvalidOptions
	}
	orderedMigrations, err := loadMigrations(migrations.FS)
	if err != nil {
		return ErrBackupRejected
	}
	dataDirectory, err := existingDataDirectory(options.DataDirectory)
	if err != nil {
		return ErrBackupRejected
	}
	maintenanceLock, err := acquireMaintenanceLock(dataDirectory, false)
	if err != nil {
		return ErrBackupRejected
	}
	defer maintenanceLock.Close()
	if err := ensureNoPendingRestore(dataDirectory); err != nil {
		return ErrBackupRejected
	}
	databasePath, err := existingDatabaseFile(dataDirectory)
	if err != nil {
		return ErrBackupRejected
	}
	destination, parent, err := validateBackupDestination(options.Destination)
	if err != nil {
		return ErrBackupRejected
	}
	if pathWithinDirectory(destination, dataDirectory) {
		return ErrBackupRejected
	}

	temporaryFile, err := os.CreateTemp(parent, ".stackplore-backup-*.tmp")
	if err != nil {
		return ErrBackupRejected
	}
	temporaryPath := temporaryFile.Name()
	if err := temporaryFile.Chmod(0o600); err != nil {
		_ = temporaryFile.Close()
		cleanupDatabaseArtifacts(temporaryPath)
		return ErrBackupRejected
	}
	if err := temporaryFile.Close(); err != nil {
		cleanupDatabaseArtifacts(temporaryPath)
		return ErrBackupRejected
	}
	defer cleanupDatabaseArtifacts(temporaryPath)

	database, err := sql.Open("sqlite", readOnlyDatabaseDSN(databasePath, options.BusyTimeout))
	if err != nil {
		return ErrBackupRejected
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if err := database.PingContext(ctx); err != nil ||
		validateCurrentMigrationLedger(ctx, database, orderedMigrations) != nil ||
		validateInstallationMetadata(ctx, database, false) != nil ||
		quickCheck(ctx, database) != nil || foreignKeyCheck(ctx, database) != nil {
		_ = database.Close()
		return ErrBackupRejected
	}
	if err := runOnlineBackup(ctx, database, temporaryPath, options.BusyTimeout); err != nil {
		_ = database.Close()
		return ErrBackupRejected
	}
	if err := database.Close(); err != nil {
		return ErrBackupRejected
	}

	if hasDatabaseSidecars(temporaryPath) || validateBackupFile(ctx, temporaryPath, options.BusyTimeout, orderedMigrations) != nil {
		return ErrBackupRejected
	}
	if err := syncPrivateFile(temporaryPath); err != nil {
		return ErrBackupRejected
	}
	if err := publishBackup(temporaryPath, destination, parent); err != nil {
		return ErrBackupRejected
	}
	return nil
}

func pathWithinDirectory(path, directory string) bool {
	relative, err := filepath.Rel(directory, path)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func publishBackup(temporaryPath, destination, parent string) error {
	if err := os.Link(temporaryPath, destination); err != nil {
		return err
	}
	if err := os.Remove(temporaryPath); err != nil {
		_ = os.Remove(destination)
		return err
	}
	if err := syncDirectory(parent); err != nil {
		_ = os.Remove(destination)
		return err
	}
	return nil
}

func validateBackupDestination(value string) (string, string, error) {
	if value == "" || len(value) > maximumPathBytes || !utf8.ValidString(value) || !filepath.IsAbs(value) ||
		filepath.Clean(value) != value || isFilesystemRoot(value) || hasRemoteOrDevicePrefix(value) {
		return "", "", ErrBackupRejected
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return "", "", ErrBackupRejected
		}
	}
	parent := filepath.Dir(value)
	if err := validateSecureDirectory(parent, false); err != nil {
		return "", "", ErrBackupRejected
	}
	if _, err := os.Lstat(value); !errors.Is(err, os.ErrNotExist) {
		return "", "", ErrBackupRejected
	}
	return value, parent, nil
}

func readOnlyDatabaseDSN(databasePath string, busyTimeout time.Duration) string {
	parameters := url.Values{}
	parameters.Set("_busy_timeout", strconv.FormatInt(busyTimeout.Milliseconds(), 10))
	parameters.Set("_dqs", "0")
	parameters.Set("_error_rc", "1")
	parameters.Set("_foreign_keys", "on")
	parameters.Set("cache", "private")
	parameters.Set("mode", "ro")
	return sqliteFileURI(databasePath, parameters)
}

func immutableDatabaseDSN(databasePath string, busyTimeout time.Duration) string {
	parameters := url.Values{}
	parameters.Set("_busy_timeout", strconv.FormatInt(busyTimeout.Milliseconds(), 10))
	parameters.Set("_dqs", "0")
	parameters.Set("_error_rc", "1")
	parameters.Set("_foreign_keys", "on")
	parameters.Set("immutable", "1")
	parameters.Set("mode", "ro")
	return sqliteFileURI(databasePath, parameters)
}

func backupTargetDSN(databasePath string) string {
	parameters := url.Values{}
	parameters.Set("_dqs", "0")
	parameters.Set("_error_rc", "1")
	parameters.Set("_foreign_keys", "on")
	parameters.Set("_synchronous", "full")
	parameters.Set("cache", "private")
	parameters.Set("mode", "rw")
	return sqliteFileURI(databasePath, parameters)
}

func sqliteFileURI(databasePath string, parameters url.Values) string {
	path := filepath.ToSlash(databasePath)
	if runtime.GOOS == "windows" && len(path) >= 2 && path[1] == ':' {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path, RawQuery: parameters.Encode()}).String()
}

func runOnlineBackup(ctx context.Context, database *sql.DB, destinationPath string, busyTimeout time.Duration) error {
	connection, err := database.Conn(ctx)
	if err != nil {
		return ErrBackupRejected
	}
	effectiveBusyTimeout := contextBusyTimeout(ctx, busyTimeout)
	adjusted := effectiveBusyTimeout != busyTimeout
	if adjusted && setBusyTimeout(ctx, connection, effectiveBusyTimeout) != nil {
		_ = connection.Close()
		return ErrBackupRejected
	}
	defer func() {
		if adjusted {
			restoreContext, cancel := context.WithTimeout(context.Background(), busyTimeout)
			if setBusyTimeout(restoreContext, connection, busyTimeout) != nil {
				_ = connection.Raw(func(any) error { return driver.ErrBadConn })
			}
			cancel()
		}
		_ = connection.Close()
	}()

	err = connection.Raw(func(driverConnection any) error {
		backuper, ok := driverConnection.(onlineBackuper)
		if !ok {
			return ErrBackupRejected
		}
		var retryDeadline time.Time
		backoff := 5 * time.Millisecond
		var backup *sqliteDriver.Backup
		for {
			backup, err = backuper.NewBackup(backupTargetDSN(destinationPath))
			if err == nil {
				break
			}
			if retryDeadline.IsZero() {
				retryDeadline = time.Now().Add(effectiveBusyTimeout)
			}
			if !retryableSQLiteLock(err) || !waitBackupRetry(ctx, retryDeadline, &backoff) {
				return ErrBackupRejected
			}
		}
		retryDeadline = time.Time{}
		finished := false
		defer func() {
			if !finished {
				_ = backup.Finish()
			}
		}()
		backoff = 5 * time.Millisecond
		for {
			if ctx.Err() != nil {
				return ErrBackupRejected
			}
			more, err := backup.Step(backupStepPages)
			if err != nil {
				if retryDeadline.IsZero() {
					retryDeadline = time.Now().Add(effectiveBusyTimeout)
				}
				if !retryableSQLiteLock(err) || !waitBackupRetry(ctx, retryDeadline, &backoff) {
					return ErrBackupRejected
				}
				continue
			}
			retryDeadline = time.Time{}
			backoff = 5 * time.Millisecond
			if !more {
				break
			}
		}
		finished = true
		if err := backup.Finish(); err != nil {
			return ErrBackupRejected
		}
		return nil
	})
	if err != nil {
		return ErrBackupRejected
	}
	return nil
}

func waitBackupRetry(ctx context.Context, deadline time.Time, backoff *time.Duration) bool {
	if ctx.Err() != nil || !time.Now().Before(deadline) {
		return false
	}
	delay := *backoff
	if remaining := time.Until(deadline); delay > remaining {
		delay = remaining
	}
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		return false
	case <-timer.C:
	}
	if *backoff < 50*time.Millisecond {
		*backoff *= 2
		if *backoff > 50*time.Millisecond {
			*backoff = 50 * time.Millisecond
		}
	}
	return true
}

func retryableSQLiteLock(err error) bool {
	var coded interface{ Code() int }
	if !errors.As(err, &coded) {
		return false
	}
	primaryCode := coded.Code() & 0xff
	return primaryCode == 5 || primaryCode == 6
}

func validateBackupFile(ctx context.Context, path string, busyTimeout time.Duration, ordered []migration) error {
	if hasDatabaseSidecars(path) {
		return ErrBackupRejected
	}
	database, err := sql.Open("sqlite", immutableDatabaseDSN(path, busyTimeout))
	if err != nil {
		return ErrBackupRejected
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if err := database.PingContext(ctx); err != nil ||
		validateCurrentMigrationLedger(ctx, database, ordered) != nil ||
		validateInstallationMetadata(ctx, database, false) != nil ||
		integrityCheck(ctx, database) != nil || foreignKeyCheck(ctx, database) != nil {
		_ = database.Close()
		return ErrBackupRejected
	}
	if err := database.Close(); err != nil {
		return ErrBackupRejected
	}
	return nil
}

func hasDatabaseSidecars(path string) bool {
	for _, suffix := range databaseSidecarSuffixes {
		if _, err := os.Lstat(path + suffix); err == nil || !errors.Is(err, os.ErrNotExist) {
			return true
		}
	}
	return false
}

func cleanupDatabaseArtifacts(path string) {
	_ = os.Remove(path)
	for _, suffix := range databaseSidecarSuffixes {
		_ = os.Remove(path + suffix)
	}
}

func syncPrivateFile(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
