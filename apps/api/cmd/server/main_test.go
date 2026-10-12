package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/alerting"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/observability"
)

type mapGetter map[string]string

func (values mapGetter) LookupEnv(key string) (string, bool) {
	value, ok := values[key]
	return value, ok
}

func (values mapGetter) Environ() []string {
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

func defaultSettings(t *testing.T) config.Config {
	t.Helper()
	settings, err := config.Load(mapGetter{})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	settings.DataDirectory = t.TempDir()
	return settings
}

func discardLogger(t *testing.T) *slog.Logger {
	t.Helper()
	logger, err := observability.New(io.Discard, slog.LevelDebug, config.LogFormatJSON)
	if err != nil {
		t.Fatalf("observability.New() error = %v", err)
	}
	return logger
}

func TestHTTPServerUsesValidatedBounds(t *testing.T) {
	t.Parallel()
	settings := defaultSettings(t)
	server, err := newHTTPServer(settings, discardLogger(t), httpapi.NewReadiness(), testAuthentication(t, settings))
	if err != nil {
		t.Fatalf("newHTTPServer() error = %v", err)
	}

	if server.Addr != settings.HTTPAddress ||
		server.ReadHeaderTimeout != settings.ReadHeaderTimeout ||
		server.ReadTimeout != settings.ReadTimeout ||
		server.WriteTimeout != settings.WriteTimeout ||
		server.IdleTimeout != settings.IdleTimeout ||
		server.MaxHeaderBytes != settings.MaxHeaderBytes {
		t.Errorf("server bounds do not match validated settings: %#v", server)
	}
	if server.Handler == nil || server.ErrorLog == nil {
		t.Error("server dependencies are missing")
	}
}

func TestServeServerReadinessAndGracefulStop(t *testing.T) {
	t.Parallel()
	settings := defaultSettings(t)
	readiness := httpapi.NewReadiness()
	readiness.SetDatabaseReady(true)
	server, err := newHTTPServer(settings, discardLogger(t), readiness, testAuthentication(t, settings))
	if err != nil {
		t.Fatalf("newHTTPServer() error = %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serveServer(ctx, server, listener, readiness, time.Second)
	}()

	waitFor(t, readiness.IsReady)
	client := &http.Client{
		Transport: &http.Transport{Proxy: nil},
		Timeout:   time.Second,
	}
	defer client.CloseIdleConnections()
	response, err := client.Get("http://" + listener.Addr().String() + "/readyz")
	if err != nil {
		cancel()
		t.Fatalf("GET /readyz error = %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("GET /readyz status = %d", response.StatusCode)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serveServer() error = %v", err)
	}
	if readiness.IsReady() {
		t.Error("readiness stayed true after stop")
	}
}

func TestServeServerSkipsCanceledStart(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	readiness := httpapi.NewReadiness()
	server := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}
	if err := serveServer(ctx, server, listener, readiness, time.Second); err != nil {
		t.Fatalf("serveServer() error = %v", err)
	}
	if readiness.IsReady() {
		t.Error("readiness became true for canceled start")
	}
}

func TestServeServerDrainsActiveRequestBeforeCancel(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	canceled := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
			close(canceled)
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})}
	readiness := httpapi.NewReadiness()
	readiness.SetDatabaseReady(true)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serveServer(ctx, server, listener, readiness, time.Second)
	}()
	waitFor(t, readiness.IsReady)

	status := make(chan int, 1)
	go func() {
		client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: time.Second}
		response, requestErr := client.Get("http://" + listener.Addr().String() + "/drain")
		if requestErr != nil {
			status <- 0
			return
		}
		_ = response.Body.Close()
		status <- response.StatusCode
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		cancel()
		t.Fatal("request did not start")
	}

	cancel()
	waitFor(t, func() bool { return !readiness.IsReady() })
	close(release)
	if got := <-status; got != http.StatusNoContent {
		t.Errorf("drained request status = %d; want %d", got, http.StatusNoContent)
	}
	if err := <-done; err != nil {
		t.Fatalf("serveServer() error = %v", err)
	}
	select {
	case <-canceled:
		t.Error("active request context was canceled before drain")
	default:
	}
}

