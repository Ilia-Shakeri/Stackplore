package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strconv"
	"time"
)

type transaction struct {
	transaction *sql.Tx
}

func (store *Store) withinTransaction(ctx context.Context, work func(*transaction) error) (err error) {
	if store == nil || store.db == nil || store.closed.Load() {
		return ErrStoreClosed
	}
	if ctx == nil || ctx.Err() != nil || work == nil {
		return ErrInvalidOptions
	}

	connection, err := store.db.Conn(ctx)
	if err != nil {
		return ErrTransactionFailed
	}
	effectiveBusyTimeout := contextBusyTimeout(ctx, store.busyTimeout)
	adjusted := effectiveBusyTimeout != store.busyTimeout
	if adjusted && setBusyTimeout(ctx, connection, effectiveBusyTimeout) != nil {
		_ = connection.Close()
		return ErrTransactionFailed
	}
	defer func() {
		if adjusted {
			restoreContext, cancel := context.WithTimeout(context.Background(), store.busyTimeout)
			restoreErr := setBusyTimeout(restoreContext, connection, store.busyTimeout)
			cancel()
			if restoreErr != nil {
				_ = connection.Raw(func(any) error { return driver.ErrBadConn })
				if err == nil {
					err = ErrTransactionFailed
				}
			}
		}
		if closeErr := connection.Close(); closeErr != nil && err == nil {
			err = ErrTransactionFailed
		}
	}()

	sqlTransaction, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return ErrTransactionFailed
	}
	committed := false
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = sqlTransaction.Rollback()
			panic(recovered)
		}
		if !committed {
			_ = sqlTransaction.Rollback()
		}
	}()

	if err := work(&transaction{transaction: sqlTransaction}); err != nil {
		return err
	}
	if err := sqlTransaction.Commit(); err != nil {
		return ErrTransactionFailed
	}
	committed = true
	return nil
}

func contextBusyTimeout(ctx context.Context, configured time.Duration) time.Duration {
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		return configured
	}
	remaining := time.Until(deadline)
	if remaining >= configured {
		return configured
	}
	if remaining < time.Millisecond {
		return time.Millisecond
	}
	return remaining.Truncate(time.Millisecond)
}

func setBusyTimeout(ctx context.Context, connection *sql.Conn, timeout time.Duration) error {
	statement := "PRAGMA busy_timeout = " + strconv.FormatInt(timeout.Milliseconds(), 10)
	_, err := connection.ExecContext(ctx, statement)
	return err
}
