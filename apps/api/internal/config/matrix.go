package config

import "strings"

func loadMatrix(getter Getter, result *Config) error {
	var err error
	if result.MatrixEnabled, err = loadBool(getter, matrixEnabledKey, false); err != nil {
		return err
	}
	fields := []struct {
		key    string
		assign func(string)
	}{
		{matrixReferenceKey, func(value string) { result.MatrixDestinationReferenceID = value }},
		{matrixOriginKey, func(value string) { result.MatrixOrigin = SensitiveString{value: value} }},
		{matrixRoomIDKey, func(value string) { result.MatrixRoomID = SensitiveString{value: value} }},
		{matrixAccessTokenKey, func(value string) { result.MatrixAccessToken = SensitiveString{value: value} }},
	}
	for _, field := range fields {
		value, present, err := readValue(getter, field.key)
		if err != nil {
			return err
		}
		if present {
			field.assign(value)
		}
	}
	return validateMatrix(*result)
}

func validateMatrix(result Config) error {
	if !result.MatrixEnabled {
		if result.MatrixDestinationReferenceID != "" || result.MatrixOrigin.Present() || result.MatrixRoomID.Present() || result.MatrixAccessToken.Present() {
			return invalid(matrixEnabledKey, "must be true when Matrix fields are set")
		}
		return nil
	}
	if !validConfigToken(result.MatrixDestinationReferenceID) {
		return invalid(matrixReferenceKey, "is required and invalid")
	}
	if result.WebhookEnabled && result.MatrixDestinationReferenceID == result.WebhookDestinationReferenceID {
		return invalid(matrixReferenceKey, "must differ from webhook reference")
	}
	origin, err := parsePublicOrigin(result.MatrixOrigin.Reveal())
	if err != nil || origin.Scheme != "https" {
		return invalid(matrixOriginKey, "requires one canonical HTTPS origin")
	}
	room := result.MatrixRoomID.Reveal()
	if len(room) < 2 || len(room) > 255 || room[0] != '!' {
		return invalid(matrixRoomIDKey, "is required and invalid")
	}
	for _, char := range room[1:] {
		if char < 33 || char > 126 || strings.ContainsRune("/\\?#%", char) {
			return invalid(matrixRoomIDKey, "is invalid")
		}
	}
	token := result.MatrixAccessToken.Reveal()
	if len(token) < 1 || len(token) > 4096 {
		return invalid(matrixAccessTokenKey, "is required and invalid")
	}
	padding := false
	characters := 0
	for _, char := range token {
		if char == '=' {
			if characters == 0 {
				return invalid(matrixAccessTokenKey, "is invalid")
			}
			padding = true
			continue
		}
		if padding || !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("-._~+/", char)) {
			return invalid(matrixAccessTokenKey, "is invalid")
		}
		characters++
	}
	return nil
}
