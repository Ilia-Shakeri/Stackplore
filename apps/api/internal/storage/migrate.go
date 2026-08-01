package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io/fs"
	"regexp"
	"strconv"
	"time"
	"unicode/utf8"
)

const (
	applicationID            = 0x5354504c
	maximumMigrationBytes    = 1 << 20
	createMigrationLedgerSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY CHECK (version >= 1),
    filename TEXT NOT NULL UNIQUE CHECK (length(filename) BETWEEN 12 AND 128),
    checksum_sha256 TEXT NOT NULL CHECK (
        length(checksum_sha256) = 64
        AND checksum_sha256 NOT GLOB '*[^0-9a-f]*'
    ),
    applied_at_us INTEGER NOT NULL CHECK (applied_at_us >= 0)
) STRICT`
	setApplicationIDSQL = "PRAGMA application_id = 1398034508"
)

var migrationFilenamePattern = regexp.MustCompile(`^([0-9]{6})_([a-z][a-z0-9_]*)\.sql$`)

type migration struct {
	version  int64
	filename string
	checksum string
	contents string
}

type appliedMigration struct {
	version  int64
	filename string
	checksum string
}

func loadMigrations(files fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil || len(entries) == 0 {
		return nil, ErrMigrationRejected
	}
	ordered := make([]migration, 0, len(entries))
	for _, entry := range entries {
		matches := migrationFilenamePattern.FindStringSubmatch(entry.Name())
		if entry.IsDir() || !entry.Type().IsRegular() || matches == nil {
			return nil, ErrMigrationRejected
		}
		info, err := entry.Info()
		if err != nil || info.Size() <= 0 || info.Size() > maximumMigrationBytes {
			return nil, ErrMigrationRejected
		}
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil || version != int64(len(ordered)+1) {
			return nil, ErrMigrationRejected
		}
		contents, err := fs.ReadFile(files, entry.Name())
		if err != nil || len(contents) == 0 || !utf8.Valid(contents) {
			return nil, ErrMigrationRejected
		}
		for _, value := range contents {
			if value == 0 {
				return nil, ErrMigrationRejected
			}
		}
		digest := sha256.Sum256(contents)
		ordered = append(ordered, migration{
			version:  version,
			filename: entry.Name(),
			checksum: hex.EncodeToString(digest[:]),
			contents: string(contents),
		})
	}
	return ordered, nil
}

func applyMigrations(ctx context.Context, database *sql.DB, ordered []migration) error {
	claimed, err := verifyDatabaseIdentity(ctx, database)
	if err != nil {
		return err
	}
	if err := bootstrapMigrationLedger(ctx, database, claimed); err != nil {
		return err
	}
	applied, err := readAppliedMigrations(ctx, database)
	if err != nil || !migrationLedgerMatches(applied, ordered) {
		return ErrMigrationRejected
	}
	for index := len(applied); index < len(ordered); index++ {
		if err := applyMigration(ctx, database, ordered[index]); err != nil {
			return err
		}
	}

	var value int64
	if err := database.QueryRowContext(ctx, "PRAGMA application_id").Scan(&value); err != nil || value != applicationID {
		return ErrMigrationRejected
	}
	return nil
}

func validateCurrentMigrationLedger(ctx context.Context, database *sql.DB, ordered []migration) error {
	claimed, err := verifyDatabaseIdentity(ctx, database)
	if err != nil || !claimed {
		return ErrMigrationRejected
	}
	applied, err := readAppliedMigrations(ctx, database)
	if err != nil || len(applied) != len(ordered) || !migrationLedgerMatches(applied, ordered) {
		return ErrMigrationRejected
	}
	return nil
}

func verifyDatabaseIdentity(ctx context.Context, database *sql.DB) (bool, error) {
	var value int64
	if err := database.QueryRowContext(ctx, "PRAGMA application_id").Scan(&value); err != nil {
		return false, ErrDatabaseUnavailable
	}
	if value == applicationID {
		return true, nil
	}
	if value != 0 {
		return false, ErrMigrationRejected
	}

	var objectName string
	err := database.QueryRowContext(ctx, `SELECT name
