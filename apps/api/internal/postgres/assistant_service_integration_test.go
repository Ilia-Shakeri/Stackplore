package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type assistantServiceAuthorizer struct {
	actor assistant.Actor
	calls atomic.Int32
}

func (a *assistantServiceAuthorizer) Authorize(ctx context.Context, actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope) error {
	a.calls.Add(1)
	if ctx.Err() != nil || actor != a.actor || run.OwnerID != actor.UserID || run.SourceID != "source_demo" || scope.Source.SourceID != "source_demo" || scope.ConfigurationVersion != 1 || scope.PolicyVersion != 1 || scope.RestoreGeneration != 0 {
		return assistant.ErrDenied
	}
	return nil
}

type assistantServiceCollector struct{ calls atomic.Int32 }

func (c *assistantServiceCollector) Collect(ctx context.Context, scope contextbuilder.Scope, kind assistant.StepKind) (contextbuilder.Request, error) {
	if ctx.Err() != nil || kind != assistant.Inventory || len(scope.Resources) != 1 {
		return contextbuilder.Request{}, assistant.ErrDenied
	}
	c.calls.Add(1)
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: scope.ObservedAt}
	return contextbuilder.Request{Scope: scope, Destination: contextbuilder.Local, Status: status, Resources: []domain.ResourceSnapshot{{Resource: domain.Resource{Ref: scope.Resources[0], DisplayName: "Fixture service", Synthetic: true}, Status: status, Health: domain.HealthHealthy}}}, nil
}

type assistantServiceBackend struct {
	store  *Store
	runID  string
	mu     sync.Mutex
	ref    string
	counts atomic.Int32
	calls  atomic.Int32
}

// The synthetic backend checks committed reservation state from another pooled
// connection before any dispatch. It is not a real inference-engine proof.
func (b *assistantServiceBackend) reserved(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, err := b.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var version int64
	if err = tx.QueryRow(ctx, `SELECT row_version FROM assistant_control WHERE singleton_id=1 FOR UPDATE`).Scan(&version); err != nil {
		return err
	}
	var held, maximum int64
	var running int
	if err = tx.QueryRow(ctx, `SELECT account.reserved_units,step.maximum_units,(SELECT count(*) FROM assistant_steps WHERE run_id=$1 AND state='running') FROM assistant_usage_accounts AS account CROSS JOIN assistant_steps AS step WHERE account.account_key='installation' AND step.run_id=$1 AND step.kind='inference' AND step.state='running'`, b.runID).Scan(&held, &maximum, &running); err != nil {
		return err
	}
	if held != 16128 || maximum != 16128 || running != 1 {
		return errors.New("dispatch reservation was not committed")
	}
	return tx.Commit(ctx)
}

func (b *assistantServiceBackend) Count(ctx context.Context, request inference.Request) (inference.CountedInput, error) {
	if err := b.reserved(ctx); err != nil {
		return inference.CountedInput{}, err
	}
	if len(request.Evidence) != 1 || len(request.Evidence[0].ID) != 67 {
		return inference.CountedInput{}, inference.ErrInvalid
	}
	b.counts.Add(1)
	b.mu.Lock()
	b.ref = request.Evidence[0].ID
	b.mu.Unlock()
	return inference.CountedInput{Tokens: []int32{1, 2, 3}, Bytes: len(request.Question) + len(request.Evidence[0].Text), Revision: "synthetic_service_revision"}, nil
}

func (b *assistantServiceBackend) Generate(ctx context.Context, input inference.CountedInput, output int) (inference.Generated, error) {
	if err := b.reserved(ctx); err != nil {
		return inference.Generated{}, err
	}
	if output != 128 {
		return inference.Generated{}, inference.ErrInvalid
	}
	b.calls.Add(1)
	b.mu.Lock()
	ref := b.ref
	b.mu.Unlock()
	contents, err := json.Marshal(inference.Answer{Sections: []inference.Section{{Kind: inference.SectionFacts, Text: "Health is healthy.", Citations: []string{ref}}}})
	if err != nil {
		return inference.Generated{}, err
	}
	return inference.Generated{Content: contents, InputTokens: len(input.Tokens), OutputTokens: 10, Complete: true, Revision: input.Revision}, nil
}

