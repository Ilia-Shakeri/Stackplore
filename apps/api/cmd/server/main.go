package main

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/observability"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/storage"
)

const databaseHealthInterval = 5 * time.Second

var (
	errHTTPBoundary = errors.New("HTTP boundary could not start")
	errListen       = errors.New("HTTP listener could not start")
	errServe        = errors.New("HTTP service stopped unexpectedly")
	errShutdown     = errors.New("HTTP service shutdown timed out")
	errAuthStart    = errors.New("authentication could not start")
	errStorageStart = errors.New("storage could not start")
	errStorageStop  = errors.New("storage could not stop cleanly")
)

type listenerFactory func(network string, address string) (net.Listener, error)

type databaseRuntime interface {
	auth.Repository
	AuthDigestKey() (string, []byte, error)
	Health(context.Context) error
	Close() error
}

type databaseFactory func(context.Context, storage.Options) (databaseRuntime, error)

func main() {
	os.Exit(realMain())
}

func realMain() int {
	bootstrap, err := observability.New(os.Stderr, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		return 1
	}

	settings, err := config.Load(config.OSGetter{})
	if err != nil {
		logConfigurationFailure(bootstrap, err)
		return 1
	}
	logger, err := observability.New(os.Stderr, settings.LogLevel, settings.LogFormat)
	if err != nil {
		bootstrap.Error("service start denied", "reason", "logger_configuration")
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, settings, logger, net.Listen); err != nil {
		logger.Error("service stopped", "reason", lifecycleReason(err))
		return 1
	}
	return 0
}

func run(ctx context.Context, settings config.Config, logger *slog.Logger, listen listenerFactory) error {
	return runWithStorage(ctx, settings, logger, listen, openDatabase)
}

func openDatabase(ctx context.Context, options storage.Options) (databaseRuntime, error) {
	return storage.Open(ctx, options)
}

func runWithStorage(
	ctx context.Context,
	settings config.Config,
	logger *slog.Logger,
	listen listenerFactory,
	open databaseFactory,
) error {
	if ctx == nil || logger == nil || listen == nil || open == nil || config.Validate(settings) != nil {
		return errHTTPBoundary
	}
	if ctx.Err() != nil {
		return nil
	}

	database, err := open(ctx, storage.Options{
		DataDirectory: settings.DataDirectory,
		BusyTimeout:   settings.DatabaseBusyTimeout,
	})
	if err != nil || database == nil {
		return errStorageStart
	}
	closeDatabase := func(primary error) error {
		if err := database.Close(); err != nil {
			return errors.Join(primary, errStorageStop)
		}
		return primary
	}

	readiness := httpapi.NewReadiness()
	readiness.SetDatabaseReady(true)
	authentication, err := buildAuthentication(ctx, settings, database, time.Now)
	if err != nil {
		readiness.SetDatabaseReady(false)
		return closeDatabase(errAuthStart)
	}
	authenticationStatus, err := authentication.SetupStatus(ctx)
	if err != nil || authenticationStatus.Required && authenticationStatus.BootstrapProofRequired &&
		!settings.SetupBootstrapExpiresAt.After(time.Now().UTC()) {
		readiness.SetDatabaseReady(false)
		return closeDatabase(errAuthStart)
	}
	server, err := newHTTPServer(settings, logger, readiness, authentication)
	if err != nil {
		readiness.SetDatabaseReady(false)
		return closeDatabase(err)
	}
	listener, err := listen("tcp", settings.HTTPAddress)
	if err != nil {
		readiness.SetDatabaseReady(false)
		return closeDatabase(errListen)
	}
	listener, err = newLimitedListener(listener, settings.MaxConnections)
	if err != nil {
		readiness.SetDatabaseReady(false)
		return closeDatabase(errHTTPBoundary)
	}

	monitorContext, stopMonitor := context.WithCancel(context.Background())
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		monitorDatabase(monitorContext, database, readiness, databaseHealthInterval, settings.DatabaseBusyTimeout)
	}()

	logger.Info("service listener ready", "environment", string(settings.Environment))
	err = serveServer(ctx, server, listener, readiness, settings.ShutdownTimeout)
	stopMonitor()
	<-monitorDone
	readiness.SetDatabaseReady(false)
	err = closeDatabase(err)
	if err == nil {
		logger.Info("service stopped", "reason", "graceful_shutdown")
	}
	return err
}

func buildAuthentication(
	ctx context.Context,
	settings config.Config,
	database databaseRuntime,
	now func() time.Time,
) (*auth.Service, error) {
	if ctx == nil || ctx.Err() != nil || database == nil || now == nil {
		return nil, errAuthStart
	}
	setupRequired, err := database.SetupRequired(ctx)
	if err != nil {
		return nil, errAuthStart
	}
	bootstrapGate, bootstrapRequired, err := buildBootstrapGate(settings, setupRequired, now())
	if err != nil {
		return nil, errAuthStart
	}

	keyID, digestKey, err := database.AuthDigestKey()
	if err != nil {
		clear(digestKey)
		return nil, errAuthStart
	}
	defer clear(digestKey)
	hasher, err := auth.NewPasswordHasher(auth.DefaultPasswordPolicy())
	if err != nil {
		return nil, errAuthStart
	}
	service, err := auth.NewService(auth.ServiceOptions{
		Repository:        database,
		PasswordHasher:    hasher,
		DigestKey:         digestKey,
		DigestKeyID:       keyID,
		BootstrapRequired: bootstrapRequired,
		BootstrapGate:     bootstrapGate,
		Now:               now,
	})
	if err != nil {
		return nil, errAuthStart
	}
	return service, nil
}

