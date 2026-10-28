// Package inference owns bounded, provider-neutral text generation contracts.
package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaximumInputBytes      = 64 * 1024
	MaximumInputTokens     = 16_000
	MaximumOutputBytes     = 64 * 1024
	MaximumOutputTokens    = 4096
	MaximumEvidence        = 64
	MaximumReferenceLength = 128
	MaximumSections        = 12
	MaximumCallDuration    = 25 * time.Second
	PreparationLifetime    = time.Minute
)

var (
	ErrInvalid       = errors.New("inference.invalid")
	ErrInputLimit    = errors.New("inference.input_limit")
	ErrOutputLimit   = errors.New("inference.output_limit")
	ErrUnavailable   = errors.New("inference.unavailable")
	ErrCancelled     = errors.New("inference.cancelled")
	ErrUncertain     = errors.New("inference.uncertain")
	ErrPrepared      = errors.New("inference.preparation_invalid")
	ErrIdentity      = errors.New("inference.identity_changed")
	ErrInvalidOutput = errors.New("inference.output_invalid")
)

type Evidence struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Request contains volatile input supplied only after the shared privacy policy.
type Request struct {
	Locale      string
	Question    string
	Evidence    []Evidence
	OutputLimit int
}

func (request Request) Validate() error {
	if request.Locale != "en" && request.Locale != "fa" || !validText(request.Question, 8192) || request.OutputLimit < 1 || request.OutputLimit > MaximumOutputTokens || len(request.Evidence) > MaximumEvidence {
		return ErrInvalid
	}
	bytes := len(request.Question)
	seen := make(map[string]struct{}, len(request.Evidence))
	for _, item := range request.Evidence {
		if !boundedID(item.ID, MaximumReferenceLength) || !validText(item.Text, 8192) {
			return ErrInvalid
		}
		if _, duplicate := seen[item.ID]; duplicate {
			return ErrInvalid
		}
		seen[item.ID] = struct{}{}
		bytes += len(item.ID) + len(item.Text)
	}
	if bytes > MaximumInputBytes {
		return ErrInputLimit
	}
	return nil
}

func (Request) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected inference input]")
}
func (Request) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

// CountedInput is volatile and must contain exact token IDs, including framing.
type CountedInput struct {
	Tokens   []int32
	Bytes    int
	Revision string
}

func (CountedInput) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected counted input]")
}
func (CountedInput) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type Generated struct {
	Content      []byte
	InputTokens  int
	OutputTokens int
	Complete     bool
	Revision     string
}

func (Generated) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected generated content]")
}
func (Generated) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

// Backend separates non-generating count work from one explicit dispatch.
// Every operation must respect its context. Generate must never retry dispatch.
type Backend interface {
	Count(context.Context, Request) (CountedInput, error)
	Generate(context.Context, CountedInput, int) (Generated, error)
}

type SectionKind string

const (
	SectionFacts      SectionKind = "facts"
	SectionHypotheses SectionKind = "hypotheses"
	SectionGaps       SectionKind = "gaps"
	SectionNextChecks SectionKind = "next_checks"
)

type Section struct {
	Kind      SectionKind `json:"kind"`
	Text      string      `json:"text"`
	Citations []string    `json:"citations"`
}

type Answer struct {
	Sections []Section `json:"sections"`
}

type Result struct {
	Answer       Answer
	InputTokens  int
	OutputTokens int
	Revision     string
}

func (Result) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected inference result]")
}
func (Result) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func parseAnswer(contents []byte, evidence map[string]struct{}) (Answer, error) {
	if len(contents) == 0 || len(contents) > MaximumOutputBytes || !utf8.Valid(contents) {
		return Answer{}, ErrOutputLimit
	}
	var answer Answer
	if !UniqueJSON(contents) {
		return Answer{}, ErrInvalidOutput
	}
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&answer) != nil || decoder.Decode(new(any)) != io.EOF || len(answer.Sections) < 1 || len(answer.Sections) > MaximumSections {
		return Answer{}, ErrInvalidOutput
	}
	if err := ValidateAnswer(answer, evidence); err != nil {
		return Answer{}, err
	}
	return answer, nil
}

