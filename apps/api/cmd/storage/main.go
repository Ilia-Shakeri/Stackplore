package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/observability"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/storage"
)

const maximumCommandPathBytes = 4096

const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

var (
	errInvalidArguments = errors.New("storage command arguments are invalid")
	errCommandBoundary  = errors.New("storage command boundary is invalid")
)

type commandName string

const (
	commandBackup  commandName = "backup"
	commandRestore commandName = "restore"
)

type command struct {
	name commandName
	path string
}

type storageOperations struct {
	backup  func(context.Context, storage.BackupOptions) error
	restore func(context.Context, storage.RestoreOptions) error
}

type contextFactory func() (context.Context, context.CancelFunc)

func main() {
	os.Exit(realMain())
}

func realMain() int {
	return runMain(
		os.Args[1:],
		os.Stderr,
		config.OSGetter{},
		processContext,
		storageOperations{
			backup:  storage.Backup,
			restore: storage.Restore,
		},
	)
}

func processContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func runMain(
	arguments []string,
	standardError io.Writer,
	getter config.Getter,
	newContext contextFactory,
	operations storageOperations,
) int {
	bootstrap, err := observability.New(standardError, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		return exitFailure
	}

	parsed, err := parseCommand(arguments)
	if err != nil {
		bootstrap.Error("storage command denied", "reason", "invalid_arguments")
		return exitUsage
	}
	settings, err := config.Load(getter)
	if err != nil {
		bootstrap.Error("storage command denied", "reason", "invalid_configuration")
		return exitFailure
	}
	logger, err := observability.New(standardError, settings.LogLevel, settings.LogFormat)
	if err != nil {
		bootstrap.Error("storage command denied", "reason", "logger_configuration")
		return exitFailure
	}
	if newContext == nil {
		logger.Error("storage command denied", "reason", "command_boundary")
		return exitFailure
	}
	ctx, stop := newContext()
	if ctx == nil || stop == nil {
		logger.Error("storage command denied", "reason", "command_boundary")
		return exitFailure
	}
	defer stop()

	if err := runCommand(ctx, settings, parsed, operations); err != nil {
		logger.Error("storage command failed", "operation", string(parsed.name), "reason", failureReason(err))
		return exitFailure
	}
	logger.Info("storage command complete", "operation", string(parsed.name))
	return exitSuccess
}

func parseCommand(arguments []string) (command, error) {
	if len(arguments) != 3 {
		return command{}, errInvalidArguments
	}

	var parsed command
	switch arguments[0] {
	case string(commandBackup):
		if arguments[1] != "--destination" {
			return command{}, errInvalidArguments
		}
		parsed.name = commandBackup
	case string(commandRestore):
		if arguments[1] != "--source" {
			return command{}, errInvalidArguments
		}
		parsed.name = commandRestore
	default:
		return command{}, errInvalidArguments
	}
	if !validCommandPath(arguments[2]) {
		return command{}, errInvalidArguments
	}
	parsed.path = arguments[2]
	return parsed, nil
}

func validCommandPath(value string) bool {
	if value == "" ||
		len(value) > maximumCommandPathBytes ||
		!utf8.ValidString(value) ||
		value != strings.TrimSpace(value) ||
		filepath.Clean(value) != value ||
		!filepath.IsAbs(value) ||
		isFilesystemRoot(value) ||
		isRemoteOrDevicePath(value) ||
		hasWindowsDeviceName(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func isFilesystemRoot(value string) bool {
	cleaned := filepath.Clean(value)
	return filepath.Dir(cleaned) == cleaned
}

func isRemoteOrDevicePath(value string) bool {
	upper := strings.ToUpper(value)
	return strings.HasPrefix(value, `\\`) ||
		strings.HasPrefix(value, "//") ||
		strings.HasPrefix(upper, `\??\`) ||
		strings.HasPrefix(upper, `\DEVICE\`) ||
		strings.HasPrefix(upper, `\GLOBALROOT\`)
}

func hasWindowsDeviceName(value string) bool {
	components := strings.FieldsFunc(value, func(character rune) bool {
		return character == '/' || character == '\\'
	})
	for _, component := range components {
		component = strings.TrimRight(component, ". ")
		base, _, _ := strings.Cut(component, ".")
		upper := strings.ToUpper(base)
		switch upper {
		case "CON", "PRN", "AUX", "NUL", "CLOCK$", "CONIN$", "CONOUT$":
			return true
		}
		characters := []rune(upper)
		if len(characters) == 4 &&
			(string(characters[:3]) == "COM" || string(characters[:3]) == "LPT") &&
			isWindowsDeviceDigit(characters[3]) {
			return true
		}
	}
	return false
}

func isWindowsDeviceDigit(character rune) bool {
	return character >= '1' && character <= '9' ||
		character == '\u00b9' ||
		character == '\u00b2' ||
		character == '\u00b3'
}

func runCommand(ctx context.Context, settings config.Config, parsed command, operations storageOperations) error {
	if ctx == nil || config.Validate(settings) != nil || !validCommandPath(parsed.path) {
		return errCommandBoundary
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	switch parsed.name {
	case commandBackup:
		if operations.backup == nil {
			return errCommandBoundary
		}
		return operations.backup(ctx, storage.BackupOptions{
			DataDirectory: settings.DataDirectory,
			BusyTimeout:   settings.DatabaseBusyTimeout,
			Destination:   parsed.path,
		})
	case commandRestore:
		if operations.restore == nil {
			return errCommandBoundary
		}
		return operations.restore(ctx, storage.RestoreOptions{
			DataDirectory: settings.DataDirectory,
			BusyTimeout:   settings.DatabaseBusyTimeout,
			Source:        parsed.path,
		})
	default:
		return errCommandBoundary
	}
}

func failureReason(err error) string {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	case errors.Is(err, errCommandBoundary), errors.Is(err, storage.ErrInvalidOptions):
		return "invalid_options"
	case errors.Is(err, storage.ErrUnsafeDataDirectory):
		return "unsafe_data_directory"
	case errors.Is(err, storage.ErrOwnershipUnavailable):
		return "ownership_unavailable"
	case errors.Is(err, storage.ErrDatabaseUnavailable):
		return "database_unavailable"
	case errors.Is(err, storage.ErrDatabaseIntegrity):
		return "database_integrity"
	case errors.Is(err, storage.ErrMigrationRejected):
		return "migration_rejected"
	case errors.Is(err, storage.ErrBackupRejected):
		return "backup_rejected"
	case errors.Is(err, storage.ErrRestoreRejected):
		return "restore_rejected"
	default:
		return "operation_failed"
	}
}
