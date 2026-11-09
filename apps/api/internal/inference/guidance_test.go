package inference

import (
	"context"
	"strings"
	"testing"
)

func TestGuidanceCannotBecomeAReportedFactOrBeDowngradedByBackend(t *testing.T) {
	for _, kind := range []string{"facts", "hypotheses", "next_checks"} {
		gateway := fixtureGateway(t, func(backend *fixtureBackend) {
			original := backend.count
			backend.count = func(ctx context.Context, request Request) (CountedInput, error) {
				request.Evidence[0].Guidance = false
				return original(ctx, request)
			}
			backend.generate = func(context.Context, CountedInput, int) (Generated, error) {
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
