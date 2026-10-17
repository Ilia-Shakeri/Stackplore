package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/alerting"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/database"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/observability"
)

const (
	databaseHealthInterval = 5 * time.Second
	alertPollInterval      = 5 * time.Second
	alertEvaluationBatch   = 32
	alertReadTimeoutLimit  = 30 * time.Second
	deliveryPollInterval   = 5 * time.Second
	deliveryLeaseDuration  = 30 * time.Second
	deliveryAttemptTimeout = 10 * time.Second
	deliveryBatchSize      = 16
)

var (
	errHTTPBoundary     = errors.New("HTTP boundary could not start")
	errListen           = errors.New("HTTP listener could not start")
	errServe            = errors.New("HTTP service stopped unexpectedly")
	errShutdown         = errors.New("HTTP service shutdown timed out")
	errAuthStart        = errors.New("authentication could not start")
	errStorageStart     = errors.New("storage could not start")
	errStorageStop      = errors.New("storage could not stop cleanly")
	errStorageOwnership = errors.New("storage ownership lost")
	errProductStart     = errors.New("infrastructure source could not start")
	errAlertStart       = errors.New("alert worker could not start")
	errDeliveryStart    = errors.New("notification delivery could not start")
	errObservationStart = errors.New("observation runtime could not start")
)

type listenerFactory func(network string, address string) (net.Listener, error)

type databaseRuntime interface {
	auth.Repository
	alerting.TaskStore
	alerting.ManagementRepository
	AuthDigestKey() (string, []byte, error)
	Health(context.Context) error
	Close() error
}

type databaseFactory func(context.Context, config.Config) (databaseRuntime, error)

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

