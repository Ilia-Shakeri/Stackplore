// Package postgres owns the PostgreSQL pool, migration, health, and transaction boundary.
package postgres

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io/fs"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	minimumConnections = 1
	maximumConnections = 256
	authDigestKeyBytes = 32
	minimumPoolSize    = 2
	runtimeLockKey     = 0x535441434B52554E
)

type Options struct {
	ConnectionString          string
	Schema                    string
	MinimumConnections        int32
	MaximumConnections        int32
	ConnectTimeout            time.Duration
	QueryTimeout              time.Duration
	MigrationTimeout          time.Duration
	HealthTimeout             time.Duration
	ShutdownTimeout           time.Duration
	MaximumConnectionLifetime time.Duration
	MaximumConnectionIdleTime time.Duration
	AuthDigestKey             [authDigestKeyBytes]byte
	AuthDigestKeyID           string
}

type Store struct {
	pool               *pgxpool.Pool
	queryTimeout       time.Duration
	healthTimeout      time.Duration
	shutdownTimeout    time.Duration
	closed             atomic.Bool
	closeOnce          sync.Once
	authDigestKey      [authDigestKeyBytes]byte
	authDigestKeyMu    sync.RWMutex
	authDigestKeyID    string
	expectedMigrations []migration
	runtimeLock        *pgxpool.Conn
	healthGate         chan struct{}
	ownershipLost      atomic.Bool
	ownershipLostOnce  sync.Once
	ownershipLostCh    chan struct{}
	lifetimeContext    context.Context
	stopLifetime       context.CancelFunc
	closeErr           error
}

