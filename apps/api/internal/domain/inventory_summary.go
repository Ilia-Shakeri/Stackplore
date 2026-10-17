package domain

import "context"

// InventorySummary counts the complete active snapshot, including resources
// outside any transport page. Unknown health remains distinct from healthy.
type InventorySummary struct {
	Status    ObservationStatus
	Resources int64
	Healthy   int64
	Degraded  int64
	Unhealthy int64
	Unknown   int64
}

func (summary InventorySummary) Validate() error {
	if summary.Status.Validate() != nil || summary.Resources < 0 || summary.Resources > 9007199254740991 || summary.Healthy < 0 || summary.Degraded < 0 || summary.Unhealthy < 0 || summary.Unknown < 0 || summary.Healthy > summary.Resources || summary.Degraded > summary.Resources || summary.Unhealthy > summary.Resources || summary.Unknown > summary.Resources || summary.Healthy+summary.Degraded+summary.Unhealthy+summary.Unknown != summary.Resources || !summary.Status.AllowsData() && summary.Resources != 0 {
		return ErrRepositoryInvalidInput
	}
	return nil
}

type InventorySummaryProvider interface {
	GetInventorySummary(context.Context, ProviderRef) (InventorySummary, error)
}
