package domain

import (
	"context"
)

const MaximumHealthHistoryPoints = 200

type HealthHistoryQuery struct {
	Source   ProviderRef
	Resource ResourceRef
	Range    TimeRange
	Limit    int
}

func (query HealthHistoryQuery) Validate() error {
	if query.Source.Validate() != nil || query.Resource.Validate() != nil || !query.Source.Matches(query.Resource) || query.Range.ValidateLookback(MaxMetricLookback) != nil || query.Limit < 1 || query.Limit > MaximumHealthHistoryPoints {
		return ErrRepositoryInvalidInput
	}
	return nil
}

type HealthHistorySample struct {
	ID         string
	ResourceID ResourceID
	State      HealthState
	Status     ObservationStatus
	Synthetic  bool
}
type HealthHistoryResult struct {
	Status  ObservationStatus
	Samples []HealthHistorySample
}

func (result HealthHistoryResult) ValidateFor(query HealthHistoryQuery) error {
	if query.Validate() != nil || result.Status.Validate() != nil || len(result.Samples) > query.Limit || !result.Status.AllowsData() && len(result.Samples) > 0 {
		return ErrRepositoryInvalidInput
	}
	seen := map[string]bool{}
	for index, sample := range result.Samples {
		if validateToken("health sample", sample.ID, maxIDBytes) != nil || sample.ResourceID != query.Resource.ID || sample.State.Validate() != nil || sample.Status.Validate() != nil || sample.Synthetic != (query.Source.Provider == ProviderDemo) || sample.Status.ObservedAt.Before(query.Range.Start) || !sample.Status.ObservedAt.Before(query.Range.End) || sample.Status.ObservedAt.After(result.Status.ObservedAt) || seen[sample.ID] || (!sample.Status.AllowsData() || sample.Status.Completeness == CompletenessPartial) && sample.State != HealthUnknown {
			return ErrRepositoryInvalidInput
		}
		if index > 0 {
			previous := result.Samples[index-1]
			if previous.Status.ObservedAt.Before(sample.Status.ObservedAt) || previous.Status.ObservedAt.Equal(sample.Status.ObservedAt) && previous.ID > sample.ID {
				return ErrRepositoryInvalidInput
			}
		}
		seen[sample.ID] = true
	}
	return nil
}

type HealthHistoryProvider interface {
	GetHealthHistory(context.Context, HealthHistoryQuery) (HealthHistoryResult, error)
}
