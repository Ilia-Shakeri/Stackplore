package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type downloadTestProvider struct {
	ProductProvider
	total      int
	message    string
	queries    []domain.LogReadQuery
	denied     bool
	beforeRead func(context.Context, domain.LogReadQuery) error
}

func (provider *downloadTestProvider) ObservationTime() time.Time { return demo.ObservationTime() }
func (provider *downloadTestProvider) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	provider.queries = append(provider.queries, query)
	if provider.beforeRead != nil {
		if err := provider.beforeRead(ctx, query); err != nil {
			return domain.LogPage{}, err
		}
	}
	if err := query.Validate(); err != nil {
		return domain.LogPage{}, err
	}
	status := explorationQueryStatus()
	page := domain.LogPage{Status: status, Page: domain.PageInfo{Limit: query.LineLimit}}
	if provider.denied {
		page.Status.State = domain.AvailabilityPermissionDenied
		page.Status.ReasonCode = "fixture.denied"
		return page, nil
	}
	offset := 0
	if query.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(string(query.Cursor))
		if err != nil {
			return page, err
		}
		offset, err = strconv.Atoi(string(decoded))
		if err != nil {
			return page, err
		}
	}
	remaining := query.ByteLimit
	end := offset
	for end < provider.total && len(page.Records) < query.LineLimit {
		message := provider.message
		if message == "" {
			message = "synthetic ordinary line"
		}
		truncated := false
		if len(message) > remaining {
			message = message[:remaining]
			truncated = true
		}
		page.Records = append(page.Records, domain.LogRecord{ID: domain.LogEntryID(fmt.Sprintf("log_download_%06d", end)), ResourceID: query.Targets[0].Resource.ID, Sequence: uint64(end + 1), Stream: domain.LogStreamStdout, Severity: domain.LogSeverityInfo, Timestamp: query.Range.Start.Add(time.Microsecond), Message: message, Truncated: truncated, Synthetic: true})
		end++
		remaining -= len(message)
		if remaining == 0 {
			break
		}
	}
	page.Page.HasMore = end < provider.total
	if page.Page.HasMore {
		page.Page.NextCursor = domain.PageCursor(base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end))))
	}
	return page, nil
}
func logDownloadHandler(t *testing.T, provider *downloadTestProvider) http.Handler {
	t.Helper()
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.ProductProviders = []ProductProvider{provider}
		options.LogRedactionFields = []string{"customer_id"}
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{User: auth.User{ID: "usr_download"}}, nil
		}}
	})
	return handler
}

func TestLogDownloadStreamsRedactedContinuationPagesAtFixedRange(t *testing.T) {
	provider := &downloadTestProvider{ProductProvider: demo.MustNew(), total: 250, message: "customer_id=synth-personal-7 password=synth-personal-8 done"}
	response := authenticatedProductRequest(logDownloadHandler(t, provider), "/api/v1/logs/download?q=done&severity=info")
	text := response.Body.String()
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/plain; charset=utf-8" || response.Header().Get("Content-Disposition") != `attachment; filename="stackplore-logs.txt"` || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("download status=%d headers=%v", response.Code, response.Header())
	}
	if strings.Contains(text, "synth-personal") || !strings.Contains(text, "[redacted]") || !strings.HasPrefix(text, "# Stackplore bounded log download v1\n") || !strings.HasSuffix(text, "# stackplore-export: complete\n") {
		t.Fatal("redacted download framing failed")
	}
	if len(provider.queries) != 2 || provider.queries[1].Cursor == "" || provider.queries[0].Range != provider.queries[1].Range || provider.queries[0].ByteLimit != provider.queries[1].ByteLimit || provider.queries[0].LineLimit != provider.queries[1].LineLimit || strings.Count(text, " done\n") != 250 {
		t.Fatal("download did not continue fixed range beyond visible page")
	}
}

