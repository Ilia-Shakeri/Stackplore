package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantconversation"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/go-chi/chi/v5"
)

type helperStatusStub struct {
	helperReadStub
	status func(context.Context, assistant.Actor, string, int64) (assistantconversation.StatusPage, error)
}

func (stub helperStatusStub) Status(ctx context.Context, actor assistant.Actor, id string, after int64) (assistantconversation.StatusPage, error) {
	return stub.status(ctx, actor, id, after)
}

func helperStatusPage(run assistant.Run) assistantconversation.StatusPage {
	return assistantconversation.StatusPage{Run: run, Events: []assistant.Event{{Sequence: run.Version, RunVersion: run.Version, State: run.State, At: run.UpdatedAt}}}
}

func helperStatusNetworkRead(t *testing.T, handler http.Handler, cursor string) (int, http.Header, []byte) {
	t.Helper()
	server := httptest.NewServer(handler)
	defer server.Close()
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/assistant/runs/"+helperRunID+"/events", nil)
	if err != nil {
		t.Fatal("construct owned status request")
	}
	request.Host = "stackplore.test"
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testOpaqueProof})
	if cursor != "" {
		request.Header.Set("Last-Event-ID", cursor)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal("read owned status response")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, assistantStatusMaximumBatch+1))
	if err != nil || len(body) > assistantStatusMaximumBatch {
		t.Fatalf("owned status body failed: unexpected_eof=%t oversized=%t", errors.Is(err, io.ErrUnexpectedEOF), len(body) > assistantStatusMaximumBatch)
	}
	return response.StatusCode, response.Header, body
}

func TestAssistantStatusNetworkPublishesOnlyValidatedMetadata(t *testing.T) {
	identity := helperIdentity(t)
	var calls atomic.Int32
	reader := helperStatusStub{status: func(ctx context.Context, actor assistant.Actor, id string, after int64) (assistantconversation.StatusPage, error) {
		calls.Add(1)
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 3*time.Second || actor.UserID != "user_helper" || id != helperRunID || after != 0 {
			t.Error("status actor, cursor or duration changed")
		}
		return helperStatusPage(helperSnapshot().Run), nil
	}}
	handler, _, logs := makeHandler(t, func(options *Options) {
		options.AssistantRuns = reader
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }}
	})
	code, headers, body := helperStatusNetworkRead(t, handler, "")
	fixture, err := os.ReadFile("../../../../contracts/fixtures/assistant-status.json")
	var expected struct {
		RunID       string `json:"run_id"`
		AfterCursor string `json:"after_cursor"`
		Wire        string `json:"wire"`
	}
	if err != nil || json.Unmarshal(fixture, &expected) != nil || expected.RunID != helperRunID || expected.AfterCursor != "0" || !bytes.Equal(body, []byte(expected.Wire)) {
		t.Fatal("native status publication differs from shared client fixture")
	}
	if code != 200 || headers.Get("Content-Type") != "text/event-stream; charset=utf-8" || headers.Get("Cache-Control") != "no-store" || headers.Get("X-Accel-Buffering") != "no" || !bytes.Contains(body, []byte("id: 3\nevent: assistant.status.v1\n")) || calls.Load() != 1 {
		t.Fatal("private status route was buffered, cacheable or malformed")
	}
	for _, marker := range []string{"Selected resource is available.", "answer", "question", "citation", "user_helper", "session_helper", "internal_profile_marker", "internal_lease_marker", "helper_key", "resource_id", "source_id"} {
		if bytes.Contains(body, []byte(marker)) || strings.Contains(logs.String(), marker) {
			t.Fatal("status wire or logs disclosed content or internal metadata")
		}
	}
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			var event map[string]any
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event) != nil || len(event) != 5 || event["run_id"] != helperRunID || event["state"] != "completed" {
				t.Fatal("status payload was not its closed schema")
			}
		}
	}
}

func TestAssistantStatusResetUsesClosedReasonAndNoStatusID(t *testing.T) {
	identity := helperIdentity(t)
	reader := helperStatusStub{status: func(_ context.Context, _ assistant.Actor, _ string, after int64) (assistantconversation.StatusPage, error) {
		if after != 9223372036854775807 {
			t.Error("maximum canonical cursor changed")
		}
		return assistantconversation.StatusPage{Run: helperSnapshot().Run, Reset: assistantconversation.ResetCursorAhead}, nil
	}}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.AssistantRuns = reader
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }}
	})
	code, _, body := helperStatusNetworkRead(t, handler, "9223372036854775807")
	if code != 200 || !bytes.Contains(body, []byte("event: assistant.reset.v1")) || !bytes.Contains(body, []byte(`"reason":"cursor_ahead"`)) || bytes.Contains(body, []byte("\nid:")) || bytes.HasPrefix(body, []byte("id:")) {
		t.Fatal("reset identity or reason drifted")
	}
}

