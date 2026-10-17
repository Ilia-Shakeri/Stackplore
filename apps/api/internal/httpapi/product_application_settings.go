package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/appsettings"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type ApplicationSettings interface {
	Get(context.Context) (domain.ApplicationSettingsRecord, error)
	AuditPolicy(context.Context) (domain.AuditPolicyStatus, error)
	Version() string
	Preview(context.Context, auth.Principal, int64, domain.ApplicationValues) (appsettings.Preview, error)
	Update(context.Context, auth.Principal, int64, domain.ApplicationValues, string, bool, auth.RecentGrantUse, domain.RequestID) (domain.ApplicationSettingsRecord, error)
	PreviewCleanup(context.Context, auth.Principal, int64, []domain.HistoryClass) (appsettings.CleanupPreview, error)
	Cleanup(context.Context, auth.Principal, int64, string, bool, auth.RecentGrantUse, domain.RequestID) (appsettings.CleanupResult, error)
	Export(context.Context, auth.Principal, auth.RecentGrantUse, domain.RequestID) (appsettings.ConfigurationExport, error)
}

type ApplicationMutationMode struct {
	Enabled                bool `json:"enabled"`
	DockerWriteEnabled     bool `json:"docker_write_enabled"`
	KubernetesWriteEnabled bool `json:"kubernetes_write_enabled"`
}

type applicationSettingsResponse struct {
	Version            int64                    `json:"version"`
	UpdatedAt          string                   `json:"updated_at"`
	Values             domain.ApplicationValues `json:"values"`
	MutationMode       ApplicationMutationMode  `json:"mutation_mode"`
	AuditPolicy        domain.AuditPolicyStatus `json:"audit_policy"`
	ApplicationVersion string                   `json:"application_version"`
}

func (handler productHTTP) applicationSettingsJSON(ctx context.Context, record domain.ApplicationSettingsRecord) (applicationSettingsResponse, error) {
	if record.Validate() != nil {
		return applicationSettingsResponse{}, appsettings.ErrUnavailable
	}
	policy, err := handler.applicationSettings.AuditPolicy(ctx)
	if err != nil {
		return applicationSettingsResponse{}, err
	}
	return applicationSettingsResponse{Version: record.Version, UpdatedAt: authTimestamp(record.UpdatedAt), Values: record.Values, MutationMode: handler.applicationMutationMode, AuditPolicy: policy, ApplicationVersion: handler.applicationSettings.Version()}, nil
}

func (handler productHTTP) getApplicationSettings(w http.ResponseWriter, r *http.Request) {
	if !requestHasNoBody(r) || !requireNoQuery(w, r) {
		if !requestHasNoBody(r) {
			writeProblem(w, r, invalidRequestProblem)
		}
		return
	}
	if _, ok := handler.authenticateIdentity(w, r); !ok {
		return
	}
	if handler.applicationSettings == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	record, err := handler.applicationSettings.Get(r.Context())
	if err != nil {
		handler.writeApplicationError(w, r, err)
		return
	}
	response, err := handler.applicationSettingsJSON(r.Context(), record)
	if err != nil {
		handler.writeApplicationError(w, r, err)
		return
	}
	setUserEntityTag(w, record.Version)
	writeJSON(w, http.StatusOK, response)
}

type applicationPreviewRequest struct {
	Values domain.ApplicationValues `json:"values"`
}
type applicationUpdateRequest struct {
	Values             domain.ApplicationValues `json:"values"`
	ConfirmationToken  secretInput              `json:"confirmation_token"`
	BackupAcknowledged *bool                    `json:"backup_acknowledged"`
}

