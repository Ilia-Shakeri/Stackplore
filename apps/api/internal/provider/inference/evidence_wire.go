package inference

import (
	"bytes"
	"encoding/json"
	"strings"

	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type countedEvidence struct {
	ID    string          `json:"id"`
	Text  json.RawMessage `json:"text"`
	Usage string          `json:"usage,omitempty"`
}

// Factor only complete matching envelopes. Observation citation metadata uses
// its already-bound private handle; facts, quality and pagination stay intact.
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
		if !item.Guidance && !compactRecordCitation(document, item.ID, aliases[index]) {
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

// Change only the literal owning metadata field, never strings inside facts.
// Unfamiliar or ambiguous records retain the entire original opaque envelope.
func compactRecordCitation(document map[string]json.RawMessage, canonical, alias string) bool {
	var records []map[string]json.RawMessage
	if json.Unmarshal(document["records"], &records) != nil || len(records) != 1 || records[0] == nil || alias == "" {
		return false
	}
	for key := range records[0] {
		if key != "citation_id" && strings.EqualFold(key, "citation_id") {
			return false
		}
	}
	var id string
	if json.Unmarshal(records[0]["citation_id"], &id) != nil || id != canonical {
		return false
	}
	encoded, err := json.Marshal(alias)
	if err != nil {
		return false
	}
	records[0]["citation_id"] = encoded
	contents, err := json.Marshal(records)
	if err != nil {
		return false
	}
	document["records"] = contents
	return true
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
