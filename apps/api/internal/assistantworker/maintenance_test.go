package assistantworker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

type maintenanceCall struct {
	at       time.Time
	limit    int
	deadline time.Time
}

type maintenanceRepository struct {
	mu       sync.Mutex
	recovery []maintenanceCall
	pruning  []maintenanceCall
	recover  func(context.Context) (int, error)
	prune    func(context.Context) (int, error)
}

func (repository *maintenanceRepository) RecoverAssistant(ctx context.Context, at time.Time, limit int) (int, error) {
	deadline, _ := ctx.Deadline()
	repository.mu.Lock()
	repository.recovery = append(repository.recovery, maintenanceCall{at, limit, deadline})
	repository.mu.Unlock()
	if repository.recover != nil {
		return repository.recover(ctx)
	}
	return 0, nil
}

func (repository *maintenanceRepository) PruneAssistant(ctx context.Context, at time.Time, limit int) (int, error) {
	deadline, _ := ctx.Deadline()
	repository.mu.Lock()
	repository.pruning = append(repository.pruning, maintenanceCall{at, limit, deadline})
	repository.mu.Unlock()
	if repository.prune != nil {
		return repository.prune(ctx)
	}
	return 0, nil
}

func (repository *maintenanceRepository) counts() (int, int) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return len(repository.recovery), len(repository.pruning)
}

func TestDisabledFeatureMaintenanceBoundsAndNoCatchUpBurst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repository := &maintenanceRepository{prune: func(context.Context) (int, error) { return MaximumRetentionBatch, nil }}
		worker, err := New(Options{Repository: repository})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- worker.Run(ctx) }()
		synctest.Wait()
		if recovery, pruning := repository.counts(); recovery != 1 || pruning != 1 {
			t.Fatal("startup did not perform one bounded maintenance pass without a dispatcher")
		}
		if err := worker.Run(ctx); err != assistant.ErrConflict {
			t.Fatal("a second scheduler overlapped maintenance")
		}
		time.Sleep(59 * time.Second)
		synctest.Wait()
		if _, pruning := repository.counts(); pruning != 1 {
			t.Fatal("a full batch caused an immediate drain loop")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if recovery, pruning := repository.counts(); recovery != 61 || pruning != 2 {
			t.Fatal("bounded periodic recovery or retention stopped")
		}
		repository.mu.Lock()
		for _, call := range repository.recovery {
			if call.limit != 18 || !assistant.Timestamp(call.at) || call.deadline.Sub(call.at) > maximumIOTimeout+time.Microsecond {
				t.Fatal("recovery had an unbounded batch, timestamp or deadline")
			}
		}
		for _, call := range repository.pruning {
			if call.limit != 128 {
				t.Fatal("retention exceeded the fixed small batch")
			}
		}
		repository.mu.Unlock()
		if snapshot := worker.LastResult(); snapshot.Recovery.Outcome != Succeeded || snapshot.Retention.Outcome != Succeeded || snapshot.Retention.Count != 128 {
			t.Fatal("safe last-result metadata lost a successful batch")
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		beforeRecovery, beforePruning := repository.counts()
		time.Sleep(time.Minute)
		if recovery, pruning := repository.counts(); recovery != beforeRecovery || pruning != beforePruning {
			t.Fatal("a stopped worker retained a scheduler")
		}
	})
}

func TestTransientFailureIsSafeAndRetriesOnNextScheduledTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var failed atomic.Bool
		failed.Store(true)
		operation := func(context.Context) (int, error) {
			if failed.Load() {
				return 999, errors.New("private SQL detail")
			}
			return 1, nil
		}
		repository := &maintenanceRepository{recover: operation, prune: operation}
		worker, err := New(Options{Repository: repository, RecoveryInterval: time.Second, RetentionInterval: time.Second, IOTimeout: time.Second, RecoveryBatch: 1, RetentionBatch: 1})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- worker.Run(ctx) }()
		synctest.Wait()
		snapshot := worker.LastResult()
		body, _ := json.Marshal(snapshot)
		if snapshot.Recovery.Outcome != Unavailable || snapshot.Retention.Outcome != Unavailable || snapshot.Recovery.Count != 0 || snapshot.Retention.Count != 0 || strings.Contains(string(body), "private") {
			t.Fatal("transient failure leaked or became a successful count")
		}
		failed.Store(false)
		time.Sleep(time.Second)
		synctest.Wait()
		if snapshot = worker.LastResult(); snapshot.Recovery.Outcome != Succeeded || snapshot.Retention.Outcome != Succeeded || snapshot.Recovery.Count != 1 || snapshot.Retention.Count != 1 {
			t.Fatal("a transient failure permanently stopped maintenance")
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}

