package domain

import (
	"strings"
	"testing"
	"time"
)

func TestAlertRuleShapesMatchStoredKinds(t *testing.T) {
	resourceID := ResourceID("r1_" + strings.Repeat("A", 43))
	comparison := ComparisonGreaterThanOrEqual
	threshold := 0.8
	count := uint64(3)
	minimum := SeverityError
	base := AlertRule{
		ID: "rule_one", SourceID: "source_one", ResourceID: &resourceID, DisplayName: "Rule one",
		Enabled: true, Severity: SeverityWarning, EvaluationInterval: time.Minute,
	}
	tests := []AlertRule{
		withAlertKind(base, AlertRuleResourceUnavailable),
		withAlertKind(base, AlertRuleResourceUnhealthy),
		withAlertThreshold(base, AlertRuleCPUThreshold, &comparison, &threshold, nil),
		withAlertThreshold(base, AlertRuleMemoryThreshold, &comparison, &threshold, nil),
		withAlertThreshold(base, AlertRuleRestartThreshold, &comparison, nil, &count),
		withAlertThreshold(base, AlertRuleWorkloadUnavailableReplicas, &comparison, nil, &count),
		withAlertSeverity(base, minimum),
		{
			ID: "rule_provider", SourceID: "source_one", DisplayName: "Provider lost",
			Kind: AlertRuleProviderConnectionLost, Enabled: true, Severity: SeverityCritical,
			EvaluationInterval: time.Minute,
		},
	}
	for _, rule := range tests {
		if err := rule.Validate(); err != nil {
			t.Errorf("%s rule rejected: %v", rule.Kind, err)
		}
	}

	invalid := tests[0]
	invalid.ThresholdCount = &count
	if invalid.Validate() == nil {
		t.Fatal("resource availability rule accepted count fields")
	}
	invalid = tests[2]
	invalid.ResourceID = nil
	if invalid.Validate() == nil {
		t.Fatal("metric rule accepted missing resource")
	}
}

func withAlertKind(base AlertRule, kind AlertRuleKind) AlertRule {
	base.Kind = kind
	return base
}

func withAlertThreshold(base AlertRule, kind AlertRuleKind, comparison *ComparisonOperator, value *float64, count *uint64) AlertRule {
	base.Kind = kind
	base.Comparison = comparison
	base.ThresholdValue = value
	base.ThresholdCount = count
	base.Window = 5 * time.Minute
	return base
}

func withAlertSeverity(base AlertRule, minimum Severity) AlertRule {
	base.Kind = AlertRuleEventSeverityMatch
	base.MinimumSeverity = &minimum
	return base
}
