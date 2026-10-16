package alerting

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const (
	MaximumEvaluationBatch = 32
	minimumPollInterval    = 100 * time.Millisecond
	maximumPollInterval    = time.Minute
	minimumReadTimeout     = 100 * time.Millisecond
	maximumReadTimeout     = 30 * time.Second
)

var ErrEvaluationUnavailable = errors.New("alert evaluation is unavailable")

type EvaluationTask struct {
	Rule                       domain.AlertRule
	Source                     domain.ProviderRef
	Resource                   *domain.ResourceRef
	Current                    *domain.Alert
	AcknowledgedByUserID       *domain.UserID
	DedupeKey                  string
	Synthetic                  bool
	ClaimedAt                  time.Time
	NextEvaluationAt           time.Time
	RuleVersion                int64
	SourceConfigurationVersion int64
	ClaimGeneration            int64
	LeaseExpiresAt             time.Time
}

func (task EvaluationTask) Validate() error {
	if err := task.Rule.Validate(); err != nil {
		return err
	}
	if err := task.Source.Validate(); err != nil {
		return err
	}
	if task.Source.SourceID != task.Rule.SourceID {
		return fmt.Errorf("alert task source does not match rule")
	}
	if task.Resource != nil {
		if err := task.Resource.Validate(); err != nil {
			return err
		}
		if !task.Source.Matches(*task.Resource) || task.Rule.ResourceID == nil || task.Resource.ID != *task.Rule.ResourceID {
			return fmt.Errorf("alert task resource does not match rule")
		}
	} else if task.Rule.ResourceID != nil {
		return fmt.Errorf("resource alert task requires a resource")
	}
	if task.Current != nil {
		if err := task.Current.Validate(); err != nil {
			return err
		}
		if task.Current.RuleID != task.Rule.ID || task.Current.SourceID != task.Rule.SourceID {
			return fmt.Errorf("alert task current alert does not match rule")
		}
		if task.Current.AcknowledgedAt != nil {
			if task.AcknowledgedByUserID == nil || task.AcknowledgedByUserID.Validate() != nil {
				return fmt.Errorf("alert task acknowledgement requires a user")
			}
		} else if task.AcknowledgedByUserID != nil {
			return fmt.Errorf("alert task has an unexpected acknowledging user")
		}
	} else if task.AcknowledgedByUserID != nil {
		return fmt.Errorf("new alert task cannot have an acknowledging user")
	}
	if err := validateToken(task.DedupeKey); err != nil {
		return err
	}
	if err := validateEvaluationTime(task.ClaimedAt); err != nil {
		return err
	}
	if err := validateEvaluationTime(task.NextEvaluationAt); err != nil || !task.NextEvaluationAt.After(task.ClaimedAt) {
		return fmt.Errorf("alert task next evaluation time is invalid")
	}
	if task.RuleVersion < 1 || task.SourceConfigurationVersion < 1 || task.ClaimGeneration < 1 || validateEvaluationTime(task.LeaseExpiresAt) != nil || !task.LeaseExpiresAt.After(task.ClaimedAt) {
		return fmt.Errorf("alert task lease is invalid")
	}
	return nil
}

type TaskStore interface {
	ClaimDueAlertEvaluations(context.Context, time.Time, int) ([]EvaluationTask, error)
	CompleteAlertEvaluation(context.Context, EvaluationTask, *domain.AlertMutation) error
}

type EvidenceResolver interface {
	ResolveAlertEvidence(context.Context, EvaluationTask, time.Time) (ConditionResult, error)
}

type IdentifierFactory interface {
	NewAlertID() (domain.AlertID, error)
	NewTransitionID() (domain.AlertTransitionID, error)
	NewNotificationID() (domain.NotificationID, error)
}

type WorkerOptions struct {
	Store        TaskStore
	Evidence     EvidenceResolver
	Identifiers  IdentifierFactory
	Now          func() time.Time
	OnError      func(error)
	PollInterval time.Duration
	ReadTimeout  time.Duration
	BatchSize    int
}

type Worker struct {
	store        TaskStore
	evidence     EvidenceResolver
	identifiers  IdentifierFactory
	now          func() time.Time
	onError      func(error)
	pollInterval time.Duration
	readTimeout  time.Duration
	batchSize    int
}

func NewWorker(options WorkerOptions) (*Worker, error) {
	if options.Store == nil || options.Evidence == nil || options.Identifiers == nil || options.Now == nil ||
		options.PollInterval < minimumPollInterval || options.PollInterval > maximumPollInterval ||
		options.ReadTimeout < minimumReadTimeout || options.ReadTimeout > maximumReadTimeout ||
		options.BatchSize < 1 || options.BatchSize > MaximumEvaluationBatch {
		return nil, fmt.Errorf("alert worker options are invalid")
	}
	if options.OnError == nil {
		options.OnError = func(error) {}
	}
	return &Worker{
		store: options.Store, evidence: options.Evidence, identifiers: options.Identifiers,
		now: options.Now, onError: options.OnError, pollInterval: options.PollInterval,
		readTimeout: options.ReadTimeout, batchSize: options.BatchSize,
	}, nil
}

