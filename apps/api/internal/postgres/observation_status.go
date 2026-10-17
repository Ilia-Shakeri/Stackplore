package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (store *observationRead) GetObservationStatus(ctx context.Context, source domain.ProviderRef) (domain.ObservationStatus, error) {
	if !store.available() || ctx == nil || source.Validate() != nil {
		return domain.ObservationStatus{}, domain.ErrRepositoryInvalidInput
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	var availability, completeness, reason sql.NullString
	var observedAt sql.NullInt64
	var captureStarted, captureFinished sql.NullInt64
	var createdAt, enabled int64
	var cached, policyPending bool
	err := store.queries.QueryRow(ctx, `SELECT status.availability,status.completeness,status.reason_code,status.observed_at_us,
source.created_at_us,source.enabled,source.capture_started_at_us,source.capture_finished_at_us,source.operator_policy_pending,
(EXISTS(SELECT 1 FROM resources WHERE source_id=source.id LIMIT 1)
 OR EXISTS(SELECT 1 FROM environments WHERE source_id=source.id LIMIT 1))
FROM infrastructure_sources AS source LEFT JOIN source_status AS status ON status.source_id=source.id
WHERE source.id=$1 AND source.provider=$2 AND source.deleted_at_us IS NULL`, string(source.SourceID), string(source.Provider)).Scan(&availability, &completeness, &reason, &observedAt, &createdAt, &enabled, &captureStarted, &captureFinished, &policyPending, &cached)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ObservationStatus{}, domain.ErrRepositoryNotFound
	}
	if err != nil {
		return domain.ObservationStatus{}, mapRepositoryReadError(ctx, err)
	}
	status := domain.ObservationStatus{State: domain.AvailabilityUnavailable, Completeness: domain.CompletenessComplete, ObservedAt: time.UnixMicro(createdAt).UTC(), ReasonCode: "source.collection_pending"}
	if policyPending {
		return domain.ObservationStatus{State: domain.AvailabilityPermissionDenied, Completeness: domain.CompletenessComplete, ObservedAt: status.ObservedAt, ReasonCode: "source.profile_changed"}, nil
	}
	if availability.Valid && completeness.Valid && observedAt.Valid {
		status = domain.ObservationStatus{State: domain.AvailabilityState(availability.String), Completeness: domain.Completeness(completeness.String), ObservedAt: time.UnixMicro(observedAt.Int64).UTC(), ReasonCode: domain.ReasonCode(reason.String)}
	}
	if enabled == 0 {
		status.State = domain.AvailabilityUnavailable
		status.Completeness = domain.CompletenessComplete
		status.ReasonCode = "source.disabled"
	}
	if status.State == domain.AvailabilityUnavailable && cached {
		status.State = domain.AvailabilityStale
		if status.ReasonCode == "" {
			status.ReasonCode = "source.collection_unavailable"
		}
	}
	if captureStarted.Valid && !captureFinished.Valid && (status.AllowsData() || status.State == domain.AvailabilityUnavailable) {
		status.State = domain.AvailabilityUnavailable
		status.Completeness = domain.CompletenessComplete
		if cached {
			status.State = domain.AvailabilityStale
			status.Completeness = domain.CompletenessPartial
		}
		if enabled != 0 {
			status.ReasonCode = "source.collection_in_progress"
		}
		attemptTime := time.UnixMicro(captureStarted.Int64).UTC()
		if attemptTime.After(status.ObservedAt) {
			status.ObservedAt = attemptTime
		}
	}
	if status.Validate() != nil {
		return domain.ObservationStatus{}, domain.ErrRepositoryUnavailable
	}
	return status, nil
}

