package assistantservice

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
)

func TestOwnerMetadataPollingNeverReadsProvidersOrErasesQueuedInput(t *testing.T) {
	for _, state := range []assistant.State{assistant.Queued, assistant.Running, assistant.Completed, assistant.Failed, assistant.Cancelled, assistant.Expired, assistant.Interrupted, assistant.Uncertain} {
		t.Run(string(state), func(t *testing.T) {
			service, repository, authorizer, model := fixtureService(t, nil)
			input := inputFixture(t, 1)
			if _, err := service.Submit(context.Background(), input); err != nil {
				t.Fatal(err)
			}
			repository.mu.Lock()
			run := repository.runs[input.Admission.ID]
			run.State = state
			repository.runs[run.ID] = run
			repository.mu.Unlock()
			before := authorizer.calls.Load()
			authorizer.denied.Store(true)
			snapshot, err := service.Get(context.Background(), input.Admission.Actor, run.ID)
			if err != nil || snapshot.Run.State != state || snapshot.Answer != nil || len(snapshot.Citations) != 0 || authorizer.calls.Load() != before || model.calls.Load() != 0 || len(repository.trace) != 0 {
				t.Fatal("metadata polling depended on content/provider authority")
			}
			service.mu.Lock()
			question := service.entries[run.ID].input.Question
			revoked := service.entries[run.ID].revoked
			service.mu.Unlock()
			if question == "" || revoked {
				t.Fatal("metadata poll erased staged private input")
			}
		})
	}
}

func TestProviderPermissionDenialInterruptsBeforeDispatch(t *testing.T) {
	service, repository, authorizer, model := fixtureService(t, nil)
	authorizer.reads.beforeLive = func(context.Context) error { return assistantevidence.ErrDenied }
	input := inputFixture(t, 1)
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if _, err := service.runOne(context.Background(), "worker"); err != assistant.ErrDenied {
		t.Fatal("provider denial lost its fixed code")
	}
	repository.mu.Lock()
	run := repository.runs[input.Admission.ID]
	repository.mu.Unlock()
	if run.State != assistant.Interrupted || model.calls.Load() != 0 {
		t.Fatal("permission loss dispatched work or became success")
	}
}

type revokeBeforeCountingRepository struct {
	*fakeRepository
	revoke func()
}

func (repository *revokeBeforeCountingRepository) BeginAssistantStep(ctx context.Context, input assistant.StepStart) (assistant.Run, assistant.Step, error) {
	run, step, err := repository.fakeRepository.BeginAssistantStep(ctx, input)
	if err == nil && input.Kind == assistant.Inference {
		repository.revoke()
	}
	return run, step, err
}

func TestPermissionRevokedAfterReservationNeverSendsContentToCount(t *testing.T) {
	service, repository, authorizer, model := fixtureService(t, nil)
	var revoked atomic.Bool
	authorizer.reads.beforeLive = func(context.Context) error {
		if revoked.Load() {
			return assistantevidence.ErrDenied
		}
		return nil
	}
	service.repository = &revokeBeforeCountingRepository{fakeRepository: repository, revoke: func() { revoked.Store(true) }}
	input := inputFixture(t, 1)
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if _, err := service.runOne(context.Background(), "worker"); err != assistant.ErrDenied || model.counts.Load() != 0 || model.calls.Load() != 0 {
		t.Fatal("revoked content reached template/token counting")
	}
}

func TestHeartbeatStoresAuthoritativeLeaseAcrossOriginalExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, repository, _, _ := fixtureService(t, nil)
		input := inputFixture(t, 1)
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		run, found, err := repository.ClaimAssistant(context.Background(), assistant.Claim{Worker: "worker", ProcessID: service.processID, At: service.time(), Lease: assistant.MaximumLeaseDuration})
		if err != nil || !found {
			t.Fatal("claim failed")
		}
		service.mu.Lock()
		entry := service.entries[run.ID]
		entry.run = run
		service.mu.Unlock()
		if _, err := service.begin(context.Background(), entry, assistant.Inventory, 0); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			time.Sleep(20 * time.Second)
			if !service.renewOnce(context.Background(), entry) {
				t.Fatal("renewal failed")
			}
		}
		service.mu.Lock()
		latest := entry.run
		service.mu.Unlock()
		repository.mu.Lock()
		stored := repository.runs[run.ID]
		repository.mu.Unlock()
		if latest.LeaseUntil != stored.LeaseUntil || !latest.LeaseUntil.After(run.LeaseUntil) || !service.time().After(run.LeaseUntil) {
			t.Fatal("renewed lease snapshot was discarded")
		}
		if _, err := service.collect(context.Background(), entry); err != nil {
			t.Fatal("old lease snapshot prevented a valid bounded read")
		}
	})
}

