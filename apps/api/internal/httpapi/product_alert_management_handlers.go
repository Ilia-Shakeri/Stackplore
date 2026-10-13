package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/alerting"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

type alertRuleRequest struct {
	SourceID                  domain.SourceID            `json:"source_id"`
	EnvironmentID             *domain.EnvironmentID      `json:"environment_id,omitempty"`
	ResourceID                *domain.ResourceID         `json:"resource_id,omitempty"`
	DisplayName               string                     `json:"display_name"`
	Kind                      domain.AlertRuleKind       `json:"kind"`
	Enabled                   *bool                      `json:"enabled"`
	Severity                  domain.Severity            `json:"severity"`
	Comparison                *domain.ComparisonOperator `json:"comparison_operator,omitempty"`
	ThresholdValue            *float64                   `json:"threshold_value,omitempty"`
	ThresholdCount            *uint64                    `json:"threshold_count,omitempty"`
	WindowSeconds             *int64                     `json:"window_seconds,omitempty"`
	MinimumSeverity           *domain.Severity           `json:"minimum_event_severity,omitempty"`
	EvaluationIntervalSeconds int64                      `json:"evaluation_interval_seconds"`
	PendingForSeconds         int64                      `json:"pending_for_seconds"`
}

type alertRuleResponse struct {
	ID                        domain.AlertRuleID         `json:"id"`
	SourceID                  domain.SourceID            `json:"source_id"`
	EnvironmentID             *domain.EnvironmentID      `json:"environment_id,omitempty"`
	ResourceID                *domain.ResourceID         `json:"resource_id,omitempty"`
	DisplayName               string                     `json:"display_name"`
	Kind                      string                     `json:"kind"`
	Enabled                   bool                       `json:"enabled"`
	Severity                  string                     `json:"severity"`
	Comparison                *domain.ComparisonOperator `json:"comparison_operator,omitempty"`
	ThresholdValue            *float64                   `json:"threshold_value,omitempty"`
	ThresholdCount            *uint64                    `json:"threshold_count,omitempty"`
	WindowSeconds             *int64                     `json:"window_seconds,omitempty"`
	MinimumSeverity           *domain.Severity           `json:"minimum_event_severity,omitempty"`
	EvaluationIntervalSeconds int64                      `json:"evaluation_interval_seconds"`
	PendingForSeconds         int64                      `json:"pending_for_seconds"`
	CreatedAt                 string                     `json:"created_at"`
	UpdatedAt                 string                     `json:"updated_at"`
	Version                   int64                      `json:"version"`
}

type alertRulePageResponse struct {
	Items []alertRuleResponse `json:"items"`
	Page  pageResponse        `json:"page"`
}

func (request alertRuleRequest) input() (alerting.RuleInput, error) {
	const maximumDurationSeconds int64 = 86_400
	const maximumSafeInteger uint64 = 1<<53 - 1
	if request.Enabled == nil || request.EvaluationIntervalSeconds < 1 || request.EvaluationIntervalSeconds > maximumDurationSeconds ||
		request.PendingForSeconds < 0 || request.PendingForSeconds > maximumDurationSeconds ||
		(request.WindowSeconds != nil && (*request.WindowSeconds < 1 || *request.WindowSeconds > maximumDurationSeconds)) ||
		(request.ThresholdCount != nil && *request.ThresholdCount > maximumSafeInteger) {
		return alerting.RuleInput{}, alerting.ErrManagementInvalid
	}
	window := time.Duration(0)
	if request.WindowSeconds != nil {
		window = time.Duration(*request.WindowSeconds) * time.Second
	}
	input := alerting.RuleInput{
		SourceID: request.SourceID, EnvironmentID: request.EnvironmentID, ResourceID: request.ResourceID,
		DisplayName: request.DisplayName, Kind: request.Kind, Enabled: *request.Enabled, Severity: request.Severity,
		Comparison: request.Comparison, ThresholdValue: request.ThresholdValue, ThresholdCount: request.ThresholdCount,
		Window: window, MinimumSeverity: request.MinimumSeverity,
		EvaluationInterval: time.Duration(request.EvaluationIntervalSeconds) * time.Second,
		PendingFor:         time.Duration(request.PendingForSeconds) * time.Second,
	}
	return input, nil
}

