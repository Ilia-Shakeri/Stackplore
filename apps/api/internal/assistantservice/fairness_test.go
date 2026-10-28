package assistantservice

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
)

func completeFixtureRun(t *testing.T, service *Service, input Input) assistant.Run {
	t.Helper()
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal("bounded fixture admission failed", err)
	}
	if found, err := service.runOne(context.Background(), "worker"); err != nil || !found {
		t.Fatal("bounded fixture completion failed", err)
	}
	snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
	if err != nil || snapshot.Run.State != assistant.Completed || snapshot.Answer == nil {
		t.Fatal("fixture did not publish its retained completed answer")
	}
	return snapshot.Run
}

func TestCompletedOwnerMemoryCannotExcludeAnotherOwner(t *testing.T) {
	t.Parallel()
	service, repository, _, model := fixtureService(t, nil)
	var first Input
	for index := range MaximumMemoryRuns {
		input := inputFixture(t, index+1)
		input.Admission.ConversationID = fmt.Sprintf("owner_conversation_%d", index/MaximumTurns)
		if index == 0 {
			first = input
		}
		completeFixtureRun(t, service, input)
	}
	service.mu.Lock()
	bounded := len(service.entries) == MaximumOwnerMemoryRuns && len(service.conversations) == 3 && service.entries[first.Admission.ID] == nil && service.conversations[first.Admission.ConversationID].turns == MaximumTurns
	service.mu.Unlock()
	if !bounded {
		t.Fatal("one owner retained the installation registry or reset its turn ledger")
	}
	lost, err := service.Get(context.Background(), first.Admission.Actor, first.Admission.ID)
	if err != nil || lost.Run.State != assistant.Completed || lost.Volatile || lost.Answer != nil || len(lost.Citations) != 0 {
		t.Fatal("evicted completed answer did not retain truthful metadata")
	}
	other := inputFixture(t, 30)
	other.Admission.Actor.UserID, other.Admission.ConversationID = "other_owner", "other_conversation"
	if run, err := service.Submit(context.Background(), other); err != nil || run.State != assistant.Queued {
		t.Fatal("completed history blocked another owner")
	}
	if run, err := service.Submit(context.Background(), first); err != nil || run.ID != first.Admission.ID || run.State != assistant.Completed {
		t.Fatal("matching evicted retry lost durable status")
	}
	service.mu.Lock()
	unchanged := service.entries[first.Admission.ID] == nil && len(service.entries) == MaximumOwnerMemoryRuns+1 && service.conversations[first.Admission.ConversationID].turns == MaximumTurns
	service.mu.Unlock()
	if !unchanged || repository.admissions != MaximumMemoryRuns+1 || model.calls.Load() != MaximumMemoryRuns {
		t.Fatal("evicted retry restored private text, consumed a turn or dispatched again")
	}
	changed := first
	changed.Question = "Explain the selected resource again."
	if _, err := service.Submit(context.Background(), changed); err != assistant.ErrConflict {
		t.Fatal("evicted retry accepted changed material")
	}
	ninth := inputFixture(t, 31)
	ninth.Admission.ConversationID = first.Admission.ConversationID
	if _, err := service.Submit(context.Background(), ninth); err != assistant.ErrQuota {
		t.Fatal("answer eviction reset the eight-turn limit")
	}
}

func TestOwnerConversationLedgerIsBoundedWithoutTakingAllSlots(t *testing.T) {
	t.Parallel()
	service, _, _, _ := fixtureService(t, nil)
	var first Input
	for index := range MaximumOwnerConversations {
		input := inputFixture(t, index+1)
		input.Admission.ConversationID = fmt.Sprintf("conversation_%d", index)
		if index == 0 {
			first = input
		}
		completeFixtureRun(t, service, input)
	}
	input := inputFixture(t, 20)
	input.Admission.ConversationID = "owner_overflow"
	if _, err := service.Submit(context.Background(), input); err != assistant.ErrQuota {
		t.Fatal("one owner exceeded the conversation ledger cap")
	}
	retained, err := service.Get(context.Background(), first.Admission.Actor, first.Admission.ID)
	if err != nil || retained.Answer == nil {
		t.Fatal("rejected conversation unnecessarily evicted a retained answer")
	}
	input.Admission.Actor.UserID = "other_owner"
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal("one owner consumed every conversation slot")
	}
}

