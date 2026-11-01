package inference

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type evidenceProbeMetadata struct {
	Locale          string `json:"locale"`
	BackendFault    string `json:"backend_fault"`
	GatewayFault    string `json:"gateway_fault"`
	InputTokens     int    `json:"input_tokens"`
	OutputTokens    int    `json:"output_tokens"`
	InputMatch      bool   `json:"input_match"`
	Complete        bool   `json:"complete"`
	JSONValid       bool   `json:"json_valid"`
	Sections        int    `json:"sections"`
	CitationsValid  bool   `json:"citations_valid"`
	IdentityMatches bool   `json:"identity_matches"`
}

type evidenceProbeBackend struct {
	*Local
	metadata *evidenceProbeMetadata
	id       string
	identity string
}

func probeFault(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, port.ErrInvalidOutput):
		return "invalid_output"
	case errors.Is(err, port.ErrOutputLimit):
		return "output_limit"
	case errors.Is(err, port.ErrInputLimit):
		return "input_limit"
	case errors.Is(err, port.ErrIdentity):
		return "identity"
	case errors.Is(err, port.ErrCancelled):
		return "cancelled"
	case errors.Is(err, port.ErrUncertain):
		return "uncertain"
	default:
		return "unavailable"
	}
}

// Inspect bounded shape/count flags before the gateway clears volatile output.
// Neither the generated text nor the expected resource identity is logged.
func (probe *evidenceProbeBackend) Generate(ctx context.Context, input port.CountedInput, limit int) (port.Generated, error) {
	generated, err := probe.Local.Generate(ctx, input, limit)
	probe.metadata.BackendFault = probeFault(err)
	probe.metadata.InputTokens = len(input.Tokens)
	probe.metadata.OutputTokens = generated.OutputTokens
	probe.metadata.InputMatch = generated.InputTokens == len(input.Tokens)
	probe.metadata.Complete = generated.Complete
	probe.metadata.JSONValid = json.Valid(generated.Content)
	var answer struct {
		Sections []struct {
			Kind      string   `json:"kind"`
			Text      string   `json:"text"`
			Citations []string `json:"citations"`
		} `json:"sections"`
	}
	if json.Unmarshal(generated.Content, &answer) == nil {
		probe.metadata.Sections = len(answer.Sections)
		if len(answer.Sections) == 1 {
			section := answer.Sections[0]
			probe.metadata.IdentityMatches = section.Kind == "facts" && section.Text == probe.identity
			probe.metadata.CitationsValid = len(section.Citations) == 1 && section.Citations[0] == probe.id
		}
	}
	return generated, err
}

func TestPinnedLocalEvidenceIntegration(t *testing.T) {
	if os.Getenv("STACKPLORE_TEST_LOCAL_INFERENCE") != "true" {
		t.Skip("operator-pinned local inference integration is not configured")
	}
	local := pinnedIntegrationLocal(t)
	for _, locale := range []string{"en", "fa"} {
		t.Run(locale, func(t *testing.T) {
			metadata := evidenceProbeMetadata{Locale: locale, BackendFault: "not_dispatched", GatewayFault: "not_dispatched"}
			t.Cleanup(func() {
				encoded, err := json.Marshal(metadata)
				if err != nil {
					t.Error("encode bounded probe metadata")
					return
				}
				t.Logf("evidence_probe=%s", encoded)
			})
			ctx, cancel := context.WithTimeout(context.Background(), 2*port.MaximumCallDuration)
			defer cancel()
			provider, err := demo.New()
			if err != nil {
				t.Fatal("create synthetic evidence provider")
			}
			capture, err := provider.CaptureInventory(ctx)
			if err != nil || len(capture.Resources) == 0 {
				t.Fatal("capture synthetic inventory")
			}
			resource := capture.Resources[0]
			at := time.Now().UTC().Truncate(time.Microsecond)
			bundle, err := contextbuilder.Build(ctx, contextbuilder.Request{
				Scope:       contextbuilder.Scope{RunID: "evidence_probe", Source: capture.Source, ConfigurationVersion: 1, PolicyVersion: 1, Range: domain.TimeRange{Start: at.Add(-15 * time.Minute), End: at}, ObservedAt: at, Resources: []domain.ResourceRef{resource.Resource.Ref}},
				Destination: contextbuilder.Local, Status: capture.Status, Resources: []domain.ResourceSnapshot{resource}, RecordLimit: 1, ByteLimit: contextbuilder.MaximumBytes,
			})
			if err != nil {
				t.Fatal("project canonical synthetic evidence")
			}
			items, err := bundle.Items()
			if err != nil || len(items) != 1 {
				t.Fatal("materialize one bounded evidence item")
			}
			backend := &evidenceProbeBackend{Local: local, metadata: &metadata, id: items[0].ID, identity: string(resource.Resource.Ref.ID)}
			gateway, err := port.New(backend, time.Now)
			if err != nil {
				t.Fatal("construct real evidence gateway")
			}
			question := "Return exactly one facts section. Its text must be exactly the resource_id in the evidence. Cite its evidence id once. No health, cause or other claims."
			if locale == "fa" {
				question = "فقط یک بخش facts بده. متن آن دقیقاً resource_id در شاهد باشد؛ همان شناسه شاهد را یک بار ارجاع بده. هیچ ادعای سلامت یا علت یا متن دیگر نده."
			}
			prepared, err := gateway.Prepare(ctx, port.Request{Locale: locale, Question: question, Evidence: []port.Evidence{{ID: items[0].ID, Text: items[0].Text}}, OutputLimit: 512})
			if err != nil {
				metadata.GatewayFault = probeFault(err)
				t.Fatal("bounded evidence preparation failed")
			}
			_, err = gateway.Dispatch(ctx, prepared)
			metadata.GatewayFault = probeFault(err)
			if err != nil || !metadata.IdentityMatches || !metadata.CitationsValid {
				t.Fatal("bounded evidence generation failed; inspect metadata flags")
			}
		})
	}
}
