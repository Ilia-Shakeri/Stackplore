// Package assistantworker owns bounded metadata maintenance without text dispatch.
package assistantworker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

const (
	MaximumRecoveryBatch  = 18
	MaximumRetentionBatch = 128
	maximumIOTimeout      = 3 * time.Second
)

// Repository deliberately exposes no claim, provider, prompt or execution port.
// Both calls must honor cancellation and commit at most their requested batch.
type Repository interface {
	RecoverAssistant(context.Context, time.Time, int) (int, error)
	PruneAssistant(context.Context, time.Time, int) (int, error)
}

type Options struct {
	Repository        Repository
	Now               func() time.Time
	RecoveryInterval  time.Duration
	RetentionInterval time.Duration
	IOTimeout         time.Duration
	RecoveryBatch     int
	RetentionBatch    int
}

type Outcome string

const (
	NotStarted    Outcome = "not_started"
	Succeeded     Outcome = "ok"
	Unavailable   Outcome = "unavailable"
	Cancelled     Outcome = "cancelled"
	InvalidClock  Outcome = "invalid_clock"
	InvalidResult Outcome = "invalid_result"
)

// Result contains safe metadata only. Upstream error text is never retained.
type Result struct {
	AttemptedAt time.Time
	Count       int
	Outcome     Outcome
}

type Snapshot struct {
	Recovery  Result
	Retention Result
}

type Worker struct {
	repository Repository
	now        func() time.Time
	recovery   time.Duration
	retention  time.Duration
	timeout    time.Duration
	recoverN   int
	pruneN     int
	running    atomic.Bool
	mu         sync.Mutex
	last       Snapshot
}

// New creates maintenance independently of feature enablement or any dispatcher.
func New(options Options) (*Worker, error) {
	if options.Repository == nil {
		return nil, assistant.ErrInvalid
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.RecoveryInterval == 0 {
		options.RecoveryInterval = time.Second
	}
	if options.RetentionInterval == 0 {
		options.RetentionInterval = time.Minute
	}
	if options.IOTimeout == 0 {
		options.IOTimeout = maximumIOTimeout
	}
	if options.RecoveryBatch == 0 {
		options.RecoveryBatch = MaximumRecoveryBatch
	}
	if options.RetentionBatch == 0 {
		options.RetentionBatch = MaximumRetentionBatch
	}
	if options.RecoveryInterval < time.Second || options.RecoveryInterval > time.Minute || options.RetentionInterval < time.Second || options.RetentionInterval > time.Minute || options.IOTimeout < time.Millisecond || options.IOTimeout > maximumIOTimeout || options.RecoveryBatch < 1 || options.RecoveryBatch > MaximumRecoveryBatch || options.RetentionBatch < 1 || options.RetentionBatch > MaximumRetentionBatch {
		return nil, assistant.ErrInvalid
	}
	return &Worker{repository: options.Repository, now: options.Now, recovery: options.RecoveryInterval, retention: options.RetentionInterval, timeout: options.IOTimeout, recoverN: options.RecoveryBatch, pruneN: options.RetentionBatch, last: Snapshot{Recovery: Result{Outcome: NotStarted}, Retention: Result{Outcome: NotStarted}}}, nil
}

// Run performs one batch per due operation, even when admission is disabled.
// Calls are sequential; missed ticks never create catch-up bursts. The caller's
// service ownership context stops I/O, and Run joins it before returning.
func (worker *Worker) Run(ctx context.Context) error {
	if worker == nil || ctx == nil {
		return assistant.ErrInvalid
	}
	if !worker.running.CompareAndSwap(false, true) {
		return assistant.ErrConflict
	}
	nextRecovery, nextRetention := time.Now(), time.Now()
	for ctx.Err() == nil {
		if !time.Now().Before(nextRecovery) {
			worker.perform(ctx, true)
			nextRecovery = time.Now().Add(worker.recovery)
		}
		if ctx.Err() != nil {
			break
		}
		if !time.Now().Before(nextRetention) {
			worker.perform(ctx, false)
			nextRetention = time.Now().Add(worker.retention)
		}
		delay := min(time.Until(nextRecovery), time.Until(nextRetention))
		timer := time.NewTimer(max(0, delay))
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return nil
}

func (worker *Worker) perform(ctx context.Context, recovery bool) {
	result := Result{Outcome: InvalidClock}
	at := worker.now().UTC().Truncate(time.Microsecond)
	if assistant.Timestamp(at) {
		result.AttemptedAt = at
		work, cancel := context.WithTimeout(ctx, worker.timeout)
		limit, call := worker.pruneN, worker.repository.PruneAssistant
		if recovery {
			limit, call = worker.recoverN, worker.repository.RecoverAssistant
		}
		count, err := call(work, at, limit)
		callError := work.Err()
		cancel()
		switch {
		case ctx.Err() != nil:
			result.Outcome = Cancelled
		case err != nil || callError != nil:
			result.Outcome = Unavailable
		case count < 0 || count > limit:
			result.Outcome = InvalidResult
		default:
			result.Outcome, result.Count = Succeeded, count
		}
	}
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if recovery {
		worker.last.Recovery = result
	} else {
		worker.last.Retention = result
	}
}

func (worker *Worker) LastResult() Snapshot {
	if worker == nil {
		return Snapshot{}
	}
	worker.mu.Lock()
	defer worker.mu.Unlock()
	return worker.last
}
