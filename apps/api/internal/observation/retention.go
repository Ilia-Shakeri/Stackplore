package observation

import (
	"context"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// Retention uses fixed deployment policy. Audit and current records are excluded
// from its domain port; changing policy belongs to the guarded settings use case.
type RetentionOptions struct {
	Repository     domain.ObservationRetentionRepository
	Now            func() time.Time
	OnError        func(error)
	PollInterval   time.Duration
	Timeout        time.Duration
	BatchLimit     int
	MaximumBatches int
	MetricAge      time.Duration
	HealthAge      time.Duration
	EventAge       time.Duration
	DeliveryAge    time.Duration
}

type RetentionWorker struct {
	repository     domain.ObservationRetentionRepository
	now            func() time.Time
	onError        func(error)
	interval       time.Duration
	timeout        time.Duration
	limit          int
	maximumBatches int
	metricAge      time.Duration
	healthAge      time.Duration
	eventAge       time.Duration
	deliveryAge    time.Duration
	gate           chan struct{}
}

func NewRetentionWorker(options RetentionOptions) (*RetentionWorker, error) {
	if options.Repository == nil || options.Now == nil || options.PollInterval < time.Second || options.PollInterval > time.Hour ||
		options.Timeout < 100*time.Millisecond || options.Timeout > time.Minute || options.BatchLimit < 4 || options.BatchLimit > 1000 ||
		options.MaximumBatches < 1 || options.MaximumBatches > 100 ||
		!validRetentionAge(options.MetricAge) || !validRetentionAge(options.HealthAge) || !validRetentionAge(options.EventAge) || !validRetentionAge(options.DeliveryAge) {
		return nil, ErrInvalidOptions
	}
	if options.OnError == nil {
		options.OnError = func(error) {}
	}
	return &RetentionWorker{repository: options.Repository, now: options.Now, onError: options.OnError,
		interval: options.PollInterval, timeout: options.Timeout, limit: options.BatchLimit, maximumBatches: options.MaximumBatches, metricAge: options.MetricAge,
		healthAge: options.HealthAge, eventAge: options.EventAge, deliveryAge: options.DeliveryAge, gate: make(chan struct{}, 1)}, nil
}

func validRetentionAge(value time.Duration) bool {
	return value >= time.Hour && value <= 365*24*time.Hour && value%time.Second == 0
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
	policy := domain.ObservationRetentionPolicy{MetricsBefore: now.Add(-worker.metricAge), HealthBefore: now.Add(-worker.healthAge),
		EventsBefore: now.Add(-worker.eventAge), DeliveriesBefore: now.Add(-worker.deliveryAge)}
	if policy.MetricsBefore.Before(time.Unix(0, 0)) || policy.HealthBefore.Before(time.Unix(0, 0)) || policy.EventsBefore.Before(time.Unix(0, 0)) || policy.DeliveriesBefore.Before(time.Unix(0, 0)) {
		return 0, ErrInvalidOptions
	}
	pruneContext, cancel := context.WithTimeout(ctx, worker.timeout)
	defer cancel()
	var total int64
	for range worker.maximumBatches {
		removed, err := worker.repository.PruneObservationHistory(pruneContext, policy, worker.limit)
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