func openDatabase(ctx context.Context, settings config.Config) (databaseRuntime, error) {
	return database.Open(ctx, settings)
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

	database, err := open(ctx, settings)
	if err != nil || database == nil {
		return errStorageStart
	}
	serviceContext, stopService := context.WithCancel(ctx)
	defer stopService()
	if ownership, ok := database.(interface{ OwnershipLost() <-chan struct{} }); ok {
		go func() {
			select {
			case <-ownership.OwnershipLost():
				stopService()
			case <-serviceContext.Done():
			}
		}()
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
	providers, err := buildProductProviders(settings)
	if err != nil {
		readiness.SetDatabaseReady(false)
		return closeDatabase(errors.Join(errHTTPBoundary, errProductStart))
	}
	defer closeProductProviders(providers)
	operations, err := buildOperationRuntime(serviceContext, settings, database, providers)
	if err != nil {
		readiness.SetDatabaseReady(false)
		return closeDatabase(errHTTPBoundary)
	}
	observations, err := buildObservationRuntime(serviceContext, settings, database, providers, logger)
	if err != nil {
		readiness.SetDatabaseReady(false)
		return closeDatabase(errObservationStart)
	}
	providers = observations.providers
	applicationSettings, err := buildApplicationSettingsRuntime(settings, database)
	if err != nil {
		readiness.SetDatabaseReady(false)
		return closeDatabase(errHTTPBoundary)
	}
	alertManager, err := alerting.NewManagementService(database, alerting.SecureIdentifierFactory{}, time.Now)
	if err != nil {
		readiness.SetDatabaseReady(false)
		return closeDatabase(errAlertStart)
	}
	historyReader, _ := database.(productHistoryReader)
	server, err := newHTTPServerWithServices(settings, logger, readiness, authentication, providers, alertManager, observations.settings, operations, applicationSettings,
		httpapi.ApplicationMutationMode{Enabled: settings.MutationsEnabled && !settings.DemoDeployment, DockerWriteEnabled: settings.DockerWriteEnabled, KubernetesWriteEnabled: settings.KubernetesWriteEnabled}, historyReader)
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
	alertWorker, err := buildAlertWorker(database, providers, logger, settings.RequestTimeout)
	if err != nil {
		_ = listener.Close()
		readiness.SetDatabaseReady(false)
		return closeDatabase(errAlertStart)
	}
	deliveryWorker, deliveryTargets, err := buildDeliveryWorker(settings, database, logger)
	if err != nil {
		_ = listener.Close()
		readiness.SetDatabaseReady(false)
		return closeDatabase(errDeliveryStart)
	}
	if deliveryTargets != nil {
		defer deliveryTargets.Close()
	}
	matrixWorker, matrixTargets, err := buildMatrixDeliveryWorker(settings, database, logger)
	if err != nil {
		_ = listener.Close()
		readiness.SetDatabaseReady(false)
		return closeDatabase(errDeliveryStart)
	}
	if matrixTargets != nil {
		defer matrixTargets.Close()
	}

	monitorContext, stopMonitor := context.WithCancel(serviceContext)
	monitorDone := make(chan struct{})
	alertDone := make(chan struct{})
	deliveryDone := make(chan struct{})
	matrixDone := make(chan struct{})
	observationDone := make(chan struct{})
	retentionDone := make(chan struct{})
	operationDone := make(chan struct{})
	auditRetentionDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		monitorDatabase(monitorContext, database, readiness, databaseHealthInterval, settings.DatabaseHealthTimeout)
	}()
	go func() {
		defer close(alertDone)
		alertWorker.Run(monitorContext)
	}()
	go func() {
		defer close(observationDone)
		observations.collection.Run(monitorContext)
	}()
	go func() {
		defer close(retentionDone)
		observations.runRetention(monitorContext, logger)
	}()
	go func() {
		defer close(auditRetentionDone)
		runAuditRetention(monitorContext, settings, applicationSettings, logger)
	}()
	if operations == nil {
		close(operationDone)
	} else {
		go func() {
			defer close(operationDone)
			operations.RunRecovery(monitorContext)
		}()
	}
	if deliveryWorker == nil {
		close(deliveryDone)
	} else {
		go func() {
			defer close(deliveryDone)
			deliveryWorker.Run(monitorContext)
		}()
	}
	if matrixWorker == nil {
		close(matrixDone)
	} else {
		go func() { defer close(matrixDone); matrixWorker.Run(monitorContext) }()
	}

	logger.Info("service listener ready", "environment", string(settings.Environment))
	err = serveServer(serviceContext, server, listener, readiness, settings.ShutdownTimeout)
	stopMonitor()
	<-monitorDone
	<-alertDone
	<-deliveryDone
	<-matrixDone
	<-observationDone
	<-retentionDone
	<-operationDone
	<-auditRetentionDone
	readiness.SetDatabaseReady(false)
	if ownership, ok := database.(interface{ OwnershipLost() <-chan struct{} }); ok {
		select {
		case <-ownership.OwnershipLost():
			err = errors.Join(err, errStorageOwnership)
		default:
		}
	}
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
	historyReaders ...productHistoryReader,
) (*http.Server, error) {
	if config.Validate(settings) != nil || logger == nil || readiness == nil || authentication == nil || len(historyReaders) > 1 {
		return nil, errHTTPBoundary
	}
	providers, err := buildProductProviders(settings)
	if err != nil {
		return nil, errors.Join(errHTTPBoundary, errProductStart)
	}
	return newHTTPServerWithProviders(settings, logger, readiness, authentication, providers, nil, nil, historyReaders...)
}

func newHTTPServerWithProviders(
	settings config.Config,
	logger *slog.Logger,
	readiness *httpapi.Readiness,
	authentication *auth.Service,
	providers []httpapi.ProductProvider,
	alertManager httpapi.AlertManager,
	sourceSettings httpapi.SourceSettings,
	historyReaders ...productHistoryReader,
) (*http.Server, error) {
	return newHTTPServerWithServices(settings, logger, readiness, authentication, providers, alertManager, sourceSettings, nil, nil, httpapi.ApplicationMutationMode{}, historyReaders...)
}

