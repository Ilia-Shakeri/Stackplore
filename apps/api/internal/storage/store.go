// Package storage owns the local SQLite database lifecycle and transaction boundary.
package storage

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/migrations"
	"github.com/gofrs/flock"
	_ "modernc.org/sqlite"
)

const (
	minimumBusyTimeout     = 100 * time.Millisecond
	maximumBusyTimeout     = 30 * time.Second
	maximumOpenConnections = 8
)

// Options contains the bounded inputs needed to open the local database.
type Options struct {
	DataDirectory string
	BusyTimeout   time.Duration
}

// Store owns the database pool and the process-wide ownership lock.
type Store struct {
	db                 *sql.DB
	ownerLock          *flock.Flock
	busyTimeout        time.Duration
	databasePath       string
	databaseFile       fs.FileInfo
	expectedMigrations []appliedMigration
	closed             atomic.Bool
	closeOnce          sync.Once
	closeErr           error
	authDigestKey      [authDigestKeyBytes]byte
	authDigestKeyID    string
}

// Open resolves the data directory, takes exclusive ownership, and applies migrations.
func Open(ctx context.Context, options Options) (*Store, error) {
	return openWithMigrations(ctx, options, migrations.FS)
}

func openWithMigrations(ctx context.Context, options Options, migrationFiles fs.FS) (*Store, error) {
	if ctx == nil || ctx.Err() != nil || !validOptions(options) || migrationFiles == nil {
		return nil, ErrInvalidOptions
	}
	orderedMigrations, err := loadMigrations(migrationFiles)
	if err != nil {
		return nil, ErrMigrationRejected
	}

	dataDirectory, err := prepareDataDirectory(options.DataDirectory)
	if err != nil {
		return nil, err
	}
	ownerLock, err := acquireOwnerLock(dataDirectory)
	if err != nil {
		return nil, err
	}
	if err := ensureNoPendingRestore(dataDirectory); err != nil {
		_ = ownerLock.Close()
		return nil, err
	}
	authDigestKey, authDigestKeyID, err := loadOrCreateAuthDigestKey(dataDirectory)
	if err != nil {
		_ = ownerLock.Close()
		return nil, err
	}

	databasePath, err := prepareDatabaseFile(dataDirectory)
	if err != nil {
		_ = ownerLock.Close()
		return nil, err
	}
	if err := inspectDatabaseIdentity(ctx, databasePath, options.BusyTimeout); err != nil {
		_ = ownerLock.Close()
		return nil, err
	}
	database, err := sql.Open("sqlite", databaseDSN(databasePath, options.BusyTimeout))
	if err != nil {
		_ = ownerLock.Close()
		return nil, ErrDatabaseUnavailable
	}
	database.SetMaxOpenConns(maximumOpenConnections)
	database.SetMaxIdleConns(maximumOpenConnections)
	database.SetConnMaxIdleTime(0)
	database.SetConnMaxLifetime(0)

	databaseFile, err := os.Lstat(databasePath)
	if err != nil {
		_ = database.Close()
		_ = ownerLock.Close()
		return nil, ErrDatabaseUnavailable
	}
	expectedMigrations := make([]appliedMigration, len(orderedMigrations))
	for index, item := range orderedMigrations {
		expectedMigrations[index] = appliedMigration{
			version:  item.version,
			filename: item.filename,
			checksum: item.checksum,
		}
	}
	store := &Store{
		db:                 database,
		ownerLock:          ownerLock,
		busyTimeout:        options.BusyTimeout,
		databasePath:       databasePath,
		databaseFile:       databaseFile,
		expectedMigrations: expectedMigrations,
		authDigestKey:      authDigestKey,
		authDigestKeyID:    authDigestKeyID,
	}
	if err := store.initialize(ctx, orderedMigrations); err != nil {
		_ = database.Close()
		_ = ownerLock.Close()
		return nil, err
	}
	return store, nil
}

// AuthDigestKey returns a private copy of the installation authentication key.
func (store *Store) AuthDigestKey() (string, []byte, error) {
	if store == nil || store.closed.Load() || store.authDigestKeyID == "" {
		return "", nil, ErrStoreClosed
	}
	key := make([]byte, len(store.authDigestKey))
	copy(key, store.authDigestKey[:])
	return store.authDigestKeyID, key, nil
}

