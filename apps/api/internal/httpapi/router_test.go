package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func testOptions(logOutput io.Writer) Options {
	return Options{
		Logger:                 slog.New(slog.NewJSONHandler(logOutput, nil)),
		Readiness:              NewReadiness(),
		Authentication:         &testAuthentication{},
		PublicOrigin:           Origin{Scheme: "http", Authority: "stackplore.test"},
		MetricsAllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
		RequestTimeout:         time.Second,
		MaximumRequestBody:     1024,
		MaximumResponseBody:    64 * 1024,
		MaximumInFlight:        8,
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 1000,
			Burst:             1000,
			MaximumClients:    64,
		},
		requestIDs: func() (string, error) { return "req_generated", nil },
		now:        time.Now,
	}
}

func makeHandler(t *testing.T, change func(*Options)) (http.Handler, *Readiness, *bytes.Buffer) {
	t.Helper()
	logs := &bytes.Buffer{}
	options := testOptions(logs)
	if change != nil {
		change(&options)
	}
	handler, err := NewHandler(options)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler, options.Readiness, logs
}

func makeRequest(method, path, remote string) *http.Request {
	request := httptest.NewRequest(method, "http://stackplore.test"+path, nil)
	request.RemoteAddr = remote
	request.Header.Set(requestIDHeader, "req_test")
	return request
}

func serve(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeProblem(t *testing.T, response *httptest.ResponseRecorder) problem {
	t.Helper()
	var result problem
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("problem decode error = %v; body = %q", err, response.Body.String())
	}
	return result
}

func TestHealthAndReadinessLifecycle(t *testing.T) {
	t.Parallel()
	handler, readiness, _ := makeHandler(t, nil)

	healthResponse := serve(handler, makeRequest(http.MethodGet, "/healthz", "192.0.2.10:4000"))
	if healthResponse.Code != http.StatusOK || healthResponse.Body.String() != healthBody {
		t.Fatalf("health response = (%d, %q); want (%d, %q)", healthResponse.Code, healthResponse.Body.String(), http.StatusOK, healthBody)
	}

	notReadyResponse := serve(handler, makeRequest(http.MethodGet, "/readyz", "192.0.2.10:4000"))
	if notReadyResponse.Code != http.StatusServiceUnavailable || notReadyResponse.Body.String() != notReadyBody {
		t.Fatalf("not-ready response = (%d, %q)", notReadyResponse.Code, notReadyResponse.Body.String())
	}

	readiness.SetDatabaseReady(true)
	readiness.SetServiceReady(true)
	readyResponse := serve(handler, makeRequest(http.MethodGet, "/readyz", "192.0.2.10:4000"))
	if readyResponse.Code != http.StatusOK || readyResponse.Body.String() != readyBody {
		t.Fatalf("ready response = (%d, %q)", readyResponse.Code, readyResponse.Body.String())
	}
	if readyResponse.Header().Get("Cache-Control") != "no-store" || readyResponse.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("ready response lacks fixed safety headers: %v", readyResponse.Header())
	}
}

func TestNewHandlerRejectsUnsafeOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*Options)
	}{
		{"nil logger", func(value *Options) { value.Logger = nil }},
		{"nil readiness", func(value *Options) { value.Readiness = nil }},
		{"nil authentication", func(value *Options) { value.Authentication = nil }},
		{"short timeout", func(value *Options) { value.RequestTimeout = time.Millisecond }},
		{"small request body", func(value *Options) { value.MaximumRequestBody = 1 }},
		{"large response body", func(value *Options) { value.MaximumResponseBody = maximumResponseBodyBytes + 1 }},
		{"zero in flight", func(value *Options) { value.MaximumInFlight = 0 }},
		{"broad metrics prefix", func(value *Options) {
			value.MetricsAllowedPrefixes = []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")}
		}},
		{"mapped metrics prefix", func(value *Options) {
			value.MetricsAllowedPrefixes = []netip.Prefix{netip.MustParsePrefix("::ffff:192.0.2.0/120")}
		}},
		{"unmasked proxy prefix", func(value *Options) {
			value.TrustedProxyPrefixes = []netip.Prefix{netip.PrefixFrom(netip.MustParseAddr("10.0.0.1"), 8)}
		}},
		{"duplicate proxy prefix", func(value *Options) {
			prefix := netip.MustParsePrefix("10.0.0.0/8")
			value.TrustedProxyPrefixes = []netip.Prefix{prefix, prefix}
		}},
		{"bad origin", func(value *Options) { value.PublicOrigin.Scheme = "ftp" }},
		{"bad rate", func(value *Options) { value.RateLimit.RequestsPerSecond = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := testOptions(io.Discard)
			test.change(&options)
			if _, err := NewHandler(options); err == nil {
				t.Fatal("NewHandler() error = nil")
			}
		})
	}
}