func TestRunFailsClosedBeforeListening(t *testing.T) {
	t.Parallel()
	settings := defaultSettings(t)
	logger := discardLogger(t)
	wantListenError := errors.New("listen failed")
	err := runWithStorage(
		context.Background(),
		settings,
		logger,
		func(string, string) (net.Listener, error) { return nil, wantListenError },
		func(context.Context, config.Config) (databaseRuntime, error) { return &fakeDatabase{}, nil },
	)
	if !errors.Is(err, errListen) || errors.Is(err, wantListenError) {
		t.Errorf("run() error = %v; want sealed listener error", err)
	}

	settings.MaxInFlight = 0
	called := false
	err = run(context.Background(), settings, logger, func(string, string) (net.Listener, error) {
		called = true
		return nil, wantListenError
	})
	if !errors.Is(err, errHTTPBoundary) || called {
		t.Errorf("invalid-boundary run = (%v, called %t)", err, called)
	}
}

func TestRunSealsStorageStartFailureBeforeListening(t *testing.T) {
	t.Parallel()
	settings := defaultSettings(t)
	marker := errors.New("raw storage failure")
	listened := false
	err := runWithStorage(
		context.Background(),
		settings,
		discardLogger(t),
		func(string, string) (net.Listener, error) {
			listened = true
			return nil, errors.New("unexpected listen")
		},
		func(context.Context, config.Config) (databaseRuntime, error) {
			return nil, marker
		},
	)
	if !errors.Is(err, errStorageStart) || errors.Is(err, marker) || listened {
		t.Errorf("runWithStorage() = (%v, listened %t); want sealed storage failure", err, listened)
	}
}

func TestRunClosesStorageAfterListenerFailure(t *testing.T) {
	t.Parallel()
	settings := defaultSettings(t)
	database := &fakeDatabase{}
	err := runWithStorage(
		context.Background(),
		settings,
		discardLogger(t),
		func(string, string) (net.Listener, error) { return nil, errors.New("raw listen failure") },
		func(context.Context, config.Config) (databaseRuntime, error) { return database, nil },
	)
	if !errors.Is(err, errListen) || !database.closed.Load() {
		t.Errorf("runWithStorage() = %v, closed = %t; want listener error after storage close", err, database.closed.Load())
	}
}

