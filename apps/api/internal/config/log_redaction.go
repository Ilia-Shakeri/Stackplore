package config

import (
	"regexp"
	"strings"
)

const logRedactionFieldsKey = prefix + "LOG_REDACT_FIELDS"

var logRedactionField = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

func loadLogRedaction(getter Getter, result *Config) error {
	value, present, err := readValue(getter, logRedactionFieldsKey)
	if err != nil || !present {
		return err
	}
	result.LogRedactionFields = strings.Split(value, ",")
	return validateLogRedaction(result.LogRedactionFields)
}
func validateLogRedaction(fields []string) error {
	if len(fields) > 32 {
		return invalid(logRedactionFieldsKey, "exceeds the field limit")
	}
	seen := map[string]bool{}
	for _, field := range fields {
		normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(field))
		if !logRedactionField.MatchString(field) || seen[normalized] {
			return invalid(logRedactionFieldsKey, "contains invalid or duplicate field names")
		}
		seen[normalized] = true
	}
	return nil
}
