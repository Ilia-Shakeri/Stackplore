package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestAssistantOldCallerTimeCannotExtendExpiredAuthority(t *testing.T) {
	for _, kind := range []string{"lease", "deadline", "session"} {
		t.Run(kind, func(t *testing.T) {
			store, actor, at := assistantFixture(t)
			ctx := context.Background()
			run := assistantAdmitClaim(t, store, actor, "clock_run", at)
			statement := `UPDATE assistant_runs SET lease_until_us=created_at_us WHERE id=$1`
			if kind == "deadline" {
				statement = `UPDATE assistant_runs SET deadline_us=created_at_us+1 WHERE id=$1`
			}
			if kind == "session" {
				statement = `UPDATE sessions SET idle_expires_at_us=created_at_us+1 WHERE user_id=$1`
			}
			id := run.ID
			if kind == "session" {
				id = actor.UserID
			}
			if _, err := store.pool.Exec(ctx, statement, id); err != nil {
				t.Fatal("expire owned authority fixture")
			}
			_, _, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at), Kind: assistant.Inventory})
			want := assistant.ErrConflict
			if kind == "session" {
				want = assistant.ErrDenied
			}
			if !errors.Is(err, want) {
				t.Fatal("old caller clock extended expired authority", err)
			}
			if _, err := store.RenewAssistantLease(ctx, assistantLease(run, at), time.Second); !errors.Is(err, want) {
				t.Fatal("old caller clock renewed expired authority", err)
			}
			if kind == "deadline" {
				closed, err := store.FinishAssistant(ctx, assistant.Finish{Lease: assistantLease(run, at), State: assistant.Expired})
				if err != nil || closed.State != assistant.Expired {
					t.Fatal("expired worker could not close safely", err)
				}
			}
		})
	}
}

