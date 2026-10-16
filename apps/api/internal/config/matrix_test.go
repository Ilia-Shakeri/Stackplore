package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestMatrixSettingsExactOriginPairedAndRedacted(t *testing.T) {
	t.Parallel()
	base := map[string]string{matrixEnabledKey: "true", matrixReferenceKey: "matrix_primary", matrixOriginKey: "https://matrix.example.test", matrixRoomIDKey: "!fixture:example.test", matrixAccessTokenKey: "synthetic_test_token"}
	settings, err := Load(getter(base))
	if err != nil || !settings.MatrixEnabled || settings.MatrixDestinationReferenceID != "matrix_primary" {
		t.Fatal("valid Matrix fixture config rejected")
	}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		printed := fmt.Sprintf(format, settings)
		if strings.Contains(printed, base[matrixAccessTokenKey]) || strings.Contains(printed, base[matrixOriginKey]) || strings.Contains(printed, base[matrixRoomIDKey]) {
			t.Fatal("formatted Matrix settings disclose target material")
		}
	}
	for name, field := range map[string]string{"path": "https://matrix.example.test/x", "query": "https://matrix.example.test?x=y", "redirect-origin": "http://matrix.example.test", "trailing-slash": "https://matrix.example.test/", "uppercase": "https://Matrix.example.test", "default-port": "https://matrix.example.test:443"} {
		t.Run(name, func(t *testing.T) {
			values := cloneMatrixConfig(base)
			values[matrixOriginKey] = field
			if _, err := Load(getter(values)); err == nil {
				t.Fatal("noncanonical Matrix origin accepted")
			}
		})
	}
	for _, key := range []string{matrixReferenceKey, matrixOriginKey, matrixRoomIDKey, matrixAccessTokenKey} {
		values := cloneMatrixConfig(base)
		delete(values, key)
		if _, err := Load(getter(values)); err == nil {
			t.Fatal("incomplete Matrix settings accepted")
		}
	}
	for _, token := range []string{"", "=", "===", "with space", "line\r\nbreak", "=middle=token", strings.Repeat("x", 4097)} {
		values := cloneMatrixConfig(base)
		values[matrixAccessTokenKey] = token
		if _, err := Load(getter(values)); err == nil {
			t.Fatal("invalid Matrix token accepted")
		}
	}
	for _, room := range []string{"room", "!a/b", "!a?b", "!a#b", "!a%2Fb", "!a\\b", "!a\x00b"} {
		values := cloneMatrixConfig(base)
		values[matrixRoomIDKey] = room
		if _, err := Load(getter(values)); err == nil {
			t.Fatal("invalid Matrix room accepted")
		}
	}
	values := cloneMatrixConfig(base)
	values[matrixEnabledKey] = "false"
	if _, err := Load(getter(values)); err == nil {
		t.Fatal("disabled Matrix stray target accepted")
	}
	values = cloneMatrixConfig(base)
	values[webhookEnabledKey] = "true"
	values[webhookReferenceKey] = "matrix_primary"
	values[webhookURLKey] = "https://hooks.example.test/fixture"
	values[webhookSigningKeyKey] = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE"
	if _, err := Load(getter(values)); err == nil {
		t.Fatal("duplicate delivery references accepted")
	}
}

func cloneMatrixConfig(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func TestWebhookConfigRejectsNoncanonicalDestinationsBeforeStart(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"https://Hooks.example.test/path", "https://hooks.example.test./path", "https://hooks.example.test/path#", "https://hooks.example.test/path?", "https://hooks.example.test:000443/path", "https://hooks.example.test:65536/path", "https://hooks.example.test:0/path", "https://bad_host.example.test/path"} {
		if validWebhookURL(value) {
			t.Fatal("noncanonical webhook config accepted")
		}
	}
	if !validWebhookURL("https://hooks.example.test:8443/path?kind=alerts") {
		t.Fatal("canonical HTTPS destination rejected")
	}
}
