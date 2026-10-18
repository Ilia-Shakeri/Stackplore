package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/alerting"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/logsafe"
	"github.com/go-chi/chi/v5"
)

type relationshipResponse struct {
	ID         domain.RelationshipID          `json:"id"`
	From       domain.ResourceID              `json:"from"`
	To         domain.ResourceID              `json:"to"`
	Kind       string                         `json:"kind"`
	Direction  string                         `json:"direction"`
	ObservedAt string                         `json:"observed_at"`
	Synthetic  bool                           `json:"synthetic"`
	Confidence float64                        `json:"confidence"`
	Evidence   []relationshipEvidenceResponse `json:"evidence"`
}

type relationshipEvidenceResponse struct {
	ID         domain.EvidenceID             `json:"id"`
	Provider   domain.ProviderKind           `json:"provider"`
	SourceID   domain.SourceID               `json:"source_id"`
	Provenance domain.RelationshipProvenance `json:"provenance"`
	Certainty  domain.EvidenceCertainty      `json:"certainty"`
	Confidence float64                       `json:"confidence"`
	ObservedAt string                        `json:"observed_at"`
	Synthetic  bool                          `json:"synthetic"`
}

type relationshipPageResponse struct {
	observationResponse
	Items []relationshipResponse `json:"items"`
	Page  pageResponse           `json:"page"`
}

type logResponse struct {
	ID         domain.LogEntryID `json:"id"`
	ResourceID domain.ResourceID `json:"resource_id"`
	Container  string            `json:"container,omitempty"`
	Stream     string            `json:"stream"`
	Sequence   uint64            `json:"sequence"`
	Severity   string            `json:"severity"`
	Timestamp  string            `json:"timestamp"`
	Message    string            `json:"message"`
	Redacted   bool              `json:"redacted"`
	Truncated  bool              `json:"truncated"`
	Synthetic  bool              `json:"synthetic"`
}

type logPageResponse struct {
	observationResponse
	Items []logResponse `json:"items"`
	Page  pageResponse  `json:"page"`
}

type metricResponse struct {
	ID              domain.MetricSampleID `json:"id"`
	ResourceID      domain.ResourceID     `json:"resource_id"`
	Name            string                `json:"name"`
	Unit            string                `json:"unit"`
	Value           float64               `json:"value"`
	SampledAt       string                `json:"sampled_at"`
	IntervalSeconds int64                 `json:"interval_seconds"`
	Synthetic       bool                  `json:"synthetic"`
}

type metricResponseSet struct {
	observationResponse
	Items []metricResponse `json:"items"`
}

type metricHistoryResponse struct {
	Availability string           `json:"availability"`
	ObservedAt   string           `json:"observed_at"`
	Completeness string           `json:"completeness"`
	ReasonCode   string           `json:"reason_code,omitempty"`
	Items        []metricResponse `json:"items"`
}

type eventResponse struct {
	ID            domain.EventID        `json:"id"`
	ResourceID    *domain.ResourceID    `json:"resource_id,omitempty"`
	EnvironmentID *domain.EnvironmentID `json:"environment_id,omitempty"`
	Kind          string                `json:"kind"`
	Severity      string                `json:"severity"`
	ReasonCode    string                `json:"reason_code"`
	Summary       string                `json:"summary"`
	OccurredAt    string                `json:"occurred_at"`
	ObservedAt    string                `json:"observed_at"`
	Synthetic     bool                  `json:"synthetic"`
}

type eventPageResponse struct {
	observationResponse
	Items []eventResponse `json:"items"`
	Page  pageResponse    `json:"page"`
}

type alertResponse struct {
	ID             domain.AlertID        `json:"id"`
	RuleID         domain.AlertRuleID    `json:"rule_id"`
	ResourceID     *domain.ResourceID    `json:"resource_id,omitempty"`
	EnvironmentID  *domain.EnvironmentID `json:"environment_id,omitempty"`
	State          string                `json:"state"`
	Severity       string                `json:"severity"`
	SummaryCode    string                `json:"summary_code"`
	StartedAt      string                `json:"started_at"`
	UpdatedAt      string                `json:"updated_at"`
	AcknowledgedAt string                `json:"acknowledged_at,omitempty"`
	ResolvedAt     string                `json:"resolved_at,omitempty"`
	Synthetic      bool                  `json:"synthetic"`
}

type alertPageResponse struct {
	Items []alertResponse `json:"items"`
	Page  pageResponse    `json:"page"`
}

