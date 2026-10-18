package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

// retainedEventReader returns stored rows without passing through a provider sanitizer.
type retainedEventReader struct {
	mu     sync.Mutex
	events []domain.InfrastructureEvent
}

func (reader *retainedEventReader) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	if err := ctx.Err(); err != nil {
		return domain.EventPage{}, err
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	page := domain.EventPage{Status: explorationQueryStatus(), Events: reader.events, Page: domain.PageInfo{Limit: query.Page.Limit}}
	return page, page.ValidateFor(query)
}

type retainedEventProvider struct {
	ProductProvider
	domain.EventHistoryProvider
}

func (retainedEventProvider) ObservationTime() time.Time { return demo.ObservationTime() }

func retainedEventMarker(t *testing.T) string {
	t.Helper()
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		t.Fatal("event fixture entropy unavailable")
	}
	value := base64.RawURLEncoding.EncodeToString(entropy[:])
	clear(entropy[:])
	return value
}

func retainedEventHandler(t *testing.T, summary string, additionalFields []string) (http.Handler, *retainedEventReader, domain.InfrastructureEvent) {
	t.Helper()
	fixture := demo.MustNew()
	page, err := fixture.ListEvents(context.Background(), domain.EventQuery{
		Source: providerRef(fixture), Range: domain.TimeRange{Start: demo.ObservationTime().Add(-24 * time.Hour), End: demo.ObservationTime().Add(time.Nanosecond)},
		Page: domain.PageRequest{Limit: 1},
	})
	if err != nil || len(page.Events) != 1 {
		t.Fatal("retained event fixture unavailable")
	}
	event := page.Events[0]
	event.Summary = summary
	if event.Validate() != nil {
		t.Fatal("retained event fixture violates domain bounds")
	}
	reader := &retainedEventReader{events: []domain.InfrastructureEvent{event}}
	provider := retainedEventProvider{ProductProvider: fixture, EventHistoryProvider: reader}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.ProductProviders = []ProductProvider{provider}
		options.LogRedactionFields = additionalFields
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{User: auth.User{ID: "usr_retained_events"}}, nil
		}}
	})
	return handler, reader, event
}

func startRetainedEventStream(t *testing.T, handler http.Handler) (<-chan []byte, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	request := makeRequest(http.MethodGet, "/api/v1/streams/events", "198.51.100.10:4000").WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: retainedEventMarker(t)})
	recorder := newStreamRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(recorder, request)
	}()
	stop := sync.OnceFunc(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("retained event stream did not stop")
		}
	})
	t.Cleanup(stop)
	return recorder.writes, stop
}

func readRetainedEventFrame(t *testing.T, writes <-chan []byte) []eventResponse {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case chunk := <-writes:
			for _, line := range bytes.Split(chunk, []byte("\n")) {
				data, ok := bytes.CutPrefix(line, []byte("data: "))
				if !ok {
					continue
				}
				var envelope struct {
					Topic streamTopic     `json:"topic"`
					Data  []eventResponse `json:"data"`
				}
				if json.Unmarshal(data, &envelope) != nil || envelope.Topic != streamEvents || len(envelope.Data) != 1 {
					t.Fatal("retained event stream frame malformed")
				}
				return envelope.Data
			}
		case <-deadline.C:
			t.Fatal("retained event stream frame unavailable")
		}
	}
}

func TestRetainedEventSummariesAreRedactedAtHTTPBoundaries(t *testing.T) {
	marker := retainedEventMarker(t)
	tests := []struct {
		name        string
		summary     string
		want        string
		fields      []string
		masked      bool
		atRuneLimit bool
	}{
		{name: "URL userinfo", summary: "legacy https://" + marker + ":x@example.test/path", want: "legacy https://[redacted]@example.test/path", masked: true},
		{name: "serialized URL", summary: `legacy https:\/\/` + marker + `:x@example.test/path`, want: `legacy https:\/\/[redacted]@example.test/path`, masked: true},
		{name: "structured URL", summary: `{"endpoint":"https://` + marker + `:x@example.test/path"}`, want: `{"endpoint":"https://[redacted]@example.test/path"}`, masked: true},
		{name: "deployment field", summary: "private_tag=" + marker + " done", want: "private_tag=[redacted] done", fields: []string{"private_tag"}, masked: true},
		{name: "safe text", summary: "service recovered https://example.test/path", want: "service recovered https://example.test/path"},
		{name: "safe Persian limit", summary: strings.Repeat("ر", domain.MaxEventSummaryLength), want: strings.Repeat("ر", domain.MaxEventSummaryLength)},
		{name: "mask expansion limit", summary: "https://" + marker + ":x@h " + strings.Repeat("h://x@h ", 1000) + "end", masked: true, atRuneLimit: true},
	}
	for _, streaming := range []bool{false, true} {
		boundary := "REST"
		if streaming {
			boundary = "SSE"
		}
		for _, test := range tests {
			t.Run(boundary+"/"+test.name, func(t *testing.T) {
				handler, reader, stored := retainedEventHandler(t, test.summary, test.fields)
				var items []eventResponse
				if streaming {
					writes, stop := startRetainedEventStream(t, handler)
					items = readRetainedEventFrame(t, writes)
					stop()
				} else {
					response := authenticatedProductRequest(handler, "/api/v1/events")
					var page eventPageResponse
					if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil {
						t.Fatal("retained event page unavailable")
					}
					items = page.Items
				}
				if len(items) != 1 {
					t.Fatal("retained event response missing stored row")
				}
				item := items[0]
				if strings.Contains(item.Summary, marker) || test.masked && !strings.Contains(item.Summary, "[redacted]") {
					t.Fatal("retained event credentials crossed transport boundary")
				}
				if !utf8.ValidString(item.Summary) || utf8.RuneCountInString(item.Summary) > domain.MaxEventSummaryLength || test.atRuneLimit && utf8.RuneCountInString(item.Summary) != domain.MaxEventSummaryLength {
					t.Fatal("retained event response exceeds summary contract")
				}
				if !test.atRuneLimit && item.Summary != test.want {
					t.Fatal("retained event safe context changed")
				}
				if item.ID != stored.ID || item.ResourceID == nil || stored.ResourceID == nil || *item.ResourceID != *stored.ResourceID || item.Kind != string(stored.Kind) || item.Severity != string(stored.Severity) || item.OccurredAt != authTimestamp(stored.OccurredAt) {
					t.Fatal("retained event metadata changed")
				}
				reader.mu.Lock()
				unchanged := reader.events[0].Summary == test.summary
				reader.mu.Unlock()
				if !unchanged {
					t.Fatal("transport masking rewrote stored event")
				}
			})
		}
	}
}

func TestRetainedEventStreamPollKeepsDeploymentRedactionPolicy(t *testing.T) {
	marker := retainedEventMarker(t)
	handler, reader, stored := retainedEventHandler(t, "first stored event", []string{"private_tag"})
	writes, stop := startRetainedEventStream(t, handler)
	if readRetainedEventFrame(t, writes)[0].Summary != stored.Summary {
		t.Fatal("initial retained event frame changed")
	}
	stored.Summary = "private_tag=" + marker + " https://" + marker + ":x@example.test/path"
	reader.mu.Lock()
	reader.events = []domain.InfrastructureEvent{stored}
	reader.mu.Unlock()
	item := readRetainedEventFrame(t, writes)[0]
	stop()
	if strings.Contains(item.Summary, marker) || item.Summary != "private_tag=[redacted] https://[redacted]@example.test/path" {
		t.Fatal("polled retained event bypassed transport policy")
	}
}