// ValidateAnswer checks decoded sections against the exact allowed references.
// The caller separately bounds serialized bytes before any publication.
func ValidateAnswer(answer Answer, evidence map[string]struct{}) error {
	if len(answer.Sections) < 1 || len(answer.Sections) > MaximumSections {
		return ErrInvalidOutput
	}
	for _, section := range answer.Sections {
		if section.Kind != SectionFacts && section.Kind != SectionHypotheses && section.Kind != SectionGaps && section.Kind != SectionNextChecks || !validText(section.Text, 4096) || section.Citations == nil || len(section.Citations) > MaximumEvidence || section.Kind == SectionFacts && len(section.Citations) == 0 {
			return ErrInvalidOutput
		}
		if strings.ContainsAny(section.Text, "<>\x00") || strings.Contains(section.Text, "](") || strings.Contains(section.Text, "http://") || strings.Contains(section.Text, "https://") {
			return ErrInvalidOutput
		}
		seen := map[string]bool{}
		for _, citation := range section.Citations {
			if _, exists := evidence[citation]; !exists || !boundedID(citation, MaximumReferenceLength) || seen[citation] {
				return ErrInvalidOutput
			}
			seen[citation] = true
		}
	}
	return nil
}

// UniqueJSON checks scalar Unicode, bounded nesting and case-folded unique object keys.
// Case folding also rejects aliases that encoding/json would decode to one field.
func UniqueJSON(contents []byte) bool {
	if !utf8.Valid(contents) || !scalarJSONStringEncoding(contents) {
		return false
	}
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.UseNumber()
	var value func(int) bool
	value = func(depth int) bool {
		if depth > 12 {
			return false
		}
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		delimiter, nested := token.(json.Delim)
		if !nested {
			return true
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				name, ok := key.(string)
				name = strings.Map(foldJSONRune, name)
				if err != nil || !ok || seen[name] {
					return false
				}
				seen[name] = true
				if !value(depth + 1) {
					return false
				}
			}
		case '[':
			for decoder.More() {
				if !value(depth + 1) {
					return false
				}
			}
		default:
			return false
		}
		end, err := decoder.Token()
		return err == nil && (delimiter == '{' && end == json.Delim('}') || delimiter == '[' && end == json.Delim(']'))
	}
	if !value(0) {
		return false
	}
	_, err := decoder.Token()
	return err == io.EOF
}

func foldJSONRune(value rune) rune {
	for {
		next := unicode.SimpleFold(value)
		if next <= value {
			return next
		}
		value = next
	}
}

func validText(value string, maximum int) bool {
	return len(value) >= 1 && len(value) <= maximum && utf8.ValidString(value) && strings.TrimSpace(value) != "" && !strings.ContainsRune(value, 0)
}

func validID(value string) bool {
	return boundedID(value, 64)
}

func boundedID(value string, maximum int) bool {
	if len(value) < 1 || len(value) > maximum {
		return false
	}
	for _, char := range value {
		if char < 'a' || char > 'z' {
			if char < 'A' || char > 'Z' {
				if char < '0' || char > '9' {
					if char != '_' && char != '-' {
						return false
					}
				}
			}
		}
	}
	return true
}

// AnswerSchema returns a fresh closed schema; callers cannot change the next one.
func AnswerSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false,"required":["sections"],"properties":{"sections":{"type":"array","minItems":1,"maxItems":12,"items":{"type":"object","additionalProperties":false,"required":["kind","text","citations"],"properties":{"kind":{"type":"string","enum":["facts","hypotheses","gaps","next_checks"]},"text":{"type":"string","minLength":1,"maxLength":4096},"citations":{"type":"array","maxItems":64,"uniqueItems":true,"items":{"type":"string","minLength":1,"maxLength":128}}}}}}}`)
}