func TestBuildDeliveryWorkerNeedsConfiguredRepositoryAndClearsTarget(t *testing.T) {
	t.Parallel()
	settings, err := config.Load(mapGetter{
		"STACKPLORE_WEBHOOK_ENABLED":                  "true",
		"STACKPLORE_WEBHOOK_DESTINATION_REFERENCE_ID": "webhook_primary",
		"STACKPLORE_WEBHOOK_URL":                      "https://hooks.example.test/path",
		"STACKPLORE_WEBHOOK_SIGNING_KEY":              "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if worker, targets, err := buildDeliveryWorker(settings, &fakeDatabase{}, discardLogger(t)); worker != nil || targets != nil || !errors.Is(err, errDeliveryStart) {
		t.Fatalf("missing repository build = (%v, %v, %v)", worker, targets, err)
	}
	database := &fakeDeliveryDatabase{fakeDatabase: &fakeDatabase{}}
	worker, targets, err := buildDeliveryWorker(settings, database, discardLogger(t))
	if err != nil || worker == nil || targets == nil {
		t.Fatalf("buildDeliveryWorker() = (%v, %v, %v)", worker, targets, err)
	}
	targets.Close()
	if _, err := targets.ResolveWebhookTarget(context.Background(), "webhook_primary"); err == nil {
		t.Fatal("closed runtime target retained secret")
	}
}

func TestOpenRemoteSetupFailsBeforeListenerWithoutLiveProof(t *testing.T) {
	t.Parallel()
	settings, err := config.Load(mapGetter{"STACKPLORE_HTTP_ADDRESS": "0.0.0.0:8080"})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	database := &fakeDatabase{setupRequired: true}
	listened := false
	err = runWithStorage(
		context.Background(),
		settings,
		discardLogger(t),
		func(string, string) (net.Listener, error) {
			listened = true
			return nil, errors.New("listener must stay closed")
		},
		func(context.Context, config.Config) (databaseRuntime, error) { return database, nil },
	)
	if !errors.Is(err, errAuthStart) || listened || !database.closed.Load() {
		t.Errorf("runWithStorage() = (%v, listened %t, closed %t); want sealed auth stop", err, listened, database.closed.Load())
	}
}

func TestClosedRemoteSetupStartsWithoutProof(t *testing.T) {
	t.Parallel()
	settings, err := config.Load(mapGetter{"STACKPLORE_HTTP_ADDRESS": "0.0.0.0:8080"})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	database := &fakeDatabase{setupRequired: false}
	err = runWithStorage(
		context.Background(),
		settings,
		discardLogger(t),
		func(string, string) (net.Listener, error) { return nil, errors.New("raw listener failure") },
		func(context.Context, config.Config) (databaseRuntime, error) { return database, nil },
	)
	if !errors.Is(err, errListen) || errors.Is(err, errAuthStart) || !database.closed.Load() {
		t.Errorf("runWithStorage() = (%v, closed %t); want listener stop after closed setup boot", err, database.closed.Load())
	}
}

func TestBootstrapGateNeedsLiveProofAndBurnsAfterSuccess(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	proof := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	settings, err := config.Load(mapGetter{
		"STACKPLORE_HTTP_ADDRESS":               "0.0.0.0:8080",
		"STACKPLORE_SETUP_BOOTSTRAP_PROOF":      proof,
		"STACKPLORE_SETUP_BOOTSTRAP_EXPIRES_AT": now.Add(30 * time.Minute).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	gate, required, err := buildBootstrapGate(settings, true, now)
	if err != nil || !required || gate == nil {
		t.Fatalf("buildBootstrapGate() = (%v, %t, %v)", gate, required, err)
	}
	uses := 0
	if err := gate.Use(context.Background(), proof, func(context.Context) error {
		uses++
		return nil
	}); err != nil {
		t.Fatalf("first gate use error = %v", err)
	}
	if err := gate.Use(context.Background(), proof, func(context.Context) error {
		uses++
		return nil
	}); !errors.Is(err, auth.ErrBootstrapDenied) {
		t.Fatalf("second gate use error = %v; want denial", err)
	}
	if uses != 1 {
		t.Errorf("gate callback uses = %d; want 1", uses)
	}

	expired, err := config.Load(mapGetter{
		"STACKPLORE_HTTP_ADDRESS":               "0.0.0.0:8080",
		"STACKPLORE_SETUP_BOOTSTRAP_PROOF":      proof,
		"STACKPLORE_SETUP_BOOTSTRAP_EXPIRES_AT": now.Add(-time.Minute).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("config.Load(expired) error = %v", err)
	}
	if gate, required, err := buildBootstrapGate(expired, true, now); gate != nil || !required || !errors.Is(err, errAuthStart) {
		t.Errorf("expired buildBootstrapGate() = (%v, %t, %v); want sealed stop", gate, required, err)
	}
	if gate, required, err := buildBootstrapGate(expired, false, now); gate != nil || !required || err != nil {
		t.Errorf("closed expired buildBootstrapGate() = (%v, %t, %v); want no gate", gate, required, err)
	}
	if gate, required, err := buildBootstrapGate(config.Config{HTTPAddress: "0.0.0.0:8080"}, false, now); gate != nil || !required || err != nil {
		t.Errorf("closed buildBootstrapGate() = (%v, %t, %v); want no gate", gate, required, err)
	}
}

func TestAuthenticationBuildClearsCopiedDigestKey(t *testing.T) {
	t.Parallel()
	settings := defaultSettings(t)
	database := &fakeDatabase{}
	if _, err := buildAuthentication(context.Background(), settings, database, time.Now); err != nil {
		t.Fatalf("buildAuthentication() error = %v", err)
	}
	for index, value := range database.authKey {
		if value != 0 {
			t.Fatalf("digest key byte %d stayed live", index)
		}
	}
}

func TestDatabaseMonitorTracksLossAndRecovery(t *testing.T) {
	t.Parallel()
	readiness := httpapi.NewReadiness()
	readiness.SetDatabaseReady(true)
	readiness.SetServiceReady(true)
	database := &fakeDatabase{}
	database.available.Store(false)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		monitorDatabase(ctx, database, readiness, time.Millisecond, 10*time.Millisecond)
		close(done)
	}()

	waitFor(t, func() bool { return !readiness.IsReady() })
	database.available.Store(true)
	waitFor(t, readiness.IsReady)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("database monitor did not stop")
	}
}

func TestServeServerSealsAcceptFailure(t *testing.T) {
	t.Parallel()
	readiness := httpapi.NewReadiness()
	readiness.SetDatabaseReady(true)
	server := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}
	err := serveServer(context.Background(), server, failingListener{}, readiness, time.Second)
	if !errors.Is(err, errServe) {
		t.Fatalf("serveServer() error = %v; want %v", err, errServe)
	}
	if readiness.IsReady() {
		t.Error("readiness stayed true after accept failure")
	}
}

func TestServeServerForcesBoundedStop(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		<-release
	})}
	readiness := httpapi.NewReadiness()
	readiness.SetDatabaseReady(true)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serveServer(ctx, server, listener, readiness, 20*time.Millisecond)
	}()
	waitFor(t, readiness.IsReady)

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: time.Second}
		response, requestErr := client.Get("http://" + listener.Addr().String() + "/hold")
		if requestErr == nil {
			_ = response.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		cancel()
		t.Fatal("request did not start")
	}

	cancel()
	if err := <-done; !errors.Is(err, errShutdown) {
		close(release)
		t.Fatalf("serveServer() error = %v; want %v", err, errShutdown)
	}
	close(release)
	select {
	case <-clientDone:
	case <-time.After(time.Second):
		t.Fatal("client did not stop")
	}
	if readiness.IsReady() {
		t.Error("readiness stayed true during forced stop")
	}
}

