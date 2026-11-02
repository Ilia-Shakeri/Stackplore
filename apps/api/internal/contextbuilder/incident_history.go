package contextbuilder

import (
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// These projections grant no authority and are not registered as reads.
func (projection *projection) alertHistory() error {
	history := projection.request.AlertHistory
	if history == nil {
		return nil
	}
	scope := projection.request.Scope
	if len(scope.Resources) != 1 || history.Status != projection.request.Status {
		return ErrInvalid
	}
	limit := domain.MaximumIncidentHistory
	if history.MoreAvailable {
		limit = len(history.Transitions)
	}
	query := domain.IncidentAlertHistoryQuery{Source: scope.Source, Resource: scope.Resources[0], Range: scope.Range, Limit: limit}
	if history.ValidateFor(query) != nil {
		return ErrInvalid
	}
	projection.bundle.document.MoreAvailable = projection.bundle.document.MoreAvailable || history.MoreAvailable
	for _, transition := range history.Transitions {
		var from *domain.AlertState
		if transition.From != nil {
			value := *transition.From
			from = &value
		}
		item := record{ResourceID: string(transition.ResourceID), Quality: projection.quality(history.Status), Alert: &alertFact{From: from, To: transition.To, OccurredAt: transition.OccurredAt}}
		// Local transition time can follow the last provider collection. The
		// actual occurrence stays distinct from this newly fenced read time.
		item.Quality.ObservedAt = scope.ObservedAt
		if err := projection.add("alert", string(transition.ID), item); err != nil {
			return err
		}
	}
	return nil
}

func (projection *projection) workloadHistory() error {
	history := projection.request.WorkloadHistory
	if history == nil {
		return nil
	}
	scope := projection.request.Scope
	if len(scope.Resources) != 1 || history.Status != projection.request.Status {
		return ErrInvalid
	}
	limit := domain.MaximumIncidentHistory
	if history.MoreAvailable {
		limit = len(history.Samples)
	}
	query := domain.IncidentWorkloadHistoryQuery{Source: scope.Source, Resource: scope.Resources[0], Range: scope.Range, Limit: limit}
	if history.ValidateFor(query) != nil {
		return ErrInvalid
	}
	projection.bundle.document.MoreAvailable = projection.bundle.document.MoreAvailable || history.MoreAvailable
	for _, sample := range history.Samples {
		if err := projection.status(sample.Status); err != nil {
			return err
		}
		fact := &workloadFact{SampledAt: sample.Status.ObservedAt}
		if sample.Status.AllowsData() {
			desired, ready := sample.Desired, sample.Ready
			fact.Desired, fact.Ready = &desired, &ready
		}
		item := record{ResourceID: string(sample.Resource.ID), Quality: projection.quality(sample.Status), Workload: fact}
		identity := fingerprint("workload_sample", string(sample.Resource.ID)+"\x00"+sample.Status.ObservedAt.Format(time.RFC3339Nano))
		if err := projection.add("workload", identity, item); err != nil {
			return err
		}
	}
	return nil
}
