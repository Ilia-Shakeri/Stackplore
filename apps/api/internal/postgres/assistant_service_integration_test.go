package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
	"github.com/jackc/pgx/v5"
)

type assistantServiceEvidence struct {
	reader     *assistantevidence.Reader
	admissions atomic.Int32
	metadata   atomic.Int32
	evidence   atomic.Int32
	reads      atomic.Int32
}

func (e *assistantServiceEvidence) AuthorizeAdmission(ctx context.Context, actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope) error {
	e.admissions.Add(1)
	return e.reader.AuthorizeAdmission(ctx, actor, run, scope)
}

func (e *assistantServiceEvidence) Authorize(ctx context.Context, actor assistant.Actor, run assistant.Run, scope contextbuilder.Scope) error {
	e.metadata.Add(1)
	return e.reader.Authorize(ctx, actor, run, scope)
}

func (e *assistantServiceEvidence) AuthorizeEvidence(ctx context.Context, actor assistant.Actor, run assistant.Run, evidence assistantevidence.Evidence) error {
	e.evidence.Add(1)
	return e.reader.AuthorizeEvidence(ctx, actor, run, evidence)
}

func (e *assistantServiceEvidence) Read(ctx context.Context, request assistantevidence.ReadRequest) (assistantevidence.Evidence, error) {
	e.reads.Add(1)
	return e.reader.Read(ctx, request)
}

// The provider and live check are controlled in-memory fixtures. Only metadata
// authority and reservation visibility below use real PostgreSQL transactions.
type assistantServiceProvider struct {
	ref      domain.ResourceRef
	reads    atomic.Int32
	live     atomic.Int32
	unwanted atomic.Int32
}

func (p *assistantServiceProvider) Check(ctx context.Context, scope contextbuilder.Scope, kind assistant.StepKind, consent bool) error {
	if ctx.Err() != nil || !scope.Source.Matches(p.ref) || len(scope.Resources) != 1 || scope.Resources[0] != p.ref || kind != assistant.Inventory || consent {
		return assistantevidence.ErrDenied
	}
	p.live.Add(1)
	return nil
}

func assistantServiceStatus() domain.ObservationStatus {
	return domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: time.Now().UTC().Truncate(time.Microsecond)}
}

func (p *assistantServiceProvider) GetResource(ctx context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	if ctx.Err() != nil || query.Validate() != nil || !query.Source.Matches(p.ref) || query.ResourceID != p.ref.ID {
		return domain.ResourceSnapshot{}, assistantevidence.ErrDenied
	}
	p.reads.Add(1)
	return domain.ResourceSnapshot{Resource: domain.Resource{Ref: p.ref, DisplayName: "Fixture service", Synthetic: true}, Status: assistantServiceStatus(), Health: domain.HealthHealthy}, nil
}

func (p *assistantServiceProvider) GetCapabilities(ctx context.Context, source domain.ProviderRef) (domain.CapabilitySet, error) {
	if ctx.Err() != nil || !source.Matches(p.ref) {
		return domain.CapabilitySet{}, assistantevidence.ErrDenied
	}
	result := domain.CapabilitySet{Source: source}
	for _, name := range domain.CapabilityNames() {
		status := assistantServiceStatus()
		if name != domain.CapabilityInventoryDetail {
			status.State, status.ReasonCode = domain.AvailabilityUnsupported, "fixture.unsupported"
		}
		result.Capabilities = append(result.Capabilities, domain.CapabilityStatus{Name: name, Status: status})
	}
	return result, nil
}

func (p *assistantServiceProvider) GetHealth(context.Context, domain.HealthQuery) (domain.HealthResult, error) {
	p.unwanted.Add(1)
	return domain.HealthResult{}, assistantevidence.ErrDenied
}
func (p *assistantServiceProvider) GetMetricHistory(context.Context, domain.MetricHistoryQuery) (domain.MetricHistoryResult, error) {
	p.unwanted.Add(1)
	return domain.MetricHistoryResult{}, assistantevidence.ErrDenied
}
func (p *assistantServiceProvider) ListEvents(context.Context, domain.EventQuery) (domain.EventPage, error) {
	p.unwanted.Add(1)
	return domain.EventPage{}, assistantevidence.ErrDenied
}
func (p *assistantServiceProvider) GetRelationships(context.Context, domain.TopologyQuery) (domain.RelationshipPage, error) {
	p.unwanted.Add(1)
	return domain.RelationshipPage{}, assistantevidence.ErrDenied
}

