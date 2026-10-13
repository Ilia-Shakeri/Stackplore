package observation

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type retentionStore struct {
	policy   domain.ObservationRetentionPolicy
	limit    int
	removed  int64
	calls    int
	sequence []int64
}

func (store *retentionStore) PruneObservationHistory(ctx context.Context, policy domain.ObservationRetentionPolicy, limit int) (int64, error) {
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > time.Second {
		return 0, ErrUnavailable
	}
	store.policy, store.limit = policy, limit
	store.calls++
	if len(store.sequence) > 0 {
		index := min(store.calls-1, len(store.sequence)-1)
		return store.sequence[index], nil
	}
	return store.removed, nil
}

func TestRetentionWorkerPassesOnlyNamedHistoryCutoffsAndAggregateBound(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	store := &retentionStore{removed: 7}
	worker, err := NewRetentionWorker(RetentionOptions{Repository: store, Now: func() time.Time { return now }, PollInterval: time.Minute, Timeout: time.Second,
		BatchLimit: 10, MaximumBatches: 1, MetricAge: 24 * time.Hour, HealthAge: 48 * time.Hour, EventAge: 72 * time.Hour, DeliveryAge: 96 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	removed, err := worker.RunOnce(context.Background())
	if err != nil || removed != 7 || store.limit != 10 || store.policy.MetricsBefore != now.Add(-24*time.Hour) ||
		store.policy.HealthBefore != now.Add(-48*time.Hour) || store.policy.EventsBefore != now.Add(-72*time.Hour) || store.policy.DeliveriesBefore != now.Add(-96*time.Hour) {
		t.Fatal("retention policy or bound changed")
	}
	store.removed = 11
	if _, err := worker.RunOnce(context.Background()); err == nil {
		t.Fatal("retention accepted removal beyond its aggregate bound")
	}
}

func TestRetentionContinuesPartialFairBatchesUntilEmptyOrRunBound(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	store := &retentionStore{sequence: []int64{2, 1, 0}}
	options := RetentionOptions{Repository: store, Now: func() time.Time { return now }, PollInterval: time.Minute, Timeout: time.Second,
		BatchLimit: 4, MaximumBatches: 10, MetricAge: 24 * time.Hour, HealthAge: 24 * time.Hour, EventAge: 24 * time.Hour, DeliveryAge: 24 * time.Hour}
	worker, err := NewRetentionWorker(options)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := worker.RunOnce(context.Background())
	if err != nil || removed != 3 || store.calls != 3 {
		t.Fatal("fair partial batch stopped before empty")
	}
	store.calls = 0
	store.sequence = []int64{1}
	options.MaximumBatches = 2
	worker, err = NewRetentionWorker(options)
	if err != nil {
		t.Fatal(err)
	}
	removed, err = worker.RunOnce(context.Background())
	if err != nil || removed != 2 || store.calls != 2 {
		t.Fatal("retention exceeded run batch bound")
	}
	for _, bound := range []int{0, 101} {
		options.MaximumBatches = bound
		if _, err := NewRetentionWorker(options); err == nil {
			t.Fatal("unsafe retention run bound accepted")
		}
	}
	options.MaximumBatches = 2
	options.BatchLimit = 3
	if _, err := NewRetentionWorker(options); err == nil {
		t.Fatal("batch cannot fairly cover four history classes")
	}
}
