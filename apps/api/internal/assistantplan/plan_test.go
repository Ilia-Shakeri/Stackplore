package assistantplan

import (
	"slices"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestPlansAreFiniteFreshAndRequireExplicitLogConsent(t *testing.T) {
	for _, consent := range []bool{false, true} {
		plan, err := Reads(assistant.InvestigateIncident, consent)
		if err != nil || len(plan) > MaximumReads || len(plan) < 5 || plan[0] != assistant.Inventory || slices.Contains(plan, assistant.Inference) || slices.Contains(plan, assistant.Logs) != consent {
			t.Fatal("investigation escaped its fixed read recipe")
		}
		seen := map[assistant.StepKind]bool{}
		for _, kind := range plan {
			if !kind.Valid() || seen[kind] {
				t.Fatal("unknown or repeated read")
			}
			seen[kind] = true
		}
		plan[0] = assistant.Logs
		again, _ := Reads(assistant.InvestigateIncident, consent)
		if again[0] != assistant.Inventory {
			t.Fatal("caller changed the next plan")
		}
	}
	plain, err := Reads(assistant.ExplainResource, false)
	if err != nil || !slices.Equal(plain, []assistant.StepKind{assistant.Inventory}) {
		t.Fatal("ordinary explanation acquired extra reads")
	}
	for _, goal := range []assistant.Goal{assistant.ExplainResource, assistant.ExplainAlert, "shell", "sql", "http://unapproved.invalid"} {
		if _, err := Reads(goal, true); err != assistant.ErrInvalid {
			t.Fatal("unapproved goal or log access accepted")
		}
	}
}
