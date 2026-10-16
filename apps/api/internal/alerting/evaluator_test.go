package alerting

import (
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestEvaluateHealthAvailabilityThresholdAndSeverity(t *testing.T) {
	at := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	rule := fixtureRule(0)
	health := domain.HealthUnhealthy
	result, err := Evaluate(rule, evaluationInput(at, &health, nil, nil, nil))
	if err != nil || !result.Matched || result.SummaryCode != "alert.resource_unhealthy.matched" {
		t.Fatalf("health evaluation = (%#v, %v)", result, err)
	}

	rule.Kind = domain.AlertRuleResourceUnavailable
	availability := domain.AvailabilityUnavailable
	result, err = Evaluate(rule, evaluationInput(at, nil, &availability, nil, nil))
	if err != nil || !result.Matched {
		t.Fatalf("availability evaluation = (%#v, %v)", result, err)
	}

	comparison := domain.ComparisonGreaterThanOrEqual
	threshold := 0.8
	rule.Kind = domain.AlertRuleCPUThreshold
	rule.Comparison = &comparison
	rule.ThresholdValue = &threshold
	rule.Window = 5 * time.Minute
	value := 0.9
	result, err = Evaluate(rule, evaluationInput(at, nil, nil, &value, nil))
	if err != nil || !result.Matched {
		t.Fatalf("threshold evaluation = (%#v, %v)", result, err)
	}

	rule.Kind = domain.AlertRuleEventSeverityMatch
	rule.Comparison = nil
	rule.ThresholdValue = nil
	rule.Window = 0
	minimum := domain.SeverityError
	rule.MinimumSeverity = &minimum
	severity := domain.SeverityCritical
	result, err = Evaluate(rule, evaluationInput(at, nil, nil, nil, &severity))
	if err != nil || !result.Matched {
		t.Fatalf("severity evaluation = (%#v, %v)", result, err)
	}
}

func TestEvaluateRejectsCrossScopeAndFractionalCounts(t *testing.T) {
	at := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	rule := fixtureRule(0)
	health := domain.HealthUnhealthy
	input := evaluationInput(at, &health, nil, nil, nil)
	input.SourceID = "source_other"
	if _, err := Evaluate(rule, input); err == nil {
		t.Fatal("cross-source evidence was accepted")
	}

	comparison := domain.ComparisonGreaterThan
	count := uint64(2)
	rule.Kind = domain.AlertRuleRestartThreshold
	rule.Comparison = &comparison
	rule.ThresholdCount = &count
	rule.Window = time.Minute
	value := 2.5
	if _, err := Evaluate(rule, evaluationInput(at, nil, nil, &value, nil)); err == nil {
		t.Fatal("fractional restart evidence was accepted")
	}
}

func TestEvaluateUnknownAvailabilityAndHealthCannotResolve(t *testing.T) {
	at := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	rule := fixtureRule(0)
	rule.Kind = domain.AlertRuleResourceUnavailable
	for _, state := range []domain.AvailabilityState{domain.AvailabilityStale, domain.AvailabilityUnsupported, domain.AvailabilityPermissionDenied} {
		if _, err := Evaluate(rule, evaluationInput(at, nil, &state, nil, nil)); err == nil {
			t.Fatalf("%s evidence produced a condition", state)
		}
	}
	rule.Kind = domain.AlertRuleResourceUnhealthy
	health := domain.HealthUnknown
	if _, err := Evaluate(rule, evaluationInput(at, &health, nil, nil, nil)); err == nil {
		t.Fatal("unknown health produced a clear condition")
	}
}

func evaluationInput(
	at time.Time,
	health *domain.HealthState,
	availability *domain.AvailabilityState,
	value *float64,
	severity *domain.Severity,
) EvaluationInput {
	rule := fixtureRule(0)
	return EvaluationInput{
		SourceID: rule.SourceID, ResourceID: rule.ResourceID,
		EvidenceRange: domain.TimeRange{Start: at.Add(-time.Minute), End: at},
		Health:        health, Availability: availability, WindowValue: value, EventSeverity: severity,
	}
}
