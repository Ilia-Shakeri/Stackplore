package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestAssistantClaimSkipsLockedCandidateWithoutChangingQuota(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, id := range []string{"claim_a_locked", "claim_b_available"} {
		if _, err := store.AdmitAssistant(ctx, assistantAdmission(actor, id, at)); err != nil {
			t.Fatal("admit bounded claim fixtures")
		}
	}
	locked, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatal("begin owned row-lock fixture")
	}
	defer func() { _ = locked.Rollback(ctx) }()
	if _, err := locked.Exec(ctx, `SELECT id FROM assistant_runs WHERE id='claim_a_locked' FOR UPDATE`); err != nil {
		t.Fatal("lock owned candidate")
	}
	claimContext, stopClaim := context.WithTimeout(ctx, 2*time.Second)
	run, found, err := store.ClaimAssistant(claimContext, assistant.Claim{Worker: "skip_worker", ProcessID: "skip_process", At: at, Lease: assistant.MaximumLeaseDuration})
	stopClaim()
	if err != nil || !found || run.ID != "claim_b_available" {
		t.Fatal("claim waited for or selected a locked candidate")
	}
	var queued, running int
	var reserved int64
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE state='queued'),count(*) FILTER(WHERE state='running'),
(SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation') FROM assistant_runs`).Scan(&queued, &running, &reserved); err != nil || queued != 1 || running != 1 || reserved != 200 {
		t.Fatal("row-lock skip changed admission or quota bounds")
	}
}

func TestAssistantStatusFailureRollsBackAdmissionAndStepTransactions(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := store.pool.Exec(ctx, `CREATE FUNCTION assistant_test_status_failure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'assistant_test_status_boundary'; END; $$`); err != nil {
		t.Fatal("create owned transaction fault fixture")
	}
	fault := func(enabled bool) {
		t.Helper()
		statement := `DROP TRIGGER assistant_test_status_failure ON assistant_status_events`
		if enabled {
			statement = `CREATE TRIGGER assistant_test_status_failure BEFORE INSERT ON assistant_status_events FOR EACH STATEMENT EXECUTE FUNCTION assistant_test_status_failure()`
		}
		if _, err := store.pool.Exec(ctx, statement); err != nil {
			t.Fatal("change owned transaction fault fixture")
		}
	}
	fault(true)
	if _, err := store.AdmitAssistant(ctx, assistantAdmission(actor, "rollback_admission", at)); !errors.Is(err, assistant.ErrUnavailable) {
		t.Fatal("admission fault was hidden")
	}
	var runs, ledger, events int
	var reserved int64
	if err := store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM assistant_runs),(SELECT count(*) FROM assistant_usage_ledger),
(SELECT count(*) FROM assistant_status_events),(SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation')`).Scan(&runs, &ledger, &events, &reserved); err != nil || runs != 0 || ledger != 0 || events != 0 || reserved != 0 {
		t.Fatal("failed admission committed a run, event or reservation")
	}
	fault(false)
	run := assistantAdmitClaim(t, store, actor, "rollback_step", at)
	fault(true)
	start := assistant.StepStart{Lease: assistantLease(run, at), Kind: assistant.Inference, MaximumUnits: 100}
	if _, _, err := store.BeginAssistantStep(ctx, start); !errors.Is(err, assistant.ErrUnavailable) {
		t.Fatal("step-start fault was hidden")
	}
	var steps int
	var version int64
	if err := store.pool.QueryRow(ctx, `SELECT row_version,(SELECT count(*) FROM assistant_steps WHERE run_id=$1),inference_calls FROM assistant_runs WHERE id=$1`, run.ID).Scan(&version, &steps, &events); err != nil || version != run.Version || steps != 0 || events != 0 {
		t.Fatal("failed step-start committed metadata or consumed a call")
	}
	fault(false)
	run, step, err := store.BeginAssistantStep(ctx, start)
	if err != nil {
		t.Fatal("valid step did not recover after rollback")
	}
	fault(true)
	finish := assistant.StepFinish{Lease: assistantLease(run, at), Number: step.Number, Units: 10}
	if _, err := store.FinishAssistantStep(ctx, finish); !errors.Is(err, assistant.ErrUnavailable) {
		t.Fatal("step-finish fault was hidden")
	}
	var state string
	var units int64
	if err := store.pool.QueryRow(ctx, `SELECT run.row_version,run.used_units,step.state FROM assistant_runs AS run
JOIN assistant_steps AS step ON step.run_id=run.id WHERE run.id=$1 AND step.step_number=$2`, run.ID, step.Number).Scan(&version, &units, &state); err != nil || version != run.Version || units != 0 || state != "running" {
		t.Fatal("failed step-finish partially committed usage or status")
	}
	fault(false)
	run, err = store.FinishAssistantStep(ctx, finish)
	if err != nil {
		t.Fatal("valid step-finish did not recover after rollback")
	}
	fault(true)
	if _, err := store.FinishAssistant(ctx, assistant.Finish{Lease: assistantLease(run, at), State: assistant.Completed}); !errors.Is(err, assistant.ErrUnavailable) {
		t.Fatal("run-finish fault was hidden")
	}
	var ledgerState string
	if err := store.pool.QueryRow(ctx, `SELECT run.state,ledger.state,(SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation')
FROM assistant_runs AS run JOIN assistant_usage_ledger AS ledger ON ledger.reservation_id=run.id WHERE run.id=$1`, run.ID).Scan(&state, &ledgerState, &reserved); err != nil || state != "running" || ledgerState != "held" || reserved != 100 {
		t.Fatal("failed run-finish released its reservation")
	}
	fault(false)
	if _, err := store.FinishAssistant(ctx, assistant.Finish{Lease: assistantLease(run, at), State: assistant.Completed}); err != nil {
		t.Fatal("valid run-finish did not recover after rollback")
	}
}