func TestConfigurationFailureDoesNotLogValue(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger, err := observability.New(&output, slog.LevelInfo, config.LogFormatJSON)
	if err != nil {
		t.Fatalf("observability.New() error = %v", err)
	}
	_, configErr := config.Load(mapGetter{"STACKPLORE_HTTP_ADDRESS": "secret-canary"})
	if configErr == nil {
		t.Fatal("config.Load() error = nil")
	}
	logConfigurationFailure(logger, configErr)
	if strings.Contains(output.String(), "secret-canary") {
		t.Fatalf("configuration value reached log: %s", output.String())
	}
	for _, field := range []string{"invalid_configuration", "STACKPLORE_HTTP_ADDRESS"} {
		if !strings.Contains(output.String(), field) {
			t.Errorf("configuration log lacks %q: %s", field, output.String())
		}
	}
}

func TestLifecycleReasonIsClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err  error
		want string
	}{
		{errHTTPBoundary, "http_boundary"},
		{errListen, "listener"},
		{errShutdown, "shutdown_timeout"},
		{errAuthStart, "authentication_start"},
		{errStorageStart, "storage_start"},
		{errStorageStop, "storage_stop"},
		{errProductStart, "product_start"},
		{errAlertStart, "alert_start"},
		{errServe, "unexpected_stop"},
		{errors.New("raw secret-canary"), "unexpected_stop"},
	}
	for _, test := range tests {
		if got := lifecycleReason(test.err); got != test.want {
			t.Errorf("lifecycleReason(%v) = %q; want %q", test.err, got, test.want)
		}
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition did not become true")
		}
		time.Sleep(time.Millisecond)
	}
}