func TestClosedReadinessRoute(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeHandler(t, nil)
	request := makeRequest(http.MethodGet, "/api/v1/system/readiness?claim=admin", "198.51.100.10:4000")
	request.Header.Set("Authorization", "Bearer false-claim")
	request.Header.Set("X-User-Role", "administrator")
	request.AddCookie(&http.Cookie{Name: "session", Value: "false-claim"})

	response := serve(handler, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusUnauthorized)
	}
	got := decodeProblem(t, response)
	if got.Code != unauthenticatedProblem.code || got.RequestID != "req_test" || got.Status != http.StatusUnauthorized {
		t.Errorf("problem = %#v", got)
	}
	if response.Header().Get(requestIDHeader) != "req_test" {
		t.Errorf("response request ID = %q", response.Header().Get(requestIDHeader))
	}
}

func TestEmbeddedWebFallbackIsSameOriginAndNeverCapturesAPI404(t *testing.T) {
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.WebAssets = fstest.MapFS{"index.html": {Data: []byte("<main>web shell</main>")}}
	})
	request := makeRequest(http.MethodGet, "/settings/application", "198.51.100.10:4000")
	request.Header.Set("Accept", "text/html")
	if response := serve(handler, request); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "web shell") {
		t.Fatalf("SPA response = %d %q", response.Code, response.Body.String())
	}
	request = makeRequest(http.MethodGet, "/api/v1/unknown", "198.51.100.10:4000")
	request.Header.Set("Accept", "text/html")
	if response := serve(handler, request); response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "web shell") {
		t.Fatalf("API 404 was captured by SPA: %d %q", response.Code, response.Body.String())
	}
}

func TestRouteBoundaries(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeHandler(t, nil)
	tests := []struct {
		name        string
		method      string
		path        string
		status      int
		contentType string
		allow       string
	}{
		{name: "API base", method: http.MethodGet, path: "/api/v1", status: http.StatusNotFound, contentType: "application/problem+json"},
		{name: "API slash", method: http.MethodGet, path: "/api/v1/", status: http.StatusNotFound, contentType: "application/problem+json"},
		{name: "API miss", method: http.MethodGet, path: "/api/v1/missing", status: http.StatusNotFound, contentType: "application/problem+json"},
		{name: "version lookalike", method: http.MethodGet, path: "/api/v10", status: http.StatusNotFound, contentType: "text/plain; charset=utf-8"},
		{name: "trailing slash", method: http.MethodGet, path: "/api/v1/system/readiness/", status: http.StatusNotFound, contentType: "application/problem+json"},
		{name: "encoded slash", method: http.MethodGet, path: "/api/v1/system%2Freadiness", status: http.StatusBadRequest, contentType: "application/problem+json"},
		{name: "head", method: http.MethodHead, path: "/api/v1/system/readiness", status: http.StatusMethodNotAllowed, contentType: "application/problem+json", allow: http.MethodGet},
		{name: "options", method: http.MethodOptions, path: "/api/v1/system/readiness", status: http.StatusMethodNotAllowed, contentType: "application/problem+json", allow: http.MethodGet},
		{name: "odd method", method: "BREW", path: "/api/v1/system/readiness", status: http.StatusMethodNotAllowed, contentType: "application/problem+json", allow: http.MethodGet},
		{name: "probe method", method: http.MethodPost, path: "/healthz", status: http.StatusMethodNotAllowed, contentType: "text/plain; charset=utf-8", allow: http.MethodGet},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := makeRequest(test.method, test.path, "198.51.100.10:4000")
			response := serve(handler, request)
			if response.Code != test.status {
				t.Fatalf("status = %d; want %d; body = %q", response.Code, test.status, response.Body.String())
			}
			if got := response.Header().Get("Content-Type"); got != test.contentType {
				t.Errorf("Content-Type = %q; want %q", got, test.contentType)
			}
			if got := response.Header().Get("Allow"); got != test.allow {
				t.Errorf("Allow = %q; want %q", got, test.allow)
			}
		})
	}
}

