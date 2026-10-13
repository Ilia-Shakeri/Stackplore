package observation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type captureStore struct {
	mu             sync.Mutex
	configurations []domain.SourceConfiguration
	batch          []domain.CaptureBatch
	completions    []domain.CaptureCompletion
	generation     int64
	retired        int
	saveErr        error
}

func (store *captureStore) ListSourceConfigurations(_ context.Context, page domain.PageRequest) (domain.SourceConfigurationPage, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return domain.SourceConfigurationPage{Configurations: append([]domain.SourceConfiguration(nil), store.configurations...), Page: domain.PageInfo{Limit: page.Limit}}, nil
}
func (store *captureStore) GetSourceConfiguration(_ context.Context, id domain.SourceID) (domain.SourceConfiguration, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, source := range store.configurations {
		if source.Source.ID == id {
			return source, nil
		}
	}
	return domain.SourceConfiguration{}, domain.ErrRepositoryNotFound
}
func (*captureStore) EnsureSourceConfigurations(context.Context, []domain.SourceConfiguration) error {
	return nil
}
func (store *captureStore) BeginCapture(ctx context.Context, id domain.SourceID, version int64, at time.Time) (domain.CaptureToken, error) {
	configuration, err := store.GetSourceConfiguration(ctx, id)
	if err != nil || !configuration.Enabled || configuration.Version != version {
		return domain.CaptureToken{}, domain.ErrRepositoryConflict
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.generation++
	return domain.CaptureToken{Source: domain.ProviderRef{Provider: configuration.Source.Provider, SourceID: id}, ConfigurationVersion: version, Generation: store.generation, ObservedAt: at}, nil
}
func (store *captureStore) SaveCapture(_ context.Context, batch domain.CaptureBatch) error {
	if batch.Validate() != nil {
		return domain.ErrRepositoryInvalidInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.saveErr != nil {
		return store.saveErr
	}
	store.batch = append(store.batch, batch)
	return nil
}
func (store *captureStore) FinishCapture(_ context.Context, completion domain.CaptureCompletion) error {
	if completion.Validate() != nil {
		return domain.ErrRepositoryInvalidInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.completions = append(store.completions, completion)
	return nil
}
func (store *captureStore) RetireUnseen(_ context.Context, token domain.CaptureToken, limit int) (int64, error) {
	if token.Validate() != nil || limit > domain.MaxRepositoryWriteBatch {
		return 0, domain.ErrRepositoryInvalidInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.retired++
	return 0, nil
}

func testConfiguration(at time.Time) domain.SourceConfiguration {
	return domain.SourceConfiguration{Source: demo.MustNew().Source(), ProfileID: domain.SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: at, UpdatedAt: at}
}

func testWorker(t *testing.T, store domain.CaptureRepository, resolver ProviderResolver, now func() time.Time, change func(*WorkerOptions)) *Worker {
	t.Helper()
	options := WorkerOptions{Repository: store, Resolver: resolver, Now: now, PollInterval: time.Second, CollectionInterval: time.Second,
		ReadTimeout: time.Second, WriteTimeout: time.Second, MaximumBackoff: 4 * time.Second, MaximumSources: 8, MaximumConcurrency: 2,
		MaximumPages: 10, MaximumItems: 1000, MaximumMetricSamples: 1000, MaximumEvents: 1000, EventLookback: 24 * time.Hour, RetirementBatches: 2}
	if change != nil {
		change(&options)
	}
	worker, err := NewWorker(options)
	if err != nil {
		t.Fatal(err)
	}
	return worker
}

func TestWorkerSavesBoundedSyntheticCaptureAndRetiresOnlyCompleteInventory(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for _, bound := range []int{2, 1000} {
		store := &captureStore{configurations: []domain.SourceConfiguration{testConfiguration(now)}}
		provider := demo.MustNew()
		worker := testWorker(t, store, ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (LiveProvider, error) { return provider, nil }), func() time.Time { return now }, func(options *WorkerOptions) { options.MaximumItems = bound })
		if err := worker.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(store.completions) != 1 {
			t.Fatalf("completions=%d", len(store.completions))
		}
		completion := store.completions[0]
		resources, health, metrics, events := 0, 0, 0, 0
		for _, batch := range store.batch {
			resources += len(batch.Resources)
			health += len(batch.Health)
			metrics += len(batch.Metrics)
			events += len(batch.Events)
			for _, sample := range batch.Metrics {
				if !sample.Synthetic || sample.SampledAt != demo.ObservationTime() {
					t.Fatal("synthetic metric clock changed")
				}
			}
		}
		if resources != min(bound, 6) || health != resources || metrics == 0 || events == 0 || completion.Status.ObservedAt != demo.ObservationTime() {
			t.Fatalf("capture counts=%d/%d/%d/%d status=%#v", resources, health, metrics, events, completion.Status)
		}
		if bound == 2 && (completion.Complete || completion.Status.Completeness != domain.CompletenessPartial || store.retired != 0) {
			t.Fatal("bounded partial capture retired unseen inventory")
		}
		if bound == 1000 && (!completion.Complete || store.retired != 1) {
			t.Fatal("complete capture did not retire unseen inventory")
		}
	}
}

type failingCapture struct {
	*demo.Provider
	calls atomic.Int32
}

func (provider *failingCapture) CaptureInventory(context.Context) (domain.InventoryCapture, error) {
	provider.calls.Add(1)
	err, _ := domain.NewProviderError("fixture.capture.read", domain.ProviderErrorPermissionDenied, false, errors.New("fixture denial"))
	return domain.InventoryCapture{}, err
}

func TestWorkerRecordsFailureAndBoundsBackoffThenResetsOnConfigChange(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	store := &captureStore{configurations: []domain.SourceConfiguration{testConfiguration(now)}}
	provider := &failingCapture{Provider: demo.MustNew()}
	worker := testWorker(t, store, ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (LiveProvider, error) { return provider, nil }), func() time.Time { return now }, nil)
	for _, advance := range []time.Duration{0, 0, time.Second, time.Second, time.Second} {
		now = now.Add(advance)
		if err := worker.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if provider.calls.Load() != 3 || len(store.batch) != 0 || store.retired != 0 || len(store.completions) != 3 {
		t.Fatal("failure schedule or read-only cache preservation failed")
	}
	for _, completion := range store.completions {
		if completion.Complete || completion.Status.State != domain.AvailabilityPermissionDenied {
			t.Fatalf("failed completion=%#v", completion)
		}
	}
	store.configurations[0].Version++
	store.configurations[0].UpdatedAt = now
	if err := worker.RunOnce(context.Background()); err != nil || provider.calls.Load() != 4 {
		t.Fatal("source version change did not reset backoff")
	}
}

type slowCapture struct {
	*demo.Provider
	source domain.InfrastructureSource
	active *atomic.Int32
	peak   *atomic.Int32
}

func (provider slowCapture) Source() domain.InfrastructureSource { return provider.source }
func (provider slowCapture) GetCapabilities(_ context.Context, source domain.ProviderRef) (domain.CapabilitySet, error) {
	return unavailableCapabilities(source, availableStatus(time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC))), nil
}
func (provider slowCapture) CaptureInventory(ctx context.Context) (domain.InventoryCapture, error) {
	active := provider.active.Add(1)
	defer provider.active.Add(-1)
	for old := provider.peak.Load(); active > old && !provider.peak.CompareAndSwap(old, active); old = provider.peak.Load() {
	}
	<-ctx.Done()
	return domain.InventoryCapture{}, ctx.Err()
}

func TestWorkerBoundsConcurrentReadsAndRecordsTimedOutSources(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	store := &captureStore{}
	for _, id := range []domain.SourceID{"source_a", "source_b", "source_c"} {
		configuration := testConfiguration(now)
		configuration.Source.ID = id
		store.configurations = append(store.configurations, configuration)
	}
	var active, peak atomic.Int32
	worker := testWorker(t, store, ProviderResolverFunc(func(_ context.Context, configuration domain.SourceConfiguration) (LiveProvider, error) {
		return slowCapture{Provider: demo.MustNew(), source: configuration.Source, active: &active, peak: &peak}, nil
	}), func() time.Time { return now }, func(options *WorkerOptions) { options.ReadTimeout = 100 * time.Millisecond })
	start := time.Now()
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if peak.Load() != 2 || active.Load() != 0 || time.Since(start) > 2*time.Second || len(store.completions) != 3 || len(store.batch) != 0 || store.retired != 0 {
		t.Fatal("concurrency or deadline bound failed")
	}
	for _, completion := range store.completions {
		if completion.Status.State != domain.AvailabilityUnavailable || completion.Complete {
			t.Fatal("timeout marked source fresh")
		}
	}
}

func TestWorkerNeverFinishesPartialWriteAsComplete(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	store := &captureStore{configurations: []domain.SourceConfiguration{testConfiguration(now)}, saveErr: domain.ErrRepositoryUnavailable}
	worker := testWorker(t, store, ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (LiveProvider, error) { return demo.MustNew(), nil }), func() time.Time { return now }, nil)
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.completions) != 1 || store.completions[0].Complete || store.completions[0].Status.State != domain.AvailabilityUnavailable || store.retired != 0 {
		t.Fatal("failed write retired old inventory or marked fresh")
	}
}

