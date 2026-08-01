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
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
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
	server, err := newHTTPServer(settings, discardLogger(t), httpapi.NewReadiness())
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
	server, err := newHTTPServer(settings, discardLogger(t), readiness)
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
	err := run(context.Background(), settings, logger, func(string, string) (net.Listener, error) {
		return nil, wantListenError
	})
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

func TestServeServerSealsAcceptFailure(t *testing.T) {
	t.Parallel()
	readiness := httpapi.NewReadiness()
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
