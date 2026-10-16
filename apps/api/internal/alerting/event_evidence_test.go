package alerting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func eventEvidenceFixture(t *testing.T) (EvaluationTask, metricEventEvidenceProvider, collectionEvidenceStub) {
	t.Helper()
	task, provider, collections := metricEvidenceFixture(t, domain.AlertRuleCPUThreshold)
	task.Rule.Kind, task.Rule.Window = domain.AlertRuleEventSeverityMatch, 0
	task.Rule.Comparison, task.Rule.ThresholdValue = nil, nil
	severity := domain.SeverityError
	task.Rule.MinimumSeverity = &severity
	provider.pages = []domain.EventPage{{Status: collectionOutcome(provider.at).Status,
		Page: domain.PageInfo{Limit: domain.MaxPageSize}, Events: []domain.InfrastructureEvent{
			evidenceEvent("event_latest", task, provider.at.Add(-time.Second), domain.SeverityInfo),
			evidenceEvent("event_older", task, provider.at.Add(-30*time.Second), domain.SeverityCritical),
		},
	}}
	return task, provider, collections
}

func evidenceEvent(id domain.EventID, task EvaluationTask, at time.Time, severity domain.Severity) domain.InfrastructureEvent {
	environmentID, resourceID := task.Resource.Identity.EnvironmentID, task.Resource.ID
	return domain.InfrastructureEvent{ID: id, Provider: task.Source.Provider, SourceID: task.Source.SourceID,
		EnvironmentID: &environmentID, ResourceID: &resourceID, Kind: "resource.changed", Severity: severity,
		ReasonCode: "resource.updated", Summary: "Safe event", OccurredAt: at, ObservedAt: at, Synthetic: true,
	}
}

func TestEventConditionSeesOlderCriticalAfterLatestInfo(t *testing.T) {
	task, provider, collections := eventEvidenceFixture(t)
	result, err := newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at)
	if err != nil || !result.Matched || result.SummaryCode != "alert.event_severity_match.matched" {
		t.Fatalf("all-window event condition = %#v, %v", result, err)
	}
}

func TestEventConditionReadsNextPageAndExactEnvironment(t *testing.T) {
	task, provider, collections := eventEvidenceFixture(t)
	environmentID := task.Resource.Identity.EnvironmentID
	task.Rule.EnvironmentID = &environmentID
	first, second := provider.pages[0].Events[0], provider.pages[0].Events[1]
	provider.pages[0].Events = []domain.InfrastructureEvent{first}
	provider.pages[0].Page.HasMore, provider.pages[0].Page.NextCursor = true, "Y3Vyc29yX3R3bw"
	provider.pages = append(provider.pages, domain.EventPage{Status: collectionOutcome(provider.at).Status,
		Page: domain.PageInfo{Limit: domain.MaxPageSize}, Events: []domain.InfrastructureEvent{second}})
	result, err := newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at)
	if err != nil || !result.Matched {
		t.Fatalf("older next-page event = %#v, %v", result, err)
	}
	// The same source's event for another environment must not match a scoped rule.
	otherID := domain.EnvironmentID("env_other")
	provider.pages[1].Events[0].EnvironmentID = &otherID
	result, err = newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at)
	if err != nil || result.Matched {
		t.Fatalf("other environment event = %#v, %v", result, err)
	}
}

func TestEventUnknownCannotClearEvenWithNoEvents(t *testing.T) {
	for _, name := range []string{"partial", "denied", "stale", "future", "capture_gap", "future_event", "repeated_cursor", "mixed_provenance"} {
		t.Run(name, func(t *testing.T) {
			task, provider, collections := eventEvidenceFixture(t)
			switch name {
			case "partial":
				provider.pages[0].Status.Completeness, provider.pages[0].Status.ReasonCode = domain.CompletenessPartial, "source.partial"
			case "denied":
				provider.pages[0].Status.State, provider.pages[0].Status.ReasonCode = domain.AvailabilityPermissionDenied, "source.denied"
				provider.pages[0].Events = nil
			case "stale":
				provider.pages[0].Status.State, provider.pages[0].Status.ReasonCode = domain.AvailabilityStale, "source.stale"
			case "future":
				collections.current.CollectedAt = provider.at.Add(time.Second)
			case "capture_gap":
				collections.history.Observations = collections.history.Observations[1:]
			case "future_event":
				provider.pages[0].Events[0].ObservedAt = provider.at.Add(time.Second)
			case "mixed_provenance":
				provider.pages[0].Events[0].Synthetic = false
			case "repeated_cursor":
				provider.pages[0].Page.HasMore, provider.pages[0].Page.NextCursor = true, "Y3Vyc29yX3R3bw"
				provider.pages = append(provider.pages, provider.pages[0])
			}
			if _, err := newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at); !errors.Is(err, ErrEvaluationUnavailable) {
				t.Fatalf("unknown event condition = %v", err)
			}
		})
	}
}

func TestCompleteEmptyEventWindowClears(t *testing.T) {
	task, provider, collections := eventEvidenceFixture(t)
	provider.pages[0].Events = nil
	result, err := newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at)
	if err != nil || result.Matched || result.SummaryCode != "alert.event_severity_match.clear" {
		t.Fatalf("complete empty event window = %#v, %v", result, err)
	}
}