type assistantServiceBackend struct {
	store    *Store
	delegate inference.Backend
	runID    string
	mu       sync.Mutex
	ref      string
	counts   atomic.Int32
	calls    atomic.Int32
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
	if b.delegate != nil {
		return b.delegate.Count(ctx, request)
	}
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
	if b.delegate != nil {
		return b.delegate.Generate(ctx, input, output)
	}
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
	for _, wire := range []bool{false, true} {
		name := "synthetic_backend"
		if wire {
			name = "bounded_http_adapter"
		}
		t.Run(name, func(t *testing.T) { testAssistantServiceGroundedDispatch(t, wire, false) })
	}
	t.Run("private_http_projection", func(t *testing.T) { testAssistantServiceGroundedDispatch(t, true, true) })
}

func testAssistantServiceGroundedDispatch(t *testing.T, wire, transport bool) {
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
	if transport {
		admission.ID = "ar_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		admission.ConversationID = "ac_cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
		admission.From = admission.Until.Add(-15 * time.Minute)
	}
	admission.ResourceID, admission.ReservedUnits = string(ref.ID), 16128
	backend := &assistantServiceBackend{store: store, runID: admission.ID}
	if wire {
		backend.delegate = newAssistantWireFixture(t, backend)
	}
	gateway, err := inference.New(backend, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	provider := &assistantServiceProvider{ref: ref}
	reader, err := assistantevidence.New(assistantevidence.Options{Gate: assistantevidence.RepositoryGate{Repository: store}, Now: time.Now, Sources: []assistantevidence.Source{{Reference: source, ConfigurationVersion: 1, Read: provider, Live: provider}}})
	if err != nil {
		t.Fatal("construct SQL-bound evidence reader")
	}
	evidence := &assistantServiceEvidence{reader: reader}
	key := make([]byte, 32)
	key[0] = 1
	service, err := assistantservice.New(assistantservice.Options{Repository: store, Gateway: gateway, Authorizer: evidence, Collector: evidence, Lifetime: ctx, Now: time.Now, ProcessID: "sql_fixture_process", FingerprintKey: key, Workers: 1, PollInterval: 100 * time.Millisecond, Heartbeat: time.Second, OutputLimit: 128})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	input := assistantservice.Input{Admission: admission, Scope: contextbuilder.Scope{Source: source, ConfigurationVersion: 1, PolicyVersion: 1, Range: domain.TimeRange{Start: admission.From, End: admission.Until}, Resources: []domain.ResourceRef{ref}}, Locale: "en", Question: "Explain the selected resource."}
	run, err := service.Submit(ctx, input)
	if err != nil || run.State != assistant.Queued || backend.calls.Load() != 0 {
		t.Fatal("admission performed work or failed", err)
	}
	queued, err := service.Get(ctx, actor, run.ID)
	if err != nil || queued.Run.State != assistant.Queued || queued.Answer != nil || len(queued.Citations) != 0 || evidence.reads.Load() != 0 || provider.reads.Load() != 0 || provider.live.Load() != 0 || backend.calls.Load() != 0 {
		t.Fatal("queued metadata poll performed a read or discarded accepted work")
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
	// Exercise completed replay explicitly; scheduler-dependent poll counts do
	// not prove that saved content rechecks its authority.
	snapshot, err = service.Get(ctx, actor, run.ID)
	if err != nil {
		t.Fatal("completed replay failed")
	}
	if snapshot.Run.State != assistant.Completed || snapshot.Answer == nil || len(snapshot.Citations) != 1 || len(snapshot.Answer.Sections) != 1 || len(snapshot.Answer.Sections[0].Citations) != 1 || snapshot.Answer.Sections[0].Citations[0] != snapshot.Citations[0].ID || snapshot.Citations[0].ResourceID != ref.ID || snapshot.Citations[0].ConfigurationVersion != 1 || snapshot.Citations[0].PolicyVersion != 1 || snapshot.Citations[0].RestoreGeneration != 0 || backend.counts.Load() != 1 || backend.calls.Load() != 1 || evidence.reads.Load() != 1 || evidence.admissions.Load() != 1 || evidence.metadata.Load() < 2 || evidence.evidence.Load() < 3 || provider.reads.Load() < 3 || provider.live.Load() < 3 || provider.unwanted.Load() != 0 {
		t.Fatalf("grounded result rejected: state=%s answer=%t citations=%d count=%d generate=%d read=%d admission=%d metadata=%d evidence=%d provider=%d live=%d unwanted=%d", snapshot.Run.State, snapshot.Answer != nil, len(snapshot.Citations), backend.counts.Load(), backend.calls.Load(), evidence.reads.Load(), evidence.admissions.Load(), evidence.metadata.Load(), evidence.evidence.Load(), provider.reads.Load(), provider.live.Load(), provider.unwanted.Load())
	}
	if snapshot.Run.UsedUnits != 13 || snapshot.Run.ToolCalls != 1 || snapshot.Run.InferenceCalls != 1 || snapshot.Run.UsageUncertain || snapshot.Run.UsageConservative {
		t.Fatal("observed usage or step counters drifted")
	}
	var held, used, settled int64
	var ledgerState string
	if err := store.pool.QueryRow(ctx, `SELECT account.reserved_units,account.used_units,ledger.settled_units,ledger.state FROM assistant_usage_accounts AS account CROSS JOIN assistant_usage_ledger AS ledger WHERE account.account_key='installation' AND ledger.reservation_id=$1`, run.ID).Scan(&held, &used, &settled, &ledgerState); err != nil || held != 0 || used != 13 || settled != 13 || ledgerState != "settled" {
		t.Fatal("committed usage did not release unused reservation")
	}
	assertAssistantServiceContentNotStored(t, ctx, store, input.Question, snapshot.Answer.Sections[0].Text, "Fixture service")
	if repeated, err := service.Submit(ctx, input); err != nil || repeated.ID != run.ID || backend.calls.Load() != 1 {
		t.Fatal("idempotent submission dispatched again", err)
	}
	input.Question = "Explain a changed question."
	if _, err := service.Submit(ctx, input); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("changed question reused a committed run", err)
	}
	if transport {
		testAssistantHTTPReplay(t, ctx, store, actor, service, run.ID)
		if backend.calls.Load() != 1 || backend.counts.Load() != 1 {
			t.Fatal("private HTTP replay/cancel repeated content dispatch")
		}
		assertAssistantServiceContentNotStored(t, ctx, store, "Explain the selected resource.", input.Question, snapshot.Answer.Sections[0].Text, "Fixture service")
		return
	}
	if _, err := store.pool.Exec(ctx, `UPDATE infrastructure_sources SET enabled=0 WHERE id='source_demo'`); err != nil {
		t.Fatal("revoke owned SQL source fixture")
	}
	if denied, err := service.Get(ctx, actor, run.ID); err == nil || denied.Answer != nil || len(denied.Citations) != 0 || backend.calls.Load() != 1 {
		t.Fatal("revoked SQL source released a saved answer or repeated dispatch")
	}
}

// Inspect every fixture table, including audit and observation tables. Report
// only a fixed failure, never row contents. This proves SQL absence for this
// synthetic journey; it does not prove deployed logs or model quality.
func assertAssistantServiceContentNotStored(t *testing.T, ctx context.Context, store *Store, markers ...string) {
	t.Helper()
	rows, err := store.pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname=current_schema() ORDER BY tablename`)
	if err != nil {
		t.Fatal("list owned privacy fixture tables")
	}
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal("read owned table name")
		}
		tables = append(tables, table)
	}
	rows.Close()
	if rows.Err() != nil || len(tables) == 0 || len(tables) > 128 {
		t.Fatal("privacy fixture table bound failed")
	}
	for _, table := range tables {
		var body []byte
		var count int
		query := `SELECT count(*),COALESCE(jsonb_agg(to_jsonb(fixture)), '[]'::jsonb) FROM (SELECT * FROM ` + pgx.Identifier{table}.Sanitize() + ` LIMIT 257) AS fixture`
		if err := store.pool.QueryRow(ctx, query).Scan(&count, &body); err != nil || count > 256 || len(body) > 1<<20 {
			t.Fatal("owned privacy fixture read failed or exceeded bound")
		}
		for _, marker := range markers {
			if marker == "" || bytes.Contains(body, []byte(marker)) {
				clear(body)
				t.Fatal("private journey content persisted in owned SQL fixture")
			}
		}
		clear(body)
	}
}
