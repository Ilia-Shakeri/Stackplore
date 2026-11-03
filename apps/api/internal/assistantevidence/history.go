package assistantevidence

import (
	"context"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// The surrounding Read path fences actor, source and exact live inventory
// before and after this bounded local repository read, and seals its proof.
func (reader *Reader) collectHistory(ctx context.Context, source Source, request ReadRequest, project *contextbuilder.Request) error {
	if request.Destination != contextbuilder.Local || request.LogConsent || len(request.Scope.Resources) != 1 || request.RecordLimit > domain.MaximumIncidentHistory {
		return ErrDenied
	}
	if source.History == nil {
		project.Status.State, project.Status.Completeness, project.Status.ReasonCode = domain.AvailabilityUnsupported, domain.CompletenessComplete, "source.history_unsupported"
		return nil
	}
	query := domain.IncidentAlertHistoryQuery{Source: request.Scope.Source, Resource: request.Scope.Resources[0], Range: request.Scope.Range, Limit: request.RecordLimit}
	if query.Validate() != nil {
		return ErrInvalid
	}
	switch request.Kind {
	case assistant.AlertHistory:
		history, err := source.History.GetIncidentAlertHistory(ctx, query)
		if err != nil {
			return err
		}
		if history.ValidateFor(query) != nil {
			return ErrUnavailable
		}
		project.Status, project.AlertHistory, project.MoreAvailable = history.Status, &history, history.MoreAvailable
	case assistant.WorkloadHistory:
		workloadQuery := domain.IncidentWorkloadHistoryQuery(query)
		history, err := source.History.GetIncidentWorkloadHistory(ctx, workloadQuery)
		if err != nil {
			return err
		}
		if history.ValidateFor(workloadQuery) != nil {
			return ErrUnavailable
		}
		project.Status, project.WorkloadHistory, project.MoreAvailable = history.Status, &history, history.MoreAvailable
	default:
		return ErrInvalid
	}
	if project.Status.State == domain.AvailabilityPermissionDenied {
		return ErrDenied
	}
	return nil
}
