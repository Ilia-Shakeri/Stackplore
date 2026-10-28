package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func assistantAuthorityCandidate(actor assistant.Actor, id string, at time.Time) (assistant.Admission, assistant.Run) {
	a := assistantAdmission(actor, id, at)
	a.Volatile, a.MemoryOwner, a.ResourceID = true, "authority_process", "authority_resource"
	run := assistant.Run{ID: a.ID, ConversationID: a.ConversationID, OwnerID: actor.UserID, SourceID: a.SourceID, ResourceID: a.ResourceID,
		Goal: a.Goal, State: assistant.Queued, SourceVersion: a.SourceVersion, RestoreGeneration: a.RestoreGeneration,
		PolicyVersion: a.PolicyVersion, ProfileID: a.ProfileID, ProfileVersion: a.ProfileVersion, Volatile: true,
		ReservedUnits: a.ReservedUnits, CreatedAt: a.At, UpdatedAt: a.At, Deadline: a.Deadline, From: a.From, Until: a.Until}
	return a, run
}

func TestAssistantAuthoritySeparatesAdmissionFromStoredEvidence(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	a, candidate := assistantAuthorityCandidate(actor, "authority_purpose", at)
	if store.AuthorizeAssistantAdmission(ctx, actor, candidate, at) != nil {
		t.Fatal("valid trusted admission authority rejected")
	}
	if store.AuthorizeAssistantEvidence(ctx, actor, candidate, at) == nil {
		t.Fatal("unregistered candidate granted stored evidence authority")
	}
	for _, change := range []func(*assistant.Run){
		func(run *assistant.Run) { run.OwnerID = "other_owner" },
		func(run *assistant.Run) { run.SourceID = "missing_source" },
		func(run *assistant.Run) { run.SourceVersion++ },
		func(run *assistant.Run) { run.ResourceID = "" },
		func(run *assistant.Run) { run.ProfileID = "other_profile" },
		func(run *assistant.Run) { run.ProfileVersion++ },
		func(run *assistant.Run) { run.PolicyVersion++ },
		func(run *assistant.Run) { run.RestoreGeneration++ },
		func(run *assistant.Run) { run.ConfigurationVersion = 2 },
		func(run *assistant.Run) { run.Fence = 1 },
		func(run *assistant.Run) { run.Attempts = 1 },
		func(run *assistant.Run) { run.UsedUnits = 1 },
		func(run *assistant.Run) { run.ToolCalls = 1 },
		func(run *assistant.Run) { run.InferenceCalls = 1 },
		func(run *assistant.Run) { run.Volatile = false },
		func(run *assistant.Run) { run.UsageUncertain = true },
		func(run *assistant.Run) { run.UsageConservative = true },
		func(run *assistant.Run) { run.UpdatedAt = run.UpdatedAt.Add(time.Microsecond) },
		func(run *assistant.Run) { run.State = assistant.Completed },
		func(run *assistant.Run) { run.LeaseOwner = "unaccepted_worker" },
	} {
		invalid := candidate
		change(&invalid)
		if store.AuthorizeAssistantAdmission(ctx, actor, invalid, at) == nil {
			t.Fatal("tampered admission candidate granted authority")
		}
	}
	stored, err := store.AdmitAssistant(ctx, a)
	if err != nil || store.AuthorizeAssistantEvidence(ctx, actor, stored, at) != nil {
		t.Fatal("exact stored queued authority rejected")
	}
	if store.AuthorizeAssistantAdmission(ctx, actor, stored, at) == nil {
		t.Fatal("stored evidence bypassed admission purpose")
	}
	// A same-ID admission retry checks current admission authority only. The
	// separate evidence method still requires its persisted version and scope.
	if store.AuthorizeAssistantAdmission(ctx, actor, candidate, at) != nil {
		t.Fatal("idempotent Submit could not check admission authority")
	}
	downgraded := stored
	downgraded.Version, downgraded.ConfigurationVersion = 0, 0
	if store.AuthorizeAssistantEvidence(ctx, actor, downgraded, at) == nil {
		t.Fatal("stored evidence accepted a zero-version downgrade")
	}
	for _, change := range []func(*assistant.Run){
		func(run *assistant.Run) { run.ID = "another_run" },
		func(run *assistant.Run) { run.ConversationID = "another_conversation" },
		func(run *assistant.Run) { run.OwnerID = "another_owner" },
		func(run *assistant.Run) { run.SourceID = "source_docker" },
		func(run *assistant.Run) { run.SourceVersion++ },
		func(run *assistant.Run) { run.ResourceID = "another_resource" },
		func(run *assistant.Run) { run.ProfileID = "another_profile" },
		func(run *assistant.Run) { run.ProfileVersion++ },
		func(run *assistant.Run) { run.PolicyVersion++ },
		func(run *assistant.Run) { run.RestoreGeneration++ },
		func(run *assistant.Run) { run.ConfigurationVersion++ },
		func(run *assistant.Run) { run.Version++ },
		func(run *assistant.Run) { run.Fence++ },
		func(run *assistant.Run) { run.From = run.From.Add(-time.Second) },
		func(run *assistant.Run) { run.Until = run.Until.Add(-time.Microsecond) },
		func(run *assistant.Run) { run.UpdatedAt = run.UpdatedAt.Add(time.Microsecond) },
		func(run *assistant.Run) { run.Deadline = run.Deadline.Add(-time.Second) },
		func(run *assistant.Run) { run.ReservedUnits-- },
		func(run *assistant.Run) { run.State = assistant.Completed },
		func(run *assistant.Run) { run.Paid = true },
		func(run *assistant.Run) { run.Volatile = false },
	} {
		invalid := stored
		change(&invalid)
		if store.AuthorizeAssistantEvidence(ctx, actor, invalid, at) == nil {
			t.Fatal("tampered stored run granted evidence authority")
		}
	}
	otherSession := actor
	otherSession.SessionDigest[0] ^= 1
	if store.AuthorizeAssistantEvidence(ctx, otherSession, stored, at) == nil {
		t.Fatal("unbound session granted evidence authority")
	}
	running, found, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "authority_worker", ProcessID: "authority_process", At: at, Lease: assistant.MaximumLeaseDuration})
	if err != nil || !found || store.AuthorizeAssistantEvidence(ctx, actor, running, at) != nil {
		t.Fatal("exact running authority rejected")
	}
	if store.AuthorizeAssistantEvidence(ctx, actor, stored, at) == nil {
		t.Fatal("stale queued version retained evidence authority")
	}
	completed, err := store.FinishAssistant(ctx, assistant.Finish{Lease: assistantLease(running, at), State: assistant.Completed})
	if err != nil || store.AuthorizeAssistantEvidence(ctx, actor, completed, at) != nil {
		t.Fatal("exact completed authority rejected")
	}
	_, candidate = assistantAuthorityCandidate(actor, "authority_cancelled", at)
	a, _ = assistantAuthorityCandidate(actor, candidate.ID, at)
	if _, err := store.AdmitAssistant(ctx, a); err != nil {
		t.Fatal("admit cancellation authority fixture")
	}
	cancelled, err := store.CancelAssistant(ctx, actor, a.ID, at)
	if err != nil || store.AuthorizeAssistantEvidence(ctx, actor, cancelled, at) == nil {
		t.Fatal("cancelled run retained evidence authority")
	}
}

