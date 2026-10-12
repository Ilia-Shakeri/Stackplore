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
	Rule                 domain.AlertRule
	Source               domain.ProviderRef
	Resource             *domain.ResourceRef
	Current              *domain.Alert
	AcknowledgedByUserID *domain.UserID
	DedupeKey            string
	Synthetic            bool
	ClaimedAt            time.Time
	NextEvaluationAt     time.Time
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
	return nil
}

type TaskStore interface {
	ClaimDueAlertEvaluations(context.Context, time.Time, int) ([]EvaluationTask, error)
	domain.AlertMutationRepository
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
	now := worker.now().UTC().Truncate(time.Microsecond)
	if err := validateEvaluationTime(now); err != nil {
		return ErrEvaluationUnavailable
	}
	tasks, err := worker.store.ClaimDueAlertEvaluations(ctx, now, worker.batchSize)
	if err != nil {
		return errors.Join(ErrEvaluationUnavailable, err)
	}
	if len(tasks) > worker.batchSize {
		return ErrEvaluationUnavailable
	}
	for _, task := range tasks {
		if err := task.Validate(); err != nil {
			worker.onError(errors.Join(ErrEvaluationUnavailable, err))
			continue
		}
		readContext, cancel := context.WithTimeout(ctx, worker.readTimeout)
		condition, err := worker.evidence.ResolveAlertEvidence(readContext, task, now)
		cancel()
		if err != nil {
			worker.onError(errors.Join(ErrEvaluationUnavailable, err))
			continue
		}
		mutation, err := worker.buildMutation(task, condition, now)
		if err != nil {
			worker.onError(errors.Join(ErrEvaluationUnavailable, err))
			continue
		}
		if mutation == nil {
			continue
		}
		if err := worker.store.SaveAlertMutation(ctx, *mutation); err != nil {
			worker.onError(errors.Join(ErrEvaluationUnavailable, err))
		}
	}
	return nil
}

func (worker *Worker) buildMutation(task EvaluationTask, condition ConditionResult, now time.Time) (*domain.AlertMutation, error) {
	alertID := domain.AlertID("")
	if task.Current == nil {
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
	if task.Current != nil {
		expectedAt := task.Current.UpdatedAt
		expectedState := task.Current.State
		mutation.ExpectedUpdatedAt = &expectedAt
		mutation.ExpectedState = &expectedState
	}
	if task.Current == nil || result.Previous != nil && *result.Previous != result.Alert.State {
		transitionID, err := worker.identifiers.NewTransitionID()
		if err != nil {
			return nil, err
		}
		transition := domain.AlertTransition{
			ID: transitionID, AlertID: result.Alert.ID, From: result.Previous, To: result.Alert.State,
			OccurredAt: now, Actor: domain.AuditActor{Kind: domain.AuditActorSystem},
			ReasonCode: transitionReason(result.Alert.State), EvidenceRange: result.Alert.EvidenceRange,
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
