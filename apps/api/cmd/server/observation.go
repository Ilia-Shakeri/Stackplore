package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/observation"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/sourcesettings"
)

type observationDatabase interface {
	domain.CaptureRepository
	observation.ObservationReader
	domain.ObservationRetentionRepository
	sourcesettings.Repository
	domain.SourceCollectionReader
	domain.AlertMetricHistoryProvider
	domain.WorkloadReplicaHistoryProvider
	domain.DemoAlertSeedRepository
	domain.AlertHistoryRetentionRepository
}

type observationRuntime struct {
	providers    []httpapi.ProductProvider
	settings     *sourcesettings.Service
	collection   *observation.Worker
	retention    *observation.RetentionWorker
	alertHistory observationDatabase
	demoSeeds    []domain.DemoAlertSeed
}

func buildObservationRuntime(ctx context.Context, settings config.Config, database databaseRuntime, providers []httpapi.ProductProvider, logger *slog.Logger) (observationRuntime, error) {
	repository, ok := database.(observationDatabase)
	if !ok || ctx == nil || len(providers) == 0 || logger == nil {
		return observationRuntime{}, errObservationStart
	}
	profiles := make(map[domain.SourceID]domain.SourceProfileID, len(providers))
	byID := make(map[domain.SourceID]observation.LiveProvider, len(providers))
	defaults := make([]domain.SourceConfiguration, 0, len(providers))
	seeds := make([]domain.DemoAlertSeed, 0, 1)
	at := time.Now().UTC().Truncate(time.Microsecond)
	for _, provider := range providers {
		source := provider.Source()
		if fixture, ok := provider.(*demo.Provider); ok {
			seed, err := buildDemoAlertSeed(ctx, fixture)
			if err != nil {
				return observationRuntime{}, errObservationStart
			}
			seeds = append(seeds, seed)
		}
		profile := profileFor(source.Provider)
		profiles[source.ID] = profile
		byID[source.ID] = provider
		defaults = append(defaults, domain.SourceConfiguration{Source: source, ProfileID: profile, Enabled: true, Version: 1, CreatedAt: at, UpdatedAt: at})
	}
	if err := repository.EnsureSourceConfigurations(ctx, defaults); err != nil {
		return observationRuntime{}, errObservationStart
	}
	page, err := repository.ListSourceConfigurations(ctx, domain.PageRequest{Limit: 64})
	if err != nil || page.ValidateFor(domain.PageRequest{Limit: 64}) != nil || page.Page.HasMore || len(page.Configurations) == 0 {
		return observationRuntime{}, errObservationStart
	}
	resolver := observation.ProviderResolverFunc(func(ctx context.Context, configuration domain.SourceConfiguration) (observation.LiveProvider, error) {
		if ctx == nil || ctx.Err() != nil || !configuration.Enabled || profiles[configuration.Source.ID] != configuration.ProfileID {
			return nil, observation.ErrProfileUnavailable
		}
		provider := byID[configuration.Source.ID]
		if provider == nil || provider.Source().Provider != configuration.Source.Provider || provider.Source().Synthetic != configuration.Source.Synthetic {
			return nil, observation.ErrProfileUnavailable
		}
		return provider, nil
	})
	cached := make([]httpapi.ProductProvider, 0, len(page.Configurations))
	for _, configuration := range page.Configurations {
		var observationTime func() time.Time
		if clock, ok := byID[configuration.Source.ID].(interface{ ObservationTime() time.Time }); ok {
			observationTime = clock.ObservationTime
		}
		fixtureVersion := ""
		if fixture, ok := byID[configuration.Source.ID].(interface{ FixtureVersion() string }); ok {
			fixtureVersion = fixture.FixtureVersion()
		}
		if configuration.Source.Synthetic && configuration.ProfileID == domain.SourceProfileDemoFixture && observationTime == nil {
			observationTime = demo.ObservationTime
		}
		provider, err := observation.NewCachedProvider(observation.CachedProviderOptions{Source: configuration.Source, Reader: repository, Resolver: resolver, ObservationTime: observationTime, FixtureVersion: fixtureVersion})
		if err != nil {
			return observationRuntime{}, errObservationStart
		}
		cached = append(cached, provider)
	}
	manager, err := sourcesettings.New(repository, profiles, time.Now)
	if err != nil {
		return observationRuntime{}, errObservationStart
	}
	collection, err := observation.NewWorker(observation.WorkerOptions{Repository: repository, Resolver: resolver, Now: time.Now,
		OnError:      func(error) { logger.Warn("observation collection delayed", "reason", "bounded_collection_failed") },
		PollInterval: time.Second, CollectionInterval: 30 * time.Second, ReadTimeout: min(settings.RequestTimeout, 20*time.Second),
		WriteTimeout: 10 * time.Second, MaximumBackoff: 5 * time.Minute, MaximumSources: 64, MaximumConcurrency: 4, MaximumPages: 100,
		MaximumItems: 1000, MaximumMetricSamples: 10000, MaximumEvents: 1000, EventLookback: 5 * time.Minute, RetirementBatches: 8})
	if err != nil {
		return observationRuntime{}, errObservationStart
	}
	retention, err := observation.NewRetentionWorker(observation.RetentionOptions{Repository: repository, Now: time.Now,
		OnError:      func(error) { logger.Warn("observation retention delayed", "reason", "bounded_retention_failed") },
		PollInterval: time.Minute, Timeout: 10 * time.Second, BatchLimit: 1000, MaximumBatches: 10, MetricAge: 7 * 24 * time.Hour, HealthAge: 30 * 24 * time.Hour,
		EventAge: 30 * 24 * time.Hour, DeliveryAge: 30 * 24 * time.Hour})
	if err != nil {
		return observationRuntime{}, errObservationStart
	}
	return observationRuntime{providers: cached, settings: manager, collection: collection, retention: retention, alertHistory: repository, demoSeeds: seeds}, nil
}

func profileFor(provider domain.ProviderKind) domain.SourceProfileID {
	switch provider {
	case domain.ProviderDemo:
		return domain.SourceProfileDemoFixture
	case domain.ProviderDocker:
		return domain.SourceProfileDockerSocketDevelopment
	case domain.ProviderKubernetes:
		return domain.SourceProfileKubernetesInCluster
	}
	return ""
}