func TestWorkerShutdownCancelsReadAndPreventsNewCaptureWrites(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	store := &captureStore{configurations: []domain.SourceConfiguration{testConfiguration(now)}}
	started := make(chan struct{})
	var active, peak atomic.Int32
	worker := testWorker(t, store, ProviderResolverFunc(func(_ context.Context, configuration domain.SourceConfiguration) (LiveProvider, error) {
		close(started)
		return slowCapture{Provider: demo.MustNew(), source: configuration.Source, active: &active, peak: &peak}, nil
	}), func() time.Time { return now }, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { worker.Run(ctx); close(done) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("collection did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if active.Load() != 0 || len(store.batch) != 0 || len(store.completions) != 0 || store.generation != 0 {
		t.Fatal("shutdown left provider call or opened a capture write")
	}
}

type failedMetrics struct {
	*demo.Provider
	code domain.ProviderErrorCode
}

type mixedMetricSets struct {
	*demo.Provider
	state domain.AvailabilityState
}

func (provider mixedMetricSets) GetCurrentMetrics(ctx context.Context, query domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	result, err := provider.Provider.GetCurrentMetrics(ctx, query)
	if err != nil || len(result.Sets) < 2 {
		return result, err
	}
	result.Sets[0].Status = unavailableStatus(provider.state, result.Status.ObservedAt, "fixture.metric_gap")
	result.Sets[0].Samples = nil
	return result, result.ValidateFor(query)
}

func TestWorkerUsesConservativePortStatusForMixedMetricResourceFailures(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for _, state := range []domain.AvailabilityState{domain.AvailabilityPermissionDenied, domain.AvailabilityUnavailable, domain.AvailabilityUnsupported} {
		store := &captureStore{configurations: []domain.SourceConfiguration{testConfiguration(now)}}
		worker := testWorker(t, store, ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (LiveProvider, error) {
			return mixedMetricSets{Provider: demo.MustNew(), state: state}, nil
		}), func() time.Time { return now }, nil)
		if err := worker.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		completion := store.completions[0]
		if !completion.Complete || store.retired != 1 {
			t.Fatal("metric resource gap hid complete inventory")
		}
		count := 0
		for _, batch := range store.batch {
			count += len(batch.Metrics)
		}
		for _, capability := range completion.Capabilities.Capabilities {
			if capability.Name != domain.CapabilityMetricCurrent && capability.Name != domain.CapabilityMetricHistory {
				continue
			}
			if state == domain.AvailabilityPermissionDenied {
				if capability.Status.State != state || capability.Status.AllowsData() || count != 0 {
					t.Fatal("mixed denial stored or exposed metric data")
				}
			} else if capability.Status.State != domain.AvailabilityStale || capability.Status.Completeness != domain.CompletenessPartial || capability.Status.ReasonCode == "" || count == 0 {
				t.Fatal("mixed resource gap claimed fresh metrics or lost valid samples")
			}
		}
	}
}

func (provider failedMetrics) GetCurrentMetrics(context.Context, domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	err, _ := domain.NewProviderError("fixture.metrics.read", provider.code, false, errors.New("fixture metric failure"))
	return domain.CurrentMetricResult{}, err
}

func TestWorkerKeepsInventoryCompleteButPropagatesMetricFailureToHistory(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		code  domain.ProviderErrorCode
		state domain.AvailabilityState
	}{{domain.ProviderErrorPermissionDenied, domain.AvailabilityPermissionDenied}, {domain.ProviderErrorUnavailable, domain.AvailabilityUnavailable}} {
		store := &captureStore{configurations: []domain.SourceConfiguration{testConfiguration(now)}}
		worker := testWorker(t, store, ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (LiveProvider, error) {
			return failedMetrics{Provider: demo.MustNew(), code: test.code}, nil
		}), func() time.Time { return now }, nil)
		if err := worker.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		completion := store.completions[0]
		if !completion.Complete || completion.Status.State != domain.AvailabilityAvailable || store.retired != 1 {
			t.Fatal("optional metric failure hid complete inventory")
		}
		for _, capability := range completion.Capabilities.Capabilities {
			if (capability.Name == domain.CapabilityMetricCurrent || capability.Name == domain.CapabilityMetricHistory) && capability.Status.State != test.state {
				t.Fatal("metric failure lost at history capability")
			}
		}
	}
}

