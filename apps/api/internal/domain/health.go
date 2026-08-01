package domain

import (
	"fmt"
	"time"
)

const MaxHealthEvidence = 32

type HealthState string

const (
	HealthHealthy   HealthState = "healthy"
	HealthDegraded  HealthState = "degraded"
	HealthUnhealthy HealthState = "unhealthy"
	HealthUnknown   HealthState = "unknown"
)

func (state HealthState) Validate() error {
	switch state {
	case HealthHealthy, HealthDegraded, HealthUnhealthy, HealthUnknown:
		return nil
	default:
		return fmt.Errorf("unknown health state %q", state)
	}
}

type HealthEvidence struct {
	ID         EvidenceID
	Code       ReasonCode
	SourceCode ReasonCode
	Status     EvidenceStatus
	ObservedAt time.Time
	Synthetic  bool
}

type EvidenceStatus string

const (
	EvidencePositive EvidenceStatus = "positive"
	EvidenceNegative EvidenceStatus = "negative"
	EvidenceUnknown  EvidenceStatus = "unknown"
)

func (status EvidenceStatus) Validate() error {
	switch status {
	case EvidencePositive, EvidenceNegative, EvidenceUnknown:
		return nil
	default:
		return fmt.Errorf("unknown health evidence status %q", status)
	}
}

func (evidence HealthEvidence) Validate() error {
	if err := evidence.ID.Validate(); err != nil {
		return err
	}
	if err := evidence.Code.Validate(); err != nil {
		return err
	}
	if err := evidence.SourceCode.Validate(); err != nil {
		return err
	}
	if err := evidence.Status.Validate(); err != nil {
		return err
	}
	return validateUTCTime("health evidence time", evidence.ObservedAt)
}

type HealthSnapshot struct {
	ResourceID ResourceID
	State      HealthState
	Status     ObservationStatus
	Evidence   []HealthEvidence
}

func (snapshot HealthSnapshot) Validate() error {
	if err := snapshot.ResourceID.Validate(); err != nil {
		return err
	}
	if err := snapshot.State.Validate(); err != nil {
		return err
	}
	if err := snapshot.Status.Validate(); err != nil {
		return err
	}
	if snapshot.Status.State != AvailabilityAvailable && snapshot.Status.State != AvailabilityStale && snapshot.State != HealthUnknown {
		return fmt.Errorf("unavailable health must be unknown")
	}
	if !snapshot.Status.AllowsData() && len(snapshot.Evidence) != 0 {
		return fmt.Errorf("unavailable health must not contain evidence")
	}
	if len(snapshot.Evidence) > MaxHealthEvidence {
		return fmt.Errorf("health evidence exceeds %d items", MaxHealthEvidence)
	}
	if snapshot.State != HealthUnknown && len(snapshot.Evidence) == 0 {
		return fmt.Errorf("known health requires evidence")
	}
	seen := make(map[EvidenceID]struct{}, len(snapshot.Evidence))
	for _, evidence := range snapshot.Evidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
		if evidence.ObservedAt.After(snapshot.Status.ObservedAt) {
			return fmt.Errorf("health evidence cannot be newer than its snapshot")
		}
		if _, exists := seen[evidence.ID]; exists {
			return fmt.Errorf("duplicate health evidence ID %q", evidence.ID)
		}
		seen[evidence.ID] = struct{}{}
	}
	return nil
}