func TestAssistantInferenceRequiresReservedRemainingUnitsBeforeDispatch(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "reserved_run", at)
	run, step, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at), Kind: assistant.Inference, MaximumUnits: 100})
	if err != nil {
		t.Fatal(err)
	}
	run, err = store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(run, at), Number: step.Number, Units: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at), Kind: assistant.Inference, MaximumUnits: 1}); !errors.Is(err, assistant.ErrQuota) {
		t.Fatal("unreserved inference dispatch accepted", err)
	}
	var count int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM assistant_steps WHERE run_id=$1`, run.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("rejected inference allocated another step")
	}
}

func TestAssistantUnknownSettlementCannotRefundConfirmedCalls(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "minimum_run", at)
	run, step, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at), Kind: assistant.Inference, MaximumUnits: 40})
	if err != nil {
		t.Fatal(err)
	}
	run, err = store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(run, at), Number: step.Number, Units: 40})
	if err != nil {
		t.Fatal(err)
	}
	run, step, err = store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at), Kind: assistant.Inference, MaximumUnits: 60})
	if err != nil {
		t.Fatal(err)
	}
	run, err = store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(run, at), Number: step.Number, Uncertain: true, Units: 20})
	if err != nil || !run.UsageUncertain {
		t.Fatal("unknown cost state hidden", err)
	}
	settlement := assistant.UsageSettlement{RunID: run.ID, Units: 0, ReceiptDigest: sha256.Sum256([]byte("test_confirmed_usage_receipt")), Audit: assistantOperator("minimum_settle", at)}
	if err := store.SettleAssistantUsage(ctx, settlement); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("confirmed usage refunded", err)
	}
	settlement.Units = 80
	if err := store.SettleAssistantUsage(ctx, settlement); err != nil {
		t.Fatal(err)
	}
	current, err := store.GetAssistant(ctx, actor, run.ID, at)
	if err != nil || current.UsageUncertain || current.UsageConservative || current.UsedUnits != 80 {
		t.Fatal("settled cost state stayed unknown", err)
	}
	var partial, floor int64
	if err := store.pool.QueryRow(ctx, `SELECT units,(SELECT minimum_units FROM assistant_usage_ledger WHERE reservation_id=$1) FROM assistant_steps WHERE run_id=$1 AND step_number=$2`, run.ID, step.Number).Scan(&partial, &floor); err != nil || partial != 20 || floor != 60 {
		t.Fatal("partial confirmed usage was lost")
	}
}

func TestAssistantConservativeSettlementStaysLabelled(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "conservative_run", at)
	run, step, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at), Kind: assistant.Inference, MaximumUnits: 100})
	if err != nil {
		t.Fatal(err)
	}
	run, err = store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(run, at), Number: step.Number, Uncertain: true})
	if err != nil {
		t.Fatal(err)
	}
	s := assistant.UsageSettlement{RunID: run.ID, Units: 100, Conservative: true, ReceiptDigest: sha256.Sum256([]byte("test_conservative_receipt")), Audit: assistantOperator("conservative", at)}
	if err := store.SettleAssistantUsage(ctx, s); err != nil {
		t.Fatal(err)
	}
	current, err := store.GetAssistant(ctx, actor, run.ID, at)
	if err != nil || !current.UsageConservative || current.UsageUncertain || current.UsedUnits != 100 {
		t.Fatal("upper-bound settlement presented as observed usage", err)
	}
	s.Conservative = false
	if err := store.SettleAssistantUsage(ctx, s); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("changed settlement mode replay accepted", err)
	}
	var kind, action string
	if err := store.pool.QueryRow(ctx, `SELECT settlement_kind,(SELECT action FROM audit_entries WHERE id=$2) FROM assistant_usage_ledger WHERE reservation_id=$1`, run.ID, s.Audit.ID).Scan(&kind, &action); err != nil || kind != "conservative" || action != "assistant.usage_conservative" {
		t.Fatal("conservative evidence missing")
	}
}

func TestAssistantConfigurationToggleCannotReviveExistingLease(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "config_run", at)
	s := assistant.Settings{Version: 2, Enabled: false, PolicyVersion: 1, ProfileID: "local_test", ProfileVersion: 1, UserQuota: 1000, InstallationQuota: 10000}
	updated, err := store.ConfigureAssistant(ctx, s, assistantOperator("disable", at))
	if err != nil {
		t.Fatal(err)
	}
	updated.Enabled = true
	if _, err := store.ConfigureAssistant(ctx, updated, assistantOperator("enable", at)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, at), Kind: assistant.Inventory}); !errors.Is(err, assistant.ErrDenied) {
		t.Fatal("old lease revived by enable toggle", err)
	}
	if _, err := store.AssistantEvents(ctx, actor, run.ID, 0, 100, at); !errors.Is(err, assistant.ErrDenied) {
		t.Fatal("old configuration replayed", err)
	}
	closed, err := store.FinishAssistant(ctx, assistant.Finish{Lease: assistantLease(run, at), State: assistant.Interrupted})
	if err != nil || closed.State != assistant.Interrupted {
		t.Fatal("fenced worker cleanup failed", err)
	}
}

func TestAssistantPaidFlagMustMatchOperatorProfile(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	s := assistant.Settings{Version: 2, Enabled: true, PaidEnabled: true, ProfilePaid: true, PolicyVersion: 1, ProfileID: "local_test", ProfileVersion: 1, UserQuota: 1000, InstallationQuota: 10000}
	if _, err := store.ConfigureAssistant(ctx, s, assistantOperator("profile_paid", at)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdmitAssistant(ctx, assistantAdmission(actor, "false_free_run", at)); !errors.Is(err, assistant.ErrDenied) {
		t.Fatal("caller bypassed paid-profile policy", err)
	}
}

func TestAssistantLeaseMinimumAndRecoveryAttemptBudget(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	if _, err := store.AdmitAssistant(ctx, assistantAdmission(actor, "attempt_run", at)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "worker", ProcessID: "process", At: at, Lease: time.Nanosecond}); !errors.Is(err, assistant.ErrInvalid) {
		t.Fatal("submicrosecond lease accepted", err)
	}
	for attempt := 1; attempt <= assistant.MaximumClaims; attempt++ {
		run, ok, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "worker", ProcessID: "process", At: at, Lease: time.Second})
		if err != nil || !ok || run.Attempts != attempt {
			t.Fatal("bounded replacement claim failed", attempt, err)
		}
		if _, err := store.pool.Exec(ctx, `UPDATE assistant_runs SET lease_until_us=created_at_us WHERE id=$1`, run.ID); err != nil {
			t.Fatal(err)
		}
		if n, err := store.RecoverAssistant(ctx, at, 10); err != nil || n != 1 {
			t.Fatal("recover failed", n, err)
		}
	}
	run, err := store.GetAssistant(ctx, actor, "attempt_run", at)
	if err != nil || run.State != assistant.Interrupted || run.Version > assistant.MaximumStatusVersions {
		t.Fatal("unbounded recovery remained active", err)
	}
	if _, ok, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "worker", ProcessID: "process", At: at, Lease: time.Second}); err != nil || ok {
		t.Fatal("exhausted run reclaimed", err)
	}
}

func TestAssistantHeartbeatPreservesFinishVersionAndCapsDeadline(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "heartbeat_run", at)
	renewed, err := store.RenewAssistantLease(ctx, assistantLease(run, at), 30*time.Second)
	if err != nil || renewed.Version != run.Version || renewed.Fence != run.Fence || renewed.LeaseUntil.After(run.Deadline) {
		t.Fatal("heartbeat changed CAS identity or deadline", err)
	}
	if _, err := store.FinishAssistant(ctx, assistant.Finish{Lease: assistantLease(run, at), State: assistant.Completed}); err != nil {
		t.Fatal("heartbeat invalidated finish CAS", err)
	}
	if _, err := store.RenewAssistantLease(ctx, assistantLease(run, at), time.Second); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("terminal run renewed", err)
	}
}

func TestAssistantSmallRecoveryBatchSkipsHealthyHead(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	for _, id := range []string{"recovery_head", "recovery_expired"} {
		if _, err := store.AdmitAssistant(ctx, assistantAdmission(actor, id, at)); err != nil {
			t.Fatal(err)
		}
	}
	// Make the healthy queued row sort first while the other run is due.
	if _, err := store.pool.Exec(ctx, `UPDATE assistant_runs SET updated_at_us=created_at_us+1 WHERE id='recovery_expired'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE assistant_runs SET deadline_us=created_at_us+1 WHERE id='recovery_expired'`); err != nil {
		t.Fatal(err)
	}
	if changed, err := store.RecoverAssistant(ctx, at, 1); err != nil || changed != 1 {
		t.Fatal("healthy head starved expired work", changed, err)
	}
	var head, expired string
	if err := store.pool.QueryRow(ctx, `SELECT (SELECT state FROM assistant_runs WHERE id='recovery_head'),(SELECT state FROM assistant_runs WHERE id='recovery_expired')`).Scan(&head, &expired); err != nil || head != "queued" || expired != "expired" {
		t.Fatal("recovery mutated healthy work or missed due work")
	}
	if changed, err := store.RecoverAssistant(ctx, at, 1); err != nil || changed != 0 {
		t.Fatal("healthy work was repeatedly transitioned", changed, err)
	}
}