func (handler productHTTP) topology(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) || !handler.authenticate(w, r) {
		return
	}
	page, resourceID, err := parseReadQuery(r, 100, false)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	refs, err := handler.optionalResourceRefs(r, provider, resourceID)
	if err != nil {
		writeResourceProblem(w, r, err)
		return
	}
	result, err := provider.GetRelationships(r.Context(), domain.TopologyQuery{Source: providerRef(provider), Resources: refs, Page: page})
	if err != nil {
		writeProviderProblem(w, r, err)
		return
	}
	items := make([]relationshipResponse, len(result.Relationships))
	for index, relationship := range result.Relationships {
		confidence := 0.0
		for _, evidence := range relationship.Evidence {
			confidence = max(confidence, evidence.Confidence)
		}
		items[index] = relationshipResponse{
			ID: relationship.ID, From: relationship.From, To: relationship.To,
			Kind: string(relationship.Kind), Direction: string(relationship.Direction),
			ObservedAt: authTimestamp(relationship.ObservedAt), Synthetic: relationshipSynthetic(relationship), Confidence: confidence,
		}
		items[index].Evidence = make([]relationshipEvidenceResponse, len(relationship.Evidence))
		for i, evidence := range relationship.Evidence {
			items[index].Evidence[i] = relationshipEvidenceResponse{ID: evidence.ID, Provider: evidence.Provider, SourceID: evidence.SourceID, Provenance: evidence.Provenance, Certainty: evidence.Certainty, Confidence: evidence.Confidence, ObservedAt: authTimestamp(evidence.ObservedAt), Synthetic: evidence.Synthetic}
		}
	}
	writeJSON(w, http.StatusOK, relationshipPageResponse{observationResponse: observationJSON(result.Status), Items: items, Page: pageJSON(result.Page)})
}

func (handler productHTTP) logs(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) || !handler.authenticate(w, r) {
		return
	}
	page, resourceID, err := parseReadQuery(r, 100, false)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	logProvider, ok := provider.(domain.LogReadProvider)
	if !ok {
		writeProblem(w, r, unavailableProblem)
		return
	}
	if handler.applicationSettings != nil || handler.logRedactionPolicy.HasAdditionalFields() {
		logProvider = configuredLogReader{reader: logProvider, settings: handler.applicationSettings, policy: handler.logRedactionPolicy}
	}
	refs, selectionStatus, err := handler.resourceSelection(r, provider, resourceID, domain.MaxLogTargets)
	if err != nil {
		writeResourceProblem(w, r, err)
		return
	}
	targets := make([]domain.LogTarget, len(refs))
	for index, ref := range refs {
		targets[index] = domain.LogTarget{Resource: ref}
	}
	end := observationTime(provider).Add(time.Nanosecond)
	rangeValue, err := parseObservationRange(r, domain.TimeRange{Start: end.Add(-24 * time.Hour), End: end}, domain.MaxLogLookback)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	container, previous, err := parseLogSelection(r, resourceID)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	for i := range targets {
		targets[i].Container = container
		targets[i].Previous = previous
	}
	if len(targets) == 0 {
		status, err := emptyReadStatus(r, provider, selectionStatus, domain.CapabilityLogRead)
		if err != nil {
			writeProviderProblem(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, logPageResponse{observationResponse: observationJSON(status), Items: []logResponse{}, Page: pageJSON(domain.PageInfo{Limit: page.Limit})})
		return
	}
	result, err := logProvider.ReadLogs(r.Context(), domain.LogReadQuery{
		Source: providerRef(provider), Targets: targets,
		Range:     rangeValue,
		LineLimit: page.Limit, ByteLimit: domain.MaxLogBytes, Cursor: page.Cursor,
	})
	if err != nil {
		writeProviderProblem(w, r, err)
		return
	}
	result.Status = withSelectionStatus(result.Status, selectionStatus)
	items := make([]logResponse, len(result.Records))
	for index, record := range result.Records {
		items[index] = logResponse{
			ID: record.ID, ResourceID: record.ResourceID, Container: record.Container, Stream: string(record.Stream), Sequence: record.Sequence,
			Severity: string(record.Severity), Timestamp: authTimestamp(record.Timestamp), Message: record.Message,
			Redacted: record.Redacted, Truncated: record.Truncated, Synthetic: record.Synthetic,
		}
	}
	writeJSON(w, http.StatusOK, logPageResponse{observationResponse: observationJSON(result.Status), Items: items, Page: pageJSON(result.Page)})
}

