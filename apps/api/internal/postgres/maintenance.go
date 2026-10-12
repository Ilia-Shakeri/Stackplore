package postgres

import (
	"context"
	"errors"
	"io/fs"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
)

// MaintenanceLock excludes the service while an offline database task runs.
type MaintenanceLock struct {
	connection *pgx.Conn
	closeOnce  sync.Once
	closeErr   error
	gate       chan struct{}
	lost       atomic.Bool
	lostOnce   sync.Once
	lostCh     chan struct{}
	stop       context.CancelFunc
	done       chan struct{}
}

// AcquireMaintenanceLock obtains the same ownership lock held by the service.
func AcquireMaintenanceLock(ctx context.Context, connectionString, schema string, timeout time.Duration) (*MaintenanceLock, error) {
	if ctx == nil || connectionString == "" || !validSchemaName(schema) || timeout <= 0 {
		return nil, ErrInvalidOptions
	}
	lockContext, cancel := boundedContext(ctx, timeout)
	defer cancel()
	connectionConfig, err := pgx.ParseConfig(connectionString)
	if err != nil {
		return nil, ErrInvalidOptions
	}
	connectionConfig.ConnectTimeout = timeout
	connectionConfig.RuntimeParams["timezone"] = "UTC"
	connectionConfig.RuntimeParams["application_name"] = "stackplore-maintenance"
	connectionConfig.RuntimeParams["search_path"] = schema
	connection, err := pgx.ConnectConfig(lockContext, connectionConfig)
	if err != nil {
		return nil, errors.Join(ErrDatabaseUnavailable, err)
	}
	var locked bool
	if err := connection.QueryRow(lockContext, `SELECT pg_try_advisory_lock($1)`, runtimeLockKey).Scan(&locked); err != nil || !locked {
		closeContext, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = connection.Close(closeContext)
		closeCancel()
		if err != nil {
			return nil, errors.Join(ErrOwnershipUnavailable, err)
		}
		return nil, ErrOwnershipUnavailable
	}
	monitorContext, stop := context.WithCancel(context.Background())
	lock := &MaintenanceLock{connection: connection, gate: make(chan struct{}, 1), lostCh: make(chan struct{}), stop: stop, done: make(chan struct{})}
	go lock.monitor(monitorContext, timeout)
	return lock, nil
}

// Empty reports whether the target schema has no application tables.
func (lock *MaintenanceLock) Empty(ctx context.Context, timeout time.Duration) (bool, error) {
	if lock == nil || ctx == nil || timeout <= 0 {
		return false, ErrStoreClosed
	}
	queryContext, cancel := boundedContext(ctx, timeout)
	defer cancel()
	if err := lock.acquire(queryContext); err != nil {
		return false, err
	}
	defer lock.release()
	if err := lock.checkOwnership(queryContext); err != nil {
		return false, err
	}
	return schemaEmpty(queryContext, lock.connection)
}

// FinalizeRestore verifies the exact migration ledger and revokes restored login state.
func (lock *MaintenanceLock) FinalizeRestore(ctx context.Context, migrationFiles fs.FS, timeout time.Duration) error {
	if lock == nil || ctx == nil || timeout <= 0 {
		return ErrStoreClosed
	}
	expected, err := loadMigrations(migrationFiles)
	if err != nil {
		return err
	}
	queryContext, cancel := boundedContext(ctx, timeout)
	defer cancel()
	if err := lock.acquire(queryContext); err != nil {
		return err
	}
	defer lock.release()
	if err := lock.checkOwnership(queryContext); err != nil {
		return err
	}
	transaction, err := lock.connection.Begin(queryContext)
	if err != nil {
		return errors.Join(ErrDatabaseUnavailable, err)
	}
	defer func() {
		rollbackContext, rollbackCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer rollbackCancel()
		_ = transaction.Rollback(rollbackContext)
	}()
	applied, err := readAppliedMigrations(queryContext, transaction, len(expected)+1)
	if err != nil || !migrationHistoryMatches(applied, expected) {
		return ErrMigrationRejected
	}
	result, err := transaction.Exec(queryContext, `UPDATE installation_metadata
SET restored_at_us = $1, restore_generation = restore_generation + 1
WHERE singleton_id = 1`, time.Now().UTC().UnixMicro())
	if err != nil || result.RowsAffected() != 1 {
		return ErrMigrationRejected
	}
	if _, err := transaction.Exec(queryContext, `DELETE FROM sessions`); err != nil {
		return errors.Join(ErrDatabaseUnavailable, err)
	}
	if _, err := transaction.Exec(queryContext, `DELETE FROM preauth_challenges`); err != nil {
		return errors.Join(ErrDatabaseUnavailable, err)
	}
	if err := transaction.Commit(queryContext); err != nil {
		return errors.Join(ErrDatabaseUnavailable, err)
	}
	return nil
}

// Close releases ownership and closes the maintenance session.
func (lock *MaintenanceLock) Close() error {
	if lock == nil || lock.gate == nil {
		return ErrStoreClosed
	}
	lock.closeOnce.Do(func() {
		if lock.stop != nil {
			lock.stop()
			<-lock.done
		}
		lock.gate <- struct{}{}
		defer lock.release()
		if lock.connection == nil {
			lock.closeErr = ErrStoreClosed
			return
		}
		closeContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var unlocked bool
		if err := lock.connection.QueryRow(closeContext, `SELECT pg_advisory_unlock($1)`, runtimeLockKey).Scan(&unlocked); err != nil || !unlocked {
			lock.closeErr = ErrOwnershipUnavailable
		}
		if err := lock.connection.Close(closeContext); err != nil {
			lock.closeErr = errors.Join(lock.closeErr, ErrDatabaseUnavailable)
		}
		lock.connection = nil
	})
	return lock.closeErr
}

// OwnershipLost closes when this maintenance session no longer owns the database.
func (lock *MaintenanceLock) OwnershipLost() <-chan struct{} {
	if lock == nil {
		return nil
	}
	return lock.lostCh
}

func (lock *MaintenanceLock) acquire(ctx context.Context) error {
	if lock == nil || lock.gate == nil || lock.lost.Load() {
		return ErrOwnershipUnavailable
	}
	select {
	case lock.gate <- struct{}{}:
		if lock.connection == nil || lock.lost.Load() {
			lock.release()
			return ErrOwnershipUnavailable
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (lock *MaintenanceLock) release() { <-lock.gate }

func (lock *MaintenanceLock) checkOwnership(ctx context.Context) error {
	var ownsLock bool
	err := lock.connection.QueryRow(ctx, `SELECT EXISTS (
    SELECT 1 FROM pg_locks WHERE locktype = 'advisory' AND pid = pg_backend_pid()
      AND classid = ($1::bigint >> 32)::oid
      AND objid = ($1::bigint & 4294967295)::oid AND objsubid = 1 AND granted
)`, int64(runtimeLockKey)).Scan(&ownsLock)
	if err == nil && ownsLock {
		return nil
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	lock.lostOnce.Do(func() { lock.lost.Store(true); close(lock.lostCh) })
	return ErrOwnershipUnavailable
}

func (lock *MaintenanceLock) monitor(ctx context.Context, timeout time.Duration) {
	defer close(lock.done)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			queryContext, cancel := boundedContext(ctx, timeout)
			if err := lock.acquire(queryContext); err != nil {
				cancel()
				if lock.lost.Load() {
					return
				}
				continue
			}
			err := lock.checkOwnership(queryContext)
			lock.release()
			cancel()
			if err != nil {
				return
			}
		}
	}
}
