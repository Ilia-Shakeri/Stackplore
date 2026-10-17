package postgres

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/operation"
)

type operationFixture struct {
	store    *Store
	actor    auth.Principal
	request  domain.OperationRequest
	provider *operationProofProvider
	service  *operation.Service
}
type operationProofProvider struct {
	calls   atomic.Int32
	execute func(context.Context, domain.OperationExecution) (domain.OperationProviderResult, error)
}

func (p *operationProofProvider) PreviewOperation(_ context.Context, r domain.OperationRequest) (domain.OperationImpact, error) {
	return domain.OperationImpact{Request: r, DisplayName: "Disposable fixture", EffectCode: "operation.docker_restart", RiskCode: "operation.brief_interruption"}, nil
}
func (p *operationProofProvider) ExecuteOperation(ctx context.Context, e domain.OperationExecution) (domain.OperationProviderResult, error) {
	p.calls.Add(1)
	if p.execute != nil {
		return p.execute(ctx, e)
	}
	return operationProofResult(e), nil
}
func operationProofResult(e domain.OperationExecution) domain.OperationProviderResult {
	return domain.OperationProviderResult{OperationID: e.OperationID, Action: e.Request.Action, ResourceID: e.Request.Resource.ID, PreviousVersion: e.Request.ExpectedVersion, CurrentVersion: "dv_after", EffectCode: "operation.docker_restart_started"}
}
func newOperationFixture(t *testing.T) operationFixture {
	t.Helper()
	store, actor, _ := sourceSettingsFixture(t, 0)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	token, err := store.BeginCapture(ctx, "source_docker", 1, at)
	if err != nil {
		t.Fatal(err)
	}
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at}
	environment := domain.Environment{ID: "environment_operations", SourceID: "source_docker", Provider: domain.ProviderDocker, Name: "Operation fixture", Status: status}
	identity, err := domain.NewResourceIdentity(domain.ProviderDocker, "source_docker", environment.ID, "docker.container", domain.Scope{Kind: domain.ScopeGlobal}, domain.NativeResourceID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveCapture(ctx, domain.CaptureBatch{Token: token, Environments: []domain.Environment{environment}, Resources: []domain.ResourceSnapshot{{Resource: domain.Resource{Ref: ref, DisplayName: "Disposable fixture", Version: "dv_before"}, Status: status}}}); err != nil {
		t.Fatal(err)
	}
	capabilities := domain.CapabilitySet{Source: token.Source}
	for _, name := range domain.CapabilityNames() {
		capabilities.Capabilities = append(capabilities.Capabilities, domain.CapabilityStatus{Name: name, Status: status})
	}
	if err = store.FinishCapture(ctx, domain.CaptureCompletion{Token: token, Status: status, Capabilities: capabilities, Complete: true}); err != nil {
		t.Fatal(err)
	}
	request := domain.OperationRequest{Source: token.Source, Resource: ref, ExpectedVersion: "dv_before", Action: domain.OperationDockerRestart}
	provider := &operationProofProvider{}
	service, err := operation.NewService(operation.ServiceOptions{Repository: store, Providers: []operation.ProviderRegistration{{Source: token.Source, Provider: provider, WriteEnabled: true}}, MutationsEnabled: true, DigestKey: make([]byte, 32), Now: time.Now, ProviderTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return operationFixture{store: store, actor: actor, request: request, provider: provider, service: service}
}
func (f operationFixture) preview(t *testing.T) operation.ConfirmationIssue {
	t.Helper()
	issue, err := f.service.Preview(context.Background(), f.actor, f.request, "request_preview")
	if err != nil {
		t.Fatal(err)
	}
	return issue
}
func (f operationFixture) grant(t *testing.T, suffix string) auth.RecentGrantUse {
	t.Helper()
	at := time.Now().UTC().Truncate(time.Microsecond)
	return issueAuthRepositoryGrant(t, f.store, f.actor, suffix, auth.RecentAuthPurposeOperationExecute, at, at)
}

func TestOperationDurableIntentAtomicGrantAndIdempotentReplay(t *testing.T) {
	f := newOperationFixture(t)
	issue := f.preview(t)
	grant := f.grant(t, "operation_once")
	f.provider.execute = func(ctx context.Context, e domain.OperationExecution) (domain.OperationProviderResult, error) {
		var intent, used, confirmation int
		if err := f.store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM operation_executions WHERE id=$1 AND state='in_progress'),(SELECT count(*) FROM recent_auth_grants WHERE consumed_at_us IS NOT NULL),(SELECT count(*) FROM operation_confirmations)`, e.OperationID).Scan(&intent, &used, &confirmation); err != nil {
			t.Fatal(err)
		}
		if intent != 1 || used != 1 || confirmation != 0 {
			t.Fatal("provider ran before committed intent and one-use gates")
		}
		return operationProofResult(e), nil
	}
	record, err := f.service.Execute(context.Background(), f.actor, f.request, issue.Proof, "browser_operation_once", grant, "request_execute")
	if err != nil || record.State != operation.ExecutionSucceeded {
		t.Fatalf("execute: %v", err)
	}
	// Replay needs neither a live preview nor another recent grant.
	replayed, err := f.service.Execute(context.Background(), f.actor, f.request, "expired_or_absent", "browser_operation_once", auth.RecentGrantUse{}, "request_replay")
	if err != nil || replayed.ID != record.ID || f.provider.calls.Load() != 1 {
		t.Fatal("replay repeated or changed durable result")
	}
	var audits int
	if err = f.store.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_entries WHERE action='infrastructure.operation' AND outcome IN('intent','success')`).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("intent/result audit: %v", err)
	}
	var events int
	if err = f.store.pool.QueryRow(context.Background(), `SELECT count(*) FROM infrastructure_events WHERE origin_kind='system' AND kind='system.operation' AND summary='Infrastructure action completed.' AND resource_id=$1`, string(f.request.Resource.ID)).Scan(&events); err != nil || events != 1 {
		t.Fatalf("single safe timeline outcome after replay: %d, %v", events, err)
	}
	eventPage, eventErr := f.store.ListEvents(context.Background(), domain.EventQuery{Source: f.request.Source, Range: domain.TimeRange{Start: record.CreatedAt.Add(-time.Hour), End: record.UpdatedAt.Add(time.Second)}, Page: domain.PageRequest{Limit: 10}})
	if eventErr != nil || len(eventPage.Events) != 1 || eventPage.Events[0].Kind != "system.operation" {
		t.Fatalf("outcome not readable before next capture: %v", eventErr)
	}
	page, err := f.service.List(context.Background(), f.actor, "source_docker", domain.PageRequest{Limit: 1})
	if err != nil || len(page.Executions) != 1 || page.Executions[0].ID != record.ID {
		t.Fatal("bounded receipt list lost operation")
	}
	changed := f.request
	changed.ExpectedVersion = "dv_other"
	if _, err = f.service.Execute(context.Background(), f.actor, changed, issue.Proof, "browser_operation_once", grant, "request_changed"); !errors.Is(err, operation.ErrIdempotencyConflict) {
		t.Fatalf("changed replay accepted: %v", err)
	}
}

