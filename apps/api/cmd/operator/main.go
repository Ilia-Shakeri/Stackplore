package main

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/observability"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/storage"
)

const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2

	commandRecoverAdministrator = "recover-admin"
	operatorActorID             = "local_console"
	requestRandomBytes          = 16
	maximumDisplayArgumentBytes = auth.MaximumDisplayRunes * utf8.UTFMax
	successOutput               = "administrator recovery complete\n"
)

var (
	errInvalidArguments = errors.New("operator command arguments are invalid")
	errInvalidPassword  = errors.New("operator password input is invalid")
	errCommandBoundary  = errors.New("operator command boundary is invalid")
	errRequestIdentity  = errors.New("operator request identity is unavailable")
)

type command struct {
	loginName   string
	displayName string
}

type passwordInput []byte

func (passwordInput) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected password]")
}

type recoveryRequest struct {
	dataDirectory string
	busyTimeout   time.Duration
	loginName     string
	displayName   string
	password      passwordInput
	requestID     domain.RequestID
}

type recoveryOperation func(context.Context, recoveryRequest) error
type contextFactory func() (context.Context, context.CancelFunc)

func main() {
	os.Exit(realMain())
}

func realMain() int {
	return runMain(
		os.Args[1:], os.Stdin, os.Stdout, os.Stderr, config.OSGetter{}, processContext,
		recoverAdministrator, cryptorand.Reader,
	)
}

func processContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func runMain(
	arguments []string,
	standardInput io.Reader,
	standardOutput io.Writer,
	standardError io.Writer,
	getter config.Getter,
	newContext contextFactory,
	recover recoveryOperation,
	random io.Reader,
) int {
	bootstrap, err := observability.New(standardError, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		return exitFailure
	}
	parsed, err := parseCommand(arguments)
	if err != nil {
		bootstrap.Error("operator command denied", "reason", "invalid_arguments")
		return exitUsage
	}
	settings, err := config.Load(getter)
	if err != nil {
		bootstrap.Error("operator command denied", "reason", "invalid_configuration")
		return exitFailure
	}
	logger, err := observability.New(standardError, settings.LogLevel, settings.LogFormat)
	if err != nil {
		bootstrap.Error("operator command denied", "reason", "logger_configuration")
		return exitFailure
	}
	if standardInput == nil || standardOutput == nil || newContext == nil || recover == nil || random == nil {
		logger.Error("operator command denied", "reason", "command_boundary")
		return exitFailure
	}
	ctx, stop := newContext()
	if ctx == nil || stop == nil {
		logger.Error("operator command denied", "reason", "command_boundary")
		return exitFailure
	}
	defer stop()
	if err := ctx.Err(); err != nil {
		logger.Error("operator command failed", "operation", commandRecoverAdministrator, "reason", "canceled")
		return exitFailure
	}

	password, err := readPassword(standardInput)
	if err != nil {
		logger.Error("operator command denied", "reason", "invalid_password_input")
		return exitUsage
	}
	defer clear(password)
	requestID, err := newRequestID(random)
	if err != nil {
		logger.Error("operator command failed", "operation", commandRecoverAdministrator, "reason", "request_identity")
		return exitFailure
	}
	request := recoveryRequest{
		dataDirectory: settings.DataDirectory,
		busyTimeout:   settings.DatabaseBusyTimeout,
		loginName:     parsed.loginName,
		displayName:   parsed.displayName,
		password:      password,
		requestID:     requestID,
	}
	if err := runCommand(ctx, settings, request, recover); err != nil {
		logger.Error("operator command failed", "operation", commandRecoverAdministrator, "reason", failureReason(err))
		return exitFailure
	}
	if _, err := io.WriteString(standardOutput, successOutput); err != nil {
		logger.Error("operator command failed", "operation", commandRecoverAdministrator, "reason", "output_unavailable")
		return exitFailure
	}
	logger.Info("operator command complete", "operation", commandRecoverAdministrator)
	return exitSuccess
}

func parseCommand(arguments []string) (command, error) {
	if len(arguments) != 5 || arguments[0] != commandRecoverAdministrator {
		return command{}, errInvalidArguments
	}
	var parsed command
	seenLogin := false
	seenDisplay := false
	for index := 1; index < len(arguments); index += 2 {
		switch arguments[index] {
		case "--login-name":
			if seenLogin {
				return command{}, errInvalidArguments
			}
			parsed.loginName = arguments[index+1]
			seenLogin = true
		case "--display-name":
			if seenDisplay {
				return command{}, errInvalidArguments
			}
			parsed.displayName = arguments[index+1]
			seenDisplay = true
		default:
			return command{}, errInvalidArguments
		}
	}
	if !seenLogin || !seenDisplay {
		return command{}, errInvalidArguments
	}
	if _, err := auth.CanonicalLoginName(parsed.loginName); err != nil || !validDisplayArgument(parsed.displayName) {
		return command{}, errInvalidArguments
	}
	return parsed, nil
}

