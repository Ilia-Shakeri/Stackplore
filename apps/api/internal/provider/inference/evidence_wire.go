package inference

import (
	"bytes"
	"encoding/json"

	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type countedEvidence struct {
	ID   string          `json:"id"`
	Text json.RawMessage `json:"text"`
	Usage string `json:"usage,omitempty"`
}

// Factor only complete matching envelopes. Every value is retained, including
// each item's quality, record, original citation ID and pagination metadata.
// This is inert data formatting; it grants no authority or new read capability.
func factorEvidence(items []port.Evidence, aliases []string) ([]countedEvidence, json.RawMessage, error) {
	if len(items) != len(aliases) {
		return nil, nil, port.ErrInvalid
	}
	var scope json.RawMessage
	documents := make([]map[string]json.RawMessage, len(items))
	group := len(items) > 0
	for index, item := range items {
		document, candidate, valid := splitEnvelope(item)
		if !valid || index > 0 && !bytes.Equal(candidate, scope) {
			group = false
			break
		}
		documents[index] = document
		if index == 0 {
			scope = candidate
		}
	}
	result := make([]countedEvidence, len(items))
	if items == nil {
		result = nil
	}
	for index, item := range items {
		var contents []byte
		var err error
		if group {
			contents, err = json.Marshal(documents[index])
		} else {
			contents, err = json.Marshal(item.Text)
		}
		if err != nil {
			return nil, nil, port.ErrInvalid
		}
		result[index] = countedEvidence{ID: aliases[index], Text: contents}
		if item.Guidance {
			result[index].Usage = "reviewed_guidance"
		}
	}
	if !group {
		scope = nil
	}
	return result, scope, nil
}

func splitEnvelope(item port.Evidence) (map[string]json.RawMessage, json.RawMessage, bool) {
	contents := []byte(item.Text)
	defer clear(contents)
	if !port.UniqueJSON(contents) {
		return nil, nil, false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(contents, &fields) != nil || len(fields) != 12 {
		return nil, nil, false
	}
	for _, key := range []string{"version", "source_reference", "provider", "configuration_version", "policy_version", "restore_generation", "range", "quality", "more_available", "records", "item_index", "item_count"} {
		if _, present := fields[key]; !present {
			return nil, nil, false
		}
	}
	if string(fields["version"]) != "1" {
		return nil, nil, false
	}
	var records []struct {
		CitationID string `json:"citation_id"`
	}
	if json.Unmarshal(fields["records"], &records) != nil || len(records) != 1 || records[0].CitationID != item.ID {
		return nil, nil, false
	}
	header := make(map[string]json.RawMessage, 7)
	for _, key := range []string{"version", "source_reference", "provider", "configuration_version", "policy_version", "restore_generation", "range"} {
		header[key] = fields[key]
		delete(fields, key)
	}
	encoded, err := json.Marshal(header)
	return fields, encoded, err == nil
}
