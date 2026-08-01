package main

import (
	"context"
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

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/observability"
)

var (
	errHTTPBoundary = errors.New("HTTP boundary could not start")
	errListen       = errors.New("HTTP listener could not start")
	errServe        = errors.New("HTTP service stopped unexpectedly")
	errShutdown     = errors.New("HTTP service shutdown timed out")
)

type listenerFactory func(network string, address string) (net.Listener, error)

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
	if ctx == nil || logger == nil || listen == nil {
		return errHTTPBoundary
	}

	readiness := httpapi.NewReadiness()
	server, err := newHTTPServer(settings, logger, readiness)
	if err != nil {
		return err
	}
	listener, err := listen("tcp", settings.HTTPAddress)
	if err != nil {
		return errListen
	}
	listener, err = newLimitedListener(listener, settings.MaxConnections)
	if err != nil {
		return errHTTPBoundary
	}

	logger.Info("service listener ready", "environment", string(settings.Environment))
	err = serveServer(ctx, server, listener, readiness, settings.ShutdownTimeout)
	if err == nil {
		logger.Info("service stopped", "reason", "graceful_shutdown")
	}
	return err
}

func newHTTPServer(settings config.Config, logger *slog.Logger, readiness *httpapi.Readiness) (*http.Server, error) {
	if config.Validate(settings) != nil || logger == nil || readiness == nil {
		return nil, errHTTPBoundary
	}
	handler, err := httpapi.NewHandler(httpapi.Options{
		Logger:                 logger,
		Readiness:              readiness,
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
	readiness.SetReady(true)
	go func() {
		serveDone <- server.Serve(listener)
	}()

	select {
	case err := <-serveDone:
		readiness.SetReady(false)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errServe
	case <-ctx.Done():
		readiness.SetReady(false)

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
	default:
		return "unexpected_stop"
	}
}
