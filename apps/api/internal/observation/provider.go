// Package observation coordinates bounded provider collection and durable reads.
package observation

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

var (
	ErrInvalidOptions = errors.New("observation options are invalid")
	ErrUnavailable    = errors.New("observation is unavailable")
)

// LiveProvider owns the safe operator-selected read boundary used by collection.
type LiveProvider interface {
	domain.InventoryProvider
	domain.HealthProvider
	domain.TopologyProvider
	domain.CapabilityProvider
	Source() domain.InfrastructureSource
}

type ProviderResolver interface {
	ResolveProvider(context.Context, domain.SourceConfiguration) (LiveProvider, error)
}

type ProviderResolverFunc func(context.Context, domain.SourceConfiguration) (LiveProvider, error)

func (resolve ProviderResolverFunc) ResolveProvider(ctx context.Context, source domain.SourceConfiguration) (LiveProvider, error) {
	return resolve(ctx, source)
}

// ObservationReader uses persisted observations. Log bodies remain at their provider.
type ObservationReader interface {
	domain.InventoryProvider
	domain.HealthProvider
	domain.TopologyProvider
	domain.CurrentMetricProvider
	domain.WorkloadReplicaProvider
	domain.MetricHistoryProvider
	domain.EventHistoryProvider
	domain.CapabilityProvider
	GetSourceConfiguration(context.Context, domain.SourceID) (domain.SourceConfiguration, error)
}

type CachedProviderOptions struct {
	Source          domain.InfrastructureSource
	Reader          ObservationReader
	Resolver        ProviderResolver
	ObservationTime func() time.Time
	FixtureVersion  string
}

type CachedProvider struct {
	source         domain.InfrastructureSource
	reader         ObservationReader
	resolver       ProviderResolver
	now            func() time.Time
	fixtureVersion string
}

func NewCachedProvider(options CachedProviderOptions) (*CachedProvider, error) {
	if options.Source.Validate() != nil || options.Reader == nil || options.Resolver == nil || !validFixtureVersion(options.Source, options.FixtureVersion) {
		return nil, ErrInvalidOptions
	}
	now := options.ObservationTime
	if now == nil {
		now = time.Now
	}
	return &CachedProvider{source: options.Source, reader: options.Reader, resolver: options.Resolver, now: now, fixtureVersion: options.FixtureVersion}, nil
}

