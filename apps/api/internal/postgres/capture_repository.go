package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (store *Store) BeginCapture(ctx context.Context, sourceID domain.SourceID, version int64, observedAt time.Time) (domain.CaptureToken, error) {
	if ctx == nil || sourceID.Validate() != nil || version < 1 || observedAt.IsZero() || observedAt.Location() != time.UTC || observedAt.UnixMicro() < 0 || observedAt.Nanosecond()%int(time.Microsecond) != 0 {
		return domain.CaptureToken{}, domain.ErrRepositoryInvalidInput
	}
	var token domain.CaptureToken
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		var provider string
		var generation int64
		err := transaction.transaction.QueryRow(ctx, `UPDATE infrastructure_sources
SET capture_generation=capture_generation+1,capture_started_at_us=$1,capture_finished_at_us=NULL
WHERE id=$2 AND row_version=$3 AND enabled=1 AND deleted_at_us IS NULL
  AND (capture_started_at_us IS NULL OR capture_started_at_us <= $1)
RETURNING provider,capture_generation`, observedAt.UnixMicro(), string(sourceID), version).Scan(&provider, &generation)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRepositoryConflict
		}
		if err != nil {
			return err
		}
		token = domain.CaptureToken{Source: domain.ProviderRef{Provider: domain.ProviderKind(provider), SourceID: sourceID}, ConfigurationVersion: version, Generation: generation, ObservedAt: observedAt}
		return token.Validate()
	})
	if err != nil {
		return domain.CaptureToken{}, mapRepositoryWriteError(ctx, err)
	}
	return token, nil
}

func requireCaptureToken(ctx context.Context, transaction pgx.Tx, token domain.CaptureToken, finished bool) error {
	var provider string
	var version, enabled, generation, started int64
	var finishedAt sql.NullInt64
	err := transaction.QueryRow(ctx, `SELECT provider,row_version,enabled,capture_generation,capture_started_at_us,capture_finished_at_us
FROM infrastructure_sources WHERE id=$1 AND deleted_at_us IS NULL FOR UPDATE`, string(token.Source.SourceID)).Scan(&provider, &version, &enabled, &generation, &started, &finishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrRepositoryConflict
	}
	if err != nil {
		return err
	}
	if provider != string(token.Source.Provider) || version != token.ConfigurationVersion || enabled != 1 || generation != token.Generation || started != token.ObservedAt.UnixMicro() || finishedAt.Valid != finished {
		return domain.ErrRepositoryConflict
	}
	return nil
}