func (worker *Worker) Run(ctx context.Context) {
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

func (worker *Worker) runAndReport(ctx context.Context) {
	if err := worker.RunOnce(ctx); err != nil && ctx.Err() == nil {
		worker.onError(err)
	}
}

func (worker *Worker) RunOnce(ctx context.Context) error {
	if worker == nil || ctx == nil || ctx.Err() != nil {
		return ErrEvaluationUnavailable
	}
	// Claim only when ready to read. A slow earlier task must not age later leases.
	for range worker.batchSize {
		if ctx.Err() != nil {
			return errors.Join(ErrEvaluationUnavailable, ctx.Err())
		}
		now := worker.now().UTC().Truncate(time.Microsecond)
		if err := validateEvaluationTime(now); err != nil {
			return ErrEvaluationUnavailable
		}
		tasks, err := worker.store.ClaimDueAlertEvaluations(ctx, now, 1)
		if err != nil {
			return errors.Join(ErrEvaluationUnavailable, err)
		}
		if len(tasks) > 1 {
			return ErrEvaluationUnavailable
		}
		if len(tasks) == 0 {
			return nil
		}
		task := tasks[0]
		if err := task.Validate(); err != nil {
			worker.onError(errors.Join(ErrEvaluationUnavailable, err))
			continue
		}
		readContext, cancel := context.WithTimeout(ctx, worker.readTimeout)
		condition, err := worker.evidence.ResolveAlertEvidence(readContext, task, now)
		if err == nil {
			err = readContext.Err()
		}
		cancel()
		var mutation *domain.AlertMutation
		if err == nil {
			mutation, err = worker.buildMutation(task, condition, now)
		}
		if err != nil {
			mutation = nil
			worker.onError(errors.Join(ErrEvaluationUnavailable, err))
		}
		// Unknown evidence changes no alert state, but still releases its claim.
		if err := worker.store.CompleteAlertEvaluation(ctx, task, mutation); err != nil {
			worker.onError(errors.Join(ErrEvaluationUnavailable, err))
		}
	}
	return nil
}

func (worker *Worker) buildMutation(task EvaluationTask, condition ConditionResult, now time.Time) (*domain.AlertMutation, error) {
	alertID := domain.AlertID("")
	newEpisode := task.Current == nil || task.Current.State == domain.AlertResolved
	if newEpisode {
		var err error
		alertID, err = worker.identifiers.NewAlertID()
		if err != nil {
			return nil, err
		}
	}
	result, err := Advance(AdvanceRequest{
		Rule: task.Rule, Current: task.Current, Condition: condition,
		CandidateID: alertID, EvaluatedAt: now, Synthetic: task.Synthetic,
	})
	if err != nil {
		return nil, err
	}
	if result.Alert == nil {
		return nil, nil
	}
	mutation := &domain.AlertMutation{
		Alert: *result.Alert, DedupeKey: task.DedupeKey,
		AcknowledgedByUserID: task.AcknowledgedByUserID,
	}
	if task.Current != nil && !newEpisode {
		expectedAt := task.Current.UpdatedAt
		expectedState := task.Current.State
		mutation.ExpectedUpdatedAt = &expectedAt
		mutation.ExpectedState = &expectedState
	}
	if newEpisode || result.Previous != nil && *result.Previous != result.Alert.State {
		transitionID, err := worker.identifiers.NewTransitionID()
		if err != nil {
			return nil, err
		}
		transition := domain.AlertTransition{
			ID: transitionID, AlertID: result.Alert.ID, From: result.Previous, To: result.Alert.State,
			OccurredAt: now, Actor: domain.AuditActor{Kind: domain.AuditActorSystem},
			ReasonCode: transitionReason(result.Alert.State), EvidenceRange: result.Alert.EvidenceRange,
		}
		if newEpisode {
			transition.From = nil
			mutation.AcknowledgedByUserID = nil
		}
		mutation.Transition = &transition
		if result.Alert.State == domain.AlertFiring || result.Alert.State == domain.AlertResolved {
			notificationID, err := worker.identifiers.NewNotificationID()
			if err != nil {
				return nil, err
			}
			notification := domain.Notification{
				ID: notificationID, SourceID: &result.Alert.SourceID, AlertID: &result.Alert.ID,
				ResourceID: result.Alert.ResourceID, Kind: notificationKind(result.Alert.State),
				Severity: result.Alert.Severity, SummaryCode: result.Alert.SummaryCode,
				CreatedAt: now, Synthetic: result.Alert.Synthetic,
			}
			mutation.Notification = &notification
		}
	}
	if err := mutation.Validate(); err != nil {
		return nil, err
	}
	return mutation, nil
}

func transitionReason(state domain.AlertState) domain.ReasonCode {
	switch state {
	case domain.AlertPending:
		return "alert.condition_pending"
	case domain.AlertFiring:
		return "alert.condition_matched"
	case domain.AlertResolved:
		return "alert.condition_cleared"
	default:
		return "alert.state_changed"
	}
}

func notificationKind(state domain.AlertState) domain.ReasonCode {
	if state == domain.AlertResolved {
		return "notification.alert_resolved"
	}
	return "notification.alert_firing"
}

func validateToken(value string) error {
	if len(value) < 1 || len(value) > 128 {
		return fmt.Errorf("alert task dedupe key is invalid")
	}
	for _, char := range value {
		if char != '_' && char != '-' && (char < '0' || char > '9') && (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') {
			return fmt.Errorf("alert task dedupe key is invalid")
		}
	}
	return nil
}

type SecureIdentifierFactory struct{}

func (SecureIdentifierFactory) NewAlertID() (domain.AlertID, error) {
	value, err := secureIdentifier("alert_")
	return domain.AlertID(value), err
}

func (SecureIdentifierFactory) NewTransitionID() (domain.AlertTransitionID, error) {
	value, err := secureIdentifier("transition_")
	return domain.AlertTransitionID(value), err
}

func (SecureIdentifierFactory) NewNotificationID() (domain.NotificationID, error) {
	value, err := secureIdentifier("notification_")
	return domain.NotificationID(value), err
}

func secureIdentifier(prefix string) (string, error) {
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(random), nil
}