type failingListener struct{}

func (failingListener) Accept() (net.Conn, error) { return nil, errors.New("raw accept failure") }
func (failingListener) Close() error              { return nil }
func (failingListener) Addr() net.Addr            { return testAddress("failed") }

type testAddress string

func (address testAddress) Network() string { return "test" }
func (address testAddress) String() string  { return string(address) }

type fakeDatabase struct {
	auth.Repository
	available     atomic.Bool
	closed        atomic.Bool
	setupRequired bool
	setupErr      error
	authKey       []byte
	authKeyErr    error
}

type fakeDeliveryDatabase struct{ *fakeDatabase }

func (*fakeDeliveryDatabase) EnqueueWebhookDeliveries(context.Context, string, int, time.Time, int) (int, error) {
	return 0, nil
}

func (*fakeDeliveryDatabase) ClaimWebhookDeliveries(context.Context, string, time.Time, time.Duration, int) ([]notification.DeliveryTask, error) {
	return nil, nil
}

func (*fakeDeliveryDatabase) CompleteWebhookDelivery(context.Context, notification.DeliveryCompletion) error {
	return nil
}

func (database *fakeDatabase) SetupRequired(context.Context) (bool, error) {
	return database.setupRequired, database.setupErr
}

func (database *fakeDatabase) AuthDigestKey() (string, []byte, error) {
	if database.authKeyErr != nil {
		return "", nil, database.authKeyErr
	}
	if database.authKey == nil {
		database.authKey = bytes.Repeat([]byte{7}, auth.DigestKeyBytes)
	}
	return "key_test", database.authKey, nil
}

func (database *fakeDatabase) Health(context.Context) error {
	if !database.available.Load() {
		return errors.New("database unavailable")
	}
	return nil
}

func (database *fakeDatabase) ClaimDueAlertEvaluations(context.Context, time.Time, int) ([]alerting.EvaluationTask, error) {
	return nil, nil
}

func (database *fakeDatabase) SaveAlertMutation(context.Context, domain.AlertMutation) error {
	return nil
}

func (database *fakeDatabase) AcknowledgeAlert(context.Context, domain.AlertAcknowledgement) (domain.Alert, error) {
	return domain.Alert{}, domain.ErrRepositoryNotFound
}

func (database *fakeDatabase) ListAlertRules(context.Context, domain.PageRequest) (domain.AlertRulePage, error) {
	return domain.AlertRulePage{}, nil
}
func (database *fakeDatabase) GetAlertRule(context.Context, domain.AlertRuleID) (domain.AlertRuleRecord, error) {
	return domain.AlertRuleRecord{}, domain.ErrRepositoryNotFound
}
func (database *fakeDatabase) CreateAlertRule(context.Context, domain.AlertRuleCreate) error {
	return nil
}
func (database *fakeDatabase) ReplaceAlertRule(context.Context, domain.AlertRuleReplace) error {
	return nil
}
func (database *fakeDatabase) DeleteAlertRule(context.Context, domain.AlertRuleDelete) error {
	return nil
}

func (database *fakeDatabase) Close() error {
	database.closed.Store(true)
	return nil
}

func testAuthentication(t *testing.T, settings config.Config) *auth.Service {
	t.Helper()
	service, err := buildAuthentication(context.Background(), settings, &fakeDatabase{}, time.Now)
	if err != nil {
		t.Fatalf("buildAuthentication() error = %v", err)
	}
	return service
}
