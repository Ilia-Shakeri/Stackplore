package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestAssistantAdmissionOutcomeSeparatesCreationFromQueuedAndRunningReuse(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	input := assistantAdmission(actor, "outcome_run", at)
	created, err := store.AdmitAssistantWithOutcome(ctx, input)
	if err != nil || !created.Created || created.Run.ID != input.ID || created.Run.State != assistant.Queued {
		t.Fatal("committed admission did not report creation")
	}
	queued, err := store.AdmitAssistantWithOutcome(ctx, input)
	if err != nil || queued.Created || queued.Run.ID != input.ID || queued.Run.State != assistant.Queued {
		t.Fatal("queued retry did not remain idempotent")
	}
	if _, found, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "outcome_worker", ProcessID: "outcome_process", At: at, Lease: 30 * time.Second}); err != nil || !found {
		t.Fatal("claim outcome fixture")
	}
	input.RequireExisting = true
	running, err := store.AdmitAssistantWithOutcome(ctx, input)
	if err != nil || running.Created || running.Run.State != assistant.Running {
		t.Fatal("running retry incorrectly owns new volatile material")
	}
	legacy, err := store.AdmitAssistant(ctx, input)
	if err != nil || legacy.ID != input.ID || legacy.State != assistant.Running {
		t.Fatal("legacy metadata caller changed behavior")
	}
	var runs, ledger, events int
	var reserved int64
	if err := store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM assistant_runs),(SELECT count(*) FROM assistant_usage_ledger),(SELECT count(*) FROM assistant_status_events),(SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation')`).Scan(&runs, &ledger, &events, &reserved); err != nil || runs != 1 || ledger != 1 || events != 2 || reserved != input.ReservedUnits {
		t.Fatal("retry duplicated durable status or quota")
	}
}

func TestAssistantAdmissionOutcomeIsNeverCreatedWhenCommitFails(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := store.pool.Exec(ctx, `CREATE FUNCTION reject_admission_commit() RETURNS trigger LANGUAGE plpgsql AS $guard$ BEGIN RAISE EXCEPTION 'synthetic_commit_failure'; RETURN NEW; END $guard$`); err != nil {
		t.Fatal("create deferred failure fixture")
	}
	if _, err := store.pool.Exec(ctx, `CREATE CONSTRAINT TRIGGER reject_admission_commit AFTER INSERT ON assistant_status_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_admission_commit()`); err != nil {
		t.Fatal("arm deferred commit failure")
	}
	outcome, err := store.AdmitAssistantWithOutcome(ctx, assistantAdmission(actor, "outcome_commit_failed", at))
	if err == nil || outcome.Created || outcome.Run.ID != "" {
		t.Fatal("uncommitted run reported volatile ownership")
	}
	var rows int
	var reserved int64
	if err := store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM assistant_runs)+(SELECT count(*) FROM assistant_usage_ledger)+(SELECT count(*) FROM assistant_status_events),(SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation')`).Scan(&rows, &reserved); err != nil || rows != 0 || reserved != 0 {
		t.Fatal("failed commit retained rows or reserved usage")
	}
}
