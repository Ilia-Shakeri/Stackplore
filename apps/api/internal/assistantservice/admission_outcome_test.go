package assistantservice

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

type concurrentAdmissionRepository struct {
	*fakeRepository
	state      assistant.State
	wrongOwner bool
	wrongID    bool
}

func (repository *concurrentAdmissionRepository) AdmitAssistantWithOutcome(ctx context.Context, input assistant.Admission) (assistant.AdmissionResult, error) {
	// An independent transaction wins after the caller's missing-row preflight.
	first, err := repository.fakeRepository.AdmitAssistantWithOutcome(ctx, input)
	if err != nil {
		return first, err
	}
	repository.mu.Lock()
	run := repository.runs[input.ID]
	run.State = repository.state
	if run.State == assistant.Running {
		run.Version++
		run.Fence++
		run.LeaseOwner = "other_worker"
		run.LeaseUntil = input.At.Add(30 * time.Second)
	}
	repository.runs[input.ID] = run
	repository.mu.Unlock()
	outcome, err := repository.fakeRepository.AdmitAssistantWithOutcome(ctx, input)
	if repository.wrongOwner {
		outcome.Run.OwnerID = "other_owner"
	}
	if repository.wrongID {
		outcome.Run.ID = "other_run"
	}
	return outcome, err
}

func TestConcurrentAdmissionNeverRestoresNewPrivateMaterial(t *testing.T) {
	for _, state := range []assistant.State{assistant.Queued, assistant.Running} {
		t.Run(string(state), func(t *testing.T) {
			t.Parallel()
			service, repository, _, model := fixtureService(t, nil)
			service.repository = &concurrentAdmissionRepository{fakeRepository: repository, state: state}
			input := inputFixture(t, 1)
			run, err := service.Submit(context.Background(), input)
			if err != nil || run.ID != input.Admission.ID || run.State != state {
				t.Fatal("idempotent race did not return authorized metadata")
			}
			service.mu.Lock()
			retained := len(service.entries) != 0 || len(service.conversations) != 0
			service.mu.Unlock()
			if retained || repository.admissions != 1 || model.calls.Load() != 0 {
				t.Fatal("reused admission retained a new question, turn or dispatch")
			}
		})
	}
}

func TestAdmissionOutcomeRejectsAnotherRunOrOwner(t *testing.T) {
	for _, owner := range []bool{false, true} {
		t.Run(map[bool]string{false: "run", true: "owner"}[owner], func(t *testing.T) {
			t.Parallel()
			service, repository, _, model := fixtureService(t, nil)
			service.repository = &concurrentAdmissionRepository{fakeRepository: repository, state: assistant.Queued, wrongOwner: owner, wrongID: !owner}
			run, err := service.Submit(context.Background(), inputFixture(t, 1))
			if err != assistant.ErrConflict || run.ID != "" {
				t.Fatal("mismatched admission metadata escaped the owner boundary")
			}
			if len(service.entries) != 0 || len(service.conversations) != 0 || model.calls.Load() != 0 {
				t.Fatal("invalid metadata retained or dispatched private material")
			}
		})
	}
}

type legacyAdmissionRepository struct {
	assistant.RuntimeRepository
}

func TestServiceRequiresExplicitCommittedAdmissionOutcome(t *testing.T) {
	service, repository, _, model := fixtureService(t, nil)
	options := Options{Repository: repository, Gateway: service.gateway, Authorizer: service.authorizer, Collector: service.collector, Lifetime: context.Background(), Now: service.now, ProcessID: service.processID, FingerprintKey: append([]byte(nil), service.key[:]...), Workers: service.workers, PollInterval: service.poll, Heartbeat: service.heartbeat, OutputLimit: service.output}
	control, err := New(options)
	if err != nil {
		t.Fatal("valid outcome repository rejected")
	}
	control.Close()
	options.Repository = &legacyAdmissionRepository{RuntimeRepository: repository}
	if rejected, err := New(options); err != assistant.ErrInvalid {
		if rejected != nil {
			rejected.Close()
		}
		t.Fatal("legacy admission cannot distinguish a committed creation")
	}
	service.repository = options.Repository
	if _, err := service.Submit(context.Background(), inputFixture(t, 1)); err != assistant.ErrUnavailable || repository.admissions != 0 || model.calls.Load() != 0 {
		t.Fatal("missing outcome port staged or dispatched work")
	}
}
