package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"
	"sync"
)

const requestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

type requestIDSource func() (string, error)

func secureRequestID() (string, error) {
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "req_" + base64.RawURLEncoding.EncodeToString(random), nil
}

func validRequestID(value string) bool {
	if len(value) < 1 || len(value) > 128 || !asciiLetter(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !asciiLetter(character) && !asciiDigit(character) && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func asciiLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func asciiDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func requestIDMiddleware(source requestIDSource, hsts bool, logger *slog.Logger, metrics *Metrics) func(http.Handler) http.Handler {
	var logFailure sync.Once
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := ""
			values := r.Header.Values(requestIDHeader)
			if len(values) == 1 && validRequestID(values[0]) {
				requestID = values[0]
			}
			if requestID == "" {
				generated, err := source()
				if err != nil || !validRequestID(generated) {
					requestID = "req_unavailable"
					r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID))
					if metrics != nil {
						metrics.recordRequestIDFailure()
					}
					if logger != nil {
						logFailure.Do(func() {
							logger.ErrorContext(r.Context(), "request ID unavailable",
								"request_id", requestID,
								"method", methodLabel(r.Method),
								"route", routeLabel(r.URL.Path),
							)
						})
					}
					w.Header().Set(requestIDHeader, requestID)
					if isOperationalRequest(r) {
						next.ServeHTTP(w, r)
						return
					}
					w.Header().Set("Retry-After", "1")
					applyFixedHeaders(w.Header(), r.URL.Path, http.StatusServiceUnavailable, requestID, hsts)
					if isAPIPath(r.URL.Path) {
						writeProblem(w, r, unavailableProblem)
					} else {
						writePlain(w, http.StatusServiceUnavailable, "service unavailable\n")
					}
					return
				}
				requestID = generated
			}

			w.Header().Set(requestIDHeader, requestID)
			ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
