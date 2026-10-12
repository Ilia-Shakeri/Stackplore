package domain

import (
	"context"
	"errors"
	"time"
)

// MaxRepositoryWriteBatch bounds each persistence transaction so ingestion
// cannot hold the durable writer for an unbounded amount of time.
const MaxRepositoryWriteBatch = 100

var (
	ErrRepositoryInvalidInput = errors.New("repository input is invalid")
	ErrRepositoryConflict     = errors.New("repository write conflicts with stored state")
	ErrRepositoryNotFound     = errors.New("repository record was not found")
	ErrRepositoryUnavailable  = errors.New("repository is unavailable")
)

// MetricSampleRepository stores immutable metric history. A batch is atomic.
type MetricSampleRepository interface {
	InsertMetricSamples(context.Context, []MetricSample) error
}

// InfrastructureEventRepository stores immutable event history. A batch is atomic.
type InfrastructureEventRepository interface {
	InsertInfrastructureEvents(context.Context, []InfrastructureEvent) error
}

// AuditEntryRepository appends immutable audit history. A batch is atomic.
type AuditEntryRepository interface {
	AppendAuditEntries(context.Context, []AuditEntry) error
}

type AuditEntryReader interface {
	ListAuditEntries(context.Context, PageRequest) (AuditPage, error)
}

type AlertReader interface {
	ListAlerts(context.Context, SourceID, PageRequest) (AlertPage, error)
}

type AlertMutationRepository interface {
	SaveAlertMutation(context.Context, AlertMutation) error
}

type NotificationReader interface {
	ListNotifications(context.Context, PageRequest) (NotificationPage, error)
}

// Repositories is the narrow set available inside a unit of work.
type Repositories interface {
	MetricSampleRepository
	InfrastructureEventRepository
	AuditEntryRepository
}

type ProductRepositories interface {
	Repositories
	AlertMutationRepository
}

// RepositoryUnitOfWork atomically combines changes from multiple repositories.
// Any repository error aborts the unit, even when its callback ignores the error.
type RepositoryUnitOfWork interface {
	WithinUnitOfWork(context.Context, func(Repositories) error) error
}

type ProductRepositoryUnitOfWork interface {
	WithinProductUnitOfWork(context.Context, func(ProductRepositories) error) error
}

// ValidateMetricSampleWriteBatch enforces the repository input contract without
// exposing model validation details outside the domain package.
func ValidateMetricSampleWriteBatch(samples []MetricSample) error {
	if !validRepositoryBatchSize(len(samples)) {
		return ErrRepositoryInvalidInput
	}
	seen := make(map[MetricSampleID]struct{}, len(samples))
	for _, sample := range samples {
		if err := sample.Validate(); err != nil || !repositoryTimeSupported(sample.SampledAt) {
			return ErrRepositoryInvalidInput
		}
		if _, exists := seen[sample.ID]; exists {
			return ErrRepositoryConflict
		}
		seen[sample.ID] = struct{}{}
	}
	return nil
}

// ValidateInfrastructureEventWriteBatch enforces the repository input contract.
func ValidateInfrastructureEventWriteBatch(events []InfrastructureEvent) error {
	if !validRepositoryBatchSize(len(events)) {
		return ErrRepositoryInvalidInput
	}
	seen := make(map[EventID]struct{}, len(events))
	for _, event := range events {
		if err := event.Validate(); err != nil ||
			!repositoryTimeSupported(event.OccurredAt) ||
			!repositoryTimeSupported(event.ObservedAt) ||
			event.Synthetic != (event.Provider == ProviderDemo) {
			return ErrRepositoryInvalidInput
		}
		if _, exists := seen[event.ID]; exists {
			return ErrRepositoryConflict
		}
		seen[event.ID] = struct{}{}
	}
	return nil
}

// ValidateAuditEntryWriteBatch enforces the repository input contract.
func ValidateAuditEntryWriteBatch(entries []AuditEntry) error {
	if !validRepositoryBatchSize(len(entries)) {
		return ErrRepositoryInvalidInput
	}
	seen := make(map[AuditEntryID]struct{}, len(entries))
	for _, entry := range entries {
		if err := entry.Validate(); err != nil || !repositoryTimeSupported(entry.OccurredAt) {
			return ErrRepositoryInvalidInput
		}
		if _, exists := seen[entry.ID]; exists {
			return ErrRepositoryConflict
		}
		seen[entry.ID] = struct{}{}
	}
	return nil
}

func validRepositoryBatchSize(size int) bool {
	return size >= 1 && size <= MaxRepositoryWriteBatch
}

func repositoryTimeSupported(value time.Time) bool {
	return !value.Before(time.Unix(0, 0).UTC()) && value.Nanosecond()%int(time.Microsecond) == 0
}
