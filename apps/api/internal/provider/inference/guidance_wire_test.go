package inference

import (
	"encoding/json"
	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
	"testing"
)

func TestGuidanceWireSeparatesProseWithoutChangingObservationOrCitationBinding(t *testing.T) {
	observation := envelopeFixture(t, "observation_original", "available", 1)
	guide := envelopeFixture(t, "guide_original", "available", 1)
	var document map[string]json.RawMessage
	if json.Unmarshal([]byte(guide.Text), &document) != nil {
		t.Fatal("decode canonical fixture")
	}
	records, err := json.Marshal([]map[string]any{{"kind": "document", "citation_id": guide.ID, "guidance": map[string]any{"title": "Guide", "excerpt": "Inspect current service health.", "truncated": false, "usage": "reviewed_guidance_not_live_facts"}}})
	if err != nil {
		t.Fatal(err)
	}
	document["records"] = records
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	guide.Text, guide.Guidance = string(raw), true
	items := []port.Evidence{observation, guide}
	wire, scope, err := factorEvidence(items, []string{"e1", "g1"})
	if err != nil || len(scope) == 0 {
		t.Fatal("matching scope not factored", err)
	}
	observed, guides, err := partitionGuidance(items, wire)
	if err != nil || len(observed) != 1 || observed[0].ID != "e1" || len(guides) != 1 || guides[0].ID != "g1" || guides[0].Title != "Guide" || guides[0].Excerpt != "Inspect current service health." || guides[0].Text != nil || items[1].Text != string(raw) {
		t.Fatal("guide mixed with observation or exact binding lost", err)
	}
	if len(wire[1].Text) == 0 {
		t.Fatal("borrowed wire data was cleared")
	}
	if _, _, err := partitionGuidance(items, wire[:1]); err != port.ErrInvalid {
		t.Fatal("unbound guide accepted", err)
	}
}
