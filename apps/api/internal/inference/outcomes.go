package inference

import (
	"encoding/json"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// ToolOutcome carries closed coverage metadata, never a fabricated evidence ID.
type ToolOutcome struct {
	Tool string `json:"tool"`
	Availability domain.AvailabilityState `json:"availability"`
	Completeness domain.Completeness `json:"completeness"`
	Records int `json:"records"`
	MoreAvailable bool `json:"more_available"`
}

func outcomeBytes(outcomes []ToolOutcome) (int, error) {
	if len(outcomes) > 6 {
		return 0, ErrInvalid
	}
	seen := make(map[string]bool, len(outcomes))
	for _, outcome := range outcomes {
		switch outcome.Tool {
		case "inventory", "health", "metrics", "events", "topology", "logs":
		default:
			return 0, ErrInvalid
		}
		if seen[outcome.Tool] || outcome.Availability.Validate() != nil || outcome.Completeness.Validate() != nil || outcome.Records < 0 || outcome.Records > 8 {
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
