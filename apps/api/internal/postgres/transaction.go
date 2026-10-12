package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type transaction struct {
	transaction pgx.Tx
	context     context.Context
}

const authTransactionLockKey int64 = 0x535441434B415554

func (store *Store) withinAuthTransaction(ctx context.Context, work func(context.Context, *transaction) error) error {
	return store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if _, err := transaction.transaction.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, authTransactionLockKey); err != nil {
			return err
		}
		return work(ctx, transaction)
	})
}

func (store *Store) withinTransaction(ctx context.Context, work func(context.Context, *transaction) error) (err error) {
	if !store.available() {
		return ErrStoreClosed
	}
	if ctx == nil || ctx.Err() != nil || work == nil {
		return ErrInvalidOptions
	}
	transactionContext, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	sqlTransaction, err := store.pool.BeginTx(transactionContext, pgx.TxOptions{})
	if err != nil {
		return errors.Join(ErrDatabaseUnavailable, err)
	}
	if _, err := sqlTransaction.Exec(
		transactionContext,
		`SELECT set_config('statement_timeout', $1, true)`,
		strconv.FormatInt(store.queryTimeout.Milliseconds(), 10)+"ms",
	); err != nil {
		rollbackContext, rollbackCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = sqlTransaction.Rollback(rollbackContext)
		rollbackCancel()
		return errors.Join(ErrDatabaseUnavailable, err)
	}
	committed := false
	defer func() {
		if recovered := recover(); recovered != nil {
			rollbackContext, rollbackCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = sqlTransaction.Rollback(rollbackContext)
			rollbackCancel()
			panic(recovered)
		}
		if !committed {
			rollbackContext, rollbackCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = sqlTransaction.Rollback(rollbackContext)
			rollbackCancel()
		}
	}()
	if err := work(transactionContext, &transaction{transaction: sqlTransaction, context: transactionContext}); err != nil {
		return err
	}
	if err := sqlTransaction.Commit(transactionContext); err != nil {
		return errors.Join(ErrDatabaseUnavailable, err)
	}
	committed = true
	return nil
}