func TestPublicOriginAndForwardingFailClosed(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeHandler(t, nil)

	badOrigin := makeRequest(http.MethodGet, "/api/v1/system/readiness", "198.51.100.10:4000")
	badOrigin.Host = "other.test"
	if response := serve(handler, badOrigin); response.Code != http.StatusBadRequest {
		t.Errorf("bad-origin status = %d; want %d", response.Code, http.StatusBadRequest)
	}

	spoofedForwarding := makeRequest(http.MethodGet, "/api/v1/system/readiness", "198.51.100.10:4000")
	spoofedForwarding.Header.Set("X-Forwarded-For", "127.0.0.1")
	if response := serve(handler, spoofedForwarding); response.Code != http.StatusBadRequest {
		t.Errorf("spoofed-forwarding status = %d; want %d", response.Code, http.StatusBadRequest)
	}
}

func TestFixedHeadersOwnTheResponse(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(requestIDHeader, "spoofed")
		w.Header().Set("Cache-Control", "public")
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Strict-Transport-Security", "bad")
		w.Header().Set("Server", "bad")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "ok")
	})
	handler := requestIDMiddleware(func() (string, error) { return "req_fixed", nil }, false, nil, nil)(fixedHeadersMiddleware(false)(inner))
	request := httptest.NewRequest(http.MethodGet, "http://stackplore.test/healthz", nil)
	response := serve(handler, request)

	if response.Header().Get(requestIDHeader) != "req_fixed" || response.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("fixed headers were replaced: %v", response.Header())
	}
	for _, name := range []string{"Access-Control-Allow-Origin", "Strict-Transport-Security", "Server"} {
		if value := response.Header().Get(name); value != "" {
			t.Errorf("%s = %q; want empty", name, value)
		}
	}
	if got := response.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	wantHeaders := map[string]string{
		"Content-Security-Policy":           "default-src 'none'; base-uri 'none'; frame-ancestors 'none'",
		"Cross-Origin-Resource-Policy":      "same-origin",
		"Permissions-Policy":                "camera=(), geolocation=(), microphone=()",
		"Referrer-Policy":                   "no-referrer",
		"X-Content-Type-Options":            "nosniff",
		"X-Frame-Options":                   "DENY",
		"X-Permitted-Cross-Domain-Policies": "none",
	}
	for name, want := range wantHeaders {
		if got := response.Header().Get(name); got != want {
			t.Errorf("%s = %q; want %q", name, got, want)
		}
	}
}

func TestHTTPSOriginAddsHSTS(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.PublicOrigin = Origin{Scheme: "https", Authority: "stackplore.test"}
	})
	request := httptest.NewRequest(http.MethodGet, "https://stackplore.test/healthz", nil)
	request.RemoteAddr = "198.51.100.10:4000"
	request.Header.Set(requestIDHeader, "req_test")
	response := serve(handler, request)
	if got := response.Header().Get("Strict-Transport-Security"); got != "max-age=31536000" {
		t.Errorf("Strict-Transport-Security = %q", got)
	}
}

func TestRequestIDValidationAndGeneratorFailure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{name: "valid", values: []string{"Client_123"}, want: "Client_123"},
		{name: "bad start", values: []string{"1bad"}, want: "req_generated"},
		{name: "Unicode", values: []string{"Bad-é"}, want: "req_generated"},
		{name: "long", values: []string{"R" + strings.Repeat("x", 128)}, want: "req_generated"},
		{name: "duplicate", values: []string{"First", "Second"}, want: "req_generated"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := requestIDMiddleware(func() (string, error) { return "req_generated", nil }, false, nil, nil)(fixedHeadersMiddleware(false)(http.HandlerFunc(health)))
			request := httptest.NewRequest(http.MethodGet, "http://stackplore.test/healthz", nil)
			for _, value := range test.values {
				request.Header.Add(requestIDHeader, value)
			}
			response := serve(handler, request)
			if got := response.Header().Get(requestIDHeader); got != test.want {
				t.Errorf("request ID = %q; want %q", got, test.want)
			}
		})
	}

	var failureLogs bytes.Buffer
	failureMetrics := NewMetrics(NewReadiness())
	handler := requestIDMiddleware(
		func() (string, error) { return "", io.ErrUnexpectedEOF },
		false,
		slog.New(slog.NewJSONHandler(&failureLogs, nil)),
		failureMetrics,
	)(http.HandlerFunc(health))
	response := serve(handler, httptest.NewRequest(http.MethodGet, "http://stackplore.test/api/v1/test", nil))
	if response.Code != http.StatusServiceUnavailable || response.Header().Get(requestIDHeader) != "req_unavailable" {
		t.Errorf("generator-failure response = (%d, %q)", response.Code, response.Header().Get(requestIDHeader))
	}
	if got := decodeProblem(t, response); got.RequestID != "req_unavailable" || got.Code != unavailableProblem.code {
		t.Errorf("generator-failure problem = %#v", got)
	}
	if failureMetrics.requestIDFailures.Load() != 1 || !strings.Contains(failureLogs.String(), "request ID unavailable") || strings.Contains(failureLogs.String(), "unexpected EOF") {
		t.Errorf("generator-failure signal = (metric %d, log %q)", failureMetrics.requestIDFailures.Load(), failureLogs.String())
	}
	operational := requestIDMiddleware(
		func() (string, error) { return "", io.ErrUnexpectedEOF },
		false,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
		NewMetrics(NewReadiness()),
	)(fixedHeadersMiddleware(false)(http.HandlerFunc(health)))
	operationalResponse := serve(operational, httptest.NewRequest(http.MethodGet, "http://stackplore.test/healthz", nil))
	if operationalResponse.Code != http.StatusOK || operationalResponse.Body.String() != healthBody || operationalResponse.Header().Get(requestIDHeader) != "req_unavailable" {
		t.Errorf("operational generator failure = (%d, %q, %q)", operationalResponse.Code, operationalResponse.Body.String(), operationalResponse.Header().Get(requestIDHeader))
	}
}

