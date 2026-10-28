package assistantconversation

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestRequestDecodeIsClosedBoundedAndProtected(t *testing.T) {
	base := `{"source_id":"source_demo","resource_id":"r1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","window_minutes":15,"locale":"fa","question":"بررسی منبع"}`
	var valid Request
	if json.Unmarshal([]byte(base), &valid) != nil || !valid.Valid() {
		t.Fatal("valid local question rejected")
	}
	if encoded, err := json.Marshal(valid); err != nil || string(encoded) != "{}" || fmt.Sprintf("%#v", valid) != "[protected conversation request]" {
		t.Fatal("generic request serialization disclosed text")
	}
	for name, raw := range map[string]string{
		"unknown":            strings.TrimSuffix(base, "}") + `,"endpoint":"private"}`,
		"alias":              strings.Replace(base, `"source_id"`, `"SOURCE_ID"`, 1),
		"duplicate":          strings.TrimSuffix(base, "}") + `,"question":"changed"}`,
		"null_question":      strings.Replace(base, `"بررسی منبع"`, `null`, 1),
		"blank":              strings.Replace(base, `"بررسی منبع"`, `"\u0085"`, 1),
		"unpaired":           strings.Replace(base, `"بررسی منبع"`, `"\ud800"`, 1),
		"null_conversation":  strings.TrimSuffix(base, "}") + `,"conversation_id":null}`,
		"empty_conversation": strings.TrimSuffix(base, "}") + `,"conversation_id":""}`,
		"named_conversation": strings.TrimSuffix(base, "}") + `,"conversation_id":"caller_name"}`,
		"window":             strings.Replace(base, `"window_minutes":15`, `"window_minutes":16`, 1),
		"fraction":           strings.Replace(base, `"window_minutes":15`, `"window_minutes":15.5`, 1),
		"missing":            strings.Replace(base, `"locale":"fa",`, "", 1),
		"array":              `[]`, "null": `null`, "oversize": strings.Repeat(" ", MaximumRequestBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			var request Request
			if err := request.UnmarshalJSON([]byte(raw)); err != assistant.ErrInvalid || request.Question != "" {
				t.Fatal("malformed private request accepted or retained partial input")
			}
		})
	}
	max := strings.Replace(base, `"بررسی منبع"`, `"`+strings.Repeat("😀", 2048)+`"`, 1)
	if json.Unmarshal([]byte(max), &valid) != nil {
		t.Fatal("exact question byte boundary rejected")
	}
	over := strings.Replace(base, `"بررسی منبع"`, `"`+strings.Repeat("😀", 2049)+`"`, 1)
	if json.Unmarshal([]byte(over), &valid) == nil {
		t.Fatal("question byte overflow accepted")
	}
	escaped := strings.Replace(base, `"بررسی منبع"`, `"`+strings.Repeat(`\u0041`, 8192)+`"`, 1)
	if json.Unmarshal([]byte(escaped), &valid) != nil {
		t.Fatal("valid escaped byte-boundary question rejected")
	}
}
