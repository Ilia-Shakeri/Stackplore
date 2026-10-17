package docker

import (
	"context"
	"errors"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"io"
	"net/http"
	"strings"
)

type engineWriteTransport struct{ read *engineReadTransport }

func exactDockerContainerID(id string) bool {
	if len(id) != 64 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func (w *engineWriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if w == nil || w.read == nil || w.read.base == nil || r == nil || r.URL == nil || r.URL.Scheme != w.read.base.Scheme || r.URL.Host != w.read.base.Host || r.Host != "" && r.Host != w.read.base.Host || r.URL.User != nil || r.URL.Fragment != "" || r.URL.RawPath != "" || r.URL.ForceQuery || strings.Contains(r.URL.Path, "%") || r.ContentLength > 0 || r.Body != nil && r.Body != http.NoBody || len(r.TransferEncoding) > 0 || r.Header.Get("Upgrade") != "" || strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") || r.Header.Get("Idempotency-Key") != "" || r.Header.Get("X-Idempotency-Key") != "" {
		return nil, ErrPermissionDenied
	}
	path := r.URL.Path
	if match := engineVersionPath.FindStringSubmatch(path); match != nil {
		path = match[2]
	}
	if path == "/_ping" && (r.Method == http.MethodHead || r.Method == http.MethodGet) && r.URL.RawQuery == "" {
		return w.read.RoundTrip(r)
	}
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[1] != "containers" || !exactDockerContainerID(parts[2]) {
		return nil, ErrPermissionDenied
	}
	if r.Method == http.MethodGet && parts[3] == "json" && r.URL.RawQuery == "" {
		return w.read.RoundTrip(r)
	}
	if r.Method != http.MethodPost || parts[3] != "restart" || r.URL.RawQuery != "signal=SIGTERM&t=10" {
		return nil, ErrPermissionDenied
	}
	response, err := w.read.next.RoundTrip(r)
	if err != nil {
		return nil, errors.Join(ErrUnavailable, domain.ErrOperationOutcomeUncertain)
	}
	if response == nil || response.Body == nil {
		return nil, errors.Join(ErrUnavailable, domain.ErrOperationOutcomeUncertain)
	}
	// Never expose a daemon error body or replay an uncertain POST.
	switch response.StatusCode {
	case http.StatusNoContent:
		if response.Header.Get("Content-Encoding") != "" && response.Header.Get("Content-Encoding") != "identity" {
			_ = response.Body.Close()
			return nil, domain.ErrOperationOutcomeUncertain
		}
		body := &engineContextBody{ReadCloser: response.Body}
		body.stop = context.AfterFunc(r.Context(), func() { _ = body.closeSource() })
		response.Body = &engineLimitedBody{ReadCloser: body, remaining: 1024}
		return response, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		_ = response.Body.Close()
		return nil, ErrPermissionDenied
	case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict:
		_ = response.Body.Close()
		return nil, ErrVersionConflict
	default:
		_ = response.Body.Close()
		return nil, errors.Join(ErrUnavailable, domain.ErrOperationOutcomeUncertain)
	}
}
func (w *engineWriteTransport) CloseIdleConnections() { w.read.CloseIdleConnections() }

var _ io.Closer = (*SDKWriter)(nil)
