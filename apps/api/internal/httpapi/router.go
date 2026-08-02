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
	Authentication         authenticationBoundary
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
	root.Mount("/api/v1", apiRouter(
		proxy,
		options.MaximumRequestBody,
		options.Authentication,
		options.Readiness,
		now,
	))
	root.NotFound(plainNotFound)
	root.MethodNotAllowed(rootMethodNotAllowed)

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
		options.Authentication == nil ||
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

func apiRouter(proxy proxyPolicy, maximumBody int64, authentication authenticationBoundary, readiness *Readiness, now func() time.Time) http.Handler {
	authRoutes := newAuthHTTP(authentication, proxy.origin, readiness, now, maximumBody)
	router := chi.NewRouter()
	router.Use(proxy.requirePublicOrigin)
	router.Use(limitRequestBody(maximumBody))
	router.Get("/setup/status", authRoutes.setupStatus)
	router.Post("/setup", authRoutes.setup)
	router.Post("/auth/preflight", authRoutes.preflight)
	router.Post("/auth/login", authRoutes.login)
	router.Get("/auth/session", authRoutes.session)
	router.Post("/auth/logout", authRoutes.logout)
	router.Post("/auth/recent-grants", authRoutes.recentGrant)
	router.Patch("/auth/password", authRoutes.changeOwnPassword)
	router.Get("/users", authRoutes.listUsers)
	router.Post("/users", authRoutes.createUser)
	router.Get("/users/{user_id}", authRoutes.getUser)
	router.Patch("/users/{user_id}", authRoutes.updateUser)
	router.Delete("/users/{user_id}", authRoutes.deleteUser)
	router.Post("/users/{user_id}/password-reset", authRoutes.resetUserPassword)
	router.Delete("/users/{user_id}/sessions", authRoutes.revokeUserSessions)
	router.Get("/system/readiness", authRoutes.systemReadiness)
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
	if allowed := allowedAPIMethods(r.URL.Path); allowed != "" {
		w.Header().Set("Allow", allowed)
	}
	writeProblem(w, r, methodProblem)
}

func allowedAPIMethods(path string) string {
	path = strings.TrimPrefix(path, "/api/v1")
	switch path {
	case "/setup/status", "/auth/session", "/system/readiness":
		return http.MethodGet
	case "/setup", "/auth/preflight", "/auth/login", "/auth/logout", "/auth/recent-grants":
		return http.MethodPost
	case "/auth/password":
		return http.MethodPatch
	case "/users":
		return http.MethodGet + ", " + http.MethodPost
	}
	parts := strings.Split(strings.TrimPrefix(path, "/users/"), "/")
	if len(parts) == 1 && parts[0] != "" {
		return http.MethodGet + ", " + http.MethodPatch + ", " + http.MethodDelete
	}
	if len(parts) == 2 && parts[0] != "" {
		switch parts[1] {
		case "password-reset":
			return http.MethodPost
		case "sessions":
			return http.MethodDelete
		}
	}
	return ""
}

func plainNotFound(w http.ResponseWriter, r *http.Request) {
	writePlain(w, http.StatusNotFound, "not found\n")
}

func plainMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", http.MethodGet)
	writePlain(w, http.StatusMethodNotAllowed, "method not allowed\n")
}

func rootMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	if isAPIPath(r.URL.Path) {
		apiMethodNotAllowed(w, r)
		return
	}
	plainMethodNotAllowed(w, r)
}