func (store *Store) SaveCapture(ctx context.Context, batch domain.CaptureBatch) error {
	if ctx == nil || batch.Validate() != nil {
		return domain.ErrRepositoryInvalidInput
	}
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if err := requireCaptureToken(ctx, transaction.transaction, batch.Token, false); err != nil {
			return err
		}
		for _, environment := range batch.Environments {
			if err := saveCapturedEnvironment(ctx, transaction.transaction, batch.Token, environment); err != nil {
				return err
			}
		}
		for _, resource := range batch.Resources {
			if err := saveCapturedResource(ctx, transaction.transaction, batch.Token, resource); err != nil {
				return err
			}
		}
		for _, health := range batch.Health {
			if err := saveCapturedHealth(ctx, transaction.transaction, batch.Token, health); err != nil {
				return err
			}
		}
		for _, workload := range batch.WorkloadReplicas {
			if err := requireCapturedResource(ctx, transaction.transaction, batch.Token, workload.Resource.ID); err != nil {
				return err
			}
			status := workload.Status
			result, err := transaction.transaction.Exec(ctx, `UPDATE resource_snapshots SET desired_replicas=$1,ready_replicas=$2,
replicas_availability=$3,replicas_completeness=$4,replicas_observed_at_us=$5,replicas_reason_code=$6
WHERE resource_id=$7`, int64(workload.Desired), int64(workload.Ready), string(status.State), string(status.Completeness), status.ObservedAt.UnixMicro(), optionalString(string(status.ReasonCode)), string(workload.Resource.ID))
			if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
				return err
			}
			if _, err := transaction.transaction.Exec(ctx, `DELETE FROM workload_replica_history WHERE resource_id=$1 AND capture_generation=$2 AND reason_code='source.replicas_unavailable'`, string(workload.Resource.ID), batch.Token.Generation); err != nil {
				return err
			}
			result, err = transaction.transaction.Exec(ctx, `INSERT INTO workload_replica_history(resource_id,sampled_at_us,desired_replicas,ready_replicas,availability,completeness,reason_code,capture_generation) VALUES($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT(resource_id,sampled_at_us) DO UPDATE SET resource_id=workload_replica_history.resource_id
WHERE workload_replica_history.desired_replicas=EXCLUDED.desired_replicas AND workload_replica_history.ready_replicas=EXCLUDED.ready_replicas
AND workload_replica_history.availability=EXCLUDED.availability AND workload_replica_history.completeness=EXCLUDED.completeness
AND workload_replica_history.reason_code IS NOT DISTINCT FROM EXCLUDED.reason_code`, string(workload.Resource.ID), status.ObservedAt.UnixMicro(), int64(workload.Desired), int64(workload.Ready), string(status.State), string(status.Completeness), optionalString(string(status.ReasonCode)), batch.Token.Generation)
			if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
				return err
			}
		}
		for _, relationship := range batch.Relationships {
			if err := saveCapturedRelationship(ctx, transaction.transaction, batch.Token, relationship); err != nil {
				return err
			}
		}
		for _, sample := range batch.Metrics {
			err := requireCapturedResource(ctx, transaction.transaction, batch.Token, sample.ResourceID)
			if err != nil {
				return err
			}
			result, err := transaction.transaction.Exec(ctx, insertMetricSampleStatement+` ON CONFLICT(id) DO UPDATE SET id=metric_samples.id
WHERE metric_samples.resource_id=EXCLUDED.resource_id AND metric_samples.metric_name=EXCLUDED.metric_name
AND metric_samples.unit=EXCLUDED.unit AND metric_samples.value=EXCLUDED.value AND metric_samples.sampled_at_us=EXCLUDED.sampled_at_us
AND metric_samples.interval_seconds=EXCLUDED.interval_seconds AND metric_samples.synthetic=EXCLUDED.synthetic`, string(sample.ID), string(sample.ResourceID), string(sample.Name), string(sample.Unit), sample.Value, sample.SampledAt.UnixMicro(), int64(sample.Interval/time.Second), boolInteger(sample.Synthetic), string(sample.ResourceID), boolInteger(sample.Synthetic), boolInteger(sample.Synthetic))
			if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
				return err
			}
		}
		for _, event := range batch.Events {
			if event.ResourceID != nil {
				if err := requireCapturedResource(ctx, transaction.transaction, batch.Token, *event.ResourceID); err != nil {
					return err
				}
			}
			result, err := transaction.transaction.Exec(ctx, insertInfrastructureEventStatement+` ON CONFLICT(id) DO UPDATE SET id=infrastructure_events.id
WHERE infrastructure_events.provider=EXCLUDED.provider AND infrastructure_events.source_id=EXCLUDED.source_id
AND infrastructure_events.environment_id IS NOT DISTINCT FROM EXCLUDED.environment_id
AND infrastructure_events.resource_id IS NOT DISTINCT FROM EXCLUDED.resource_id AND infrastructure_events.kind=EXCLUDED.kind
AND infrastructure_events.severity=EXCLUDED.severity AND infrastructure_events.reason_code=EXCLUDED.reason_code
AND infrastructure_events.summary=EXCLUDED.summary AND infrastructure_events.occurred_at_us=EXCLUDED.occurred_at_us
AND infrastructure_events.synthetic=EXCLUDED.synthetic`, string(event.ID), string(event.Provider), string(event.SourceID), optionalEnvironmentID(event.EnvironmentID), optionalResourceID(event.ResourceID), string(event.Kind), string(event.Severity), string(event.ReasonCode), event.Summary, event.OccurredAt.UnixMicro(), event.ObservedAt.UnixMicro(), boolInteger(event.Synthetic))
			if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
				return err
			}
		}
		return nil
	})
	return mapRepositoryWriteError(ctx, err)
}