func TestOperationIntentAuditFailureRollsBackConfirmationAndGrant(t *testing.T) {
	f := newOperationFixture(t)
	issue := f.preview(t)
	grant := f.grant(t, "operation_rollback")
	_, err := f.store.pool.Exec(context.Background(), `CREATE FUNCTION reject_operation_intent() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='infrastructure.operation' AND NEW.outcome='intent' THEN RAISE EXCEPTION 'intent rejected'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_operation_intent BEFORE INSERT ON audit_entries FOR EACH ROW EXECUTE FUNCTION reject_operation_intent()`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Execute(context.Background(), f.actor, f.request, issue.Proof, "browser_rollback", grant, "request_rollback"); !errors.Is(err, operation.ErrUnavailable) {
		t.Fatalf("intent rejection: %v", err)
	}
	var confirmations, used, executions int
	if err = f.store.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM operation_confirmations),(SELECT count(*) FROM recent_auth_grants WHERE consumed_at_us IS NOT NULL),(SELECT count(*) FROM operation_executions)`).Scan(&confirmations, &used, &executions); err != nil || confirmations != 1 || used != 0 || executions != 0 || f.provider.calls.Load() != 0 {
		t.Fatalf("failed intent consumed gates: %v", err)
	}
	if _, err = f.store.pool.Exec(context.Background(), `DROP TRIGGER reject_operation_intent ON audit_entries; DROP FUNCTION reject_operation_intent()`); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Execute(context.Background(), f.actor, f.request, issue.Proof, "browser_rollback", grant, "request_recovered"); err != nil || f.provider.calls.Load() != 1 {
		t.Fatalf("rolled back gates unusable: %v", err)
	}
}

func TestOperationConcurrentReplayAndTargetFenceNeverRepeatProvider(t *testing.T) {
	f := newOperationFixture(t)
	issue := f.preview(t)
	grant := f.grant(t, "operation_race")
	entered := make(chan struct{})
	release := make(chan struct{})
	f.provider.execute = func(ctx context.Context, e domain.OperationExecution) (domain.OperationProviderResult, error) {
		close(entered)
		select {
		case <-release:
			return operationProofResult(e), nil
		case <-ctx.Done():
			return domain.OperationProviderResult{}, ctx.Err()
		}
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var first operation.ExecutionRecord
	var firstErr error
	go func() {
		defer wg.Done()
		first, firstErr = f.service.Execute(context.Background(), f.actor, f.request, issue.Proof, "browser_race", grant, "request_race")
	}()
	<-entered
	replay, err := f.service.Execute(context.Background(), f.actor, f.request, issue.Proof, "browser_race", grant, "request_race_replay")
	if !errors.Is(err, operation.ErrInProgress) || replay.ID == "" {
		t.Fatalf("concurrent replay: %v", err)
	}
	second := f.preview(t)
	secondGrant := f.grant(t, "operation_busy")
	if _, err = f.service.Execute(context.Background(), f.actor, f.request, second.Proof, "browser_second", secondGrant, "request_second"); !errors.Is(err, operation.ErrInProgress) {
		t.Fatalf("same target not fenced: %v", err)
	}
	close(release)
	wg.Wait()
	if firstErr != nil || first.ID != replay.ID || f.provider.calls.Load() != 1 {
		t.Fatal("concurrent operation repeated provider")
	}
}

func TestOperationClientCancellationStillPersistsOutcomeAndUnknownNeverReplays(t *testing.T) {
	f := newOperationFixture(t)
	issue := f.preview(t)
	grant := f.grant(t, "operation_cancel")
	ctx, cancel := context.WithCancel(context.Background())
	f.provider.execute = func(call context.Context, e domain.OperationExecution) (domain.OperationProviderResult, error) {
		cancel()
		if call.Err() != nil {
			t.Fatal("request disconnect canceled committed effect context")
		}
		return domain.OperationProviderResult{}, errors.Join(domain.ErrOperationOutcomeUncertain, errors.New("private provider failure"))
	}
	record, err := f.service.Execute(ctx, f.actor, f.request, issue.Proof, "browser_uncertain", grant, "request_uncertain")
	if !errors.Is(err, operation.ErrUncertain) || record.State != operation.ExecutionUncertain {
		t.Fatalf("lost acknowledgement: %v", err)
	}
	if _, err = f.service.Execute(context.Background(), f.actor, f.request, issue.Proof, "browser_uncertain", grant, "request_uncertain_replay"); !errors.Is(err, operation.ErrUncertain) || f.provider.calls.Load() != 1 {
		t.Fatal("uncertain effect resent")
	}
	got, err := f.service.Get(context.Background(), f.actor, record.ID)
	if err != nil || got.State != operation.ExecutionUncertain {
		t.Fatal("detached result audit not durable")
	}
}

func TestOperationExpiredLeaseBecomesUncertainWithoutDispatch(t *testing.T) {
	f := newOperationFixture(t)
	issue := f.preview(t)
	grant := f.grant(t, "operation_orphan")
	f.provider.execute = func(ctx context.Context, e domain.OperationExecution) (domain.OperationProviderResult, error) {
		// Simulate process loss after intent: reject only its result audit.
		_, err := f.store.pool.Exec(ctx, `CREATE FUNCTION reject_operation_result() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='infrastructure.operation' AND NEW.outcome<>'intent' AND NEW.actor_kind='user' THEN RAISE EXCEPTION 'result rejected'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_operation_result BEFORE INSERT ON audit_entries FOR EACH ROW EXECUTE FUNCTION reject_operation_result()`)
		if err != nil {
			t.Fatal(err)
		}
		return operationProofResult(e), nil
	}
	record, err := f.service.Execute(context.Background(), f.actor, f.request, issue.Proof, "browser_orphan", grant, "request_orphan")
	if !errors.Is(err, operation.ErrUncertain) || record.State != operation.ExecutionInProgress {
		t.Fatalf("failed result persistence: %v", err)
	}
	if _, err = f.store.pool.Exec(context.Background(), `DROP TRIGGER reject_operation_result ON audit_entries; DROP FUNCTION reject_operation_result()`); err != nil {
		t.Fatal(err)
	}
	count, err := f.store.RecoverOrphanedOperations(context.Background(), record.LeaseUntil.Add(time.Microsecond), 100)
	if err != nil || count != 1 {
		t.Fatalf("orphan recovery: %v", err)
	}
	got, err := f.service.Get(context.Background(), f.actor, record.ID)
	if err != nil || got.State != operation.ExecutionUncertain {
		t.Fatal("orphan not fenced as uncertain")
	}
	count, err = f.store.RecoverOrphanedOperations(context.Background(), record.LeaseUntil.Add(time.Second), 100)
	if err != nil || count != 0 || f.provider.calls.Load() != 1 {
		t.Fatal("orphan recovery resent or duplicated result")
	}
}