func (handler productHTTP) metrics(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) || !handler.authenticate(w, r) {
		return
	}
	_, resourceID, err := parseReadQuery(r, domain.MaxPageSize, true)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	metricProvider, ok := provider.(domain.CurrentMetricProvider)
	if !ok {
		writeProblem(w, r, unavailableProblem)
		return
	}
	refs, selectionStatus, err := handler.resourceSelection(r, provider, resourceID, domain.MaxMetricTargets)
	if err != nil {
		writeResourceProblem(w, r, err)
		return
	}
	names := []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes, domain.MetricRestartCount}
	if len(refs) == 0 {
		status, err := emptyReadStatus(r, provider, selectionStatus, domain.CapabilityMetricCurrent)
		if err != nil {
			writeProviderProblem(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, metricResponseSet{observationResponse: observationJSON(status), Items: []metricResponse{}})
		return
	}
	result, err := metricProvider.GetCurrentMetrics(r.Context(), domain.CurrentMetricQuery{Source: providerRef(provider), Resources: refs, Names: names})
	if err != nil {
		writeProviderProblem(w, r, err)
		return
	}
	items := make([]metricResponse, 0, len(refs)*len(names))
	for _, set := range result.Sets {
		for _, sample := range set.Samples {
			items = append(items, metricResponse{
				ID: sample.ID, ResourceID: sample.ResourceID, Name: string(sample.Name), Unit: string(sample.Unit),
				Value: sample.Value, SampledAt: authTimestamp(sample.SampledAt),
				IntervalSeconds: int64(sample.Interval / time.Second), Synthetic: sample.Synthetic,
			})
		}
	}
	status := withSelectionStatus(currentMetricStatus(result), selectionStatus)
	if len(items) > domain.MaxPageSize {
		items = items[:domain.MaxPageSize]
		status.Completeness = domain.CompletenessPartial
		status.ReasonCode = "metric.response_bound"
	}
	writeJSON(w, http.StatusOK, metricResponseSet{observationResponse: observationJSON(status), Items: items})
}

func currentMetricStatus(result domain.CurrentMetricResult) domain.ObservationStatus {
	status := result.Status
	if !status.AllowsData() || len(result.Sets) == 0 {
		return status
	}
	missing := 0
	states := make(map[domain.AvailabilityState]int)
	partial := status.Completeness == domain.CompletenessPartial
	for _, set := range result.Sets {
		states[set.Status.State]++
		if !set.Status.AllowsData() {
			missing++
		}
		if set.Status.State != domain.AvailabilityAvailable || set.Status.Completeness != domain.CompletenessComplete {
			partial = true
		}
	}
	if missing == len(result.Sets) {
		status.State = domain.AvailabilityUnavailable
		status.ReasonCode = "metric.resources_unavailable"
		if states[domain.AvailabilityPermissionDenied] == missing {
			status.State, status.ReasonCode = domain.AvailabilityPermissionDenied, "metric.resources_permission_denied"
		} else if states[domain.AvailabilityUnsupported] == missing {
			status.State, status.ReasonCode = domain.AvailabilityUnsupported, "metric.resources_unsupported"
		}
		status.Completeness = domain.CompletenessComplete
		return status
	}
	if states[domain.AvailabilityStale] == len(result.Sets) {
		status.State, status.ReasonCode = domain.AvailabilityStale, "metric.resources_stale"
	}
	if partial {
		status.Completeness = domain.CompletenessPartial
		if status.ReasonCode == "" {
			status.ReasonCode = "metric.partial_resources"
		}
	}
	return status
}

