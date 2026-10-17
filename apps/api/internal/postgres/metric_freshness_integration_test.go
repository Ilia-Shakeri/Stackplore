package postgres

import (
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestCurrentMetricsDoNotRefreshOldSamplesFromFreshInventory(t *testing.T) {
	fixture := newObservationFixture(t)
	query := domain.CurrentMetricQuery{Source: fixture.token.Source, Resources: []domain.ResourceRef{fixture.batch.Resources[0].Resource.Ref}, Names: []domain.MetricName{domain.MetricCPUUsageCores}}
	// Synthetic time also advances explicitly: fresh inventory cannot refresh
	// the timestamp of a previously persisted usage sample.
	at := fixture.at.Add(10 * time.Minute)
	token, err := fixture.store.BeginCapture(fixture.ctx, query.Source.SourceID, 1, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.SaveCapture(fixture.ctx, domain.CaptureBatch{Token: token, Environments: fixture.batch.Environments, Resources: fixture.batch.Resources}); err != nil {
		t.Fatal(err)
	}
	completion := fixture.completion
	completion.Token = token
	completion.Status.ObservedAt = at
	for index := range completion.Capabilities.Capabilities {
		completion.Capabilities.Capabilities[index].Status.ObservedAt = at
	}
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.store.GetCurrentMetrics(fixture.ctx, query)
	if err != nil || len(result.Sets) != 1 || len(result.Sets[0].Samples) != 1 || result.Status.State != domain.AvailabilityStale || result.Sets[0].Status.State != domain.AvailabilityStale || result.Status.ReasonCode != "source.metric_stale" {
		t.Fatalf("old sample presented current: %#v %v", result, err)
	}
	if !result.Sets[0].Samples[0].SampledAt.Equal(fixture.batch.Metrics[0].SampledAt) {
		t.Fatal("old sample clock changed")
	}
	// A future-dated retained row must not shadow the last valid observation.
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO metric_samples(id,resource_id,metric_name,unit,value,sampled_at_us,interval_seconds,synthetic) VALUES ('metric_future_poison',$1,'cpu.usage.cores','cores',99,$2,5,1)`, string(query.Resources[0].ID), at.Add(time.Hour).UnixMicro()); err != nil {
		t.Fatal(err)
	}
	result, err = fixture.store.GetCurrentMetrics(fixture.ctx, query)
	if err != nil || len(result.Sets) != 1 || len(result.Sets[0].Samples) != 1 || result.Sets[0].Samples[0].Value == 99 {
		t.Fatalf("future sample shadowed valid history: %v", err)
	}
}
