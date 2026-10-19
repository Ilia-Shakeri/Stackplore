package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/realtime"
)

type streamBudgetProvider struct {
	*demo.Provider
	mu         sync.Mutex
	large      bool
	generation int
	denied     bool
}

func (provider *streamBudgetProvider) change(large, denied bool, generation int) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.large, provider.denied, provider.generation = large, denied, generation
}

func (provider *streamBudgetProvider) state() (bool, bool, int) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.large, provider.denied, provider.generation
}

func (provider *streamBudgetProvider) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	page, err := provider.Provider.ListEvents(ctx, query)
	if err != nil || len(page.Events) == 0 {
		return page, err
	}
	large, denied, generation := provider.state()
	if denied {
		page.Events = nil
		page.Page = domain.PageInfo{Limit: query.Page.Limit}
		page.Status.State, page.Status.ReasonCode = domain.AvailabilityPermissionDenied, "provider.permission_denied"
		return page, page.ValidateFor(query)
	}
	event := page.Events[0]
	count := 1
	if large {
		count = min(100, query.Page.Limit)
		event.Summary = strings.Repeat("ن", domain.MaxEventSummaryLength)
	}
	event.ObservedAt = event.ObservedAt.Add(time.Duration(generation) * time.Microsecond)
	page.Status.ObservedAt = page.Status.ObservedAt.Add(time.Duration(generation) * time.Microsecond)
	page.Events = make([]domain.InfrastructureEvent, count)
	page.Page = domain.PageInfo{Limit: query.Page.Limit}
	for index := range page.Events {
		page.Events[index] = event
		page.Events[index].ID = domain.EventID(fmt.Sprintf("event_budget_%03d", index))
	}
	return page, page.ValidateFor(query)
}

func (provider *streamBudgetProvider) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	page, err := provider.Provider.ReadLogs(ctx, query)
	if err != nil || len(page.Records) == 0 {
		return page, err
	}
	large, denied, generation := provider.state()
	if denied {
		page.Records = nil
		page.Page = domain.PageInfo{Limit: query.LineLimit}
		page.Status.State, page.Status.ReasonCode = domain.AvailabilityPermissionDenied, "provider.permission_denied"
		return page, page.ValidateFor(query)
	}
	record := page.Records[0]
	count := 1
	if large {
		count = min(100, query.LineLimit)
		record.Message = strings.Repeat("ن", domain.MaxLogLineLength)
	}
	record.Timestamp = record.Timestamp.Add(time.Duration(generation) * time.Microsecond)
	page.Status.ObservedAt = page.Status.ObservedAt.Add(time.Duration(generation) * time.Microsecond)
	page.Records = make([]domain.LogRecord, count)
	page.Page = domain.PageInfo{Limit: query.LineLimit}
	for index := range page.Records {
		page.Records[index] = record
		page.Records[index].ID = domain.LogEntryID(fmt.Sprintf("log_budget_%03d", index))
		page.Records[index].Sequence = uint64(index + 1)
	}
	return page, page.ValidateFor(query)
}

func streamBudgetServer(t *testing.T, provider *streamBudgetProvider) *httptest.Server {
	t.Helper()
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.RequestTimeout = 15 * time.Second
		options.MaximumResponseBody = 16 * 1024 * 1024
		options.ProductProviders = []ProductProvider{provider}
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{User: auth.User{ID: "usr_stream_budget"}}, nil
		}}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func assertStreamBudgetReset(t *testing.T, stream *authorityHTTPStream, expectedID uint64) {
	t.Helper()
	id, event, data := stream.next(t)
	if id != expectedID || event != "stream.reset.v1" || string(data) != `{"reason":"snapshot_overflow"}` || len(data) > 256 {
		t.Fatal("oversized legal snapshot did not emit its bounded refetch reset")
	}
}

func TestStreamBudgetLegalBulkEventsUseBoundedResetAndREST(t *testing.T) {
	provider := &streamBudgetProvider{Provider: demo.MustNew(), large: true}
	server := streamBudgetServer(t, provider)
	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/events?limit=100", nil)
	if err != nil {
		t.Fatal("create bulk REST request")
	}
	request.Host = "stackplore.test"
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: retainedEventMarker(t)})
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal("bulk REST request failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	var page struct {
		Items []eventResponse `json:"items"`
	}
	if err != nil || response.StatusCode != http.StatusOK || len(body) <= 256*1024 || json.Unmarshal(body, &page) != nil || len(page.Items) != 100 {
		t.Fatalf("legal bulk REST fixture failed: status=%d bytes=%d items=%d", response.StatusCode, len(body), len(page.Items))
	}
	t.Logf("legal REST status=%d bytes=%d items=%d", response.StatusCode, len(body), len(page.Items))
	stream := startAuthorityHTTPStream(t, server, streamEvents, "")
	if stream.response.StatusCode != http.StatusOK {
		t.Fatal("bulk stream handshake failed")
	}
	assertStreamBudgetReset(t, stream, 1)
	provider.change(false, false, 1)
	assertAuthoritySnapshot(t, stream, streamEvents, 2)
}