func newHTTPServerWithServices(
	settings config.Config,
	logger *slog.Logger,
	readiness *httpapi.Readiness,
	authentication *auth.Service,
	providers []httpapi.ProductProvider,
	alertManager httpapi.AlertManager,
	sourceSettings httpapi.SourceSettings,
	operations httpapi.OperationManager,
	applicationSettings httpapi.ApplicationSettings,
	applicationMode httpapi.ApplicationMutationMode,
	historyReaders ...productHistoryReader,
) (*http.Server, error) {
	if config.Validate(settings) != nil || logger == nil || readiness == nil || authentication == nil ||
		len(providers) == 0 || len(historyReaders) > 1 {
		return nil, errHTTPBoundary
	}
	var historyReader productHistoryReader
	if len(historyReaders) == 1 {
		historyReader = historyReaders[0]
	}
	notificationSettings, err := notification.NewCombinedRuntimeSettings(
		settings.WebhookEnabled, settings.WebhookDestinationReferenceID, settings.MatrixEnabled, settings.MatrixDestinationReferenceID,
	)
	if err != nil {
		return nil, errHTTPBoundary
	}
	notificationTester, err := buildNotificationTesters(settings, historyReader)
	if err != nil {
		return nil, errHTTPBoundary
	}
	handler, err := httpapi.NewHandler(httpapi.Options{
		Logger:                  logger,
		Readiness:               readiness,
		Authentication:          authentication,
		PublicOrigin:            httpapi.Origin{Scheme: settings.PublicOrigin.Scheme, Authority: settings.PublicOrigin.Host},
		TrustedProxyPrefixes:    settings.TrustedProxyPrefixes,
		MetricsAllowedPrefixes:  settings.MetricsAllowedPrefixes,
		RequestTimeout:          settings.RequestTimeout,
		MaximumRequestBody:      settings.MaxRequestBodyBytes,
		MaximumResponseBody:     settings.MaximumResponseBytes,
		MaximumInFlight:         settings.MaxInFlight,
		ProductProviders:        providers,
		AuditReader:             historyReader,
		AlertReader:             historyReader,
		NotificationReader:      historyReader,
		NotificationSettings:    notificationSettings,
		NotificationTester:      notificationTester,
		AlertManager:            alertManager,
		SourceSettings:          sourceSettings,
		Operations:              operations,
		ApplicationSettings:     applicationSettings,
		ApplicationMutationMode: applicationMode,
		GuestDemo:               guestHTTPOptions(settings),
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

func buildAlertWorker(
	store alerting.TaskStore,
	providers []httpapi.ProductProvider,
	logger *slog.Logger,
	readTimeout time.Duration,
) (*alerting.Worker, error) {
	if store == nil || len(providers) == 0 || logger == nil {
		return nil, errAlertStart
	}
	if readTimeout > alertReadTimeoutLimit {
		readTimeout = alertReadTimeoutLimit
	}
	evidenceProviders := make([]alerting.EvidenceProvider, 0, len(providers))
	for _, provider := range providers {
		evidenceProviders = append(evidenceProviders, provider)
	}
	collections, ok := store.(domain.SourceCollectionReader)
	if !ok {
		return nil, errAlertStart
	}
	evidence, err := alerting.NewProviderEvidenceWithOptions(alerting.ProviderEvidenceOptions{
		Providers: evidenceProviders, Collections: collections, MaximumCollectionGap: 90 * time.Second,
	})
	if err != nil {
		return nil, errAlertStart
	}
	worker, err := alerting.NewWorker(alerting.WorkerOptions{
		Store: store, Evidence: evidence, Identifiers: alerting.SecureIdentifierFactory{},
		Now: time.Now, PollInterval: alertPollInterval, ReadTimeout: readTimeout,
		BatchSize: alertEvaluationBatch,
		OnError: func(error) {
			logger.Error("alert evaluation failed", "reason", "evaluation_unavailable")
		},
	})
	if err != nil {
		return nil, errAlertStart
	}
	return worker, nil
}

func buildDeliveryWorker(
	settings config.Config,
	database databaseRuntime,
	logger *slog.Logger,
) (*notification.DeliveryWorker, *notification.StaticWebhookTargetResolver, error) {
	if !settings.WebhookEnabled {
		return nil, nil, nil
	}
	repository, ok := database.(notification.AdapterDeliveryRepository)
	if !ok || logger == nil {
		return nil, nil, errDeliveryStart
	}
	signingKey := settings.WebhookSigningKey.Bytes()
	defer clear(signingKey)
	targets, err := notification.NewStaticWebhookTargetResolver(
		settings.WebhookDestinationReferenceID, settings.WebhookURL.Reveal(), signingKey,
	)
	if err != nil {
		return nil, nil, errDeliveryStart
	}
	randomBytes := make([]byte, 18)
	if _, err := rand.Read(randomBytes); err != nil {
		targets.Close()
		return nil, nil, errDeliveryStart
	}
	workerID, err := notification.NewDeliveryWorkerID(randomBytes)
	clear(randomBytes)
	if err != nil {
		targets.Close()
		return nil, nil, errDeliveryStart
	}
	sender, err := notification.NewWebhookSender(notification.WebhookSenderOptions{
		Resolver: net.DefaultResolver,
		Dialer:   &net.Dialer{Timeout: deliveryAttemptTimeout, KeepAlive: 30 * time.Second},
		Timeout:  deliveryAttemptTimeout, MaximumResponseBytes: 64 * 1024,
	})
	if err != nil {
		targets.Close()
		return nil, nil, errDeliveryStart
	}
	worker, err := notification.NewDeliveryWorker(notification.DeliveryWorkerOptions{
		AdapterRepository: repository, Adapter: notification.DeliveryAdapterWebhook, Targets: targets, Sender: sender,
		DestinationReferenceID: settings.WebhookDestinationReferenceID, LeaseOwner: workerID,
		Now: time.Now, PollInterval: deliveryPollInterval, LeaseDuration: deliveryLeaseDuration,
		BatchSize: deliveryBatchSize, MaximumAttempts: 3,
		RetrySchedule: []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute},
		OnError: func(error) {
			logger.Error("notification delivery failed", "reason", "delivery_unavailable")
		},
	})
	if err != nil {
		targets.Close()
		return nil, nil, errDeliveryStart
	}
	return worker, targets, nil
}

type productHistoryReader interface {
	httpapi.AuditReader
	httpapi.AlertReader
	httpapi.NotificationReader
}

func buildProductProviders(settings config.Config) ([]httpapi.ProductProvider, error) {
	return configuredProductProviders(settings)
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	if limit < 1 {
		return nil, errProductStart
	}
	root, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return nil, errProductStart
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, errProductStart
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || !filepath.IsLocal(relative) {
		return nil, errProductStart
	}
	initial, err := os.Lstat(resolved)
	if err != nil || !initial.Mode().IsRegular() || initial.Size() > limit {
		return nil, errProductStart
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, errProductStart
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(initial, opened) || opened.Size() > limit {
		return nil, errProductStart
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		clear(data)
		return nil, errProductStart
	}
	return data, nil
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
	streamContext, cancelStreams := context.WithCancel(context.Background())
	defer cancelStreams()
	server.Handler = withStreamShutdown(server.Handler, streamContext)

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
		cancelStreams()

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
	case errors.Is(err, errStorageOwnership):
		return "storage_ownership_lost"
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
	case errors.Is(err, errProductStart):
		return "product_start"
	case errors.Is(err, errAlertStart):
		return "alert_start"
	case errors.Is(err, errDeliveryStart):
		return "notification_delivery_start"
	case errors.Is(err, errObservationStart):
		return "observation_start"
	default:
		return "unexpected_stop"
	}
}
