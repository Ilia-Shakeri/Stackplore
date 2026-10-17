package observation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestProfileFailureStaysSeparateFromConnectionOutage(t *testing.T) {
	at := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		err    error
		reason domain.ReasonCode
	}{
		{ErrProfileUnavailable, "observation.profile_unavailable"},
		{errors.Join(ErrUnavailable, ErrProfileUnavailable), "observation.profile_unavailable"},
		{errors.New("bounded provider read failed"), "observation.provider_unavailable"},
		{context.DeadlineExceeded, "observation.deadline_exceeded"},
	}
	for _, test := range tests {
		status := errorStatus(test.err, at)
		if status.Validate() != nil || status.State != domain.AvailabilityUnavailable || status.ReasonCode != test.reason {
			t.Fatalf("fixed collection reason = %#v; want %s", status, test.reason)
		}
	}
}

func TestWorkerPersistsProfileFailureWithoutCallingItConnectionLoss(t *testing.T) {
	at := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	configuration := testConfiguration(at)
	configuration.Source.Provider, configuration.Source.Synthetic = domain.ProviderDocker, false
	configuration.ProfileID = domain.SourceProfileDockerSocketDevelopment
	store := &captureStore{configurations: []domain.SourceConfiguration{configuration}}
	worker := testWorker(t, store, ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (LiveProvider, error) {
		return nil, ErrProfileUnavailable
	}), func() time.Time { return at }, nil)
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.completions) != 1 || store.completions[0].Status.ReasonCode != "observation.profile_unavailable" ||
		len(store.batch) != 0 || store.retired != 0 {
		t.Fatalf("profile failure became wrong durable evidence: completions = %#v", store.completions)
	}
}