func inspectDatabaseIdentity(ctx context.Context, databasePath string, busyTimeout time.Duration) error {
	database, err := sql.Open("sqlite", readOnlyDatabaseDSN(databasePath, busyTimeout))
	if err != nil {
		return ErrDatabaseUnavailable
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	defer database.Close()
	if err := database.PingContext(ctx); err != nil {
		return ErrDatabaseUnavailable
	}
	if _, err := verifyDatabaseIdentity(ctx, database); err != nil {
		return err
	}
	return nil
}

func validOptions(options Options) bool {
	return options.BusyTimeout >= minimumBusyTimeout &&
		options.BusyTimeout <= maximumBusyTimeout &&
		options.BusyTimeout%time.Millisecond == 0
}

func databaseDSN(databasePath string, busyTimeout time.Duration) string {
	path := filepath.ToSlash(databasePath)
	if runtime.GOOS == "windows" && len(path) >= 2 && path[1] == ':' {
		path = "/" + path
	}
	parameters := url.Values{}
	parameters.Set("_busy_timeout", strconv.FormatInt(busyTimeout.Milliseconds(), 10))
	parameters.Set("_dqs", "0")
	parameters.Set("_error_rc", "1")
	parameters.Set("_foreign_keys", "on")
	parameters.Set("_journal_mode", "wal")
	parameters.Set("_synchronous", "full")
	parameters.Set("_txlock", "immediate")
	parameters.Set("cache", "private")
	parameters.Set("mode", "rwc")
	return (&url.URL{Scheme: "file", Path: path, RawQuery: parameters.Encode()}).String()
}

func (store *Store) initialize(ctx context.Context, orderedMigrations []migration) error {
	if err := store.db.PingContext(ctx); err != nil {
		return ErrDatabaseUnavailable
	}
	if err := verifyConnectionPolicy(ctx, store.db, store.busyTimeout); err != nil {
		return err
	}
	if err := quickCheck(ctx, store.db); err != nil {
		return err
	}
	if err := applyMigrations(ctx, store.db, orderedMigrations); err != nil {
		return err
	}
	if err := ensureInstallationMetadata(ctx, store.db); err != nil {
		return err
	}
	if err := foreignKeyCheck(ctx, store.db); err != nil {
		return err
	}
	return quickCheck(ctx, store.db)
}

func verifyConnectionPolicy(ctx context.Context, database *sql.DB, busyTimeout time.Duration) error {
	var journalMode string
	if err := database.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil || !strings.EqualFold(journalMode, "wal") {
		return ErrDatabaseUnavailable
	}
	var foreignKeys int
	if err := database.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		return ErrDatabaseUnavailable
	}
	var timeoutMilliseconds int64
	if err := database.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeoutMilliseconds); err != nil || timeoutMilliseconds != busyTimeout.Milliseconds() {
		return ErrDatabaseUnavailable
	}
	var synchronous int
	if err := database.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil || synchronous != 2 {
		return ErrDatabaseUnavailable
	}
	return nil
}

// Ping checks whether the database pool can obtain a live connection.
func (store *Store) Ping(ctx context.Context) error {
	if store == nil || store.db == nil || store.closed.Load() {
		return ErrStoreClosed
	}
	if ctx == nil || ctx.Err() != nil {
		return ErrDatabaseUnavailable
	}
	if err := store.db.PingContext(ctx); err != nil {
		return ErrDatabaseUnavailable
	}
	return nil
}

