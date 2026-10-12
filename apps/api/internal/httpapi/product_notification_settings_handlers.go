package httpapi

import (
	"context"
	"net/http"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
)

type NotificationSettingsReader interface {
	GetNotificationSettings(context.Context) (notification.RuntimeSettings, error)
}

type NotificationTester interface {
	RequestTest(context.Context, domain.UserID, domain.RequestID) (domain.Notification, error)
}

type notificationSettingsResponse struct {
	Adapter                string `json:"adapter"`
	Enabled                bool   `json:"enabled"`
	DestinationReferenceID string `json:"destination_reference_id,omitempty"`
	ConfigurationSource    string `json:"configuration_source"`
	DeliverySemantics      string `json:"delivery_semantics"`
}

type notificationTestResponse struct {
	NotificationID         domain.NotificationID `json:"notification_id"`
	DestinationReferenceID string                `json:"destination_reference_id"`
	State                  string                `json:"state"`
}

func (handler productHTTP) notificationSettings(w http.ResponseWriter, r *http.Request) {
	if !requireNoBodyAndQuery(w, r) {
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
	if handler.notificationSettingsReader == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	settings, err := handler.notificationSettingsReader.GetNotificationSettings(r.Context())
	if err != nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	writeJSON(w, http.StatusOK, notificationSettingsResponse{
		Adapter: notification.WebhookAdapter, Enabled: settings.Enabled,
		DestinationReferenceID: settings.DestinationReferenceID,
		ConfigurationSource:    "operator_environment", DeliverySemantics: "at_least_once",
	})
}

func (handler productHTTP) testNotification(w http.ResponseWriter, r *http.Request) {
	if handler.notificationTester == nil || handler.notificationSettingsReader == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	settings, err := handler.notificationSettingsReader.GetNotificationSettings(r.Context())
	if err != nil || !settings.Enabled {
		writeProblem(w, r, unavailableProblem)
		return
	}
	if !handler.auth.requireSameOrigin(w, r) || !requireNoBodyAndQuery(w, r) {
		return
	}
	sessionToken, csrfProof, recentProof, failure, ok := readAuthenticationMaterial(r, true, true)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	requestID := requestIDForAuth(r)
	identity, err := handler.auth.service.AuthorizeRecent(r.Context(), auth.RecentAuthorizationInput{
		SessionToken: sessionToken, CSRFProof: csrfProof, RecentProof: recentProof,
		Purpose: auth.RecentAuthPurposeNotificationTest, RequestID: requestID,
	})
	if err != nil {
		handler.auth.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	item, err := handler.notificationTester.RequestTest(
		r.Context(), domain.UserID(identity.Principal.UserID), requestID,
	)
	if err != nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	handler.auth.renewSessionCookie(w, r, sessionToken)
	writeJSON(w, http.StatusAccepted, notificationTestResponse{
		NotificationID: item.ID, DestinationReferenceID: settings.DestinationReferenceID, State: "queued",
	})
}
