package httpapi

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func safeTestRequest() *http.Request {
	request := httptest.NewRequest(http.MethodGet, "http://stackplore.test/api/v1/test", nil)
	return request.WithContext(context.WithValue(request.Context(), requestIDContextKey{}, "req_safe"))
}

func TestSafeExecutionSealsPanicAfterPartialResponse(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	metrics := NewMetrics(NewReadiness())
	handler := safeExecutionMiddleware(
		time.Second,
		64*1024,
		2,
		slog.New(slog.NewJSONHandler(&logs, nil)),
		metrics,
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Unsafe", "secret-canary")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "partial-secret-canary")
		panic("secret-canary")
	}))

	response := serve(handler, safeTestRequest())
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusInternalServerError)
	}
	got := decodeProblem(t, response)
	if got.Code != internalProblem.code || got.RequestID != "req_safe" {
		t.Errorf("problem = %#v", got)
	}
	if strings.Contains(response.Body.String(), "secret-canary") || response.Header().Get("X-Unsafe") != "" {
		t.Errorf("partial response escaped: headers=%v body=%q", response.Header(), response.Body.String())
	}
	if strings.Contains(logs.String(), "secret-canary") || metrics.panics.Load() != 1 {
		t.Errorf("panic handling leaked data or missed metric: log=%s metric=%d", logs.String(), metrics.panics.Load())
	}
}

func TestSafeExecutionRejectsOverflowAndBadStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		handler http.Handler
	}{
		{
			name: "overflow",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "secret-canary")
			}),
		},
		{
			name: "bad status",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(99)
				_, _ = io.WriteString(w, "secret-canary")
			}),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metrics := NewMetrics(NewReadiness())
			handler := safeExecutionMiddleware(time.Second, 4, 2, slog.New(slog.NewJSONHandler(io.Discard, nil)), metrics)(test.handler)
			response := serve(handler, safeTestRequest())
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d; want %d", response.Code, http.StatusInternalServerError)
			}
			if strings.Contains(response.Body.String(), "secret-canary") {
				t.Errorf("unsafe body escaped: %q", response.Body.String())
			}
		})
	}
}

func TestSafeExecutionTimeoutCancelsHandler(t *testing.T) {
	t.Parallel()
	metrics := NewMetrics(NewReadiness())
	canceled := make(chan struct{})
	handler := safeExecutionMiddleware(
		20*time.Millisecond,
		64*1024,
		2,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
		metrics,
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(canceled)
	}))

	response := serve(handler, safeTestRequest())
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "1" {
		t.Fatalf("timeout response = (%d, %q)", response.Code, response.Header().Get("Retry-After"))
	}
	if got := decodeProblem(t, response); got.Code != timeoutProblem.code {
		t.Errorf("timeout problem = %#v", got)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("handler did not see cancellation")
	}
	if metrics.timeouts.Load() != 1 {
		t.Errorf("timeout metric = %d; want 1", metrics.timeouts.Load())
	}
}

func TestSafeExecutionKeepsIgnoredHandlersBounded(t *testing.T) {
	t.Parallel()
	metrics := NewMetrics(NewReadiness())
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			writePlain(w, http.StatusOK, healthBody)
			return
		}
		once.Do(func() { close(started) })
		<-release
	})
	handler := safeExecutionMiddleware(
		20*time.Millisecond,
		64*1024,
		1,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
		metrics,
	)(inner)

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		firstDone <- serve(handler, safeTestRequest())
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("first handler did not start")
	}
	healthRequest := httptest.NewRequest(http.MethodGet, "http://stackplore.test/healthz", nil)
	healthResponse := serve(handler, healthRequest)
	if healthResponse.Code != http.StatusOK || healthResponse.Body.String() != healthBody {
		close(release)
		t.Fatalf("health under API saturation = (%d, %q)", healthResponse.Code, healthResponse.Body.String())
	}

	second := serve(handler, safeTestRequest())
	if second.Code != http.StatusServiceUnavailable {
		close(release)
		t.Fatalf("overload status = %d", second.Code)
	}
	if got := decodeProblem(t, second); got.Code != unavailableProblem.code {
		t.Errorf("overload problem = %#v", got)
	}
	if metrics.overloaded.Load() != 1 {
		t.Errorf("overload metric = %d; want 1", metrics.overloaded.Load())
	}

	first := <-firstDone
	if first.Code != http.StatusServiceUnavailable {
		close(release)
		t.Errorf("first status = %d", first.Code)
	}
	close(release)
}

func TestSafeExecutionRejectsCanceledParent(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodGet, "http://stackplore.test/api/v1/test", nil)
	request = request.WithContext(context.WithValue(ctx, requestIDContextKey{}, "req_canceled"))
	called := false
	handler := safeExecutionMiddleware(
		time.Second,
		64*1024,
		1,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
		NewMetrics(NewReadiness()),
	)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	response := serve(handler, request)
	if response.Code != http.StatusServiceUnavailable || called {
		t.Errorf("canceled request = (status %d, called %t)", response.Code, called)
	}
}

func TestSafeExecutionHoldsSlotThroughResponseCommit(t *testing.T) {
	t.Parallel()
	metrics := NewMetrics(NewReadiness())
	handler := safeExecutionMiddleware(
		time.Second,
		64*1024,
		1,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
		metrics,
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))

	blocked := newBlockingResponseWriter()
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		handler.ServeHTTP(blocked, safeTestRequest())
	}()
	select {
	case <-blocked.started:
	case <-time.After(time.Second):
		close(blocked.release)
		t.Fatal("first response did not reach commit")
	}

	second := serve(handler, safeTestRequest())
	if second.Code != http.StatusServiceUnavailable || metrics.overloaded.Load() != 1 {
		close(blocked.release)
		t.Fatalf("blocked-commit overload = (status %d, metric %d)", second.Code, metrics.overloaded.Load())
	}
	close(blocked.release)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first response did not finish")
	}
}

type blockingResponseWriter struct {
	header  http.Header
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingResponseWriter() *blockingResponseWriter {
	return &blockingResponseWriter{
		header:  make(http.Header),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (writer *blockingResponseWriter) Header() http.Header {
	return writer.header
}

func (writer *blockingResponseWriter) WriteHeader(int) {}

func (writer *blockingResponseWriter) Write(value []byte) (int, error) {
	writer.once.Do(func() { close(writer.started) })
	<-writer.release
	return len(value), nil
}
