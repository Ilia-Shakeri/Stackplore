package main

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// These lifecycle doubles never claim to exercise observation SQL behavior.
func (*fakeDatabase) EnsureSourceConfigurations(context.Context, []domain.SourceConfiguration) error {
	return nil
}
func (*fakeDatabase) ListSourceConfigurations(_ context.Context, page domain.PageRequest) (domain.SourceConfigurationPage, error) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	configuration := domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_demo", Provider: domain.ProviderDemo, Name: "Demo", Synthetic: true},
		ProfileID: domain.SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: at, UpdatedAt: at}
	return domain.SourceConfigurationPage{Configurations: []domain.SourceConfiguration{configuration}, Page: domain.PageInfo{Limit: page.Limit}}, nil
}
func (*fakeDatabase) BeginCapture(context.Context, domain.SourceID, int64, time.Time) (domain.CaptureToken, error) {
	return domain.CaptureToken{}, domain.ErrRepositoryConflict
}
func (*fakeDatabase) PruneObservationHistory(context.Context, domain.ObservationRetentionPolicy, int) (int64, error) {
	return 0, nil
}
func (*fakeDatabase) GetSourceCollectionStatus(context.Context, domain.ProviderRef) (domain.SourceCollectionOutcome, error) {
	return domain.SourceCollectionOutcome{}, domain.ErrRepositoryNotFound
}
func (*fakeDatabase) ListSourceCollectionHistory(context.Context, domain.ProviderRef, domain.TimeRange, int) (domain.SourceCollectionHistory, error) {
	return domain.SourceCollectionHistory{}, domain.ErrRepositoryNotFound
}
func (*fakeDatabase) EnsureDemoAlerts(context.Context, domain.DemoAlertSeed) error     { return nil }
func (*fakeDatabase) PruneAlertHistory(context.Context, time.Time, int) (int64, error) { return 0, nil }

type databaseWithoutObservations struct{ databaseRuntime }

func TestRunRefusesMissingObservationRepositoryBeforeListen(t *testing.T) {
	database := &fakeDatabase{}
	listened := false
	err := runWithStorage(context.Background(), defaultSettings(t), discardLogger(t), func(string, string) (net.Listener, error) {
		listened = true
		return nil, errors.New("unexpected listener")
	},
		func(context.Context, config.Config) (databaseRuntime, error) {
			return databaseWithoutObservations{database}, nil
		})
	if !errors.Is(err, errObservationStart) || listened || !database.closed.Load() {
		t.Fatal("missing durable ports did not close startup")
	}
}
