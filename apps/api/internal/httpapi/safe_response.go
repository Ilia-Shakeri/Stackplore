package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type responseBuffer struct {
	mu       sync.Mutex
	header   http.Header
	body     bytes.Buffer
	status   int
	maximum  int
	closed   bool
	overflow bool
	invalid  bool
}

func newResponseBuffer(maximum int) *responseBuffer {
	return &responseBuffer{header: make(http.Header), maximum: maximum}
}

func (buffer *responseBuffer) Header() http.Header {
	return buffer.header
}

func (buffer *responseBuffer) WriteHeader(status int) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if buffer.closed || buffer.status != 0 {
		return
	}
	if status < 200 || status > 599 {
		buffer.invalid = true
		return
	}
	buffer.status = status
}

func (buffer *responseBuffer) Write(value []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if buffer.closed {
		return 0, http.ErrHandlerTimeout
	}
	if buffer.status == 0 {
		buffer.status = http.StatusOK
	}
	if len(value) > buffer.maximum-buffer.body.Len() {
		buffer.overflow = true
		return 0, http.ErrContentLength
	}
	return buffer.body.Write(value)
}

func (buffer *responseBuffer) close() {
	buffer.mu.Lock()
	buffer.closed = true
	buffer.mu.Unlock()
}

func (buffer *responseBuffer) commit(w http.ResponseWriter) bool {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if buffer.overflow || buffer.invalid {
		return false
	}
	for name, values := range buffer.header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	status := buffer.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(buffer.body.Bytes())
	return true
}

type executionResult struct {
	panicked bool
}

func safeExecutionMiddleware(
	timeout time.Duration,
	maximumResponseBytes int,
	maximumInFlight int,
	logger *slog.Logger,
	metrics *Metrics,
) func(http.Handler) http.Handler {
	slots := make(chan struct{}, maximumInFlight)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isStreamPath(r.URL.Path) {
				defer func() {
					if recover() != nil {
						metrics.recordPanic()
						logger.ErrorContext(r.Context(), "stream panic recovered",
							"request_id", requestIDFromContext(r.Context()),
							"method", methodLabel(r.Method),
							"route", routeLabel(r.URL.Path),
						)
					}
				}()
				next.ServeHTTP(w, r)
				return
			}
			if !isAPIPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if r.Context().Err() != nil {
				w.Header().Set("Retry-After", "1")
				writeProblem(w, r, unavailableProblem)
				return
			}
			select {
			case slots <- struct{}{}:
			case <-r.Context().Done():
				w.Header().Set("Retry-After", "1")
				writeProblem(w, r, unavailableProblem)
				return
			default:
				metrics.recordOverload()
				w.Header().Set("Retry-After", "1")
				writeProblem(w, r, unavailableProblem)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			buffer := newResponseBuffer(maximumResponseBytes)
			done := make(chan executionResult, 1)
			var participants atomic.Int32
			participants.Store(2)
			releaseSlot := func() {
				if participants.Add(-1) == 0 {
					<-slots
				}
			}
			defer releaseSlot()
			writeStopped := func() {
				buffer.close()
				kind := timeoutProblem
				reason := "timeout"
				if errors.Is(r.Context().Err(), context.Canceled) {
					kind = unavailableProblem
					reason = "canceled"
				} else {
					metrics.recordTimeout()
				}
				logger.WarnContext(r.Context(), "request stopped",
					"request_id", requestIDFromContext(r.Context()),
					"method", methodLabel(r.Method),
					"route", routeLabel(r.URL.Path),
					"reason", reason,
				)
				w.Header().Set("Retry-After", "1")
				writeProblem(w, r, kind)
			}

			go func() {
				defer releaseSlot()
				result := executionResult{}
				defer func() {
					if recover() != nil {
						result.panicked = true
					}
					done <- result
				}()
				next.ServeHTTP(buffer, r.WithContext(ctx))
			}()

			select {
			case result := <-done:
				buffer.close()
				if ctx.Err() != nil {
					writeStopped()
					return
				}
				if result.panicked {
					metrics.recordPanic()
					logger.ErrorContext(r.Context(), "request panic recovered",
						"request_id", requestIDFromContext(r.Context()),
						"method", methodLabel(r.Method),
						"route", routeLabel(r.URL.Path),
					)
					writeProblem(w, r, internalProblem)
					return
				}
				if !buffer.commit(w) {
					logger.ErrorContext(r.Context(), "response exceeded safe limit",
						"request_id", requestIDFromContext(r.Context()),
						"method", methodLabel(r.Method),
						"route", routeLabel(r.URL.Path),
					)
					writeProblem(w, r, internalProblem)
				}
			case <-ctx.Done():
				writeStopped()
			}
		})
	}
}
