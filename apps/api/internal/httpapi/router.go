package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	minimumRequestBodyBytes  = 1024
	maximumRequestBodyBytes  = 16 * 1024 * 1024
	minimumResponseBodyBytes = 64 * 1024
	maximumResponseBodyBytes = 64 * 1024 * 1024
	minimumInFlightRequests  = 1
	maximumInFlightRequests  = 4096
)

var errInvalidOptions = errors.New("invalid HTTP service options")

// Options contains validated dependencies and limits for the HTTP service.
type Options struct {
	Logger                 *slog.Logger
	Readiness              *Readiness
	PublicOrigin           Origin
	TrustedProxyPrefixes   []netip.Prefix
	MetricsAllowedPrefixes []netip.Prefix
	RequestTimeout         time.Duration
	MaximumRequestBody     int64
	MaximumResponseBody    int64
	MaximumInFlight        int
	RateLimit              RateLimitConfig

	requestIDs requestIDSource
	now        func() time.Time
}

// NewHandler builds the complete HTTP boundary.
func NewHandler(options Options) (http.Handler, error) {
	if err := validateOptions(options); err != nil {
		return nil, err
	}

	requestIDs := options.requestIDs
	if requestIDs == nil {
		requestIDs = secureRequestID
	}
	now := options.now
	if now == nil {
		now = time.Now
	}

	proxy, err := newProxyPolicy(options.PublicOrigin, options.TrustedProxyPrefixes)
	if err != nil {
		return nil, errInvalidOptions
	}
	limiter, err := newRateLimiter(options.RateLimit, now)
	if err != nil {
		return nil, errInvalidOptions
	}
	metrics := NewMetrics(options.Readiness)

	root := chi.NewRouter()
	root.Get("/healthz", health)
	root.Get("/readyz", ready(options.Readiness))
	root.Get("/metrics", requireMetricsAccess(proxy, options.MetricsAllowedPrefixes, http.HandlerFunc(metrics.serveHTTP)).ServeHTTP)
	root.Mount("/api/v1", apiRouter(proxy, options.MaximumRequestBody))
	root.NotFound(plainNotFound)
	root.MethodNotAllowed(plainMethodNotAllowed)

	var handler http.Handler = root
	handler = rejectEscapedPathMiddleware(handler)
	handler = safeExecutionMiddleware(
		options.RequestTimeout,
		int(options.MaximumResponseBody),
		options.MaximumInFlight,
		options.Logger,
		metrics,
	)(handler)
	handler = rateLimitMiddleware(limiter, metrics)(handler)
	handler = accessLogMiddleware(options.Logger, now)(handler)
	handler = metrics.middleware(handler)
	handler = proxy.middleware(handler)
	handler = fixedHeadersMiddleware(proxy.origin.Scheme == "https")(handler)
	handler = requestIDMiddleware(requestIDs, proxy.origin.Scheme == "https", options.Logger, metrics)(handler)
	return handler, nil
}

func validateOptions(options Options) error {
	if options.Logger == nil ||
		options.Readiness == nil ||
		options.RequestTimeout < 100*time.Millisecond ||
		options.RequestTimeout > 2*time.Minute ||
		options.MaximumRequestBody < minimumRequestBodyBytes ||
		options.MaximumRequestBody > maximumRequestBodyBytes ||
		options.MaximumResponseBody < minimumResponseBodyBytes ||
		options.MaximumResponseBody > maximumResponseBodyBytes ||
		options.MaximumInFlight < minimumInFlightRequests ||
		options.MaximumInFlight > maximumInFlightRequests {
		return errInvalidOptions
	}
	if !validPrefixes(options.TrustedProxyPrefixes) || !validPrefixes(options.MetricsAllowedPrefixes) {
		return errInvalidOptions
	}
	return nil
}

func validPrefixes(prefixes []netip.Prefix) bool {
	seen := make(map[netip.Prefix]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		if !prefix.IsValid() ||
			prefix.Bits() == 0 ||
			prefix != prefix.Masked() ||
			prefix.Addr().Is4In6() {
			return false
		}
		if _, exists := seen[prefix]; exists {
			return false
		}
		seen[prefix] = struct{}{}
	}
	return true
}

func apiRouter(proxy proxyPolicy, maximumBody int64) http.Handler {
	router := chi.NewRouter()
	router.Use(proxy.requirePublicOrigin)
	router.Use(limitRequestBody(maximumBody))
	router.Get("/system/readiness", closedReadiness)
	router.NotFound(apiNotFound)
	router.MethodNotAllowed(apiMethodNotAllowed)
	return router
}

func isAPIPath(path string) bool {
	return path == "/api/v1" || strings.HasPrefix(path, "/api/v1/")
}

func isOperationalPath(path string) bool {
	return path == "/healthz" || path == "/readyz" || path == "/metrics"
}

func isOperationalRequest(r *http.Request) bool {
	return r.URL.EscapedPath() == r.URL.Path && isOperationalPath(r.URL.Path)
}

func apiNotFound(w http.ResponseWriter, r *http.Request) {
	writeProblem(w, r, notFoundProblem)
}

func apiMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", http.MethodGet)
	writeProblem(w, r, methodProblem)
}

func plainNotFound(w http.ResponseWriter, r *http.Request) {
	writePlain(w, http.StatusNotFound, "not found\n")
}

func plainMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", http.MethodGet)
	writePlain(w, http.StatusMethodNotAllowed, "method not allowed\n")
}
