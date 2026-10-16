package alerting

import (
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestLifecycleMovesPendingFiringAcknowledgedAndResolved(t *testing.T) {
	start := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	rule := fixtureRule(2 * time.Minute)
	first := advanceMatched(t, rule, nil, start, "alert_one")
	if first.Alert == nil || first.Alert.State != domain.AlertPending || first.Previous != nil {
		t.Fatalf("first result = %#v", first)
	}
	second := advanceMatched(t, rule, first.Alert, start.Add(time.Minute), "")
	if second.Alert.State != domain.AlertPending || second.Previous == nil || *second.Previous != domain.AlertPending {
		t.Fatalf("second result = %#v", second)
	}
	third := advanceMatched(t, rule, second.Alert, start.Add(2*time.Minute), "")
	if third.Alert.State != domain.AlertFiring || third.Previous == nil || *third.Previous != domain.AlertPending {
		t.Fatalf("third result = %#v", third)
	}
	acknowledged, err := Acknowledge(*third.Alert, start.Add(3*time.Minute))
	if err != nil || acknowledged.State != domain.AlertAcknowledged || acknowledged.AcknowledgedAt == nil {
		t.Fatalf("Acknowledge() = (%#v, %v)", acknowledged, err)
	}
	fourth := advanceMatched(t, rule, &acknowledged, start.Add(4*time.Minute), "")
	if fourth.Alert.State != domain.AlertAcknowledged {
		t.Fatalf("acknowledged match result = %#v", fourth)
	}
	resolved, err := Advance(AdvanceRequest{
		Rule: rule, Current: fourth.Alert, Condition: condition(false, start.Add(5*time.Minute)),
		EvaluatedAt: start.Add(5 * time.Minute), Synthetic: true,
	})
	if err != nil || resolved.Alert == nil || resolved.Alert.State != domain.AlertResolved || resolved.Alert.ResolvedAt == nil {
		t.Fatalf("resolved result = (%#v, %v)", resolved, err)
	}
}

func TestLifecycleResolvesWhenRuleIsDisabledAndRejectsWrongOwner(t *testing.T) {
	start := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	rule := fixtureRule(0)
	current := advanceMatched(t, rule, nil, start, "alert_one").Alert
	rule.Enabled = false
	result, err := Advance(AdvanceRequest{
		Rule: rule, Current: current, Condition: condition(true, start.Add(time.Minute)),
		EvaluatedAt: start.Add(time.Minute), Synthetic: true,
	})
	if err != nil || result.Alert.State != domain.AlertResolved {
		t.Fatalf("disabled result = (%#v, %v)", result, err)
	}

	other := fixtureRule(0)
	other.ID = "rule_other"
	if _, err := Advance(AdvanceRequest{
		Rule: other, Current: current, Condition: condition(true, start.Add(time.Minute)),
		EvaluatedAt: start.Add(time.Minute), Synthetic: true,
	}); err == nil {
		t.Fatal("wrong rule owner was accepted")
	}
}

func TestPendingDoesNotCountMissingEvaluationTime(t *testing.T) {
	start := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	rule := fixtureRule(2 * time.Minute)
	first := advanceMatched(t, rule, nil, start, "alert_pending")
	// Unknown evidence caused the intervening scheduled evaluation to make no write.
	second := advanceMatched(t, rule, first.Alert, start.Add(2*time.Minute), "")
	if second.Alert.State != domain.AlertPending || !second.Alert.StartedAt.Equal(start.Add(2*time.Minute)) {
		t.Fatalf("pending after missing poll = %#v; unknown time must not fire", second.Alert)
	}
	third := advanceMatched(t, rule, second.Alert, start.Add(3*time.Minute), "")
	fourth := advanceMatched(t, rule, third.Alert, start.Add(4*time.Minute), "")
	if fourth.Alert.State != domain.AlertFiring {
		t.Fatalf("continuous pending after gap = %#v", fourth.Alert)
	}
}

func TestResolvedEpisodeReopensWithNewIdentityAndClearEvidence(t *testing.T) {
	start := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	rule := fixtureRule(0)
	first := advanceMatched(t, rule, nil, start, "alert_first")
	clear := condition(false, start.Add(time.Minute))
	clear.SummaryCode = "alert.resource_unhealthy.clear"
	resolved, err := Advance(AdvanceRequest{Rule: rule, Current: first.Alert, Condition: clear, EvaluatedAt: start.Add(time.Minute), Synthetic: true})
	if err != nil || resolved.Alert.SummaryCode != clear.SummaryCode || !resolved.Alert.EvidenceRange.End.Equal(clear.EvidenceRange.End) {
		t.Fatalf("resolved evidence = %#v, %v", resolved.Alert, err)
	}
	second := advanceMatched(t, rule, resolved.Alert, start.Add(2*time.Minute), "alert_second")
	if second.Alert.ID != "alert_second" || second.Alert.ResolvedAt != nil || second.Previous != nil {
		t.Fatalf("new episode = %#v", second)
	}
}

func advanceMatched(t *testing.T, rule domain.AlertRule, current *domain.Alert, at time.Time, id domain.AlertID) LifecycleResult {
	t.Helper()
	result, err := Advance(AdvanceRequest{
		Rule: rule, Current: current, Condition: condition(true, at), CandidateID: id,
		EvaluatedAt: at, Synthetic: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func fixtureRule(pending time.Duration) domain.AlertRule {
	resourceID := domain.ResourceID("r1_" + strings.Repeat("A", 43))
	return domain.AlertRule{
		ID: "rule_one", SourceID: "source_demo", ResourceID: &resourceID,
		DisplayName: "Unhealthy resource", Kind: domain.AlertRuleResourceUnhealthy,
		Enabled: true, Severity: domain.SeverityError, EvaluationInterval: time.Minute,
		PendingFor: pending,
	}
}

func condition(matched bool, at time.Time) ConditionResult {
	return ConditionResult{
		Matched: matched, EvidenceRange: domain.TimeRange{Start: at.Add(-time.Minute), End: at},
		SummaryCode: "alert.resource_unhealthy",
	}
}
