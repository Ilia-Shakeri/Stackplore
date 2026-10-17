package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
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
	Logger                  *slog.Logger
	Readiness               *Readiness
	Authentication          authenticationBoundary
	PublicOrigin            Origin
	TrustedProxyPrefixes    []netip.Prefix
	MetricsAllowedPrefixes  []netip.Prefix
	RequestTimeout          time.Duration
	MaximumRequestBody      int64
	MaximumResponseBody     int64
	MaximumInFlight         int
	RateLimit               RateLimitConfig
	ProductProviders        []ProductProvider
	AuditReader             AuditReader
	AlertReader             AlertReader
	NotificationReader      NotificationReader
	NotificationSettings    NotificationSettingsReader
	NotificationTester      NotificationTester
	AlertManager            AlertManager
	SourceSettings          SourceSettings
	Operations              OperationManager
	ApplicationSettings     ApplicationSettings
	ApplicationMutationMode ApplicationMutationMode
	GuestDemo               *GuestDemoOptions

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
		options.ProductProviders,
		options.AuditReader,
		options.AlertReader,
		options.NotificationReader,
		options.NotificationSettings,
		options.NotificationTester,
		options.AlertManager,
		options.SourceSettings,
		options.Operations,
		options.ApplicationSettings,
		options.ApplicationMutationMode,
		options.GuestDemo,
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
	if err := validateProductProviders(options.ProductProviders); err != nil {
		return errInvalidOptions
	}
	if !validGuestDemo(options.GuestDemo, options.ProductProviders) {
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

func apiRouter(proxy proxyPolicy, maximumBody int64, authentication authenticationBoundary, readiness *Readiness, now func() time.Time, providers []ProductProvider, auditReader AuditReader, alertReader AlertReader, notificationReader NotificationReader, notificationSettings NotificationSettingsReader, notificationTester NotificationTester, alertManager AlertManager, sourceSettings SourceSettings, operations OperationManager, applicationSettings ApplicationSettings, applicationMode ApplicationMutationMode, guest ...*GuestDemoOptions) http.Handler {
	authRoutes := newAuthHTTP(authentication, proxy.origin, readiness, now, maximumBody)
	if len(guest) == 1 {
		authRoutes.guestDemo = guest[0]
	}
	if len(providers) == 0 {
		providers = []ProductProvider{demo.MustNew()}
	}
	productRoutes := newProductHTTP(authRoutes, providers, auditReader, alertReader, notificationReader, notificationSettings, notificationTester, alertManager)
	productRoutes.sourceSettings = sourceSettings
	productRoutes.operations = operations
	productRoutes.applicationSettings = applicationSettings
	productRoutes.applicationMutationMode = applicationMode
	router := chi.NewRouter()
	router.Use(proxy.requirePublicOrigin)
	router.Use(limitRequestBody(maximumBody))
	router.Use(rejectMalformedQuery)
	router.Use(guestReadBoundary(authRoutes.guestDemo))
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
	router.Get("/sources", productRoutes.sources)
	router.Get("/environments", productRoutes.environments)
	router.Get("/resources", productRoutes.resources)
	router.Get("/resources/{resource_id}", productRoutes.resource)
	router.Get("/resources/{resource_id}/health", productRoutes.health)
	router.Get("/topology", productRoutes.topology)
	router.Get("/logs", productRoutes.logs)
	router.Get("/metrics/current", productRoutes.metrics)
	router.Get("/metrics/history", productRoutes.metricHistory)
	router.Get("/events", productRoutes.events)
	router.Get("/alerts", productRoutes.alerts)
	router.Post("/alerts/{alert_id}/acknowledge", productRoutes.acknowledgeAlert)
	router.Get("/alert-rules", productRoutes.listAlertRules)
	router.Post("/alert-rules", productRoutes.createAlertRule)
	router.Put("/alert-rules/{rule_id}", productRoutes.replaceAlertRule)
	router.Delete("/alert-rules/{rule_id}", productRoutes.deleteAlertRule)
	router.Get("/notifications", productRoutes.notifications)
	router.Get("/settings/notifications", productRoutes.notificationSettings)
	router.Get("/settings/sources", productRoutes.listSourceSettings)
	router.Get("/settings/application", productRoutes.getApplicationSettings)
	router.Post("/settings/application/previews", productRoutes.previewApplicationSettings)
	router.Put("/settings/application", productRoutes.updateApplicationSettings)
	router.Post("/settings/cleanup/previews", productRoutes.previewHistoryCleanup)
	router.Post("/settings/cleanup", productRoutes.cleanupHistory)
	router.Post("/settings/export", productRoutes.exportConfiguration)
	router.Post("/settings/sources/{source_id}/preview", productRoutes.previewSourceUpdate)
	router.Patch("/settings/sources/{source_id}", productRoutes.updateSourceSettings)
	router.Post("/settings/notifications/test", productRoutes.testNotification)
	router.Get("/audit", productRoutes.audit)
	router.Post("/operations/previews", productRoutes.previewOperation)
	router.Post("/operations", productRoutes.executeOperation)
	router.Get("/operations", productRoutes.listOperations)
	router.Get("/operations/{operation_id}", productRoutes.getOperation)
	router.Get("/streams/health", productRoutes.streamHealth)
	router.Get("/streams/metrics", productRoutes.streamMetrics)
	router.Get("/streams/events", productRoutes.streamEvents)
	router.Get("/streams/logs", productRoutes.streamLogs)
	router.Get("/streams/alerts", productRoutes.streamAlerts)
	router.NotFound(apiNotFound)
	router.MethodNotAllowed(apiMethodNotAllowed)
	return router
}

func isAPIPath(path string) bool {
	return path == "/api/v1" || strings.HasPrefix(path, "/api/v1/")
}

func isStreamPath(path string) bool {
	switch path {
	case "/api/v1/streams/health", "/api/v1/streams/metrics", "/api/v1/streams/events", "/api/v1/streams/logs", "/api/v1/streams/alerts":
		return true
	default:
		return false
	}
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
	case "/setup/status", "/auth/session", "/system/readiness", "/sources", "/environments", "/resources",
		"/topology", "/logs", "/metrics/current", "/metrics/history", "/events", "/streams/health", "/streams/metrics",
		"/streams/events", "/streams/logs", "/streams/alerts", "/audit", "/notifications", "/settings/notifications":
		return http.MethodGet
	case "/alerts":
		return http.MethodGet
	case "/alert-rules":
		return http.MethodGet + ", " + http.MethodPost
	case "/operations":
		return http.MethodGet + ", " + http.MethodPost
	case "/operations/previews":
		return http.MethodPost
	case "/settings/application":
		return http.MethodGet + ", " + http.MethodPut
	case "/settings/application/previews", "/settings/cleanup/previews", "/settings/cleanup", "/settings/export":
		return http.MethodPost
	case "/setup", "/auth/preflight", "/auth/login", "/auth/logout", "/auth/recent-grants":
		return http.MethodPost
	case "/settings/notifications/test":
		return http.MethodPost
	case "/auth/password":
		return http.MethodPatch
	case "/users":
		return http.MethodGet + ", " + http.MethodPost
	}
	alertParts := strings.Split(strings.TrimPrefix(path, "/alerts/"), "/")
	if strings.HasPrefix(path, "/operations/") && len(strings.Split(strings.TrimPrefix(path, "/operations/"), "/")) == 1 {
		return http.MethodGet
	}
	if len(alertParts) == 2 && alertParts[0] != "" && alertParts[1] == "acknowledge" {
		return http.MethodPost
	}
	ruleParts := strings.Split(strings.TrimPrefix(path, "/alert-rules/"), "/")
	if len(ruleParts) == 1 && ruleParts[0] != "" {
		return http.MethodPut + ", " + http.MethodDelete
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
	resourceParts := strings.Split(strings.TrimPrefix(path, "/resources/"), "/")
	if len(resourceParts) == 1 && resourceParts[0] != "" ||
		len(resourceParts) == 2 && resourceParts[0] != "" && resourceParts[1] == "health" {
		return http.MethodGet
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
