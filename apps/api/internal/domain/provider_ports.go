package domain

import "context"

// Page-returning providers bind each opaque cursor to the normalized query and
// move it forward in the fixed result order. Page validation catches immediate
// cursor reuse; the provider owns full multi-page progress.
type InventoryProvider interface {
	ListEnvironments(context.Context, EnvironmentQuery) (EnvironmentPage, error)
	ListResources(context.Context, InventoryQuery) (ResourcePage, error)
	GetResource(context.Context, ResourceDetailQuery) (ResourceSnapshot, error)
}

type HealthProvider interface {
	GetHealth(context.Context, HealthQuery) (HealthResult, error)
}

type WorkloadReplicaProvider interface {
	GetWorkloadReplicas(context.Context, ResourceDetailQuery) (WorkloadReplicaSnapshot, error)
}

type TopologyProvider interface {
	GetRelationships(context.Context, TopologyQuery) (RelationshipPage, error)
}

type LogReadProvider interface {
	ReadLogs(context.Context, LogReadQuery) (LogPage, error)
}

// LogTail must unblock Next when its context is canceled. Close must unblock a
// pending Next call and remain safe when called more than once.
type LogTail interface {
	Next(context.Context) (LogRecord, error)
	Close() error
}

type LogTailProvider interface {
	TailLogs(context.Context, LogTailQuery) (LogTail, error)
}

type CurrentMetricProvider interface {
	GetCurrentMetrics(context.Context, CurrentMetricQuery) (CurrentMetricResult, error)
}

type MetricHistoryProvider interface {
	GetMetricHistory(context.Context, MetricHistoryQuery) (MetricHistoryResult, error)
}

type EventHistoryProvider interface {
	ListEvents(context.Context, EventQuery) (EventPage, error)
}

// EventWatch is at-least-once and has no cross-resource order promise. Consumers
// deduplicate by event ID. It must unblock Next when its context is canceled.
// Close must unblock a pending Next call and remain safe when called more than
// once.
type EventWatch interface {
	Next(context.Context) (InfrastructureEvent, error)
	Close() error
}

type EventWatchProvider interface {
	WatchEvents(context.Context, EventWatchQuery) (EventWatch, error)
}

type CapabilityProvider interface {
	GetCapabilities(context.Context, ProviderRef) (CapabilitySet, error)
}

type OperationProvider interface {
	PreviewOperation(context.Context, OperationRequest) (OperationImpact, error)
	ExecuteOperation(context.Context, OperationExecution) (OperationProviderResult, error)
}
