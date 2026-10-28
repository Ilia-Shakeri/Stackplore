package logsafe

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
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
	fields      map[string]struct{}
	fingerprint [32]byte
}

// Bump this identity whenever built-in masking semantics change.
const policyFingerprintVersion = "stackplore-log-redaction-v1"

func (Policy) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[protected log policy]"))
}

func (Policy) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

// PolicyFingerprint identifies masking semantics without exposing field names.
// The zero policy still represents the valid, immutable built-in policy.
func (policy Policy) PolicyFingerprint() [32]byte {
	if policy.fingerprint == ([32]byte{}) {
		return sha256.Sum256([]byte(policyFingerprintVersion + "\x00"))
	}
	return policy.fingerprint
}

func NewPolicy(fields []string) (Policy, error) {
	if len(fields) > MaximumAdditionalFields {
		return Policy{}, ErrInvalidPolicy
	}
	if len(fields) == 0 {
		return Policy{fingerprint: sha256.Sum256([]byte(policyFingerprintVersion + "\x00"))}, nil
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
	keys := make([]string, 0, len(result.fields))
	for key := range result.fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result.fingerprint = sha256.Sum256([]byte(policyFingerprintVersion + "\x00" + strings.Join(keys, "\x00")))
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
