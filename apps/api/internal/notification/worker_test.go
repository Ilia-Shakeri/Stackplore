package notification

import (
	"context"
	"errors"
	"testing"
	"time"
)

type deliveryRepositoryStub struct {
	enqueueReference string
	enqueueAttempts  int
	enqueueAt        time.Time
	enqueueLimit     int
	tasks            []DeliveryTask
	claimOwner       string
	claimAt          time.Time
	claimLease       time.Duration
	completions      []DeliveryCompletion
	err              error
}

func (repository *deliveryRepositoryStub) EnqueueWebhookDeliveries(
	_ context.Context, reference string, attempts int, at time.Time, limit int,
) (int, error) {
	repository.enqueueReference = reference
	repository.enqueueAttempts = attempts
	repository.enqueueAt = at
	repository.enqueueLimit = limit
	return 1, repository.err
}

func (repository *deliveryRepositoryStub) ClaimWebhookDeliveries(
	_ context.Context, owner string, at time.Time, lease time.Duration, _ int,
) ([]DeliveryTask, error) {
	repository.claimOwner = owner
	repository.claimAt = at
	repository.claimLease = lease
	return repository.tasks, repository.err
}

func (repository *deliveryRepositoryStub) CompleteWebhookDelivery(_ context.Context, completion DeliveryCompletion) error {
	repository.completions = append(repository.completions, completion)
	return repository.err
}

type targetResolverStub struct {
	reference string
	target    WebhookTarget
	err       error
}

func (resolver *targetResolverStub) ResolveWebhookTarget(_ context.Context, reference string) (WebhookTarget, error) {
	resolver.reference = reference
	return resolver.target, resolver.err
}

type webhookSenderStub struct {
	target   WebhookTarget
	delivery WebhookDelivery
	result   WebhookResult
	err      error
}

func (sender *webhookSenderStub) Send(_ context.Context, target WebhookTarget, delivery WebhookDelivery) (WebhookResult, error) {
	sender.target = target
	sender.delivery = delivery
	return sender.result, sender.err
}

func TestDeliveryWorkerCompletesSuccessfulAttempt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	repository := &deliveryRepositoryStub{tasks: []DeliveryTask{deliveryTask(now, 1, 3)}}
	resolver := &targetResolverStub{target: WebhookTarget{URL: "https://hooks.example.test", SigningKey: make([]byte, 32)}}
	sender := &webhookSenderStub{result: WebhookResult{Status: 204}}
	worker := newDeliveryWorkerForTest(t, repository, resolver, sender, now)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if repository.enqueueReference != "webhook_primary" || repository.enqueueAttempts != 3 ||
		repository.claimOwner != "worker_test" || repository.claimLease != time.Minute {
		t.Fatalf("repository calls = %#v", repository)
	}
	if resolver.reference != "webhook_primary" || sender.delivery.ID != "delivery_one" ||
		string(sender.delivery.Payload) != `{"ok":true}` || len(repository.completions) != 1 {
		t.Fatalf("delivery path = (%q, %q, %d)", resolver.reference, sender.delivery.ID, len(repository.completions))
	}
	for _, value := range sender.target.SigningKey {
		if value != 0 {
			t.Fatal("worker retained resolved signing key after attempt")
		}
	}
	completion := repository.completions[0]
	if completion.Validate() != nil || completion.Outcome != DeliverySucceeded || completion.ResponseStatus == nil ||
		*completion.ResponseStatus != 204 || completion.RetryAt != nil {
		t.Fatalf("completion = %#v", completion)
	}
}

func TestDeliveryWorkerRetriesUncertainAttemptWithFixedPayload(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	repository := &deliveryRepositoryStub{tasks: []DeliveryTask{deliveryTask(now, 2, 3)}}
	sender := &webhookSenderStub{err: &DeliveryError{
		Code: "notification.timeout", Retryable: true, Uncertain: true,
	}}
	worker := newDeliveryWorkerForTest(t, repository, &targetResolverStub{}, sender, now)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	completion := repository.completions[0]
	if completion.Outcome != DeliveryUncertain || completion.ErrorCode != "notification.timeout" ||
		completion.RetryAt == nil || !completion.RetryAt.Equal(now.Add(30*time.Second)) ||
		string(sender.delivery.Payload) != `{"ok":true}` {
		t.Fatalf("completion = %#v, payload = %q", completion, sender.delivery.Payload)
	}
}

func TestDeliveryWorkerStopsRetryAtAttemptLimit(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	repository := &deliveryRepositoryStub{tasks: []DeliveryTask{deliveryTask(now, 3, 3)}}
	sender := &webhookSenderStub{err: &DeliveryError{Code: "notification.remote_rejected", Retryable: true}}
	worker := newDeliveryWorkerForTest(t, repository, &targetResolverStub{}, sender, now)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	completion := repository.completions[0]
	if completion.Outcome != DeliveryFailed || completion.RetryAt != nil || completion.Validate() != nil {
		t.Fatalf("completion = %#v", completion)
	}
}

func TestDeliveryWorkerReportsRepositoryFailure(t *testing.T) {
	t.Parallel()
	repository := &deliveryRepositoryStub{err: errors.New("store down")}
	worker := newDeliveryWorkerForTest(
		t, repository, &targetResolverStub{}, &webhookSenderStub{},
		time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC),
	)
	if err := worker.RunOnce(context.Background()); !errors.Is(err, ErrDeliveryUnavailable) {
		t.Fatalf("RunOnce() error = %v", err)
	}
}

func TestDeliveryWorkerStopsOnCancellation(t *testing.T) {
	t.Parallel()
	worker := newDeliveryWorkerForTest(
		t, &deliveryRepositoryStub{}, &targetResolverStub{}, &webhookSenderStub{},
		time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC),
	)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("delivery worker did not stop")
	}
}

func deliveryTask(now time.Time, attempt, maximum int) DeliveryTask {
	return DeliveryTask{
		ID: "delivery_one", NotificationID: "notification_one", DestinationReferenceID: "webhook_primary",
		Payload: []byte(`{"ok":true}`), AttemptNumber: attempt, MaximumAttempts: maximum,
		LeaseOwner: "worker_test", LeaseExpiresAt: now.Add(time.Minute),
	}
}

func newDeliveryWorkerForTest(
	t *testing.T,
	repository DeliveryRepository,
	resolver WebhookTargetResolver,
	sender WebhookAttemptSender,
	now time.Time,
) *DeliveryWorker {
	t.Helper()
	worker, err := NewDeliveryWorker(DeliveryWorkerOptions{
		Repository: repository, Targets: resolver, Sender: sender,
		DestinationReferenceID: "webhook_primary", LeaseOwner: "worker_test",
		Now: func() time.Time { return now }, PollInterval: time.Second, LeaseDuration: time.Minute,
		BatchSize: 4, MaximumAttempts: 3,
		RetrySchedule: []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute},
	})
	if err != nil {
		t.Fatalf("NewDeliveryWorker() error = %v", err)
	}
	return worker
}
