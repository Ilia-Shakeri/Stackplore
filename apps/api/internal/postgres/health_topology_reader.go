package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func resourceIDs(resources []domain.ResourceRef) []string {
	ids := make([]string, 0, len(resources))
	for _, resource := range resources {
		ids = append(ids, string(resource.ID))
	}
	return ids
}

func (store *observationRead) GetHealth(ctx context.Context, query domain.HealthQuery) (domain.HealthResult, error) {
	if ctx == nil || query.Validate() != nil {
		return domain.HealthResult{}, domain.ErrRepositoryInvalidInput
	}
	status, err := store.GetObservationStatus(ctx, query.Source)
	if err != nil {
		return domain.HealthResult{}, err
	}
	result := domain.HealthResult{Status: status, Snapshots: []domain.HealthSnapshot{}}
	if !status.AllowsData() {
		return result, nil
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	ids := resourceIDs(query.Resources)
	rows, err := store.queries.Query(ctx, `SELECT health.resource_id,health.health_state,health.availability,health.completeness,
health.observed_at_us,health.reason_code FROM resource_health_current AS health
JOIN resources AS resource ON resource.id=health.resource_id
JOIN infrastructure_sources AS source ON source.id=resource.source_id
WHERE resource.source_id=$1 AND resource.provider=$2 AND resource.id=ANY($3::text[])
AND resource.retired_at_us IS NULL AND resource.capture_generation>=source.completed_generation LIMIT $4`, string(query.Source.SourceID), string(query.Source.Provider), ids, len(ids))
	if err != nil {
		return domain.HealthResult{}, mapRepositoryReadError(ctx, err)
	}
	byID := make(map[domain.ResourceID]domain.HealthSnapshot, len(ids))
	for rows.Next() {
		var snapshot domain.HealthSnapshot
		var state, availability, completeness string
		var observedAt int64
		var reason sql.NullString
		if rows.Scan(&snapshot.ResourceID, &state, &availability, &completeness, &observedAt, &reason) != nil {
			rows.Close()
			return domain.HealthResult{}, domain.ErrRepositoryUnavailable
		}
		snapshot.State = domain.HealthState(state)
		snapshot.Status = scanObservationStatus(availability, completeness, observedAt, reason)
		snapshot.Evidence = []domain.HealthEvidence{}
		byID[snapshot.ResourceID] = snapshot
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.HealthResult{}, mapRepositoryReadError(ctx, err)
	}
	rows.Close()
	rows, err = store.queries.Query(ctx, `SELECT resource_id,evidence_id,code,source_code,status,observed_at_us,synthetic
FROM health_evidence WHERE resource_id=ANY($1::text[]) ORDER BY resource_id,evidence_id LIMIT $2`, ids, len(ids)*domain.MaxHealthEvidence+1)
	if err != nil {
		return domain.HealthResult{}, mapRepositoryReadError(ctx, err)
	}
	defer rows.Close()
	for rows.Next() {
		var resourceID domain.ResourceID
		var evidence domain.HealthEvidence
		var code, sourceCode, evidenceStatus string
		var observedAt, synthetic int64
		if rows.Scan(&resourceID, &evidence.ID, &code, &sourceCode, &evidenceStatus, &observedAt, &synthetic) != nil {
			return domain.HealthResult{}, domain.ErrRepositoryUnavailable
		}
		snapshot, exists := byID[resourceID]
		if !exists {
			continue
		}
		evidence.Code, evidence.SourceCode, evidence.Status = domain.ReasonCode(code), domain.ReasonCode(sourceCode), domain.EvidenceStatus(evidenceStatus)
		evidence.ObservedAt = time.UnixMicro(observedAt).UTC()
		evidence.Synthetic = synthetic == 1
		snapshot.Evidence = append(snapshot.Evidence, evidence)
		if len(snapshot.Evidence) > domain.MaxHealthEvidence {
			return domain.HealthResult{}, domain.ErrRepositoryUnavailable
		}
		byID[resourceID] = snapshot
	}
	if err := rows.Err(); err != nil {
		return domain.HealthResult{}, mapRepositoryReadError(ctx, err)
	}
	rows.Close()
	status, err = store.observationCapabilityStatus(ctx, query.Source, domain.CapabilityHealth, status, len(byID) > 0)
	if err != nil {
		return domain.HealthResult{}, err
	}
	result.Status = status
	if !status.AllowsData() {
		return result, nil
	}
	for _, resource := range query.Resources {
		snapshot, exists := byID[resource.ID]
		if !exists {
			snapshot = domain.HealthSnapshot{ResourceID: resource.ID, State: domain.HealthUnknown, Evidence: []domain.HealthEvidence{},
				Status: domain.ObservationStatus{State: domain.AvailabilityUnavailable, Completeness: domain.CompletenessComplete, ObservedAt: status.ObservedAt, ReasonCode: "source.health_unavailable"}}
		}
		if status.State == domain.AvailabilityStale && snapshot.Status.AllowsData() {
			snapshot.Status.State = domain.AvailabilityStale
			snapshot.Status.ReasonCode = status.ReasonCode
		}
		result.Snapshots = append(result.Snapshots, snapshot)
	}
	if result.ValidateFor(query) != nil {
		return domain.HealthResult{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func (store *observationRead) GetRelationships(ctx context.Context, query domain.TopologyQuery) (domain.RelationshipPage, error) {
	if ctx == nil || query.Validate() != nil {
		return domain.RelationshipPage{}, domain.ErrRepositoryInvalidInput
	}
	status, err := store.GetObservationStatus(ctx, query.Source)
	if err != nil {
		return domain.RelationshipPage{}, err
	}
	result := domain.RelationshipPage{Status: status, Relationships: []domain.Relationship{}, Page: domain.PageInfo{Limit: query.Page.Limit}}
	if !status.AllowsData() {
		return result, nil
	}
	queryBinding := query
	queryBinding.Page = domain.PageRequest{}
	queryHash := observationQueryHash(queryBinding)
	cursor, err := store.decodeObservationCursor(query.Page.Cursor, "relationships", queryHash)
	if err != nil {
		return domain.RelationshipPage{}, err
	}
	after := ""
	if cursor != nil {
		after = cursor.ID
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	ids := resourceIDs(query.Resources)
	rows, err := store.queries.Query(ctx, `SELECT relationship.id,relationship.from_resource_id,relationship.to_resource_id,
relationship.kind,relationship.direction,relationship.observed_at_us
FROM relationships AS relationship JOIN infrastructure_sources AS source ON source.id=relationship.source_id
JOIN resources AS origin ON origin.id=relationship.from_resource_id
JOIN resources AS target ON target.id=relationship.to_resource_id
WHERE relationship.source_id=$1 AND source.provider=$2 AND relationship.retired_at_us IS NULL
AND relationship.capture_generation>=source.relationship_completed_generation
AND origin.retired_at_us IS NULL AND origin.capture_generation>=source.completed_generation
AND target.retired_at_us IS NULL AND target.capture_generation>=source.completed_generation
AND (cardinality($3::text[])=0 OR relationship.from_resource_id=ANY($3::text[]) OR relationship.to_resource_id=ANY($3::text[]))
AND relationship.id COLLATE "C">$4 COLLATE "C" ORDER BY relationship.id COLLATE "C" LIMIT $5`, string(query.Source.SourceID), string(query.Source.Provider), ids, after, query.Page.Limit+1)
	if err != nil {
		return domain.RelationshipPage{}, mapRepositoryReadError(ctx, err)
	}
	byID := make(map[domain.RelationshipID]int, query.Page.Limit+1)
	relationshipIDs := make([]string, 0, query.Page.Limit+1)
	for rows.Next() {
		var relationship domain.Relationship
		var kind, direction string
		var observedAt int64
		if rows.Scan(&relationship.ID, &relationship.From, &relationship.To, &kind, &direction, &observedAt) != nil {
			rows.Close()
			return domain.RelationshipPage{}, domain.ErrRepositoryUnavailable
		}
		relationship.Kind, relationship.Direction = domain.RelationshipKind(kind), domain.RelationshipDirection(direction)
		relationship.ObservedAt = time.UnixMicro(observedAt).UTC()
		relationship.Evidence = []domain.RelationshipEvidence{}
		byID[relationship.ID] = len(result.Relationships)
		relationshipIDs = append(relationshipIDs, string(relationship.ID))
		result.Relationships = append(result.Relationships, relationship)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.RelationshipPage{}, mapRepositoryReadError(ctx, err)
	}
	rows.Close()
	if len(relationshipIDs) > 0 {
		rows, err = store.queries.Query(ctx, `SELECT relationship_id,evidence_id,provider,source_id,provenance,certainty,confidence,observed_at_us,synthetic
FROM relationship_evidence WHERE relationship_id=ANY($1::text[]) ORDER BY relationship_id,evidence_id LIMIT $2`, relationshipIDs, len(relationshipIDs)*domain.MaxRelationshipEvidence+1)
		if err != nil {
			return domain.RelationshipPage{}, mapRepositoryReadError(ctx, err)
		}
		defer rows.Close()
		for rows.Next() {
			var relationshipID domain.RelationshipID
			var evidence domain.RelationshipEvidence
			var provider, provenance, certainty string
			var observedAt, synthetic int64
			if rows.Scan(&relationshipID, &evidence.ID, &provider, &evidence.SourceID, &provenance, &certainty, &evidence.Confidence, &observedAt, &synthetic) != nil {
				return domain.RelationshipPage{}, domain.ErrRepositoryUnavailable
			}
			index, exists := byID[relationshipID]
			if !exists {
				return domain.RelationshipPage{}, domain.ErrRepositoryUnavailable
			}
			evidence.Provider, evidence.Provenance, evidence.Certainty = domain.ProviderKind(provider), domain.RelationshipProvenance(provenance), domain.EvidenceCertainty(certainty)
			evidence.ObservedAt = time.UnixMicro(observedAt).UTC()
			evidence.Synthetic = synthetic == 1
			result.Relationships[index].Evidence = append(result.Relationships[index].Evidence, evidence)
			if len(result.Relationships[index].Evidence) > domain.MaxRelationshipEvidence {
				return domain.RelationshipPage{}, domain.ErrRepositoryUnavailable
			}
		}
		if err := rows.Err(); err != nil {
			return domain.RelationshipPage{}, mapRepositoryReadError(ctx, err)
		}
	}
	rows.Close()
	status, err = store.observationCapabilityStatus(ctx, query.Source, domain.CapabilityTopology, status, len(result.Relationships) > 0)
	if err != nil {
		return domain.RelationshipPage{}, err
	}
	result.Status = status
	if !status.AllowsData() {
		result.Relationships = []domain.Relationship{}
		return result, nil
	}
	if len(result.Relationships) > query.Page.Limit {
		result.Relationships = result.Relationships[:query.Page.Limit]
		result.Page.HasMore = true
		result.Page.NextCursor, err = store.encodeObservationCursor("relationships", queryHash, "", string(result.Relationships[len(result.Relationships)-1].ID))
		if err != nil {
			return domain.RelationshipPage{}, err
		}
	}
	if result.ValidateFor(query) != nil {
		return domain.RelationshipPage{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}
