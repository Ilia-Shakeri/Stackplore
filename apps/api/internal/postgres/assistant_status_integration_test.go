package postgres

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

// Identity is the same controlled facade as the public admission fixture. Every
// snapshot/replay still obtains current SQL authority and never reads content.
func testAssistantSQLStatus(t *testing.T, ctx context.Context, store *Store, actor assistant.Actor, handler http.Handler, runID string, revoked bool) {
	t.Helper()
	server := httptest.NewServer(handler)
	defer server.Close()
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	read := func(cursor string) (int, []byte) {
		r, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/assistant/runs/"+runID+"/events", nil)
		if err != nil {
			t.Fatal("construct SQL status fixture request")
		}
		r.Host = "stackplore.test"
		r.AddCookie(&http.Cookie{Name: "stackplore_session", Value: strings.Repeat("A", 43)})
		if cursor != "" {
			r.Header.Set("Last-Event-ID", cursor)
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal("read SQL-authorized status response")
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 64*1024+1))
		if err != nil || len(body) > 64*1024 || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("SQL status response failed or exceeded its private bound")
		}
		for _, marker := range []string{"Explain the selected resource.", "Health is healthy.", "answer", "citation", actor.UserID, "Fixture service", "internal_profile"} {
			if bytes.Contains(body, []byte(marker)) {
				t.Fatal("SQL status response disclosed content or owner metadata")
			}
		}
		return response.StatusCode, body
	}
	code, body := read("")
	if revoked {
		if code != 404 || bytes.Contains(body, []byte("assistant.status.v1")) {
			t.Fatal("SQL-revoked actor obtained private status")
		}
		return
	}
	current, err := store.GetAssistant(ctx, actor, runID, time.Now().UTC().Truncate(time.Microsecond))
	if err != nil || !current.State.Terminal() {
		t.Fatal("status fixture did not retain terminal metadata")
	}
	if code != 200 || bytes.Count(body, []byte("event: assistant.status.v1")) != 1 || !bytes.Contains(body, []byte("id: "+strconv.FormatInt(current.Version, 10)+"\n")) {
		t.Fatal("SQL status snapshot lost its durable version")
	}
	code, replay := read("1")
	if code != 200 || bytes.Count(replay, []byte("event: assistant.status.v1")) != int(current.Version-1) || !bytes.Contains(replay, []byte(`"state":"completed"`)) {
		t.Fatal("SQL status replay lost retained progress")
	}
	code, ahead := read("9223372036854775807")
	if code != 200 || !bytes.Contains(ahead, []byte(`"reason":"cursor_ahead"`)) || bytes.Contains(ahead, []byte("assistant.status.v1")) {
		t.Fatal("future SQL replay cursor did not reset")
	}
	// Remove one status only inside this disposable fixture. It must not cause
	// a partially published prefix or silently skip a retained sequence.
	if _, err := store.pool.Exec(ctx, `DELETE FROM assistant_status_events WHERE run_id=$1 AND sequence=2`, runID); err != nil {
		t.Fatal("create owned replay gap fixture")
	}
	code, gap := read("1")
	if code != 200 || !bytes.Contains(gap, []byte(`"reason":"replay_gap"`)) || bytes.Contains(gap, []byte("assistant.status.v1")) {
		t.Fatal("missing SQL status sequence was silently skipped")
	}
}