func TestGlobalPressureEvictsOldestTerminalAndPreservesLedger(t *testing.T) {
	t.Parallel()
	service, _, _, model := fixtureService(t, nil)
	var first Input
	for index := range MaximumMemoryRuns {
		input := inputFixture(t, index+1)
		input.Admission.Actor.UserID = fmt.Sprintf("owner_%d", index/6)
		input.Admission.ConversationID = fmt.Sprintf("conversation_%d", index/6)
		if index == 0 {
			first = input
		}
		completeFixtureRun(t, service, input)
	}
	other := inputFixture(t, 30)
	other.Admission.Actor.UserID, other.Admission.ConversationID = "fourth_owner", "fourth_conversation"
	if _, err := service.Submit(context.Background(), other); err != nil {
		t.Fatal("terminal-only global pressure denied new work")
	}
	service.mu.Lock()
	bounded := len(service.entries) == MaximumMemoryRuns && service.entries[first.Admission.ID] == nil && service.conversations[first.Admission.ConversationID].turns == 6
	service.mu.Unlock()
	if !bounded || model.calls.Load() != MaximumMemoryRuns {
		t.Fatal("global eviction grew memory, reset conversation age or dispatched polling")
	}
	snapshot, err := service.Get(context.Background(), first.Admission.Actor, first.Admission.ID)
	if err != nil || snapshot.Volatile || snapshot.Answer != nil || snapshot.Run.State != assistant.Completed {
		t.Fatal("global eviction erased durable status or invented retained output")
	}
}

