// Package assistantconversation adapts explicit resource questions to trusted runs.
package assistantconversation

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
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
}

func (Request) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected conversation request]")
}
func (Request) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (request Request) Valid() bool {
	return sourceID.MatchString(string(request.SourceID)) && len(request.ResourceID) == 46 && request.ResourceID.Validate() == nil &&
		slices.Contains([]int{5, 15, 60, 360, 1440}, request.WindowMinutes) &&
		(inference.Request{Locale: request.Locale, Question: request.Question, OutputLimit: 1}).Validate() == nil &&
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
	if json.Unmarshal(contents, &fields) != nil || len(fields) < 5 || len(fields) > 6 {
		return assistant.ErrInvalid
	}
	allowed := []string{"source_id", "resource_id", "window_minutes", "locale", "question", "conversation_id"}
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
	if !decoded.Valid() {
		return assistant.ErrInvalid
	}
	*request = decoded
	return nil
}
