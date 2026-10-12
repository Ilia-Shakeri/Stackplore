package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	migrationLockKey      int64 = 0x535441434B504C52
	maximumMigrationBytes       = 4 * 1024 * 1024
)

const migrationLedgerSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version bigint PRIMARY KEY CHECK (version > 0),
    filename text NOT NULL UNIQUE CHECK (filename ~ '^[0-9]{6}_[a-z0-9_]+[.]sql$'),
    checksum_sha256 bytea NOT NULL CHECK (octet_length(checksum_sha256) = 32),
    applied_at timestamptz NOT NULL
)`

type migration struct {
	version  int64
	filename string
	checksum [sha256.Size]byte
	contents string
}

type appliedMigration struct {
	version  int64
	filename string
	checksum []byte
}

func loadMigrations(migrationFiles fs.FS) ([]migration, error) {
	if migrationFiles == nil {
		return nil, ErrMigrationRejected
	}
	entries, err := fs.ReadDir(migrationFiles, ".")
	if err != nil {
		return nil, ErrMigrationRejected
	}
	loaded := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".sql" {
			continue
		}
		version, valid := migrationVersion(entry.Name())
		if !valid {
			return nil, ErrMigrationRejected
		}
		contents, err := fs.ReadFile(migrationFiles, entry.Name())
		if err != nil || len(contents) == 0 || len(contents) > maximumMigrationBytes || strings.IndexByte(string(contents), 0) >= 0 {
			return nil, ErrMigrationRejected
		}
		loaded = append(loaded, migration{
			version:  version,
			filename: entry.Name(),
			checksum: sha256.Sum256(contents),
			contents: string(contents),
		})
	}
	if len(loaded) == 0 {
		return nil, ErrMigrationRejected
	}
	sort.Slice(loaded, func(left, right int) bool { return loaded[left].version < loaded[right].version })
	for index, item := range loaded {
		if item.version != int64(index+1) || index > 0 && loaded[index-1].filename == item.filename {
			return nil, ErrMigrationRejected
		}
	}
	return loaded, nil
}

func migrationVersion(filename string) (int64, bool) {
	if len(filename) < len("000001_a.sql") || path.Base(filename) != filename || path.Ext(filename) != ".sql" {
		return 0, false
	}
	prefix, suffix, found := strings.Cut(filename, "_")
	if !found || len(prefix) != 6 || len(suffix) < len("a.sql") {
		return 0, false
	}
	for _, character := range strings.TrimSuffix(suffix, ".sql") {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' {
			return 0, false
		}
	}
	version, err := strconv.ParseInt(prefix, 10, 64)
	return version, err == nil && version > 0
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool, migrations []migration, timeout time.Duration) (resultErr error) {
	if ctx == nil || pool == nil || len(migrations) == 0 || timeout <= 0 {
		return ErrMigrationRejected
	}
	migrationContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	connection, err := pool.Acquire(migrationContext)
	if err != nil {
		return errors.Join(ErrDatabaseUnavailable, err)
	}
	locked := false
	defer func() {
		if locked {
			unlockContext, unlockCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, unlockErr := connection.Exec(unlockContext, `SELECT pg_advisory_unlock($1)`, migrationLockKey)
			unlockCancel()
			if unlockErr != nil {
				rawConnection := connection.Hijack()
				closeContext, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
				_ = rawConnection.Close(closeContext)
				closeCancel()
				resultErr = errors.Join(ErrMigrationRejected, unlockErr)
				return
			}
		}
		connection.Release()
	}()
	if _, err := connection.Exec(migrationContext, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return errors.Join(ErrMigrationRejected, err)
	}
	locked = true

	ledgerExists, err := tableExists(migrationContext, connection, "schema_migrations")
	if err != nil {
		return errors.Join(ErrMigrationRejected, err)
	}
	if !ledgerExists {
		clean, err := schemaEmpty(migrationContext, connection)
		if err != nil || !clean {
			return ErrMigrationRejected
		}
	}
	if _, err := connection.Exec(migrationContext, migrationLedgerSQL); err != nil {
		return errors.Join(ErrMigrationRejected, err)
	}

	applied, err := readAppliedMigrations(migrationContext, connection, len(migrations)+1)
	if err != nil || len(applied) > len(migrations) {
		return ErrMigrationRejected
	}
	for index, existing := range applied {
		expected := migrations[index]
		if existing.version != expected.version || existing.filename != expected.filename ||
			hex.EncodeToString(existing.checksum) != hex.EncodeToString(expected.checksum[:]) {
			return ErrMigrationRejected
		}
	}

	for _, item := range migrations[len(applied):] {
		transaction, err := connection.BeginTx(migrationContext, pgx.TxOptions{})
		if err != nil {
			return errors.Join(ErrMigrationRejected, err)
		}
		if _, err = transaction.Exec(migrationContext, item.contents, pgx.QueryExecModeSimpleProtocol); err == nil {
			_, err = transaction.Exec(migrationContext, `INSERT INTO schema_migrations
    (version, filename, checksum_sha256, applied_at)
VALUES ($1, $2, $3, $4)`, item.version, item.filename, item.checksum[:], time.Now().UTC().Truncate(time.Microsecond))
		}
		if err != nil {
			_ = transaction.Rollback(migrationContext)
			return errors.Join(ErrMigrationRejected, err)
		}
		if err := transaction.Commit(migrationContext); err != nil {
			return errors.Join(ErrMigrationRejected, err)
		}
	}
	return nil
}

func migrationHistoryMatches(applied []appliedMigration, expected []migration) bool {
	if len(applied) != len(expected) {
		return false
	}
	for index, existing := range applied {
		wanted := expected[index]
		if existing.version != wanted.version || existing.filename != wanted.filename ||
			hex.EncodeToString(existing.checksum) != hex.EncodeToString(wanted.checksum[:]) {
			return false
		}
	}
	return true
}

type migrationConnection interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func tableExists(ctx context.Context, connection migrationConnection, table string) (bool, error) {
	var exists bool
	err := connection.QueryRow(ctx, `SELECT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = current_schema() AND table_name = $1
)`, table).Scan(&exists)
	return exists, err
}

func schemaEmpty(ctx context.Context, connection migrationConnection) (bool, error) {
	var count int64
	err := connection.QueryRow(ctx, `SELECT count(*)
FROM information_schema.tables
WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'`).Scan(&count)
	return count == 0, err
}

func readAppliedMigrations(ctx context.Context, connection migrationConnection, maximumRows int) ([]appliedMigration, error) {
	if maximumRows < 1 {
		return nil, ErrMigrationRejected
	}
	rows, err := connection.Query(ctx, `SELECT version, filename, checksum_sha256
FROM schema_migrations
ORDER BY version LIMIT $1`, maximumRows)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]appliedMigration, 0)
	for rows.Next() {
		var item appliedMigration
		if err := rows.Scan(&item.version, &item.filename, &item.checksum); err != nil || len(item.checksum) != sha256.Size {
			return nil, ErrMigrationRejected
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read migration ledger: %w", err)
	}
	return result, nil
}
