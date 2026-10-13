package main

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/observation"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type demoAlertReader struct{ observation.ObservationReader }

func demoAlertCache(t *testing.T, source domain.InfrastructureSource) *observation.CachedProvider {
	t.Helper()
	provider, err := observation.NewCachedProvider(observation.CachedProviderOptions{Source: source, Reader: demoAlertReader{},
		Resolver: observation.ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (observation.LiveProvider, error) {
			t.Fatal("fixed demo alerts called live provider")
			return nil, observation.ErrUnavailable
		}), ObservationTime: demo.ObservationTime, FixtureVersion: demo.FixtureVersion})
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func TestDemoAlertWrapperPreservesFixtureAndClonesBothBoundaries(t *testing.T) {
	fixture := demo.MustNew()
	alerts := fixture.Alerts()
	provider, err := newDemoAlertProvider(demoAlertCache(t, fixture.Source()), alerts)
	if err != nil {
		t.Fatal(err)
	}
	want := *alerts[0].ResourceID
	*alerts[0].ResourceID = "resource_changed_input"
	alerts[0].SummaryCode = "fixture.changed_input"
	first := provider.Alerts()
	if len(first) != 1 || first[0].State != domain.AlertFiring || !first[0].Synthetic || *first[0].ResourceID != want {
		t.Fatal("fixed firing fixture changed")
	}
	*first[0].ResourceID = "resource_changed_output"
	first[0].SummaryCode = "fixture.changed_output"
	second := provider.Alerts()
	if *second[0].ResourceID != want || second[0].SummaryCode == first[0].SummaryCode || provider.FixtureVersion() != demo.FixtureVersion || provider.ObservationTime() != demo.ObservationTime() {
		t.Fatal("wrapper leaked mutable fixture or lost cache metadata")
	}
}

func TestDemoAlertWrapperRejectsLiveForeignAndDuplicateAlerts(t *testing.T) {
	fixture := demo.MustNew()
	for _, mutate := range []func([]domain.Alert) []domain.Alert{
		func(alerts []domain.Alert) []domain.Alert { alerts[0].Synthetic = false; return alerts },
		func(alerts []domain.Alert) []domain.Alert { alerts[0].SourceID = "source_foreign"; return alerts },
		func(alerts []domain.Alert) []domain.Alert { return append(alerts, alerts[0]) },
		func(alerts []domain.Alert) []domain.Alert { alerts[0].UpdatedAt = time.Time{}; return alerts },
	} {
		if _, err := newDemoAlertProvider(demoAlertCache(t, fixture.Source()), mutate(fixture.Alerts())); err == nil {
			t.Fatal("unsafe demo alert accepted")
		}
	}
	live := fixture.Source()
	live.Provider, live.Synthetic = domain.ProviderDocker, false
	if _, err := newDemoAlertProvider(demoAlertCacheWithoutFixture(t, live), fixture.Alerts()); err == nil {
		t.Fatal("live source accepted fixed demo alerts")
	}
}

func demoAlertCacheWithoutFixture(t *testing.T, source domain.InfrastructureSource) *observation.CachedProvider {
	t.Helper()
	provider, err := observation.NewCachedProvider(observation.CachedProviderOptions{Source: source, Reader: demoAlertReader{},
		Resolver: observation.ProviderResolverFunc(func(context.Context, domain.SourceConfiguration) (observation.LiveProvider, error) {
			return nil, observation.ErrUnavailable
		})})
	if err != nil {
		t.Fatal(err)
	}
	return provider
}