func TestOwnedContextCancellationJoinsInFlightStorage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		observed, release := make(chan struct{}), make(chan struct{})
		repository := &maintenanceRepository{recover: func(ctx context.Context) (int, error) {
			<-ctx.Done()
			close(observed)
			<-release
			return 0, ctx.Err()
		}}
		worker, _ := New(Options{Repository: repository})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- worker.Run(ctx) }()
		synctest.Wait()
		cancel()
		synctest.Wait()
		<-observed
		select {
		case <-done:
			t.Fatal("Run returned before its owned storage call stopped")
		default:
		}
		close(release)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if _, pruning := repository.counts(); pruning != 0 || worker.LastResult().Recovery.Outcome != Cancelled {
			t.Fatal("ownership loss started more I/O or hid cancellation")
		}
	})
}

func TestStorageTimeoutCannotBecomeSuccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repository := &maintenanceRepository{recover: func(ctx context.Context) (int, error) {
			<-ctx.Done()
			return 1, nil
		}}
		worker, _ := New(Options{Repository: repository})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- worker.Run(ctx) }()
		synctest.Wait()
		time.Sleep(maximumIOTimeout)
		synctest.Wait()
		if snapshot := worker.LastResult(); snapshot.Recovery.Outcome != Unavailable || snapshot.Recovery.Count != 0 || snapshot.Retention.Outcome != Succeeded {
			t.Fatal("late storage success bypassed its deadline or blocked retention")
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}

func TestInvalidClockAndRepositoryCountsFailClosed(t *testing.T) {
	for _, scenario := range []string{"clock", "negative", "too_many"} {
		t.Run(scenario, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				count := -1
				if scenario == "too_many" {
					count = 1000
				}
				repository := &maintenanceRepository{recover: func(context.Context) (int, error) { return count, nil }, prune: func(context.Context) (int, error) { return count, nil }}
				options := Options{Repository: repository}
				if scenario == "clock" {
					options.Now = func() time.Time { return time.Time{} }
				}
				worker, _ := New(options)
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan error, 1)
				go func() { done <- worker.Run(ctx) }()
				synctest.Wait()
				want := InvalidResult
				if scenario == "clock" {
					want = InvalidClock
					if recovery, pruning := repository.counts(); recovery != 0 || pruning != 0 {
						t.Fatal("invalid clock reached durable storage")
					}
				}
				if snapshot := worker.LastResult(); snapshot.Recovery.Outcome != want || snapshot.Retention.Outcome != want || snapshot.Recovery.Count != 0 || snapshot.Retention.Count != 0 {
					t.Fatal("invalid maintenance result was accepted")
				}
				cancel()
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

func TestMaintenanceConstructionAndCancelledStartup(t *testing.T) {
	repository := &maintenanceRepository{}
	for _, alter := range []func(*Options){
		func(options *Options) { options.Repository = nil },
		func(options *Options) { options.RecoveryInterval = time.Millisecond },
		func(options *Options) { options.RecoveryInterval = time.Minute + time.Nanosecond },
		func(options *Options) { options.RetentionInterval = time.Millisecond },
		func(options *Options) { options.RetentionInterval = time.Minute + time.Nanosecond },
		func(options *Options) { options.IOTimeout = time.Nanosecond },
		func(options *Options) { options.IOTimeout = maximumIOTimeout + time.Nanosecond },
		func(options *Options) { options.RecoveryBatch = -1 },
		func(options *Options) { options.RecoveryBatch = 19 },
		func(options *Options) { options.RetentionBatch = -1 },
		func(options *Options) { options.RetentionBatch = 129 },
	} {
		options := Options{Repository: repository}
		alter(&options)
		if worker, err := New(options); worker != nil || err != assistant.ErrInvalid {
			t.Fatal("unbounded maintenance configuration accepted")
		}
	}
	var absent *Worker
	if err := absent.Run(context.Background()); err != assistant.ErrInvalid || absent.LastResult() != (Snapshot{}) {
		t.Fatal("nil worker boundary changed")
	}
	worker, _ := New(Options{Repository: repository})
	if err := worker.Run(nil); err != assistant.ErrInvalid {
		t.Fatal("nil ownership context accepted")
	}
	if snapshot := worker.LastResult(); snapshot.Recovery.Outcome != NotStarted || snapshot.Retention.Outcome != NotStarted {
		t.Fatal("unstarted maintenance claimed success")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := worker.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if recovery, pruning := repository.counts(); recovery != 0 || pruning != 0 {
		t.Fatal("cancelled startup touched storage")
	}
}
