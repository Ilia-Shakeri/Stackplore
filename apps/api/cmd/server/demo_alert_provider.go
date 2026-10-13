package main

import (
	"slices"
	"strings"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/observation"
)

// demoAlertProvider keeps the fixed demo alert visible until durable demo alert
// seeding ships. Collection and all other reads remain on the cached provider.
type demoAlertProvider struct {
	*observation.CachedProvider
	alerts []domain.Alert
}

func newDemoAlertProvider(provider *observation.CachedProvider, alerts []domain.Alert) (*demoAlertProvider, error) {
	if provider == nil || provider.Source().Provider != domain.ProviderDemo || !provider.Source().Synthetic || len(alerts) > domain.MaxPageSize {
		return nil, errObservationStart
	}
	cloned := cloneDemoAlerts(alerts)
	slices.SortFunc(cloned, func(left, right domain.Alert) int {
		if comparison := right.UpdatedAt.Compare(left.UpdatedAt); comparison != 0 {
			return comparison
		}
		return strings.Compare(string(left.ID), string(right.ID))
	})
	page := domain.AlertPage{Alerts: cloned, Page: domain.PageInfo{Limit: domain.MaxPageSize}}
	if page.ValidateFor(provider.Source().ID, domain.PageRequest{Limit: domain.MaxPageSize}) != nil {
		return nil, errObservationStart
	}
	for _, alert := range cloned {
		if !alert.Synthetic {
			return nil, errObservationStart
		}
	}
	return &demoAlertProvider{CachedProvider: provider, alerts: cloned}, nil
}

func (provider *demoAlertProvider) Alerts() []domain.Alert { return cloneDemoAlerts(provider.alerts) }

func cloneDemoAlerts(alerts []domain.Alert) []domain.Alert {
	cloned := slices.Clone(alerts)
	for index := range cloned {
		item := &cloned[index]
		if item.EnvironmentID != nil {
			value := *item.EnvironmentID
			item.EnvironmentID = &value
		}
		if item.ResourceID != nil {
			value := *item.ResourceID
			item.ResourceID = &value
		}
		if item.AcknowledgedAt != nil {
			value := *item.AcknowledgedAt
			item.AcknowledgedAt = &value
		}
		if item.ResolvedAt != nil {
			value := *item.ResolvedAt
			item.ResolvedAt = &value
		}
	}
	return cloned
}
