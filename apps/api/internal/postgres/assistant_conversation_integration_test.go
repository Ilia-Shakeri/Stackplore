package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantconversation"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

// Only the SQL authority, coordinator and HTTP adapter are real here. Identity,
// resource selection and the loopback processor protocol are controlled fixtures.
type conversationSQLResolver struct{ ref domain.ResourceRef }

func (resolver conversationSQLResolver) Resolve(ctx context.Context, actor assistant.Actor, source domain.SourceID, resource domain.ResourceID, selected domain.TimeRange) (contextbuilder.Scope, error) {
	if ctx.Err() != nil || !actor.Valid() || source != resolver.ref.Identity.SourceID || resource != resolver.ref.ID {
		return contextbuilder.Scope{}, assistant.ErrDenied
	}
	return contextbuilder.Scope{Source: domain.ProviderRef{Provider: resolver.ref.Identity.Provider, SourceID: source}, ConfigurationVersion: 1, PolicyVersion: 1, Resources: []domain.ResourceRef{resolver.ref}, Range: selected}, nil
}

func TestAssistantConversationHTTPAdmissionRetainsOneSQLReservation(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := store.ConfigureAssistant(ctx, assistant.Settings{Version: 2, Enabled: true, PolicyVersion: 1, ProfileID: "local_test", ProfileVersion: 1, UserQuota: 100000, InstallationQuota: 1000000}, assistantOperator("conversation_quota", at)); err != nil {
		t.Fatal("configure conversation fixture")
	}
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "source_demo"}
	identity, err := domain.NewResourceIdentity(source.Provider, source.SourceID, "environment", "demo.service", domain.Scope{Kind: domain.ScopeProject, Name: "fixture"}, "native_fixture")
	if err != nil {
		t.Fatal("construct conversation identity")
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal("construct conversation reference")
	}
	backend := &assistantServiceBackend{store: store}
	backend.delegate = newAssistantWireFixture(t, backend)
	gateway, err := inference.New(backend, time.Now)
	if err != nil {
		t.Fatal("construct conversation gateway")
	}
	provider := &assistantServiceProvider{ref: ref}
	reader, err := assistantevidence.New(assistantevidence.Options{Gate: assistantevidence.RepositoryGate{Repository: store}, Now: time.Now, Sources: []assistantevidence.Source{{Reference: source, ConfigurationVersion: 1, Read: provider, Live: provider}}})
	if err != nil {
		t.Fatal("construct conversation authority")
	}
	evidence := &assistantServiceEvidence{reader: reader}
	key := make([]byte, 32)
	key[0] = 1
	runtime, err := assistantservice.New(assistantservice.Options{Repository: store, Gateway: gateway, Authorizer: evidence, Collector: evidence, Lifetime: ctx, Now: time.Now, ProcessID: "conversation_fixture", FingerprintKey: key, Workers: 1, PollInterval: 100 * time.Millisecond, Heartbeat: time.Second, OutputLimit: 128})
	if err != nil {
		t.Fatal("construct conversation runtime")
	}
	defer runtime.Close()
	service, err := assistantconversation.New(assistantconversation.Options{Runtime: runtime, Repository: store, Resolver: conversationSQLResolver{ref: ref}, Binding: assistantconversation.Binding{ProfileID: "local_test", ProfileVersion: 1, PolicyVersion: 1, ReservedUnits: 16128}, Key: key, Now: time.Now})
	clear(key)
	if err != nil {
		t.Fatal("construct conversation coordinator")
	}
	defer service.Close()
	handler, logs := newAssistantSQLHTTPFixture(t, ctx, store, actor, service)
	question := "Explain the selected resource."
	nonce := strings.Repeat("e", 64)
	post := func(window int, text, conversation string) *httptest.ResponseRecorder {
		body := map[string]any{"source_id": string(source.SourceID), "resource_id": string(ref.ID), "window_minutes": window, "locale": "en", "question": text}
		if conversation != "" {
			body["conversation_id"] = conversation
		}
		wire, encodeErr := json.Marshal(body)
		if encodeErr != nil {
			t.Fatal("encode bounded conversation fixture")
		}
		r := httptest.NewRequest(http.MethodPost, "http://stackplore.test/api/v1/assistant/runs", bytes.NewReader(wire)).WithContext(ctx)
		r.AddCookie(&http.Cookie{Name: "stackplore_session", Value: strings.Repeat("A", 43)})
		r.Header.Set("Origin", "http://stackplore.test")
		r.Header.Set("X-Stackplore-CSRF", strings.Repeat("B", 42)+"A")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", nonce)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		clear(wire)
		return w
	}
	first := post(15, question, "")
	var receipt struct {
		Run struct {
			ID             string `json:"id"`
			ConversationID string `json:"conversation_id"`
		} `json:"run"`
	}
	if first.Code != http.StatusAccepted || first.Header().Get("Cache-Control") != "no-store" || json.Unmarshal(first.Body.Bytes(), &receipt) != nil || len(receipt.Run.ID) != 67 || len(receipt.Run.ConversationID) != 67 {
		t.Fatal("public admission did not issue owned metadata")
	}
	repeat := post(15, question, "")
	if repeat.Code != http.StatusAccepted || !bytes.Equal(first.Body.Bytes(), repeat.Body.Bytes()) || backend.counts.Load() != 0 || backend.calls.Load() != 0 || evidence.reads.Load() != 0 {
		t.Fatal("public retry changed its frozen receipt or dispatched before worker")
	}
	for _, response := range []*httptest.ResponseRecorder{post(60, question, ""), post(15, "Explain a changed question.", ""), post(15, question, receipt.Run.ConversationID)} {
		if response.Code != http.StatusConflict {
			t.Fatal("changed public request reused retained admission")
		}
	}
	var count int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM assistant_runs`).Scan(&count); err != nil || count != 1 {
		t.Fatal("public retries created extra SQL runs")
	}
	backend.runID = receipt.Run.ID
	done := make(chan error, 1)
	go func() { done <- runtime.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(4 * time.Second):
			t.Error("conversation worker did not join")
		}
	}()
	var snapshot assistantservice.Snapshot
	for {
		snapshot, err = service.Get(ctx, actor, receipt.Run.ID)
		if err != nil {
			t.Fatal("read admitted conversation")
		}
		if snapshot.Run.State.Terminal() && (snapshot.Run.State != assistant.Completed || snapshot.Answer != nil) {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("conversation did not complete within fixture bound")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if snapshot.Run.State != assistant.Completed || snapshot.Answer == nil || backend.counts.Load() != 1 || backend.calls.Load() != 1 || snapshot.Run.UsedUnits != 13 {
		t.Fatal("public conversation did not settle one grounded dispatch")
	}
	if response := post(15, question, ""); response.Code != http.StatusAccepted || strings.Contains(response.Body.String(), "answer") || backend.calls.Load() != 1 {
		t.Fatal("completed public retry dispatched or disclosed text")
	}
	// This helper reads the answer and citation, cancels retained text, and then
	// revokes the SQL actor. The stale synthetic authentication must not suffice.
	reads, live := provider.reads.Load(), provider.live.Load()
	testAssistantSQLStatus(t, ctx, store, actor, handler, receipt.Run.ID, false)
	if provider.reads.Load() != reads || provider.live.Load() != live || backend.calls.Load() != 1 {
		t.Fatal("metadata status replay performed provider or processor work")
	}
	testAssistantHTTPReads(t, ctx, store, actor, handler, logs, receipt.Run.ID)
	testAssistantSQLStatus(t, ctx, store, actor, handler, receipt.Run.ID, true)
	if response := post(15, question, ""); response.Code != http.StatusNotFound || backend.calls.Load() != 1 {
		t.Fatal("revoked public retry restored or dispatched content")
	}
	assertAssistantServiceContentNotStored(t, ctx, store, question, "Explain a changed question.", snapshot.Answer.Sections[0].Text, nonce, "Fixture service")
	for _, marker := range []string{question, "Explain a changed question.", snapshot.Answer.Sections[0].Text, nonce, "Fixture service"} {
		if strings.Contains(logs.String(), marker) {
			t.Fatal("public conversation content entered captured logs")
		}
	}
}
