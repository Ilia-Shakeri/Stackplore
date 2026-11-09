package inference

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestExactGuideQuoteCannotBorrowAnObservationCitation(t *testing.T) {
	const quote = "Inspect current service health."
	for _, correct := range []bool{false, true} {
		gateway := fixtureGateway(t, func(backend *fixtureBackend) {
			backend.generate = func(context.Context, CountedInput, int) (Generated, error) {
				id := "observation"
				if correct {
					id = "guide"
				}
				value := fixtureGenerated()
				value.Content = []byte(`{"sections":[{"kind":"next_checks","text":"` + quote + `","citations":["` + id + `"]}]}`)
				return value, nil
			}
		})
		request := fixtureRequest()
		request.Evidence = []Evidence{
			{ID: "observation", Text: "Current resource observation"},
			{ID: "guide", Guidance: true, Text: `{"records":[{"citation_id":"guide","kind":"document","guidance":{"excerpt":"` + quote + `","usage":"reviewed_guidance_not_live_facts"}}]}`},
		}
		prepared, err := gateway.Prepare(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(fmt.Sprintf("%#v", prepared), quote) {
			t.Fatal("private quote escaped preparation formatting")
		}
		_, err = gateway.Dispatch(context.Background(), prepared)
		if correct && err != nil || !correct && err != ErrInvalidOutput {
			t.Fatal("literal guide quote attribution mismatch", err)
		}
	}
}

func TestGuidanceCannotBecomeAReportedFactOrBeDowngradedByBackend(t *testing.T) {
	for _, kind := range []string{"facts", "hypotheses", "next_checks"} {
		gateway := fixtureGateway(t, func(backend *fixtureBackend) {
			original := backend.count
			backend.count = func(ctx context.Context, request Request) (CountedInput, error) {
				request.Evidence[0].Guidance = false
				return original(ctx, request)
			}
			backend.generate = func(_ context.Context, input CountedInput, _ int) (Generated, error) {
				if len(input.GuidanceIDs) != 1 || input.GuidanceIDs[0] != fixtureRequest().Evidence[0].ID {
					t.Fatal("backend rewrote guidance classification")
				}
				value := fixtureGenerated()
				value.Content = []byte(strings.Replace(string(value.Content), `"kind":"facts"`, `"kind":"`+kind+`"`, 1))
				return value, nil
			}
		})
		request := fixtureRequest()
		request.Evidence[0].Guidance = true
		prepared, err := gateway.Prepare(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		request.Evidence[0].Guidance = false
		_, err = gateway.Dispatch(context.Background(), prepared)
		if kind == "facts" && err != ErrInvalidOutput {
			t.Fatal("guide cited as observed fact", err)
		}
		if kind != "facts" && err != nil {
			t.Fatal("qualified guide use rejected", err)
		}
	}
}
