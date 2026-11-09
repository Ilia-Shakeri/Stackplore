package inference

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

func TestGuidanceHandlesAreDistinctAndBoundToTheirOriginalReferences(t *testing.T) {
	ids := []string{"canonical_observation", "canonical_guide"}
	aliases, bindings, err := compactEvidenceReferences(ids, []string{ids[1]})
	if err != nil || len(aliases) != 2 || aliases[0] != "e1" || aliases[1] != "g1" || bindings["g1"] != ids[1] || bindings["e1"] != ids[0] {
		t.Fatal("guidance handle classification lost", err)
	}
	encoded, err := canonicalAnswer([]byte(`{"sections":[{"kind":"next_checks","text":"Inspect current health.","citations":["g1"]}]}`), bindings)
	if err != nil || !strings.Contains(string(encoded), ids[1]) || strings.Contains(string(encoded), ids[0]) {
		t.Fatal("guide handle rebound to observation", err)
	}
	for _, guides := range [][]string{{"unknown"}, {ids[1], ids[1]}} {
		if _, _, err := compactEvidenceReferences(ids, guides); err != port.ErrInvalid {
			t.Fatal("unbound guidance classification accepted", err)
		}
	}
}

func TestCompactCitationHandlesRestoreOnlyTheOriginalOrderedReferences(t *testing.T) {
	t.Parallel()
	references := []string{"ev_" + strings.Repeat("a", 64), "ev_" + strings.Repeat("b", 64)}
	aliases, bindings, err := compactReferences(references)
	if err != nil || len(aliases) != 2 || aliases[0] != "e1" || aliases[1] != "e2" {
		t.Fatal("compact handles escaped their fixed table")
	}
	encoded, err := canonicalAnswer([]byte(`{"sections":[{"kind":"facts","text":"Observed fact.","citations":["e2"]}]}`), bindings)
	var answer port.Answer
	if err != nil || json.Unmarshal(encoded, &answer) != nil || answer.Sections[0].Citations[0] != references[1] {
		t.Fatal("handle did not restore the actual reference")
	}
	for _, invalid := range []string{
		`{"sections":[{"kind":"facts","text":"Observed fact.","citations":["e3"]}]}`,
		`{"sections":[{"kind":"facts","text":"Observed fact.","citations":["e1","e1"]}]}`,
		`{"sections":[{"kind":"facts","text":"Observed fact.","citations":[]}]}`,
		`{"sections":[{"kind":"facts","text":"https://unapproved.invalid","citations":["e1"]}]}`,
		`{"sections":[{"kind":"facts","text":"Observed fact.","citations":["` + references[0] + `"]}]}`,
		`{"sections":[{"kind":"facts","text":"Observed fact.","citations":["e1"],"action":"shell"}]}`,
		`{"sections":[],"sections":[{"kind":"gaps","text":"Unknown.","citations":[]}]}`,
	} {
		if _, err := canonicalAnswer([]byte(invalid), bindings); err == nil {
			t.Fatal("unbound or malformed compact output escaped validation")
		}
	}
	for _, invalid := range [][]string{{references[0], references[0]}, {"../unapproved"}, make([]string, 65)} {
		if _, _, err := compactReferences(invalid); err == nil {
			t.Fatal("invalid reference set gained compact authority")
		}
	}
	_, other, _ := compactReferences([]string{"other_run_reference"})
	if bindings["e1"] != references[0] || other["e1"] == bindings["e1"] {
		t.Fatal("handle table leaked across independent requests")
	}
}

func TestLocalCompactWirePreservesCanonicalGatewayAuthorityAndExactUsage(t *testing.T) {
	t.Parallel()
	first, second := "ev_"+strings.Repeat("a", 64), "ev_"+strings.Repeat("b", 64)
	fixture := &protocolFixture{}
	fixture.mutate = func(response http.ResponseWriter, request *http.Request) bool {
		if request.URL.Path == "/apply-template" {
			var body struct {
				Messages []message `json:"messages"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || len(body.Messages) != 2 {
				t.Error("invalid counted framing")
				response.WriteHeader(400)
				return true
			}
			var user struct {
				Evidence []port.Evidence `json:"evidence"`
			}
			if json.Unmarshal([]byte(body.Messages[1].Content), &user) != nil || len(user.Evidence) != 2 || user.Evidence[0].ID != "e1" || user.Evidence[1].ID != "e2" {
				t.Error("canonical IDs were not replaced in counted outer handles")
			}
			_ = json.NewEncoder(response).Encode(map[string]string{"prompt": "exact formatted fixture prompt"})
			return true
		}
		if request.URL.Path != "/completion" {
			return false
		}
		var payload struct {
			Schema json.RawMessage `json:"json_schema"`
		}
		if json.NewDecoder(request.Body).Decode(&payload) != nil || strings.Contains(string(payload.Schema), first) || !strings.Contains(string(payload.Schema), `"e2"`) {
			t.Error("completion schema escaped compact handles")
		}
		_ = json.NewEncoder(response).Encode(map[string]any{"content": `{"sections":[{"kind":"facts","text":"Observed fact.","citations":["e2"]}]}`, "tokens_predicted": 20, "tokens_evaluated": 3, "truncated": false, "stop_type": "eos", "model": fixtureAlias})
		return true
	}
	local, _ := fixtureLocal(t, fixture, Options{})
	gateway, err := port.New(local, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	request := testRequest()
	request.Evidence = []port.Evidence{{ID: first, Text: "First actual fact."}, {ID: second, Text: "Second actual fact."}}
	prepared, err := gateway.Prepare(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	result, err := gateway.Dispatch(context.Background(), prepared)
	if err != nil || result.Answer.Sections[0].Citations[0] != second || result.InputTokens != 3 || result.OutputTokens != 20 || request.Evidence[0].ID != first {
		t.Fatal("compact transport changed authority, caller data or actual usage", err)
	}
}

func TestCanonicalCitationExpansionRetainsTheOutputByteCeiling(t *testing.T) {
	t.Parallel()
	references := make([]string, 64)
	for index := range references {
		references[index] = strings.Repeat("a", 120) + strconv.Itoa(index)
	}
	aliases, bindings, err := compactReferences(references)
	if err != nil {
		t.Fatal(err)
	}
	answer := port.Answer{}
	for range 12 {
		answer.Sections = append(answer.Sections, port.Section{Kind: port.SectionFacts, Text: strings.Repeat("x", 3000), Citations: aliases})
	}
	raw, err := json.Marshal(answer)
	if err != nil || len(raw) > port.MaximumOutputBytes {
		t.Fatal("compact output budget fixture is invalid")
	}
	if _, err := canonicalAnswer(raw, bindings); err != port.ErrOutputLimit {
		t.Fatal("canonical expansion escaped the original output byte ceiling")
	}
}
