package observation

import (
	"context"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// Retention delegates each bounded batch to the effective persisted policy.
type RetentionOptions struct {
	Repository     domain.ObservationRetentionRepository
	Now            func() time.Time
	OnError        func(error)
	PollInterval   time.Duration
	Timeout        time.Duration
	BatchLimit     int
	MaximumBatches int
}

type RetentionWorker struct {
	repository     domain.ObservationRetentionRepository
	now            func() time.Time
	onError        func(error)
	interval       time.Duration
	timeout        time.Duration
	limit          int
	maximumBatches int
	gate           chan struct{}
}

func NewRetentionWorker(options RetentionOptions) (*RetentionWorker, error) {
	if options.Repository == nil || options.Now == nil || options.PollInterval < time.Second || options.PollInterval > time.Hour ||
		options.Timeout < 100*time.Millisecond || options.Timeout > time.Minute || options.BatchLimit < 4 || options.BatchLimit > 1000 ||
		options.MaximumBatches < 1 || options.MaximumBatches > 100 {
		return nil, ErrInvalidOptions
	}
	if options.OnError == nil {
		options.OnError = func(error) {}
	}
	return &RetentionWorker{repository: options.Repository, now: options.Now, onError: options.OnError,
		interval: options.PollInterval, timeout: options.Timeout, limit: options.BatchLimit, maximumBatches: options.MaximumBatches, gate: make(chan struct{}, 1)}, nil
}

func (worker *RetentionWorker) Run(ctx context.Context) {
	if worker == nil || ctx == nil {
		return
	}
	worker.runAndReport(ctx)
	ticker := time.NewTicker(worker.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			worker.runAndReport(ctx)
		}
	}
}

func (worker *RetentionWorker) runAndReport(ctx context.Context) {
	if _, err := worker.RunOnce(ctx); err != nil && ctx.Err() == nil {
		worker.onError(err)
	}
}

func (worker *RetentionWorker) RunOnce(ctx context.Context) (int64, error) {
	if worker == nil || ctx == nil || ctx.Err() != nil {
		return 0, ErrUnavailable
	}
	select {
	case worker.gate <- struct{}{}:
		defer func() { <-worker.gate }()
	case <-ctx.Done():
		return 0, ErrUnavailable
	default:
		return 0, ErrUnavailable
	}
	now := worker.now().UTC().Truncate(time.Microsecond)
	if now.IsZero() || now.UnixMicro() < 0 {
		return 0, ErrInvalidOptions
	}
	pruneContext, cancel := context.WithTimeout(ctx, worker.timeout)
	defer cancel()
	var total int64
	for range worker.maximumBatches {
		removed, err := worker.repository.PruneObservationHistory(pruneContext, now, worker.limit)
		if removed < 0 || removed > int64(worker.limit) {
			return total, ErrUnavailable
		}
		total += removed
		if err != nil {
			return total, err
		}
		if removed == 0 {
			break
		}
	}
	return total, nil
}