func TestProviderReadLeavesHeartbeatFreeAndKeepsOneLogicalRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started, release := make(chan struct{}), make(chan struct{})
		service, repository, _, model := fixtureService(t, func(options *Options, _ *modelFixture) {
			collector := options.Collector
			options.Collector = collectorFunc(func(ctx context.Context, request assistantevidence.ReadRequest) (assistantevidence.Evidence, error) {
				close(started)
				select {
				case <-release:
					return collector.Read(ctx, request)
				case <-ctx.Done():
					return assistantevidence.Evidence{}, ctx.Err()
				}
			})
		})
		input := inputFixture(t, 1)
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		finished := make(chan error, 1)
		go func() { _, err := service.runOne(context.Background(), "worker"); finished <- err }()
		<-started
		service.mu.Lock()
		entry := service.entries[input.Admission.ID]
		first := entry.run.LeaseUntil
		service.mu.Unlock()
		time.Sleep(2 * time.Second)
		synctest.Wait()
		service.mu.Lock()
		renewed := entry.run.LeaseUntil
		service.mu.Unlock()
		if !renewed.After(first) {
			t.Fatal("provider read starved heartbeat")
		}
		close(release)
		if err := <-finished; err != nil {
			t.Fatal("renewed read failed", err)
		}
		snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
		if err != nil || snapshot.Answer == nil || snapshot.Run.ToolCalls != 1 || snapshot.Run.InferenceCalls != 1 || model.calls.Load() != 1 {
			t.Fatal("heartbeat caused duplicate work or lost output")
		}
		repository.mu.Lock()
		reads := 0
		for _, step := range repository.trace {
			if step == "read:inventory" {
				reads++
			}
		}
		repository.mu.Unlock()
		if reads != 1 {
			t.Fatal("authority probes became logical tool calls")
		}
	})
}

func TestShutdownClearsSealAndCancelsBlockedEvidenceRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started, cancelled := make(chan struct{}), make(chan struct{})
		service, _, _, model := fixtureService(t, func(options *Options, _ *modelFixture) {
			options.Collector = collectorFunc(func(ctx context.Context, _ assistantevidence.ReadRequest) (assistantevidence.Evidence, error) {
				close(started)
				<-ctx.Done()
				close(cancelled)
				return assistantevidence.Evidence{}, ctx.Err()
			})
		})
		input := inputFixture(t, 1)
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		finished := make(chan error, 1)
		go func() { _, err := service.runOne(context.Background(), "worker"); finished <- err }()
		<-started
		service.mu.Lock()
		entry := service.entries[input.Admission.ID]
		service.mu.Unlock()
		service.Close()
		synctest.Wait()
		<-cancelled
		if err := <-finished; err == nil {
			t.Fatal("shutdown accepted blocked read")
		}
		service.mu.Lock()
		cleared := entry.revoked && entry.evidence == nil && entry.answer == nil && len(entry.citations) == 0 && entry.input.Question == ""
		service.mu.Unlock()
		if !cleared || model.calls.Load() != 0 {
			t.Fatal("shutdown retained seal or dispatched after read cancellation")
		}
	})
}

func TestTwoProviderSlotsLeaveRenewalMetadataAndCancellationFree(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started, release, generated := make(chan struct{}, 2), make(chan struct{}), make(chan struct{})
		service, _, authorizer, model := fixtureService(t, func(options *Options, model *modelFixture) {
			options.Workers = 2
			model.hook = func(context.Context) error { close(generated); return nil }
		})
		var probes atomic.Int32
		authorizer.reads.beforeLive = func(ctx context.Context) error {
			if probes.Add(1) > 2 {
				return nil
			}
			started <- struct{}{}
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		first, second := inputFixture(t, 1), inputFixture(t, 2)
		for _, input := range []Input{first, second} {
			if _, err := service.Submit(context.Background(), input); err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- service.Run(ctx) }()
		<-started
		<-started
		service.mu.Lock()
		initialFirst := service.entries[first.Admission.ID].run.LeaseUntil
		initialSecond := service.entries[second.Admission.ID].run.LeaseUntil
		service.mu.Unlock()
		time.Sleep(2 * time.Second)
		synctest.Wait()
		service.mu.Lock()
		freshFirst := service.entries[first.Admission.ID].run.LeaseUntil
		freshSecond := service.entries[second.Admission.ID].run.LeaseUntil
		service.mu.Unlock()
		if !freshFirst.After(initialFirst) || !freshSecond.After(initialSecond) {
			t.Fatal("two provider reads starved lease authority")
		}
		for _, input := range []Input{first, second} {
			snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
			if err != nil || snapshot.Run.State != assistant.Running || snapshot.Answer != nil {
				t.Fatal("metadata waited for provider slots")
			}
		}
		if run, err := service.Cancel(context.Background(), first.Admission.Actor, first.Admission.ID); err != nil || run.State != assistant.Cancelled {
			t.Fatal("cancellation waited for provider slots")
		}
		synctest.Wait()
		service.mu.Lock()
		cleared := service.entries[first.Admission.ID].revoked && service.entries[first.Admission.ID].evidence == nil
		service.mu.Unlock()
		if !cleared {
			t.Fatal("cancel did not fence blocked read")
		}
		close(release)
		<-generated
		synctest.Wait()
		snapshot, err := service.Get(context.Background(), second.Admission.Actor, second.Admission.ID)
		if err != nil || snapshot.Run.State != assistant.Completed || snapshot.Answer == nil || snapshot.Run.ToolCalls != 1 || snapshot.Run.InferenceCalls != 1 || model.calls.Load() != 1 {
			t.Fatal("live sibling lost result or duplicated a reserved call")
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}