func (store *observationRead) GetCapabilities(ctx context.Context, source domain.ProviderRef) (domain.CapabilitySet, error) {
	status, err := store.GetObservationStatus(ctx, source)
	if err != nil {
		return domain.CapabilitySet{}, err
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	names := domain.CapabilityNames()
	rows, err := store.queries.Query(ctx, `SELECT capability_name,availability,completeness,observed_at_us,reason_code
FROM source_capabilities WHERE source_id=$1 LIMIT $2`, string(source.SourceID), len(names)+1)
	if err != nil {
		return domain.CapabilitySet{}, mapRepositoryReadError(ctx, err)
	}
	defer rows.Close()
	byName := make(map[domain.CapabilityName]domain.ObservationStatus, len(names))
	for rows.Next() {
		var name, availability, completeness string
		var observedAt int64
		var reason sql.NullString
		if err := rows.Scan(&name, &availability, &completeness, &observedAt, &reason); err != nil {
			return domain.CapabilitySet{}, domain.ErrRepositoryUnavailable
		}
		capability := domain.CapabilityName(name)
		item := domain.ObservationStatus{State: domain.AvailabilityState(availability), Completeness: domain.Completeness(completeness), ObservedAt: time.UnixMicro(observedAt).UTC(), ReasonCode: domain.ReasonCode(reason.String)}
		if capability.Validate() != nil || item.Validate() != nil {
			return domain.CapabilitySet{}, domain.ErrRepositoryUnavailable
		}
		byName[capability] = item
	}
	if err := rows.Err(); err != nil {
		return domain.CapabilitySet{}, mapRepositoryReadError(ctx, err)
	}
	rows.Close()
	localHistory := map[domain.CapabilityName]bool{}
	if status.AllowsData() {
		for _, name := range []domain.CapabilityName{domain.CapabilityMetricHistory, domain.CapabilityEventHistory} {
			cached, err := store.observationHistoryCached(ctx, source, name)
			if err != nil {
				return domain.CapabilitySet{}, err
			}
			localHistory[name] = cached
		}
	}
	initialPorts := map[domain.CapabilityName]bool{}
	if status.ReasonCode == "source.collection_in_progress" {
		initialPorts, err = store.observationInitialPorts(ctx, source)
		if err != nil {
			return domain.CapabilitySet{}, err
		}
	}
	result := domain.CapabilitySet{Source: source, Capabilities: make([]domain.CapabilityStatus, 0, len(names))}
	for _, name := range names {
		item, exists := byName[name]
		if !exists {
			item = domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: status.ObservedAt, ReasonCode: "source.capability_unavailable"}
			if initialPorts[name] {
				item = status
			}
		}
		if localHistory[name] {
			item = cachedHistoryStatus(item)
		}
		if !status.AllowsData() {
			item = status
		} else if status.State == domain.AvailabilityStale && item.AllowsData() {
			item.State = domain.AvailabilityStale
			item.ReasonCode = status.ReasonCode
			if status.Completeness == domain.CompletenessPartial {
				item.Completeness = domain.CompletenessPartial
			}
		}
		result.Capabilities = append(result.Capabilities, domain.CapabilityStatus{Name: name, Status: item})
	}
	if result.ValidateFor(source) != nil {
		return domain.CapabilitySet{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func scanObservationStatus(availability, completeness string, observedAt int64, reason sql.NullString) domain.ObservationStatus {
	return domain.ObservationStatus{State: domain.AvailabilityState(availability), Completeness: domain.Completeness(completeness), ObservedAt: time.UnixMicro(observedAt).UTC(), ReasonCode: domain.ReasonCode(reason.String)}
}

func (store *observationRead) observationCapabilityStatus(ctx context.Context, source domain.ProviderRef, name domain.CapabilityName, sourceStatus domain.ObservationStatus, cached bool) (domain.ObservationStatus, error) {
	if !sourceStatus.AllowsData() {
		return sourceStatus, nil
	}
	var availability, completeness string
	var observedAt int64
	var reason sql.NullString
	err := store.queries.QueryRow(ctx, `SELECT availability,completeness,observed_at_us,reason_code
FROM source_capabilities WHERE source_id=$1 AND capability_name=$2`, string(source.SourceID), string(name)).Scan(&availability, &completeness, &observedAt, &reason)
	status := domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: sourceStatus.ObservedAt, ReasonCode: "source.capability_unavailable"}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.ObservationStatus{}, mapRepositoryReadError(ctx, err)
	}
	if err == nil {
		status = scanObservationStatus(availability, completeness, observedAt, reason)
	} else if cached && sourceStatus.ReasonCode == "source.collection_in_progress" {
		status = sourceStatus
	}
	if sourceStatus.ObservedAt.After(status.ObservedAt) {
		status.ObservedAt = sourceStatus.ObservedAt
	}
	if name == domain.CapabilityMetricHistory || name == domain.CapabilityEventHistory {
		if !cached && status.State != domain.AvailabilityPermissionDenied {
			cached, err = store.observationHistoryCached(ctx, source, name)
			if err != nil {
				return domain.ObservationStatus{}, err
			}
		}
		if cached {
			status = cachedHistoryStatus(status)
		}
	}
	if cached && status.State == domain.AvailabilityUnavailable {
		status.State = domain.AvailabilityStale
	}
	if sourceStatus.State == domain.AvailabilityStale && status.AllowsData() {
		status.State = domain.AvailabilityStale
		status.ReasonCode = sourceStatus.ReasonCode
		if sourceStatus.Completeness == domain.CompletenessPartial {
			status.Completeness = domain.CompletenessPartial
		}
	}
	if (name == domain.CapabilityInventoryList || name == domain.CapabilityInventoryDetail) && sourceStatus.Completeness == domain.CompletenessPartial && status.AllowsData() {
		status.Completeness = domain.CompletenessPartial
		if status.ReasonCode == "" {
			status.ReasonCode = "source.collection_partial"
		}
	}
	if status.Validate() != nil {
		return domain.ObservationStatus{}, domain.ErrRepositoryUnavailable
	}
	return status, nil
}

func cachedHistoryStatus(status domain.ObservationStatus) domain.ObservationStatus {
	switch status.State {
	case domain.AvailabilityUnsupported:
		status.State = domain.AvailabilityAvailable
		if status.Completeness == domain.CompletenessComplete {
			status.ReasonCode = ""
		}
	case domain.AvailabilityUnavailable:
		status.State = domain.AvailabilityStale
		status.Completeness = domain.CompletenessPartial
	}
	return status
}

func (store *observationRead) observationHistoryCached(ctx context.Context, source domain.ProviderRef, name domain.CapabilityName) (bool, error) {
	statement := `SELECT EXISTS(SELECT 1 FROM infrastructure_events
WHERE source_id=$1 AND provider=$2 LIMIT 1)`
	if name == domain.CapabilityMetricHistory {
		statement = `SELECT EXISTS(SELECT 1 FROM resources AS resource
JOIN metric_samples AS sample ON sample.resource_id=resource.id
WHERE resource.source_id=$1 AND resource.provider=$2 LIMIT 1)`
	}
	var cached bool
	if err := store.queries.QueryRow(ctx, statement, string(source.SourceID), string(source.Provider)).Scan(&cached); err != nil {
		return false, mapRepositoryReadError(ctx, err)
	}
	return cached, nil
}

func (store *observationRead) observationInitialPorts(ctx context.Context, source domain.ProviderRef) (map[domain.CapabilityName]bool, error) {
	var environments, resources, health, topology, metrics bool
	err := store.queries.QueryRow(ctx, `WITH active_resources AS NOT MATERIALIZED (
SELECT resource.id FROM resources AS resource
JOIN infrastructure_sources AS source ON source.id=resource.source_id
JOIN resource_snapshots AS snapshot ON snapshot.resource_id=resource.id
WHERE resource.source_id=$1 AND resource.provider=$2 AND `+activeResourcePredicate+`)
SELECT EXISTS(SELECT 1 FROM environments AS environment JOIN infrastructure_sources AS source ON source.id=environment.source_id
WHERE environment.source_id=$1 AND environment.provider=$2 AND `+activeEnvironmentPredicate+` LIMIT 1),
EXISTS(SELECT 1 FROM active_resources LIMIT 1),
EXISTS(SELECT 1 FROM active_resources AS resource JOIN resource_health_current AS health ON health.resource_id=resource.id LIMIT 1),
EXISTS(SELECT 1 FROM relationships AS relationship JOIN infrastructure_sources AS source ON source.id=relationship.source_id
JOIN active_resources AS origin ON origin.id=relationship.from_resource_id JOIN active_resources AS target ON target.id=relationship.to_resource_id
WHERE relationship.source_id=$1 AND relationship.retired_at_us IS NULL
AND relationship.capture_generation>=source.relationship_completed_generation LIMIT 1),
EXISTS(SELECT 1 FROM active_resources AS resource JOIN metric_samples AS sample ON sample.resource_id=resource.id LIMIT 1)`, string(source.SourceID), string(source.Provider)).Scan(&environments, &resources, &health, &topology, &metrics)
	if err != nil {
		return nil, mapRepositoryReadError(ctx, err)
	}
	return map[domain.CapabilityName]bool{
		domain.CapabilityInventoryList:   environments || resources,
		domain.CapabilityInventoryDetail: resources,
		domain.CapabilityHealth:          health,
		domain.CapabilityTopology:        topology,
		domain.CapabilityMetricCurrent:   metrics,
	}, nil
}
