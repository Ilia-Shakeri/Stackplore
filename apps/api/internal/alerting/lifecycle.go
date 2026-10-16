package alerting

import (
	"fmt"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type ConditionResult struct {
	Matched       bool
	EvidenceRange domain.TimeRange
	SummaryCode   domain.ReasonCode
}

func (result ConditionResult) Validate(evaluatedAt time.Time) error {
	if err := result.EvidenceRange.Validate(); err != nil {
		return err
	}
	if result.EvidenceRange.End.After(evaluatedAt) {
		return fmt.Errorf("alert evidence cannot follow evaluation time")
	}
	return result.SummaryCode.Validate()
}

type AdvanceRequest struct {
	Rule        domain.AlertRule
	Current     *domain.Alert
	Condition   ConditionResult
	CandidateID domain.AlertID
	EvaluatedAt time.Time
	Synthetic   bool
}

type LifecycleResult struct {
	Alert    *domain.Alert
	Previous *domain.AlertState
	Changed  bool
}

func Advance(request AdvanceRequest) (LifecycleResult, error) {
	if err := request.Rule.Validate(); err != nil {
		return LifecycleResult{}, err
	}
	if err := validateEvaluationTime(request.EvaluatedAt); err != nil {
		return LifecycleResult{}, err
	}
	if err := request.Condition.Validate(request.EvaluatedAt); err != nil {
		return LifecycleResult{}, err
	}
	current, err := validatedCurrent(request)
	if err != nil {
		return LifecycleResult{}, err
	}
	matched := request.Condition.Matched && request.Rule.Enabled
	if current == nil || current.State == domain.AlertResolved {
		if !matched {
			return LifecycleResult{}, nil
		}
		return openAlert(request)
	}
	previous := current.State
	if !matched {
		resolved := request.EvaluatedAt
		current.State = domain.AlertResolved
		current.ResolvedAt = &resolved
		current.UpdatedAt = request.EvaluatedAt
		current.EvidenceRange = mergeRange(current.EvidenceRange, request.Condition.EvidenceRange)
		current.SummaryCode = request.Condition.SummaryCode
		if err := current.Validate(); err != nil {
			return LifecycleResult{}, err
		}
		return LifecycleResult{Alert: current, Previous: &previous, Changed: true}, nil
	}
	// A missed or unknown evaluation cannot count toward a continuous pending period.
	// The small margin allows ordinary scheduling delay without counting a skipped poll.
	margin := min(request.Rule.EvaluationInterval/10, 5*time.Second)
	if current.State == domain.AlertPending && request.EvaluatedAt.Sub(current.UpdatedAt) > request.Rule.EvaluationInterval+margin {
		current.StartedAt = request.EvaluatedAt
		current.EvidenceRange = request.Condition.EvidenceRange
	} else {
		current.EvidenceRange = mergeRange(current.EvidenceRange, request.Condition.EvidenceRange)
	}
	current.SummaryCode = request.Condition.SummaryCode
	current.Severity = request.Rule.Severity
	current.UpdatedAt = request.EvaluatedAt
	if current.State == domain.AlertPending && request.EvaluatedAt.Sub(current.StartedAt) >= request.Rule.PendingFor {
		current.State = domain.AlertFiring
	}
	if err := current.Validate(); err != nil {
		return LifecycleResult{}, err
	}
	return LifecycleResult{Alert: current, Previous: &previous, Changed: true}, nil
}

func Acknowledge(current domain.Alert, acknowledgedAt time.Time) (domain.Alert, error) {
	if err := current.Validate(); err != nil {
		return domain.Alert{}, err
	}
	if current.State != domain.AlertFiring {
		return domain.Alert{}, fmt.Errorf("only a firing alert can be acknowledged")
	}
	if err := validateEvaluationTime(acknowledgedAt); err != nil {
		return domain.Alert{}, err
	}
	if acknowledgedAt.Before(current.UpdatedAt) {
		return domain.Alert{}, fmt.Errorf("alert acknowledgement cannot precede its last update")
	}
	current.State = domain.AlertAcknowledged
	current.AcknowledgedAt = &acknowledgedAt
	current.UpdatedAt = acknowledgedAt
	if err := current.Validate(); err != nil {
		return domain.Alert{}, err
	}
	return current, nil
}

func validatedCurrent(request AdvanceRequest) (*domain.Alert, error) {
	if request.Current == nil {
		return nil, nil
	}
	current := *request.Current
	if err := current.Validate(); err != nil {
		return nil, err
	}
	if current.RuleID != request.Rule.ID || current.SourceID != request.Rule.SourceID ||
		!sameOptional(current.EnvironmentID, request.Rule.EnvironmentID) ||
		!sameOptional(current.ResourceID, request.Rule.ResourceID) || current.Synthetic != request.Synthetic {
		return nil, fmt.Errorf("current alert does not belong to the evaluated rule")
	}
	if !request.EvaluatedAt.After(current.UpdatedAt) {
		return nil, fmt.Errorf("alert evaluation time must follow its last update")
	}
	return &current, nil
}

func openAlert(request AdvanceRequest) (LifecycleResult, error) {
	if err := request.CandidateID.Validate(); err != nil {
		return LifecycleResult{}, err
	}
	state := domain.AlertPending
	if request.Rule.PendingFor == 0 {
		state = domain.AlertFiring
	}
	alert := &domain.Alert{
		ID: request.CandidateID, RuleID: request.Rule.ID, SourceID: request.Rule.SourceID,
		EnvironmentID: request.Rule.EnvironmentID, ResourceID: request.Rule.ResourceID,
		State: state, Severity: request.Rule.Severity, EvidenceRange: request.Condition.EvidenceRange,
		StartedAt: request.EvaluatedAt, UpdatedAt: request.EvaluatedAt,
		SummaryCode: request.Condition.SummaryCode, Synthetic: request.Synthetic,
	}
	if err := alert.Validate(); err != nil {
		return LifecycleResult{}, err
	}
	return LifecycleResult{Alert: alert, Changed: true}, nil
}

func validateEvaluationTime(value time.Time) error {
	if value.IsZero() || value.Location() != time.UTC || value.Nanosecond()%int(time.Microsecond) != 0 {
		return fmt.Errorf("alert evaluation time must use UTC microsecond precision")
	}
	return nil
}

func mergeRange(left, right domain.TimeRange) domain.TimeRange {
	if right.Start.Before(left.Start) {
		left.Start = right.Start
	}
	if right.End.After(left.End) {
		left.End = right.End
	}
	return left
}

func sameOptional[T comparable](left, right *T) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}