func buildBootstrapGate(
	settings config.Config,
	setupRequired bool,
	now time.Time,
) (*auth.BootstrapGate, bool, error) {
	bootstrapRequired := config.SetupBootstrapRequired(settings)
	if !setupRequired || !bootstrapRequired {
		return nil, bootstrapRequired, nil
	}
	proof := settings.SetupBootstrapProof.Bytes()
	defer clear(proof)
	if len(proof) != auth.DigestKeyBytes || now.IsZero() || !settings.SetupBootstrapExpiresAt.After(now.UTC()) {
		return nil, bootstrapRequired, errAuthStart
	}
	encoded := make([]byte, base64.RawURLEncoding.EncodedLen(len(proof)))
	base64.RawURLEncoding.Encode(encoded, proof)
	defer clear(encoded)
	gate, err := auth.NewBootstrapGate(string(encoded), settings.SetupBootstrapExpiresAt)
	if err != nil {
		return nil, bootstrapRequired, errAuthStart
	}
	return gate, bootstrapRequired, nil
}

func monitorDatabase(
	ctx context.Context,
	database databaseRuntime,
	readiness *httpapi.Readiness,
	interval time.Duration,
	checkTimeout time.Duration,
) {
	if ctx == nil || database == nil || readiness == nil || interval <= 0 || checkTimeout <= 0 {
		if readiness != nil {
			readiness.SetDatabaseReady(false)
		}
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkContext, cancel := context.WithTimeout(ctx, checkTimeout)
			err := database.Health(checkContext)
			cancel()
			readiness.SetDatabaseReady(err == nil)
		}
	}
}

func newHTTPServer(
	settings config.Config,
	logger *slog.Logger,
	readiness *httpapi.Readiness,
	authentication *auth.Service,
) (*http.Server, error) {
	if config.Validate(settings) != nil || logger == nil || readiness == nil || authentication == nil {
		return nil, errHTTPBoundary
	}
	handler, err := httpapi.NewHandler(httpapi.Options{
		Logger:                 logger,
		Readiness:              readiness,
		Authentication:         authentication,
		PublicOrigin:           httpapi.Origin{Scheme: settings.PublicOrigin.Scheme, Authority: settings.PublicOrigin.Host},
		TrustedProxyPrefixes:   settings.TrustedProxyPrefixes,
		MetricsAllowedPrefixes: settings.MetricsAllowedPrefixes,
		RequestTimeout:         settings.RequestTimeout,
		MaximumRequestBody:     settings.MaxRequestBodyBytes,
		MaximumResponseBody:    settings.MaximumResponseBytes,
		MaximumInFlight:        settings.MaxInFlight,
		RateLimit: httpapi.RateLimitConfig{
			RequestsPerSecond: settings.RatePerSecond,
			Burst:             settings.RateBurst,
			MaximumClients:    settings.RateMaxClients,
		},
	})
	if err != nil {
		return nil, errHTTPBoundary
	}

	return &http.Server{
		Addr:              settings.HTTPAddress,
		Handler:           handler,
		ErrorLog:          log.New(io.Discard, "", 0),
		ReadHeaderTimeout: settings.ReadHeaderTimeout,
		ReadTimeout:       settings.ReadTimeout,
		WriteTimeout:      settings.WriteTimeout,
		IdleTimeout:       settings.IdleTimeout,
		MaxHeaderBytes:    settings.MaxHeaderBytes,
	}, nil
}

func serveServer(
	ctx context.Context,
	server *http.Server,
	listener net.Listener,
	readiness *httpapi.Readiness,
	shutdownTimeout time.Duration,
) error {
	if ctx == nil || server == nil || listener == nil || readiness == nil || shutdownTimeout <= 0 {
		if listener != nil {
			_ = listener.Close()
		}
		return errHTTPBoundary
	}
	if ctx.Err() != nil {
		_ = listener.Close()
		return nil
	}

	baseContext, cancelBase := context.WithCancel(context.Background())
	defer cancelBase()
	server.BaseContext = func(net.Listener) context.Context { return baseContext }

	serveDone := make(chan error, 1)
	readiness.SetServiceReady(true)
	go func() {
		serveDone <- server.Serve(listener)
	}()

	select {
	case err := <-serveDone:
		readiness.SetServiceReady(false)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errServe
	case <-ctx.Done():
		readiness.SetServiceReady(false)

		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
		shutdownErr := server.Shutdown(shutdownContext)
		cancelShutdown()
		cancelBase()
		if shutdownErr != nil {
			_ = server.Close()
		}
		serveErr := <-serveDone
		if shutdownErr != nil {
			return errShutdown
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return errServe
		}
		return nil
	}
}

func logConfigurationFailure(logger *slog.Logger, err error) {
	var fieldError *config.FieldError
	if errors.As(err, &fieldError) {
		logger.Error("service start denied",
			"reason", "invalid_configuration",
			"field", fieldError.Field,
			"rule", fieldError.Reason,
		)
		return
	}
	logger.Error("service start denied", "reason", "invalid_configuration")
}

func lifecycleReason(err error) string {
	switch {
	case errors.Is(err, errHTTPBoundary):
		return "http_boundary"
	case errors.Is(err, errListen):
		return "listener"
	case errors.Is(err, errShutdown):
		return "shutdown_timeout"
	case errors.Is(err, errAuthStart):
		return "authentication_start"
	case errors.Is(err, errStorageStart):
		return "storage_start"
	case errors.Is(err, errStorageStop):
		return "storage_stop"
	default:
		return "unexpected_stop"
	}
}