func TestAssistantAuthorityRechecksCurrentSessionSourceAndPolicy(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	a, candidate := assistantAuthorityCandidate(actor, "authority_recheck", at)
	run, err := store.AdmitAssistant(ctx, a)
	if err != nil {
		t.Fatal("admit authority recheck fixture")
	}
	for _, mutation := range []struct{ name, revoke, restore string }{
		{"session_revoked", `UPDATE sessions SET revoked_at_us=created_at_us,revoke_reason_code='session.revoked'`, `UPDATE sessions SET revoked_at_us=NULL,revoke_reason_code=NULL`},
		{"session_idle_expired", `UPDATE sessions SET idle_expires_at_us=created_at_us+1`, `UPDATE sessions SET idle_expires_at_us=created_at_us+1800000000`},
		{"session_absolute_expired", `UPDATE sessions SET idle_expires_at_us=created_at_us+1,absolute_expires_at_us=created_at_us+1`, `UPDATE sessions SET idle_expires_at_us=created_at_us+1800000000,absolute_expires_at_us=created_at_us+43200000000`},
		{"user_disabled", `UPDATE users SET enabled=0,disabled_at_us=created_at_us`, `UPDATE users SET enabled=1,disabled_at_us=NULL`},
		{"role_revision", `UPDATE users SET role='viewer',auth_revision=auth_revision+1`, `UPDATE users SET role='administrator',auth_revision=auth_revision-1`},
		{"source_disabled", `UPDATE infrastructure_sources SET enabled=0 WHERE id='source_demo'`, `UPDATE infrastructure_sources SET enabled=1 WHERE id='source_demo'`},
		{"source_version", `UPDATE infrastructure_sources SET row_version=2 WHERE id='source_demo'`, `UPDATE infrastructure_sources SET row_version=1 WHERE id='source_demo'`},
		{"feature_disabled", `UPDATE assistant_control SET enabled=FALSE`, `UPDATE assistant_control SET enabled=TRUE`},
		{"configuration_version", `UPDATE assistant_control SET row_version=3`, `UPDATE assistant_control SET row_version=2`},
		{"profile_changed", `UPDATE assistant_control SET profile_id='changed_profile'`, `UPDATE assistant_control SET profile_id='local_test'`},
		{"profile_version", `UPDATE assistant_control SET profile_version=2`, `UPDATE assistant_control SET profile_version=1`},
		{"policy_version", `UPDATE assistant_control SET policy_version=2`, `UPDATE assistant_control SET policy_version=1`},
		{"restore_generation", `UPDATE installation_metadata SET restore_generation=1,restored_at_us=created_at_us`, `UPDATE installation_metadata SET restore_generation=0,restored_at_us=NULL`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if _, err := store.pool.Exec(ctx, mutation.revoke); err != nil {
				t.Fatal("apply owned authority revocation")
			}
			if store.AuthorizeAssistantEvidence(ctx, actor, run, at) == nil {
				t.Fatal("revoked current authority was accepted")
			}
			if mutation.name != "configuration_version" && store.AuthorizeAssistantAdmission(ctx, actor, candidate, at) == nil {
				t.Fatal("revoked current admission authority was accepted")
			}
			if _, err := store.pool.Exec(ctx, mutation.restore); err != nil {
				t.Fatal("restore owned authority fixture")
			}
			if store.AuthorizeAssistantEvidence(ctx, actor, run, at) != nil {
				t.Fatal("valid authority did not recover after fixture reset")
			}
		})
	}
}

