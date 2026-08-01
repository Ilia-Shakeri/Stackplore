package httpapi

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func accessLogMiddleware(logger *slog.Logger, now func() time.Time) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isOperationalRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			started := now()
			wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(wrapped, r)
			status := wrapped.Status()
			if status == 0 {
				status = http.StatusOK
			}
			if status == http.StatusTooManyRequests {
				return
			}
			elapsed := now().Sub(started)
			if elapsed < 0 {
				elapsed = 0
			}
			logger.InfoContext(r.Context(), "request complete",
				"request_id", requestIDFromContext(r.Context()),
				"method", methodLabel(r.Method),
				"route", routeLabel(r.URL.Path),
				"status", status,
				"duration_ms", elapsed.Milliseconds(),
			)
		})
	}
}

func limitRequestBody(maximum int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength < -1 {
				writeProblem(w, r, invalidRequestProblem)
				return
			}
			if r.ContentLength > maximum {
				writeProblem(w, r, payloadProblem)
				return
			}
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maximum)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rejectEscapedPathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != r.URL.Path {
			if isAPIPath(r.URL.Path) {
				writeProblem(w, r, invalidRequestProblem)
				return
			}
			writePlain(w, http.StatusNotFound, "not found\n")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func rateLimitMiddleware(limiter *rateLimiter, metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isOperationalRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			info := networkInfoFromContext(r.Context())
			invalidNetwork := info.err != nil || info.untrustedForwarding || !info.clientVerified || !info.clientIP.IsValid()
			key := "invalid_network"
			if info.clientIP.IsValid() {
				key = info.clientIP.String()
			}
			allowed, wait := limiter.allow(key)
			if !allowed {
				metrics.recordRateLimit()
				seconds := int64(math.Ceil(wait.Seconds()))
				if seconds < 1 {
					seconds = 1
				}
				if seconds > 86400 {
					seconds = 86400
				}
				w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
				if isAPIPath(r.URL.Path) {
					writeProblem(w, r, rateLimitProblem)
				} else {
					writePlain(w, http.StatusTooManyRequests, "too many requests\n")
				}
				return
			}
			if invalidNetwork {
				if isAPIPath(r.URL.Path) {
					writeProblem(w, r, invalidRequestProblem)
				} else {
					writePlain(w, http.StatusNotFound, "not found\n")
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
