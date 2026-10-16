package domain

import (
	"context"
	"time"
)

const MaximumCollectionHistory = 4096
const MaximumAlertMetricHistory = 4096

type AlertMetricHistoryQuery struct {
	Source   ProviderRef
	Resource ResourceRef
	Name     MetricName
	Range    TimeRange
	Limit    int
}

func (query AlertMetricHistoryQuery) Validate() error {
	if query.Source.Validate() != nil || query.Resource.Validate() != nil || !query.Source.Matches(query.Resource) || query.Name.Validate() != nil || query.Range.Validate() != nil || query.Limit < 1 || query.Limit > MaximumAlertMetricHistory {
		return ErrRepositoryInvalidInput
	}
	return nil
}

type AlertMetricHistoryProvider interface {
	GetAlertMetricHistory(context.Context, AlertMetricHistoryQuery) (MetricHistoryResult, error)
}

type WorkloadReplicaHistoryQuery struct {
	Source   ProviderRef
	Resource ResourceRef
	Range    TimeRange
	Limit    int
}

func (query WorkloadReplicaHistoryQuery) Validate() error {
	if query.Source.Validate() != nil || query.Resource.Validate() != nil || !query.Source.Matches(query.Resource) || query.Range.Validate() != nil || query.Limit < 1 || query.Limit > MaximumAlertMetricHistory {
		return ErrRepositoryInvalidInput
	}
	return nil
}

type WorkloadReplicaHistoryResult struct {
	Status  ObservationStatus
	Samples []WorkloadReplicaSnapshot
}

type WorkloadReplicaHistoryProvider interface {
	GetWorkloadReplicaHistory(context.Context, WorkloadReplicaHistoryQuery) (WorkloadReplicaHistoryResult, error)
}

// SourceCollectionOutcome records collection health independently from cached reads.
type SourceCollectionOutcome struct {
	CollectedAt  time.Time
	Status       ObservationStatus
	MetricStatus ObservationStatus
	EventStatus  ObservationStatus
}

type SourceCollectionHistory struct {
	Observations []SourceCollectionOutcome
	Complete     bool
}

type SourceCollectionReader interface {
	GetSourceCollectionStatus(context.Context, ProviderRef) (SourceCollectionOutcome, error)
	ListSourceCollectionHistory(context.Context, ProviderRef, TimeRange, int) (SourceCollectionHistory, error)
}

type AlertChangeKind string

const (
	AlertChangeAlert       AlertChangeKind = "alert.changed"
	AlertChangeRuleCreated AlertChangeKind = "rule.created"
	AlertChangeRuleChanged AlertChangeKind = "rule.changed"
	AlertChangeRuleDeleted AlertChangeKind = "rule.deleted"
)

type AlertChange struct {
	ID         int64
	SourceID   SourceID
	RuleID     AlertRuleID
	AlertID    *AlertID
	Kind       AlertChangeKind
	OccurredAt time.Time
}

type AlertChangePage struct {
	Changes []AlertChange
	NextID  int64
	Reset   bool
}

type AlertChangeReader interface {
	AlertChangePosition(context.Context, SourceID) (int64, error)
	ListAlertChanges(context.Context, SourceID, int64, int) (AlertChangePage, error)
}

type AlertHistoryRetentionRepository interface {
	PruneAlertHistory(context.Context, time.Time, int) (int64, error)
}

// DemoAlertSeed is versioned fixture input, never an operator-controlled payload.
type DemoAlertSeed struct {
	FixtureVersion string
	Rule           AlertRule
	Mutation       AlertMutation
}

func (seed DemoAlertSeed) Validate() error {
	if validateToken("fixture version", seed.FixtureVersion, maxIDBytes) != nil || seed.Rule.Validate() != nil ||
		seed.Mutation.Validate() != nil || !seed.Mutation.Alert.Synthetic || seed.Rule.ID != seed.Mutation.Alert.RuleID ||
		seed.Rule.SourceID != seed.Mutation.Alert.SourceID || seed.Mutation.ExpectedUpdatedAt != nil || seed.Mutation.ExpectedState != nil {
		return ErrRepositoryInvalidInput
	}
	return nil
}

type DemoAlertSeedRepository interface {
	EnsureDemoAlerts(context.Context, DemoAlertSeed) error
}
