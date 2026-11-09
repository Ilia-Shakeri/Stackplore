package runbook

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestProvenanceReviewIsExplicitBoundedAndBoundIntoIdentity(t *testing.T) {
	original := fixture()
	before, err := original.Digest()
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Provenance){
		func(p *Provenance) { p.AssistanceReviewed = true },
		func(p *Provenance) { p.Kind = "licensed_copy" },
		func(p *Provenance) { p.Source += " Changed" },
		func(p *Provenance) { p.License += " Changed" },
	} {
		changed := fixture()
		mutate(&changed.Provenance)
		after, err := changed.Digest()
		if err != nil || after == before {
			t.Fatal("source review detached from exact approval")
		}
	}
	for _, mutate := range []func(*Provenance){
		func(p *Provenance) { *p = Provenance{} },
		func(p *Provenance) { p.Kind = "inferred" },
		func(p *Provenance) { p.UseReviewed = false },
		func(p *Provenance) { p.Source = " " },
		func(p *Provenance) { p.Source = strings.Repeat("ظ", 257) },
		func(p *Provenance) { p.Source = "bad\u202e" },
		func(p *Provenance) { p.License = strings.Repeat("x", 129) },
	} {
		changed := fixture()
		mutate(&changed.Provenance)
		if changed.Validate() != ErrInvalid {
			t.Fatal("missing or invalid usage review approved")
		}
	}
	encoded, err := json.Marshal(original.Provenance)
	if err != nil || string(encoded) != "{}" || strings.Contains(fmt.Sprintf("%+v", original.Provenance), "Synthetic") {
		t.Fatal("ordinary formatting leaked attribution")
	}
}

func TestLegacyReadDoesNotInferApprovalRights(t *testing.T) {
	original := fixture()
	legacy := Content{Document: original.Documents[0]}
	if legacy.Validate() != nil || !legacy.Provenance.Empty() {
		t.Fatal("legacy inert read lost")
	}
	original.Provenance = Provenance{}
	if original.Validate() != ErrInvalid {
		t.Fatal("legacy absence became a new approval")
	}
	malformed := legacy
	malformed.Provenance.Source = "partial record"
	if malformed.Validate() != ErrInvalid {
		t.Fatal("partial source record passed as legacy")
	}
}
