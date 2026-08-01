package httpapi

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

const metricsContentType = "text/plain; version=0.0.4; charset=utf-8"

var routeLabels = [...]string{"health", "readiness", "metrics", "api_readiness", "api_other", "other"}
var methodLabels = [...]string{"get", "head", "other"}
var statusLabels = [...]string{"2xx", "3xx", "4xx", "5xx"}

type Metrics struct {
	requests          [6][3][4]atomic.Uint64
	inFlight          atomic.Int64
	panics            atomic.Uint64
	timeouts          atomic.Uint64
	rateLimited       atomic.Uint64
	overloaded        atomic.Uint64
	requestIDFailures atomic.Uint64
	readiness         *Readiness
}

func NewMetrics(readiness *Readiness) *Metrics {
	return &Metrics{readiness: readiness}
}

func (metrics *Metrics) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.inFlight.Add(1)
		defer metrics.inFlight.Add(-1)
		wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrapped, r)
		status := wrapped.Status()
		if status == 0 {
			status = http.StatusOK
		}
		metrics.requests[routeIndex(r.URL.Path)][methodIndex(r.Method)][statusIndex(status)].Add(1)
	})
}

func (metrics *Metrics) recordPanic() {
	metrics.panics.Add(1)
}

func (metrics *Metrics) recordTimeout() {
	metrics.timeouts.Add(1)
}

func (metrics *Metrics) recordRateLimit() {
	metrics.rateLimited.Add(1)
}

func (metrics *Metrics) recordOverload() {
	metrics.overloaded.Add(1)
}

func (metrics *Metrics) recordRequestIDFailure() {
	metrics.requestIDFailures.Add(1)
}

func (metrics *Metrics) serveHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", metricsContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "# HELP stackplore_build_info Static build identity.")
	_, _ = fmt.Fprintln(w, "# TYPE stackplore_build_info gauge")
	_, _ = fmt.Fprintln(w, `stackplore_build_info{version="0.0.0"} 1`)
	_, _ = fmt.Fprintln(w, "# HELP stackplore_ready Whether the local service is ready.")
	_, _ = fmt.Fprintln(w, "# TYPE stackplore_ready gauge")
	ready := 0
	if metrics.readiness.IsReady() {
		ready = 1
	}
	_, _ = fmt.Fprintf(w, "stackplore_ready %d\n", ready)
	_, _ = fmt.Fprintln(w, "# HELP stackplore_http_in_flight_requests Requests currently running.")
	_, _ = fmt.Fprintln(w, "# TYPE stackplore_http_in_flight_requests gauge")
	_, _ = fmt.Fprintf(w, "stackplore_http_in_flight_requests %d\n", metrics.inFlight.Load())
	_, _ = fmt.Fprintln(w, "# HELP stackplore_http_requests_total Completed HTTP requests.")
	_, _ = fmt.Fprintln(w, "# TYPE stackplore_http_requests_total counter")
	for route := range routeLabels {
		for method := range methodLabels {
			for status := range statusLabels {
				value := metrics.requests[route][method][status].Load()
				if value == 0 {
					continue
				}
				_, _ = fmt.Fprintf(
					w,
					"stackplore_http_requests_total{method=%q,route=%q,status=%q} %d\n",
					methodLabels[method],
					routeLabels[route],
					statusLabels[status],
					value,
				)
			}
		}
	}
	_, _ = fmt.Fprintln(w, "# HELP stackplore_http_panics_total Recovered request panics.")
	_, _ = fmt.Fprintln(w, "# TYPE stackplore_http_panics_total counter")
	_, _ = fmt.Fprintf(w, "stackplore_http_panics_total %d\n", metrics.panics.Load())
	_, _ = fmt.Fprintln(w, "# HELP stackplore_http_timeouts_total Requests stopped at the time limit.")
	_, _ = fmt.Fprintln(w, "# TYPE stackplore_http_timeouts_total counter")
	_, _ = fmt.Fprintf(w, "stackplore_http_timeouts_total %d\n", metrics.timeouts.Load())
	_, _ = fmt.Fprintln(w, "# HELP stackplore_http_rate_limited_total Requests denied by the rate limit.")
	_, _ = fmt.Fprintln(w, "# TYPE stackplore_http_rate_limited_total counter")
	_, _ = fmt.Fprintf(w, "stackplore_http_rate_limited_total %d\n", metrics.rateLimited.Load())
	_, _ = fmt.Fprintln(w, "# HELP stackplore_http_overload_total Requests denied at the in-flight bound.")
	_, _ = fmt.Fprintln(w, "# TYPE stackplore_http_overload_total counter")
	_, _ = fmt.Fprintf(w, "stackplore_http_overload_total %d\n", metrics.overloaded.Load())
	_, _ = fmt.Fprintln(w, "# HELP stackplore_http_request_id_failures_total Requests denied because correlation ID generation failed.")
	_, _ = fmt.Fprintln(w, "# TYPE stackplore_http_request_id_failures_total counter")
	_, _ = fmt.Fprintf(w, "stackplore_http_request_id_failures_total %d\n", metrics.requestIDFailures.Load())
}

func requireMetricsAccess(policy proxyPolicy, networks []netip.Prefix, next http.Handler) http.Handler {
	allowedNetworks := append([]netip.Prefix(nil), networks...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := networkInfoFromContext(r.Context())
		allowed := info.err == nil &&
			!info.untrustedForwarding &&
			info.clientVerified &&
			info.scheme == policy.origin.Scheme &&
			info.authority == policy.origin.Authority
		if allowed {
			allowed = false
			for _, prefix := range allowedNetworks {
				if prefix.Contains(info.clientIP) {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			writePlain(w, http.StatusNotFound, "not found\n")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func routeIndex(path string) int {
	switch path {
	case "/healthz":
		return 0
	case "/readyz":
		return 1
	case "/metrics":
		return 2
	case "/api/v1/system/readiness":
		return 3
	default:
		if path == "/api/v1" || strings.HasPrefix(path, "/api/v1/") {
			return 4
		}
		return 5
	}
}

func routeLabel(path string) string {
	return routeLabels[routeIndex(path)]
}

func methodIndex(method string) int {
	switch method {
	case http.MethodGet:
		return 0
	case http.MethodHead:
		return 1
	default:
		return 2
	}
}

func methodLabel(method string) string {
	return methodLabels[methodIndex(method)]
}

func statusIndex(status int) int {
	switch {
	case status >= 200 && status < 300:
		return 0
	case status >= 300 && status < 400:
		return 1
	case status >= 400 && status < 500:
		return 2
	default:
		return 3
	}
}