func saveCapturedEnvironment(ctx context.Context, transaction pgx.Tx, token domain.CaptureToken, environment domain.Environment) error {
	status := environment.Status
	result, err := transaction.Exec(ctx, `INSERT INTO environments
(id,source_id,provider,display_name,availability,completeness,observed_at_us,reason_code,synthetic,first_seen_at_us,last_seen_at_us,capture_generation)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$7,$7,$10)
ON CONFLICT(id) DO UPDATE SET display_name=EXCLUDED.display_name,availability=EXCLUDED.availability,
completeness=EXCLUDED.completeness,observed_at_us=EXCLUDED.observed_at_us,reason_code=EXCLUDED.reason_code,
first_seen_at_us=LEAST(environments.first_seen_at_us,EXCLUDED.first_seen_at_us),
last_seen_at_us=GREATEST(environments.last_seen_at_us,EXCLUDED.last_seen_at_us),retired_at_us=NULL,
capture_generation=EXCLUDED.capture_generation,row_version=environments.row_version+1
WHERE environments.source_id=EXCLUDED.source_id AND environments.provider=EXCLUDED.provider`, string(environment.ID), string(environment.SourceID), string(environment.Provider), environment.Name, string(status.State), string(status.Completeness), status.ObservedAt.UnixMicro(), optionalString(string(status.ReasonCode)), boolInteger(environment.Synthetic), token.Generation)
	return exactInsertResult(ctx, result.RowsAffected(), err)
}

func saveCapturedResource(ctx context.Context, transaction pgx.Tx, token domain.CaptureToken, snapshot domain.ResourceSnapshot) error {
	resource, status := snapshot.Resource, snapshot.Status
	identity := resource.Ref.Identity
	var refresh bool
	var hadReplicas bool
	err := transaction.QueryRow(ctx, `SELECT resource.capture_generation<>$2,snapshot.desired_replicas IS NOT NULL FROM resources AS resource LEFT JOIN resource_snapshots AS snapshot ON snapshot.resource_id=resource.id WHERE resource.id=$1`, string(resource.Ref.ID), token.Generation).Scan(&refresh, &hadReplicas)
	if errors.Is(err, pgx.ErrNoRows) {
		refresh = true
	} else if err != nil {
		return err
	}
	if refresh && hadReplicas {
		if _, err := transaction.Exec(ctx, `INSERT INTO workload_replica_history(resource_id,capture_generation,sampled_at_us,desired_replicas,ready_replicas,availability,completeness,reason_code) VALUES($1,$2,$3,0,0,'unavailable','complete','source.replicas_unavailable') ON CONFLICT(resource_id,sampled_at_us) DO NOTHING`, string(resource.Ref.ID), token.Generation, token.ObservedAt.UnixMicro()); err != nil {
			return err
		}
	}
	result, err := transaction.Exec(ctx, `INSERT INTO resources
(id,source_id,provider,environment_id,kind,scope_kind,scope_name,native_id,synthetic,first_seen_at_us,last_seen_at_us,capture_generation)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$11)
ON CONFLICT(id) DO UPDATE SET first_seen_at_us=LEAST(resources.first_seen_at_us,EXCLUDED.first_seen_at_us),
last_seen_at_us=GREATEST(resources.last_seen_at_us,EXCLUDED.last_seen_at_us),retired_at_us=NULL,
capture_generation=EXCLUDED.capture_generation,row_version=resources.row_version+1
WHERE resources.source_id=EXCLUDED.source_id AND resources.provider=EXCLUDED.provider
  AND resources.environment_id=EXCLUDED.environment_id AND resources.kind=EXCLUDED.kind
  AND resources.scope_kind=EXCLUDED.scope_kind AND resources.scope_name=EXCLUDED.scope_name AND resources.native_id=EXCLUDED.native_id`, string(resource.Ref.ID), string(identity.SourceID), string(identity.Provider), string(identity.EnvironmentID), string(identity.Kind), string(identity.Scope.Kind), identity.Scope.Name, string(identity.NativeID()), boolInteger(resource.Synthetic), status.ObservedAt.UnixMicro(), token.Generation)
	if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
		return err
	}
	_, err = transaction.Exec(ctx, `INSERT INTO resource_snapshots
(resource_id,display_name,resource_version,availability,completeness,observed_at_us,reason_code)
VALUES($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT(resource_id) DO UPDATE SET display_name=EXCLUDED.display_name,resource_version=EXCLUDED.resource_version,
availability=EXCLUDED.availability,completeness=EXCLUDED.completeness,observed_at_us=EXCLUDED.observed_at_us,
reason_code=EXCLUDED.reason_code,row_version=resource_snapshots.row_version+1`, string(resource.Ref.ID), resource.DisplayName, resource.Version, string(status.State), string(status.Completeness), status.ObservedAt.UnixMicro(), optionalString(string(status.ReasonCode)))
	if err == nil && refresh {
		_, err = transaction.Exec(ctx, `UPDATE resource_snapshots SET desired_replicas=NULL,ready_replicas=NULL,replicas_availability=NULL,replicas_completeness=NULL,replicas_observed_at_us=NULL,replicas_reason_code=NULL WHERE resource_id=$1`, string(resource.Ref.ID))
	}
	return err
}

