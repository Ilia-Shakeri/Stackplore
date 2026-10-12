// Package postgresops owns safe local PostgreSQL backup and restore process boundaries.
package postgresops

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/postgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runtimefiles"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
)

var (
	ErrInvalidOptions  = errors.New("invalid PostgreSQL operation options")
	ErrToolUnavailable = errors.New("PostgreSQL client tool unavailable")
	ErrBackupRejected  = errors.New("PostgreSQL backup rejected")
	ErrRestoreRejected = errors.New("PostgreSQL restore rejected")
)

type Options struct {
	Settings config.Config
	Path     string
}

const operationTimeout = 15 * time.Minute

type commandSpec struct {
	name string
	args []string
	env  []string
}

var execute = executeCommand

// Backup creates and validates one PostgreSQL custom-format archive before atomic publication.
func Backup(ctx context.Context, options Options) error {
	if err := validateOptions(ctx, options); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if _, err := os.Lstat(options.Path); !errors.Is(err, os.ErrNotExist) {
		return ErrBackupRejected
	}
	parent := filepath.Dir(options.Path)
	temporary, err := os.CreateTemp(parent, ".stackplore-backup-*.dump")
	if err != nil {
		return ErrBackupRejected
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil || temporary.Close() != nil {
		return ErrBackupRejected
	}
	connection, err := parseConnection(options.Settings.DatabaseURL.Reveal())
	if err != nil {
		return err
	}
	if err := execute(ctx, commandSpec{
		name: "pg_dump",
		args: append(connection.args(), "--schema=public", "--format=custom", "--no-owner", "--no-privileges", "--file", temporaryPath),
		env:  connection.environment(),
	}); err != nil {
		return errors.Join(ErrBackupRejected, err)
	}
	if err := validateArchive(ctx, connection, temporaryPath); err != nil {
		return errors.Join(ErrBackupRejected, err)
	}
	info, err := os.Stat(temporaryPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return ErrBackupRejected
	}
	archive, err := os.OpenFile(temporaryPath, os.O_RDWR, 0)
	if err != nil {
		return ErrBackupRejected
	}
	syncErr := archive.Sync()
	closeErr := archive.Close()
	if syncErr != nil || closeErr != nil {
		return ErrBackupRejected
	}
	if err := os.Link(temporaryPath, options.Path); err != nil {
		return ErrBackupRejected
	}
	if err := runtimefiles.SyncDirectory(parent); err != nil {
		return ErrBackupRejected
	}
	return nil
}

// Restore installs an archive only into an empty target database, then verifies and seals it.
func Restore(ctx context.Context, options Options) (result error) {
	if err := validateOptions(ctx, options); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	info, err := os.Stat(options.Path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return ErrRestoreRejected
	}
	connection, err := parseConnection(options.Settings.DatabaseURL.Reveal())
	if err != nil {
		return err
	}
	if err := validateArchive(ctx, connection, options.Path); err != nil {
		return errors.Join(ErrRestoreRejected, err)
	}
	lock, err := postgres.AcquireMaintenanceLock(ctx, options.Settings.DatabaseURL.Reveal(), "public", options.Settings.DatabaseConnectTimeout)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, lock.Close()) }()
	jobContext, stopJob := context.WithCancel(ctx)
	defer stopJob()
	go func() {
		select {
		case <-lock.OwnershipLost():
			stopJob()
		case <-jobContext.Done():
		}
	}()
	ctx = jobContext
	empty, err := lock.Empty(ctx, options.Settings.DatabaseQueryTimeout)
	if err != nil || !empty {
		return ErrRestoreRejected
	}
	if err := execute(ctx, commandSpec{
		name: "pg_restore",
		args: append(connection.args(), "--schema=public", "--single-transaction", "--exit-on-error", "--no-owner", "--no-privileges", options.Path),
		env:  connection.environment(),
	}); err != nil {
		return errors.Join(ErrRestoreRejected, err)
	}
	if err := lock.FinalizeRestore(ctx, postgresmigrations.FS, options.Settings.DatabaseMigrationTimeout); err != nil {
		return errors.Join(ErrRestoreRejected, err)
	}
	return nil
}

func validateOptions(ctx context.Context, options Options) error {
	if ctx == nil || ctx.Err() != nil || config.Validate(options.Settings) != nil || options.Path == "" || !filepath.IsAbs(options.Path) {
		return ErrInvalidOptions
	}
	return nil
}

func validateArchive(ctx context.Context, connection toolConnection, path string) error {
	if err := execute(ctx, commandSpec{name: "pg_restore", args: []string{"--list", path}, env: connection.environment()}); err != nil {
		return err
	}
	return nil
}

func executeCommand(ctx context.Context, specification commandSpec) error {
	path, err := exec.LookPath(specification.name)
	if err != nil {
		return ErrToolUnavailable
	}
	command := exec.CommandContext(ctx, path, specification.args...)
	command.Env = specification.env
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", specification.name, err)
	}
	return nil
}

type toolConnection struct {
	host     string
	port     string
	user     string
	database string
	password string
	query    url.Values
}

func parseConnection(value string) (toolConnection, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User == nil || parsed.Hostname() == "" || len(parsed.Path) < 2 {
		return toolConnection{}, ErrInvalidOptions
	}
	password, _ := parsed.User.Password()
	database, err := url.PathUnescape(strings.TrimPrefix(parsed.Path, "/"))
	if err != nil || database == "" {
		return toolConnection{}, ErrInvalidOptions
	}
	return toolConnection{
		host: parsed.Hostname(), port: parsed.Port(), user: parsed.User.Username(), database: database,
		password: password, query: parsed.Query(),
	}, nil
}

func (connection toolConnection) args() []string {
	result := []string{"--host", connection.host, "--username", connection.user, "--dbname", connection.database, "--no-password"}
	if connection.port != "" {
		result = append(result, "--port", connection.port)
	}
	return result
}

func (connection toolConnection) environment() []string {
	blocked := map[string]struct{}{
		"PGPASSWORD": {}, "PGSSLMODE": {}, "PGSSLROOTCERT": {}, "PGSSLCERT": {}, "PGSSLKEY": {},
		"PGSSLPASSWORD": {}, "PGAPPNAME": {}, "PGTARGETSESSIONATTRS": {},
	}
	result := make([]string, 0, len(os.Environ())+8)
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, exists := blocked[strings.ToUpper(key)]; !exists {
			result = append(result, item)
		}
	}
	values := map[string]string{
		"PGPASSWORD": connection.password, "PGSSLMODE": connection.query.Get("sslmode"),
		"PGSSLROOTCERT": connection.query.Get("sslrootcert"), "PGSSLCERT": connection.query.Get("sslcert"),
		"PGSSLKEY": connection.query.Get("sslkey"), "PGSSLPASSWORD": connection.query.Get("sslpassword"),
		"PGAPPNAME": connection.query.Get("application_name"), "PGTARGETSESSIONATTRS": connection.query.Get("target_session_attrs"),
	}
	for _, key := range []string{"PGPASSWORD", "PGSSLMODE", "PGSSLROOTCERT", "PGSSLCERT", "PGSSLKEY", "PGSSLPASSWORD", "PGAPPNAME", "PGTARGETSESSIONATTRS"} {
		if values[key] != "" {
			result = append(result, key+"="+values[key])
		}
	}
	return result
}
