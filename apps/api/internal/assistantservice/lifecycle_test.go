package assistantservice

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
)

type heartbeatContextKey struct{}

func TestInFlightHeartbeatSerializesStateAndFencesObservedDenial(t *testing.T) {
	for _, failure := range []string{"authority", "lease", "none"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			modelStarted, releaseModel := make(chan struct{}), make(chan struct{})
			observed, releaseAuthority := make(chan struct{}), make(chan struct{})
			postProof := make(chan struct{})
			var modelReleased atomic.Bool
			var proofOnce sync.Once
			var observeOnce sync.Once
			service, repository, _, _ := fixtureService(t, func(_ *Options, model *modelFixture) {
				model.hook = func(ctx context.Context) error {
					close(modelStarted)
					select {
					case <-releaseModel:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			})
			renewal := &renewalRepository{fakeRepository: repository, renewed: make(chan struct{})}
			if failure == "lease" {
				renewal.err = assistant.ErrConflict
			}
			service.repository = renewal
			t.Cleanup(func() {
				for _, signal := range []chan struct{}{releaseModel, releaseAuthority} {
					select {
					case <-signal:
					default:
						close(signal)
					}
				}
			})
			service.authorizer = withAuthorizer(service.authorizer, func(ctx context.Context, _ assistant.Actor, _ assistant.Run, _ contextbuilder.Scope) error {
				if ctx.Value(heartbeatContextKey{}) == true {
					observeOnce.Do(func() { close(observed) })
					select {
					case <-releaseAuthority:
						if failure == "authority" {
							return assistant.ErrDenied
						}
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				if modelReleased.Load() {
					proofOnce.Do(func() { close(postProof) })
				}
				return nil
			})
			input := inputFixture(t, 1)
			if _, err := service.Submit(context.Background(), input); err != nil {
				t.Fatal(err)
			}
			run, found, err := repository.ClaimAssistant(context.Background(), assistant.Claim{Worker: "worker", ProcessID: service.processID, At: service.time(), Lease: assistant.MaximumLeaseDuration})
			if err != nil || !found {
				t.Fatal("fixture failed to claim admitted run")
			}
			work, cancel := context.WithCancel(context.Background())
			defer cancel()
			service.mu.Lock()
			entry := service.entries[run.ID]
			entry.run, entry.active, entry.cancel = run, true, cancel
			service.mu.Unlock()
			finished := make(chan error, 1)
			go func() { finished <- service.execute(work, entry) }()
			waitSignal(t, modelStarted)
			stop, renewed := make(chan struct{}), make(chan struct{})
			go service.renew(context.WithValue(work, heartbeatContextKey{}, true), entry, stop, renewed)
			waitSignal(t, observed)
			modelReleased.Store(true)
			close(releaseModel)
			waitSignal(t, postProof)
			service.mu.Lock()
			answerBefore, stateBefore := entry.answer, entry.run.State
			service.mu.Unlock()
			if answerBefore != nil || stateBefore != assistant.Running {
				t.Fatal("state mutation crossed pending metadata renewal")
			}
			close(releaseAuthority)
			var executionError error
			select {
			case executionError = <-finished:
			case <-time.After(2 * time.Second):
				t.Fatal("serialized worker did not finish")
			}
			close(stop)
			waitSignal(t, renewed)
			service.mu.Lock()
			revoked, answer := entry.revoked, entry.answer
			service.mu.Unlock()
			denied := failure != "none"
			if (executionError != nil) != denied || revoked != denied || (answer == nil) != denied || (work.Err() != nil) != denied {
				t.Fatal("observed failure or successful renewal crossed the content fence")
			}
		})
	}
}

type renewalRepository struct {
	*fakeRepository
	renewed chan struct{}
	err     error
	calls   atomic.Int32
}

func (repository *renewalRepository) RenewAssistantLease(ctx context.Context, lease assistant.Lease, duration time.Duration) (assistant.Run, error) {
	if repository.calls.Add(1) == 1 {
		close(repository.renewed)
	}
	if repository.err != nil {
		return assistant.Run{}, repository.err
	}
	return repository.fakeRepository.RenewAssistantLease(ctx, lease, duration)
}

func TestHeartbeatRenewsThenStopsOnLostAuthorityOrLease(t *testing.T) {
	for _, failure := range []string{"authority", "lease"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			started := make(chan struct{})
			service, repository, authorizer, model := fixtureService(t, func(_ *Options, model *modelFixture) {
				model.hook = func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() }
			})
			renewal := &renewalRepository{fakeRepository: repository, renewed: make(chan struct{})}
			if failure == "lease" {
				renewal.err = assistant.ErrConflict
			}
			service.repository = renewal
			input := inputFixture(t, 1)
			if _, err := service.Submit(context.Background(), input); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { _, err := service.runOne(context.Background(), "worker"); done <- err }()
			waitSignal(t, started)
			waitSignal(t, renewal.renewed)
			if failure == "authority" {
				authorizer.denied.Store(true)
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("lost authority or lease published success")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("observed heartbeat failure left network work alive")
			}
			service.mu.Lock()
			entry := service.entries[input.Admission.ID]
			cleared := entry.revoked && entry.input.Question == "" && entry.answer == nil && entry.cancel == nil
			service.mu.Unlock()
			if !cleared || model.calls.Load() != 1 {
				t.Fatal("heartbeat failure retained text or retried inference")
			}
		})
	}
}

func TestMemoryExpiryCancelsActiveWorkAndCannotBeReadBack(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	var offset atomic.Int64
	service, _, _, _ := fixtureService(t, func(options *Options, model *modelFixture) {
		options.Now = func() time.Time { return time.Now().Add(time.Duration(offset.Load())) }
		model.hook = func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() }
	})
	input := inputFixture(t, 1)
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := service.runOne(context.Background(), "worker"); done <- err }()
	waitSignal(t, started)
	offset.Store(int64(MemoryLifetime + time.Second))
	service.prune()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expired memory retained an active task")
	}
	snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
	if err != nil || snapshot.Answer != nil || snapshot.Volatile || snapshot.Run.State != assistant.Expired {
		t.Fatal("expired active answer was exposed")
	}
}

func TestUserRevocationAndReadAuthorizationFenceCompletedAnswer(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"user", "read"} {
		service, _, authorizer, _ := fixtureService(t, nil)
		input := inputFixture(t, 1)
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		if _, err := service.runOne(context.Background(), "worker"); err != nil {
			t.Fatal(err)
		}
		if mode == "user" {
			service.InvalidateUser(input.Admission.Actor.UserID)
		} else {
			authorizer.denied.Store(true)
		}
		snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
		if mode == "read" && err != assistant.ErrDenied || snapshot.Answer != nil || snapshot.Volatile {
			t.Fatal("revoked completed answer escaped its memory fence")
		}
	}
}
