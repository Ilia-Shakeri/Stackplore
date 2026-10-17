package observation

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type syntheticClockProvider struct {
	LiveProvider
	at time.Time
}

func (provider syntheticClockProvider) ObservationTime() time.Time { return provider.at }

func TestWorkerRefusesUntrustedOrInvalidSyntheticObservationClocks(t *testing.T) {
	now := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	fixture := demo.MustNew()
	for _, testcase := range []struct {
		name          string
		provider      LiveProvider
		resolverError error
	}{
		{"missing_clock", struct{ LiveProvider }{fixture}, nil},
		{"zero", syntheticClockProvider{fixture, time.Time{}}, nil},
		{"future", syntheticClockProvider{fixture, now.Add(time.Microsecond)}, nil},
		{"submicrosecond", syntheticClockProvider{fixture, demo.ObservationTime().Add(time.Nanosecond)}, nil},
		{"not_utc", syntheticClockProvider{fixture, demo.ObservationTime().In(time.FixedZone("offset", 3600))}, nil},
		{"pre_epoch", syntheticClockProvider{fixture, time.Unix(-1, 0).UTC()}, nil},
		{"unresolved", nil, ErrProfileUnavailable},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			store := &captureStore{configurations: []domain.SourceConfiguration{testConfiguration(now)}}
			reported := 0
			worker := testWorker(t, store, ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (LiveProvider, error) {
				return testcase.provider, testcase.resolverError
			}), func() time.Time { return now }, func(options *WorkerOptions) { options.OnError = func(error) { reported++ } })
			if err := worker.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if store.generation != 0 || len(store.batch) != 0 || len(store.completions) != 0 || reported != 1 {
				t.Fatal("invalid synthetic clock opened a capture or hid failure")
			}
		})
	}
}
