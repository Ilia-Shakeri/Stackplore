package observation

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type fixtureReader struct {
	*demo.Provider
	configuration domain.SourceConfiguration
	readErr       error
}

func (reader *fixtureReader) GetSourceConfiguration(context.Context, domain.SourceID) (domain.SourceConfiguration, error) {
	return reader.configuration, nil
}
func (reader *fixtureReader) GetWorkloadReplicas(context.Context, domain.ResourceDetailQuery) (domain.WorkloadReplicaSnapshot, error) {
	return domain.WorkloadReplicaSnapshot{}, domain.ErrRepositoryNotFound
}
func (reader *fixtureReader) ListResources(ctx context.Context, query domain.InventoryQuery) (domain.ResourcePage, error) {
	if reader.readErr != nil {
		return domain.ResourcePage{}, reader.readErr
	}
	return reader.Provider.ListResources(ctx, query)
}

func TestCachedReadsUseRepositoryAndOnlyLogsResolveLiveProvider(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	reader := &fixtureReader{Provider: demo.MustNew(), configuration: testConfiguration(now)}
	var resolves atomic.Int32
	provider, err := NewCachedProvider(CachedProviderOptions{Source: reader.Source(), Reader: reader,
		Resolver: ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (LiveProvider, error) {
			resolves.Add(1)
			return demo.MustNew(), nil
		}),
		ObservationTime: demo.ObservationTime, FixtureVersion: demo.FixtureVersion})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: demo.SourceID}
	query := domain.InventoryQuery{Source: source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 10}}
	page, err := provider.ListResources(ctx, query)
	if err != nil || len(page.Resources) != 9 {
		t.Fatal("cached resources failed")
	}
	ref := page.Resources[0].Resource.Ref
	if _, err := provider.ListEnvironments(ctx, domain.EnvironmentQuery{Source: source, Page: domain.PageRequest{Limit: 10}}); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.GetResource(ctx, domain.ResourceDetailQuery{Source: source, ResourceID: ref.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.GetHealth(ctx, domain.HealthQuery{Source: source, Resources: []domain.ResourceRef{ref}}); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.GetRelationships(ctx, domain.TopologyQuery{Source: source, Page: domain.PageRequest{Limit: 10}}); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.GetCurrentMetrics(ctx, domain.CurrentMetricQuery{Source: source, Resources: []domain.ResourceRef{ref}, Names: []domain.MetricName{domain.MetricCPUUsageCores}}); err != nil {
		t.Fatal(err)
	}
	rangeValue := domain.TimeRange{Start: demo.ObservationTime().Add(-time.Hour), End: demo.ObservationTime().Add(time.Microsecond)}
	if _, err := provider.GetMetricHistory(ctx, domain.MetricHistoryQuery{Source: source, Resources: []domain.ResourceRef{ref}, Names: []domain.MetricName{domain.MetricCPUUsageCores}, Range: rangeValue, PointLimit: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.ListEvents(ctx, domain.EventQuery{Source: source, Range: rangeValue, Page: domain.PageRequest{Limit: 10}}); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.GetCapabilities(ctx, source); err != nil {
		t.Fatal(err)
	}
	if resolves.Load() != 0 || provider.ObservationTime() != demo.ObservationTime() || provider.FixtureVersion() != demo.FixtureVersion {
		t.Fatal("durable reads reached live provider or changed synthetic clock")
	}
	logs := domain.LogReadQuery{Source: source, Targets: []domain.LogTarget{{Resource: ref}}, Range: rangeValue, LineLimit: 10, ByteLimit: 1024}
	if _, err := provider.ReadLogs(ctx, logs); err != nil || resolves.Load() != 1 {
		t.Fatal("live log read failed")
	}
	reader.configuration.Enabled = false
	if _, err := provider.ReadLogs(ctx, logs); err == nil || resolves.Load() != 1 {
		t.Fatal("disabled source still reached live logs")
	}
	query.Source.SourceID = "source_other"
	if _, err := provider.ListResources(ctx, query); err == nil {
		t.Fatal("cached provider accepted another source")
	}
}

type unreachableReader struct{ ObservationReader }

func TestCachedFacadeRejectsCrossSourceBeforeAnyRepositoryRead(t *testing.T) {
	source := demo.MustNew().Source()
	provider, err := NewCachedProvider(CachedProviderOptions{Source: source, Reader: unreachableReader{}, Resolver: ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (LiveProvider, error) {
		t.Fatal("cross-source reached resolver")
		return nil, ErrUnavailable
	})})
	if err != nil {
		t.Fatal(err)
	}
	other := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "source_other"}
	identity, err := domain.NewResourceIdentity(domain.ProviderDemo, other.SourceID, demo.EnvironmentID, "demo.service", domain.Scope{Kind: domain.ScopeProject, Name: "demo-shop"}, "api")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	page := domain.PageRequest{Limit: 10}
	times := domain.TimeRange{Start: demo.ObservationTime().Add(-time.Hour), End: demo.ObservationTime().Add(time.Microsecond)}
	checks := []func() error{
		func() error {
			_, err := provider.ListEnvironments(ctx, domain.EnvironmentQuery{Source: other, Page: page})
			return err
		},
		func() error {
			_, err := provider.ListResources(ctx, domain.InventoryQuery{Source: other, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: page})
			return err
		},
		func() error {
			_, err := provider.GetResource(ctx, domain.ResourceDetailQuery{Source: other, ResourceID: ref.ID})
			return err
		},
		func() error {
			_, err := provider.GetWorkloadReplicas(ctx, domain.ResourceDetailQuery{Source: other, ResourceID: ref.ID})
			return err
		},
		func() error {
			_, err := provider.GetHealth(ctx, domain.HealthQuery{Source: other, Resources: []domain.ResourceRef{ref}})
			return err
		},
		func() error {
			_, err := provider.GetRelationships(ctx, domain.TopologyQuery{Source: other, Resources: []domain.ResourceRef{ref}, Page: page})
			return err
		},
		func() error {
			_, err := provider.GetCurrentMetrics(ctx, domain.CurrentMetricQuery{Source: other, Resources: []domain.ResourceRef{ref}, Names: []domain.MetricName{domain.MetricCPUUsageCores}})
			return err
		},
		func() error {
			_, err := provider.GetMetricHistory(ctx, domain.MetricHistoryQuery{Source: other, Resources: []domain.ResourceRef{ref}, Names: []domain.MetricName{domain.MetricCPUUsageCores}, Range: times, PointLimit: 10})
			return err
		},
		func() error {
			_, err := provider.ListEvents(ctx, domain.EventQuery{Source: other, Range: times, Page: page})
			return err
		},
		func() error { _, err := provider.GetCapabilities(ctx, other); return err },
		func() error {
			_, err := provider.ReadLogs(ctx, domain.LogReadQuery{Source: other, Targets: []domain.LogTarget{{Resource: ref}}, Range: times, LineLimit: 10, ByteLimit: 1024})
			return err
		},
	}
	for _, read := range checks {
		var failure *domain.ProviderError
		if err := read(); !errors.As(err, &failure) || failure.Code != domain.ProviderErrorInvalidRequest {
			t.Fatal("cross-source query not rejected at facade boundary")
		}
	}
}

func TestCachedReaderPreservesSafeProviderErrors(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	reader := &fixtureReader{Provider: demo.MustNew(), configuration: testConfiguration(now), readErr: domain.ErrRepositoryUnavailable}
	provider, _ := NewCachedProvider(CachedProviderOptions{Source: reader.Source(), Reader: reader, Resolver: ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (LiveProvider, error) { return nil, ErrUnavailable })})
	query := domain.InventoryQuery{Source: domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: demo.SourceID}, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 10}}
	_, err := provider.ListResources(context.Background(), query)
	var failure *domain.ProviderError
	if !errors.As(err, &failure) || failure.Code != domain.ProviderErrorUnavailable {
		t.Fatal("repository outage lost safe provider error")
	}
	denied, _ := domain.NewProviderError("fixture.read.denied", domain.ProviderErrorPermissionDenied, false, errors.New("private fixture error"))
	reader.readErr = denied
	_, err = provider.ListResources(context.Background(), query)
	if !errors.As(err, &failure) || failure.Code != domain.ProviderErrorPermissionDenied || err.Error() == "private fixture error" {
		t.Fatal("typed denial was changed or raw error escaped")
	}
}
