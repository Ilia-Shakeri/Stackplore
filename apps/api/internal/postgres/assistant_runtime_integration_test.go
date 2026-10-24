package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
)

func assistantFixture(t *testing.T) (*Store, assistant.Actor, time.Time) {
	t.Helper()
	store, principal, at := sourceSettingsFixture(t, time.Duration(0))
	actor := assistant.Actor{UserID: string(principal.UserID), AuthRevision: int64(principal.AuthRevision)}
	var digest []byte
	if err := store.pool.QueryRow(context.Background(), `SELECT token_digest,token_digest_key_id FROM sessions WHERE user_id=$1`, actor.UserID).Scan(&digest, &actor.DigestKeyID); err != nil {
		t.Fatal("read owned session digest reference")
	}
	copy(actor.SessionDigest[:], digest)
	_, err := store.ConfigureAssistant(context.Background(), assistant.Settings{Version: 1, Enabled: true, PolicyVersion: 1, ProfileID: "local_test", ProfileVersion: 1, UserQuota: 1000, InstallationQuota: 10000}, assistantOperator("configure", at))
	if err != nil {
		t.Fatal(err)
	}
	return store, actor, at
}
func assistantOperator(id string, at time.Time) assistant.OperatorAudit {
	return assistant.OperatorAudit{ID: "assistant_audit_" + id, OperatorID: "test_operator", RequestID: "assistant_request_" + id, At: at}
}
func assistantAdmission(actor assistant.Actor, id string, at time.Time) assistant.Admission {
	return assistant.Admission{ID: id, ConversationID: "conversation_" + actor.UserID, Actor: actor, IdempotencyDigest: sha256.Sum256([]byte("test_idempotency_" + id)), Fingerprint: sha256.Sum256([]byte("test_material_" + id)), Goal: assistant.ExplainResource, SourceID: "source_demo", SourceVersion: 1, From: at.Add(-time.Minute), Until: at, PolicyVersion: 1, ProfileID: "local_test", ProfileVersion: 1, ReservedUnits: 100, At: at, Deadline: at.Add(120 * time.Second)}
}
func assistantLease(r assistant.Run, at time.Time) assistant.Lease {
	return assistant.Lease{RunID: r.ID, Worker: r.LeaseOwner, Version: r.Version, Fence: r.Fence, At: at}
}
func assistantAdmitClaim(t *testing.T, store *Store, actor assistant.Actor, id string, at time.Time) assistant.Run {
	t.Helper()
	if _, err := store.AdmitAssistant(context.Background(), assistantAdmission(actor, id, at)); err != nil {
		t.Fatal(err)
	}
	r, ok, err := store.ClaimAssistant(context.Background(), assistant.Claim{Worker: "worker_a", ProcessID: "process_a", At: at.Add(time.Second), Lease: 30 * time.Second})
	if err != nil || !ok {
		t.Fatal("claim failed", err)
	}
	return r
}

