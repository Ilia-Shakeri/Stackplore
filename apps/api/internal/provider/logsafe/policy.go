package logsafe

import (
	"errors"
	"strings"
)

const (
	MaximumAdditionalFields     = 32
	MaximumAdditionalFieldBytes = 64
)

var ErrInvalidPolicy = errors.New("log redaction fields are invalid")

// Policy is immutable after construction. Its zero value preserves built-in masking.
// Additional fields can only increase masking; built-in credential fields remain protected.
type Policy struct {
	fields map[string]struct{}
}

func NewPolicy(fields []string) (Policy, error) {
	if len(fields) > MaximumAdditionalFields {
		return Policy{}, ErrInvalidPolicy
	}
	if len(fields) == 0 {
		return Policy{}, nil
	}
	result := Policy{fields: make(map[string]struct{}, len(fields))}
	for _, field := range fields {
		if len(field) < 1 || len(field) > MaximumAdditionalFieldBytes {
			return Policy{}, ErrInvalidPolicy
		}
		for index, character := range field {
			letter := character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
			if !letter && (index == 0 || character != '_' && character != '-' && (character < '0' || character > '9')) {
				return Policy{}, ErrInvalidPolicy
			}
		}
		key := normalizedKey(field)
		if _, exists := result.fields[key]; exists {
			return Policy{}, ErrInvalidPolicy
		}
		result.fields[key] = struct{}{}
	}
	return result, nil
}

func (policy Policy) HasAdditionalFields() bool { return len(policy.fields) != 0 }

func (policy Policy) sensitiveKey(value string) bool {
	if sensitiveKey(value) {
		return true
	}
	_, exists := policy.fields[normalizedKey(value)]
	return exists
}

func normalizedKey(value string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(value))
}
