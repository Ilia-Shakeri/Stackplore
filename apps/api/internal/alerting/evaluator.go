package alerting

import (
	"fmt"
	"math"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type EvaluationInput struct {
	SourceID      domain.SourceID
	EnvironmentID *domain.EnvironmentID
	ResourceID    *domain.ResourceID
	EvidenceRange domain.TimeRange
	Availability  *domain.AvailabilityState
	Health        *domain.HealthState
	WindowValue   *float64
	EventSeverity *domain.Severity
}

func Evaluate(rule domain.AlertRule, input EvaluationInput) (ConditionResult, error) {
	if err := rule.Validate(); err != nil {
		return ConditionResult{}, err
	}
	if err := validateEvaluationInput(rule, input); err != nil {
		return ConditionResult{}, err
	}
	matched := false
	switch rule.Kind {
	case domain.AlertRuleResourceUnavailable, domain.AlertRuleProviderConnectionLost:
		matched = *input.Availability == domain.AvailabilityUnavailable ||
			*input.Availability == domain.AvailabilityStale
	case domain.AlertRuleResourceUnhealthy:
		matched = *input.Health == domain.HealthUnhealthy
	case domain.AlertRuleCPUThreshold, domain.AlertRuleMemoryThreshold:
		matched = compare(*rule.Comparison, *input.WindowValue, *rule.ThresholdValue)
	case domain.AlertRuleRestartThreshold, domain.AlertRuleWorkloadUnavailableReplicas:
		matched = compare(*rule.Comparison, *input.WindowValue, float64(*rule.ThresholdCount))
	case domain.AlertRuleEventSeverityMatch:
		matched = severityRank(*input.EventSeverity) >= severityRank(*rule.MinimumSeverity)
	}
	suffix := "clear"
	if matched {
		suffix = "matched"
	}
	return ConditionResult{
		Matched: matched, EvidenceRange: input.EvidenceRange,
		SummaryCode: domain.ReasonCode("alert." + string(rule.Kind) + "." + suffix),
	}, nil
}

func validateEvaluationInput(rule domain.AlertRule, input EvaluationInput) error {
	if err := input.SourceID.Validate(); err != nil || input.SourceID != rule.SourceID {
		return fmt.Errorf("alert evidence belongs to another source")
	}
	if err := input.EvidenceRange.Validate(); err != nil {
		return err
	}
	if rule.EnvironmentID != nil && !sameOptional(rule.EnvironmentID, input.EnvironmentID) {
		return fmt.Errorf("alert evidence belongs to another environment")
	}
	if rule.ResourceID != nil && !sameOptional(rule.ResourceID, input.ResourceID) {
		return fmt.Errorf("alert evidence belongs to another resource")
	}
	if input.Availability != nil {
		if err := input.Availability.Validate(); err != nil {
			return err
		}
	}
	if input.Health != nil {
		if err := input.Health.Validate(); err != nil {
			return err
		}
	}
	if input.EventSeverity != nil {
		if err := input.EventSeverity.Validate(); err != nil {
			return err
		}
	}
	if input.WindowValue != nil && (math.IsNaN(*input.WindowValue) || math.IsInf(*input.WindowValue, 0) ||
		*input.WindowValue < 0 || *input.WindowValue > 1<<53-1) {
		return fmt.Errorf("alert window value must be finite and non-negative")
	}
	exact := func(availability, health, window, severity bool) bool {
		return (input.Availability != nil) == availability && (input.Health != nil) == health &&
			(input.WindowValue != nil) == window && (input.EventSeverity != nil) == severity
	}
	switch rule.Kind {
	case domain.AlertRuleResourceUnavailable, domain.AlertRuleProviderConnectionLost:
		if !exact(true, false, false, false) {
			return fmt.Errorf("availability rule requires only availability evidence")
		}
	case domain.AlertRuleResourceUnhealthy:
		if !exact(false, true, false, false) {
			return fmt.Errorf("health rule requires only health evidence")
		}
	case domain.AlertRuleCPUThreshold, domain.AlertRuleMemoryThreshold,
		domain.AlertRuleRestartThreshold, domain.AlertRuleWorkloadUnavailableReplicas:
		if !exact(false, false, true, false) {
			return fmt.Errorf("threshold rule requires only one window value")
		}
		if (rule.Kind == domain.AlertRuleRestartThreshold || rule.Kind == domain.AlertRuleWorkloadUnavailableReplicas) &&
			math.Trunc(*input.WindowValue) != *input.WindowValue {
			return fmt.Errorf("count threshold evidence must be a whole value")
		}
	case domain.AlertRuleEventSeverityMatch:
		if !exact(false, false, false, true) {
			return fmt.Errorf("event rule requires only event severity evidence")
		}
	}
	return nil
}

func compare(operator domain.ComparisonOperator, actual, threshold float64) bool {
	switch operator {
	case domain.ComparisonGreaterThan:
		return actual > threshold
	case domain.ComparisonGreaterThanOrEqual:
		return actual >= threshold
	case domain.ComparisonLessThan:
		return actual < threshold
	case domain.ComparisonLessThanOrEqual:
		return actual <= threshold
	case domain.ComparisonEqual:
		return actual == threshold
	default:
		return false
	}
}

func severityRank(value domain.Severity) int {
	switch value {
	case domain.SeverityDebug:
		return 0
	case domain.SeverityInfo:
		return 1
	case domain.SeverityWarning:
		return 2
	case domain.SeverityError:
		return 3
	case domain.SeverityCritical:
		return 4
	default:
		return -1
	}
}
