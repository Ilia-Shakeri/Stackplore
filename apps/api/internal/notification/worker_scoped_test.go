package notification

import (
	"context"
	"testing"
	"time"
)

type scopedRepositoryFixture struct {
	testing     *testing.T
	claimed     []time.Time
	completions []DeliveryCompletion
}

func (repository *scopedRepositoryFixture) EnqueueDeliveries(_ context.Context, adapter DeliveryAdapter, reference string, _ int, _ time.Time, _ int) (int, error) {
	if adapter != DeliveryAdapterMatrix || reference != "matrix_fixture" {
		repository.testing.Error("enqueue escaped adapter reference")
	}
	return 2, nil
}

func (repository *scopedRepositoryFixture) ClaimDeliveries(_ context.Context, adapter DeliveryAdapter, reference, owner string, at time.Time, lease time.Duration, limit int) ([]DeliveryTask, error) {
	if adapter != DeliveryAdapterMatrix || reference != "matrix_fixture" || limit != 1 {
		repository.testing.Error("claim escaped scope or leased a waiting batch")
	}
	repository.claimed = append(repository.claimed, at)
	return []DeliveryTask{{ID: "delivery_fixture", NotificationID: "notification_fixture", DestinationReferenceID: reference, Payload: []byte(`{"ok":true}`), AttemptNumber: 1, MaximumAttempts: 3, LeaseOwner: owner, LeaseExpiresAt: at.Add(lease)}}, nil
}

func (repository *scopedRepositoryFixture) CompleteDelivery(_ context.Context, adapter DeliveryAdapter, reference string, completion DeliveryCompletion) error {
	if adapter != DeliveryAdapterMatrix || reference != "matrix_fixture" {
		repository.testing.Error("completion escaped adapter reference")
	}
	repository.completions = append(repository.completions, completion)
	return nil
}

func TestScopedWorkerClaimsEachTaskWhenItsAttemptCanStart(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC)
	repository := &scopedRepositoryFixture{testing: t}
	worker, err := NewDeliveryWorker(DeliveryWorkerOptions{AdapterRepository: repository, Adapter: DeliveryAdapterMatrix, DestinationReferenceID: "matrix_fixture", LeaseOwner: "worker_fixture", Now: func() time.Time { return at }, PollInterval: time.Second, LeaseDuration: 30 * time.Second, BatchSize: 2, MaximumAttempts: 3, RetrySchedule: []time.Duration{time.Second}, Attempt: func(ctx context.Context, task DeliveryTask) (int, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("attempt lacks lease deadline")
		}
		at = at.Add(15 * time.Second)
		return 200, nil
	}})
	if err != nil {
		t.Fatal("create scoped worker fixture")
	}
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal("run scoped worker fixture")
	}
	if len(repository.claimed) != 2 || repository.claimed[1].Sub(repository.claimed[0]) != 15*time.Second || len(repository.completions) != 2 {
		t.Fatal("waiting tasks consumed their lease before their attempt")
	}
	for _, completion := range repository.completions {
		if completion.Validate() != nil || completion.Outcome != DeliverySucceeded {
			t.Fatal("scoped completion invalid")
		}
	}
}

func TestAdapterWorkerRequiresScopedRepository(t *testing.T) {
	t.Parallel()
	_, err := NewDeliveryWorker(DeliveryWorkerOptions{Repository: &deliveryRepositoryStub{}, Adapter: DeliveryAdapterMatrix, Attempt: func(context.Context, DeliveryTask) (int, error) { return 200, nil }})
	if err == nil {
		t.Fatal("adapter accepted unscoped repository")
	}
}
