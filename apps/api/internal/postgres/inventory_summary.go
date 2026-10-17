package postgres

import (
	"context"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (store *Store) GetInventorySummary(ctx context.Context, source domain.ProviderRef) (domain.InventorySummary, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.InventorySummary, error) {
		return reader.GetInventorySummary(ctx, source)
	})
}

const inventorySummarySQL = `SELECT count(*),
count(*) FILTER (WHERE health_state='healthy'),count(*) FILTER (WHERE health_state='degraded'),
count(*) FILTER (WHERE health_state='unhealthy'),count(*) FILTER (WHERE health_state='unknown'),
COALESCE(bool_or(NOT complete_health),false)
FROM (SELECT ` + resourceHealthExpression + ` AS health_state,
EXISTS(SELECT 1 FROM resource_health_current health WHERE health.resource_id=resource.id AND health.availability='available' AND health.completeness='complete') AS complete_health` + activeResourceJoin + `) counts`

func (reader *observationRead) GetInventorySummary(ctx context.Context, source domain.ProviderRef) (domain.InventorySummary, error) {
	if ctx == nil || source.Validate() != nil {
		return domain.InventorySummary{}, domain.ErrRepositoryInvalidInput
	}
	status, err := reader.GetObservationStatus(ctx, source)
	if err != nil {
		return domain.InventorySummary{}, err
	}
	result := domain.InventorySummary{Status: status}
	if !status.AllowsData() {
		return result, nil
	}
	ctx, cancel := reader.queryContext(ctx, reader.queryTimeout)
	defer cancel()
	var incompleteHealth bool
	err = reader.queries.QueryRow(ctx, inventorySummarySQL, string(source.SourceID), string(source.Provider)).Scan(&result.Resources, &result.Healthy, &result.Degraded, &result.Unhealthy, &result.Unknown, &incompleteHealth)
	if err != nil {
		return domain.InventorySummary{}, mapRepositoryReadError(ctx, err)
	}
	result.Status, err = reader.observationCapabilityStatus(ctx, source, domain.CapabilityInventoryList, status, result.Resources > 0)
	if err != nil {
		return domain.InventorySummary{}, err
	}
	if !result.Status.AllowsData() {
		return domain.InventorySummary{Status: result.Status}, nil
	}
	healthStatus, err := reader.observationCapabilityStatus(ctx, source, domain.CapabilityHealth, status, result.Resources > 0)
	if err != nil {
		return domain.InventorySummary{}, err
	}
	if !healthStatus.AllowsData() {
		result.Healthy = 0
		result.Degraded = 0
		result.Unhealthy = 0
		result.Unknown = result.Resources
	}
	incompleteHealth = incompleteHealth || result.Resources > 0 && (healthStatus.State != domain.AvailabilityAvailable || healthStatus.Completeness != domain.CompletenessComplete)
	if incompleteHealth {
		result.Status.Completeness = domain.CompletenessPartial
		if result.Status.ReasonCode == "" {
			result.Status.ReasonCode = "source.health_partial"
		}
	}
	if result.Validate() != nil {
		return domain.InventorySummary{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}
