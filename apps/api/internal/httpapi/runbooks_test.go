package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbookservice"
)

type runbookHTTPStub struct {
	calls   int
	purpose auth.RecentAuthPurpose
	deny    bool
}

func (stub *runbookHTTPStub) Catalog(context.Context, auth.Principal) (runbook.Catalog, error) {
	stub.calls++
	return runbook.Catalog{Generation: 0, Version: 1, Revisions: []runbook.Revision{}}, nil
}
func (stub *runbookHTTPStub) Preview(_ context.Context, actor auth.Principal, m runbookservice.Material) (runbookservice.Draft, error) {
	stub.calls++
	if actor.UserID != "user_helper" || actor.Role != auth.RoleAdministrator || m.Corpus.ID != "ops" || m.Digest == [32]byte{} {
		return runbookservice.Draft{}, runbook.ErrInvalid
	}
	return runbookservice.Draft{Token: strings.Repeat("A", 43), ExpiresAt: time.Now().UTC().Add(time.Minute)}, nil
}
func (stub *runbookHTTPStub) Commit(_ context.Context, _ auth.Principal, m runbookservice.Material, _ string, confirmed bool, grant auth.RecentGrantUse, _ domain.RequestID) (runbook.Revision, error) {
	stub.calls++
	stub.purpose = grant.Purpose
	if !confirmed {
		return runbook.Revision{}, runbook.ErrInvalid
	}
	return runbook.Revision{ID: m.Corpus.ID, Version: m.Corpus.Version, Digest: m.Digest, Audience: m.Corpus.Audience, Approved: true, Generation: m.Generation, Documents: len(m.Corpus.Documents), Bytes: 10}, nil
}
func (stub *runbookHTTPStub) Search(_ context.Context, actor auth.Principal, query, locale string, limit int, cursor string) (runbookservice.Results, error) {
	stub.calls++
	if stub.deny {
		return runbookservice.Results{}, errors.New("private-prose-failure")
	}
	if actor.UserID != "user_helper" || query != "private-search" || locale != "en" || limit != 10 || cursor != "" {
		return runbookservice.Results{}, runbook.ErrInvalid
	}
	return runbookservice.Results{Hits: []runbook.Hit{}}, nil
}
func (stub *runbookHTTPStub) Read(_ context.Context, _ auth.Principal, ref runbook.Reference) (runbook.Content, error) {
	stub.calls++
	return runbook.Content{Document: runbook.Document{ID: ref.DocumentID, Locale: ref.Locale, Title: "Reviewed help", Body: "<script>inert-prose</script>"}}, nil
}
func runbookRequest(path, body string) *http.Request {
	return withJSONBody(helperRequest(http.MethodPost, "/api/v1/assistant/runbooks"+path), body)
}

const runbookMaterialBody = `{"action":"approve","expected_version":0,"generation":0,"digest":"","corpus":{"id":"ops","version":1,"audience":"all_users","provenance":{"kind":"operator_authored","source":"Synthetic fixture","license":"Fixture author permission","use_reviewed":true,"assistance_reviewed":false},"documents":[{"id":"help","locale":"en","title":"Help","body":"Reviewed prose"}]}}`

