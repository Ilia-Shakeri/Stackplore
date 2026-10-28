package assistantservice

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

// Run owns a fixed worker pool. It performs no inference without an admitted run.
func (service *Service) Run(ctx context.Context) error {
	if service == nil || ctx == nil {
		return assistant.ErrInvalid
	}
	if !service.running.CompareAndSwap(false, true) {
		return assistant.ErrConflict
	}
	defer service.Close()
	service.mu.Lock()
	closed := service.closed
	service.mu.Unlock()
	if closed {
		return assistant.ErrUnavailable
	}
	work, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(service.lifetime, cancel)
	defer stop()
	defer cancel()
	var workers sync.WaitGroup
	workers.Go(func() {
		<-work.Done()
		service.Close()
	})
	workers.Go(func() {
		ticker := time.NewTicker(service.poll)
		defer ticker.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-ticker.C:
				service.prune()
			}
		}
	})
	for index := range service.workers {
		workerID := service.processID + "_worker_" + strconv.Itoa(index)
		workers.Go(func() {
			ticker := time.NewTicker(service.poll)
			defer ticker.Stop()
			for {
				if work.Err() != nil {
					return
				}
				_, _ = service.runOne(work, workerID)
				select {
				case <-work.Done():
					return
				case <-ticker.C:
				}
			}
		})
	}
	workers.Wait()
	return nil
}

func (service *Service) runOne(ctx context.Context, workerID string) (bool, error) {
	work, cancel := service.ioContext(ctx)
	_, err := service.repository.RecoverAssistant(work, service.time(), MaximumMemoryRuns)
	if err != nil {
		cancel()
		return false, ownedError(err)
	}
	run, found, err := service.repository.ClaimAssistant(work, assistant.Claim{Worker: workerID, ProcessID: service.processID, At: service.time(), Lease: assistant.MaximumLeaseDuration})
	cancel()
	if err != nil || !found {
		if err != nil {
			return false, ownedError(err)
		}
		return false, nil
	}
	service.mu.Lock()
	entry := service.entries[run.ID]
	service.mu.Unlock()
	if entry == nil {
		service.finish(run, assistant.Interrupted)
		return true, nil
	}
	readyContext, cancelReady := service.ioContext(ctx)
	readyContext, cancelDeadline := context.WithTimeout(readyContext, max(0, min(run.Deadline.Sub(service.time()), run.LeaseUntil.Sub(service.time()))))
	defer cancelReady()
	defer cancelDeadline()
	select {
	case <-entry.ready:
	case <-readyContext.Done():
		service.invalidateEntry(entry)
		service.finish(run, assistant.Interrupted)
		return true, nil
	}
	service.mu.Lock()
	if entry.revoked || entry.active || !service.time().Before(entry.expires) || !matchesRun(run, entry.input) {
		service.mu.Unlock()
		service.invalidateEntry(entry)
		service.finish(run, assistant.Interrupted)
		return true, nil
	}
	entry.run, entry.active = run, true
	deadline := min(assistant.MaximumRunDuration, run.Deadline.Sub(service.time()))
	runContext, cancelRun := context.WithTimeout(ctx, max(0, deadline))
	entry.cancel = cancelRun
	service.mu.Unlock()
	stopHeartbeat := make(chan struct{})
	heartbeatDone := make(chan struct{})
	go service.renew(runContext, entry, stopHeartbeat, heartbeatDone)
	err = service.execute(runContext, entry)
	close(stopHeartbeat)
	<-heartbeatDone
	interrupted := runContext.Err() != nil
	cancelRun()
	service.mu.Lock()
	entry.active, entry.cancel = false, nil
	entry.finishing = false
	entry.input.Question = ""
	latest := entry.run
	if err != nil {
		service.eraseLocked(entry)
	}
	service.mu.Unlock()
	if err != nil && !latest.State.Terminal() {
		state := assistant.Failed
		if interrupted || err == assistant.ErrDenied {
			state = assistant.Interrupted
		}
		if !service.time().Before(latest.Deadline) {
			state = assistant.Expired
		}
		service.finish(latest, state)
	}
	return true, err
}

func (service *Service) renew(ctx context.Context, entry *memoryRun, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(service.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !service.renewOnce(ctx, entry) {
				return
			}
		}
	}
}

func (service *Service) renewOnce(ctx context.Context, entry *memoryRun) bool {
	unlock, err := service.lockRun(ctx, entry)
	if err != nil {
		return false
	}
	defer unlock()
	service.mu.Lock()
	run, scope, actor, revoked, finishing := entry.run, cloneScope(entry.input.Scope), entry.input.Admission.Actor, entry.revoked, entry.finishing
	input := entry.input
	service.mu.Unlock()
	if revoked || finishing {
		return false
	}
	work, cancel := service.ioContext(ctx)
	defer cancel()
	if err := service.authorizer.Authorize(work, actor, run, scope); err != nil {
		service.invalidateEntry(entry)
		return false
	}
	updated, err := service.repository.RenewAssistantLease(work, service.lease(run), assistant.MaximumLeaseDuration)
	if err != nil {
		service.invalidateEntry(entry)
		return false
	}
	if !matchesRun(updated, input) || updated.State != assistant.Running || updated.Version < run.Version || updated.Fence != run.Fence || updated.Attempts != run.Attempts || updated.LeaseOwner != run.LeaseOwner || !updated.LeaseUntil.After(service.time()) {
		service.invalidateEntry(entry)
		return false
	}
	service.mu.Lock()
	if !entry.revoked && !service.closed {
		entry.run = updated
	}
	service.mu.Unlock()
	return true
}

func matchesRun(run assistant.Run, input Input) bool {
	a := input.Admission
	return run.ID == a.ID && run.ConversationID == a.ConversationID && run.OwnerID == a.Actor.UserID && run.SourceID == a.SourceID && run.ResourceID == a.ResourceID && run.Goal == a.Goal && run.SourceVersion == a.SourceVersion && run.PolicyVersion == a.PolicyVersion && run.ProfileID == a.ProfileID && run.ProfileVersion == a.ProfileVersion && run.RestoreGeneration == a.RestoreGeneration && run.Paid == a.Paid && run.Volatile && run.ConfigurationVersion > 0 && run.Version > 0 && run.Fence > 0 && run.From.Equal(a.From) && run.Until.Equal(a.Until)
}

func (service *Service) lease(run assistant.Run) assistant.Lease {
	return assistant.Lease{RunID: run.ID, Worker: run.LeaseOwner, Version: run.Version, Fence: run.Fence, At: service.time()}
}

func (service *Service) finish(run assistant.Run, state assistant.State) {
	ctx, cancel := context.WithTimeout(context.Background(), storageTimeout)
	defer cancel()
	terminal, err := service.repository.FinishAssistant(ctx, assistant.Finish{Lease: service.lease(run), State: state})
	if err == nil {
		service.mu.Lock()
		service.rememberTerminalLocked(service.entries[run.ID], terminal)
		service.mu.Unlock()
	}
}