func TestStreamBudgetLegalBulkLogsUseBoundedReset(t *testing.T) {
	provider := &streamBudgetProvider{Provider: demo.MustNew(), large: true}
	server := streamBudgetServer(t, provider)
	stream := startAuthorityHTTPStream(t, server, streamLogs, "")
	if stream.response.StatusCode != http.StatusOK {
		t.Fatal("bulk log stream handshake failed")
	}
	assertStreamBudgetReset(t, stream, 1)
	provider.change(false, false, 1)
	assertAuthoritySnapshot(t, stream, streamLogs, 2)
}

func streamBudgetFrame(t *testing.T, subscription *realtime.Subscription) realtime.Frame {
	t.Helper()
	select {
	case frame, open := <-subscription.C:
		if !open || len(frame.Data) > transientStreamMaximumFrame {
			t.Fatal("bounded live stream frame unavailable")
		}
		return frame
	case <-time.After(3 * time.Second):
		t.Fatal("bounded live stream frame timed out")
	}
	return realtime.Frame{}
}

func TestStreamBudgetPollOverflowChangesRecoveryAndDenial(t *testing.T) {
	for _, topic := range []streamTopic{streamEvents, streamLogs} {
		t.Run(string(topic), func(t *testing.T) {
			provider := &streamBudgetProvider{Provider: demo.MustNew()}
			initial, err := buildStreamSnapshot(context.Background(), topic, provider)
			if err != nil {
				t.Fatal("build initial budget fixture")
			}
			manager, err := realtime.NewManager(realtime.ManagerOptions{MaximumFeeds: 1, ReplaySize: 4, QueueSize: 4, MaximumFrame: transientStreamMaximumFrame})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(manager.Close)
			subscription, err := manager.Subscribe(string(topic)+"/budget", 0, streamProducerWithInterval(topic, provider, initial, 100*time.Millisecond))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(subscription.Close)
			if frame := streamBudgetFrame(t, subscription); frame.ID != 1 || frame.Event != string(topic)+".snapshot.v1" {
				t.Fatal("initial small snapshot changed")
			}
			provider.change(true, false, 1)
			if frame := streamBudgetFrame(t, subscription); frame.ID != 2 || frame.Event != "stream.reset.v1" || string(frame.Data) != `{"reason":"snapshot_overflow"}` {
				t.Fatal("poll overflow did not publish bounded refetch reset")
			}
			select {
			case <-subscription.C:
				t.Fatal("unchanged oversized snapshot repeated its reset")
			case <-time.After(350 * time.Millisecond):
			}
			provider.change(true, false, 2)
			if frame := streamBudgetFrame(t, subscription); frame.ID != 3 || frame.Event != "stream.reset.v1" || string(frame.Data) != `{"reason":"snapshot_overflow"}` {
				t.Fatal("changed oversized snapshot lost its refetch reset")
			}
			provider.change(false, false, 3)
			if frame := streamBudgetFrame(t, subscription); frame.ID != 4 || frame.Event != string(topic)+".snapshot.v1" {
				t.Fatal("in-budget recovery did not resume normal snapshots")
			}
			provider.change(true, true, 4)
			select {
			case _, open := <-subscription.C:
				if open || !subscription.Revoked() {
					t.Fatal("overflow fallback bypassed permission revocation")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("permission denial did not stop oversized producer")
			}
		})
	}
}

func TestStreamBudgetFrameLimitAppliesToEveryTransientTopic(t *testing.T) {
	for _, topic := range []streamTopic{streamHealth, streamMetrics, streamEvents, streamLogs} {
		for _, length := range []int{transientStreamMaximumFrame, transientStreamMaximumFrame + 1} {
			data := []byte(`{"text":"` + strings.Repeat("x", length-len(`{"text":""}`)) + `"}`)
			if len(data) != length || !json.Valid(data) {
				t.Fatal("frame boundary fixture invalid")
			}
			event, frame := streamSnapshotFrame(topic, data)
			if length == transientStreamMaximumFrame {
				if event != string(topic)+".snapshot.v1" || string(frame) != string(data) {
					t.Fatal("in-budget boundary snapshot changed")
				}
			} else if event != "stream.reset.v1" || string(frame) != `{"reason":"snapshot_overflow"}` {
				t.Fatal("overflow boundary did not use bounded reset")
			}
		}
	}
}
