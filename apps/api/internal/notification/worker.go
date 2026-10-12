package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const MaximumDeliveryBatch = 32

var ErrDeliveryUnavailable = errors.New("notification delivery is unavailable")

type DeliveryTask struct {
	ID                     string
	NotificationID         string
	DestinationReferenceID string
	Payload                []byte
	AttemptNumber          int
	MaximumAttempts        int
	LeaseOwner             string
	LeaseExpiresAt         time.Time
}

func (task DeliveryTask) Validate(now time.Time) error {
	if !validDeliveryID(task.ID) || !validDeliveryID(task.NotificationID) ||
		!validDeliveryID(task.DestinationReferenceID) || !validDeliveryID(task.LeaseOwner) ||
		len(task.Payload) < 2 || len(task.Payload) > maximumWebhookPayload ||
		!json.Valid(task.Payload) ||
		task.AttemptNumber < 1 || task.MaximumAttempts < task.AttemptNumber || task.MaximumAttempts > 100 ||
		task.LeaseExpiresAt.IsZero() || task.LeaseExpiresAt.Location() != time.UTC ||
		!task.LeaseExpiresAt.After(now) {
		return ErrDeliveryUnavailable
	}
	return nil
}

type DeliveryOutcome string

const (
	DeliverySucceeded DeliveryOutcome = "success"
	DeliveryFailed    DeliveryOutcome = "failure"
	DeliveryUncertain DeliveryOutcome = "uncertain"
)

type DeliveryCompletion struct {
	DeliveryID     string
	AttemptNumber  int
	LeaseOwner     string
	FinishedAt     time.Time
	Outcome        DeliveryOutcome
	ResponseStatus *int
	ErrorCode      string
	RetryAt        *time.Time
}

func (completion DeliveryCompletion) Validate() error {
	if !validDeliveryID(completion.DeliveryID) || !validDeliveryID(completion.LeaseOwner) ||
		completion.AttemptNumber < 1 || completion.AttemptNumber > 100 || completion.FinishedAt.IsZero() ||
		completion.FinishedAt.Location() != time.UTC || completion.FinishedAt.Nanosecond()%int(time.Microsecond) != 0 {
		return ErrDeliveryUnavailable
	}
	if completion.ResponseStatus != nil && (*completion.ResponseStatus < 100 || *completion.ResponseStatus > 599) {
		return ErrDeliveryUnavailable
	}
	switch completion.Outcome {
	case DeliverySucceeded:
		if completion.ErrorCode != "" || completion.RetryAt != nil || completion.ResponseStatus == nil ||
			*completion.ResponseStatus < 200 || *completion.ResponseStatus >= 300 {
			return ErrDeliveryUnavailable
		}
	case DeliveryFailed, DeliveryUncertain:
		if !validReasonCode(completion.ErrorCode) {
			return ErrDeliveryUnavailable
		}
		if completion.RetryAt != nil && (completion.RetryAt.Location() != time.UTC ||
			completion.RetryAt.Nanosecond()%int(time.Microsecond) != 0 || !completion.RetryAt.After(completion.FinishedAt)) {
			return ErrDeliveryUnavailable
		}
	default:
		return ErrDeliveryUnavailable
	}
	return nil
}

type DeliveryRepository interface {
	EnqueueWebhookDeliveries(context.Context, string, int, time.Time, int) (int, error)
	ClaimWebhookDeliveries(context.Context, string, time.Time, time.Duration, int) ([]DeliveryTask, error)
	CompleteWebhookDelivery(context.Context, DeliveryCompletion) error
}

type WebhookTargetResolver interface {
	ResolveWebhookTarget(context.Context, string) (WebhookTarget, error)
}

type WebhookAttemptSender interface {
	Send(context.Context, WebhookTarget, WebhookDelivery) (WebhookResult, error)
}

type DeliveryWorkerOptions struct {
	Repository             DeliveryRepository
	Targets                WebhookTargetResolver
	Sender                 WebhookAttemptSender
	DestinationReferenceID string
	LeaseOwner             string
	Now                    func() time.Time
	OnError                func(error)
	PollInterval           time.Duration
	LeaseDuration          time.Duration
	BatchSize              int
	MaximumAttempts        int
	RetrySchedule          []time.Duration
}

type DeliveryWorker struct {
	repository             DeliveryRepository
	targets                WebhookTargetResolver
	sender                 WebhookAttemptSender
	destinationReferenceID string
	leaseOwner             string
	now                    func() time.Time
	onError                func(error)
	pollInterval           time.Duration
	leaseDuration          time.Duration
	batchSize              int
	maximumAttempts        int
	retrySchedule          []time.Duration
}

