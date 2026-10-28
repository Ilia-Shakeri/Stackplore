package assistantservice

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestLifetimeLossClearsQueuedAndCompletedVolatileStateBeforeRun(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lifetime, cancel := context.WithCancel(context.Background())
		defer cancel()
		service, _, _, model := fixtureService(t, func(options *Options, _ *modelFixture) { options.Lifetime = lifetime })
		first, queued := inputFixture(t, 1), inputFixture(t, 2)
		if _, err := service.Submit(context.Background(), first); err != nil {
			t.Fatal(err)
		}
		if _, err := service.runOne(context.Background(), "worker"); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Submit(context.Background(), queued); err != nil {
			t.Fatal(err)
		}
		service.mu.Lock()
		completedEntry, queuedEntry := service.entries[first.Admission.ID], service.entries[queued.Admission.ID]
		service.mu.Unlock()
		cancel()
		synctest.Wait()
		service.mu.Lock()
		cleared := service.closed && service.key == [32]byte{} && len(service.entries) == 0 && len(service.conversations) == 0
		for _, entry := range []*memoryRun{completedEntry, queuedEntry} {
			cleared = cleared && entry.revoked && entry.input.Question == "" && entry.input.Admission.Actor == (assistant.Actor{}) && len(entry.input.Scope.Resources) == 0 && entry.answer == nil && len(entry.citations) == 0
		}
		service.mu.Unlock()
		if !cleared || model.calls.Load() != 1 {
			t.Fatal("ownership loss retained a completed answer or queued volatile input")
		}
		if err := service.Run(context.Background()); err != assistant.ErrUnavailable {
			t.Fatal("an already closed service started workers")
		}
		if _, err := service.Submit(context.Background(), inputFixture(t, 3)); err != assistant.ErrUnavailable {
			t.Fatal("ownership loss accepted fresh text")
		}
	})
}

func TestRunShutdownFencesMemoryThenJoinsActiveProducer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
		service, _, _, model := fixtureService(t, func(_ *Options, model *modelFixture) {
			model.hook = func(ctx context.Context) error {
				close(started)
				<-ctx.Done()
				close(cancelled)
				<-release
				return ctx.Err()
			}
		})
		input := inputFixture(t, 1)
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- service.Run(ctx) }()
		<-started
		cancel()
		synctest.Wait()
		<-cancelled
		service.mu.Lock()
		cleared := service.closed && len(service.entries) == 0 && len(service.conversations) == 0 && service.key == [32]byte{}
		service.mu.Unlock()
		if !cleared {
			t.Fatal("shutdown waited for external work before releasing owned private references")
		}
		select {
		case <-done:
			t.Fatal("Run returned with an active producer")
		default:
		}
		close(release)
		if err := <-done; err != nil || model.calls.Load() != 1 {
			t.Fatal("shutdown failed to join or repeated dispatched work")
		}
		service.Close()
		service.Close()
	})
}

type closingCancellationRepository struct {
	*fakeRepository
	started chan struct{}
	release chan struct{}
}

func (repository *closingCancellationRepository) CancelAssistant(ctx context.Context, actor assistant.Actor, id string, at time.Time) (assistant.Run, error) {
	close(repository.started)
	<-repository.release
	return repository.fakeRepository.CancelAssistant(ctx, actor, id, at)
}

func TestConcurrentCloseAndCancellationCannotReadClearedOwner(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, repository, _, _ := fixtureService(t, nil)
		closing := &closingCancellationRepository{fakeRepository: repository, started: make(chan struct{}), release: make(chan struct{})}
		service.repository = closing
		input := inputFixture(t, 1)
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			_, err := service.Cancel(context.Background(), input.Admission.Actor, input.Admission.ID)
			done <- err
		}()
		<-closing.started
		service.Close()
		close(closing.release)
		if err := <-done; err != nil && err != assistant.ErrDenied && err != assistant.ErrUnavailable {
			t.Fatal("concurrent close lost its safe cancellation boundary")
		}
		service.mu.Lock()
		cleared := len(service.entries) == 0 && len(service.conversations) == 0
		service.mu.Unlock()
		if !cleared {
			t.Fatal("cancellation resurrected cleared volatile state")
		}
	})
}

type postCloseAdmissionRepository struct {
	*fakeRepository
	committed chan struct{}
	release   chan struct{}
}

func (repository *postCloseAdmissionRepository) AdmitAssistantWithOutcome(ctx context.Context, admission assistant.Admission) (assistant.AdmissionResult, error) {
	outcome, err := repository.fakeRepository.AdmitAssistantWithOutcome(ctx, admission)
	if err != nil {
		return outcome, err
	}
	close(repository.committed)
	<-repository.release
	return outcome, nil
}

func TestLateAdmissionCommitCannotRestoreClosedConversation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, repository, _, model := fixtureService(t, nil)
		late := &postCloseAdmissionRepository{fakeRepository: repository, committed: make(chan struct{}), release: make(chan struct{})}
		service.repository = late
		done := make(chan error, 1)
		go func() { _, err := service.Submit(context.Background(), inputFixture(t, 1)); done <- err }()
		<-late.committed
		service.Close()
		close(late.release)
		if err := <-done; err != assistant.ErrUnavailable {
			t.Fatal("late durable admission reopened a closed volatile conversation")
		}
		service.mu.Lock()
		cleared := len(service.entries) == 0 && len(service.conversations) == 0 && service.key == [32]byte{}
		service.mu.Unlock()
		if !cleared || model.counts.Load() != 0 || model.calls.Load() != 0 {
			t.Fatal("late commit retained text or dispatched work after shutdown")
		}
	})
}