func TestAssistantAuthorityDatabaseErrorsFailClosed(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	a, _ := assistantAuthorityCandidate(actor, "authority_database", at)
	run, err := store.AdmitAssistant(ctx, a)
	if err != nil {
		t.Fatal("admit unavailable authority fixture")
	}
	locked, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatal("begin owned authority lock fixture")
	}
	defer func() { _ = locked.Rollback(ctx) }()
	if _, err := locked.Exec(ctx, `SELECT singleton_id FROM assistant_control FOR UPDATE`); err != nil {
		t.Fatal("lock owned authority control")
	}
	work, stop := context.WithTimeout(ctx, 30*time.Millisecond)
	err = store.AuthorizeAssistantEvidence(work, actor, run, at)
	stop()
	if !errors.Is(err, assistant.ErrUnavailable) {
		t.Fatal("database timeout did not fail closed")
	}
	if locked.Rollback(ctx) != nil || store.Close() != nil {
		t.Fatal("release owned unavailable authority fixture")
	}
	if store.AuthorizeAssistantEvidence(ctx, actor, run, at) == nil {
		t.Fatal("closed database granted evidence authority")
	}
}

func TestAssistantAuthorityAllowsOnlyForwardProgressWithinSameClaim(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	a, _ := assistantAuthorityCandidate(actor, "authority_progress", at)
	if _, err := store.AdmitAssistant(ctx, a); err != nil {
		t.Fatal("admit forward authority fixture")
	}
	observed, found, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "authority_worker", ProcessID: a.MemoryOwner, At: at, Lease: assistant.MaximumLeaseDuration})
	if err != nil || !found {
		t.Fatal("claim forward authority fixture")
	}
	renewed, err := store.RenewAssistantLease(ctx, assistantLease(observed, at), assistant.MaximumLeaseDuration)
	if err != nil || !renewed.LeaseUntil.After(observed.LeaseUntil) || renewed.Version != observed.Version || store.AuthorizeAssistantEvidence(ctx, actor, observed, at) != nil {
		t.Fatal("legitimate lease renewal invalidated an unchanged claim")
	}
	rollback, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatal("begin owned authority rollback")
	}
	defer func() { _ = rollback.Rollback(ctx) }()
	rolledBack, err := assistantScan(rollback.QueryRow(ctx, `UPDATE assistant_runs SET row_version=row_version+1,tool_calls=tool_calls+1,updated_at_us=(extract(epoch FROM clock_timestamp())*1000000)::BIGINT WHERE id=$1 RETURNING `+assistantRunColumns, observed.ID))
	if err != nil || rollback.Rollback(ctx) != nil {
		t.Fatal("roll back owned authority progress")
	}
	if store.AuthorizeAssistantEvidence(ctx, actor, rolledBack.Run, at) == nil || store.AuthorizeAssistantEvidence(ctx, actor, observed, at) != nil {
		t.Fatal("uncommitted progress gained authority or valid claim was lost")
	}
	running, step, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(renewed, at), Kind: assistant.Inventory})
	if err != nil || store.AuthorizeAssistantEvidence(ctx, actor, observed, at) != nil {
		t.Fatal("committed forward tool progress invalidated a live claim")
	}
	running, err = store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(running, at), Number: step.Number})
	if err != nil {
		t.Fatal("finish owned authority tool")
	}
	running, step, err = store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(running, at), Kind: assistant.Inference, MaximumUnits: 100})
	if err != nil {
		t.Fatal("begin owned authority inference metadata")
	}
	running, err = store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(running, at), Number: step.Number, Units: 2})
	if err != nil || store.AuthorizeAssistantEvidence(ctx, actor, observed, at) != nil {
		t.Fatal("committed forward accounting invalidated a live claim")
	}
	for _, change := range []func(*assistant.Run){
		func(run *assistant.Run) { run.Version++ },
		func(run *assistant.Run) { run.UpdatedAt = run.UpdatedAt.Add(time.Microsecond) },
		func(run *assistant.Run) { run.LeaseUntil = run.LeaseUntil.Add(time.Microsecond) },
		func(run *assistant.Run) { run.ToolCalls++ },
		func(run *assistant.Run) { run.ToolCalls-- },
		func(run *assistant.Run) { run.InferenceCalls++ },
		func(run *assistant.Run) { run.InferenceCalls-- },
		func(run *assistant.Run) { run.UsedUnits++ },
		func(run *assistant.Run) { run.UsedUnits-- },
		func(run *assistant.Run) { run.Attempts++ },
		func(run *assistant.Run) { run.Fence++ },
		func(run *assistant.Run) { run.LeaseOwner = "other_worker" },
		func(run *assistant.Run) { run.ResourceID = "other_resource" },
		func(run *assistant.Run) { run.ConfigurationVersion++ },
		func(run *assistant.Run) { run.UsageUncertain = true },
		func(run *assistant.Run) { run.UsageConservative = true },
	} {
		invalid := running
		change(&invalid)
		if store.AuthorizeAssistantEvidence(ctx, actor, invalid, at) == nil {
			t.Fatal("future, rolled back or tampered same-claim metadata granted authority")
		}
	}
	completed, err := store.FinishAssistant(ctx, assistant.Finish{Lease: assistantLease(running, at), State: assistant.Completed})
	if err != nil || store.AuthorizeAssistantEvidence(ctx, actor, running, at) == nil || store.AuthorizeAssistantEvidence(ctx, actor, completed, at) != nil {
		t.Fatal("terminal transition retained stale running authority")
	}
}

