package domain

import (
	"fmt"
	"time"
)

type AvailabilityState string

const (
	AvailabilityAvailable        AvailabilityState = "available"
	AvailabilityStale            AvailabilityState = "stale"
	AvailabilityUnavailable      AvailabilityState = "unavailable"
	AvailabilityUnsupported      AvailabilityState = "unsupported"
	AvailabilityPermissionDenied AvailabilityState = "permission_denied"
)

func (state AvailabilityState) Validate() error {
	switch state {
	case AvailabilityAvailable, AvailabilityStale, AvailabilityUnavailable, AvailabilityUnsupported, AvailabilityPermissionDenied:
		return nil
	default:
		return fmt.Errorf("unknown availability state %q", state)
	}
}

type Completeness string

const (
	CompletenessComplete Completeness = "complete"
	CompletenessPartial  Completeness = "partial"
)

func (value Completeness) Validate() error {
	switch value {
	case CompletenessComplete, CompletenessPartial:
		return nil
	default:
		return fmt.Errorf("unknown completeness %q", value)
	}
}

type ReasonCode string

func (code ReasonCode) Validate() error {
	return validateNamespacedCode("reason code", string(code))
}

type ObservationStatus struct {
	State        AvailabilityState
	Completeness Completeness
	ObservedAt   time.Time
	ReasonCode   ReasonCode
}

func (status ObservationStatus) Validate() error {
	if err := status.State.Validate(); err != nil {
		return err
	}
	if err := status.Completeness.Validate(); err != nil {
		return err
	}
	if err := validateUTCTime("observed time", status.ObservedAt); err != nil {
		return err
	}
	if status.Completeness == CompletenessPartial && status.State != AvailabilityAvailable && status.State != AvailabilityStale {
		return fmt.Errorf("partial completeness requires available or stale data")
	}
	reasonRequired := status.State != AvailabilityAvailable || status.Completeness == CompletenessPartial
	if reasonRequired && status.ReasonCode == "" {
		return fmt.Errorf("reason code is required for non-complete availability")
	}
	if status.ReasonCode != "" {
		if err := status.ReasonCode.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (status ObservationStatus) AllowsData() bool {
	return status.State == AvailabilityAvailable || status.State == AvailabilityStale
}
