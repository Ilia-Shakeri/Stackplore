// Package testpostgres serializes disposable database fixtures across test binaries.
package testpostgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

const gateKey int64 = 0x535441434B544553

// OpenGate returns a connection holding a database-wide fixture gate until Close.
func OpenGate(ctx context.Context, connectionString string) (*pgx.Conn, error) {
	connection, err := pgx.Connect(ctx, connectionString)
	if err != nil {
		return nil, err
	}
	if _, err := connection.Exec(ctx, `SELECT pg_advisory_lock($1)`, gateKey); err != nil {
		closeContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = connection.Close(closeContext)
		return nil, err
	}
	return connection, nil
}
