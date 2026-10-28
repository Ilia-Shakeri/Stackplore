package assistantservice

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

type delayedAdmission struct {
	*fakeRepository
	committed chan struct{}
	release   chan struct{}
	claimed   chan struct{}
	lease     time.Duration
	err       error
	replay    bool
	calls     atomic.Int32
}

func (repository *delayedAdmission) AdmitAssistantWithOutcome(ctx context.Context, input assistant.Admission) (assistant.AdmissionResult, error) {
	outcome, err := repository.fakeRepository.AdmitAssistantWithOutcome(ctx, input)
	if err != nil || repository.replay && repository.calls.Add(1) == 1 {
		return outcome, err
	}
	close(repository.committed)
	select {
	case <-repository.release:
		return outcome, repository.err
	case <-ctx.Done():
		return assistant.AdmissionResult{}, assistant.ErrUnavailable
	}
}

func (repository *delayedAdmission) ClaimAssistant(ctx context.Context, claim assistant.Claim) (assistant.Run, bool, error) {
	run, found, err := repository.fakeRepository.ClaimAssistant(ctx, claim)
	if found {
		if repository.lease > 0 {
			run.LeaseUntil = time.Now().Add(repository.lease)
		}
		if repository.claimed != nil {
			close(repository.claimed)
		}
	}
	return run, found, err
}

func TestUncertainAdmissionCannotHoldWorkerPastLease(t *testing.T) {
	t.Parallel()
	service, repository, _, model := fixtureService(t, nil)
	delayed := &delayedAdmission{fakeRepository: repository, committed: make(chan struct{}), release: make(chan struct{}), claimed: make(chan struct{}), lease: 50 * time.Millisecond, err: assistant.ErrUnavailable}
	service.repository = delayed
	input := inputFixture(t, 1)
	submitted, finished := make(chan error, 1), make(chan error, 1)
	go func() { _, err := service.Submit(context.Background(), input); submitted <- err }()
	waitSignal(t, delayed.committed)
	go func() { _, err := service.runOne(context.Background(), "worker"); finished <- err }()
	waitSignal(t, delayed.claimed)
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal("uncertain admission cleanup failed")
		}
	case <-time.After(time.Second):
		t.Fatal("pending admission blocked the fixed worker")
	}
	close(delayed.release)
	if err := <-submitted; err != assistant.ErrUnavailable {
		t.Fatal("uncertain admission lost its safe error")
	}
	if model.counts.Load() != 0 || model.calls.Load() != 0 || len(service.entries) != 0 || repository.runs[input.Admission.ID].State != assistant.Interrupted {
		t.Fatal("uncertain admission used missing text or retained capacity")
	}
}

func TestCancelPendingAdmissionClosesReadinessWithoutReopeningMemory(t *testing.T) {
	t.Parallel()
	service, repository, _, model := fixtureService(t, nil)
	delayed := &delayedAdmission{fakeRepository: repository, committed: make(chan struct{}), release: make(chan struct{}), claimed: make(chan struct{})}
	service.repository = delayed
	input := inputFixture(t, 1)
	submitted, finished := make(chan error, 1), make(chan error, 1)
	go func() { _, err := service.Submit(context.Background(), input); submitted <- err }()
	waitSignal(t, delayed.committed)
	if _, err := service.Submit(context.Background(), inputFixture(t, 2)); err != assistant.ErrQuota {
		t.Fatal("concurrent admission created an unbounded waiter")
	}
	go func() { _, err := service.runOne(context.Background(), "worker"); finished <- err }()
	waitSignal(t, delayed.claimed)
	if run, err := service.Cancel(context.Background(), input.Admission.Actor, input.Admission.ID); err != nil || run.State != assistant.Cancelled {
		t.Fatal("cancel lost a committed pending admission")
	}
	service.mu.Lock()
	pending := service.entries[input.Admission.ID]
	protected := pending != nil && pending.admitting && pending.run.State == assistant.Cancelled && !service.evictTerminalLocked("")
	service.mu.Unlock()
	if !protected {
		t.Fatal("cancelled admission was evictable before its transaction result returned")
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("pending worker stayed blocked after cancel")
	}
	close(delayed.release)
	if err := <-submitted; err != nil {
		t.Fatal("successful metadata admission changed")
	}
	snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
	if err != nil || snapshot.Run.State != assistant.Cancelled || snapshot.Volatile || snapshot.Answer != nil || model.calls.Load() != 0 {
		t.Fatal("late admission restored cancelled private material")
	}
}

func TestIdempotentReplayCannotOverwriteRunningStepVersion(t *testing.T) {
	t.Parallel()
	started, releaseModel := make(chan struct{}), make(chan struct{})
	service, repository, _, _ := fixtureService(t, func(_ *Options, model *modelFixture) {
		model.hook = func(context.Context) error { close(started); <-releaseModel; return nil }
	})
	delayed := &delayedAdmission{fakeRepository: repository, committed: make(chan struct{}), release: make(chan struct{}), replay: true}
	service.repository = delayed
	input := inputFixture(t, 1)
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	replayed, finished := make(chan error, 1), make(chan error, 1)
	go func() { _, err := service.Submit(context.Background(), input); replayed <- err }()
	waitSignal(t, delayed.committed)
	go func() { _, err := service.runOne(context.Background(), "worker"); finished <- err }()
	waitSignal(t, started)
	close(delayed.release)
	if err := <-replayed; err != nil {
		t.Fatal("matching replay failed")
	}
	close(releaseModel)
	if err := <-finished; err != nil {
		t.Fatal("old replay replaced the worker transaction version")
	}
	snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
	if err != nil || snapshot.Run.State != assistant.Completed || snapshot.Answer == nil {
		t.Fatal("replay prevented a valid bounded result")
	}
}
