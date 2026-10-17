package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type AuditReader interface {
	ListAuditEntries(context.Context, domain.PageRequest) (domain.AuditPage, error)
}

type AlertReader interface {
	ListAlerts(context.Context, domain.SourceID, domain.PageRequest) (domain.AlertPage, error)
}

type NotificationReader interface {
	ListNotifications(context.Context, domain.PageRequest) (domain.NotificationPage, error)
}

type auditEntryResponse struct {
	ID          domain.AuditEntryID `json:"id"`
	OccurredAt  string              `json:"occurred_at"`
	Actor       auditActorResponse  `json:"actor"`
	Action      string              `json:"action"`
	Target      auditTargetResponse `json:"target"`
	Outcome     string              `json:"outcome"`
	RequestID   domain.RequestID    `json:"request_id"`
	SummaryCode string              `json:"summary_code"`
}

type auditActorResponse struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}

type auditTargetResponse struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}

type auditPageResponse struct {
	Items []auditEntryResponse `json:"items"`
	Page  pageResponse         `json:"page"`
}

type notificationResponse struct {
	ID          domain.NotificationID `json:"id"`
	SourceID    *domain.SourceID      `json:"source_id,omitempty"`
	AlertID     *domain.AlertID       `json:"alert_id,omitempty"`
	ResourceID  *domain.ResourceID    `json:"resource_id,omitempty"`
	Kind        string                `json:"kind"`
	Severity    string                `json:"severity"`
	SummaryCode string                `json:"summary_code"`
	CreatedAt   string                `json:"created_at"`
	Synthetic   bool                  `json:"synthetic"`
}

type notificationPageResponse struct {
	Items []notificationResponse `json:"items"`
	Page  pageResponse           `json:"page"`
}

func (handler productHTTP) notifications(w http.ResponseWriter, r *http.Request) {
	if !requestHasNoBody(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	if !handler.authenticate(w, r) {
		return
	}
	if handler.notificationReader == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	if r.URL.Query().Has("source_id") {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	pageRequest, err := parsePageQuery(r, 50)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	page, err := handler.notificationReader.ListNotifications(r.Context(), pageRequest)
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
	items := make([]notificationResponse, len(page.Notifications))
	for index, notification := range page.Notifications {
		if handler.auth.guestDemo != nil && (!notification.Synthetic || notification.SourceID == nil || *notification.SourceID != handler.ordered[0].Source().ID) {
			writeProblem(w, r, unavailableProblem)
			return
		}
		items[index] = notificationResponse{
			ID: notification.ID, SourceID: notification.SourceID, AlertID: notification.AlertID,
			ResourceID: notification.ResourceID, Kind: string(notification.Kind),
			Severity: string(notification.Severity), SummaryCode: string(notification.SummaryCode),
			CreatedAt: authTimestamp(notification.CreatedAt), Synthetic: notification.Synthetic,
		}
	}
	writeJSON(w, http.StatusOK, notificationPageResponse{Items: items, Page: pageJSON(page.Page)})
}

func (handler productHTTP) audit(w http.ResponseWriter, r *http.Request) {
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
	if handler.auditReader == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	if r.URL.Query().Has("source_id") {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	pageRequest, err := parsePageQuery(r, 50)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	page, err := handler.auditReader.ListAuditEntries(r.Context(), pageRequest)
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
	items := make([]auditEntryResponse, len(page.Entries))
	for index, entry := range page.Entries {
		items[index] = auditEntryResponse{
			ID: entry.ID, OccurredAt: authTimestamp(entry.OccurredAt),
			Actor: auditActorResponse{Kind: string(entry.Actor.Kind), ID: entry.Actor.ID}, Action: string(entry.Action),
			Target:  auditTargetResponse{Kind: string(entry.Target.Kind), ID: entry.Target.ID},
			Outcome: string(entry.Outcome), RequestID: entry.RequestID, SummaryCode: string(entry.ReasonCode),
		}
	}
	writeJSON(w, http.StatusOK, auditPageResponse{Items: items, Page: pageJSON(page.Page)})
}
