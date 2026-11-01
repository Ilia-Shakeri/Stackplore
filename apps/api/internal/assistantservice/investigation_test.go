package assistantservice

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestInvestigationReservesFixedReadsAndRetainsSeparateCoverage(t *testing.T) {
	t.Parallel()
	for _, available := range []bool{false, true} {
		service, repository, authority, model := fixtureService(t, nil)
		authority.reads.extraReads = available
		input := inputFixture(t, 1)
		input.Admission.Goal = assistant.InvestigateIncident
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		if found, err := service.runOne(context.Background(), "worker"); !found || err != nil {
			t.Fatal("bounded investigation failed", err)
		}
		snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
		wantRecords := 1
		if available {
			wantRecords = 4 // inventory, health, metric, event; no in-scope edge.
		}
		if err != nil || snapshot.Answer == nil || snapshot.Run.State != assistant.Completed || snapshot.Run.ToolCalls != 5 || snapshot.Run.InferenceCalls != 1 || model.calls.Load() != 1 || len(snapshot.Outcomes) != 5 || len(snapshot.Citations) != wantRecords {
			t.Fatal("investigation escaped its bounded metadata contract", err)
		}
		want := []string{}
		for _, kind := range []string{"inventory", "health", "metrics", "events", "topology"} {
			want = append(want, "begin:"+kind, "read:"+kind, "finish:"+kind)
		}
		want = append(want, "begin:inference", "finish:inference", "terminal:completed")
		if !slices.Equal(repository.trace, want) {
			t.Fatal("read crossed its durable reservation or followed untrusted prose")
		}
		if !available {
			for _, outcome := range snapshot.Outcomes[1:] {
				if outcome.Availability != domain.AvailabilityUnsupported || outcome.Records != 0 {
					t.Fatal("unsupported family fabricated evidence or healthy coverage")
				}
			}
		}
		snapshot.Outcomes[0].Records = 100
		again, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
		if err != nil || again.Outcomes[0].Records != 1 {
			t.Fatal("coverage snapshot mutated retained state")
		}
		if available {
			authority.reads.denyHealth.Store(true)
			if _, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID); err != assistant.ErrDenied {
				t.Fatal("later health revocation was hidden by the inventory proof")
			}
			service.mu.Lock()
			entry := service.entries[input.Admission.ID]
			cleared := entry.revoked && entry.evidence == nil && len(entry.extraEvidence) == 0 && entry.answer == nil && entry.outcomes == nil
			service.mu.Unlock()
			if !cleared {
				t.Fatal("revocation left partial private results")
			}
		}
	}
}

func TestInvestigationFailureCannotDispatchPartialEvidence(t *testing.T) {
	t.Parallel()
	service, _, _, model := fixtureService(t, func(options *Options, _ *modelFixture) {
		original := options.Collector
		options.Collector = collectorFunc(func(ctx context.Context, request assistantevidence.ReadRequest) (assistantevidence.Evidence, error) {
			if request.Kind == assistant.Events {
				return assistantevidence.Evidence{}, assistantevidence.ErrDenied
			}
			return original.Read(ctx, request)
		})
	})
	input := inputFixture(t, 1)
	input.Admission.Goal = assistant.InvestigateIncident
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if _, err := service.runOne(context.Background(), "worker"); err != assistant.ErrDenied || model.counts.Load() != 0 || model.calls.Load() != 0 {
		t.Fatal("partial denied evidence reached counting or generation")
	}
	snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
	if err != nil || snapshot.Answer != nil || snapshot.Volatile || len(snapshot.Outcomes) != 0 {
		t.Fatal("failed investigation retained visible private material")
	}
}

func TestInvestigationGoalAndConsentBindIdempotency(t *testing.T) {
	t.Parallel()
	service, repository, _, model := fixtureService(t, nil)
	input := inputFixture(t, 1)
	input.Admission.Goal = assistant.InvestigateIncident
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	changed := input
	changed.LogConsent = true
	if _, err := service.Submit(context.Background(), changed); err != assistant.ErrConflict {
		t.Fatal("log consent changed under an admitted identity")
	}
	changed = input
	changed.Admission.Goal = assistant.ExplainResource
	if _, err := service.Submit(context.Background(), changed); err != assistant.ErrConflict {
		t.Fatal("goal changed under an admitted identity")
	}
	changed = inputFixture(t, 2)
	changed.LogConsent = true
	if _, err := service.Submit(context.Background(), changed); err != assistant.ErrInvalid {
		t.Fatal("ordinary explanation acquired log consent")
	}
	changed.Admission.Goal = assistant.InvestigateIncident
	changed.Scope.Range.Start = changed.Scope.Range.End.Add(-16 * time.Minute)
	changed.Admission.From = changed.Scope.Range.Start
	if _, err := service.Submit(context.Background(), changed); err != assistant.ErrInvalid || repository.admissions != 1 || model.calls.Load() != 0 {
		t.Fatal("oversized log window reached admission")
	}
}

