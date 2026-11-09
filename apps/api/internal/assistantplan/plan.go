// Package assistantplan owns finite read-only investigation recipes.
package assistantplan

import "github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"

const (
	MaximumReads   = 8
	RecordsPerRead = 8
	BytesPerRead   = 8 * 1024
)

// Reads returns a fresh server-owned plan. No model text or browser tool list
// enters this boundary; every dispatch still needs current authority and budget.
func Reads(goal assistant.Goal, logConsent bool) ([]assistant.StepKind, error) {
	switch goal {
	case assistant.ExplainResource:
		if logConsent {
			return nil, assistant.ErrInvalid
		}
		return []assistant.StepKind{assistant.Inventory}, nil
	case assistant.InvestigateIncident:
		plan := []assistant.StepKind{assistant.Inventory, assistant.Health, assistant.Metrics, assistant.Events, assistant.Topology, assistant.AlertHistory, assistant.WorkloadHistory}
		if logConsent {
			plan = append(plan, assistant.Logs)
		}
		return plan, nil
	default:
		return nil, assistant.ErrInvalid
	}
}

// WithDocument adds only an explicitly selected local guidance read. Log and
// document consent cannot displace a required family or raise the fixed cap.
func WithDocument(goal assistant.Goal, logConsent, selected bool) ([]assistant.StepKind, error) {
	plan, err := Reads(goal, logConsent)
	if err != nil {
		return nil, err
	}
	if !selected {
		return plan, nil
	}
	if logConsent || len(plan) >= MaximumReads {
		return nil, assistant.ErrInvalid
	}
	return append(plan, assistant.ReviewedDocument), nil
}