func (handler productHTTP) listAlertRules(w http.ResponseWriter, r *http.Request) {
	if !requestHasNoBody(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	identity, ok := handler.authenticateIdentity(w, r)
	if !ok {
		return
	}
	if identity.Principal.Role != auth.RoleAdministrator {
		writeProblem(w, r, forbiddenProblem)
		return
	}
	if handler.alertManager == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	request, err := parsePageQuery(r, 50)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	page, err := handler.alertManager.ListRules(r.Context(), request)
	if err != nil {
		writeAlertManagementError(w, r, err)
		return
	}
	items := make([]alertRuleResponse, len(page.Rules))
	for index, record := range page.Rules {
		items[index] = alertRuleJSON(record)
	}
	writeJSON(w, http.StatusOK, alertRulePageResponse{Items: items, Page: pageJSON(page.Page)})
}

func (handler productHTTP) createAlertRule(w http.ResponseWriter, r *http.Request) {
	identity, sessionToken, ok := handler.authorizeAlertRuleMutation(w, r)
	if !ok {
		return
	}
	var request alertRuleRequest
	if err := decodeJSONBody(r, handler.auth.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	input, err := request.input()
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	record, err := handler.alertManager.CreateRule(r.Context(), input, domain.UserID(identity.Principal.UserID), requestIDForAuth(r))
	if err != nil {
		writeAlertManagementError(w, r, err)
		return
	}
	setUserEntityTag(w, record.Version)
	handler.auth.renewSessionCookie(w, r, sessionToken)
	writeJSON(w, http.StatusCreated, alertRuleJSON(record))
}

func (handler productHTTP) replaceAlertRule(w http.ResponseWriter, r *http.Request) {
	identity, sessionToken, ok := handler.authorizeAlertRuleMutation(w, r)
	if !ok {
		return
	}
	ruleID := domain.AlertRuleID(chi.URLParam(r, "rule_id"))
	version, err := parseExpectedVersion(r)
	if ruleID.Validate() != nil || err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	var request alertRuleRequest
	if err := decodeJSONBody(r, handler.auth.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	input, err := request.input()
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	record, err := handler.alertManager.ReplaceRule(r.Context(), ruleID, version, input, domain.UserID(identity.Principal.UserID), requestIDForAuth(r))
	if err != nil {
		writeAlertManagementError(w, r, err)
		return
	}
	setUserEntityTag(w, record.Version)
	handler.auth.renewSessionCookie(w, r, sessionToken)
	writeJSON(w, http.StatusOK, alertRuleJSON(record))
}

func (handler productHTTP) deleteAlertRule(w http.ResponseWriter, r *http.Request) {
	if !requestHasNoBody(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	identity, sessionToken, ok := handler.authorizeAlertRuleMutation(w, r)
	if !ok {
		return
	}
	ruleID := domain.AlertRuleID(chi.URLParam(r, "rule_id"))
	version, err := parseExpectedVersion(r)
	if ruleID.Validate() != nil || err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	if err := handler.alertManager.DeleteRule(r.Context(), ruleID, version, domain.UserID(identity.Principal.UserID), requestIDForAuth(r)); err != nil {
		writeAlertManagementError(w, r, err)
		return
	}
	handler.auth.renewSessionCookie(w, r, sessionToken)
	writeNoContent(w)
}

func (handler productHTTP) authorizeAlertRuleMutation(w http.ResponseWriter, r *http.Request) (auth.Authentication, string, bool) {
	identity, token, ok := handler.authorizeAdministratorMutation(w, r)
	if !ok {
		return auth.Authentication{}, "", false
	}
	if handler.alertManager == nil {
		writeProblem(w, r, unavailableProblem)
		return auth.Authentication{}, "", false
	}
	return identity, token, true
}

func (handler productHTTP) authorizeAdministratorMutation(w http.ResponseWriter, r *http.Request) (auth.Authentication, string, bool) {
	if !handler.auth.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return auth.Authentication{}, "", false
	}
	sessionToken, csrfProof, _, failure, ok := readAuthenticationMaterial(r, true, false)
	if !ok {
		writeProblem(w, r, failure)
		return auth.Authentication{}, "", false
	}
	identity, err := handler.auth.service.AuthorizeUnsafe(r.Context(), sessionToken, csrfProof)
	if err != nil {
		handler.auth.writeAuthenticatedError(w, r, sessionToken, err)
		return auth.Authentication{}, "", false
	}
	if identity.Principal.Role != auth.RoleAdministrator {
		writeProblem(w, r, forbiddenProblem)
		return auth.Authentication{}, "", false
	}
	return identity, sessionToken, true
}

func alertRuleJSON(record domain.AlertRuleRecord) alertRuleResponse {
	response := alertRuleResponse{
		ID: record.Rule.ID, SourceID: record.Rule.SourceID, EnvironmentID: record.Rule.EnvironmentID,
		ResourceID: record.Rule.ResourceID, DisplayName: record.Rule.DisplayName, Kind: string(record.Rule.Kind),
		Enabled: record.Rule.Enabled, Severity: string(record.Rule.Severity), Comparison: record.Rule.Comparison,
		ThresholdValue: record.Rule.ThresholdValue, ThresholdCount: record.Rule.ThresholdCount,
		MinimumSeverity:           record.Rule.MinimumSeverity,
		EvaluationIntervalSeconds: int64(record.Rule.EvaluationInterval / time.Second),
		PendingForSeconds:         int64(record.Rule.PendingFor / time.Second), CreatedAt: authTimestamp(record.CreatedAt),
		UpdatedAt: authTimestamp(record.UpdatedAt), Version: record.Version,
	}
	if record.Rule.Window > 0 {
		value := int64(record.Rule.Window / time.Second)
		response.WindowSeconds = &value
	}
	return response
}

func writeAlertManagementError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, alerting.ErrManagementInvalid):
		writeProblem(w, r, invalidRequestProblem)
	case errors.Is(err, alerting.ErrAlertNotFound):
		writeProblem(w, r, notFoundProblem)
	case errors.Is(err, alerting.ErrAlertStateConflict):
		writeProblem(w, r, conflictProblem)
	case errors.Is(err, alerting.ErrManagementUnavailable):
		writeProblem(w, r, unavailableProblem)
	default:
		writeProblem(w, r, internalProblem)
	}
}
