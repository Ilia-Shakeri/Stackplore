package domain

import (
	"fmt"
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