func (handler productHTTP) metricHistory(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) || !handler.authenticate(w, r) {
		return
	}
	resourceID, lookback, limit, err := parseMetricHistoryQuery(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	historyProvider, ok := provider.(domain.MetricHistoryProvider)
	if !ok {
		writeProblem(w, r, unavailableProblem)
		return
	}
	refs, selectionStatus, err := handler.resourceSelection(r, provider, resourceID, domain.MaxMetricTargets)
	if err != nil {
		writeResourceProblem(w, r, err)
		return
	}
	names := []domain.MetricName{
		domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes,
		domain.MetricNetworkReceiveBytesTotal, domain.MetricNetworkSendBytesTotal, domain.MetricRestartCount,
	}
	end := observationTime(provider).Add(time.Nanosecond)
	rangeValue, err := parseObservationRange(r, domain.TimeRange{Start: end.Add(-lookback), End: end}, domain.MaxMetricLookback)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	if len(refs) == 0 {
		status, err := emptyReadStatus(r, provider, selectionStatus, domain.CapabilityMetricHistory)
		if err != nil {
			writeProviderProblem(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, metricHistoryResponse{Availability: string(status.State), Completeness: string(status.Completeness), ObservedAt: authTimestamp(status.ObservedAt), ReasonCode: string(status.ReasonCode), Items: []metricResponse{}})
		return
	}
	result, err := historyProvider.GetMetricHistory(r.Context(), domain.MetricHistoryQuery{
		Source: providerRef(provider), Resources: refs, Names: names,
		Range: rangeValue, PointLimit: limit,
	})
	if err != nil {
		writeProviderProblem(w, r, err)
		return
	}
	result.Status = withSelectionStatus(result.Status, selectionStatus)
	items := make([]metricResponse, len(result.Samples))
	for index, sample := range result.Samples {
		items[index] = metricResponse{
			ID: sample.ID, ResourceID: sample.ResourceID, Name: string(sample.Name), Unit: string(sample.Unit),
			Value: sample.Value, SampledAt: authTimestamp(sample.SampledAt),
			IntervalSeconds: int64(sample.Interval / time.Second), Synthetic: sample.Synthetic,
		}
	}
	writeJSON(w, http.StatusOK, metricHistoryResponse{
		Availability: string(result.Status.State), ObservedAt: authTimestamp(result.Status.ObservedAt), Completeness: string(result.Status.Completeness),
		ReasonCode: string(result.Status.ReasonCode), Items: items,
	})
}

func (handler productHTTP) events(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) || !handler.authenticate(w, r) {
		return
	}
	page, resourceID, err := parseReadQuery(r, 100, false)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	eventProvider, ok := provider.(domain.EventHistoryProvider)
	if !ok {
		writeProblem(w, r, unavailableProblem)
		return
	}
	refs, err := handler.optionalResourceRefs(r, provider, resourceID)
	if err != nil {
		writeResourceProblem(w, r, err)
		return
	}
	end := observationTime(provider).Add(time.Nanosecond)
	rangeValue, err := parseObservationRange(r, domain.TimeRange{Start: end.Add(-24 * time.Hour), End: end}, 30*24*time.Hour)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	query := domain.EventQuery{
		Source: providerRef(provider), Resources: refs,
		Range: rangeValue, Page: page,
		EnvironmentID: domain.EnvironmentID(r.URL.Query().Get("environment_id")), Search: r.URL.Query().Get("q"),
	}
	if value := r.URL.Query().Get("severity"); value != "" {
		query.Severities = []domain.Severity{domain.Severity(value)}
	}
	if query.Validate() != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	result, err := eventProvider.ListEvents(r.Context(), query)
	if err != nil {
		writeProviderProblem(w, r, err)
		return
	}
	items := make([]eventResponse, len(result.Events))
	for index, event := range result.Events {
		items[index] = eventJSON(event, handler.logRedactionPolicy)
	}
	writeJSON(w, http.StatusOK, eventPageResponse{observationResponse: observationJSON(result.Status), Items: items, Page: pageJSON(result.Page)})
}

func eventJSON(event domain.InfrastructureEvent, policy logsafe.Policy) eventResponse {
	summary, _, _ := policy.Message(event.Summary, false)
	if utf8.RuneCountInString(summary) > domain.MaxEventSummaryLength {
		summary = strings.TrimSpace(string([]rune(summary)[:domain.MaxEventSummaryLength]))
	}
	return eventResponse{
		ID: event.ID, ResourceID: event.ResourceID, EnvironmentID: event.EnvironmentID,
		Kind: string(event.Kind), Severity: string(event.Severity), ReasonCode: string(event.ReasonCode),
		Summary: summary, OccurredAt: authTimestamp(event.OccurredAt),
		ObservedAt: authTimestamp(event.ObservedAt), Synthetic: event.Synthetic,
	}
}

