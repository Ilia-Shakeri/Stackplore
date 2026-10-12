package logsafe

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestMessageMasksCredentialFormsAndKeepsUsefulContext(t *testing.T) {
	t.Parallel()
	for _, line := range []string{
		"request Authorization: Bearer synth-private-9 done",
		"request authorization: Basic synth-private-9 done",
		"request bearer synth-private-9 done",
		"request password = synth-private-9 done",
		"request accessToken=synth-private-9 done",
		"request service_api_key=synth-private-9 done",
		"request password='synth-private-9 with spaces' done",
		`request "token":"synth-private-9" done`,
		"request url=https://example.test/?access_token=synth-private-9&ok=1 done",
		`{"request":{"headers":{"AUTHORIZATION":"Bearer synth-private-9"}},"message":"done"}`,
		`{"private_key":{"value":"synth-private-9"},"message":"done"}`,
		`{"pa\u0073sword":"synth-private-9","message":"done"}`,
		`INFO {"refresh_token":"synth-private-9","message":"done"}`,
		`INFO {"access\u005ftoken":"synth-private-9"} done`,
		`["token=synth-private-9","done"]`,
	} {
		t.Run(line, func(t *testing.T) {
			message, redacted, truncated := Message(line, false)
			if !redacted || truncated || strings.Contains(message, "synth-private-9") || !strings.Contains(message, mask) || !strings.Contains(message, "done") {
				t.Fatalf("unsafe or incomplete result: %q, redacted %v, truncated %v", message, redacted, truncated)
			}
		})
	}
}

func TestMessageKeepsStructuredShapeAndNonSecretFields(t *testing.T) {
	t.Parallel()
	message, redacted, truncated := Message(`{"password":"synth-private-9","request_id":"request_one","token_count":4,"details":{"api-key":"synth-private-8"}}`, false)
	var record map[string]json.RawMessage
	if !redacted || truncated || json.Unmarshal([]byte(message), &record) != nil || string(record["request_id"]) != `"request_one"` || string(record["token_count"]) != "4" || strings.Contains(message, "synth-private") {
		t.Fatalf("structured result = %q", message)
	}
	plain := "request completed status=200 token_count=4 duration_ms=34"
	if result, changed, cut := Message(plain, false); result != plain || changed || cut {
		t.Fatalf("ordinary result = %q, %v, %v", result, changed, cut)
	}
}

func TestMessageBoundsUnicodeAndTruncatedCredentialText(t *testing.T) {
	t.Parallel()
	for _, line := range []string{
		strings.Repeat("x", domain.MaxLogLineLength+1),
		strings.Repeat("界", domain.MaxLogLineLength+1),
		"password=" + strings.Repeat("s", domain.MaxLogLineLength+1),
		`{"token":"` + strings.Repeat("s", domain.MaxLogLineLength+1),
	} {
		message, _, truncated := Message(line, false)
		if !truncated || !utf8.ValidString(message) || len(message) > domain.MaxLogLineBytes || utf8.RuneCountInString(message) > domain.MaxLogLineLength {
			t.Fatalf("message bounds missed: bytes %d, runes %d, truncated %v", len(message), utf8.RuneCountInString(message), truncated)
		}
		if strings.HasPrefix(line, "password=") || strings.HasPrefix(line, `{"token"`) {
			if strings.Contains(message, strings.Repeat("s", 8)) {
				t.Fatal("truncated credential remains visible")
			}
		}
	}
	message, _, truncated := Message("done\x00\u202e\t", true)
	if message != "done " || !truncated {
		t.Fatalf("unsafe Unicode not removed: %q", message)
	}
}

func TestMessageMasksCookieAndMalformedCredentialObjects(t *testing.T) {
	t.Parallel()
	for _, line := range []string{
		"Cookie: session=synth-private-9; other=synth-private-8",
		`INFO credentials={"key":"synth-private-9"}`,
		`INFO {"access\u005ftoken":"synth-private-9`,
		`INFO \"token\":\"synth-private-9\" done`,
		"private_key=-----BEGIN PRIVATE KEY-----synth-private-9",
	} {
		message, redacted, _ := Message(line, false)
		if !redacted || strings.Contains(message, "synth-private") {
			t.Fatalf("credential remains visible: %q", message)
		}
	}
}
