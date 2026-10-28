package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
)

// Authentication is synthetic here. The real service/repository independently
// rechecks the retained SQL actor, authority and current revocation on each read.
type helperHTTPAuthentication struct {
	*auth.Service
	current auth.Authentication
}

func (fixture *helperHTTPAuthentication) Authenticate(context.Context, string) (auth.Authentication, error) {
	return fixture.current, nil
}
func (fixture *helperHTTPAuthentication) AuthorizeUnsafe(context.Context, string, string) (auth.Authentication, error) {
	return fixture.current, nil
}

func testAssistantHTTPReplay(t *testing.T, ctx context.Context, store *Store, actor assistant.Actor, service *assistantservice.Service, runID string) {
	t.Helper()
	var sessionID string
	if err := store.pool.QueryRow(ctx, `SELECT id FROM sessions WHERE user_id=$1 AND token_digest=$2`, actor.UserID, actor.SessionDigest[:]).Scan(&sessionID); err != nil {
		t.Fatal("read owned HTTP actor fixture")
	}
	digest, err := auth.ParseSessionTokenDigest(actor.SessionDigest[:])
	if err != nil {
		t.Fatal("decode owned actor reference")
	}
	identity := auth.Authentication{Principal: auth.Principal{UserID: auth.UserID(actor.UserID), SessionID: auth.SessionID(sessionID), Role: auth.RoleAdministrator, AuthRevision: actor.AuthRevision}, Session: auth.Session{ID: auth.SessionID(sessionID), UserID: auth.UserID(actor.UserID), TokenDigest: digest, TokenDigestKeyID: actor.DigestKeyID, UserAuthRevision: actor.AuthRevision}}
	var logs bytes.Buffer
	handler, err := httpapi.NewHandler(httpapi.Options{Logger: slog.New(slog.NewJSONHandler(&logs, nil)), Readiness: httpapi.NewReadiness(), Authentication: &helperHTTPAuthentication{current: identity}, PublicOrigin: httpapi.Origin{Scheme: "http", Authority: "stackplore.test"}, RequestTimeout: 5 * time.Second, MaximumRequestBody: 4096, MaximumResponseBody: 128 * 1024, MaximumInFlight: 4, RateLimit: httpapi.RateLimitConfig{RequestsPerSecond: 100, Burst: 100, MaximumClients: 16}, AssistantRuns: service})
	if err != nil {
		t.Fatal("construct owned SQL-bound private HTTP fixture")
	}
	request := func(method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://stackplore.test"+path, nil)
		r = r.WithContext(ctx)
		r.AddCookie(&http.Cookie{Name: "stackplore_session", Value: strings.Repeat("A", 43)})
		if method == http.MethodPost {
			r.Header.Set("Origin", "http://stackplore.test")
			r.Header.Set("X-Stackplore-CSRF", strings.Repeat("B", 42)+"A")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	base := "/api/v1/assistant/runs/" + runID
	ready := request(http.MethodGet, base)
	var reply struct {
		ContentState string           `json:"content_state"`
		Answer       *json.RawMessage `json:"answer"`
		Citations    []struct {
			ID string `json:"id"`
		} `json:"citations"`
	}
	if ready.Code != 200 || ready.Header().Get("Cache-Control") != "no-store" || json.Unmarshal(ready.Body.Bytes(), &reply) != nil || reply.ContentState != "ready" || reply.Answer == nil || len(reply.Citations) != 1 {
		t.Fatal("SQL-authorized HTTP answer was not a complete private reply")
	}
	citation := request(http.MethodGet, base+"/citations/"+reply.Citations[0].ID)
	if citation.Code != 200 || strings.Contains(citation.Body.String(), "answer") || strings.Contains(citation.Body.String(), "object_id") || strings.Contains(citation.Body.String(), "href") {
		t.Fatal("SQL-bound citation projection failed")
	}
	unknown := request(http.MethodGet, base+"/citations/ev_"+strings.Repeat("d", 64))
	if unknown.Code != 404 {
		t.Fatal("unknown SQL-bound citation resolved")
	}
	cancelled := request(http.MethodPost, base+"/cancel")
	if cancelled.Code != 200 || strings.Contains(cancelled.Body.String(), "answer") || strings.Contains(cancelled.Body.String(), "content_state") {
		t.Fatal("SQL-bound cancellation disclosed retained text")
	}
	lost := request(http.MethodGet, base)
	if lost.Code != 200 || !strings.Contains(lost.Body.String(), `"content_state":"volatile_lost"`) || strings.Contains(lost.Body.String(), "answer") {
		t.Fatal("cancelled retention did not preserve completed metadata and truthfully lose content")
	}
	if _, err := store.pool.Exec(ctx, `UPDATE users SET auth_revision=auth_revision+1 WHERE id=$1`, actor.UserID); err != nil {
		t.Fatal("revoke owned HTTP actor revision")
	}
	denied := request(http.MethodGet, base)
	if denied.Code != 404 || strings.Contains(denied.Body.String(), "answer") {
		t.Fatal("SQL-revoked actor read private run through HTTP")
	}
	for _, marker := range []string{"Explain the selected resource.", "Fixture service", strings.Repeat("A", 43), strings.Repeat("B", 42) + "A"} {
		if strings.Contains(logs.String(), marker) {
			t.Fatal("private HTTP material entered captured logs")
		}
	}
}
