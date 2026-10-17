package observation

import (
	"context"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// Only packaged synthetic fixtures may bootstrap history. Live providers keep
// their sampled PostgreSQL history and never fabricate earlier measurements.
func (worker *Worker) collectFixtureMetricHistory(ctx context.Context, provider LiveProvider, collected *collection) {
	if !provider.Source().Synthetic || provider.Source().Provider != domain.ProviderDemo {
		return
	}
	for _, capability := range collected.capabilities.Capabilities {
		if (capability.Name == domain.CapabilityMetricCurrent || capability.Name == domain.CapabilityMetricHistory) &&
			(!capability.Status.AllowsData() || capability.Status.Completeness != domain.CompletenessComplete) {
			return
		}
	}
	history, ok := provider.(domain.MetricHistoryProvider)
	if !ok {
		return
	}
	seen := make(map[domain.MetricSampleID]struct{}, len(collected.metrics))
	for _, sample := range collected.metrics {
		seen[sample.ID] = struct{}{}
	}
	names := []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes, domain.MetricNetworkReceiveBytesTotal, domain.MetricNetworkSendBytesTotal, domain.MetricRestartCount, domain.MetricCPURequestCores, domain.MetricCPULimitCores, domain.MetricMemoryRequestBytes, domain.MetricMemoryLimitBytes}
	end := collected.inventory.Status.ObservedAt.Add(time.Microsecond)
	for _, resource := range collected.inventory.Resources {
		query := domain.MetricHistoryQuery{Source: collected.inventory.Source, Resources: []domain.ResourceRef{resource.Resource.Ref}, Names: names, Range: domain.TimeRange{Start: end.Add(-time.Hour), End: end}, PointLimit: domain.MaxMetricHistoryPoints}
		result, err := history.GetMetricHistory(ctx, query)
		if err != nil || result.ValidateFor(query) != nil || !result.Status.AllowsData() || result.Status.Completeness != domain.CompletenessComplete {
			setCapability(&collected.capabilities, domain.CapabilityMetricHistory, partialStatus(collected.inventory.Status, "observation.fixture_history_partial"))
			return
		}
		for _, sample := range result.Samples {
			if _, exists := seen[sample.ID]; exists {
				continue
			}
			if len(collected.metrics) >= worker.maximumMetricSamples {
				setCapability(&collected.capabilities, domain.CapabilityMetricHistory, partialStatus(result.Status, "observation.metric_bound"))
				return
			}
			seen[sample.ID] = struct{}{}
			collected.metrics = append(collected.metrics, sample)
		}
	}
}
