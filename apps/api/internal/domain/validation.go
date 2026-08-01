package domain

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maxIDBytes           = 128
	maxCodeBytes         = 96
	maxDisplayNameLength = 256
	maxNativeIDBytes     = 512
)

func validateToken(name, value string, maxBytes int) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if len(value) > maxBytes {
		return fmt.Errorf("%s exceeds %d bytes", name, maxBytes)
	}
	if !isASCIILetter(rune(value[0])) {
		return fmt.Errorf("%s must start with an ASCII letter", name)
	}
	for index, char := range value {
		if isASCIILetter(char) || isASCIIDigit(char) || char == '-' || char == '_' {
			continue
		}
		return fmt.Errorf("%s contains an invalid character at byte %d", name, index)
	}
	return nil
}

func validateNamespacedCode(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if len(value) > maxCodeBytes {
		return fmt.Errorf("%s exceeds %d bytes", name, maxCodeBytes)
	}
	segments := strings.Split(value, ".")
	if len(segments) < 2 {
		return fmt.Errorf("%s must be namespaced", name)
	}
	for _, segment := range segments {
		if segment == "" || !isASCIILower(segment[0]) {
			return fmt.Errorf("%s has an invalid segment", name)
		}
		for index := 1; index < len(segment); index++ {
			char := segment[index]
			if isASCIILower(char) || isASCIIDigit(rune(char)) || char == '-' || char == '_' {
				continue
			}
			return fmt.Errorf("%s has an invalid segment", name)
		}
	}
	return nil
}

func validateDisplayText(name, value string, maxLength int, allowEmpty bool) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s must be valid UTF-8", name)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%s must not have outer whitespace", name)
	}
	if !allowEmpty && value == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if utf8.RuneCountInString(value) > maxLength {
		return fmt.Errorf("%s exceeds %d characters", name, maxLength)
	}
	hasVisibleBase := value == ""
	for _, char := range value {
		if isUnsafeDisplayRune(char) {
			return fmt.Errorf("%s contains an unsafe display character", name)
		}
		if unicode.IsLetter(char) || unicode.IsNumber(char) || unicode.IsPunct(char) || unicode.IsSymbol(char) {
			hasVisibleBase = true
		}
	}
	if !hasVisibleBase {
		return fmt.Errorf("%s must contain a visible base character", name)
	}
	return nil
}

func validateNativeValue(name, value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s must be valid UTF-8", name)
	}
	if value == "" || strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if len(value) > maxNativeIDBytes {
		return fmt.Errorf("%s exceeds %d bytes", name, maxNativeIDBytes)
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return fmt.Errorf("%s contains a control character", name)
		}
	}
	return nil
}

func validateUTCTime(name string, value time.Time) error {
	if value.IsZero() {
		return fmt.Errorf("%s must not be zero", name)
	}
	if value.Year() < 1 || value.Year() > 9999 {
		return fmt.Errorf("%s year must be between 1 and 9999", name)
	}
	_, offset := value.Zone()
	if offset != 0 {
		return fmt.Errorf("%s must use UTC", name)
	}
	return nil
}

func decodeCanonicalRawURLBase64(name string, value string) ([]byte, error) {
	if value == "" {
		return nil, fmt.Errorf("%s must not be empty", name)
	}
	for _, char := range []byte(value) {
		if isASCIILetter(rune(char)) || isASCIIDigit(rune(char)) || char == '-' || char == '_' {
			continue
		}
		return nil, fmt.Errorf("%s must use raw URL-safe base64", name)
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return nil, fmt.Errorf("%s must use canonical raw URL-safe base64", name)
	}
	return decoded, nil
}

func isASCIILetter(char rune) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
}

func isASCIILower(char byte) bool {
	return char >= 'a' && char <= 'z'
}

func isASCIIDigit(char rune) bool {
	return char >= '0' && char <= '9'
}

func isUnsafeDisplayRune(char rune) bool {
	if unicode.IsControl(char) || unicode.In(char, unicode.Zl, unicode.Zp) {
		return true
	}
	return unicode.In(char, unicode.Cf) && char != '\u200c' && char != '\u200d'
}
