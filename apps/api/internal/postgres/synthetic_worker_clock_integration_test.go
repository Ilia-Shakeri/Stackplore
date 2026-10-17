package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/observation"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type syntheticClockCaptureStore struct {
	*Store
	checkWrites bool
	checked     int
}

func (store *syntheticClockCaptureStore) SaveCapture(ctx context.Context, batch domain.CaptureBatch) error {
	if store.checkWrites {
		status, err := store.GetObservationStatus(ctx, batch.Token.Source)
		if err != nil {
			return err
		}
		if status.ObservedAt != demo.ObservationTime() || batch.Token.ObservedAt != demo.ObservationTime() || status.ReasonCode != "source.collection_in_progress" {
			return domain.ErrRepositoryInvalidInput
		}
		store.checked++
	}
	return store.Store.SaveCapture(ctx, batch)
}

type syntheticReadFailure struct{ *demo.Provider }

func (syntheticReadFailure) CaptureInventory(context.Context) (domain.InventoryCapture, error) {
	result, _ := domain.NewProviderError("fixture.capture", domain.ProviderErrorUnavailable, false, domain.ErrRepositoryUnavailable)
	return domain.InventoryCapture{}, result
}

func TestSyntheticWorkerKeepsScenarioClockDuringRealWritesAndFailure(t *testing.T) {
	repository, _, _ := openIntegrationStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	now := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	fixture := demo.MustNew()
	configuration := domain.SourceConfiguration{Source: fixture.Source(), ProfileID: domain.SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := repository.EnsureSourceConfigurations(ctx, []domain.SourceConfiguration{configuration}); err != nil {
		t.Fatal(err)
	}
	store := &syntheticClockCaptureStore{Store: repository}
	var live observation.LiveProvider = fixture
	resolves := 0
	worker, err := observation.NewWorker(observation.WorkerOptions{Repository: store, Resolver: observation.ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (observation.LiveProvider, error) {
		resolves++
		return live, nil
	}), Now: func() time.Time { return now }, PollInterval: time.Second, CollectionInterval: time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 20 * time.Second, MaximumBackoff: time.Minute, MaximumSources: 4, MaximumConcurrency: 1, MaximumPages: 10, MaximumItems: 1000, MaximumMetricSamples: 1000, MaximumEvents: 1000, EventLookback: time.Hour, RetirementBatches: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	first, err := repository.GetSourceCollectionStatus(ctx, domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: demo.SourceID})
	if err != nil || first.Status.State != domain.AvailabilityAvailable || first.CollectedAt != now || first.Status.ObservedAt != demo.ObservationTime() {
		t.Fatalf("initial capture lost clock separation: %#v, %v", first, err)
	}
	store.checkWrites = true
	now = now.Add(time.Second)
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	second, err := repository.GetSourceCollectionStatus(ctx, domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: demo.SourceID})
	if err != nil || second.Status.State != domain.AvailabilityAvailable || second.CollectedAt != now || second.Status.ObservedAt != demo.ObservationTime() || store.checked == 0 {
		t.Fatalf("in-progress write used runtime clock: %#v, checks=%d, %v", second, store.checked, err)
	}
	live = syntheticReadFailure{fixture}
	now = now.Add(time.Second)
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	status, err := repository.GetObservationStatus(ctx, domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: demo.SourceID})
	if err != nil || status.State != domain.AvailabilityStale || status.ObservedAt != demo.ObservationTime() {
		t.Fatalf("failed attempt invented synthetic observation: %#v, %v", status, err)
	}
	failed, err := repository.GetSourceCollectionStatus(ctx, domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: demo.SourceID})
	if err != nil || failed.Status.State != domain.AvailabilityUnavailable || failed.CollectedAt != now || failed.Status.ObservedAt != demo.ObservationTime() {
		t.Fatalf("failed receipt lost runtime time: %#v, %v", failed, err)
	}
	if err := worker.RunOnce(ctx); err != nil || resolves != 3 {
		t.Fatal("synthetic observation clock replaced real scheduling/backoff")
	}
}