func TestWriterFlushesBoundedBatchesInReferenceOrder(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	store := &captureStore{configurations: []domain.SourceConfiguration{testConfiguration(now)}}
	worker := testWorker(t, store, ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (LiveProvider, error) { return demo.MustNew(), nil }), func() time.Time { return now }, nil)
	token, err := store.BeginCapture(context.Background(), demo.SourceID, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := demo.MustNew().CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	collected := collection{inventory: domain.InventoryCapture{Environments: fixture.Environments}}
	base := fixture.Resources[0]
	for index := 0; index < 205; index++ {
		resource := base
		baseIdentity := base.Resource.Ref.Identity
		identity, err := domain.NewResourceIdentity(baseIdentity.Provider, baseIdentity.SourceID, baseIdentity.EnvironmentID, baseIdentity.Kind, baseIdentity.Scope, domain.NativeResourceID(fmt.Sprintf("fixture-resource-%03d", index)))
		if err != nil {
			t.Fatal(err)
		}
		resource.Resource.Ref, err = domain.NewResourceRef(identity)
		if err != nil {
			t.Fatal(err)
		}
		collected.inventory.Resources = append(collected.inventory.Resources, resource)
		collected.inventory.Health = append(collected.inventory.Health, domain.HealthSnapshot{ResourceID: resource.Resource.Ref.ID, State: domain.HealthUnknown, Status: availableStatus(demo.ObservationTime())})
		collected.metrics = append(collected.metrics, domain.MetricSample{ID: domain.MetricSampleID(fmt.Sprintf("metric_fixture_%03d", index)), ResourceID: resource.Resource.Ref.ID,
			Name: domain.MetricCPUUsageCores, Unit: domain.MetricUnitCores, Value: 1, SampledAt: demo.ObservationTime(), Interval: time.Minute, Synthetic: true})
	}
	if err := worker.save(context.Background(), token, collected); err != nil {
		t.Fatal(err)
	}
	if len(store.batch) != 7 {
		t.Fatalf("batches=%d", len(store.batch))
	}
	known := make(map[domain.ResourceID]bool)
	count := 0
	for _, batch := range store.batch {
		bound := len(batch.Environments) + len(batch.Resources) + len(batch.Health) + len(batch.WorkloadReplicas) + len(batch.Relationships) + len(batch.Metrics) + len(batch.Events)
		if bound < 1 || bound > domain.MaxRepositoryWriteBatch || batch.Validate() != nil {
			t.Fatal("write batch bound failed")
		}
		count += bound
		for _, resource := range batch.Resources {
			known[resource.Resource.Ref.ID] = true
		}
		for _, health := range batch.Health {
			if !known[health.ResourceID] {
				t.Fatal("health saved before resource")
			}
		}
		for _, sample := range batch.Metrics {
			if !known[sample.ResourceID] {
				t.Fatal("metric saved before resource")
			}
		}
	}
	if count != len(fixture.Environments)+615 || len(known) != 205 {
		t.Fatal("writer lost bounded capture data")
	}
}
