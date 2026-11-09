package inference

import (
	"encoding/json"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// ToolOutcome carries closed coverage metadata, never a fabricated evidence ID.
type ToolOutcome struct {
	Tool          string                   `json:"tool"`
	Availability  domain.AvailabilityState `json:"availability"`
	Completeness  domain.Completeness      `json:"completeness"`
	Records       int                      `json:"records"`
	MoreAvailable bool                     `json:"more_available"`
}

// ValidateOutcomes binds declared coverage to the number of actual evidence items.
func ValidateOutcomes(outcomes []ToolOutcome, records int) error {
	if _, err := outcomeBytes(outcomes); err != nil {
		return err
	}
	if len(outcomes) == 0 {
		return nil
	}
	count := 0
	for _, outcome := range outcomes {
		count += outcome.Records
	}
	if count != records {
		return ErrInvalid
	}
	return nil
}

func outcomeBytes(outcomes []ToolOutcome) (int, error) {
	if len(outcomes) > 8 {
		return 0, ErrInvalid
	}
	seen := make(map[string]bool, len(outcomes))
	for _, outcome := range outcomes {
		switch outcome.Tool {
		case "inventory", "health", "metrics", "events", "topology", "logs", "alert_history", "workload_history","reviewed_document":
		default:
			return 0, ErrInvalid
		}
		if seen[outcome.Tool] || outcome.Availability.Validate() != nil || outcome.Availability == domain.AvailabilityPermissionDenied || outcome.Completeness.Validate() != nil || outcome.Records < 0 || outcome.Records > 8 {
			return 0, ErrInvalid
		}
		if outcome.Availability != domain.AvailabilityAvailable && outcome.Availability != domain.AvailabilityStale && outcome.Records != 0 {
			return 0, ErrInvalid
		}
		if outcome.Availability != domain.AvailabilityAvailable && outcome.Availability != domain.AvailabilityStale && outcome.Completeness != domain.CompletenessComplete {
			return 0, ErrInvalid
		}
		seen[outcome.Tool] = true
	}
	if len(outcomes) == 0 {
		return 0, nil
	}
	encoded, err := json.Marshal(outcomes)
	if err != nil {
		return 0, ErrInvalid
	}
	return len(encoded), nil
}
