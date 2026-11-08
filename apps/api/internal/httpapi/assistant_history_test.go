package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistanthistory"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
)

type historyReadStub struct {
	helperReadStub
	read func(context.Context, assistant.Actor, assistant.State, int, string) (assistanthistory.Page, error)
}

func (stub historyReadStub) History(ctx context.Context, actor assistant.Actor, state assistant.State, limit int, cursor string) (assistanthistory.Page, error) {
	return stub.read(ctx, actor, state, limit, cursor)
}

func historyEntry() assistant.HistoryEntry {
	at := time.Now().UTC().Truncate(time.Microsecond)
	return assistant.HistoryEntry{ID: helperRunID, SourceID: "source_demo", ResourceID: helperSnapshot().Run.ResourceID, Goal: assistant.ExplainResource, State: assistant.Completed, CreatedAt: at.Add(-time.Second), UpdatedAt: at, From: at.Add(-time.Minute), Until: at.Add(-time.Second), ToolCalls: 1, InferenceCalls: 1}
}

func TestPrivateHistoryRouteBindsCurrentActorAndRejectsExtraArguments(t *testing.T) {
	identity := helperIdentity(t)
	calls, denied := 0, false
	stub := historyReadStub{read: func(ctx context.Context, actor assistant.Actor, state assistant.State, limit int, cursor string) (assistanthistory.Page, error) {
		calls++
		if ctx == nil || actor.UserID != "user_helper" || actor.DigestKeyID != "helper_key" || state != assistant.Completed || limit != 10 || cursor != "" {
			t.Error("history changed authenticated scope")
		}
		if denied {
			return assistanthistory.Page{}, assistant.ErrDenied
		}
		return assistanthistory.Page{Entries: []assistant.HistoryEntry{historyEntry()}}, nil
	}}
	handler, _, logs := makeHandler(t, func(options *Options) {
		options.AssistantRuns = stub
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }}
	})
	path := "/api/v1/assistant/history"
	response := serve(handler, helperRequest(http.MethodGet, path))
	if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), `"next_cursor":null`) || calls != 1 {
		t.Fatal("private history route failed", response.Code)
	}
	for _, forbidden := range []string{"session_helper", "helper_key", "session_digest", "user_helper", "answer", "question", "fingerprint", "report", "generation"} {
		if strings.Contains(response.Body.String(), forbidden) || strings.Contains(logs.String(), helperRunID) {
			t.Fatal("history exposed internal content or logs")
		}
	}
	for _, query := range []string{"?owner=other", "?state=completed&state=running", "?state=", "?limit=0", "?limit=11", "?limit=01", "?cursor=bad", "?limit=%ZZ", "?source=other", "?cursor="} {
		response := serve(handler, helperRequest(http.MethodGet, path+query))
		if response.Code != 400 || calls != 1 {
			t.Fatal("invalid history arguments reached reader", query, response.Code)
		}
	}
	response = serve(handler, makeRequest(http.MethodGet, path, "198.51.100.10:4000"))
	if response.Code != 401 || calls != 1 {
		t.Fatal("anonymous history reached private reader")
	}
	denied = true
	response = serve(handler, helperRequest(http.MethodGet, path))
	if response.Code < 400 || strings.Contains(response.Body.String(), helperRunID) {
		t.Fatal("revoked history disclosed a row")
	}
}

func TestHistoryProjectionIsClosedAndDoesNotReconstructText(t *testing.T) {
	entry := historyEntry()
	reply, err := historyReply(assistanthistory.Page{Entries: []assistant.HistoryEntry{entry}}, assistant.Completed, 10)
	if err != nil || len(reply.Entries) != 1 || reply.NextCursor != nil {
		t.Fatal("metadata projection failed", err)
	}
	encoded, err := json.Marshal(reply)
	if err != nil || string(encoded) != "{}" || fmt.Sprintf("%+v", reply) != "[protected history reply]" {
		t.Fatal("ordinary projection exposed metadata")
	}
	for _, mutate := range []func(*assistanthistory.Page){func(p *assistanthistory.Page) { p.Entries[0].ID = "private_internal_id" }, func(p *assistanthistory.Page) { p.Entries[0].SourceID = "../source" }, func(p *assistanthistory.Page) { p.Entries[0].State = assistant.Queued }, func(p *assistanthistory.Page) { p.NextCursor = "bad" }, func(p *assistanthistory.Page) { p.Entries = make([]assistant.HistoryEntry, 11) }, func(p *assistanthistory.Page) { p.NextCursor = strings.Repeat("a", 50) + "." + strings.Repeat("b", 43) }} {
		page := assistanthistory.Page{Entries: []assistant.HistoryEntry{entry}}
		mutate(&page)
		if _, err := historyReply(page, assistant.Completed, 10); err == nil {
			t.Fatal("invalid metadata projection accepted")
		}
	}
	cursor := strings.Repeat("a", 50) + "." + strings.Repeat("b", 43)
	if state, limit, got, err := historyQuery("state=cancelled&limit=1&cursor=" + cursor); err != nil || state != assistant.Cancelled || limit != 1 || got != cursor {
		t.Fatal("bounded query rejected")
	}
	if _, err := historyReply(assistanthistory.Page{Entries: []assistant.HistoryEntry{entry}, NextCursor: cursor}, assistant.Completed, 1); err != nil {
		t.Fatal("valid bounded next page rejected")
	}
}