func validFixtureVersion(source domain.InfrastructureSource, value string) bool {
	if value == "" {
		return true
	}
	if !source.Synthetic || source.Provider != domain.ProviderDemo || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character != '-' && character != '_' && character != '.' && (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func (provider *CachedProvider) FixtureVersion() string { return provider.fixtureVersion }

func (provider *CachedProvider) Source() domain.InfrastructureSource { return provider.source }

func (provider *CachedProvider) ObservationTime() time.Time {
	return provider.now().UTC().Truncate(time.Microsecond)
}

func (provider *CachedProvider) ListEnvironments(ctx context.Context, query domain.EnvironmentQuery) (domain.EnvironmentPage, error) {
	if err := provider.checkQuery(ctx, query.Source, query.Validate()); err != nil {
		return domain.EnvironmentPage{}, err
	}
	result, err := provider.reader.ListEnvironments(ctx, query)
	return result, readError(ctx, "observation.inventory.environments", err)
}

func (provider *CachedProvider) ListResources(ctx context.Context, query domain.InventoryQuery) (domain.ResourcePage, error) {
	if err := provider.checkQuery(ctx, query.Source, query.Validate()); err != nil {
		return domain.ResourcePage{}, err
	}
	result, err := provider.reader.ListResources(ctx, query)
	return result, readError(ctx, "observation.inventory.resources", err)
}

func (provider *CachedProvider) GetResource(ctx context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	if err := provider.checkQuery(ctx, query.Source, query.Validate()); err != nil {
		return domain.ResourceSnapshot{}, err
	}
	result, err := provider.reader.GetResource(ctx, query)
	return result, readError(ctx, "observation.inventory.detail", err)
}

func (provider *CachedProvider) GetHealth(ctx context.Context, query domain.HealthQuery) (domain.HealthResult, error) {
	if err := provider.checkQuery(ctx, query.Source, query.Validate()); err != nil {
		return domain.HealthResult{}, err
	}
	result, err := provider.reader.GetHealth(ctx, query)
	return result, readError(ctx, "observation.health.read", err)
}

func (provider *CachedProvider) GetWorkloadReplicas(ctx context.Context, query domain.ResourceDetailQuery) (domain.WorkloadReplicaSnapshot, error) {
	if err := provider.checkQuery(ctx, query.Source, query.Validate()); err != nil {
		return domain.WorkloadReplicaSnapshot{}, err
	}
	result, err := provider.reader.GetWorkloadReplicas(ctx, query)
	return result, readError(ctx, "observation.workload.replicas", err)
}

func (provider *CachedProvider) GetRelationships(ctx context.Context, query domain.TopologyQuery) (domain.RelationshipPage, error) {
	if err := provider.checkQuery(ctx, query.Source, query.Validate()); err != nil {
		return domain.RelationshipPage{}, err
	}
	result, err := provider.reader.GetRelationships(ctx, query)
	return result, readError(ctx, "observation.topology.read", err)
}

func (provider *CachedProvider) GetCurrentMetrics(ctx context.Context, query domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	if err := provider.checkQuery(ctx, query.Source, query.Validate()); err != nil {
		return domain.CurrentMetricResult{}, err
	}
	result, err := provider.reader.GetCurrentMetrics(ctx, query)
	return result, readError(ctx, "observation.metric.current", err)
}

func (provider *CachedProvider) GetMetricHistory(ctx context.Context, query domain.MetricHistoryQuery) (domain.MetricHistoryResult, error) {
	if err := provider.checkQuery(ctx, query.Source, query.Validate()); err != nil {
		return domain.MetricHistoryResult{}, err
	}
	result, err := provider.reader.GetMetricHistory(ctx, query)
	return result, readError(ctx, "observation.metric.history", err)
}

func (provider *CachedProvider) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	if err := provider.checkQuery(ctx, query.Source, query.Validate()); err != nil {
		return domain.EventPage{}, err
	}
	result, err := provider.reader.ListEvents(ctx, query)
	return result, readError(ctx, "observation.event.history", err)
}

func (provider *CachedProvider) GetCapabilities(ctx context.Context, source domain.ProviderRef) (domain.CapabilitySet, error) {
	if err := provider.checkQuery(ctx, source, source.Validate()); err != nil {
		return domain.CapabilitySet{}, err
	}
	result, err := provider.reader.GetCapabilities(ctx, source)
	return result, readError(ctx, "observation.capability.read", err)
}

func (provider *CachedProvider) checkQuery(ctx context.Context, source domain.ProviderRef, validation error) error {
	if ctx == nil || validation != nil || source != (domain.ProviderRef{Provider: provider.source.Provider, SourceID: provider.source.ID}) {
		return readError(ctx, "observation.query.validate", domain.ErrRepositoryInvalidInput)
	}
	if ctx.Err() != nil {
		return readError(ctx, "observation.query.validate", ctx.Err())
	}
	return nil
}

func (provider *CachedProvider) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	if ctx == nil || query.Validate() != nil || query.Source != (domain.ProviderRef{Provider: provider.source.Provider, SourceID: provider.source.ID}) {
		return domain.LogPage{}, readError(ctx, "observation.log.read", domain.ErrRepositoryInvalidInput)
	}
	configuration, err := provider.reader.GetSourceConfiguration(ctx, provider.source.ID)
	if err != nil {
		return domain.LogPage{}, readError(ctx, "observation.log.read", err)
	}
	if !configuration.Enabled {
		return domain.LogPage{}, readError(ctx, "observation.log.read", domain.ErrRepositoryUnavailable)
	}
	live, err := provider.resolver.ResolveProvider(ctx, configuration)
	if err != nil || live == nil || live.Source().ID != provider.source.ID || live.Source().Provider != provider.source.Provider {
		return domain.LogPage{}, readError(ctx, "observation.log.read", ErrUnavailable)
	}
	logs, supported := live.(domain.LogReadProvider)
	if !supported {
		failure, _ := domain.NewProviderError("observation.log.read", domain.ProviderErrorUnsupported, false, ErrUnavailable)
		return domain.LogPage{}, failure
	}
	return logs.ReadLogs(ctx, query)
}

func readError(ctx context.Context, operation domain.ProviderOperation, err error) error {
	if err == nil {
		return nil
	}
	var failure *domain.ProviderError
	if errors.As(err, &failure) {
		return err
	}
	code, retryable := domain.ProviderErrorUnavailable, true
	switch {
	case errors.Is(err, context.Canceled) || ctx != nil && errors.Is(ctx.Err(), context.Canceled):
		code, retryable = domain.ProviderErrorCanceled, false
	case errors.Is(err, context.DeadlineExceeded) || ctx != nil && errors.Is(ctx.Err(), context.DeadlineExceeded):
		code = domain.ProviderErrorDeadlineExceeded
	case errors.Is(err, domain.ErrRepositoryInvalidInput), errors.Is(err, domain.ErrRepositoryNotFound):
		code, retryable = domain.ProviderErrorInvalidRequest, false
	}
	failure, _ = domain.NewProviderError(operation, code, retryable, err)
	return failure
}

var _ domain.InventoryProvider = (*CachedProvider)(nil)
var _ domain.HealthProvider = (*CachedProvider)(nil)
var _ domain.TopologyProvider = (*CachedProvider)(nil)
var _ domain.CurrentMetricProvider = (*CachedProvider)(nil)
var _ domain.MetricHistoryProvider = (*CachedProvider)(nil)
var _ domain.EventHistoryProvider = (*CachedProvider)(nil)
var _ domain.LogReadProvider = (*CachedProvider)(nil)