func TestMetricsGateAndFixedLabels(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeHandler(t, nil)
	for index := 0; index < 20; index++ {
		path := "/miss-secret-canary-" + strings.Repeat("x", index)
		_ = serve(handler, makeRequest(http.MethodGet, path, "198.51.100.10:4000"))
	}

	denied := serve(handler, makeRequest(http.MethodGet, "/metrics", "198.51.100.10:4000"))
	if denied.Code != http.StatusNotFound || denied.Body.String() != "not found\n" {
		t.Fatalf("denied metrics = (%d, %q)", denied.Code, denied.Body.String())
	}
	hostileHost := makeRequest(http.MethodGet, "/metrics", "127.0.0.1:4000")
	hostileHost.Host = "attacker.test"
	if response := serve(handler, hostileHost); response.Code != http.StatusNotFound || response.Body.String() != "not found\n" {
		t.Fatalf("hostile-host metrics = (%d, %q)", response.Code, response.Body.String())
	}

	allowed := serve(handler, makeRequest(http.MethodGet, "/metrics", "127.0.0.1:4000"))
	if allowed.Code != http.StatusOK || allowed.Header().Get("Content-Type") != metricsContentType {
		t.Fatalf("allowed metrics = (%d, %q)", allowed.Code, allowed.Header().Get("Content-Type"))
	}
	body := allowed.Body.String()
	if !strings.Contains(body, "stackplore_http_requests_total") || strings.Contains(body, "secret-canary") || strings.Contains(body, "127.0.0.1") {
		t.Errorf("metrics body has bad shape: %q", body)
	}
}

func TestRateLimitUsesVerifiedClientAddress(t *testing.T) {
	t.Parallel()
	fixedNow := time.Unix(1_700_000_000, 0)
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.RateLimit = RateLimitConfig{RequestsPerSecond: 0.1, Burst: 1, MaximumClients: 8}
		options.now = func() time.Time { return fixedNow }
	})

	first := makeRequest(http.MethodGet, "/api/v1/system/readiness", "198.51.100.10:4000")
	first.Header.Set(requestIDHeader, "req_first")
	if response := serve(handler, first); response.Code != http.StatusUnauthorized {
		t.Fatalf("first status = %d", response.Code)
	}

	second := makeRequest(http.MethodGet, "/api/v1/system/readiness", "198.51.100.10:5000")
	second.Header.Set(requestIDHeader, "req_second")
	response := serve(handler, second)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "10" {
		t.Fatalf("limited response = (%d, %q)", response.Code, response.Header().Get("Retry-After"))
	}
	if got := decodeProblem(t, response); got.Code != rateLimitProblem.code || got.RequestID != "req_second" {
		t.Errorf("limited problem = %#v", got)
	}

	third := makeRequest(http.MethodGet, "/api/v1/system/readiness", "198.51.100.11:4000")
	if response := serve(handler, third); response.Code != http.StatusUnauthorized {
		t.Errorf("other-client status = %d", response.Code)
	}
}