func NewDeliveryWorker(options DeliveryWorkerOptions) (*DeliveryWorker, error) {
	if options.Repository == nil || options.Targets == nil || options.Sender == nil || options.Now == nil ||
		!validDeliveryID(options.DestinationReferenceID) || !validDeliveryID(options.LeaseOwner) ||
		options.PollInterval < 100*time.Millisecond || options.PollInterval > time.Minute ||
		options.LeaseDuration < time.Second || options.LeaseDuration > 10*time.Minute ||
		options.BatchSize < 1 || options.BatchSize > MaximumDeliveryBatch ||
		options.MaximumAttempts < 1 || options.MaximumAttempts > 100 || len(options.RetrySchedule) < 1 ||
		len(options.RetrySchedule) > options.MaximumAttempts {
		return nil, ErrDeliveryUnavailable
	}
	for index, delay := range options.RetrySchedule {
		if delay < time.Second || delay > 24*time.Hour || index > 0 && delay < options.RetrySchedule[index-1] {
			return nil, ErrDeliveryUnavailable
		}
	}
	if options.OnError == nil {
		options.OnError = func(error) {}
	}
	return &DeliveryWorker{
		repository: options.Repository, targets: options.Targets, sender: options.Sender,
		destinationReferenceID: options.DestinationReferenceID, leaseOwner: options.LeaseOwner,
		now: options.Now, onError: options.OnError, pollInterval: options.PollInterval,
		leaseDuration: options.LeaseDuration, batchSize: options.BatchSize,
		maximumAttempts: options.MaximumAttempts, retrySchedule: append([]time.Duration(nil), options.RetrySchedule...),
	}, nil
}

func (worker *DeliveryWorker) Run(ctx context.Context) {
	if worker == nil || ctx == nil {
		return
	}
	worker.runAndReport(ctx)
	ticker := time.NewTicker(worker.pollInterval)
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

func (worker *DeliveryWorker) runAndReport(ctx context.Context) {
	if err := worker.RunOnce(ctx); err != nil && ctx.Err() == nil {
		worker.onError(err)
	}
}

func (worker *DeliveryWorker) RunOnce(ctx context.Context) error {
	if worker == nil || ctx == nil || ctx.Err() != nil {
		return ErrDeliveryUnavailable
	}
	now := worker.now().UTC().Truncate(time.Microsecond)
	if now.UnixMicro() < 0 {
		return ErrDeliveryUnavailable
	}
	if _, err := worker.repository.EnqueueWebhookDeliveries(
		ctx, worker.destinationReferenceID, worker.maximumAttempts, now, worker.batchSize,
	); err != nil {
		return errors.Join(ErrDeliveryUnavailable, err)
	}
	tasks, err := worker.repository.ClaimWebhookDeliveries(ctx, worker.leaseOwner, now, worker.leaseDuration, worker.batchSize)
	if err != nil {
		return errors.Join(ErrDeliveryUnavailable, err)
	}
	if len(tasks) > worker.batchSize {
		return ErrDeliveryUnavailable
	}
	for _, task := range tasks {
		if err := worker.deliver(ctx, task, now); err != nil && ctx.Err() == nil {
			worker.onError(errors.Join(ErrDeliveryUnavailable, err))
		}
	}
	return nil
}

func (worker *DeliveryWorker) deliver(ctx context.Context, task DeliveryTask, claimedAt time.Time) error {
	if err := task.Validate(claimedAt); err != nil {
		return err
	}
	target, err := worker.targets.ResolveWebhookTarget(ctx, task.DestinationReferenceID)
	result := WebhookResult{}
	if err == nil {
		defer clear(target.SigningKey)
		result, err = worker.sender.Send(ctx, target, WebhookDelivery{ID: task.ID, Payload: task.Payload})
	}
	finishedAt := worker.now().UTC().Truncate(time.Microsecond)
	if finishedAt.Before(claimedAt) {
		finishedAt = claimedAt
	}
	completion := DeliveryCompletion{
		DeliveryID: task.ID, AttemptNumber: task.AttemptNumber, LeaseOwner: task.LeaseOwner,
		FinishedAt: finishedAt,
	}
	if result.Status != 0 {
		completion.ResponseStatus = &result.Status
	}
	if err == nil {
		completion.Outcome = DeliverySucceeded
	} else {
		failure := &DeliveryError{Code: "notification.target_unavailable"}
		if !errors.As(err, &failure) {
			failure = &DeliveryError{Code: "notification.target_unavailable"}
		}
		completion.Outcome = DeliveryFailed
		if failure.Uncertain {
			completion.Outcome = DeliveryUncertain
		}
		completion.ErrorCode = failure.Code
		if failure.Retryable && task.AttemptNumber < task.MaximumAttempts {
			retryAt := finishedAt.Add(worker.retryDelay(task.AttemptNumber)).UTC().Truncate(time.Microsecond)
			completion.RetryAt = &retryAt
		}
	}
	if completion.Validate() != nil {
		return fmt.Errorf("delivery completion is invalid")
	}
	return worker.repository.CompleteWebhookDelivery(ctx, completion)
}

func (worker *DeliveryWorker) retryDelay(attempt int) time.Duration {
	index := attempt - 1
	if index >= len(worker.retrySchedule) {
		index = len(worker.retrySchedule) - 1
	}
	return worker.retrySchedule[index]
}

func validReasonCode(value string) bool {
	if len(value) < 3 || len(value) > 96 {
		return false
	}
	for _, char := range value {
		if char != '.' && char != '_' && char != '-' && (char < '0' || char > '9') &&
			(char < 'a' || char > 'z') {
			return false
		}
	}
	return true
}
