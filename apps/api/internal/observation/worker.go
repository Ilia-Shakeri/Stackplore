package observation

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type WorkerOptions struct {
	Repository           domain.CaptureRepository
	Resolver             ProviderResolver
	Now                  func() time.Time
	OnError              func(error)
	PollInterval         time.Duration
	CollectionInterval   time.Duration
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	MaximumBackoff       time.Duration
	MaximumSources       int
	MaximumConcurrency   int
	MaximumPages         int
	MaximumItems         int
	MaximumMetricSamples int
	MaximumEvents        int
	EventLookback        time.Duration
	RetirementBatches    int
}

type sourceSchedule struct {
	version  int64
	failures int
	next     time.Time
}

type Worker struct {
	repository           domain.CaptureRepository
	resolver             ProviderResolver
	now                  func() time.Time
	onError              func(error)
	pollInterval         time.Duration
	collectionInterval   time.Duration
	readTimeout          time.Duration
	writeTimeout         time.Duration
	maximumBackoff       time.Duration
	maximumSources       int
	maximumConcurrency   int
	maximumPages         int
	maximumItems         int
	maximumMetricSamples int
	maximumEvents        int
	eventLookback        time.Duration
	retirementBatches    int
	gate                 chan struct{}
	schedules            map[domain.SourceID]sourceSchedule
}

func NewWorker(options WorkerOptions) (*Worker, error) {
	if options.Repository == nil || options.Resolver == nil || options.Now == nil ||
		options.PollInterval < 100*time.Millisecond || options.PollInterval > time.Minute ||
		options.CollectionInterval < time.Second || options.CollectionInterval > 15*time.Minute ||
		options.ReadTimeout < 100*time.Millisecond || options.ReadTimeout > 2*time.Minute ||
		options.WriteTimeout < 100*time.Millisecond || options.WriteTimeout > 2*time.Minute ||
		options.MaximumBackoff < options.CollectionInterval || options.MaximumBackoff > time.Hour ||
		options.MaximumSources < 1 || options.MaximumSources > 64 ||
		options.MaximumConcurrency < 1 || options.MaximumConcurrency > 8 ||
		options.MaximumPages < 1 || options.MaximumPages > 100 ||
		options.MaximumItems < 1 || options.MaximumItems > domain.MaxInventoryCaptureItems ||
		options.MaximumMetricSamples < 1 || options.MaximumMetricSamples > domain.MaxInventoryCaptureItems*9 ||
		options.MaximumEvents < 1 || options.MaximumEvents > domain.MaxInventoryCaptureItems ||
		options.EventLookback < time.Second || options.EventLookback > 24*time.Hour ||
		options.RetirementBatches < 1 || options.RetirementBatches > 100 {
		return nil, ErrInvalidOptions
	}
	if options.OnError == nil {
		options.OnError = func(error) {}
	}
	return &Worker{
		repository: options.Repository, resolver: options.Resolver, now: options.Now, onError: options.OnError,
		pollInterval: options.PollInterval, collectionInterval: options.CollectionInterval, readTimeout: options.ReadTimeout,
		writeTimeout: options.WriteTimeout, maximumBackoff: options.MaximumBackoff, maximumSources: options.MaximumSources,
		maximumConcurrency: options.MaximumConcurrency, maximumPages: options.MaximumPages, maximumItems: options.MaximumItems,
		maximumMetricSamples: options.MaximumMetricSamples, maximumEvents: options.MaximumEvents, eventLookback: options.EventLookback,
		retirementBatches: options.RetirementBatches, gate: make(chan struct{}, 1), schedules: make(map[domain.SourceID]sourceSchedule),
	}, nil
}

func (worker *Worker) Run(ctx context.Context) {
	if worker == nil || ctx == nil {
		return
	}
	worker.runAndReport(ctx)
	ticker := time.NewTicker(worker.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			worker.runAndReport(ctx)
		}
	}
}

func (worker *Worker) runAndReport(ctx context.Context) {
	if err := worker.RunOnce(ctx); err != nil && ctx.Err() == nil {
		worker.onError(err)
	}
}

