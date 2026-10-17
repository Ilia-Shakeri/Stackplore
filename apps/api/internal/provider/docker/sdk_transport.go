package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

var errEngineResponseLimit = errors.New("Docker response limit reached")

const maximumEngineObservationBytes = int64(32 << 20)

type engineObservationBudgetKey struct{}

func engineObservationContext(ctx context.Context) context.Context {
	budget := new(atomic.Int64)
	budget.Store(maximumEngineObservationBytes)
	return context.WithValue(ctx, engineObservationBudgetKey{}, budget)
}

var engineVersionPath = regexp.MustCompile(`^/v1\.(4[0-9]|5[0-6])(/.*)$`)

// engineReadTransport keeps the SDK on the reviewed observation surface.
// Its origin comes only from the operator-owned transport configuration.
type engineReadTransport struct {
	base    *url.URL
	next    http.RoundTripper
	maximum int64
}

func (transport *engineReadTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	path, allowed := reviewedEngineRequest(request, transport.base)
	if !allowed {
		return nil, ErrPermissionDenied
	}
	budget, _ := request.Context().Value(engineObservationBudgetKey{}).(*atomic.Int64)
	if budget != nil && budget.Load() <= 0 {
		return nil, errEngineResponseLimit
	}
	response, err := transport.next.RoundTrip(request)
	if err != nil {
		return nil, ErrUnavailable
	}
	if response == nil || response.Body == nil {
		return nil, ErrUnavailable
	}
	originalBody := response.Body
	contextBody := &engineContextBody{ReadCloser: originalBody}
	contextBody.stop = context.AfterFunc(request.Context(), func() { _ = contextBody.closeSource() })
	response.Body = &engineLimitedBody{ReadCloser: contextBody, remaining: transport.maximum, budget: budget}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		_ = response.Body.Close()
		return nil, ErrPermissionDenied
	}
	if response.StatusCode >= 300 && response.StatusCode < 400 ||
		response.Header.Get("Content-Encoding") != "" && response.Header.Get("Content-Encoding") != "identity" {
		_ = response.Body.Close()
		return nil, ErrUnavailable
	}
	if request.Method == http.MethodHead || path == "/_ping" {
		response.Body = &engineLimitedBody{ReadCloser: response.Body, remaining: 1024}
		return response, nil
	}
	if path == "/events" || strings.HasSuffix(path, "/logs") {
		response.Body = &engineLimitedBody{ReadCloser: response.Body, remaining: transport.maximum}
		return response, nil
	}
	// SDK JSON methods decode a value without checking trailing input. Bound and
	// validate the complete document before the vendor decoder allocates models.
	data, readErr := io.ReadAll(io.LimitReader(response.Body, transport.maximum+1))
	_ = response.Body.Close()
	if readErr != nil || int64(len(data)) > transport.maximum {
		return nil, errEngineResponseLimit
	}
	if !json.Valid(data) {
		return nil, ErrUnavailable
	}
	response.Body = io.NopCloser(bytes.NewReader(data))
	return response, nil
}

func (transport *engineReadTransport) CloseIdleConnections() {
	if closer, ok := transport.next.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

type engineLimitedBody struct {
	io.ReadCloser
	remaining int64
	budget    *atomic.Int64
}

type engineContextBody struct {
	io.ReadCloser
	stop     func() bool
	once     sync.Once
	closeErr error
}

func (body *engineContextBody) Close() error { body.stop(); return body.closeSource() }
func (body *engineContextBody) closeSource() error {
	body.once.Do(func() { body.closeErr = body.ReadCloser.Close() })
	return body.closeErr
}

func (body *engineLimitedBody) Read(buffer []byte) (int, error) {
	if body.remaining < 0 {
		return 0, errEngineResponseLimit
	}
	if int64(len(buffer)) > body.remaining+1 {
		buffer = buffer[:body.remaining+1]
	}
	reserved := int64(0)
	if body.budget != nil {
		for {
			available := body.budget.Load()
			if available <= 0 {
				_ = body.ReadCloser.Close()
				return 0, errEngineResponseLimit
			}
			reserved = min(available, int64(len(buffer)))
			if body.budget.CompareAndSwap(available, available-reserved) {
				buffer = buffer[:reserved]
				break
			}
		}
	}
	n, err := body.ReadCloser.Read(buffer)
	if body.budget != nil {
		body.budget.Add(reserved - int64(n))
	}
	body.remaining -= int64(n)
	if body.remaining < 0 {
		_ = body.ReadCloser.Close()
		return n, errEngineResponseLimit
	}
	return n, err
}

func reviewedEngineRequest(request *http.Request, base *url.URL) (string, bool) {
	if request == nil || request.URL == nil || base == nil || request.URL.Scheme != base.Scheme || request.URL.Host != base.Host ||
		request.Host != "" && request.Host != base.Host || request.ContentLength > 0 || len(request.TransferEncoding) > 0 ||
		request.URL.User != nil || request.URL.Fragment != "" || request.URL.RawPath != "" || strings.Contains(request.URL.Path, "%") ||
		request.Body != nil && request.Body != http.NoBody || request.Header.Get("Upgrade") != "" ||
		strings.Contains(strings.ToLower(request.Header.Get("Connection")), "upgrade") {
		return "", false
	}
	path := request.URL.Path
	if match := engineVersionPath.FindStringSubmatch(path); match != nil {
		path = match[2]
	}
	if path == "/_ping" {
		return path, (request.Method == http.MethodGet || request.Method == http.MethodHead) && request.URL.RawQuery == ""
	}
	if request.Method != http.MethodGet {
		return "", false
	}
	values, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return "", false
	}
	for _, items := range values {
		if len(items) != 1 {
			return "", false
		}
	}
	switch path {
	case "/info", "/version", "/images/json", "/networks", "/volumes":
		return path, len(values) == 0
	case "/containers/json":
		return path, len(values) == 1 && values.Get("all") == "1"
	case "/events":
		return path, len(values) == 2 && values.Get("since") == "0" && engineTimestamp(values.Get("until"))
	}
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[1] != "containers" || !safeDockerID(parts[2]) {
		return "", false
	}
	switch parts[3] {
	case "json":
		return path, len(values) == 0
	case "stats":
		return path, len(values) == 1 && values.Get("stream") == "false"
	case "logs":
		tail, tailErr := strconv.Atoi(values.Get("tail"))
		return path, len(values) == 6 && values.Get("stdout") == "1" && values.Get("stderr") == "1" &&
			values.Get("timestamps") == "1" && values.Get("follow") == "" && values.Get("details") == "" &&
			engineTimestamp(values.Get("since")) && engineTimestamp(values.Get("until")) && tailErr == nil && tail > 0 && tail <= domain.MaxLogLines
	}
	return "", false
}

func engineTimestamp(value string) bool {
	if len(value) == 0 || len(value) > 32 {
		return false
	}
	seconds, fraction, _ := strings.Cut(value, ".")
	parsed, err := strconv.ParseInt(seconds, 10, 64)
	if err != nil || parsed < 0 || len(fraction) > 9 {
		return false
	}
	for _, digit := range fraction {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