func requireCapturedResource(ctx context.Context, transaction pgx.Tx, token domain.CaptureToken, resourceID domain.ResourceID) error {
	var marker int
	err := transaction.QueryRow(ctx, `SELECT 1 FROM resources
WHERE id=$1 AND source_id=$2 AND provider=$3 AND synthetic=$4 AND retired_at_us IS NULL`, string(resourceID), string(token.Source.SourceID), string(token.Source.Provider), boolInteger(token.Source.Provider == domain.ProviderDemo)).Scan(&marker)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrRepositoryConflict
	}
	return err
}

func saveCapturedHealth(ctx context.Context, transaction pgx.Tx, token domain.CaptureToken, health domain.HealthSnapshot) error {
	if err := requireCapturedResource(ctx, transaction, token, health.ResourceID); err != nil {
		return err
	}
	status := health.Status
	var previousState, previousAvailability, previousCompleteness string
	var previousReason sql.NullString
	lookupErr := transaction.QueryRow(ctx, `SELECT health_state,availability,completeness,reason_code FROM resource_health_current WHERE resource_id=$1`, string(health.ResourceID)).Scan(&previousState, &previousAvailability, &previousCompleteness, &previousReason)
	if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
		return lookupErr
	}
	changed := errors.Is(lookupErr, pgx.ErrNoRows) || previousState != string(health.State) || previousAvailability != string(status.State) ||
		previousCompleteness != string(status.Completeness) || previousReason.String != string(status.ReasonCode)
	if _, err := transaction.Exec(ctx, `INSERT INTO resource_health_current
(resource_id,health_state,availability,completeness,observed_at_us,reason_code)
VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(resource_id) DO UPDATE SET health_state=EXCLUDED.health_state,availability=EXCLUDED.availability,
completeness=EXCLUDED.completeness,observed_at_us=EXCLUDED.observed_at_us,reason_code=EXCLUDED.reason_code,row_version=resource_health_current.row_version+1`, string(health.ResourceID), string(health.State), string(status.State), string(status.Completeness), status.ObservedAt.UnixMicro(), optionalString(string(status.ReasonCode))); err != nil {
		return err
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM health_evidence WHERE resource_id=$1`, string(health.ResourceID)); err != nil {
		return err
	}
	for _, evidence := range health.Evidence {
		if _, err := transaction.Exec(ctx, `INSERT INTO health_evidence(resource_id,evidence_id,code,source_code,status,observed_at_us,synthetic)
VALUES($1,$2,$3,$4,$5,$6,$7)`, string(health.ResourceID), string(evidence.ID), string(evidence.Code), string(evidence.SourceCode), string(evidence.Status), evidence.ObservedAt.UnixMicro(), boolInteger(evidence.Synthetic)); err != nil {
			return err
		}
	}
	if !changed {
		return nil
	}
	id := "health_" + observationQueryHash(struct {
		ResourceID   domain.ResourceID
		ObservedAt   int64
		State        domain.HealthState
		Availability domain.AvailabilityState
		Completeness domain.Completeness
		Reason       domain.ReasonCode
	}{health.ResourceID, status.ObservedAt.UnixMicro(), health.State, status.State, status.Completeness, status.ReasonCode})
	_, err := transaction.Exec(ctx, `INSERT INTO health_history(id,resource_id,health_state,availability,completeness,observed_at_us,reason_code,synthetic)
VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(id) DO NOTHING`, id, string(health.ResourceID), string(health.State), string(status.State), string(status.Completeness), status.ObservedAt.UnixMicro(), optionalString(string(status.ReasonCode)), boolInteger(token.Source.Provider == domain.ProviderDemo))
	return err
}

func saveCapturedRelationship(ctx context.Context, transaction pgx.Tx, token domain.CaptureToken, relationship domain.Relationship) error {
	if err := requireCapturedResource(ctx, transaction, token, relationship.From); err != nil {
		return err
	}
	if err := requireCapturedResource(ctx, transaction, token, relationship.To); err != nil {
		return err
	}
	result, err := transaction.Exec(ctx, `INSERT INTO relationships
(id,source_id,from_resource_id,to_resource_id,kind,direction,observed_at_us,first_seen_at_us,last_seen_at_us,capture_generation)
VALUES($1,$2,$3,$4,$5,$6,$7,$7,$7,$8)
ON CONFLICT(id) DO UPDATE SET observed_at_us=EXCLUDED.observed_at_us,
first_seen_at_us=LEAST(relationships.first_seen_at_us,EXCLUDED.first_seen_at_us),
last_seen_at_us=GREATEST(relationships.last_seen_at_us,EXCLUDED.last_seen_at_us),retired_at_us=NULL,
capture_generation=EXCLUDED.capture_generation,row_version=relationships.row_version+1
WHERE relationships.source_id=EXCLUDED.source_id AND relationships.from_resource_id=EXCLUDED.from_resource_id
AND relationships.to_resource_id=EXCLUDED.to_resource_id AND relationships.kind=EXCLUDED.kind AND relationships.direction=EXCLUDED.direction`, string(relationship.ID), string(token.Source.SourceID), string(relationship.From), string(relationship.To), string(relationship.Kind), string(relationship.Direction), relationship.ObservedAt.UnixMicro(), token.Generation)
	if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
		return err
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM relationship_evidence WHERE relationship_id=$1`, string(relationship.ID)); err != nil {
		return err
	}
	for _, evidence := range relationship.Evidence {
		if _, err := transaction.Exec(ctx, `INSERT INTO relationship_evidence
(relationship_id,evidence_id,provider,source_id,provenance,certainty,confidence,observed_at_us,synthetic)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, string(relationship.ID), string(evidence.ID), string(evidence.Provider), string(evidence.SourceID), string(evidence.Provenance), string(evidence.Certainty), evidence.Confidence, evidence.ObservedAt.UnixMicro(), boolInteger(evidence.Synthetic)); err != nil {
			return err
		}
	}
	return nil
}

func (store *Store) FinishCapture(ctx context.Context, completion domain.CaptureCompletion) error {
	if ctx == nil || completion.Validate() != nil {
		return domain.ErrRepositoryInvalidInput
	}
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if err := requireCaptureToken(ctx, transaction.transaction, completion.Token, false); err != nil {
			return err
		}
		status := completion.Status
		if !completion.Complete && status.AllowsData() && status.Completeness == domain.CompletenessComplete {
			status.Completeness = domain.CompletenessPartial
			status.ReasonCode = "source.collection_partial"
		}
		if err := saveCollectionOutcome(ctx, transaction, completion, status); err != nil {
			return err
		}
		if _, err := transaction.transaction.Exec(ctx, `INSERT INTO source_status(source_id,availability,completeness,observed_at_us,reason_code)
VALUES($1,$2,$3,$4,$5) ON CONFLICT(source_id) DO UPDATE SET availability=EXCLUDED.availability,
completeness=EXCLUDED.completeness,observed_at_us=EXCLUDED.observed_at_us,reason_code=EXCLUDED.reason_code,row_version=source_status.row_version+1`, string(completion.Token.Source.SourceID), string(status.State), string(status.Completeness), status.ObservedAt.UnixMicro(), optionalString(string(status.ReasonCode))); err != nil {
			return err
		}
		topologyComplete := false
		for _, capability := range completion.Capabilities.Capabilities {
			if capability.Name == domain.CapabilityTopology && capability.Status.State == domain.AvailabilityAvailable && capability.Status.Completeness == domain.CompletenessComplete {
				topologyComplete = true
			}
			if _, err := transaction.transaction.Exec(ctx, `INSERT INTO source_capabilities(source_id,capability_name,availability,completeness,observed_at_us,reason_code)
VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(source_id,capability_name) DO UPDATE SET availability=EXCLUDED.availability,
completeness=EXCLUDED.completeness,observed_at_us=EXCLUDED.observed_at_us,reason_code=EXCLUDED.reason_code,row_version=source_capabilities.row_version+1`, string(completion.Token.Source.SourceID), string(capability.Name), string(capability.Status.State), string(capability.Status.Completeness), capability.Status.ObservedAt.UnixMicro(), optionalString(string(capability.Status.ReasonCode))); err != nil {
				return err
			}
		}
		_, err := transaction.transaction.Exec(ctx, `UPDATE infrastructure_sources SET capture_finished_at_us=$1,
completed_generation=CASE WHEN $2 THEN capture_generation ELSE completed_generation END,
relationship_completed_generation=CASE WHEN $3 THEN capture_generation ELSE relationship_completed_generation END
WHERE id=$4`, completion.Token.ObservedAt.UnixMicro(), completion.Complete, completion.Complete && topologyComplete, string(completion.Token.Source.SourceID))
		return err
	})
	return mapRepositoryWriteError(ctx, err)
}

func (store *Store) RetireUnseen(ctx context.Context, token domain.CaptureToken, limit int) (int64, error) {
	if ctx == nil || token.Validate() != nil || limit < 1 || limit > MaximumRetentionBatch {
		return 0, domain.ErrRepositoryInvalidInput
	}
	var retired int64
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if err := requireCaptureToken(ctx, transaction.transaction, token, true); err != nil {
			return err
		}
		for _, table := range []string{"relationships", "resources", "environments"} {
			remaining := int64(limit) - retired
			if remaining == 0 {
				break
			}
			generationColumn := "completed_generation"
			if table == "relationships" {
				generationColumn = "relationship_completed_generation"
			}
			statement := `WITH unseen AS (SELECT item.id FROM ` + table + ` AS item
JOIN infrastructure_sources AS source ON source.id=item.source_id
WHERE item.source_id=$1 AND source.` + generationColumn + `=$2 AND item.capture_generation<$2 AND item.retired_at_us IS NULL
ORDER BY item.capture_generation,item.id LIMIT $3 FOR UPDATE OF item SKIP LOCKED)
UPDATE ` + table + ` AS item SET retired_at_us=GREATEST(item.last_seen_at_us,$4),row_version=item.row_version+1
FROM unseen WHERE item.id=unseen.id`
			result, err := transaction.transaction.Exec(ctx, statement, string(token.Source.SourceID), token.Generation, remaining, token.ObservedAt.UnixMicro())
			if err != nil {
				return err
			}
			retired += result.RowsAffected()
		}
		return nil
	})
	if err != nil {
		return 0, mapRepositoryWriteError(ctx, err)
	}
	return retired, nil
}

var _ domain.CaptureRepository = (*Store)(nil)
