package storage

import "errors"

var (
	ErrInvalidOptions       = errors.New("invalid storage options")
	ErrUnsafeDataDirectory  = errors.New("unsafe storage data directory")
	ErrOwnershipUnavailable = errors.New("storage ownership unavailable")
	ErrDatabaseUnavailable  = errors.New("storage database unavailable")
	ErrDatabaseIntegrity    = errors.New("storage database integrity check failed")
	ErrMigrationRejected    = errors.New("storage migration rejected")
	ErrTransactionFailed    = errors.New("storage transaction failed")
	ErrRetentionRejected    = errors.New("storage retention rejected")
	ErrBackupRejected       = errors.New("storage backup rejected")
	ErrRestoreRejected      = errors.New("storage restore rejected")
	ErrStoreClosed          = errors.New("storage is closed")
	ErrCloseFailed          = errors.New("storage close failed")
)
