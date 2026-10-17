package kubernetes

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

type boundedTransport struct {
	base          http.RoundTripper
	tokens        *bearerSource
	namespaces    []string
	clusterScoped bool
	maximum       int64
	writer        bool
}

func (transport *boundedTransport) CloseIdleConnections() {
	if closer, ok := transport.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func (transport *boundedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme != "https" || request.URL.Host != "kubernetes.default.svc" ||
		request.URL.User != nil || request.URL.RawPath != "" || !transport.permits(request.Method, request.URL.Path) || transport.writer && !validWriterQuery(request) {
		return nil, ErrPermissionDenied
	}
	budget, _ := request.Context().Value(observationBudgetKey{}).(*atomic.Int64)
	if budget != nil && budget.Load() <= 0 {
		return nil, ErrTruncated
	}
	copy := request.Clone(request.Context())
	copy.Header = request.Header.Clone()
	token, err := transport.tokens.current(request.Context())
	if err != nil {
		return nil, err
	}
	copy.Header.Set("Authorization", "Bearer "+token)
	copy.Header.Set("Accept-Encoding", "identity")
	response, err := transport.base.RoundTrip(copy)
	if err != nil {
		return nil, ErrUnavailable
	}
	if response == nil || response.Body == nil {
		return nil, ErrUnavailable
	}
	if response.StatusCode == http.StatusUnauthorized {
		transport.tokens.invalidate(token)
		if request.Method == http.MethodGet && transport.tokens.read != nil {
			response.Body.Close()
			refreshed, refreshErr := transport.tokens.current(request.Context())
			if refreshErr != nil {
				return nil, refreshErr
			}
			if refreshed == token {
				return nil, ErrPermissionDenied
			}
			copy.Header.Set("Authorization", "Bearer "+refreshed)
			response, err = transport.base.RoundTrip(copy)
			if err != nil || response == nil || response.Body == nil {
				return nil, ErrUnavailable
			}
			if response.StatusCode == http.StatusUnauthorized {
				transport.tokens.invalidate(refreshed)
			}
		}
	}
	if response.ContentLength > transport.maximum || response.StatusCode == http.StatusTooManyRequests ||
		response.StatusCode >= 300 && response.StatusCode < 400 ||
		response.Header.Get("Content-Encoding") != "" && response.Header.Get("Content-Encoding") != "identity" {
		response.Body.Close()
		return nil, ErrUnavailable
	}
	response.Body = &boundedBody{ReadCloser: response.Body, remaining: transport.maximum, budget: budget}
	return response, nil
}

func validWriterQuery(request *http.Request) bool {
	if request.URL.RawQuery == "" {
		return true
	}
	query := request.URL.Query()
	values := query["timeout"]
	if len(query) != 1 || len(values) != 1 {
		return false
	}
	duration, err := time.ParseDuration(values[0])
	return err == nil && duration > 0 && duration <= 15*time.Second
}

// Proxy, exec, Secret and ConfigMap object endpoints never pass this guard.
func (transport *boundedTransport) permits(method, path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if strings.Contains(path, "//") || strings.Contains(path, "%") {
		return false
	}
	if transport.writer {
		return (method == http.MethodGet || method == http.MethodPatch) && len(parts) == 7 &&
			parts[0] == "apis" && parts[1] == "apps" && parts[2] == "v1" && parts[3] == "namespaces" &&
			slices.Contains(transport.namespaces, parts[4]) && slices.Contains([]string{"deployments", "statefulsets", "daemonsets"}, parts[5]) && safeKubernetesPathSegment(parts[6])
	}
	if method == http.MethodGet && transport.clusterScoped && len(parts) == 3 && path == "/api/v1/nodes" {
		return true
	}
	if method == http.MethodGet && transport.clusterScoped && len(parts) == 4 &&
		parts[0] == "api" && parts[1] == "v1" && parts[2] == "namespaces" && slices.Contains(transport.namespaces, parts[3]) {
		return true
	}
	if len(parts) < 5 {
		return false
	}
	var namespace, resource string
	var rest []string
	if parts[0] == "api" && parts[1] == "v1" && parts[2] == "namespaces" {
		namespace, resource, rest = parts[3], parts[4], parts[5:]
		if !slices.Contains(transport.namespaces, namespace) || method != http.MethodGet {
			return false
		}
		if resource == "pods" && len(rest) == 2 && rest[1] == "log" {
			return safeKubernetesPathSegment(rest[0])
		}
		if resource == "pods" && len(rest) == 1 {
			return safeKubernetesPathSegment(rest[0])
		}
		return len(rest) == 0 && slices.Contains([]string{"pods", "services", "persistentvolumeclaims", "events"}, resource)
	}
	if len(parts) < 6 || parts[0] != "apis" || parts[3] != "namespaces" {
		return false
	}
	namespace, resource, rest = parts[4], parts[5], parts[6:]
	if !slices.Contains(transport.namespaces, namespace) {
		return false
	}
	allowed := false
	switch parts[1] + "/" + parts[2] {
	case "apps/v1":
		allowed = slices.Contains([]string{"deployments", "statefulsets", "daemonsets", "replicasets"}, resource)
	case "batch/v1":
		allowed = resource == "jobs" || resource == "cronjobs"
	case "networking.k8s.io/v1":
		allowed = resource == "ingresses"
	case "discovery.k8s.io/v1":
		allowed = resource == "endpointslices"
	case "metrics.k8s.io/v1beta1":
		allowed = resource == "pods"
	}
	if !allowed {
		return false
	}
	if method == http.MethodGet {
		return len(rest) == 0
	}
	return false
}

func safeOperationPath(resource string, rest []string) bool {
	if len(rest) < 1 || len(rest) > 2 || !safeKubernetesPathSegment(rest[0]) {
		return false
	}
	if len(rest) == 1 {
		return resource == "deployments" || resource == "statefulsets" || resource == "daemonsets"
	}
	return rest[1] == "scale" && (resource == "deployments" || resource == "statefulsets")
}

type boundedBody struct {
	io.ReadCloser
	remaining int64
	budget    *atomic.Int64
}

func (body *boundedBody) Read(buffer []byte) (int, error) {
	if body.remaining < 0 {
		return 0, ErrTruncated
	}
	if int64(len(buffer)) > body.remaining+1 {
		buffer = buffer[:body.remaining+1]
	}
	if body.budget != nil {
		remaining := body.budget.Load()
		if remaining < 0 {
			return 0, ErrTruncated
		}
		if int64(len(buffer)) > remaining+1 {
			buffer = buffer[:remaining+1]
		}
	}
	n, err := body.ReadCloser.Read(buffer)
	body.remaining -= int64(n)
	if body.budget != nil && body.budget.Add(-int64(n)) < 0 {
		return n, ErrTruncated
	}
	if body.remaining < 0 {
		return n, ErrTruncated
	}
	return n, err
}