func TestAssistantAdmissionSerializesQueueQuotaAndIdempotency(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	var wait sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 12)
	for i := range 12 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := store.AdmitAssistant(ctx, assistantAdmission(actor, fmt.Sprintf("run_%d", i), at))
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		} else if !errors.Is(err, assistant.ErrQuota) {
			t.Fatal(err)
		}
	}
	if wins != 2 {
		t.Fatal("per-owner queue exceeded", wins)
	}
	var queued int
	var reserved int64
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM assistant_runs`).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation'`).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if queued != 2 || reserved != 200 {
		t.Fatal("admission reservation not atomic")
	}
	var id string
	if err := store.pool.QueryRow(ctx, `SELECT id FROM assistant_runs ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	a := assistantAdmission(actor, id, at)
	for range 2 {
		if run, err := store.AdmitAssistant(ctx, a); err != nil || run.ID != id {
			t.Fatal("matching retry did not return original", err)
		}
	}
	a.Fingerprint = sha256.Sum256([]byte("changed_material"))
	if _, err := store.AdmitAssistant(ctx, a); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("changed fingerprint accepted", err)
	}
	a = assistantAdmission(actor, id, at)
	a.Goal = assistant.InvestigateIncident
	if _, err := store.AdmitAssistant(ctx, a); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("same text fingerprint bypassed changed typed goal", err)
	}
}

func TestAssistantConcurrentCancelAndStepResultCommitOneOutcome(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "cancel_race", at)
	run, step, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at.Add(2*time.Second)), Kind: assistant.Inference, MaximumUnits: 100})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, err := store.CancelAssistant(ctx, actor, run.ID, at.Add(3*time.Second))
		results <- err
	}()
	go func() {
		<-start
		_, err := store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(run, at.Add(3*time.Second)), Number: step.Number, Units: 1})
		results <- err
	}()
	close(start)
	for range 2 {
		err := <-results
		if err != nil && !errors.Is(err, assistant.ErrConflict) {
			t.Fatal(err)
		}
	}
	closed, err := store.GetAssistant(ctx, actor, run.ID, at.Add(4*time.Second))
	if err != nil || closed.State != assistant.Cancelled {
		t.Fatal("cancel race resurrected run", err)
	}
	events, err := store.AssistantEvents(ctx, actor, run.ID, 0, 100, at.Add(4*time.Second))
	if err != nil || events[len(events)-1].State != assistant.Cancelled {
		t.Fatal("late publication followed cancellation", err)
	}
	var used, held int64
	if err := store.pool.QueryRow(ctx, `SELECT used_units,reserved_units FROM assistant_usage_accounts WHERE account_key='installation'`).Scan(&used, &held); err != nil || !((used == 0 && held == 100) || (used == 1 && held == 0)) {
		t.Fatal("cancel race double-settled usage")
	}
}

func TestAssistantRevocationFencesPublicationAndRecovery(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "denied_run", at)
	run, step, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at.Add(2*time.Second)), Kind: assistant.Inference, MaximumUnits: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE infrastructure_sources SET enabled=0,row_version=row_version+1,config_revision=config_revision+1,updated_at_us=$1 WHERE id='source_demo'`, at.Add(3*time.Second).UnixMicro()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(run, at.Add(4*time.Second)), Number: step.Number, Units: 1}); !errors.Is(err, assistant.ErrDenied) {
		t.Fatal("denied step result published", err)
	}
	if _, err := store.AssistantEvents(ctx, actor, run.ID, 0, 100, at.Add(4*time.Second)); !errors.Is(err, assistant.ErrDenied) {
		t.Fatal("denied status replayed", err)
	}
	if n, err := store.RecoverAssistant(ctx, at.Add(5*time.Second), 10); err != nil || n != 1 {
		t.Fatal("denied run not fenced", n, err)
	}
	var state string
	if err := store.pool.QueryRow(ctx, `SELECT state FROM assistant_runs WHERE id=$1`, run.ID).Scan(&state); err != nil || state != "uncertain" {
		t.Fatal("denied dispatch did not retain unknown outcome")
	}
}

func TestAssistantReadRecoveryRequeuesAndFencesOldLease(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "read_recovery", at)
	run, step, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at.Add(2*time.Second)), Kind: assistant.Inventory})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE assistant_runs SET lease_until_us=(extract(epoch FROM clock_timestamp())*1000000)::BIGINT-1 WHERE id=$1`, run.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := store.RecoverAssistant(ctx, at.Add(33*time.Second), 10); err != nil || n != 1 {
		t.Fatal("safe read lease not recovered", n, err)
	}
	newRun, ok, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "worker_b", ProcessID: "process_b", At: at.Add(34 * time.Second), Lease: 30 * time.Second})
	if err != nil || !ok || newRun.Fence <= run.Fence {
		t.Fatal("replacement lease not fenced", err)
	}
	if _, err := store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(run, at.Add(35*time.Second)), Number: step.Number}); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("replaced read lease accepted", err)
	}
	newRun, err = store.FinishAssistant(ctx, assistant.Finish{Lease: assistantLease(newRun, at.Add(35*time.Second)), State: assistant.Completed})
	if err != nil || newRun.State != assistant.Completed {
		t.Fatal("safe recovered run failed", err)
	}
	var held int64
	if err := store.pool.QueryRow(ctx, `SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation'`).Scan(&held); err != nil || held != 0 {
		t.Fatal("completed read leaked reservation")
	}
}