func (worker *Worker) RunOnce(ctx context.Context) error {
	if worker == nil || ctx == nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	select {
	case worker.gate <- struct{}{}:
		defer func() { <-worker.gate }()
	case <-ctx.Done():
		return ErrUnavailable
	default:
		return ErrUnavailable
	}
	readContext, cancel := context.WithTimeout(ctx, worker.readTimeout)
	request := domain.PageRequest{Limit: worker.maximumSources}
	page, err := worker.repository.ListSourceConfigurations(readContext, request)
	cancel()
	if err != nil || page.ValidateFor(request) != nil || page.Page.HasMore {
		return errors.Join(ErrUnavailable, err)
	}
	now := worker.time()
	present := make(map[domain.SourceID]struct{}, len(page.Configurations))
	due := make([]domain.SourceConfiguration, 0, len(page.Configurations))
	for _, configuration := range page.Configurations {
		id := configuration.Source.ID
		present[id] = struct{}{}
		if !configuration.Enabled {
			delete(worker.schedules, id)
			continue
		}
		schedule := worker.schedules[id]
		if schedule.version != configuration.Version {
			schedule = sourceSchedule{version: configuration.Version}
			worker.schedules[id] = schedule
		}
		if !schedule.next.After(now) {
			due = append(due, configuration)
		}
	}
	for id := range worker.schedules {
		if _, exists := present[id]; !exists {
			delete(worker.schedules, id)
		}
	}
	type outcome struct {
		configuration domain.SourceConfiguration
		err           error
	}
	jobs := make(chan domain.SourceConfiguration)
	results := make(chan outcome, len(due))
	var workers sync.WaitGroup
	for range min(worker.maximumConcurrency, len(due)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for configuration := range jobs {
				results <- outcome{configuration: configuration, err: worker.captureOne(ctx, configuration)}
			}
		}()
	}
	for _, configuration := range due {
		select {
		case jobs <- configuration:
		case <-ctx.Done():
			break
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	workers.Wait()
	close(results)
	for result := range results {
		schedule := worker.schedules[result.configuration.Source.ID]
		delay := worker.collectionInterval
		if result.err == nil {
			schedule.failures = 0
		} else {
			schedule.failures = min(schedule.failures+1, 16)
			for count := 1; count < schedule.failures && delay < worker.maximumBackoff; count++ {
				delay = min(delay*2, worker.maximumBackoff)
			}
			if ctx.Err() == nil {
				worker.onError(result.err)
			}
		}
		schedule.next = worker.time().Add(delay)
		worker.schedules[result.configuration.Source.ID] = schedule
	}
	return ctx.Err()
}

func (worker *Worker) captureOne(ctx context.Context, configuration domain.SourceConfiguration) error {
	readContext, cancelRead := context.WithTimeout(ctx, worker.readTimeout)
	live, collectionErr := worker.resolver.ResolveProvider(readContext, configuration)
	if collectionErr == nil && (live == nil || live.Source().Validate() != nil || live.Source().ID != configuration.Source.ID || live.Source().Provider != configuration.Source.Provider || live.Source().Synthetic != configuration.Source.Synthetic) {
		collectionErr = ErrUnavailable
	}
	observationClock := worker.time
	if configuration.Source.Synthetic {
		at, err := worker.syntheticCaptureTime(configuration, live, collectionErr)
		if err != nil {
			cancelRead()
			return err
		}
		observationClock = func() time.Time { return at }
	}
	var collected collection
	if collectionErr == nil {
		collected, collectionErr = worker.collect(readContext, configuration, live, observationClock)
	}
	cancelRead()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	writeContext, cancelWrite := context.WithTimeout(ctx, worker.writeTimeout)
	defer cancelWrite()
	token, err := worker.repository.BeginCapture(writeContext, configuration.Source.ID, configuration.Version, observationClock())
	if err != nil || token.Validate() != nil {
		return errors.Join(ErrUnavailable, err)
	}
	if collectionErr != nil {
		status := errorStatus(collectionErr, token.ObservedAt)
		completion := domain.CaptureCompletion{Token: token, CollectedAt: worker.time(), Status: status, Capabilities: unavailableCapabilities(token.Source, status)}
		return errors.Join(collectionErr, worker.repository.FinishCapture(writeContext, completion))
	}
	if err := worker.save(writeContext, token, collected); err != nil {
		status := unavailableStatus(domain.AvailabilityUnavailable, token.ObservedAt, "observation.persistence_failed")
		_ = worker.repository.FinishCapture(writeContext, domain.CaptureCompletion{Token: token, CollectedAt: worker.time(), Status: status, Capabilities: unavailableCapabilities(token.Source, status)})
		return errors.Join(ErrUnavailable, err)
	}
	completion := domain.CaptureCompletion{Token: token, CollectedAt: worker.time(), Status: collected.inventory.Status, Capabilities: collected.capabilities, Complete: collected.inventory.Complete, Coverage: collected.inventory.Coverage}
	if completion.Validate() != nil {
		return ErrUnavailable
	}
	if err := worker.repository.FinishCapture(writeContext, completion); err != nil {
		return errors.Join(ErrUnavailable, err)
	}
	if !completion.Status.AllowsData() {
		return ErrUnavailable
	}
	if !completion.Complete && len(completion.Coverage) == 0 {
		return nil
	}
	for range worker.retirementBatches {
		removed, err := worker.repository.RetireUnseen(writeContext, token, domain.MaxRepositoryWriteBatch)
		if err != nil {
			return errors.Join(ErrUnavailable, err)
		}
		if removed < domain.MaxRepositoryWriteBatch {
			break
		}
	}
	return nil
}

func (worker *Worker) syntheticCaptureTime(configuration domain.SourceConfiguration, provider LiveProvider, resolveErr error) (time.Time, error) {
	if resolveErr != nil || provider == nil || configuration.Source.Provider != domain.ProviderDemo || configuration.ProfileID != domain.SourceProfileDemoFixture {
		return time.Time{}, errors.Join(ErrUnavailable, resolveErr)
	}
	clock, ok := provider.(interface{ ObservationTime() time.Time })
	if !ok {
		return time.Time{}, ErrUnavailable
	}
	at := clock.ObservationTime()
	token := domain.CaptureToken{Source: domain.ProviderRef{Provider: configuration.Source.Provider, SourceID: configuration.Source.ID}, ConfigurationVersion: configuration.Version, Generation: 1, ObservedAt: at}
	if token.Validate() != nil || at.After(worker.time()) {
		return time.Time{}, ErrUnavailable
	}
	return at, nil
}

func (worker *Worker) time() time.Time { return worker.now().UTC().Truncate(time.Microsecond) }
