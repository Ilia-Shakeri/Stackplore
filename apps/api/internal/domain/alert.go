package domain

import (
	"fmt"
	"strings"
	"time"
)

type AlertState string

const (
	AlertPending      AlertState = "pending"
	AlertFiring       AlertState = "firing"
	AlertAcknowledged AlertState = "acknowledged"
	AlertResolved     AlertState = "resolved"
)

func (state AlertState) Validate() error {
	switch state {
	case AlertPending, AlertFiring, AlertAcknowledged, AlertResolved:
		return nil
	default:
		return fmt.Errorf("unknown alert state %q", state)
	}
}

type Alert struct {
	ID             AlertID
	RuleID         AlertRuleID
	SourceID       SourceID
	EnvironmentID  *EnvironmentID
	ResourceID     *ResourceID
	State          AlertState
	Severity       Severity
	EvidenceRange  TimeRange
	StartedAt      time.Time
	UpdatedAt      time.Time
	AcknowledgedAt *time.Time
	ResolvedAt     *time.Time
	SummaryCode    ReasonCode
	Synthetic      bool
}

type AlertPage struct {
	Alerts []Alert
	Page   PageInfo
}

type AlertAcknowledgement struct {
	AlertID      AlertID
	UserID       UserID
	TransitionID AlertTransitionID
	OccurredAt   time.Time
	Audit        AuditEntry
}

func (command AlertAcknowledgement) Validate() error {
	if command.AlertID.Validate() != nil || command.UserID.Validate() != nil ||
		command.TransitionID.Validate() != nil || validateUTCTime("alert acknowledgement time", command.OccurredAt) != nil ||
		command.Audit.Validate() != nil {
		return fmt.Errorf("alert acknowledgement is invalid")
	}
	if command.Audit.Actor.Kind != AuditActorUser || command.Audit.Actor.ID != string(command.UserID) ||
		command.Audit.Action != "alert.acknowledge" || command.Audit.Target.Kind != "alert.instance" ||
		command.Audit.Target.ID != string(command.AlertID) || command.Audit.Outcome != AuditSuccess ||
		command.Audit.ReasonCode != "alert.acknowledged" || command.Audit.OccurredAt != command.OccurredAt {
		return fmt.Errorf("alert acknowledgement audit does not match command")
	}
	return nil
}

func (page AlertPage) ValidateFor(sourceID SourceID, request PageRequest) error {
	if err := sourceID.Validate(); err != nil {
		return err
	}
	if err := page.Page.ValidateFor(request); err != nil {
		return err
	}
	if len(page.Alerts) > request.Limit {
		return fmt.Errorf("alert page exceeds requested limit")
	}
	seen := make(map[AlertID]struct{}, len(page.Alerts))
	for index, alert := range page.Alerts {
		if err := alert.Validate(); err != nil {
			return err
		}
		if alert.SourceID != sourceID {
			return fmt.Errorf("alert page contains another source")
		}
		if _, exists := seen[alert.ID]; exists {
			return fmt.Errorf("alert page contains a duplicate alert")
		}
		if index > 0 {
			previous := page.Alerts[index-1]
			if previous.UpdatedAt.Before(alert.UpdatedAt) ||
				(previous.UpdatedAt.Equal(alert.UpdatedAt) && strings.Compare(string(previous.ID), string(alert.ID)) > 0) {
				return fmt.Errorf("alert page must use descending update time and ascending ID order")
			}
		}
		seen[alert.ID] = struct{}{}
	}
	return nil
}

func (alert Alert) Validate() error {
	if err := alert.ID.Validate(); err != nil {
		return err
	}
	if err := alert.RuleID.Validate(); err != nil {
		return err
	}
	if err := alert.SourceID.Validate(); err != nil {
		return err
	}
	if alert.EnvironmentID != nil {
		if err := alert.EnvironmentID.Validate(); err != nil {
			return err
		}
	}
	if alert.ResourceID != nil {
		if err := alert.ResourceID.Validate(); err != nil {
			return err
		}
	}
	if err := alert.State.Validate(); err != nil {
		return err
	}
	if err := alert.Severity.Validate(); err != nil {
		return err
	}
	if err := alert.SummaryCode.Validate(); err != nil {
		return err
	}
	if err := alert.EvidenceRange.Validate(); err != nil {
		return err
	}
	if err := validateUTCTime("alert start time", alert.StartedAt); err != nil {
		return err
	}
	if err := validateUTCTime("alert update time", alert.UpdatedAt); err != nil {
		return err
	}
	if alert.UpdatedAt.Before(alert.StartedAt) {
		return fmt.Errorf("alert update time cannot precede start")
	}
	if alert.AcknowledgedAt != nil {
		if err := validateUTCTime("alert acknowledgement time", *alert.AcknowledgedAt); err != nil {
			return err
		}
		if alert.AcknowledgedAt.Before(alert.StartedAt) {
			return fmt.Errorf("alert acknowledgement cannot precede start")
		}
		if alert.AcknowledgedAt.After(alert.UpdatedAt) {
			return fmt.Errorf("alert acknowledgement cannot follow update time")
		}
	}
	if alert.ResolvedAt != nil {
		if err := validateUTCTime("alert resolution time", *alert.ResolvedAt); err != nil {
			return err
		}
		if alert.ResolvedAt.Before(alert.StartedAt) {
			return fmt.Errorf("alert resolution cannot precede start")
		}
		if alert.ResolvedAt.After(alert.UpdatedAt) {
			return fmt.Errorf("alert resolution cannot follow update time")
		}
		if alert.AcknowledgedAt != nil && alert.AcknowledgedAt.After(*alert.ResolvedAt) {
			return fmt.Errorf("alert acknowledgement cannot follow resolution")
		}
		if alert.EvidenceRange.End.After(*alert.ResolvedAt) {
			return fmt.Errorf("alert evidence cannot follow resolution")
		}
	}
	if alert.EvidenceRange.End.After(alert.UpdatedAt) {
		return fmt.Errorf("alert evidence cannot follow update time")
	}
	if alert.State == AlertAcknowledged && alert.AcknowledgedAt == nil {
		return fmt.Errorf("acknowledged alert requires an acknowledgement time")
	}
	if (alert.State == AlertPending || alert.State == AlertFiring) && alert.AcknowledgedAt != nil {
		return fmt.Errorf("unacknowledged alert state cannot have an acknowledgement time")
	}
	if alert.State == AlertResolved && alert.ResolvedAt == nil {
		return fmt.Errorf("resolved alert requires a resolution time")
	}
	if alert.State != AlertResolved && alert.ResolvedAt != nil {
		return fmt.Errorf("unresolved alert cannot have a resolution time")
	}
	return nil
}
