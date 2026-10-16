package alerting

import (
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestMetricWindowUsesWholeWeightedWindow(t *testing.T) {
	at := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	window := domain.TimeRange{Start: at.Add(-time.Minute), End: at}
	samples := metricSamples(window.Start, domain.MetricCPUUsageCores, 30*time.Second, 1, 0)
	value, err := metricWindowValue(samples, window, 90*time.Second, false)
	if err != nil || value != .5 {
		t.Fatalf("weighted window = %v, %v; want .5", value, err)
	}
	// Unequal sample spacing must have unequal weight.
	samples[1].SampledAt = window.Start.Add(15 * time.Second)
	value, err = metricWindowValue(samples, window, 90*time.Second, false)
	if err != nil || value != .25 {
		t.Fatalf("weighted uneven window = %v, %v; want .25", value, err)
	}
}

func TestRestartWindowCountsIncreasesAndResets(t *testing.T) {
	at := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	window := domain.TimeRange{Start: at.Add(-2 * time.Minute), End: at}
	samples := metricSamples(window.Start, domain.MetricRestartCount, 30*time.Second, 100, 100, 102, 0, 1)
	value, err := metricWindowValue(samples, window, 90*time.Second, true)
	if err != nil || value != 3 {
		t.Fatalf("restart increase = %v, %v; want 3", value, err)
	}
	samples = metricSamples(window.Start, domain.MetricRestartCount, time.Minute, 100, 100, 100)
	value, err = metricWindowValue(samples, window, 90*time.Second, true)
	if err != nil || value != 0 {
		t.Fatalf("unchanged old count = %v, %v; want 0", value, err)
	}
	samples = metricSamples(window.Start, domain.MetricRestartCount, time.Minute, 100, 2, 4)
	value, err = metricWindowValue(samples, window, 90*time.Second, true)
	if err != nil || value != 4 {
		t.Fatalf("reset increase = %v, %v; want 4", value, err)
	}
}

func TestMetricWindowRejectsMissingOldFutureAndDuplicateEvidence(t *testing.T) {
	at := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	window := domain.TimeRange{Start: at.Add(-time.Minute), End: at}
	tests := map[string][]domain.MetricSample{
		"empty":        nil,
		"no baseline":  metricSamples(window.Start.Add(time.Second), domain.MetricCPUUsageCores, time.Second, 1),
		"old baseline": metricSamples(window.Start.Add(-2*time.Minute), domain.MetricCPUUsageCores, time.Minute, 1),
		"old last":     metricSamples(window.Start, domain.MetricCPUUsageCores, time.Minute, 1),
		"future":       metricSamples(window.Start, domain.MetricCPUUsageCores, 61*time.Second, 1, 2),
		"same time":    metricSamples(window.Start, domain.MetricCPUUsageCores, 0, 1, 2),
		"gap":          metricSamples(window.Start, domain.MetricCPUUsageCores, time.Minute, 1, 2),
	}
	for name, samples := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := metricWindowValue(samples, window, 45*time.Second, false); err == nil {
				t.Fatal("incomplete evidence produced a value")
			}
		})
	}
}

func metricSamples(start time.Time, name domain.MetricName, interval time.Duration, values ...float64) []domain.MetricSample {
	unit := domain.MetricUnitCores
	if name == domain.MetricRestartCount {
		unit = domain.MetricUnitCount
	}
	if name == domain.MetricMemoryUsageBytes {
		unit = domain.MetricUnitBytes
	}
	samples := make([]domain.MetricSample, len(values))
	for index, value := range values {
		samples[index] = domain.MetricSample{
			ID: domain.MetricSampleID("sample_" + string(rune('a'+index))), ResourceID: *fixtureRule(0).ResourceID,
			Name: name, Unit: unit, Value: value, SampledAt: start.Add(time.Duration(index) * interval), Synthetic: true,
		}
	}
	return samples
}