func TestAssistantStatusRejectsBadRequestBeforeMetadataRead(t *testing.T) {
	identity := helperIdentity(t)
	calls := 0
	reader := helperStatusStub{status: func(context.Context, assistant.Actor, string, int64) (assistantconversation.StatusPage, error) {
		calls++
		return helperStatusPage(helperSnapshot().Run), nil
	}}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.AssistantRuns = reader
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }}
	})
	for _, cursor := range []string{"", "01", "-1", "+1", " 1", "1 ", "1,2", "9223372036854775808", "99999999999999999999"} {
		request := helperRequest(http.MethodGet, "/api/v1/assistant/runs/"+helperRunID+"/events")
		request.Header.Set("Last-Event-ID", cursor)
		response := serve(handler, request)
		if response.Code != 400 || !json.Valid(response.Body.Bytes()) {
			t.Fatal("malformed replay cursor reached stream or produced duplicate problem")
		}
	}
	for _, change := range []func(*http.Request){func(r *http.Request) { r.URL.RawQuery = "extra=1" }, func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader("private")); r.ContentLength = 7 }, func(r *http.Request) { r.Header.Add("Last-Event-ID", "1"); r.Header.Add("Last-Event-ID", "2") }} {
		request := helperRequest(http.MethodGet, "/api/v1/assistant/runs/"+helperRunID+"/events")
		change(request)
		response := serve(handler, request)
		if response.Code != 400 || !json.Valid(response.Body.Bytes()) {
			t.Fatal("extra private stream input reached use case or malformed its problem")
		}
	}
	if calls != 0 {
		t.Fatal("invalid stream input performed metadata I/O")
	}
}

func TestAssistantStatusBatchRejectsWholeMalformedPage(t *testing.T) {
	identity := helperIdentity(t)
	actor, err := assistantActor(identity)
	if err != nil {
		t.Fatal("construct trusted status actor")
	}
	for _, change := range []func(*assistantconversation.StatusPage){
		func(p *assistantconversation.StatusPage) { p.Events[0].State = "private_state" },
		func(p *assistantconversation.StatusPage) { p.Events[0].Sequence = 2 },
		func(p *assistantconversation.StatusPage) { p.Events[0].RunVersion = 64 },
		func(p *assistantconversation.StatusPage) { p.Events[0].At = time.Time{} },
		func(p *assistantconversation.StatusPage) { p.Run.OwnerID = "other" },
		func(p *assistantconversation.StatusPage) { p.Reset = "private_reason"; p.Events = nil },
		func(p *assistantconversation.StatusPage) { p.Reset = assistantconversation.ResetReplayGap },
		func(p *assistantconversation.StatusPage) { p.Events = append(p.Events, p.Events[0]) },
	} {
		page := helperStatusPage(helperSnapshot().Run)
		change(&page)
		if body, _, err := assistantStatusBatch(page, actor, helperRunID, 0); err == nil || len(body) != 0 {
			t.Fatal("malformed status page released a prefix")
		}
	}
}

func helperShortStatusHandler(t *testing.T, statuses helperStatusStub, timers assistantStatusTimers, gate *streamGate) http.Handler {
	t.Helper()
	identity := helperIdentity(t)
	helper := assistantHTTP{product: productHTTP{auth: authHTTP{service: &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }}}}, statuses: statuses, statusGate: gate}
	router := chi.NewRouter()
	router.Get("/api/v1/assistant/runs/{run_id}/events", func(w http.ResponseWriter, r *http.Request) { helper.streamStatusWithTimers(w, r, timers) })
	return router
}

