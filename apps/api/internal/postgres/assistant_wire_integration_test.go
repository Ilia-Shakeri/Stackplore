package postgres

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
	adapter "github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/inference"
)

// This uses the production HTTP adapter with a synthetic protocol server and
// actual SQL reservations. It does not measure an installed engine or quality.
func newAssistantWireFixture(t *testing.T, backend *assistantServiceBackend) inference.Backend {
	t.Helper()
	const alias = "sha256-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const template = "synthetic bounded service template"
	const formatted = "synthetic exact formatted input"
	var mu sync.Mutex
	calls := map[string]int{}
	invalid := false
	decode := func(body []byte, target any) bool {
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		return inference.UniqueJSON(body) && decoder.Decode(target) == nil && decoder.Decode(new(any)) == io.EOF
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls[r.URL.Path]++
		mu.Unlock()
		reject := func() {
			mu.Lock()
			invalid = true
			mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, inference.MaximumInputBytes+1))
		defer clear(body)
		if err != nil || len(body) > inference.MaximumInputBytes || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.URL.RawQuery != "" {
			reject()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models", "/props":
			if r.Method != http.MethodGet || len(body) != 0 {
				reject()
				return
			}
			if r.URL.Path == "/v1/models" {
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": alias}}})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"build_info": "synthetic_sql_wire", "chat_template": template, "total_slots": 1, "default_generation_settings": map[string]int{"n_ctx": 24576}})
			}
		case "/apply-template":
			var request struct {
				Messages []struct{ Role, Content string } `json:"messages"`
			}
			if r.Method != http.MethodPost || backend.reserved(r.Context()) != nil || !decode(body, &request) || len(request.Messages) != 2 || request.Messages[0].Role != "system" || request.Messages[1].Role != "user" {
				reject()
				return
			}
			var input struct {
				Locale   string                      `json:"locale"`
				Question string                      `json:"question"`
				Evidence []struct{ ID, Text string } `json:"evidence"`
			}
			if !decode([]byte(request.Messages[1].Content), &input) || input.Locale != "en" || input.Question != "Explain the selected resource." || len(input.Evidence) != 1 || len(input.Evidence[0].Text) > 8192 {
				reject()
				return
			}
			var document struct {
				Records []json.RawMessage
			}
			// Explicit keys below preserve the closed projection's snake-case wire.
			var fields map[string]json.RawMessage
			if json.Unmarshal([]byte(input.Evidence[0].Text), &fields) != nil || len(fields) != 12 {
				reject()
				return
			}
			allowed := []string{"version", "source_reference", "provider", "configuration_version", "policy_version", "restore_generation", "range", "quality", "more_available", "records", "item_index", "item_count"}
			for _, key := range allowed {
				if _, ok := fields[key]; !ok {
					reject()
					return
				}
			}
			if json.Unmarshal(fields["records"], &document.Records) != nil || len(document.Records) != 1 {
				reject()
				return
			}
			var record struct {
				CitationID string          `json:"citation_id"`
				Kind       string          `json:"kind"`
				ResourceID string          `json:"resource_id"`
				Quality    json.RawMessage `json:"quality"`
				Health     string          `json:"health"`
			}
			if !decode(document.Records[0], &record) || record.CitationID != input.Evidence[0].ID || record.Kind != "resource" || record.Health != "healthy" {
				reject()
				return
			}
			for _, forbidden := range []string{"Fixture service", "native_fixture", "annotations", "labels", "kubeconfig", "endpoint", "href"} {
				if strings.Contains(input.Evidence[0].Text, forbidden) {
					reject()
					return
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"prompt": formatted})
		case "/tokenize":
			var request struct {
				Content      string `json:"content"`
				AddSpecial   bool   `json:"add_special"`
				ParseSpecial bool   `json:"parse_special"`
				WithPieces   bool   `json:"with_pieces"`
			}
			if r.Method != http.MethodPost || backend.reserved(r.Context()) != nil || !decode(body, &request) || request.Content != formatted || !request.AddSpecial || !request.ParseSpecial || request.WithPieces {
				reject()
				return
			}
			_, _ = w.Write([]byte(`{"tokens":[1,2,3]}`))
		case "/completion":
			var request struct {
				Prompt         []int32         `json:"prompt"`
				Schema         json.RawMessage `json:"json_schema"`
				Predict        int             `json:"n_predict"`
				Temperature    float64         `json:"temperature"`
				Stream         bool            `json:"stream"`
				Cache          bool            `json:"cache_prompt"`
				ResponseFields []string        `json:"response_fields"`
			}
			if r.Method != http.MethodPost || backend.reserved(r.Context()) != nil || !decode(body, &request) || len(request.Prompt) != 3 || request.Prompt[0] != 1 || request.Prompt[1] != 2 || request.Prompt[2] != 3 || request.Predict != 128 || request.Stream || request.Cache || request.Temperature != 0 || !json.Valid(request.Schema) || len(request.ResponseFields) != 6 {
				reject()
				return
			}
			backend.mu.Lock()
			ref := backend.ref
			backend.mu.Unlock()
			content, _ := json.Marshal(inference.Answer{Sections: []inference.Section{{Kind: inference.SectionFacts, Text: "Health is healthy.", Citations: []string{ref}}}})
			_ = json.NewEncoder(w).Encode(map[string]any{"content": string(content), "tokens_predicted": 10, "tokens_evaluated": 3, "truncated": false, "stop_type": "eos", "model": alias})
		default:
			reject()
		}
	}))
	t.Cleanup(func() {
		server.Close()
		mu.Lock()
		defer mu.Unlock()
		if invalid || calls["/apply-template"] != 1 || calls["/tokenize"] != 1 || calls["/completion"] != 1 || calls["/v1/models"] != 2 || calls["/props"] != 2 {
			t.Error("bounded synthetic adapter wire inventory failed")
		}
	})
	digest := sha256.Sum256([]byte(template))
	local, err := adapter.NewLocal(adapter.Profile{ID: "local_sql_wire", Origin: server.URL, ApprovedAddresses: []netip.Addr{netip.MustParseAddr("127.0.0.1")}, ExpectedAlias: alias, ExpectedBuild: "synthetic_sql_wire", ExpectedTemplateSHA256: hex.EncodeToString(digest[:]), ExpectedContextTokens: 24576, Generation: 1}, adapter.Options{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal("construct bounded synthetic HTTP adapter")
	}
	return local
}