func TestAssistantAuthorityFencesActualExpiryTakeoverAndCancellation(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	a, _ := assistantAuthorityCandidate(actor, "authority_takeover", at)
	a.Volatile, a.MemoryOwner = false, ""
	if _, err := store.AdmitAssistant(ctx, a); err != nil {
		t.Fatal("admit owned nonvolatile authority fixture")
	}
	old, found, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "old_authority_worker", ProcessID: "old_authority_process", At: at, Lease: assistant.MinimumLeaseDuration})
	if err != nil || !found || store.AuthorizeAssistantEvidence(ctx, actor, old, at) != nil {
		t.Fatal("claim owned expiring authority fixture")
	}
	for {
		var remaining int64
		if err := store.pool.QueryRow(ctx, `SELECT lease_until_us-(extract(epoch FROM clock_timestamp())*1000000)::BIGINT FROM assistant_runs WHERE id=$1`, old.ID).Scan(&remaining); err != nil {
			t.Fatal("observe owned lease expiry")
		}
		if remaining <= 0 {
			break
		}
		wait := time.NewTimer(min(time.Duration(remaining)*time.Microsecond, 50*time.Millisecond))
		select {
		case <-ctx.Done():
			wait.Stop()
			t.Fatal("owned lease did not expire within bound")
		case <-wait.C:
		}
	}
	if store.AuthorizeAssistantEvidence(ctx, actor, old, at) == nil {
		t.Fatal("expired actual lease retained read authority")
	}
	if changed, err := store.RecoverAssistant(ctx, at, 1); err != nil || changed != 1 {
		t.Fatal("recover owned expired authority fixture")
	}
	current, found, err := store.ClaimAssistant(ctx, assistant.Claim{Worker: "new_authority_worker", ProcessID: "new_authority_process", At: at, Lease: assistant.MaximumLeaseDuration})
	if err != nil || !found || current.Fence <= old.Fence || current.Attempts != old.Attempts+1 || store.AuthorizeAssistantEvidence(ctx, actor, old, at) == nil || store.AuthorizeAssistantEvidence(ctx, actor, current, at) != nil {
		t.Fatal("takeover retained old authority or lost the new fenced claim")
	}
	if _, err := store.CancelAssistant(ctx, actor, current.ID, at); err != nil || store.AuthorizeAssistantEvidence(ctx, actor, current, at) == nil || store.AuthorizeAssistantEvidence(ctx, actor, old, at) == nil {
		t.Fatal("cancellation retained same-claim or prior-claim authority")
	}
}