func TestAssistantStatusLifetimeCancelsReadAndFreesSlot(t *testing.T) {
	var calls atomic.Int32
	var cancelled atomic.Bool
	run := helperSnapshot().Run
	run.State = assistant.Running
	run.Version = 1
	run.UpdatedAt = run.CreatedAt
	statuses := helperStatusStub{status: func(ctx context.Context, _ assistant.Actor, _ string, after int64) (assistantconversation.StatusPage, error) {
		if calls.Add(1) == 1 {
			return helperStatusPage(run), nil
		}
		if after != 1 {
			t.Error("completed status cursor did not advance")
		}
		<-ctx.Done()
		cancelled.Store(true)
		return assistantconversation.StatusPage{}, assistant.ErrUnavailable
	}}
	gate := newStreamGate(2, 1)
	handler := helperShortStatusHandler(t, statuses, assistantStatusTimers{life: 40 * time.Millisecond, poll: time.Millisecond, keepalive: 5 * time.Millisecond}, gate)
	code, _, body := helperStatusNetworkRead(t, handler, "")
	if code != 200 || !bytes.Contains(body, []byte("assistant.status.v1")) || !cancelled.Load() {
		t.Fatal("subscription lifetime did not cancel its metadata read")
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.active != 0 || len(gate.byUser) != 0 {
		t.Fatal("closed subscription retained its owner slot")
	}
}

func TestAssistantStatusRevocationAndScopeSwapCloseWithoutNewFrame(t *testing.T) {
	for _, revoke := range []bool{true, false} {
		var calls atomic.Int32
		run := helperSnapshot().Run
		run.State = assistant.Running
		run.Version = 1
		run.UpdatedAt = run.CreatedAt
		statuses := helperStatusStub{status: func(context.Context, assistant.Actor, string, int64) (assistantconversation.StatusPage, error) {
			if calls.Add(1) == 1 {
				return helperStatusPage(run), nil
			}
			if revoke {
				return assistantconversation.StatusPage{}, assistant.ErrDenied
			}
			changed := run
			changed.SourceID = "src_other"
			changed.Version = 2
			changed.UpdatedAt = changed.CreatedAt.Add(time.Second)
			return helperStatusPage(changed), nil
		}}
		gate := newStreamGate(2, 1)
		handler := helperShortStatusHandler(t, statuses, assistantStatusTimers{life: time.Second, poll: time.Millisecond, keepalive: 10 * time.Millisecond}, gate)
		code, _, body := helperStatusNetworkRead(t, handler, "")
		if code != 200 || bytes.Count(body, []byte("event: assistant.status.v1")) != 1 || calls.Load() != 2 {
			t.Fatal("revoked or replaced authority published another status")
		}
	}
}

func TestAssistantStatusUsesOnlyCanonicalStreamPaths(t *testing.T) {
	valid := "/api/v1/assistant/runs/" + helperRunID + "/events"
	if !isStreamPath(valid) {
		t.Fatal("private status path did not enter bounded stream transport")
	}
	for _, path := range []string{valid + "/", strings.Replace(valid, "ar_", "xx_", 1), strings.Replace(valid, strings.Repeat("a", 64), strings.Repeat("A", 64), 1), valid + "/extra", "/api/v1/assistant/runs/not-an-id/events"} {
		if isStreamPath(path) {
			t.Fatal("malformed private path bypassed ordinary response boundary")
		}
	}
}

func TestAssistantStatusOwnerQuotaAndUnsupportedWriterStayClosed(t *testing.T) {
	var calls atomic.Int32
	statuses := helperStatusStub{status: func(context.Context, assistant.Actor, string, int64) (assistantconversation.StatusPage, error) {
		calls.Add(1)
		return helperStatusPage(helperSnapshot().Run), nil
	}}
	gate := newStreamGate(1, 1)
	release, ok := gate.acquire("user_helper")
	if !ok {
		t.Fatal("hold owned status slot")
	}
	defer release()
	handler := helperShortStatusHandler(t, statuses, assistantStatusTimers{life: time.Second, poll: time.Millisecond, keepalive: 10 * time.Millisecond}, gate)
	code, _, body := helperStatusNetworkRead(t, handler, "")
	if code != 429 || calls.Load() != 0 || !json.Valid(body) {
		t.Fatal("owner quota did not reject before metadata I/O")
	}
	release()
	response := serve(handler, helperRequest(http.MethodGet, "/api/v1/assistant/runs/"+helperRunID+"/events"))
	if response.Code != 503 || !json.Valid(response.Body.Bytes()) || bytes.Contains(response.Body.Bytes(), []byte("assistant.status.v1")) {
		t.Fatal("writer without enforceable deadlines published an unbounded stream")
	}
}

func TestAssistantStatusReauthenticationKeepsFailureKindsAndDoesNotReadOnFailure(t *testing.T) {
	for _, test := range []struct {
		err       error
		malformed bool
		code      int
	}{{auth.ErrUnauthenticated, false, 404}, {errors.New("private storage fault"), false, 503}, {nil, true, 503}} {
		identity := helperIdentity(t)
		var authentications, reads atomic.Int32
		statuses := helperStatusStub{status: func(context.Context, assistant.Actor, string, int64) (assistantconversation.StatusPage, error) {
			reads.Add(1)
			return helperStatusPage(helperSnapshot().Run), nil
		}}
		handler, _, logs := makeHandler(t, func(options *Options) {
			options.AssistantRuns = statuses
			options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
				if authentications.Add(1) > 1 {
					current := identity
					if test.malformed {
						current.Principal.UserID = ""
					}
					return current, test.err
				}
				return identity, nil
			}}
		})
		code, _, body := helperStatusNetworkRead(t, handler, "")
		if code != test.code || reads.Load() != 0 || !json.Valid(body) || bytes.Contains(body, []byte("private storage fault")) || strings.Contains(logs.String(), "private storage fault") {
			t.Fatal("reauthentication fault crossed the metadata or diagnostic boundary")
		}
	}
}

func TestStatusOwnerGateAcceptsTheFullValidIdentityBound(t *testing.T) {
	gate := newStreamGate(2, 1)
	owner := auth.UserID(strings.Repeat("a", 128))
	if owner.Validate() != nil {
		t.Fatal("maximum owner identity fixture is invalid")
	}
	release, ok := gate.acquire(owner)
	if !ok {
		t.Fatal("valid maximum-length owner lost its namespaced stream slot")
	}
	defer release()
	if _, ok := gate.acquire(owner); ok {
		t.Fatal("long owner bypassed per-user limit")
	}
	other, ok := gate.acquire(auth.UserID(strings.Repeat("a", 127) + "b"))
	if !ok {
		t.Fatal("distinct long owner shared a bucket")
	}
	other()
	if _, ok := gate.acquire(auth.UserID(strings.Repeat("a", 129))); ok {
		t.Fatal("oversized owner identity accepted")
	}
}
