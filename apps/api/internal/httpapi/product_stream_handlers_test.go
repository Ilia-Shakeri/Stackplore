package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
)

type streamRecorder struct {
	mu     sync.Mutex
	header http.Header
	status int
	writes chan []byte
}

func newStreamRecorder() *streamRecorder {
	return &streamRecorder{header: make(http.Header), writes: make(chan []byte, 32)}
}

func (recorder *streamRecorder) Header() http.Header { return recorder.header }

func (recorder *streamRecorder) WriteHeader(status int) {
	recorder.mu.Lock()
	if recorder.status == 0 {
		recorder.status = status
	}
	recorder.mu.Unlock()
}

func (recorder *streamRecorder) Write(value []byte) (int, error) {
	recorder.WriteHeader(http.StatusOK)
	recorder.writes <- bytes.Clone(value)
	return len(value), nil
}

func (*streamRecorder) Flush() {}

func TestAuthenticatedHealthStreamFlushesTypedFrameAndStopsOnCancel(t *testing.T) {
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{User: auth.User{ID: "usr_stream_01"}}, nil
		}}
	})
	ctx, cancel := context.WithCancel(context.Background())
	request := makeRequest(http.MethodGet, "/api/v1/streams/health", "198.51.100.10:4000").WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	recorder := newStreamRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(recorder, request)
		close(done)
	}()

	var body bytes.Buffer
	deadline := time.After(3 * time.Second)
	for !bytes.Contains(body.Bytes(), []byte("event: health.snapshot.v1")) || !bytes.Contains(body.Bytes(), []byte(`"topic":"health"`)) {
		select {
		case chunk := <-recorder.writes:
			body.Write(chunk)
		case <-deadline:
			cancel()
			t.Fatalf("stream frame timed out: %q", body.String())
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not stop after cancellation")
	}
	recorder.mu.Lock()
	status := recorder.status
	recorder.mu.Unlock()
	if status != http.StatusOK || recorder.Header().Get("Content-Type") != "text/event-stream; charset=utf-8" ||
		recorder.Header().Get("X-Accel-Buffering") != "no" || !bytes.Contains(body.Bytes(), []byte("id: 1")) {
		t.Fatalf("stream = status %d, headers %#v, body %q", status, recorder.Header(), body.String())
	}
}

func TestStreamRejectsMissingSessionAndMalformedReplayID(t *testing.T) {
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{User: auth.User{ID: "usr_stream_01"}}, nil
		}}
	})
	missing := serve(handler, makeRequest(http.MethodGet, "/api/v1/streams/events", "198.51.100.10:4000"))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status = %d", missing.Code)
	}
	request := makeRequest(http.MethodGet, "/api/v1/streams/events", "198.51.100.10:4000")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	request.Header.Set("Last-Event-ID", "01")
	malformed := serve(handler, request)
	if malformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed replay status = %d", malformed.Code)
	}
}

func TestStreamGateEnforcesPerUserAndGlobalLimits(t *testing.T) {
	gate := newStreamGate(2, 1)
	firstRelease, ok := gate.acquire("usr_one")
	if !ok {
		t.Fatal("first stream denied")
	}
	if _, ok := gate.acquire("usr_one"); ok {
		t.Fatal("per-user stream limit missed")
	}
	secondRelease, ok := gate.acquire("usr_two")
	if !ok {
		t.Fatal("second user stream denied")
	}
	if _, ok := gate.acquire("usr_three"); ok {
		t.Fatal("global stream limit missed")
	}
	firstRelease()
	firstRelease()
	if release, ok := gate.acquire("usr_three"); !ok {
		t.Fatal("released stream slot stayed closed")
	} else {
		release()
	}
	secondRelease()
}