func TestAssistantBudgetsBoundCallsAndSettleConfirmedUsage(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "budget_run", at)
	clock := at.Add(2 * time.Second)
	for i := 0; i < assistant.MaximumToolCalls+assistant.MaximumInferenceCalls; i++ {
		kind := assistant.Inventory
		units := int64(0)
		if i >= assistant.MaximumToolCalls {
			kind = assistant.Inference
			units = 1
		}
		var step assistant.Step
		var err error
		run, step, err = store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, clock), Kind: kind, MaximumUnits: units})
		if err != nil {
			t.Fatal(err)
		}
		clock = clock.Add(time.Millisecond)
		run, err = store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(run, clock), Number: step.Number, Units: units})
		if err != nil {
			t.Fatal(err)
		}
		clock = clock.Add(time.Millisecond)
	}
	for _, kind := range []assistant.StepKind{assistant.Inventory, assistant.Inference} {
		maximum := int64(0)
		if kind == assistant.Inference {
			maximum = 1
		}
		if _, _, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, clock), Kind: kind, MaximumUnits: maximum}); !errors.Is(err, assistant.ErrQuota) {
			t.Fatal("call budget exceeded", err)
		}
	}
	closed, err := store.FinishAssistant(ctx, assistant.Finish{Lease: assistantLease(run, clock), State: assistant.Completed})
	if err != nil || closed.UsedUnits != 3 {
		t.Fatal("confirmed usage not finalized", err)
	}
	var used, held int64
	if err := store.pool.QueryRow(ctx, `SELECT used_units,reserved_units FROM assistant_usage_accounts WHERE account_key='installation'`).Scan(&used, &held); err != nil || used != 3 || held != 0 {
		t.Fatal("confirmed usage settlement mismatch")
	}
	countRows := func() int64 {
		t.Helper()
		var n int64
		if err := store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM assistant_runs)+(SELECT count(*) FROM assistant_steps)+(SELECT count(*) FROM assistant_status_events)+(SELECT count(*) FROM assistant_usage_ledger)`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for range 30 {
		before := countRows()
		removed, err := store.PruneAssistant(ctx, at.Add(8*24*time.Hour), 3)
		if err != nil {
			t.Fatal(err)
		}
		after := countRows()
		if removed > 3 || before-after != int64(removed) {
			t.Fatal("retention cascade exceeded aggregate row budget", removed, before-after)
		}
		if removed == 0 {
			break
		}
	}
	if countRows() != 0 {
		t.Fatal("bounded retention failed to finish eligible fixture")
	}
}

func TestAssistantClaimEnforcesTwoGlobalAndOneOwner(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	actors := []assistant.Actor{actor}
	for i := 1; i < 3; i++ {
		user := authRepositoryTestUser(fmt.Sprintf("assistant_user_%d", i), fmt.Sprintf("assistantuser%d", i), auth.RoleViewer, at)
		session := authRepositoryTestSession(t, store, user, fmt.Sprintf("assistant_session_%d", i), fmt.Sprintf("assistant%d", i), at)
		if err := store.withinTransaction(ctx, func(ctx context.Context, tx *transaction) error {
			if err := insertUserTx(ctx, tx.transaction, user); err != nil {
				return err
			}
			return insertSessionTx(ctx, tx.transaction, session)
		}); err != nil {
			t.Fatal("seed additional owned actor")
		}
		a := assistant.Actor{UserID: string(user.ID), AuthRevision: int64(user.AuthRevision)}
		var digest []byte
		if err := store.pool.QueryRow(ctx, `SELECT token_digest,token_digest_key_id FROM sessions WHERE user_id=$1`, a.UserID).Scan(&digest, &a.DigestKeyID); err != nil {
			t.Fatal(err)
		}
		copy(a.SessionDigest[:], digest)
		actors = append(actors, a)
	}
	for i, a := range actors {
		for j := range 2 {
			if _, err := store.AdmitAssistant(ctx, assistantAdmission(a, fmt.Sprintf("owner_%d_run_%d", i, j), at)); err != nil {
				t.Fatal(err)
			}
		}
	}
	var wait sync.WaitGroup
	claimed := make(chan assistant.Run, 8)
	failures := make(chan error, 8)
	for i := range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			r, ok, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: fmt.Sprintf("worker_%d", i), ProcessID: "process_a", At: at.Add(time.Second), Lease: 30 * time.Second})
			if err != nil {
				failures <- err
			}
			if ok {
				claimed <- r
			}
		}()
	}
	wait.Wait()
	close(claimed)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	owners := map[string]bool{}
	n := 0
	for r := range claimed {
		n++
		if owners[r.OwnerID] {
			t.Fatal("owner has two running runs")
		}
		owners[r.OwnerID] = true
	}
	if n != 2 {
		t.Fatal("installation active limit incorrect", n)
	}
}

func TestAssistantCancelFencesPendingInferenceAndLateResult(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "cancel_run", at)
	run, step, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at.Add(2*time.Second)), Kind: assistant.Inference, MaximumUnits: 100})
	if err != nil {
		t.Fatal(err)
	}
	old := assistantLease(run, at.Add(4*time.Second))
	closed, err := store.CancelAssistant(ctx, actor, run.ID, at.Add(3*time.Second))
	if err != nil || closed.State != assistant.Cancelled {
		t.Fatal("cancel failed", err)
	}
	if _, err = store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: old, Number: step.Number, Units: 1}); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("late result accepted", err)
	}
	if _, err = store.CancelAssistant(ctx, actor, run.ID, at.Add(4*time.Second)); err != nil {
		t.Fatal("repeat cancel failed", err)
	}
	events, err := store.AssistantEvents(ctx, actor, run.ID, 0, 100, at.Add(5*time.Second))
	if err != nil || events[len(events)-1].State != assistant.Cancelled {
		t.Fatal("cancel status not durable", err)
	}
	var state string
	var held int64
	if err = store.pool.QueryRow(ctx, `SELECT state,reserved_units FROM assistant_usage_ledger WHERE reservation_id=$1`, run.ID).Scan(&state, &held); err != nil || state != "unknown" || held != 100 {
		t.Fatal("uncertain call lost reservation")
	}
	other := actor
	other.UserID = "other_owner"
	if _, err = store.GetAssistant(ctx, other, run.ID, at.Add(5*time.Second)); !errors.Is(err, assistant.ErrDenied) {
		t.Fatal("cross-owner result readable", err)
	}
}

func TestAssistantLeaseRecoveryDoesNotRepeatUncertainInference(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "lease_run", at)
	run, step, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at.Add(2*time.Second)), Kind: assistant.Inference, MaximumUnits: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE assistant_runs SET lease_until_us=(extract(epoch FROM clock_timestamp())*1000000)::BIGINT-1 WHERE id=$1`, run.ID); err != nil {
		t.Fatal(err)
	}
	n, err := store.RecoverAssistant(ctx, at.Add(33*time.Second), 10)
	if err != nil || n != 1 {
		t.Fatal("lease recovery failed", n, err)
	}
	current, err := store.GetAssistant(ctx, actor, run.ID, at.Add(34*time.Second))
	if err != nil || current.State != assistant.Uncertain {
		t.Fatal("uncertain dispatch resumed", err)
	}
	if _, ok, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "worker_b", ProcessID: "process_b", At: at.Add(34 * time.Second), Lease: time.Second}); err != nil || ok {
		t.Fatal("uncertain call claimed again", err)
	}
	if _, err = store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(run, at.Add(34*time.Second)), Number: step.Number, Units: 1}); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("stale lease completed", err)
	}
}