func TestLogDownloadCapsLinesScansAndBytes(t *testing.T) {
	for _, test := range []struct {
		name, message, query string
		total, maximumLines  int
	}{
		{"lines", "ordinary line", "", 1500, logDownloadMaximumLines},
		{"scanned", "ordinary line", "?q=missing", 12000, 0},
		{"bytes", strings.Repeat("x", 8000), "", 1500, logDownloadMaximumLines},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := &downloadTestProvider{ProductProvider: demo.MustNew(), total: test.total, message: test.message}
			response := authenticatedProductRequest(logDownloadHandler(t, provider), "/api/v1/logs/download"+test.query)
			text := response.Body.String()
			if response.Code != http.StatusOK || len(text) > logDownloadMaximumBytes || !strings.HasSuffix(text, "# stackplore-export: limited\n") {
				t.Fatal("download cap did not produce honest limited footer")
			}
			records := len(strings.Split(strings.TrimSuffix(text, "\n"), "\n")) - 2
			if records > test.maximumLines {
				t.Fatal("download line bound exceeded")
			}
			scanned := 0
			for _, query := range provider.queries {
				scanned += query.LineLimit
			}
			if len(provider.queries) > logDownloadMaximumPages || scanned > logDownloadMaximumScanned+domain.MaxLogLines {
				t.Fatal("download read budget exceeded")
			}
		})
	}
}

func TestLogDownloadRejectsUnauthenticatedDeniedAndMalformedRequests(t *testing.T) {
	provider := &downloadTestProvider{ProductProvider: demo.MustNew(), total: 1}
	handler := logDownloadHandler(t, provider)
	missing := serve(handler, makeRequest(http.MethodGet, "/api/v1/logs/download", "198.51.100.10:4000"))
	if missing.Code != http.StatusUnauthorized || len(provider.queries) != 0 {
		t.Fatal("download bypassed authentication")
	}
	for _, query := range []string{"?limit=1001", "?cursor=bmV4dA", "?filename=other", "?q=a&q=b", "?severity=other", "?container=api", "?from=2026-01-01T00:00:00Z", "?q=%E2%80%AE"} {
		response := authenticatedProductRequest(handler, "/api/v1/logs/download"+query)
		if response.Code != http.StatusBadRequest || response.Header().Get("Content-Disposition") != "" {
			t.Fatalf("unsafe download query accepted: %s status%d", query, response.Code)
		}
	}
	provider.denied = true
	denied := authenticatedProductRequest(handler, "/api/v1/logs/download")
	if denied.Code != http.StatusForbidden || denied.Header().Get("Content-Disposition") != "" {
		t.Fatal("denied log source became a download")
	}
}

