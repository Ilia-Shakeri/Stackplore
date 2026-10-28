package assistantevidence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestMetadataAuthorizationNeverReadsProviderForAnyStoredState(t *testing.T) {
	for _, state := range []assistant.State{assistant.Queued, assistant.Running, assistant.Completed, assistant.Failed, assistant.Cancelled, assistant.Expired, assistant.Interrupted, assistant.Uncertain} {
		t.Run(string(state), func(t *testing.T) {
			f := newFixture(t)
			run := f.request.Run
			run.State = state
			if err := f.reader.Authorize(context.Background(), f.request.Actor, run, f.request.Scope); err != nil {
				t.Fatal("metadata gate result changed")
			}
			if f.gate.checks.Load() != 1 || f.live.calls.Load() != 0 || f.reads.resourceCalls != 0 {
				t.Fatal("metadata polling read provider")
			}
		})
	}
}

func TestSealedEvidenceAllowsNormalStepLeaseAndCompletionTransitions(t *testing.T) {
	f := newFixture(t)
	evidence, err := f.reader.Read(context.Background(), f.request)
	if err != nil {
		t.Fatal(err)
	}
	run := f.request.Run
	run.Version += 3
	run.ToolCalls++
	run.InferenceCalls++
	run.LeaseUntil = run.LeaseUntil.Add(5 * time.Second)
	run.UpdatedAt = run.UpdatedAt.Add(time.Second)
	if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, run, evidence); err != nil {
		t.Fatal("normal step or lease renewal rejected")
	}
	run.State = assistant.Completed
	run.Version++
	run.Fence++
	run.LeaseOwner = ""
	run.LeaseUntil = time.Time{}
	if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, run, evidence); err != nil {
		t.Fatal("normal completion rejected")
	}
	if _, err := f.reader.Resolve(context.Background(), f.request.Actor, run, evidence, evidence.References()[0].ID); err != nil {
		t.Fatal("completed reference rejected")
	}
}

func TestSealedEvidenceRejectsLeaseLossTakeoverAndTerminalFailure(t *testing.T) {
	for name, edit := range map[string]func(*assistant.Run){
		"expired-lease": func(r *assistant.Run) { r.LeaseUntil = r.CreatedAt }, "expired-deadline": func(r *assistant.Run) { r.Deadline = r.CreatedAt },
		"takeover": func(r *assistant.Run) { r.Fence++; r.Attempts++; r.LeaseOwner = "other_worker" }, "changed-owner": func(r *assistant.Run) { r.LeaseOwner = "other_worker" },
		"changed-attempt": func(r *assistant.Run) { r.Attempts++ }, "old-version": func(r *assistant.Run) { r.Version-- },
		"failed": func(r *assistant.Run) { r.State = assistant.Failed; r.Fence++ }, "cancelled": func(r *assistant.Run) { r.State = assistant.Cancelled; r.Fence++ },
		"uncertain": func(r *assistant.Run) { r.State = assistant.Uncertain; r.Fence++; r.UsageUncertain = true },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			evidence, err := f.reader.Read(context.Background(), f.request)
			if err != nil {
				t.Fatal(err)
			}
			run := f.request.Run
			edit(&run)
			if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, run, evidence); !errors.Is(err, ErrDenied) {
				t.Fatal("lost lease or invalid transition released evidence")
			}
		})
	}
}
