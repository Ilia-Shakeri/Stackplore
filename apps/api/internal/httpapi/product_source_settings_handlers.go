package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/sourcesettings"
	"github.com/go-chi/chi/v5"
)

type SourceSettings interface {
	List(context.Context, domain.PageRequest) (domain.SourceConfigurationPage, error)
	ProfileAvailable(domain.SourceConfiguration) bool
	Preview(context.Context, auth.Principal, domain.SourceID, int64, bool) (sourcesettings.Preview, error)
	Update(context.Context, auth.Principal, domain.SourceID, int64, bool, string, auth.RecentGrantUse, domain.RequestID) (domain.SourceConfiguration, error)
}

type sourceMutationPreparer interface {
	PrepareSourceMutation(context.Context, auth.RecentAuthorizationInput) (auth.Authentication, auth.RecentGrantUse, error)
}

type sourceSettingsRecord struct {
	ID               domain.SourceID     `json:"id"`
	Provider         domain.ProviderKind `json:"provider"`
	Name             string              `json:"name"`
	Synthetic        bool                `json:"synthetic"`
	Enabled          bool                `json:"enabled"`
	Version          int64               `json:"version"`
	ProfileAvailable bool                `json:"profile_available"`
}

type sourceSettingsPage struct {
	Items []sourceSettingsRecord `json:"items"`
	Page  pageResponse           `json:"page"`
}

type sourceUpdateRequest struct {
	Enabled           *bool       `json:"enabled"`
	ConfirmationToken secretInput `json:"confirmation_token"`
}

type sourcePreviewRequest struct {
	Enabled *bool `json:"enabled"`
}

type sourceUpdatePreview struct {
	Source            sourceSettingsRecord `json:"source"`
	RequestedEnabled  bool                 `json:"requested_enabled"`
	Impact            string               `json:"impact"`
	ConfirmationToken string               `json:"confirmation_token"`
	ExpiresAt         string               `json:"expires_at"`
}

func (handler productHTTP) sourceSettingsJSON(configuration domain.SourceConfiguration) sourceSettingsRecord {
	return sourceSettingsRecord{ID: configuration.Source.ID, Provider: configuration.Source.Provider, Name: configuration.Source.Name,
		Synthetic: configuration.Source.Synthetic, Enabled: configuration.Enabled, Version: configuration.Version,
		ProfileAvailable: handler.sourceSettings.ProfileAvailable(configuration)}
}

func (handler productHTTP) listSourceSettings(w http.ResponseWriter, r *http.Request) {
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
	if handler.sourceSettings == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	request, err := parsePageQuery(r, 50)
	if err != nil || request.Limit > 100 {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	page, err := handler.sourceSettings.List(r.Context(), request)
	if err != nil {
		handler.writeSourceSettingsError(w, r, err)
		return
	}
	if page.ValidateFor(request) != nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	items := make([]sourceSettingsRecord, len(page.Configurations))
	for index, configuration := range page.Configurations {
		items[index] = handler.sourceSettingsJSON(configuration)
	}
	writeJSON(w, http.StatusOK, sourceSettingsPage{Items: items, Page: pageJSON(page.Page)})
}

func (handler productHTTP) previewSourceUpdate(w http.ResponseWriter, r *http.Request) {
	identity, token, ok := handler.authorizeAdministratorMutation(w, r)
	if !ok {
		return
	}
	if handler.sourceSettings == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	id := domain.SourceID(chi.URLParam(r, "source_id"))
	version, err := parseExpectedVersion(r)
	if id.Validate() != nil || err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	var request sourcePreviewRequest
	if err := decodeJSONBody(r, handler.auth.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	if request.Enabled == nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	preview, err := handler.sourceSettings.Preview(r.Context(), identity.Principal, id, version, *request.Enabled)
	if err != nil {
		handler.writeSourceSettingsError(w, r, err)
		return
	}
	impact := "collection.stop"
	if preview.RequestedEnabled {
		impact = "collection.start"
	}
	handler.auth.renewSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, sourceUpdatePreview{Source: handler.sourceSettingsJSON(preview.Configuration),
		RequestedEnabled: preview.RequestedEnabled, Impact: impact, ConfirmationToken: preview.ConfirmationToken, ExpiresAt: authTimestamp(preview.ExpiresAt)})
}

func (handler productHTTP) updateSourceSettings(w http.ResponseWriter, r *http.Request) {
	if !handler.auth.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return
	}
	token, csrf, recent, failure, ok := readAuthenticationMaterial(r, true, true)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	preparer, ok := handler.auth.service.(sourceMutationPreparer)
	if !ok || handler.sourceSettings == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	if _, _, ok := handler.authorizeAdministratorMutation(w, r); !ok {
		return
	}
	id := domain.SourceID(chi.URLParam(r, "source_id"))
	version, err := parseExpectedVersion(r)
	if id.Validate() != nil || err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	var request sourceUpdateRequest
	defer func() { clearSecret(request.ConfirmationToken) }()
	if err := decodeJSONBody(r, handler.auth.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	if request.Enabled == nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	identity, grant, err := preparer.PrepareSourceMutation(r.Context(), auth.RecentAuthorizationInput{
		SessionToken: token, CSRFProof: csrf, RecentProof: recent, Purpose: auth.RecentAuthPurposeSourceUpdate, RequestID: requestIDForAuth(r)})
	if err != nil {
		handler.auth.writeAuthenticatedError(w, r, token, err)
		return
	}
	configuration, err := handler.sourceSettings.Update(r.Context(), identity.Principal, id, version, *request.Enabled, string(request.ConfirmationToken), grant, requestIDForAuth(r))
	if err != nil {
		handler.writeSourceSettingsError(w, r, err)
		return
	}
	setUserEntityTag(w, configuration.Version)
	handler.auth.renewSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, handler.sourceSettingsJSON(configuration))
}

func (handler productHTTP) writeSourceSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, sourcesettings.ErrInvalid), errors.Is(err, domain.ErrRepositoryInvalidInput):
		writeProblem(w, r, invalidRequestProblem)
	case errors.Is(err, domain.ErrRepositoryNotFound):
		writeProblem(w, r, notFoundProblem)
	case errors.Is(err, sourcesettings.ErrVersionConflict):
		writeProblem(w, r, problemKind{status: 409, code: "source.version_conflict", title: "Source version changed"})
	case errors.Is(err, sourcesettings.ErrConfirmationRejected):
		writeProblem(w, r, problemKind{status: 409, code: "source.confirmation_rejected", title: "Source confirmation rejected"})
	case errors.Is(err, sourcesettings.ErrStateUnchanged):
		writeProblem(w, r, problemKind{status: 409, code: "source.state_unchanged", title: "Source state unchanged"})
	case errors.Is(err, sourcesettings.ErrProfileUnavailable):
		writeProblem(w, r, problemKind{status: 422, code: "source.profile_unavailable", title: "Source profile unavailable"})
	case errors.Is(err, auth.ErrAuthorizationDenied), errors.Is(err, auth.ErrRecentAuthRejected):
		handler.auth.writeError(w, r, err)
	default:
		writeProblem(w, r, unavailableProblem)
	}
}