func (handler productHTTP) alerts(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) || !handler.authenticate(w, r) {
		return
	}
	pageRequest, err := parsePageQuery(r, 100)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	if handler.alertReader != nil {
		page, err := handler.alertReader.ListAlerts(r.Context(), provider.Source().ID, pageRequest)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrRepositoryInvalidInput):
				writeProblem(w, r, invalidRequestProblem)
			case errors.Is(err, domain.ErrRepositoryUnavailable):
				writeProblem(w, r, unavailableProblem)
			default:
				writeProblem(w, r, internalProblem)
			}
			return
		}
		writeJSON(w, http.StatusOK, alertPageJSON(page.Alerts, page.Page))
		return
	}
	if pageRequest.Cursor != "" {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	alertProvider, ok := provider.(interface{ Alerts() []domain.Alert })
	if !ok {
		writeJSON(w, http.StatusOK, alertPageResponse{Items: []alertResponse{}, Page: pageResponse{Limit: pageRequest.Limit, HasMore: false}})
		return
	}
	alerts := alertProvider.Alerts()
	if len(alerts) > pageRequest.Limit {
		alerts = alerts[:pageRequest.Limit]
	}
	writeJSON(w, http.StatusOK, alertPageJSON(alerts, domain.PageInfo{Limit: pageRequest.Limit}))
}

func (handler productHTTP) acknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	if !handler.auth.requireSameOrigin(w, r) || !requireNoBodyAndQuery(w, r) {
		return
	}
	sessionToken, csrfProof, _, failure, ok := readAuthenticationMaterial(r, true, false)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	identity, err := handler.auth.service.AuthorizeUnsafe(r.Context(), sessionToken, csrfProof)
	if err != nil {
		handler.auth.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	if identity.Principal.Role != auth.RoleAdministrator {
		writeProblem(w, r, forbiddenProblem)
		return
	}
	alertID := domain.AlertID(chi.URLParam(r, "alert_id"))
	if alertID.Validate() != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	if handler.alertManager == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	alert, err := handler.alertManager.Acknowledge(
		r.Context(), alertID, domain.UserID(identity.Principal.UserID), domain.RequestID(requestIDFromContext(r.Context())),
	)
	if err != nil {
		switch {
		case errors.Is(err, alerting.ErrAlertNotFound):
			writeProblem(w, r, notFoundProblem)
		case errors.Is(err, alerting.ErrAlertStateConflict):
			writeProblem(w, r, conflictProblem)
		case errors.Is(err, alerting.ErrManagementInvalid):
			writeProblem(w, r, invalidRequestProblem)
		case errors.Is(err, alerting.ErrManagementUnavailable):
			writeProblem(w, r, unavailableProblem)
		default:
			writeProblem(w, r, internalProblem)
		}
		return
	}
	handler.auth.renewSessionCookie(w, r, sessionToken)
	writeJSON(w, http.StatusOK, alertJSON(alert))
}

func alertPageJSON(alerts []domain.Alert, page domain.PageInfo) alertPageResponse {
	items := make([]alertResponse, len(alerts))
	for index, alert := range alerts {
		items[index] = alertJSON(alert)
	}
	return alertPageResponse{Items: items, Page: pageJSON(page)}
}

func alertJSON(alert domain.Alert) alertResponse {
	response := alertResponse{
		ID: alert.ID, RuleID: alert.RuleID, ResourceID: alert.ResourceID, EnvironmentID: alert.EnvironmentID,
		State: string(alert.State), Severity: string(alert.Severity), SummaryCode: string(alert.SummaryCode),
		StartedAt: authTimestamp(alert.StartedAt), UpdatedAt: authTimestamp(alert.UpdatedAt), Synthetic: alert.Synthetic,
	}
	if alert.AcknowledgedAt != nil {
		response.AcknowledgedAt = authTimestamp(*alert.AcknowledgedAt)
	}
	if alert.ResolvedAt != nil {
		response.ResolvedAt = authTimestamp(*alert.ResolvedAt)
	}
	return response
}

func parseReadQuery(r *http.Request, defaultLimit int, metrics bool) (domain.PageRequest, domain.ResourceID, error) {
	values := r.URL.Query()
	for key, items := range values {
		if len(items) != 1 || items[0] == "" {
			return domain.PageRequest{}, "", errors.New("invalid read query")
		}
		switch key {
		case "resource_id", "source_id":
		case "environment_id", "q", "severity":
			if r.URL.Path != "/api/v1/events" {
				return domain.PageRequest{}, "", errors.New("event filter unsupported")
			}
		case "from", "to":
			if r.URL.Path != "/api/v1/logs" && r.URL.Path != "/api/v1/events" {
				return domain.PageRequest{}, "", errors.New("time filter unsupported")
			}
		case "container", "previous":
			if r.URL.Path != "/api/v1/logs" {
				return domain.PageRequest{}, "", errors.New("container filter unsupported")
			}
		case "limit", "cursor":
			if metrics {
				return domain.PageRequest{}, "", errors.New("metrics do not use pages")
			}
		default:
			return domain.PageRequest{}, "", errors.New("unknown read query")
		}
	}
	limit := defaultLimit
	if value := values.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return domain.PageRequest{}, "", err
		}
		limit = parsed
	}
	page := domain.PageRequest{Limit: limit, Cursor: domain.PageCursor(values.Get("cursor"))}
	if err := page.Validate(); err != nil {
		return domain.PageRequest{}, "", err
	}
	resourceID := domain.ResourceID(values.Get("resource_id"))
	if resourceID != "" && resourceID.Validate() != nil {
		return domain.PageRequest{}, "", errors.New("invalid resource ID")
	}
	return page, resourceID, nil
}

