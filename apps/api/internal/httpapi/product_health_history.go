package httpapi

import (
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"net/http"
	"time"
)

type healthHistoryItemResponse struct {
	ID           string                   `json:"id"`
	ResourceID   domain.ResourceID        `json:"resource_id"`
	State        domain.HealthState       `json:"state"`
	Availability domain.AvailabilityState `json:"availability"`
	Completeness domain.Completeness      `json:"completeness"`
	ReasonCode   domain.ReasonCode        `json:"reason_code,omitempty"`
	ObservedAt   string                   `json:"observed_at"`
	Synthetic    bool                     `json:"synthetic"`
}
type healthHistoryResponse struct {
	observationResponse
	Items []healthHistoryItemResponse `json:"items"`
}

func (handler productHTTP) healthHistory(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) || !handler.authenticate(w, r) {
		return
	}
	resourceID, lookback, limit, err := parseMetricHistoryQuery(r)
	if err != nil || resourceID == "" {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	refs, status, err := handler.resourceSelection(r, provider, resourceID, 1)
	if err != nil {
		writeResourceProblem(w, r, err)
		return
	}
	if len(refs) != 1 {
		writeJSON(w, http.StatusOK, healthHistoryResponse{observationResponse: observationJSON(status), Items: []healthHistoryItemResponse{}})
		return
	}
	reader, ok := provider.(domain.HealthHistoryProvider)
	if !ok {
		status.State = domain.AvailabilityUnsupported
		status.Completeness = domain.CompletenessComplete
		status.ReasonCode = "source.health_history_unsupported"
		writeJSON(w, http.StatusOK, healthHistoryResponse{observationResponse: observationJSON(status), Items: []healthHistoryItemResponse{}})
		return
	}
	end := observationTime(provider).Add(time.Microsecond)
	window, err := parseObservationRange(r, domain.TimeRange{Start: end.Add(-lookback), End: end}, domain.MaxMetricLookback)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	query := domain.HealthHistoryQuery{Source: providerRef(provider), Resource: refs[0], Range: window, Limit: limit}
	result, err := reader.GetHealthHistory(r.Context(), query)
	if err != nil {
		writeProviderProblem(w, r, err)
		return
	}
	if result.ValidateFor(query) != nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	items := make([]healthHistoryItemResponse, 0, len(result.Samples))
	for _, sample := range result.Samples {
		items = append(items, healthHistoryItemResponse{ID: sample.ID, ResourceID: sample.ResourceID, State: sample.State, Availability: sample.Status.State, Completeness: sample.Status.Completeness, ReasonCode: sample.Status.ReasonCode, ObservedAt: authTimestamp(sample.Status.ObservedAt), Synthetic: sample.Synthetic})
	}
	writeJSON(w, http.StatusOK, healthHistoryResponse{observationResponse: observationJSON(withSelectionStatus(result.Status, status)), Items: items})
}
