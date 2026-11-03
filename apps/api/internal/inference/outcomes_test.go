package inference

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestOutcomeCoverageIsClosedAndCannotInventRecords(t *testing.T) {
	valid := ToolOutcome{Tool: "inventory", Availability: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, Records: 1}
	request := Request{Locale: "en", Question: "Explain selected facts.", Evidence: []Evidence{{ID: "evidence_one", Text: "bounded fact"}}, Outcomes: []ToolOutcome{valid}, OutputLimit: 128}
	if request.Validate() != nil {
		t.Fatal("bounded actual coverage rejected")
	}
	for _, edit := range []func(*ToolOutcome){
		func(value *ToolOutcome) { value.Tool = "shell" },
		func(value *ToolOutcome) { value.Availability = "healthy" },
		func(value *ToolOutcome) { value.Completeness = "all" },
		func(value *ToolOutcome) { value.Records = -1 },
		func(value *ToolOutcome) { value.Records = 9 },
		func(value *ToolOutcome) { value.Availability = domain.AvailabilityUnsupported },
		func(value *ToolOutcome) { value.Records = 0 },
	} {
		changed := valid
		edit(&changed)
		request.Outcomes = []ToolOutcome{changed}
		if request.Validate() == nil {
			t.Fatal("untrusted or inconsistent coverage accepted")
		}
	}
	request.Outcomes = []ToolOutcome{valid, valid}
	if request.Validate() == nil {
		t.Fatal("duplicate tool coverage accepted")
	}
	gap := ToolOutcome{Tool: "health", Availability: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete}
	request.Outcomes = []ToolOutcome{valid, gap}
	if request.Validate() != nil {
		t.Fatal("honest unsupported coverage rejected")
	}
	encoded, _ := json.Marshal(request)
	if string(encoded) != "{}" {
		t.Fatal("coverage exposed private request serialization")
	}
	gap.Completeness = domain.CompletenessPartial
	if _, err := outcomeBytes([]ToolOutcome{gap}); err == nil {
		t.Fatal("invalid unavailable partial state accepted")
	}
	if _, err := outcomeBytes(make([]ToolOutcome, 9)); err == nil {
		t.Fatal("unbounded family metadata accepted")
	}
}

func TestHistoryCoverageRemainsClosedAndSharesGlobalRecordAccounting(t *testing.T) {
	var outcomes []ToolOutcome
	for _, tool := range []string{"inventory", "health", "metrics", "events", "topology", "logs", "alert_history", "workload_history"} {
		outcomes = append(outcomes, ToolOutcome{Tool: tool, Availability: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, Records: 1})
	}
	if ValidateOutcomes(outcomes, 8) != nil {
		t.Fatal("eight bounded read families rejected")
	}
	if ValidateOutcomes(outcomes, 6) == nil {
		t.Fatal("history records escaped aggregate accounting")
	}
	for _, index := range []int{6, 7} {
		changed := append([]ToolOutcome{}, outcomes...)
		changed[index].Records = 9
		if ValidateOutcomes(changed, 16) == nil {
			t.Fatal("history exceeded its per-read cap")
		}
		changed[index] = outcomes[index]
		changed[index].Tool = "unbounded_history"
		if ValidateOutcomes(changed, 8) == nil {
			t.Fatal("unregistered history family accepted")
		}
	}
}

func TestCoverageBytesParticipateInAggregateInputBudget(t *testing.T) {
	request := Request{Locale: "en", Question: strings.Repeat("q", 8050), OutputLimit: 128}
	for _, id := range []string{"first_fact", "second_fact", "third_fact", "fourth_fact", "fifth_fact", "sixth_fact", "seventh_fact"} {
		request.Evidence = append(request.Evidence, Evidence{ID: id, Text: strings.Repeat("x", 8192)})
	}
	if request.Validate() != nil {
		t.Fatal("bounded evidence-only boundary fixture rejected")
	}
	request.Outcomes = []ToolOutcome{{Tool: "metrics", Availability: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, Records: 7}}
	if request.Validate() != ErrInputLimit {
		t.Fatal("coverage metadata escaped the complete input byte budget")
	}
}
