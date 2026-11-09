// Package assistantconversation adapts explicit resource questions to trusted runs.
package assistantconversation

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
)

const MaximumRequestBytes = 64 * 1024

var sourceID = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,127}$`)
var conversationID = regexp.MustCompile(`^ac_[0-9a-f]{64}$`)
var noncePattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func ValidNonce(value string) bool { return len(value) == 64 && noncePattern.MatchString(value) }

type Request struct {
	SourceID       domain.SourceID
	ResourceID     domain.ResourceID
	WindowMinutes  int
	Locale         string
	Question       string
	ConversationID string
	Goal           assistant.Goal
	LogConsent     bool
	Document *runbook.Reference
}

func (request Request) EffectiveGoal() assistant.Goal {
	if request.Goal == "" {
		return assistant.ExplainResource
	}
	return request.Goal
}

func (Request) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected conversation request]")
}
func (Request) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (request Request) Valid() bool {
	return sourceID.MatchString(string(request.SourceID)) && len(request.ResourceID) == 46 && request.ResourceID.Validate() == nil &&
		slices.Contains([]int{5, 15, 60, 360, 1440}, request.WindowMinutes) &&
		(inference.Request{Locale: request.Locale, Question: request.Question, OutputLimit: 1}).Validate() == nil &&
		(request.EffectiveGoal() == assistant.ExplainResource || request.EffectiveGoal() == assistant.InvestigateIncident) &&
		(!request.LogConsent || request.EffectiveGoal() == assistant.InvestigateIncident && request.WindowMinutes <= 15) &&
		(request.Document==nil || !request.LogConsent && request.Document.Validate()==nil && request.Document.Locale==request.Locale) &&
		(request.ConversationID == "" || conversationID.MatchString(request.ConversationID))
}

// UnmarshalJSON enforces exact property names and scalar Unicode before decoding.
func (request *Request) UnmarshalJSON(contents []byte) error {
	if request == nil || len(contents) == 0 || len(contents) > MaximumRequestBytes || !inference.UniqueJSON(contents) {
		return assistant.ErrInvalid
	}
	var fields map[string]json.RawMessage
	defer func() {
		for _, field := range fields {
			clear(field)
		}
	}()
	if json.Unmarshal(contents, &fields) != nil || len(fields) < 5 || len(fields) > 9 {
		return assistant.ErrInvalid
	}
	allowed := []string{"source_id", "resource_id", "window_minutes", "locale", "question", "conversation_id", "goal", "log_consent","reviewed_document"}
	for key := range fields {
		if !slices.Contains(allowed, key) {
			return assistant.ErrInvalid
		}
	}
	for _, key := range allowed[:5] {
		if _, ok := fields[key]; !ok {
			return assistant.ErrInvalid
		}
	}
	var decoded Request
	for _, field := range []struct {
		name   string
		target any
	}{
		{"source_id", &decoded.SourceID}, {"resource_id", &decoded.ResourceID}, {"window_minutes", &decoded.WindowMinutes}, {"locale", &decoded.Locale}, {"question", &decoded.Question},
	} {
		if json.Unmarshal(fields[field.name], field.target) != nil {
			return assistant.ErrInvalid
		}
	}
	if raw, exists := fields["conversation_id"]; exists {
		if json.Unmarshal(raw, &decoded.ConversationID) != nil || decoded.ConversationID == "" {
			return assistant.ErrInvalid
		}
	}
	if raw, exists := fields["goal"]; exists {
		if json.Unmarshal(raw, &decoded.Goal) != nil || decoded.Goal == "" {
			return assistant.ErrInvalid
		}
	}
	if raw, exists := fields["log_consent"]; exists {
		var consent *bool
		if json.Unmarshal(raw, &consent) != nil || consent == nil {
			return assistant.ErrInvalid
		}
		decoded.LogConsent = *consent
	}
	if raw,exists:=fields["reviewed_document"];exists {
		var selection map[string]json.RawMessage
		if json.Unmarshal(raw,&selection)!=nil || len(selection)!=2 || selection["consent"]==nil || selection["reference"]==nil {return assistant.ErrInvalid}
		var referenceFields map[string]json.RawMessage
		if json.Unmarshal(selection["reference"],&referenceFields)!=nil || len(referenceFields)!=6 {return assistant.ErrInvalid}
		for _,key:=range []string{"corpus_id","version","digest","document_id","locale","generation"} {if referenceFields[key]==nil {return assistant.ErrInvalid}}
		var wire struct {
			Consent *bool `json:"consent"`
			Reference *struct {
				CorpusID string `json:"corpus_id"`
				Version int64 `json:"version"`
				Digest string `json:"digest"`
				DocumentID string `json:"document_id"`
				Locale string `json:"locale"`
				Generation *int64 `json:"generation"`
			} `json:"reference"`
		}
		decoder:=json.NewDecoder(bytes.NewReader(raw));decoder.DisallowUnknownFields()
		if decoder.Decode(&wire)!=nil || wire.Consent==nil || !*wire.Consent || wire.Reference==nil || wire.Reference.Generation==nil {return assistant.ErrInvalid}
		ref:=wire.Reference
		digest,err:=hex.DecodeString(ref.Digest)
		if err!=nil || len(digest)!=32 || hex.EncodeToString(digest)!=ref.Digest {return assistant.ErrInvalid}
		selected:=runbook.Reference{CorpusID:ref.CorpusID,Version:ref.Version,DocumentID:ref.DocumentID,Locale:ref.Locale,Generation:*ref.Generation}
		copy(selected.Digest[:],digest)
		if selected.Validate()!=nil {return assistant.ErrInvalid}
		decoded.Document=&selected
	}
	if !decoded.Valid() {
		return assistant.ErrInvalid
	}
	*request = decoded
	return nil
}
