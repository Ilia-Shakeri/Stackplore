package logsafe

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestMessageMasksSerializedURLSeparatorsWithoutChangingSafeURLs(t *testing.T) {
	t.Parallel()
	var entropy [24]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		t.Fatal("fixture generation failed")
	}
	marker := hex.EncodeToString(entropy[:])
	clear(entropy[:])
	encode := func(value any) string {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal("structured fixture generation failed")
		}
		return string(encoded)
	}
	for _, layers := range []int{1, 2, 4} {
		separator := strings.Repeat(`\`, layers) + "/"
		prefix := "https:" + separator + separator
		credential := prefix + "fixture_user:" + marker + "@example.test/path"
		safeCredential := prefix + mask + "@example.test/path"
		cut := prefix + "fixture_user:" + marker
		ordinary := prefix + "example.test/path"
		for _, test := range []struct {
			name, input, want string
			truncated         bool
		}{
			{"text", "request " + credential + " done", "request " + safeCredential + " done", false},
			{"outer object string", encode(map[string]string{"message": credential}), encode(map[string]string{"message": safeCredential}), false},
			{"outer array string", encode([]string{credential}), encode([]string{safeCredential}), false},
			{"malformed object", `{"message":"` + credential + `",}`, `{"message":"` + safeCredential + `",}`, false},
			{"member name", encode(map[string]string{credential: "ordinary"}), mask, false},
			{"cut text", "request " + cut, "request " + prefix + mask, true},
			{"cut object string", encode(map[string]string{"message": cut}), encode(map[string]string{"message": prefix + mask}), true},
			{"safe text", "request " + ordinary + " done", "request " + ordinary + " done", false},
			{"safe object string", encode(map[string]string{"message": ordinary}), encode(map[string]string{"message": ordinary}), false},
		} {
			t.Run(strconv.Itoa(layers)+"/"+test.name, func(t *testing.T) {
				message, redacted, truncated := Message(test.input, test.truncated)
				if message != test.want || strings.Contains(message, marker) || redacted != (test.input != test.want) || truncated != test.truncated {
					t.Fatal("serialized URL masking or unchanged-text invariant failed")
				}
			})
		}
	}
}

func TestMessageMasksURLUserinfoAcrossTextAndStructuredValues(t *testing.T) {
	t.Parallel()
	var entropy [24]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		t.Fatal("fixture generation failed")
	}
	marker := hex.EncodeToString(entropy[:])
	clear(entropy[:])
	for _, test := range []struct {
		name, template string
	}{
		{"database", "connect postgres://fixture_user:SECRET@database.example.test:5432/app done"},
		{"web", "request https://fixture_user:SECRET@example.test/path done"},
		{"embedded scheme", "endpoint_https://fixture_user:SECRET@example.test/path done"},
		{"embedded structured scheme", `{"message":"endpoint_https://fixture_user:SECRET@example.test/path done"}`},
		{"apostrophe password", "request https://fixture_user:SECRET'suffix@example.test/path done"},
		{"all subdelimiters", "request https://fixture!$&'()*+,;=:SECRET!$&'()*+,;=@example.test/path done"},
		{"structured apostrophe", `{"message":"request https://fixture_user:SECRET'suffix@example.test/path done"}`},
		{"token user", "request https://SECRET@example.test/path done"},
		{"escaped password", "connect postgresql+driver://fixture_user:%70SECRET@database.example.test/app done"},
		{"multiple delimiters", "connect postgres://fixture_user:SECRET@part@example.test/app done"},
		{"quoted", `request url="https://fixture_user:SECRET@example.test/path" done`},
		{"structured object", `{"message":"connect postgres://fixture_user:SECRET@database.example.test/app done"}`},
		{"structured array", `["https://fixture_user:SECRET@example.test/path", "done"]`},
		{"nested object", `{"details":{"url":"https://fixture_user:SECRET@example.test/path"},"message":"done"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			line := strings.ReplaceAll(test.template, "SECRET", marker)
			message, redacted, truncated := Message(line, false)
			if strings.Contains(message, marker) || !redacted || truncated || !strings.Contains(message, mask+"@") || !strings.Contains(message, "done") {
				t.Fatal("URL credential masking invariant failed")
			}
			if json.Valid([]byte(line)) && !json.Valid([]byte(message)) {
				t.Fatal("structured log shape changed")
			}
		})
	}
}

func TestMessageMasksURLAuthorityWhenCredentialMayBeTruncated(t *testing.T) {
	t.Parallel()
	var entropy [24]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		t.Fatal("fixture generation failed")
	}
	marker := hex.EncodeToString(entropy[:])
	clear(entropy[:])
	for _, test := range []struct {
		name, value string
		truncated   bool
	}{
		{"length bound", "request https://fixture:" + marker + strings.Repeat("x", domain.MaxLogLineBytes) + "@example.test/path", false},
		{"upstream text", "request https://fixture:" + marker, true},
		{"upstream object", `{"message":"https://fixture:` + marker + `"}`, true},
		{"escaped object", `{"message":"https:\/\/fixture:` + marker + `"}`, true},
		{"broken escaped object", `{"message":"https:\/\/fixture:` + marker, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			message, redacted, truncated := Message(test.value, test.truncated)
			if strings.Contains(message, marker) || !redacted || !truncated {
				t.Fatal("truncated credential masking invariant failed")
			}
		})
	}
}

func TestMessageKeepsURLsWithoutUserinfo(t *testing.T) {
	t.Parallel()
	for _, line := range []string{
		"request https://example.test:8443/path?ok=1 done",
		"connect postgres://database.example.test:5432/app done",
		`{"message":"request https://example.test/path done"}`,
		"mail fixture_user@example.test done",
	} {
		message, redacted, truncated := Message(line, false)
		if message != line || redacted || truncated {
			t.Fatal("non-credential URL or ordinary text changed")
		}
	}
}

func TestMessageRejectsCredentialURLsInStructuredMemberNames(t *testing.T) {
	t.Parallel()
	var entropy [24]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		t.Fatal("fixture generation failed")
	}
	marker := hex.EncodeToString(entropy[:])
	clear(entropy[:])
	for _, key := range []string{"https://fixture:" + marker + "@example.test/path", "endpoint_https://fixture:" + marker + "@example.test/path"} {
		line, err := json.Marshal(map[string]string{key: "ordinary value", "message": "done"})
		if err != nil {
			t.Fatal("structured fixture generation failed")
		}
		message, redacted, truncated := Message(string(line), false)
		if strings.Contains(message, marker) || !redacted || truncated {
			t.Fatal("structured member credential masking invariant failed")
		}
	}
}