// Health checks that the store can execute a bounded read.
func (store *Store) Health(ctx context.Context) error {
	if err := store.Ping(ctx); err != nil {
		return err
	}
	currentFile, err := os.Lstat(store.databasePath)
	if err != nil || currentFile.Mode()&os.ModeSymlink != 0 || !currentFile.Mode().IsRegular() ||
		!privatePermissions(currentFile.Mode()) || !hasSingleLink(store.databasePath, currentFile) ||
		!os.SameFile(store.databaseFile, currentFile) {
		return ErrDatabaseUnavailable
	}
	var appID int64
	if err := store.db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&appID); err != nil || appID != applicationID {
		return ErrDatabaseUnavailable
	}
	applied, err := readAppliedMigrations(ctx, store.db)
	if err != nil || !appliedMigrationLedgersEqual(applied, store.expectedMigrations) {
		return ErrDatabaseUnavailable
	}
	if err := validateInstallationMetadata(ctx, store.db, false); err != nil {
		return ErrDatabaseUnavailable
	}
	key, err := readAuthDigestKey(filepath.Join(filepath.Dir(store.databasePath), authDigestKeyFilename))
	if err != nil || subtle.ConstantTimeCompare(key[:], store.authDigestKey[:]) != 1 {
		clear(key[:])
		return ErrDatabaseUnavailable
	}
	clear(key[:])
	return nil
}

func ensureInstallationMetadata(ctx context.Context, database *sql.DB) error {
	var tableCount int
	if err := database.QueryRowContext(ctx, `SELECT count(*)
FROM sqlite_schema
WHERE type = 'table' AND name = 'installation_metadata'`).Scan(&tableCount); err != nil {
		return ErrDatabaseUnavailable
	}
	if tableCount == 0 {
		return nil
	}
	if err := validateInstallationMetadata(ctx, database, true); err == nil {
		return nil
	}

	var count int
	if err := database.QueryRowContext(ctx, "SELECT count(*) FROM installation_metadata").Scan(&count); err != nil || count != 0 {
		return ErrDatabaseIntegrity
	}
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return ErrDatabaseUnavailable
	}
	installationID := "install_" + hex.EncodeToString(randomBytes[:])
	if _, err := database.ExecContext(ctx, `INSERT INTO installation_metadata
(singleton_id, installation_id, data_mode, created_at_us)
VALUES (1, ?, 'normal', ?)`, installationID, time.Now().UTC().UnixMicro()); err != nil {
		return ErrDatabaseUnavailable
	}
	return validateInstallationMetadata(ctx, database, false)
}

func validateInstallationMetadata(ctx context.Context, database *sql.DB, allowEmpty bool) error {
	var installationID string
	var dataMode string
	var fixtureVersion sql.NullString
	var createdAt int64
	var restoreGeneration int64
	err := database.QueryRowContext(ctx, `SELECT installation_id, data_mode, demo_fixture_version,
       created_at_us, restore_generation
FROM installation_metadata
WHERE singleton_id = 1`).Scan(
		&installationID,
		&dataMode,
		&fixtureVersion,
		&createdAt,
		&restoreGeneration,
	)
	if errors.Is(err, sql.ErrNoRows) && allowEmpty {
		return ErrDatabaseUnavailable
	}
	if err != nil || len(installationID) < 1 || len(installationID) > 128 || createdAt < 0 || restoreGeneration < 0 {
		return ErrDatabaseIntegrity
	}
	if dataMode == "normal" && !fixtureVersion.Valid {
		return nil
	}
	if dataMode == "guest_demo" && fixtureVersion.Valid && fixtureVersion.String != "" {
		return nil
	}
	return ErrDatabaseIntegrity
}

func appliedMigrationLedgersEqual(left, right []appliedMigration) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// Close checkpoints the WAL, closes the pool, and releases process ownership.
func (store *Store) Close() error {
	if store == nil {
		return ErrStoreClosed
	}
	store.closeOnce.Do(func() {
		store.closed.Store(true)
		defer func() {
			for index := range store.authDigestKey {
				store.authDigestKey[index] = 0
			}
			store.authDigestKeyID = ""
		}()
		failed := false
		if store.db != nil {
			checkpointTimeout := store.busyTimeout + time.Second
			ctx, cancel := context.WithTimeout(context.Background(), checkpointTimeout)
			var busy, logFrames, checkpointedFrames int
			if err := store.db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointedFrames); err != nil || busy != 0 {
				failed = true
			}
			cancel()
			if err := store.db.Close(); err != nil {
				failed = true
			}
		}
		if store.ownerLock != nil {
			if err := store.ownerLock.Close(); err != nil {
				failed = true
			}
		}
		if failed {
			store.closeErr = ErrCloseFailed
		}
	})
	return store.closeErr
}