func TestOwnerPressureCannotEvictQueuedOrActiveQuestions(t *testing.T) {
	t.Parallel()
	started, release := make(chan struct{}), make(chan struct{})
	service, _, _, model := fixtureService(t, func(_ *Options, model *modelFixture) {
		model.hook = func(ctx context.Context) error {
			close(started)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
	var first Input
	for index := range MaximumOwnerMemoryRuns {
		input := inputFixture(t, index+1)
		input.Admission.ConversationID = fmt.Sprintf("conversation_%d", index)
		if index == 0 {
			first = input
		}
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal("queued fixture admission failed")
		}
	}
	done := make(chan error, 1)
	go func() { _, err := service.runOne(context.Background(), "worker"); done <- err }()
	waitSignal(t, started)
	input := inputFixture(t, 20)
	input.Admission.ConversationID = first.Admission.ConversationID
	_, err := service.Submit(context.Background(), input)
	service.mu.Lock()
	preserved := len(service.entries) == MaximumOwnerMemoryRuns
	for _, entry := range service.entries {
		preserved = preserved && !entry.revoked && entry.input.Question != ""
	}
	active := service.entries[first.Admission.ID].active
	service.mu.Unlock()
	close(release)
	select {
	case workerErr := <-done:
		if workerErr != nil {
			t.Fatal("preserved active fixture failed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("active fixture did not stop")
	}
	if err != assistant.ErrQuota || !preserved || !active || model.calls.Load() != 1 {
		t.Fatal("memory pressure discarded queued or active work")
	}
}

type retryPreflightRepository struct {
	*fakeRepository
	lookupError error
	afterLookup func()
	required    atomic.Bool
}

func (repository *retryPreflightRepository) GetAssistant(ctx context.Context, actor assistant.Actor, id string, at time.Time) (assistant.Run, error) {
	if repository.lookupError != nil {
		return assistant.Run{}, repository.lookupError
	}
	run, err := repository.fakeRepository.GetAssistant(ctx, actor, id, at)
	if err == nil && repository.afterLookup != nil {
		repository.afterLookup()
	}
	return run, err
}

func (repository *retryPreflightRepository) AdmitAssistantWithOutcome(ctx context.Context, input assistant.Admission) (assistant.AdmissionResult, error) {
	repository.required.Store(input.RequireExisting)
	return repository.fakeRepository.AdmitAssistantWithOutcome(ctx, input)
}

func TestRetryRemovedAfterPreflightCannotCreateOrRestoreQuestion(t *testing.T) {
	t.Parallel()
	service, repository, _, model := fixtureService(t, nil)
	input := inputFixture(t, 1)
	completeFixtureRun(t, service, input)
	service.mu.Lock()
	service.evictTerminalLocked(input.Admission.Actor.UserID)
	service.mu.Unlock()
	retry := &retryPreflightRepository{fakeRepository: repository}
	retry.afterLookup = func() {
		repository.mu.Lock()
		delete(repository.runs, input.Admission.ID)
		delete(repository.actors, input.Admission.ID)
		delete(repository.digests, input.Admission.IdempotencyDigest)
		delete(repository.fingerprints, input.Admission.ID)
		repository.mu.Unlock()
	}
	service.repository = retry
	if _, err := service.Submit(context.Background(), input); err != assistant.ErrConflict || !retry.required.Load() {
		t.Fatal("removed retry recreated accepted work")
	}
	service.mu.Lock()
	unchanged := len(service.entries) == 0 && service.conversations[input.Admission.ConversationID].turns == 1
	service.mu.Unlock()
	if !unchanged || len(repository.runs) != 0 || repository.admissions != 1 || model.calls.Load() != 1 {
		t.Fatal("retry race retained text, consumed a turn or dispatched twice")
	}
}

func TestTerminalAdmissionRaceDiscardsStagedQuestionWithoutTurn(t *testing.T) {
	t.Parallel()
	service, repository, _, model := fixtureService(t, nil)
	input := inputFixture(t, 1)
	completeFixtureRun(t, service, input)
	service.mu.Lock()
	service.evictTerminalLocked(input.Admission.Actor.UserID)
	service.mu.Unlock()
	service.repository = &retryPreflightRepository{fakeRepository: repository, lookupError: assistant.ErrDenied}
	if run, err := service.Submit(context.Background(), input); err != nil || run.State != assistant.Completed {
		t.Fatal("concurrent terminal retry lost safe metadata")
	}
	service.mu.Lock()
	unchanged := len(service.entries) == 0 && service.conversations[input.Admission.ConversationID].turns == 1
	service.mu.Unlock()
	if !unchanged || repository.admissions != 1 || model.calls.Load() != 1 {
		t.Fatal("terminal fallback restored question text or consumed a turn")
	}
}

func TestPreflightStorageFailureCannotStagePrivateMaterial(t *testing.T) {
	t.Parallel()
	service, repository, _, model := fixtureService(t, nil)
	service.repository = &retryPreflightRepository{fakeRepository: repository, lookupError: errors.New("fixture lookup failure")}
	if _, err := service.Submit(context.Background(), inputFixture(t, 1)); err != assistant.ErrUnavailable || len(service.entries) != 0 || len(service.conversations) != 0 || repository.admissions != 0 || model.calls.Load() != 0 {
		t.Fatal("failed metadata lookup staged or dispatched private work")
	}
}

func TestCancelledAndFailedTerminalEntriesCanReleaseMemorySlots(t *testing.T) {
	for _, state := range []assistant.State{assistant.Cancelled, assistant.Failed} {
		t.Run(string(state), func(t *testing.T) {
			t.Parallel()
			service, _, _, model := fixtureService(t, nil)
			if state == assistant.Failed {
				service.collector = collectorFunc(func(context.Context, assistantevidence.ReadRequest) (assistantevidence.Evidence, error) {
					return assistantevidence.Evidence{}, errors.New("fixture read unavailable")
				})
			}
			var first Input
			for index := range MaximumOwnerMemoryRuns {
				input := inputFixture(t, index+1)
				input.Admission.ConversationID = fmt.Sprintf("conversation_%d", index%2)
				if index == 0 {
					first = input
				}
				if _, err := service.Submit(context.Background(), input); err != nil {
					t.Fatal("terminal fixture admission failed")
				}
				if state == assistant.Cancelled {
					if _, err := service.Cancel(context.Background(), input.Admission.Actor, input.Admission.ID); err != nil {
						t.Fatal("terminal fixture cancellation failed")
					}
				} else if _, err := service.runOne(context.Background(), "worker"); err == nil {
					t.Fatal("failed read unexpectedly completed")
				}
			}
			service.mu.Lock()
			terminal := service.entries[first.Admission.ID].run.State == state && !service.entries[first.Admission.ID].active
			service.mu.Unlock()
			input := inputFixture(t, 20)
			input.Admission.ConversationID = first.Admission.ConversationID
			if _, err := service.Submit(context.Background(), input); err != nil {
				t.Fatal("durable terminal state did not release a memory slot")
			}
			service.mu.Lock()
			bounded := len(service.entries) == MaximumOwnerMemoryRuns && service.entries[first.Admission.ID] == nil
			service.mu.Unlock()
			if !terminal || !bounded || model.calls.Load() != 0 {
				t.Fatal("terminal cleanup lost its authoritative state")
			}
		})
	}
}

type failedTerminalRepository struct{ *fakeRepository }

func (*failedTerminalRepository) FinishAssistant(context.Context, assistant.Finish) (assistant.Run, error) {
	return assistant.Run{}, assistant.ErrUnavailable
}

func TestFailedTerminalCommitCannotMakeRunningQuestionEvictable(t *testing.T) {
	t.Parallel()
	service, repository, _, _ := fixtureService(t, nil)
	service.repository = &failedTerminalRepository{fakeRepository: repository}
	service.collector = collectorFunc(func(context.Context, assistantevidence.ReadRequest) (assistantevidence.Evidence, error) {
		return assistantevidence.Evidence{}, errors.New("fixture read unavailable")
	})
	first := inputFixture(t, 1)
	if _, err := service.Submit(context.Background(), first); err != nil {
		t.Fatal("fixture admission failed")
	}
	if _, err := service.runOne(context.Background(), "worker"); err == nil {
		t.Fatal("fixture read did not fail")
	}
	for index := 1; index < MaximumOwnerMemoryRuns; index++ {
		input := inputFixture(t, index+1)
		input.Admission.ConversationID = fmt.Sprintf("conversation_%d", index)
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal("queued fixture admission failed")
		}
	}
	input := inputFixture(t, 20)
	input.Admission.ConversationID = first.Admission.ConversationID
	if _, err := service.Submit(context.Background(), input); err != assistant.ErrQuota {
		t.Fatal("failed terminal commit freed a still-running slot")
	}
	service.mu.Lock()
	entry := service.entries[first.Admission.ID]
	protected := entry != nil && entry.run.State == assistant.Running && entry.revoked && len(service.entries) == MaximumOwnerMemoryRuns
	service.mu.Unlock()
	if !protected {
		t.Fatal("uncommitted terminal state was used as eviction authority")
	}
}
