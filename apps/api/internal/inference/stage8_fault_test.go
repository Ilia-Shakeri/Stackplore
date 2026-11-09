package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Boundary permutations are policy regressions, not independent curated attacks.
func TestStage8MalformedOutputMatrixNeverRetriesOrPublishes(t *testing.T) {
	for _, locale := range []string{"en", "fa"} {
		for _, text := range []string{"Missing.", "شاهد کافی نیست.", strings.Repeat("x", 8192)} {
			for _, extra := range []string{"action", "tool", "endpoint", "command", "token", "html"} {
				for _, placement := range []string{"root", "section"} {
					t.Run(fmt.Sprintf("%s/%d/%s/%s", locale, len(text), extra, placement), func(t *testing.T) {
						section := map[string]any{"kind": "gaps", "text": text, "citations": []string{}}
						output := map[string]any{"sections": []any{section}}
						if placement == "root" {
							output[extra] = "inert synthetic value"
						} else {
							section[extra] = "inert synthetic value"
						}
						encoded, err := json.Marshal(output)
						if err != nil {
							t.Fatal("encode synthetic mutation")
						}
						calls := 0
						gateway := fixtureGateway(t, func(backend *fixtureBackend) {
							backend.generate = func(context.Context, CountedInput, int) (Generated, error) {
								calls++
								result := fixtureGenerated()
								result.Content = encoded
								return result, nil
							}
						})
						request := fixtureRequest()
						request.Locale = locale
						prepared, err := gateway.Prepare(context.Background(), request)
						if err != nil {
							t.Fatal("prepare policy mutation")
						}
						result, err := gateway.Dispatch(context.Background(), prepared)
						if !errors.Is(err, ErrInvalidOutput) || len(result.Answer.Sections) != 0 {
							t.Fatal("malformed output escaped")
						}
						if _, err = gateway.Dispatch(context.Background(), prepared); !errors.Is(err, ErrPrepared) || calls != 1 {
							t.Fatal("failed dispatch retried")
						}
					})
				}
			}
		}
	}
}

func FuzzStage8ClosedAnswerBoundary(f *testing.F) {
	for _, seed := range []string{
		`{"sections":[{"kind":"gaps","text":"Unknown.","citations":[]}]}`,
		`{"sections":[{"kind":"facts","text":"Known.","citations":["e_1"]}]}`,
		`{"sections":[],"sections":[]}`, `{"sections":[{"kind":"exec"}]}`, "\xff", strings.Repeat("[", 100),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > MaximumOutputBytes+1 {
			return
		}
		answer, err := parseAnswer([]byte(input), map[string]struct{}{"e_1": {}})
		if err != nil {
			return
		}
		if len(answer.Sections) < 1 || len(answer.Sections) > MaximumSections {
			t.Fatal("unbounded answer accepted")
		}
		for _, section := range answer.Sections {
			for _, id := range section.Citations {
				if id != "e_1" {
					t.Fatal("unissued citation accepted")
				}
			}
		}
	})
}