FROM sqlite_schema
WHERE name NOT LIKE 'sqlite_%'
LIMIT 1`).Scan(&objectName)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, ErrDatabaseUnavailable
	}
	return false, ErrMigrationRejected
}

func bootstrapMigrationLedger(ctx context.Context, database *sql.DB, claimed bool) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return ErrMigrationRejected
	}
	committed := false
	defer func() {
		if !committed {
			_ = transaction.Rollback()
		}
	}()

	if _, err := transaction.ExecContext(ctx, createMigrationLedgerSQL); err != nil {
		return ErrMigrationRejected
	}
	if !claimed {
		if _, err := transaction.ExecContext(ctx, setApplicationIDSQL); err != nil {
			return ErrMigrationRejected
		}
	}
	if err := transaction.Commit(); err != nil {
		return ErrMigrationRejected
	}
	committed = true
	return nil
}

func readAppliedMigrations(ctx context.Context, database *sql.DB) ([]appliedMigration, error) {
	rows, err := database.QueryContext(ctx, `SELECT version, filename, checksum_sha256
FROM schema_migrations
ORDER BY version`)
	if err != nil {
		return nil, ErrMigrationRejected
	}
	defer rows.Close()

	var applied []appliedMigration
	for rows.Next() {
		var item appliedMigration
		if err := rows.Scan(&item.version, &item.filename, &item.checksum); err != nil {
			return nil, ErrMigrationRejected
		}
		applied = append(applied, item)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrMigrationRejected
	}
	return applied, nil
}

func migrationLedgerMatches(applied []appliedMigration, ordered []migration) bool {
	if len(applied) > len(ordered) {
		return false
	}
	for index, item := range applied {
		expected := ordered[index]
		if item.version != int64(index+1) || item.version != expected.version ||
			item.filename != expected.filename || item.checksum != expected.checksum {
			return false
		}
	}
	return true
}

func applyMigration(ctx context.Context, database *sql.DB, item migration) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return ErrMigrationRejected
	}
	committed := false
	defer func() {
		if !committed {
			_ = transaction.Rollback()
		}
	}()

	if _, err := transaction.ExecContext(ctx, item.contents); err != nil {
		return ErrMigrationRejected
	}
	if _, err := transaction.ExecContext(
		ctx,
		`INSERT INTO schema_migrations (version, filename, checksum_sha256, applied_at_us)
VALUES (?, ?, ?, ?)`,
		item.version,
		item.filename,
		item.checksum,
		time.Now().UTC().UnixMicro(),
	); err != nil {
		return ErrMigrationRejected
	}
	if err := transaction.Commit(); err != nil {
		return ErrMigrationRejected
	}
	committed = true
	return nil
}

func foreignKeyCheck(ctx context.Context, database *sql.DB) error {
	rows, err := database.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return ErrDatabaseIntegrity
	}
	defer rows.Close()
	if rows.Next() {
		return ErrDatabaseIntegrity
	}
	if err := rows.Err(); err != nil {
		return ErrDatabaseIntegrity
	}
	return nil
}

func quickCheck(ctx context.Context, database *sql.DB) error {
	rows, err := database.QueryContext(ctx, "PRAGMA quick_check(1)")
	if err != nil {
		return ErrDatabaseIntegrity
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil || result != "ok" {
			return ErrDatabaseIntegrity
		}
		count++
	}
	if err := rows.Err(); err != nil || count != 1 {
		return ErrDatabaseIntegrity
	}
	return nil
}

func integrityCheck(ctx context.Context, database *sql.DB) error {
	rows, err := database.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return ErrDatabaseIntegrity
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil || result != "ok" {
			return ErrDatabaseIntegrity
		}
		count++
	}
	if err := rows.Err(); err != nil || count != 1 {
		return ErrDatabaseIntegrity
	}
	return nil
}
