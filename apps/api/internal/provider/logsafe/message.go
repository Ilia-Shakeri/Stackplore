// Package logsafe bounds observed text and masks credential fields before transport.
package logsafe

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const mask = "[redacted]"

var fieldAssignment = regexp.MustCompile(`(?i)(^|[^a-z0-9_-])([a-z][a-z0-9_-]{0,127})(?:\\?["'])?\s*[:=]\s*`)
var bearerValue = regexp.MustCompile(`(?i)\b(?:bearer|basic)\s+[^\s,;"'}\[\]]+`)

// Message returns bounded UTF-8 text. Structured fields are masked recursively;
// plain text supports quoted assignments, HTTP credentials, and URL parameters.
func Message(value string, truncated bool) (string, bool, bool) {
	var cleaned strings.Builder
	characters := 0
	for _, character := range value {
		if character == utf8.RuneError {
			character = ' '
		}
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf, unicode.Zl, unicode.Zp) && character != '\u200c' && character != '\u200d' {
			if character == '\t' {
				cleaned.WriteByte(' ')
			}
			continue
		}
		if cleaned.Len()+utf8.RuneLen(character) > domain.MaxLogLineBytes {
			truncated = true
			break
		}
		cleaned.WriteRune(character)
		characters++
		if characters > domain.MaxLogLineLength {
			truncated = true
		}
	}
	message, redacted, structured := redactStructured(cleaned.String())
	if !structured {
		message, redacted = redactText(message)
	}
	message, bounded := bound(message)
	return message, redacted, truncated || bounded
}

func redactStructured(value string) (string, bool, bool) {
	start := strings.IndexAny(value, "{[")
	if start < 0 {
		return value, false, false
	}
	decoder := json.NewDecoder(strings.NewReader(value[start:]))
	decoder.UseNumber()
	var record any
	if decoder.Decode(&record) != nil {
		if strings.Contains(value[start:], `\u`) {
			// Escaped field names in a broken object cannot be checked safely.
			prefix, _ := redactText(value[:start])
			return prefix + mask, true, true
		}
		return value, false, false
	}
	prefix, prefixRedacted := redactText(value[:start])
	suffix, suffixRedacted := redactText(value[start+int(decoder.InputOffset()):])
	fields := fieldAssignment.FindAllStringSubmatchIndex(value[:start], -1)
	if len(fields) > 0 && fields[len(fields)-1][1] == start && sensitiveKey(value[fields[len(fields)-1][4]:fields[len(fields)-1][5]]) {
		// The whole structured value belongs to a credential assignment.
		return prefix, true, true
	}
	budget := 1024
	redacted, safe := redactObject(record, 0, &budget)
	if !safe {
		return prefix + mask, true, true
	}
	if !redacted && !prefixRedacted && !suffixRedacted {
		return value, false, true
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return prefix + mask, true, true
	}
	return prefix + string(encoded) + suffix, true, true
}

func redactObject(value any, depth int, budget *int) (bool, bool) {
	*budget--
	if depth > 32 || *budget < 0 {
		return false, false
	}
	redacted := false
	switch record := value.(type) {
	case map[string]any:
		for key, item := range record {
			if sensitiveKey(key) {
				record[key], redacted = mask, true
				continue
			}
			if text, ok := item.(string); ok {
				safeText, changed := redactText(text)
				record[key] = safeText
				redacted = redacted || changed
				continue
			}
			changed, safe := redactObject(item, depth+1, budget)
			if !safe {
				return false, false
			}
			redacted = redacted || changed
		}
	case []any:
		for index, item := range record {
			if text, ok := item.(string); ok {
				safeText, changed := redactText(text)
				record[index] = safeText
				redacted = redacted || changed
				continue
			}
			changed, safe := redactObject(item, depth+1, budget)
			if !safe {
				return false, false
			}
			redacted = redacted || changed
		}
	}
	return redacted, true
}

func sensitiveKey(value string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(value))
	switch normalized {
	case "authorization", "proxyauthorization", "password", "passwd", "passphrase", "secret", "token", "accesstoken", "refreshtoken", "idtoken", "sessiontoken", "apikey", "xapikey", "privatekey", "clientsecret", "credential", "credentials", "cookie", "setcookie", "sessionid":
		return true
	}
	return strings.HasSuffix(normalized, "password") || strings.HasSuffix(normalized, "secret") ||
		strings.HasSuffix(normalized, "token") || strings.HasSuffix(normalized, "apikey") || strings.HasSuffix(normalized, "privatekey")
}

func redactText(value string) (string, bool) {
	redacted := false
	if start := strings.Index(strings.ToUpper(value), "-----BEGIN "); start >= 0 && strings.Contains(strings.ToUpper(value[start:]), "PRIVATE KEY-----") {
		value = value[:start] + mask
		redacted = true
	}
	var result strings.Builder
	remaining := value
	for {
		field := fieldAssignment.FindStringSubmatchIndex(remaining)
		if field == nil {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:field[1]])
		key := strings.ToLower(remaining[field[4]:field[5]])
		remaining = remaining[field[1]:]
		if !sensitiveKey(key) {
			continue
		}
		if remaining == "" {
			result.WriteString(mask)
			redacted = true
			break
		}
		end := valueEnd(remaining, strings.Contains(key, "authorization"), strings.Contains(key, "cookie"))
		if end == 0 {
			// No value exists at this delimiter. Make progress without changing text.
			result.WriteByte(remaining[0])
			remaining = remaining[1:]
			continue
		}
		if remaining[0] == '"' || remaining[0] == '\'' {
			result.WriteByte(remaining[0])
			result.WriteString(mask)
			result.WriteByte(remaining[0])
		} else {
			result.WriteString(mask)
		}
		remaining = remaining[end:]
		redacted = true
	}
	message := result.String()
	if bearerValue.MatchString(message) {
		message = bearerValue.ReplaceAllString(message, mask)
		redacted = true
	}
	if start := strings.Index(strings.ToUpper(message), "-----BEGIN "); start >= 0 && strings.Contains(strings.ToUpper(message[start:]), "PRIVATE KEY-----") {
		message = message[:start] + mask
		redacted = true
	}
	return message, redacted
}

func valueEnd(value string, authorization, cookie bool) int {
	if len(value) > 1 && value[0] == '\\' && (value[1] == '"' || value[1] == '\'') {
		// An escaped quoted field may contain nested escapes; mask the tail.
		return len(value)
	}
	if value[0] == '"' || value[0] == '\'' {
		quote := value[0]
		for index := 1; index < len(value); index++ {
			if value[index] == '\\' {
				index++
			} else if value[index] == quote {
				return index + 1
			}
		}
		return len(value)
	}
	if value[0] == '{' || value[0] == '[' {
		// Credential objects and arrays are not safe display data, even if malformed.
		return len(value)
	}
	if cookie {
		return len(value)
	}
	start := 0
	if authorization {
		if scheme, _, found := strings.Cut(value, " "); found && (strings.EqualFold(scheme, "Bearer") || strings.EqualFold(scheme, "Basic")) {
			start = len(scheme) + 1
			for start < len(value) && value[start] == ' ' {
				start++
			}
		}
	}
	for index := start; index < len(value); index++ {
		if strings.ContainsRune(" ,;&}]\"'", rune(value[index])) {
			return index
		}
	}
	return len(value)
}

func bound(value string) (string, bool) {
	characters := 0
	for index, character := range value {
		if index+utf8.RuneLen(character) > domain.MaxLogLineBytes || characters >= domain.MaxLogLineLength {
			return value[:index], true
		}
		characters++
	}
	return value, false
}
