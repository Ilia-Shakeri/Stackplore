package inference

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type seedMeasurement struct {
	CaseID         string `json:"case_id"`
	Locale         string `json:"locale"`
	Pass           int    `json:"pass"`
	Status         string `json:"status"`
	Fault          string `json:"fault"`
	InputTokens    int    `json:"input_tokens"`
	OutputTokens   int    `json:"output_tokens"`
	ElapsedMS      int64  `json:"elapsed_ms"`
	Sections       int    `json:"sections"`
	CitationsValid bool   `json:"citations_valid"`
	GapSection     bool   `json:"gap_section"`
}

// Public synthetic development cases. No output text is retained or scored as human judgment.
func TestPinnedSeedEvaluationIntegration(t *testing.T) {
	if os.Getenv("STACKPLORE_TEST_SEED_EVALUATION") != "true" {
		t.Skip("hosted seed evaluation is not configured")
	}
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Fatal("seed_evaluation.hosted_required")
	}
	local := pinnedIntegrationLocal(t)
	gateway, err := port.New(local, time.Now)
	if err != nil {
		t.Fatal("seed_evaluation.gateway")
	}
	bytes, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "tests", "fixtures", "assistant", "corpus.seed.json"))
	if err != nil || len(bytes) > 4*1024*1024 {
		t.Fatal("seed_evaluation.corpus")
	}
	var corpus struct {
		Scenarios []struct {
			ID        string            `json:"id"`
			Questions map[string]string `json:"questions"`
			Evidence  []struct {
				ID           string          `json:"id"`
				Type         string          `json:"type"`
				ObservedAt   string          `json:"observed_at"`
				Availability string          `json:"availability"`
				Completeness string          `json:"completeness"`
				Synthetic    bool            `json:"synthetic"`
				Facts        json.RawMessage `json:"facts"`
			} `json:"evidence"`
		} `json:"scenarios"`
	}
	if json.Unmarshal(bytes, &corpus) != nil || len(corpus.Scenarios) < 1 || len(corpus.Scenarios) > 200 {
		t.Fatal("seed_evaluation.corpus")
	}
	overall, stop := context.WithTimeout(context.Background(), 18*time.Minute)
	defer stop()
	for pass := 1; pass <= 3; pass++ {
		for _, item := range corpus.Scenarios {
			for _, locale := range []string{"en", "fa"} {
				measured := seedMeasurement{CaseID: item.ID, Locale: locale, Pass: pass, Status: "not_run", Fault: "budget"}
				if overall.Err() == nil {
					started := time.Now()
					ctx, cancel := context.WithTimeout(overall, port.MaximumCallDuration)
					evidence := make([]port.Evidence, 0, len(item.Evidence))
					for _, record := range item.Evidence {
						encoded, encodeErr := json.Marshal(record)
						if encodeErr != nil {
							t.Fatal("seed_evaluation.evidence")
						}
						evidence = append(evidence, port.Evidence{ID: record.ID, Text: string(encoded)})
					}
					measured.Status = "rejected"
					prepared, prepareErr := gateway.Prepare(ctx, port.Request{Locale: locale, Question: item.Questions[locale], Evidence: evidence, OutputLimit: 256})
					measured.Fault = probeFault(prepareErr)
					if prepareErr == nil {
						result, dispatchErr := gateway.Dispatch(ctx, prepared)
						measured.Fault = probeFault(dispatchErr)
						if dispatchErr == nil {
							measured.Status = "completed"
							measured.InputTokens = result.InputTokens
							measured.OutputTokens = result.OutputTokens
							measured.Sections = len(result.Answer.Sections)
							measured.CitationsValid = true
							for _, section := range result.Answer.Sections {
								if section.Kind == port.SectionGaps {
									measured.GapSection = true
								}
							}
						}
					}
					cancel()
					measured.ElapsedMS = time.Since(started).Milliseconds()
				}
				encoded, encodeErr := json.Marshal(measured)
				if encodeErr != nil {
					t.Fatal("seed_evaluation.metadata")
				}
				t.Logf("seed_measurement=%s", encoded)
			}
		}
	}
}
