package assistantservice

import (
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantplan"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/incidentreport"
)

// Only freshly authorized sealed proofs can establish readable history coverage.
func historyCoverage(proofs []assistantevidence.Evidence) incidentreport.HistoryCoverage {
	var coverage incidentreport.HistoryCoverage
	for _, proof := range proofs {
		state := proof.Bundle().Summary().Availability
		if state != domain.AvailabilityAvailable && state != domain.AvailabilityStale {
			continue
		}
		switch proof.ReadKind() {
		case assistant.AlertHistory:
			coverage.AlertsRead = true
		case assistant.WorkloadHistory:
			coverage.WorkloadRead = true
		}
	}
	return coverage
}

// The surrounding execution path freshly checks all proofs before this pure
// projection and again after durable completion, before volatile publication.
func (service *Service) incidentReport(entry *memoryRun) (*incidentreport.Report, error) {
	service.mu.Lock()
	if entry.revoked || service.closed {
		service.mu.Unlock()
		return nil, assistant.ErrDenied
	}
	if entry.input.Admission.Goal != assistant.InvestigateIncident {
		service.mu.Unlock()
		return nil, nil
	}
	scope := cloneScope(entry.input.Scope)
	proofs := service.evidenceSetLocked(entry)
	service.mu.Unlock()
	defer clear(proofs)
	if len(proofs) < 1 || len(proofs) > assistantplan.MaximumReads {
		return nil, assistant.ErrDenied
	}
	scope.ObservedAt = service.time()
	var observations []contextbuilder.Observation
	defer func() { clear(observations) }()
	for _, proof := range proofs {
		if proof.ReadKind() == assistant.ReviewedDocument {
			continue
		}
		items, err := proof.Bundle().Observations()
		if err != nil || len(observations)+len(items) > incidentreport.MaximumObservations {
			clear(items)
			return nil, assistant.ErrUnavailable
		}
		observations = append(observations, items...)
		clear(items)
	}
	report, err := incidentreport.BuildWithHistory(incidentreport.Scope{Source: scope.Source, ResourceID: scope.Resources[0].ID, ConfigurationVersion: scope.ConfigurationVersion, PolicyVersion: scope.PolicyVersion, RestoreGeneration: scope.RestoreGeneration, Range: scope.Range, ObservedAt: scope.ObservedAt}, observations, historyCoverage(proofs))
	if err != nil {
		return nil, assistant.ErrUnavailable
	}
	return &report, nil
}