func TestRunbookHTTPPrivateBodyAuthorityAndOnePurpose(t *testing.T) {
	identity := helperIdentity(t)
	stub := &runbookHTTPStub{}
	boundary := operationAuthentication{testAuthentication: &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }, authorizeUnsafeFunc: func(context.Context, string, string) (auth.Authentication, error) { return identity, nil }}, prepare: func(_ context.Context, input auth.RecentAuthorizationInput) (auth.Authentication, auth.RecentGrantUse, error) {
		if input.Purpose != auth.RecentAuthPurposeRunbookManage {
			return auth.Authentication{}, auth.RecentGrantUse{}, auth.ErrRecentAuthRejected
		}
		return identity, auth.RecentGrantUse{Actor: identity.Principal, Purpose: input.Purpose}, nil
	}}
	handler, _, logs := makeHandler(t, func(options *Options) { options.Runbooks = stub; options.Authentication = boundary })
	body := `{"query":"private-search","locale":"en","limit":10,"cursor":""}`
	response := serve(handler, runbookRequest("/search", body))
	if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), `"hits":[]`) {
		t.Fatal("private body search failed", response.Code)
	}
	for _, request := range []*http.Request{runbookRequest("/search?query=private-search", body), runbookRequest("/search", `{"query":"private-search","locale":"en","limit":10,"cursor":"","owner":"other"}`), runbookRequest("/search", `{"query":"private-search","query":"other","locale":"en","limit":10,"cursor":""}`)} {
		if serve(handler, request).Code != 400 || stub.calls != 1 {
			t.Fatal("unclosed input reached search")
		}
	}
	request := runbookRequest("/search", body)
	request.Header.Set("Origin", "https://foreign.test")
	if serve(handler, request).Code != 403 || stub.calls != 1 {
		t.Fatal("foreign origin reached search")
	}
	request = runbookRequest("/search", body)
	request.Header.Del(csrfHeaderName)
	if serve(handler, request).Code < 400 || stub.calls != 1 {
		t.Fatal("missing proof reached search")
	}
	if serve(handler, runbookRequest("/preview", runbookMaterialBody)).Code != 403 || stub.calls != 1 {
		t.Fatal("viewer prepared approval")
	}
	identity.Principal.Role = auth.RoleAdministrator
	response = serve(handler, runbookRequest("/preview", runbookMaterialBody))
	if response.Code != 200 || stub.calls != 2 || !strings.Contains(response.Body.String(), `"confirmation_token"`) {
		t.Fatal("administrator preview failed", response.Code)
	}
	commit := `{"material":` + runbookMaterialBody + `,"confirmation_token":"` + strings.Repeat("A", 43) + `","confirmed":true}`
	if serve(handler, runbookRequest("/commit", commit)).Code < 400 || stub.calls != 2 {
		t.Fatal("missing recent proof reached commit")
	}
	request = runbookRequest("/commit", commit)
	request.Header.Set(recentAuthHeaderName, strings.Repeat("A", 43))
	response = serve(handler, request)
	if response.Code != 200 || stub.calls != 3 || stub.purpose != auth.RecentAuthPurposeRunbookManage {
		t.Fatal("approval purpose not bound", response.Code)
	}
	stub.deny = true
	response = serve(handler, runbookRequest("/search", body))
	if response.Code != 503 || strings.Contains(response.Body.String(), "private-prose-failure") {
		t.Fatal("raw error escaped")
	}
	if strings.Contains(logs.String(), "private-search") || strings.Contains(logs.String(), "Reviewed prose") {
		t.Fatal("private body entered logs")
	}
}

func TestRunbookMaterialCanonicalDigestAndExactReference(t *testing.T) {
	for _, value := range []string{strings.Repeat("A", 64), strings.Repeat("0", 64), "x"} {
		if _, err := runbookHexDigest(value); err == nil {
			t.Fatal("invalid digest accepted")
		}
	}
	version, generation := int64(0), int64(0)
	referenceOnly := false
	wire := runbookMaterialJSON{Action: "approve", ExpectedVersion: &version, Generation: &generation, Corpus: runbookCorpusJSON{ID: "ops", Version: 1, Audience: runbook.AllUsers, Provenance: &runbookProvenanceJSON{Kind: "operator_authored", Source: "Synthetic fixture", License: "Fixture author permission", UseReviewed: true, AssistanceReviewed: &referenceOnly}, Documents: []runbookDocumentJSON{{ID: "help", Locale: "en", Title: "Help", Body: "Private prose"}}}}
	material, err := wire.material()
	if err != nil || material.Digest == [32]byte{} {
		t.Fatal("exact material unavailable")
	}
	wire.Digest = strings.Repeat("a", 64)
	if _, err := wire.material(); err == nil {
		t.Fatal("approval accepted caller digest")
	}
	ref := runbookReferenceJSON{CorpusID: "ops", Version: 1, Digest: strings.Repeat("a", 64), DocumentID: "help", Locale: "en", Generation: 0}
	parsed, err := ref.reference()
	if err != nil || parsed.DocumentID != "help" {
		t.Fatal("exact reference lost")
	}
}
