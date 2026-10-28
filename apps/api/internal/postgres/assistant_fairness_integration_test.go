package postgres

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type assistantNoDispatchBackend struct{ attempts atomic.Int32 }

func (backend *assistantNoDispatchBackend) Count(context.Context, inference.Request) (inference.CountedInput, error) {
	backend.attempts.Add(1)
	return inference.CountedInput{}, inference.ErrUnavailable
}
func (backend *assistantNoDispatchBackend) Generate(context.Context, inference.CountedInput, int) (inference.Generated, error) {
	backend.attempts.Add(1)
	return inference.Generated{}, inference.ErrUnavailable
}

func TestAssistantServiceTerminalHistoryCannotExhaustAnotherOwnersAdmission(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := store.ConfigureAssistant(ctx, assistant.Settings{Version: 2, Enabled: true, PolicyVersion: 1, ProfileID: "local_test", ProfileVersion: 1, UserQuota: 100000, InstallationQuota: 1000000}, assistantOperator("fairness_quota", at)); err != nil {
		t.Fatal("configure owned fairness fixture")
	}
	user := authRepositoryTestUser("fairness_other", "fairnessother", auth.RoleViewer, at)
	session := authRepositoryTestSession(t, store, user, "fairness_session", "fairnessfixture", at)
	if err := store.withinTransaction(ctx, func(ctx context.Context, tx *transaction) error {
		if err := insertUserTx(ctx, tx.transaction, user); err != nil {
			return err
		}
		return insertSessionTx(ctx, tx.transaction, session)
	}); err != nil {
		t.Fatal("seed other owned fairness actor")
	}
	other := assistant.Actor{UserID: string(user.ID), AuthRevision: int64(user.AuthRevision)}
	var digest []byte
	if err := store.pool.QueryRow(ctx, `SELECT token_digest,token_digest_key_id FROM sessions WHERE user_id=$1`, other.UserID).Scan(&digest, &other.DigestKeyID); err != nil || len(digest) != len(other.SessionDigest) {
		t.Fatal("read other owned fairness digest reference")
	}
	copy(other.SessionDigest[:], digest)
	clear(digest)
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "source_demo"}
	identity, err := domain.NewResourceIdentity(source.Provider, source.SourceID, "environment", "demo.service", domain.Scope{Kind: domain.ScopeProject, Name: "fixture"}, "native_fixture")
	if err != nil {
		t.Fatal("create normalized fairness resource")
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal("create normalized fairness reference")
	}
	provider := &assistantServiceProvider{ref: ref}
	reader, err := assistantevidence.New(assistantevidence.Options{Gate: assistantevidence.RepositoryGate{Repository: store}, Now: time.Now, Sources: []assistantevidence.Source{{Reference: source, ConfigurationVersion: 1, Read: provider, Live: provider}}})
	if err != nil {
		t.Fatal("construct metadata-bound fairness reader")
	}
	evidence := &assistantServiceEvidence{reader: reader}
	backend := &assistantNoDispatchBackend{}
	gateway, err := inference.New(backend, time.Now)
	if err != nil {
		t.Fatal("construct non-dispatch fairness fixture")
	}
	key := [32]byte{1}
	service, err := assistantservice.New(assistantservice.Options{Repository: store, Gateway: gateway, Authorizer: evidence, Collector: evidence, Lifetime: ctx, Now: time.Now, ProcessID: "fairness_process", FingerprintKey: key[:], Workers: 1, PollInterval: 100 * time.Millisecond, Heartbeat: time.Second, OutputLimit: 128})
	if err != nil {
		t.Fatal("construct fairness service")
	}
	defer service.Close()
	input := func(owner assistant.Actor, index int) assistantservice.Input {
		a := assistantAdmission(owner, fmt.Sprintf("fairness_run_%d", index), at)
		a.ConversationID = fmt.Sprintf("fairness_conversation_%d", index/assistantservice.MaximumTurns)
		a.ResourceID, a.ReservedUnits = string(ref.ID), 16128
		return assistantservice.Input{Admission: a, Scope: contextbuilder.Scope{Source: source, ConfigurationVersion: 1, PolicyVersion: 1, Range: domain.TimeRange{Start: a.From, End: a.Until}, Resources: []domain.ResourceRef{ref}}, Locale: "en", Question: "Explain the selected resource."}
	}
	first := input(actor, 0)
	for index := range assistantservice.MaximumMemoryRuns {
		request := input(actor, index)
		if run, err := service.Submit(ctx, request); err != nil || run.State != assistant.Queued {
			t.Fatal("sequential owner admission failed")
		}
		if run, err := service.Cancel(ctx, actor, request.Admission.ID); err != nil || run.State != assistant.Cancelled {
			t.Fatal("durable cancellation failed to release owner quota")
		}
	}
	request := input(other, 30)
	request.Admission.ConversationID = "other_fairness_conversation"
	if run, err := service.Submit(ctx, request); err != nil || run.State != assistant.Queued {
		t.Fatal("terminal history blocked another real owner")
	}
	if run, err := service.Submit(ctx, first); err != nil || run.State != assistant.Cancelled {
		t.Fatal("old retry did not return its original durable state")
	}
	snapshot, err := service.Get(ctx, actor, first.Admission.ID)
	if err != nil || snapshot.Volatile || snapshot.Answer != nil || len(snapshot.Citations) != 0 || snapshot.Run.State != assistant.Cancelled {
		t.Fatal("old retry restored lost private material")
	}
	var runs, queued, ledger int
	var held, used int64
	if err := store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM assistant_runs),(SELECT count(*) FROM assistant_runs WHERE state='queued'),(SELECT count(*) FROM assistant_usage_ledger),(SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation'),(SELECT used_units FROM assistant_usage_accounts WHERE account_key='installation')`).Scan(&runs, &queued, &ledger, &held, &used); err != nil || runs != assistantservice.MaximumMemoryRuns+1 || queued != 1 || ledger != runs || held != 16128 || used != 0 {
		t.Fatal("retry or terminal history changed durable quota, ledger or admission")
	}
	if backend.attempts.Load() != 0 || evidence.reads.Load() != 0 || provider.reads.Load() != 0 || provider.live.Load() != 0 {
		t.Fatal("metadata-only fairness fixture performed provider work")
	}
}
