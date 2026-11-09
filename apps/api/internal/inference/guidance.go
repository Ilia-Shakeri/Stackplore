package inference

import "encoding/json"

// Recognize only a trusted, classified guide record. Untrusted
// prose cannot promote itself by merely containing similarly named JSON fields.
func guidanceQuote(item Evidence) (string, bool) {
	if !item.Guidance || !UniqueJSON([]byte(item.Text)) {
		return "", false
	}
	var document struct {
		Records []struct {
			ID       string `json:"citation_id"`
			Kind     string `json:"kind"`
			Guidance *struct {
				Excerpt string `json:"excerpt"`
				Usage   string `json:"usage"`
			} `json:"guidance"`
		} `json:"records"`
	}
	if json.Unmarshal([]byte(item.Text), &document) != nil || len(document.Records) != 1 {
		return "", false
	}
	record := document.Records[0]
	if record.ID != item.ID || record.Kind != "document" || record.Guidance == nil || record.Guidance.Usage != "reviewed_guidance_not_live_facts" || !validText(record.Guidance.Excerpt, 256) {
		return "", false
	}
	return record.Guidance.Excerpt, true
}