func Open(ctx context.Context, options Options, migrationFiles fs.FS) (*Store, error) {
	if ctx == nil || ctx.Err() != nil || !validOptions(options) {
		return nil, ErrInvalidOptions
	}
	migrations, err := loadMigrations(migrationFiles)
	if err != nil {
		return nil, err
	}
	poolConfig, err := pgxpool.ParseConfig(options.ConnectionString)
	if err != nil {
		return nil, ErrInvalidOptions
	}
	poolConfig.MinConns = options.MinimumConnections
	poolConfig.MaxConns = options.MaximumConnections
	poolConfig.MaxConnLifetime = options.MaximumConnectionLifetime
	poolConfig.MaxConnIdleTime = options.MaximumConnectionIdleTime
	poolConfig.HealthCheckPeriod = minDuration(options.HealthTimeout*5, 30*time.Second)
	poolConfig.ConnConfig.ConnectTimeout = options.ConnectTimeout
	poolConfig.ConnConfig.RuntimeParams["timezone"] = "UTC"
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "stackplore-api"
	poolConfig.ConnConfig.RuntimeParams["search_path"] = options.Schema

	connectContext, cancel := context.WithTimeout(ctx, options.ConnectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(connectContext, poolConfig)
	if err != nil {
		return nil, errors.Join(ErrDatabaseUnavailable, err)
	}
	if err := pool.Ping(connectContext); err != nil {
		pool.Close()
		return nil, errors.Join(ErrDatabaseUnavailable, err)
	}
	runtimeLock, err := acquireRuntimeLock(ctx, pool, options.ConnectTimeout)
	if err != nil {
		pool.Close()
		return nil, err
	}
	if err := applyMigrations(ctx, pool, migrations, options.MigrationTimeout); err != nil {
		_ = releaseRuntimeLock(runtimeLock)
		pool.Close()
		return nil, err
	}
	if err := ensureInstallationMetadata(ctx, pool, options.QueryTimeout); err != nil {
		_ = releaseRuntimeLock(runtimeLock)
		pool.Close()
		return nil, err
	}
	lifetimeContext, stopLifetime := context.WithCancel(context.Background())
	store := &Store{
		pool:               pool,
		queryTimeout:       options.QueryTimeout,
		healthTimeout:      options.HealthTimeout,
		shutdownTimeout:    options.ShutdownTimeout,
		authDigestKey:      options.AuthDigestKey,
		authDigestKeyID:    options.AuthDigestKeyID,
		expectedMigrations: migrations,
		runtimeLock:        runtimeLock,
		healthGate:         make(chan struct{}, 1),
		ownershipLostCh:    make(chan struct{}),
		lifetimeContext:    lifetimeContext,
		stopLifetime:       stopLifetime,
	}
	if err := store.Health(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (store *Store) AuthDigestKey() (string, []byte, error) {
	if store == nil {
		return "", nil, ErrStoreClosed
	}
	store.authDigestKeyMu.RLock()
	defer store.authDigestKeyMu.RUnlock()
	if !store.available() || store.authDigestKeyID == "" {
		return "", nil, ErrStoreClosed
	}
	key := make([]byte, authDigestKeyBytes)
	copy(key, store.authDigestKey[:])
	return store.authDigestKeyID, key, nil
}

func (store *Store) Health(ctx context.Context) error {
	if !store.available() {
		if store != nil && store.ownershipLost.Load() {
			return ErrOwnershipUnavailable
		}
		return ErrStoreClosed
	}
	if ctx == nil || ctx.Err() != nil {
		return ErrDatabaseUnavailable
	}
	healthContext, cancel := store.queryContext(ctx, store.healthTimeout)
	defer cancel()
	select {
	case store.healthGate <- struct{}{}:
		defer func() { <-store.healthGate }()
	case <-healthContext.Done():
		return ErrDatabaseUnavailable
	}
	if !store.available() {
		return ErrStoreClosed
	}
	var ownsLock bool
	lockErr := ErrOwnershipUnavailable
	if store.runtimeLock != nil {
		lockErr = store.runtimeLock.QueryRow(healthContext, `SELECT EXISTS (
    SELECT 1 FROM pg_locks WHERE locktype = 'advisory' AND pid = pg_backend_pid()
      AND classid = ($1::bigint >> 32)::oid
      AND objid = ($1::bigint & 4294967295)::oid AND objsubid = 1 AND granted
)`, int64(runtimeLockKey)).Scan(&ownsLock)
	}
	if lockErr != nil || !ownsLock {
		if errors.Is(ctx.Err(), context.Canceled) || store.closed.Load() {
			return ErrDatabaseUnavailable
		}
		store.loseOwnership()
		return ErrOwnershipUnavailable
	}
	if err := store.pool.Ping(healthContext); err != nil {
		return ErrDatabaseUnavailable
	}
	applied, err := readAppliedMigrations(healthContext, store.pool, len(store.expectedMigrations)+1)
	if err != nil || !migrationHistoryMatches(applied, store.expectedMigrations) {
		return ErrDatabaseUnavailable
	}
	return nil
}

func (store *Store) Close() error {
	if store == nil || store.pool == nil || store.healthGate == nil {
		return ErrStoreClosed
	}
	store.closeOnce.Do(func() {
		store.closed.Store(true)
		if store.stopLifetime != nil {
			store.stopLifetime()
		}
		store.healthGate <- struct{}{}
		defer func() { <-store.healthGate }()
		store.closeErr = releaseRuntimeLock(store.runtimeLock)
		store.runtimeLock = nil
		poolClosed := make(chan struct{})
		go func() {
			store.pool.Close()
			close(poolClosed)
		}()
		select {
		case <-poolClosed:
			store.authDigestKeyMu.Lock()
			clear(store.authDigestKey[:])
			store.authDigestKeyMu.Unlock()
		case <-time.After(store.shutdownTimeout):
			store.closeErr = errors.Join(store.closeErr, ErrDatabaseUnavailable)
		}
	})
	return store.closeErr
}

func (store *Store) snapshotAuthDigestKey() [authDigestKeyBytes]byte {
	store.authDigestKeyMu.RLock()
	defer store.authDigestKeyMu.RUnlock()
	return store.authDigestKey
}

// OwnershipLost closes once ownership can no longer be proved; a new process must recover it.
func (store *Store) OwnershipLost() <-chan struct{} {
	if store == nil {
		return nil
	}
	return store.ownershipLostCh
}

func (store *Store) available() bool {
	return store != nil && store.pool != nil && !store.closed.Load() && !store.ownershipLost.Load()
}

func (store *Store) loseOwnership() {
	store.ownershipLostOnce.Do(func() {
		store.ownershipLost.Store(true)
		if store.stopLifetime != nil {
			store.stopLifetime()
		}
		close(store.ownershipLostCh)
	})
}

func (store *Store) queryContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	queryContext, cancel := boundedContext(ctx, timeout)
	stop := func() bool { return true }
	if store != nil && store.lifetimeContext != nil {
		stop = context.AfterFunc(store.lifetimeContext, cancel)
	}
	return queryContext, func() { stop(); cancel() }
}

func acquireRuntimeLock(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) (*pgxpool.Conn, error) {
	if ctx == nil || pool == nil || timeout <= 0 {
		return nil, ErrOwnershipUnavailable
	}
	lockContext, cancel := boundedContext(ctx, timeout)
	defer cancel()
	connection, err := pool.Acquire(lockContext)
	if err != nil {
		return nil, errors.Join(ErrOwnershipUnavailable, err)
	}
	var locked bool
	if err := connection.QueryRow(lockContext, `SELECT pg_try_advisory_lock($1)`, runtimeLockKey).Scan(&locked); err != nil || !locked {
		connection.Release()
		if err != nil {
			return nil, errors.Join(ErrOwnershipUnavailable, err)
		}
		return nil, ErrOwnershipUnavailable
	}
	return connection, nil
}

func releaseRuntimeLock(connection *pgxpool.Conn) error {
	if connection == nil {
		return ErrOwnershipUnavailable
	}
	unlockContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var unlocked bool
	if err := connection.QueryRow(unlockContext, `SELECT pg_advisory_unlock($1)`, runtimeLockKey).Scan(&unlocked); err != nil || !unlocked {
		rawConnection := connection.Hijack()
		closeContext, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = rawConnection.Close(closeContext)
		closeCancel()
		if err != nil {
			return errors.Join(ErrOwnershipUnavailable, err)
		}
		return ErrOwnershipUnavailable
	}
	connection.Release()
	return nil
}

func ensureInstallationMetadata(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) error {
	if ctx == nil || pool == nil || timeout <= 0 {
		return ErrDatabaseUnavailable
	}
	queryContext, cancel := boundedContext(ctx, timeout)
	defer cancel()
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return ErrDatabaseUnavailable
	}
	installationID := "install_" + hex.EncodeToString(randomBytes[:])
	if _, err := pool.Exec(queryContext, `INSERT INTO installation_metadata
    (singleton_id, installation_id, data_mode, created_at_us)
VALUES (1, $1, 'normal', $2)
ON CONFLICT (singleton_id) DO NOTHING`, installationID, time.Now().UTC().UnixMicro()); err != nil {
		return errors.Join(ErrDatabaseUnavailable, err)
	}
	var storedID string
	var dataMode string
	var fixtureVersion *string
	var createdAt int64
	var restoreGeneration int64
	if err := pool.QueryRow(queryContext, `SELECT installation_id, data_mode, demo_fixture_version,
       created_at_us, restore_generation
FROM installation_metadata
WHERE singleton_id = 1`).Scan(&storedID, &dataMode, &fixtureVersion, &createdAt, &restoreGeneration); err != nil {
		return errors.Join(ErrDatabaseUnavailable, err)
	}
	validMode := dataMode == "normal" && fixtureVersion == nil ||
		dataMode == "guest_demo" && fixtureVersion != nil && *fixtureVersion != ""
	if storedID == "" || len(storedID) > 128 || createdAt < 0 || restoreGeneration < 0 || !validMode {
		return ErrMigrationRejected
	}
	return nil
}

func validOptions(options Options) bool {
	return options.ConnectionString != "" &&
		validSchemaName(options.Schema) &&
		options.MinimumConnections >= minimumConnections &&
		options.MaximumConnections >= options.MinimumConnections &&
		options.MaximumConnections >= minimumPoolSize &&
		options.MaximumConnections <= maximumConnections &&
		options.ConnectTimeout >= time.Second && options.ConnectTimeout <= 30*time.Second &&
		options.QueryTimeout >= 100*time.Millisecond && options.QueryTimeout <= 2*time.Minute &&
		options.MigrationTimeout >= 5*time.Second && options.MigrationTimeout <= 15*time.Minute &&
		options.HealthTimeout >= 100*time.Millisecond && options.HealthTimeout <= 10*time.Second &&
		options.ShutdownTimeout >= time.Second && options.ShutdownTimeout <= 2*time.Minute &&
		options.MaximumConnectionLifetime >= time.Minute && options.MaximumConnectionLifetime <= 24*time.Hour &&
		options.MaximumConnectionIdleTime >= time.Second && options.MaximumConnectionIdleTime <= time.Hour &&
		validDigestKey(options.AuthDigestKey, options.AuthDigestKeyID)
}

func validDigestKey(key [authDigestKeyBytes]byte, keyID string) bool {
	if !validSchemaName(keyID) {
		return false
	}
	var zero [authDigestKeyBytes]byte
	return subtle.ConstantTimeCompare(key[:], zero[:]) == 0
}

func validSchemaName(value string) bool {
	if value == "" || len(value) > 63 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, character := range value[1:] {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}

func boundedContext(parent context.Context, maximum time.Duration) (context.Context, context.CancelFunc) {
	if deadline, exists := parent.Deadline(); exists && time.Until(deadline) <= maximum {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, maximum)
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}
