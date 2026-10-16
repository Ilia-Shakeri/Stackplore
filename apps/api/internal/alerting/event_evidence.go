package alerting

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const maximumEventEvidencePages = 20

func (resolver *ProviderEvidence) eventCondition(ctx context.Context, provider EvidenceProvider, task EvaluationTask, collection domain.SourceCollectionOutcome, evaluatedAt, evidenceAt time.Time) (ConditionResult, error) {
	eventProvider, ok := provider.(domain.EventHistoryProvider)
	if !ok || !resolver.freshCompleteStatus(collection.EventStatus, evidenceAt) {
		return ConditionResult{}, ErrEvaluationUnavailable
	}
	window := domain.TimeRange{Start: evidenceAt.Add(-task.Rule.EvaluationInterval), End: evidenceAt}
	if err := resolver.requireCollectionCoverage(ctx, task.Source, domain.TimeRange{Start: evaluatedAt.Add(-task.Rule.EvaluationInterval), End: evaluatedAt}, evaluatedAt, domain.CapabilityEventHistory); err != nil {
		return ConditionResult{}, err
	}
	query := domain.EventQuery{Source: task.Source, Range: window, Page: domain.PageRequest{Limit: domain.MaxPageSize}}
	if task.Resource != nil {
		query.Resources = []domain.ResourceRef{*task.Resource}
	}
	seenCursors := make(map[domain.PageCursor]struct{}, maximumEventEvidencePages)
	seenEvents := make(map[domain.EventID]struct{})
	matched := false
	var previous *domain.InfrastructureEvent
	for index := 0; index < maximumEventEvidencePages; index++ {
		page, err := eventProvider.ListEvents(ctx, query)
		if err != nil || page.ValidateFor(query) != nil || !resolver.freshCompleteStatus(page.Status, evidenceAt) {
			return ConditionResult{}, errors.Join(ErrEvaluationUnavailable, err)
		}
		for _, event := range page.Events {
			if event.Synthetic != task.Synthetic || event.ObservedAt.After(evidenceAt) || previous != nil && (previous.OccurredAt.Before(event.OccurredAt) ||
				previous.OccurredAt.Equal(event.OccurredAt) && previous.ID >= event.ID) {
				return ConditionResult{}, ErrEvaluationUnavailable
			}
			if _, duplicate := seenEvents[event.ID]; duplicate {
				return ConditionResult{}, ErrEvaluationUnavailable
			}
			seenEvents[event.ID] = struct{}{}
			current := event
			previous = &current
			if task.Rule.EnvironmentID != nil && !sameOptional(task.Rule.EnvironmentID, event.EnvironmentID) {
				continue
			}
			if severityRank(event.Severity) >= severityRank(*task.Rule.MinimumSeverity) {
				matched = true
			}
		}
		if !page.Page.HasMore {
			suffix := "clear"
			if matched {
				suffix = "matched"
			}
			return ConditionResult{Matched: matched, EvidenceRange: window, SummaryCode: domain.ReasonCode("alert.event_severity_match." + suffix)}, nil
		}
		if _, repeated := seenCursors[page.Page.NextCursor]; repeated {
			return ConditionResult{}, ErrEvaluationUnavailable
		}
		seenCursors[page.Page.NextCursor] = struct{}{}
		query.Page.Cursor = page.Page.NextCursor
	}
	return ConditionResult{}, ErrEvaluationUnavailable
}
