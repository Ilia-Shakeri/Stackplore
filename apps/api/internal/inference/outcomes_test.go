package inference

import (
	"encoding/json"
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
	if _, err := outcomeBytes(make([]ToolOutcome, 7)); err == nil {
		t.Fatal("unbounded family metadata accepted")
	}
}