func (handler productHTTP) previewApplicationSettings(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := handler.authorizeAdministratorMutation(w, r)
	if !ok {
		return
	}
	if handler.applicationSettings == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	version, err := parseExpectedVersion(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	var request applicationPreviewRequest
	if err := decodeJSONBody(r, handler.auth.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	preview, err := handler.applicationSettings.Preview(r.Context(), identity.Principal, version, request.Values)
	if err != nil {
		handler.writeApplicationError(w, r, err)
		return
	}
	settings, err := handler.applicationSettingsJSON(r.Context(), preview.Settings)
	if err != nil {
		handler.writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Settings          applicationSettingsResponse `json:"settings"`
		Requested         domain.ApplicationValues    `json:"requested"`
		Impact            domain.HistoryImpact        `json:"impact"`
		ConfirmationToken string                      `json:"confirmation_token"`
		ExpiresAt         string                      `json:"expires_at"`
	}{settings, preview.Requested, preview.Impact, preview.ConfirmationToken, authTimestamp(preview.ExpiresAt)})
}

type applicationMutationPreparer interface {
	PrepareRecentMutation(context.Context, auth.RecentAuthorizationInput) (auth.Authentication, auth.RecentGrantUse, error)
}

func (handler productHTTP) prepareApplicationMutation(w http.ResponseWriter, r *http.Request, purpose auth.RecentAuthPurpose) (auth.Authentication, auth.RecentGrantUse, bool) {
	if !handler.auth.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	token, csrf, recent, failure, ok := readAuthenticationMaterial(r, true, true)
	if !ok {
		writeProblem(w, r, failure)
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	if _, _, ok := handler.authorizeAdministratorMutation(w, r); !ok {
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	preparer, ok := handler.auth.service.(applicationMutationPreparer)
	if !ok || handler.applicationSettings == nil {
		writeProblem(w, r, unavailableProblem)
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	identity, grant, err := preparer.PrepareRecentMutation(r.Context(), auth.RecentAuthorizationInput{SessionToken: token, CSRFProof: csrf, RecentProof: recent, Purpose: purpose, RequestID: requestIDForAuth(r)})
	if err != nil {
		handler.auth.writeAuthenticatedError(w, r, token, err)
		return auth.Authentication{}, auth.RecentGrantUse{}, false
	}
	return identity, grant, true
}

func (handler productHTTP) updateApplicationSettings(w http.ResponseWriter, r *http.Request) {
	identity, grant, ok := handler.prepareApplicationMutation(w, r, auth.RecentAuthPurposeSettingsUpdate)
	if !ok {
		return
	}
	version, err := parseExpectedVersion(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	var request applicationUpdateRequest
	defer func() { clearSecret(request.ConfirmationToken) }()
	if err := decodeJSONBody(r, handler.auth.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	if request.BackupAcknowledged == nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	record, err := handler.applicationSettings.Update(r.Context(), identity.Principal, version, request.Values, string(request.ConfirmationToken), *request.BackupAcknowledged, grant, requestIDForAuth(r))
	if err != nil {
		handler.writeApplicationError(w, r, err)
		return
	}
	response, err := handler.applicationSettingsJSON(r.Context(), record)
	if err != nil {
		handler.writeApplicationError(w, r, err)
		return
	}
	setUserEntityTag(w, record.Version)
	writeJSON(w, http.StatusOK, response)
}

func (handler productHTTP) previewHistoryCleanup(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := handler.authorizeAdministratorMutation(w, r)
	if !ok {
		return
	}
	if handler.applicationSettings == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	version, err := parseExpectedVersion(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	var request struct {
		Classes []domain.HistoryClass `json:"classes"`
	}
	if err := decodeJSONBody(r, handler.auth.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	preview, err := handler.applicationSettings.PreviewCleanup(r.Context(), identity.Principal, version, request.Classes)
	if err != nil {
		handler.writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		PolicyVersion     int64                `json:"policy_version"`
		Impact            domain.HistoryImpact `json:"impact"`
		ConfirmationToken string               `json:"confirmation_token"`
		ExpiresAt         string               `json:"expires_at"`
	}{preview.PolicyVersion, preview.Impact, preview.ConfirmationToken, authTimestamp(preview.ExpiresAt)})
}

func (handler productHTTP) cleanupHistory(w http.ResponseWriter, r *http.Request) {
	identity, grant, ok := handler.prepareApplicationMutation(w, r, auth.RecentAuthPurposeDataCleanup)
	if !ok {
		return
	}
	version, err := parseExpectedVersion(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	var request struct {
		ConfirmationToken  secretInput `json:"confirmation_token"`
		BackupAcknowledged *bool       `json:"backup_acknowledged"`
	}
	defer func() { clearSecret(request.ConfirmationToken) }()
	if err := decodeJSONBody(r, handler.auth.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	if request.BackupAcknowledged == nil || !*request.BackupAcknowledged {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	result, err := handler.applicationSettings.Cleanup(r.Context(), identity.Principal, version, string(request.ConfirmationToken), true, grant, requestIDForAuth(r))
	if err != nil {
		handler.writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (handler productHTTP) exportConfiguration(w http.ResponseWriter, r *http.Request) {
	identity, grant, ok := handler.prepareApplicationMutation(w, r, auth.RecentAuthPurposeConfigurationExport)
	if !ok {
		return
	}
	var request struct{}
	if err := decodeJSONBody(r, handler.auth.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	result, err := handler.applicationSettings.Export(r.Context(), identity.Principal, grant, requestIDForAuth(r))
	if err != nil {
		handler.writeApplicationError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `attachment; filename="stackplore-configuration.json"`)
	writeJSON(w, http.StatusOK, result)
}

func (handler productHTTP) writeApplicationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, appsettings.ErrInvalid):
		writeProblem(w, r, invalidRequestProblem)
	case errors.Is(err, appsettings.ErrForbidden), errors.Is(err, auth.ErrAuthorizationDenied):
		writeProblem(w, r, forbiddenProblem)
	case errors.Is(err, auth.ErrRecentAuthRejected):
		handler.auth.writeError(w, r, err)
	case errors.Is(err, appsettings.ErrVersion):
		writeProblem(w, r, problemKind{status: 409, code: "settings.version_conflict", title: "Settings version changed"})
	case errors.Is(err, appsettings.ErrConfirmation):
		writeProblem(w, r, problemKind{status: 409, code: "settings.confirmation_rejected", title: "Settings confirmation rejected"})
	case errors.Is(err, appsettings.ErrBackup):
		writeProblem(w, r, problemKind{status: 409, code: "settings.backup_required", title: "Backup acknowledgement required"})
	default:
		writeProblem(w, r, unavailableProblem)
	}
}