func TestGlobalRateLimitBoundsMissesAndKeepsProbesOpen(t *testing.T) {
	t.Parallel()
	fixedNow := time.Unix(1_700_000_000, 0)
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.RateLimit = RateLimitConfig{RequestsPerSecond: 0.1, Burst: 1, MaximumClients: 8}
		options.now = func() time.Time { return fixedNow }
	})

	first := serve(handler, makeRequest(http.MethodGet, "/first-miss", "198.51.100.10:4000"))
	if first.Code != http.StatusNotFound {
		t.Fatalf("first miss status = %d", first.Code)
	}
	second := serve(handler, makeRequest(http.MethodGet, "/second-miss", "198.51.100.10:5000"))
	if second.Code != http.StatusTooManyRequests || second.Header().Get("Retry-After") != "10" || second.Body.String() != "too many requests\n" {
		t.Fatalf("limited miss = (%d, %q, %q)", second.Code, second.Header().Get("Retry-After"), second.Body.String())
	}
	healthResponse := serve(handler, makeRequest(http.MethodGet, "/healthz", "198.51.100.10:6000"))
	if healthResponse.Code != http.StatusOK || healthResponse.Body.String() != healthBody {
		t.Errorf("health after rate limit = (%d, %q)", healthResponse.Code, healthResponse.Body.String())
	}
	escapedFirst := serve(handler, makeRequest(http.MethodGet, "/he%61lthz", "198.51.100.11:4000"))
	if escapedFirst.Code != http.StatusNotFound {
		t.Fatalf("first escaped-probe status = %d", escapedFirst.Code)
	}
	escapedSecond := serve(handler, makeRequest(http.MethodGet, "/he%61lthz", "198.51.100.11:5000"))
	if escapedSecond.Code != http.StatusTooManyRequests {
		t.Fatalf("second escaped-probe status = %d", escapedSecond.Code)
	}
}

func TestInvalidNetworkRequestsAreRateBounded(t *testing.T) {
	t.Parallel()
	fixedNow := time.Unix(1_700_000_000, 0)
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.RateLimit = RateLimitConfig{RequestsPerSecond: 0.1, Burst: 1, MaximumClients: 8}
		options.now = func() time.Time { return fixedNow }
	})
	first := makeRequest(http.MethodGet, "/api/v1/system/readiness", "198.51.100.10:4000")
	first.Header.Set("X-Forwarded-For", "127.0.0.1")
	if response := serve(handler, first); response.Code != http.StatusBadRequest {
		t.Fatalf("first invalid-network status = %d", response.Code)
	}
	second := makeRequest(http.MethodGet, "/api/v1/system/readiness", "198.51.100.10:5000")
	second.Header.Set("X-Forwarded-For", "127.0.0.1")
	if response := serve(handler, second); response.Code != http.StatusTooManyRequests {
		t.Fatalf("second invalid-network status = %d", response.Code)
	}
}

func TestAccessLogDoesNotCopyRequestData(t *testing.T) {
	t.Parallel()
	handler, _, logs := makeHandler(t, nil)
	request := makeRequest(http.MethodGet, "/miss?token=secret-canary", "198.51.100.10:4000")
	request.Header.Set("User-Agent", "secret-canary")
	_ = serve(handler, request)
	if strings.Contains(logs.String(), "secret-canary") || strings.Contains(logs.String(), "/miss") || strings.Contains(logs.String(), "198.51.100.10") {
		t.Fatalf("access log copied request data: %s", logs.String())
	}
	for _, field := range []string{"request_id", "method", "route", "status", "duration_ms"} {
		if !strings.Contains(logs.String(), field) {
			t.Errorf("access log lacks %q: %s", field, logs.String())
		}
	}
}

func TestRequestBodyReaderIsBounded(t *testing.T) {
	t.Parallel()
	handler := limitRequestBody(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			writeProblem(w, r, payloadProblem)
			return
		}
		writePlain(w, http.StatusOK, "ok\n")
	}))
	request := httptest.NewRequest(http.MethodPost, "http://stackplore.test/value", strings.NewReader("12345"))
	request = request.WithContext(context.WithValue(request.Context(), requestIDContextKey{}, "req_body"))
	response := serve(handler, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestAPILargeBodyIsRejectedBeforeHandler(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeHandler(t, nil)
	request := httptest.NewRequest(
		http.MethodGet,
		"http://stackplore.test/api/v1/system/readiness",
		strings.NewReader(strings.Repeat("x", 1025)),
	)
	request.RemoteAddr = "198.51.100.10:4000"
	request.Header.Set(requestIDHeader, "req_large_body")
	response := serve(handler, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
	if got := decodeProblem(t, response); got.Code != payloadProblem.code || got.RequestID != "req_large_body" {
		t.Errorf("problem = %#v", got)
	}
}
