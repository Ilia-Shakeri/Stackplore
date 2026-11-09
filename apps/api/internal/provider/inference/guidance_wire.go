package inference

import (
	"encoding/json"
	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
	"unicode/utf8"
)

// Guidance is separate untrusted data, never an observation or system instruction.
// Exact revision authority remains in the owning server proof and citation map.
type countedGuidance struct {
	ID        string          `json:"id"`
	Title     string          `json:"title,omitempty"`
	Excerpt   string          `json:"excerpt,omitempty"`
	Truncated bool            `json:"truncated"`
	Text      json.RawMessage `json:"text,omitempty"`
}

func partitionGuidance(items []port.Evidence, wire []countedEvidence) ([]countedEvidence, []countedGuidance, error) {
	if len(items) != len(wire) {
		return nil, nil, port.ErrInvalid
	}
	observations := make([]countedEvidence, 0, len(items))
	if items == nil {
		observations = nil
	}
	var guides []countedGuidance
	for index, item := range items {
		if !item.Guidance {
			observations = append(observations, wire[index])
			continue
		}
		guide := countedGuidance{ID: wire[index].ID, Text: wire[index].Text}
		// Flatten only the canonical owning projection; unfamiliar data stays opaque.
		if _, _, valid := splitEnvelope(item); valid {
			var document struct {
				Records []struct {
					CitationID string `json:"citation_id"`
					Kind       string `json:"kind"`
					Guidance   *struct {
						Title     string `json:"title"`
						Excerpt   string `json:"excerpt"`
						Truncated *bool  `json:"truncated"`
						Usage     string `json:"usage"`
					} `json:"guidance"`
				} `json:"records"`
			}
			if json.Unmarshal([]byte(item.Text), &document) != nil || len(document.Records) != 1 {
				return nil, nil, port.ErrInvalid
			}
			record := document.Records[0]
			content := record.Guidance
			if record.Kind != "document" || record.CitationID != item.ID || content == nil || content.Truncated == nil || content.Usage != "reviewed_guidance_not_live_facts" || len(content.Title) < 1 || len(content.Title) > 256 || len(content.Excerpt) < 1 || len(content.Excerpt) > 256 || !utf8.ValidString(content.Title) || !utf8.ValidString(content.Excerpt) {
				return nil, nil, port.ErrInvalid
			}
			guide.Title, guide.Excerpt, guide.Truncated, guide.Text = content.Title, content.Excerpt, *content.Truncated, nil
		}
		guides = append(guides, guide)
	}
	return observations, guides, nil
}
