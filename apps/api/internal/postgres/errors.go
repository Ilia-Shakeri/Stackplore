package postgres

import "errors"

var (
	ErrInvalidOptions       = errors.New("invalid PostgreSQL options")
	ErrDatabaseUnavailable  = errors.New("PostgreSQL is unavailable")
	ErrMigrationRejected    = errors.New("PostgreSQL migration rejected")
	ErrOwnershipUnavailable = errors.New("PostgreSQL runtime ownership unavailable")
	ErrStoreClosed          = errors.New("PostgreSQL store is closed")
)