func TestAssistantServiceCommitsReservationBeforeOneGroundedLocalDispatch(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := store.ConfigureAssistant(ctx, assistant.Settings{Version: 2, Enabled: true, PolicyVersion: 1, ProfileID: "local_test", ProfileVersion: 1, UserQuota: 100000, InstallationQuota: 1000000}, assistantOperator("service_quota", at)); err != nil {
		t.Fatal(err)
	}
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "source_demo"}
	identity, err := domain.NewResourceIdentity(source.Provider, source.SourceID, "environment", "demo.service", domain.Scope{Kind: domain.ScopeProject, Name: "fixture"}, "native_fixture")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	admission := assistantAdmission(actor, "service_sql_run", at)
	admission.ResourceID, admission.ReservedUnits = string(ref.ID), 16128
	backend := &assistantServiceBackend{store: store, runID: admission.ID}
	gateway, err := inference.New(backend, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	authorizer, collector := &assistantServiceAuthorizer{actor: actor}, &assistantServiceCollector{}
	key := make([]byte, 32)
	key[0] = 1
	service, err := assistantservice.New(assistantservice.Options{Repository: store, Gateway: gateway, Authorizer: authorizer, Collector: collector, Lifetime: ctx, Now: time.Now, ProcessID: "sql_fixture_process", FingerprintKey: key, Workers: 1, PollInterval: 100 * time.Millisecond, Heartbeat: time.Second, OutputLimit: 128})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	input := assistantservice.Input{Admission: admission, Scope: contextbuilder.Scope{Source: source, ConfigurationVersion: 1, PolicyVersion: 1, Range: domain.TimeRange{Start: admission.From, End: admission.Until}, Resources: []domain.ResourceRef{ref}}, Locale: "en", Question: "Explain the selected resource."}
	run, err := service.Submit(ctx, input)
	if err != nil || run.State != assistant.Queued || backend.calls.Load() != 0 {
		t.Fatal("admission performed work or failed", err)
	}
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(4 * time.Second):
			t.Error("service left a worker after cancellation")
		}
	}()
	var snapshot assistantservice.Snapshot
	for {
		snapshot, err = service.Get(ctx, actor, run.ID)
		if err != nil {
			t.Fatal("read admitted run", err)
		}
		if snapshot.Run.State.Terminal() && (snapshot.Run.State != assistant.Completed || snapshot.Answer != nil) {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("bounded local explanation did not complete")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if snapshot.Run.State != assistant.Completed || snapshot.Answer == nil || len(snapshot.Citations) != 1 || len(snapshot.Answer.Sections) != 1 || len(snapshot.Answer.Sections[0].Citations) != 1 || snapshot.Answer.Sections[0].Citations[0] != snapshot.Citations[0].ID || snapshot.Citations[0].ResourceID != ref.ID || snapshot.Citations[0].ConfigurationVersion != 1 || snapshot.Citations[0].PolicyVersion != 1 || snapshot.Citations[0].RestoreGeneration != 0 || backend.counts.Load() != 1 || backend.calls.Load() != 1 || collector.calls.Load() != 1 || authorizer.calls.Load() < 5 {
		t.Fatal("single grounded local result was not published")
	}
	if snapshot.Run.UsedUnits != 13 || snapshot.Run.ToolCalls != 1 || snapshot.Run.InferenceCalls != 1 || snapshot.Run.UsageUncertain || snapshot.Run.UsageConservative {
		t.Fatal("observed usage or step counters drifted")
	}
	var held, used, settled int64
	var ledgerState string
	if err := store.pool.QueryRow(ctx, `SELECT account.reserved_units,account.used_units,ledger.settled_units,ledger.state FROM assistant_usage_accounts AS account CROSS JOIN assistant_usage_ledger AS ledger WHERE account.account_key='installation' AND ledger.reservation_id=$1`, run.ID).Scan(&held, &used, &settled, &ledgerState); err != nil || held != 0 || used != 13 || settled != 13 || ledgerState != "settled" {
		t.Fatal("committed usage did not release unused reservation")
	}
	if repeated, err := service.Submit(ctx, input); err != nil || repeated.ID != run.ID || backend.calls.Load() != 1 {
		t.Fatal("idempotent submission dispatched again", err)
	}
	input.Question = "Explain a changed question."
	if _, err := service.Submit(ctx, input); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("changed question reused a committed run", err)
	}
}