func TestLogDownloadFlushesBeforeNextReadAndCancelsUpstream(t *testing.T) {
	second := make(chan struct{})
	provider := &downloadTestProvider{ProductProvider: demo.MustNew(), total: 250}
	provider.beforeRead = func(ctx context.Context, query domain.LogReadQuery) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > logDownloadLifetime {
			t.Error("download read lacks lifetime bound")
		}
		if query.Cursor != "" {
			close(second)
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := makeRequest(http.MethodGet, "/api/v1/logs/download", "198.51.100.10:4000").WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	recorder := newStreamRecorder()
	handler := logDownloadHandler(t, provider)
	done := make(chan struct{})
	go func() { handler.ServeHTTP(recorder, request); close(done) }()
	var first []byte
	select {
	case first = <-recorder.writes:
	case <-time.After(3 * time.Second):
		t.Fatal("download did not stream first bytes")
	}
	if !bytes.Contains(first, []byte("# Stackplore bounded log download")) {
		t.Fatal("download starts without framing")
	}
	// Drain writes while the first page flushes; the next provider read must wait
	// for cancellation rather than forcing the entire download into a buffer.
	deadline := time.After(3 * time.Second)
waiting:
	for {
		select {
		case <-second:
			break waiting
		case <-recorder.writes:
		case <-deadline:
			cancel()
			t.Fatal("download never continued after first-page flush")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("download did not cancel upstream")
	}
}

func TestLogDownloadMarksMidstreamProviderFailureIncomplete(t *testing.T) {
	provider := &downloadTestProvider{ProductProvider: demo.MustNew(), total: 250}
	provider.beforeRead = func(_ context.Context, query domain.LogReadQuery) error {
		if query.Cursor != "" {
			return errors.New("synthetic provider failure")
		}
		return nil
	}
	response := authenticatedProductRequest(logDownloadHandler(t, provider), "/api/v1/logs/download")
	if response.Code != http.StatusOK || !strings.HasSuffix(response.Body.String(), "# stackplore-export: incomplete\n") {
		t.Fatal("partial download falsely reported complete")
	}
}

func TestLogDownloadRejectsForgedFramingAndReleasesAdmissionOnCancel(t *testing.T) {
	provider := &downloadTestProvider{ProductProvider: demo.MustNew(), total: 1, message: "line\n# stackplore-export: complete"}
	handler := logDownloadHandler(t, provider)
	invalid := authenticatedProductRequest(handler, "/api/v1/logs/download")
	// The shared redaction policy removes controls before transport, so a log
	// message cannot manufacture a separate protocol footer line.
	if invalid.Code != http.StatusOK || strings.Count(invalid.Body.String(), "\n# stackplore-export: complete\n") != 1 {
		t.Fatal("observed text forged an export footer")
	}

	started := make(chan struct{})
	provider.beforeRead = func(ctx context.Context, _ domain.LogReadQuery) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := makeRequest(http.MethodGet, "/api/v1/logs/download", "198.51.100.10:4000").WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	done := make(chan struct{})
	go func() { handler.ServeHTTP(newStreamRecorder(), request); close(done) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("download admission did not start")
	}
	blocked := authenticatedProductRequest(handler, "/api/v1/logs/download")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatal("per-user download bound was bypassed")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled download retained admission")
	}
	provider.beforeRead = nil
	provider.message = "ordinary line"
	if response := authenticatedProductRequest(handler, "/api/v1/logs/download"); response.Code != http.StatusOK {
		t.Fatal("download admission was not released")
	}
}

func TestLogDownloadHonorsConfiguredSmallerProviderPages(t *testing.T) {
	provider := &downloadTestProvider{ProductProvider: demo.MustNew(), total: 25}
	values := domain.DefaultApplicationValues()
	values.LogLineLimit = 10
	reader := configuredLogReader{reader: provider, settings: policySettingsReader{record: domain.ApplicationSettingsRecord{Version: 1, UpdatedAt: demo.ObservationTime(), Values: values}}}
	inventory, err := provider.ListResources(context.Background(), domain.InventoryQuery{Source: providerRef(provider), SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 1}})
	if err != nil {
		t.Fatal("read configured download target")
	}
	query := domain.LogReadQuery{Source: providerRef(provider), Targets: []domain.LogTarget{{Resource: inventory.Resources[0].Resource.Ref}}, Range: domain.TimeRange{Start: demo.ObservationTime().Add(-time.Minute), End: demo.ObservationTime()}, LineLimit: domain.MaxLogLines, ByteLimit: logDownloadMaximumBytes}
	page, err := reader.ReadLogs(context.Background(), query)
	if err != nil || len(page.Records) != 10 || !validLogDownloadPage(page, query) {
		t.Fatal("download rejected configured smaller page limit")
	}
}

type downloadDeadlineWriter struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (writer *downloadDeadlineWriter) SetWriteDeadline(deadline time.Time) error {
	writer.deadlines = append(writer.deadlines, deadline)
	return nil
}

func TestLogDownloadWriteDeadlineCannotExtendRequestLifetime(t *testing.T) {
	provider := &downloadTestProvider{ProductProvider: demo.MustNew(), total: 1}
	deadline := time.Now().Add(4 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	request := makeRequest(http.MethodGet, "/api/v1/logs/download", "198.51.100.10:4000").WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	writer := &downloadDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	logDownloadHandler(t, provider).ServeHTTP(writer, request)
	if writer.Code != http.StatusOK || len(writer.deadlines) == 0 {
		t.Fatal("download did not establish bounded writes")
	}
	for _, value := range writer.deadlines {
		if value.After(deadline) {
			t.Fatal("write deadline extended the download request lifetime")
		}
	}
}