func TestAssistantVolatileQuestionRequiresOriginalProcess(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	a := assistantAdmission(actor, "volatile_run", at)
	a.Volatile = true
	a.MemoryOwner = "old_process"
	if _, err := store.AdmitAssistant(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "worker", ProcessID: "new_process", At: at.Add(time.Second), Lease: time.Second}); err != nil || ok {
		t.Fatal("lost volatile text reconstructed", err)
	}
	r, err := store.GetAssistant(ctx, actor, a.ID, at.Add(2*time.Second))
	if err != nil || r.State != assistant.Interrupted {
		t.Fatal("lost question not interrupted", err)
	}
}

func TestAssistantUnknownUsageSurvivesPruneAndQuotaReset(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "unknown_run", at)
	run, step, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at.Add(2*time.Second)), Kind: assistant.Inference, MaximumUnits: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(run, at.Add(3*time.Second)), Number: step.Number, Uncertain: true}); err != nil {
		t.Fatal(err)
	}
	later := at.Add(8 * 24 * time.Hour)
	for range 10 {
		n, err := store.PruneAssistant(ctx, later, 3)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	if err = store.ResetAssistantQuota(ctx, assistantOperator("reset", later)); err != nil {
		t.Fatal(err)
	}
	var held int64
	var ledger, runs, audits int
	if err = store.pool.QueryRow(ctx, `SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation'`).Scan(&held); err != nil {
		t.Fatal(err)
	}
	if err = store.pool.QueryRow(ctx, `SELECT count(*) FROM assistant_usage_ledger WHERE state='unknown'`).Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	if err = store.pool.QueryRow(ctx, `SELECT count(*) FROM assistant_runs`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err = store.pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE action LIKE 'assistant.%'`).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if held != 100 || ledger != 1 || runs != 0 || audits != 2 {
		t.Fatal("prune/reset lost unknown usage or audit", held, ledger, runs, audits)
	}
	if _, err = store.pool.Exec(ctx, `UPDATE sessions SET idle_expires_at_us=$1,absolute_expires_at_us=$1`, later.Add(time.Hour).UnixMicro()); err != nil {
		t.Fatal(err)
	}
	retry := assistantAdmission(actor, run.ID, later)
	if _, err = store.AdmitAssistant(ctx, retry); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("unknown tombstone allowed expired retry", err)
	}
	settlement := assistant.UsageSettlement{RunID: run.ID, Units: 100, Conservative: true, ReceiptDigest: sha256.Sum256([]byte("test_receipt")), Audit: assistantOperator("settle", later)}
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() { defer wait.Done(); results <- store.SettleAssistantUsage(ctx, settlement) }()
	}
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	var used int64
	if err = store.pool.QueryRow(ctx, `SELECT used_units,reserved_units FROM assistant_usage_accounts WHERE account_key='installation'`).Scan(&used, &held); err != nil || used != 100 || held != 0 {
		t.Fatal("concurrent settlement not once")
	}
}

func TestAssistantRestoreBlocksPaidAdmissionUntilReconciled(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	settings := assistant.Settings{Version: 2, Enabled: true, PaidEnabled: true, ProfilePaid: true, PolicyVersion: 1, ProfileID: "local_test", ProfileVersion: 1, UserQuota: 1000, InstallationQuota: 10000}
	if _, err := store.ConfigureAssistant(ctx, settings, assistantOperator("paid_config", at)); err != nil {
		t.Fatal(err)
	}
	a := assistantAdmission(actor, "paid_run", at)
	a.Paid = true
	if _, err := store.AdmitAssistant(ctx, a); !errors.Is(err, assistant.ErrDenied) {
		t.Fatal("paid initial proof bypass", err)
	}
	proof := assistant.RestoreSettlement{Generation: 0, ReceiptDigest: sha256.Sum256([]byte("test_initial_usage_receipt")), Audit: assistantOperator("paid_initial", at)}
	if err := store.SettleAssistantRestore(ctx, proof); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdmitAssistant(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE installation_metadata SET restore_generation=1,restored_at_us=$1 WHERE singleton_id=1`, at.UnixMicro()); err != nil {
		t.Fatal(err)
	}
	a = assistantAdmission(actor, "restored_paid", at.Add(time.Second))
	a.Paid = true
	a.RestoreGeneration = 1
	if _, err := store.AdmitAssistant(ctx, a); !errors.Is(err, assistant.ErrDenied) {
		t.Fatal("old balance authorized paid run", err)
	}
	proof = assistant.RestoreSettlement{Generation: 1, ExposureUnits: 500, ReceiptDigest: sha256.Sum256([]byte("test_restored_usage_receipt")), Audit: assistantOperator("paid_restore", at.Add(time.Second))}
	if err := store.SettleAssistantRestore(ctx, proof); err != nil {
		t.Fatal(err)
	}
	if err := store.SettleAssistantRestore(ctx, proof); err != nil {
		t.Fatal("restore proof replay", err)
	}
	if _, err := store.AdmitAssistant(ctx, a); err != nil {
		t.Fatal("reconciled paid run blocked", err)
	}
	var held int64
	if err := store.pool.QueryRow(ctx, `SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation'`).Scan(&held); err != nil || held != 700 {
		t.Fatal("restore reservation omitted or duplicated", held)
	}
}