func TestConcurrentInvestigationsNeverShareRunBoundEvidence(t *testing.T) {
	t.Parallel()
	service, repository, authority, model := fixtureService(t, func(options *Options, _ *modelFixture) { options.Workers = 2 })
	authority.reads.extraReads = true
	inputs := []Input{inputFixture(t, 1), inputFixture(t, 2)}
	for index := range inputs {
		inputs[index].Admission.Goal = assistant.InvestigateIncident
		if _, err := service.Submit(context.Background(), inputs[index]); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan error, 2)
	for _, worker := range []string{"worker_one", "worker_two"} {
		go func() {
			found, err := service.runOne(context.Background(), worker)
			if !found && err == nil {
				err = assistant.ErrUnavailable
			}
			done <- err
		}()
	}
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				repository.mu.Lock()
				trace := slices.Clone(repository.trace)
				repository.mu.Unlock()
				t.Logf("bounded step trace: %v; counts: %d; dispatches: %d", trace, model.counts.Load(), model.calls.Load())
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("bounded concurrent investigation stalled")
		}
	}
	seen := map[string]bool{}
	for _, input := range inputs {
		snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
		if err != nil || snapshot.Answer == nil || snapshot.Run.ToolCalls != 5 || len(snapshot.Citations) != 4 {
			t.Fatal("concurrent proof set failed publication", err)
		}
		for _, reference := range snapshot.Citations {
			if seen[reference.ID] {
				t.Fatal("run-bound citation crossed concurrent ownership")
			}
			seen[reference.ID] = true
		}
	}
	if model.calls.Load() != 2 {
		t.Fatal("concurrent read caused another dispatch")
	}
}

func TestSourceRevocationDuringExtraReadPreventsLatePublication(t *testing.T) {
	t.Parallel()
	for _, stopAt := range []assistant.StepKind{assistant.Health, assistant.Metrics, assistant.Events, assistant.Topology} {
		service, _, _, model := fixtureService(t, nil)
		original := service.collector
		service.collector = collectorFunc(func(ctx context.Context, request assistantevidence.ReadRequest) (assistantevidence.Evidence, error) {
			if request.Kind == stopAt {
				service.InvalidateSource(string(request.Scope.Source.SourceID))
			}
			return original.Read(ctx, request)
		})
		input := inputFixture(t, 1)
		input.Admission.Goal = assistant.InvestigateIncident
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		if _, err := service.runOne(context.Background(), "worker"); err == nil || model.counts.Load() != 0 || model.calls.Load() != 0 {
			t.Fatal("revoked partial context reached dispatch")
		}
		snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
		if err != nil || snapshot.Volatile || snapshot.Answer != nil || len(snapshot.Citations) != 0 || len(snapshot.Outcomes) != 0 {
			t.Fatal("revoked extra read published late output")
		}
	}
}

func TestLaterFamilyCannotReuseAnotherRunsSealedProof(t *testing.T) {
	t.Parallel()
	service, _, authority, model := fixtureService(t, nil)
	authority.reads.extraReads = true
	first, second := inputFixture(t, 1), inputFixture(t, 2)
	first.Admission.Goal, second.Admission.Goal = assistant.InvestigateIncident, assistant.InvestigateIncident
	second.Scope.Range = first.Scope.Range
	second.Admission.From, second.Admission.Until = first.Admission.From, first.Admission.Until
	var retained *assistantevidence.Evidence
	original := service.collector
	service.collector = collectorFunc(func(ctx context.Context, request assistantevidence.ReadRequest) (assistantevidence.Evidence, error) {
		if request.Kind == assistant.Health && retained != nil {
			return *retained, nil
		}
		proof, err := original.Read(ctx, request)
		if request.Kind == assistant.Health && err == nil {
			retained = &proof
		}
		return proof, err
	})
	for _, input := range []Input{first, second} {
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.runOne(context.Background(), "worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.runOne(context.Background(), "worker"); err != assistant.ErrDenied || model.counts.Load() != 1 || model.calls.Load() != 1 {
		t.Fatal("same-scope proof from another run authorized generation")
	}
	snapshot, err := service.Get(context.Background(), second.Admission.Actor, second.Admission.ID)
	if err != nil || snapshot.Answer != nil || snapshot.Volatile || len(snapshot.Outcomes) != 0 {
		t.Fatal("cross-run proof retained partial private output")
	}
}