func parseMetricHistoryQuery(r *http.Request) (domain.ResourceID, time.Duration, int, error) {
	values := r.URL.Query()
	for key, items := range values {
		if len(items) != 1 || items[0] == "" || key != "resource_id" && key != "minutes" && key != "limit" && key != "source_id" && key != "from" && key != "to" {
			return "", 0, 0, errors.New("invalid metric history query")
		}
	}
	resourceID := domain.ResourceID(values.Get("resource_id"))
	if resourceID != "" && resourceID.Validate() != nil {
		return "", 0, 0, errors.New("invalid resource ID")
	}
	minutes := 60
	if values.Has("minutes") && (values.Has("from") || values.Has("to")) {
		return "", 0, 0, errors.New("conflicting history range")
	}
	if value := values.Get("minutes"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return "", 0, 0, err
		}
		minutes = parsed
	}
	limit := domain.MaxMetricHistoryPoints
	if value := values.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return "", 0, 0, err
		}
		limit = parsed
	}
	if minutes < 5 || minutes > 30*24*60 || limit < 1 || limit > domain.MaxMetricHistoryPoints {
		return "", 0, 0, errors.New("metric history bounds are invalid")
	}
	return resourceID, time.Duration(minutes) * time.Minute, limit, nil
}

func (handler productHTTP) resourceRefs(r *http.Request, provider ProductProvider, selected domain.ResourceID, limit int) ([]domain.ResourceRef, error) {
	refs, _, err := handler.resourceSelection(r, provider, selected, limit)
	return refs, err
}

func (handler productHTTP) optionalResourceRefs(r *http.Request, provider ProductProvider, selected domain.ResourceID) ([]domain.ResourceRef, error) {
	if selected == "" {
		return nil, nil
	}
	return handler.resourceRefs(r, provider, selected, 1)
}

func (handler productHTTP) resourceSelection(r *http.Request, provider ProductProvider, selected domain.ResourceID, limit int) ([]domain.ResourceRef, domain.ObservationStatus, error) {
	if selected != "" {
		snapshot, err := provider.GetResource(r.Context(), domain.ResourceDetailQuery{Source: providerRef(provider), ResourceID: selected})
		if err != nil {
			return nil, domain.ObservationStatus{}, err
		}
		return []domain.ResourceRef{snapshot.Resource.Ref}, snapshot.Status, nil
	}
	page, err := provider.ListResources(r.Context(), domain.InventoryQuery{
		Source: providerRef(provider), SortField: domain.ResourceSortName, SortDirection: domain.SortAscending,
		Page: domain.PageRequest{Limit: limit},
	})
	if err != nil {
		return nil, domain.ObservationStatus{}, err
	}
	refs := make([]domain.ResourceRef, 0, len(page.Resources))
	for _, resource := range page.Resources {
		refs = append(refs, resource.Resource.Ref)
	}
	status := page.Status
	if page.Page.HasMore && status.AllowsData() {
		status.Completeness = domain.CompletenessPartial
		if status.ReasonCode == "" {
			status.ReasonCode = "source.resource_selection_bound"
		}
	}
	return refs, status, nil
}

func observationTime(provider ProductProvider) time.Time {
	if clock, ok := provider.(interface{ ObservationTime() time.Time }); ok {
		return clock.ObservationTime()
	}
	return time.Now().UTC().Truncate(time.Microsecond)
}

func relationshipSynthetic(relationship domain.Relationship) bool {
	for _, evidence := range relationship.Evidence {
		if evidence.Synthetic {
			return true
		}
	}
	return false
}

func pageJSON(page domain.PageInfo) pageResponse {
	return pageResponse{Limit: page.Limit, HasMore: page.HasMore, NextCursor: string(page.NextCursor)}
}