func validDisplayArgument(value string) bool {
	if value == "" || len(value) > maximumDisplayArgumentBytes || !utf8.ValidString(value) ||
		value != strings.TrimSpace(value) || utf8.RuneCountInString(value) > auth.MaximumDisplayRunes {
		return false
	}
	hasVisibleBase := false
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Zl, unicode.Zp) ||
			unicode.In(character, unicode.Cf) && character != '\u200c' && character != '\u200d' {
			return false
		}
		if unicode.IsLetter(character) || unicode.IsNumber(character) || unicode.IsPunct(character) || unicode.IsSymbol(character) {
			hasVisibleBase = true
		}
	}
	return hasVisibleBase
}

func readPassword(reader io.Reader) (passwordInput, error) {
	if reader == nil {
		return nil, errInvalidPassword
	}
	maximumInputBytes := int64(auth.MaximumPasswordBytes + len("\r\n") + 1)
	contents, err := io.ReadAll(io.LimitReader(reader, maximumInputBytes))
	if err != nil {
		clear(contents)
		return nil, errInvalidPassword
	}
	password := contents
	switch {
	case bytes.HasSuffix(password, []byte("\r\n")):
		password = password[:len(password)-2]
	case bytes.HasSuffix(password, []byte("\n")):
		password = password[:len(password)-1]
	}
	if len(contents) >= int(maximumInputBytes) || len(password) < auth.MinimumPasswordBytes ||
		len(password) > auth.MaximumPasswordBytes || bytes.ContainsAny(password, "\r\n") {
		clear(contents)
		return nil, errInvalidPassword
	}
	return passwordInput(password), nil
}

func newRequestID(random io.Reader) (domain.RequestID, error) {
	if random == nil {
		return "", errRequestIdentity
	}
	var entropy [requestRandomBytes]byte
	if _, err := io.ReadFull(random, entropy[:]); err != nil {
		clear(entropy[:])
		return "", errRequestIdentity
	}
	requestID := domain.RequestID("operator_" + base64.RawURLEncoding.EncodeToString(entropy[:]))
	clear(entropy[:])
	if requestID.Validate() != nil {
		return "", errRequestIdentity
	}
	return requestID, nil
}

func runCommand(ctx context.Context, settings config.Config, request recoveryRequest, recover recoveryOperation) error {
	if ctx == nil || recover == nil || config.Validate(settings) != nil ||
		request.dataDirectory != settings.DataDirectory || request.busyTimeout != settings.DatabaseBusyTimeout ||
		request.requestID.Validate() != nil || len(request.password) < auth.MinimumPasswordBytes ||
		len(request.password) > auth.MaximumPasswordBytes || bytes.ContainsAny(request.password, "\r\n") {
		return errCommandBoundary
	}
	if _, err := auth.CanonicalLoginName(request.loginName); err != nil || !validDisplayArgument(request.displayName) {
		return errCommandBoundary
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return recover(ctx, request)
}

func recoverAdministrator(ctx context.Context, request recoveryRequest) (result error) {
	store, err := storage.Open(ctx, storage.Options{
		DataDirectory: request.dataDirectory,
		BusyTimeout:   request.busyTimeout,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := store.Close(); err != nil {
			result = errors.Join(result, err)
		}
	}()

	digestKeyID, digestKey, err := store.AuthDigestKey()
	if err != nil {
		return err
	}
	defer clear(digestKey)
	hasher, err := auth.NewPasswordHasher(auth.DefaultPasswordPolicy())
	if err != nil {
		return err
	}
	service, err := auth.NewService(auth.ServiceOptions{
		Repository: store, PasswordHasher: hasher,
		DigestKey: digestKey, DigestKeyID: digestKeyID,
	})
	if err != nil {
		return err
	}
	_, err = service.RecoverAdministrator(ctx, auth.OperatorRecoveryInput{
		OperatorID: operatorActorID, LoginName: request.loginName, DisplayName: request.displayName,
		Password: []byte(request.password), RequestID: request.requestID,
	})
	return err
}

func failureReason(err error) string {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	case errors.Is(err, errCommandBoundary):
		return "command_boundary"
	case errors.Is(err, storage.ErrInvalidOptions):
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
	case errors.Is(err, storage.ErrAuthKeyUnavailable):
		return "authentication_key"
	case errors.Is(err, storage.ErrStoreClosed), errors.Is(err, storage.ErrCloseFailed):
		return "storage_close"
	case errors.Is(err, auth.ErrInvalidServiceInput):
		return "invalid_account_input"
	case errors.Is(err, auth.ErrAuthConflict):
		return "account_conflict"
	case errors.Is(err, auth.ErrAuthUnavailable):
		return "authentication_unavailable"
	case errors.Is(err, errRequestIdentity):
		return "request_identity"
	default:
		return "operation_failed"
	}
}
