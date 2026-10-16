package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/alerting"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

const claimDueAlertEvaluationsStatement = `WITH due AS (
    SELECT id
    FROM alert_rules
    WHERE enabled = 1
      AND deleted_at_us IS NULL
      AND COALESCE(next_evaluation_at_us, created_at_us) <= $1
      AND (evaluation_lease_until_us IS NULL OR evaluation_lease_until_us <= $1)
      AND EXISTS(SELECT 1 FROM infrastructure_sources AS source WHERE source.id=alert_rules.source_id AND source.enabled=1 AND source.deleted_at_us IS NULL)
    ORDER BY COALESCE(next_evaluation_at_us, created_at_us), id
    FOR UPDATE SKIP LOCKED
    LIMIT $2
), claimed AS (
    UPDATE alert_rules AS rule SET
        next_evaluation_at_us = $1 + rule.evaluation_interval_seconds * 1000000,
        evaluation_generation = rule.evaluation_generation + 1,
        evaluation_lease_until_us = $1 + 60000000
    FROM due
    WHERE rule.id = due.id
    RETURNING rule.id, rule.source_id, rule.environment_id, rule.resource_id,
        rule.display_name, rule.rule_kind, rule.enabled, rule.severity,
        rule.comparison_operator, rule.threshold_value, rule.threshold_count,
        rule.window_seconds, rule.minimum_event_severity,
        rule.evaluation_interval_seconds, rule.pending_for_seconds,
        rule.next_evaluation_at_us,rule.row_version,rule.evaluation_generation,rule.evaluation_lease_until_us
)
SELECT claimed.id, claimed.source_id, claimed.environment_id, claimed.resource_id,
       claimed.display_name, claimed.rule_kind, claimed.enabled, claimed.severity,
       claimed.comparison_operator, claimed.threshold_value, claimed.threshold_count,
       claimed.window_seconds, claimed.minimum_event_severity,
       claimed.evaluation_interval_seconds, claimed.pending_for_seconds,
       claimed.next_evaluation_at_us,claimed.row_version,claimed.evaluation_generation,claimed.evaluation_lease_until_us,
       source.provider, source.synthetic,source.row_version,
       resource.id, resource.provider, resource.environment_id, resource.kind,
       resource.scope_kind, resource.scope_name, resource.native_id,
       current_alert.id, current_alert.rule_id, current_alert.source_id,
       current_alert.environment_id, current_alert.resource_id, current_alert.state,
       current_alert.severity, current_alert.evidence_start_at_us,
       current_alert.evidence_end_at_us, current_alert.started_at_us,
       current_alert.updated_at_us, current_alert.acknowledged_at_us,
       current_alert.resolved_at_us, current_alert.summary_code,
       current_alert.synthetic, current_alert.acknowledged_by_user_id
FROM claimed
JOIN infrastructure_sources AS source ON source.id = claimed.source_id
LEFT JOIN resources AS resource ON resource.id = claimed.resource_id
LEFT JOIN LATERAL (
    SELECT alert.id, alert.rule_id, alert.source_id, alert.environment_id,
           alert.resource_id, alert.state, alert.severity,
           alert.evidence_start_at_us, alert.evidence_end_at_us,
           alert.started_at_us, alert.updated_at_us, alert.acknowledged_at_us,
           alert.resolved_at_us, alert.summary_code, alert.synthetic,
           alert.acknowledged_by_user_id
    FROM alerts AS alert
    WHERE alert.rule_id = claimed.id
      AND alert.dedupe_key = claimed.id
      AND alert.resolved_at_us IS NULL
    ORDER BY alert.started_at_us DESC, alert.id
    LIMIT 1
) AS current_alert ON TRUE
ORDER BY claimed.next_evaluation_at_us, claimed.id`

var _ alerting.TaskStore = (*Store)(nil)

func (store *Store) ClaimDueAlertEvaluations(ctx context.Context, claimedAt time.Time, limit int) ([]alerting.EvaluationTask, error) {
	if !store.available() || ctx == nil || ctx.Err() != nil ||
		claimedAt.IsZero() || claimedAt.Location() != time.UTC || claimedAt.Nanosecond()%int(time.Microsecond) != 0 ||
		limit < 1 || limit > alerting.MaximumEvaluationBatch {
		return nil, domain.ErrRepositoryInvalidInput
	}
	queryContext, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	rows, err := store.pool.Query(queryContext, claimDueAlertEvaluationsStatement, claimedAt.UnixMicro(), limit)
	if err != nil {
		return nil, mapRepositoryReadError(queryContext, err)
	}
	defer rows.Close()
	tasks := make([]alerting.EvaluationTask, 0, limit)
	for rows.Next() {
		task, err := scanEvaluationTask(rows, claimedAt)
		if err != nil {
			return nil, errors.Join(domain.ErrRepositoryUnavailable, err)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, mapRepositoryReadError(queryContext, err)
	}
	return tasks, nil
}

// Completion fences both lease recovery and edits that happened during evidence reads.
func (store *Store) CompleteAlertEvaluation(ctx context.Context, task alerting.EvaluationTask, mutation *domain.AlertMutation) error {
	if ctx == nil || task.Validate() != nil || mutation != nil && mutation.Validate() != nil {
		return domain.ErrRepositoryInvalidInput
	}
	if mutation != nil && (mutation.Alert.RuleID != task.Rule.ID || mutation.Alert.SourceID != task.Source.SourceID || mutation.DedupeKey != task.DedupeKey || mutation.Alert.Synthetic != task.Synthetic) {
		return domain.ErrRepositoryInvalidInput
	}
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if err := lockAlertChanges(ctx, transaction); err != nil {
			return err
		}
		// Hold the source configuration stable through the evidence commit.
		var sourceVersion int64
		if err := transaction.transaction.QueryRow(ctx, `SELECT row_version FROM infrastructure_sources WHERE id=$1 AND provider=$2 AND enabled=1 AND deleted_at_us IS NULL FOR SHARE`, string(task.Source.SourceID), string(task.Source.Provider)).Scan(&sourceVersion); errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRepositoryConflict
		} else if err != nil {
			return err
		}
		if sourceVersion != task.SourceConfigurationVersion {
			return domain.ErrRepositoryConflict
		}
		result, err := transaction.transaction.Exec(ctx, `UPDATE alert_rules SET evaluation_lease_until_us=NULL,last_evaluated_at_us=$1
WHERE id=$2 AND source_id=$3 AND enabled=1 AND deleted_at_us IS NULL AND row_version=$4 AND evaluation_generation=$5
AND evaluation_lease_until_us=$6 AND evaluation_lease_until_us>floor(extract(epoch FROM clock_timestamp())*1000000)::bigint`, task.ClaimedAt.UnixMicro(), string(task.Rule.ID), string(task.Source.SourceID), task.RuleVersion, task.ClaimGeneration, task.LeaseExpiresAt.UnixMicro())
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return err
		}
		if mutation == nil {
			return nil
		}
		return (&transactionRepositories{transaction: transaction}).SaveAlertMutation(ctx, *mutation)
	})
	return mapRepositoryWriteError(ctx, err)
}

func scanEvaluationTask(row rowScanner, claimedAt time.Time) (alerting.EvaluationTask, error) {
	var task alerting.EvaluationTask
	var ruleEnvironmentID, ruleResourceID sql.NullString
	var comparison, minimumSeverity sql.NullString
	var thresholdValue sql.NullFloat64
	var thresholdCount, windowSeconds sql.NullInt64
	var ruleKind, ruleSeverity, sourceProvider string
	var enabled, sourceSynthetic int64
	var evaluationSeconds, pendingSeconds, nextEvaluationUS int64
	var leaseExpiresUS int64
	var resourceID, resourceProvider, resourceEnvironmentID, resourceKind sql.NullString
	var resourceScopeKind, resourceScopeName, resourceNativeID sql.NullString
	var currentID, currentRuleID, currentSourceID sql.NullString
	var currentEnvironmentID, currentResourceID, currentState, currentSeverity sql.NullString
	var evidenceStartUS, evidenceEndUS, startedAtUS, updatedAtUS sql.NullInt64
	var acknowledgedAtUS, resolvedAtUS sql.NullInt64
	var summaryCode sql.NullString
	var currentSynthetic sql.NullInt64
	var acknowledgingUserID sql.NullString
	if err := row.Scan(
		&task.Rule.ID, &task.Rule.SourceID, &ruleEnvironmentID, &ruleResourceID,
		&task.Rule.DisplayName, &ruleKind, &enabled, &ruleSeverity,
		&comparison, &thresholdValue, &thresholdCount, &windowSeconds, &minimumSeverity,
		&evaluationSeconds, &pendingSeconds, &nextEvaluationUS, &task.RuleVersion, &task.ClaimGeneration, &leaseExpiresUS,
		&sourceProvider, &sourceSynthetic, &task.SourceConfigurationVersion,
		&resourceID, &resourceProvider, &resourceEnvironmentID, &resourceKind,
		&resourceScopeKind, &resourceScopeName, &resourceNativeID,
		&currentID, &currentRuleID, &currentSourceID, &currentEnvironmentID,
		&currentResourceID, &currentState, &currentSeverity, &evidenceStartUS,
		&evidenceEndUS, &startedAtUS, &updatedAtUS, &acknowledgedAtUS,
		&resolvedAtUS, &summaryCode, &currentSynthetic, &acknowledgingUserID,
	); err != nil {
		return alerting.EvaluationTask{}, err
	}
	if enabled != 1 || sourceSynthetic < 0 || sourceSynthetic > 1 || evaluationSeconds < 1 || pendingSeconds < 0 {
		return alerting.EvaluationTask{}, domain.ErrRepositoryUnavailable
	}
	task.Rule.Enabled = true
	task.Rule.Kind = domain.AlertRuleKind(ruleKind)
	task.Rule.Severity = domain.Severity(ruleSeverity)
	task.Rule.EvaluationInterval = time.Duration(evaluationSeconds) * time.Second
	task.Rule.PendingFor = time.Duration(pendingSeconds) * time.Second
	task.Source = domain.ProviderRef{Provider: domain.ProviderKind(sourceProvider), SourceID: task.Rule.SourceID}
	task.Synthetic = sourceSynthetic == 1
	task.ClaimedAt = claimedAt
	task.NextEvaluationAt = time.UnixMicro(nextEvaluationUS).UTC()
	task.LeaseExpiresAt = time.UnixMicro(leaseExpiresUS).UTC()
	task.DedupeKey = string(task.Rule.ID)
	if ruleEnvironmentID.Valid {
		value := domain.EnvironmentID(ruleEnvironmentID.String)
		task.Rule.EnvironmentID = &value
	}
	if ruleResourceID.Valid {
		value := domain.ResourceID(ruleResourceID.String)
		task.Rule.ResourceID = &value
	}
	if comparison.Valid {
		value := domain.ComparisonOperator(comparison.String)
		task.Rule.Comparison = &value
	}
	if thresholdValue.Valid {
		value := thresholdValue.Float64
		task.Rule.ThresholdValue = &value
	}
	if thresholdCount.Valid {
		if thresholdCount.Int64 < 1 {
			return alerting.EvaluationTask{}, domain.ErrRepositoryUnavailable
		}
		value := uint64(thresholdCount.Int64)
		task.Rule.ThresholdCount = &value
	}
	if windowSeconds.Valid {
		task.Rule.Window = time.Duration(windowSeconds.Int64) * time.Second
	}
	if minimumSeverity.Valid {
		value := domain.Severity(minimumSeverity.String)
		task.Rule.MinimumSeverity = &value
	}
	if resourceID.Valid {
		if !resourceProvider.Valid || !resourceEnvironmentID.Valid || !resourceKind.Valid ||
			!resourceScopeKind.Valid || !resourceScopeName.Valid || !resourceNativeID.Valid {
			return alerting.EvaluationTask{}, domain.ErrRepositoryUnavailable
		}
		identity, err := domain.NewResourceIdentity(
			domain.ProviderKind(resourceProvider.String), task.Rule.SourceID,
			domain.EnvironmentID(resourceEnvironmentID.String), domain.ResourceKind(resourceKind.String),
			domain.Scope{Kind: domain.ScopeKind(resourceScopeKind.String), Name: resourceScopeName.String},
			domain.NativeResourceID(resourceNativeID.String),
		)
		if err != nil {
			return alerting.EvaluationTask{}, err
		}
		ref := domain.ResourceRef{ID: domain.ResourceID(resourceID.String), Identity: identity}
		task.Resource = &ref
	}
	if currentID.Valid {
		current, err := scanClaimedAlert(
			currentID, currentRuleID, currentSourceID, currentEnvironmentID, currentResourceID,
			currentState, currentSeverity, evidenceStartUS, evidenceEndUS, startedAtUS,
			updatedAtUS, acknowledgedAtUS, resolvedAtUS, summaryCode, currentSynthetic,
		)
		if err != nil {
			return alerting.EvaluationTask{}, err
		}
		task.Current = &current
		if acknowledgingUserID.Valid {
			value := domain.UserID(acknowledgingUserID.String)
			task.AcknowledgedByUserID = &value
		}
	} else if currentRuleID.Valid || currentSourceID.Valid || acknowledgingUserID.Valid {
		return alerting.EvaluationTask{}, domain.ErrRepositoryUnavailable
	}
	if err := task.Validate(); err != nil {
		return alerting.EvaluationTask{}, err
	}
	return task, nil
}

func scanClaimedAlert(
	id, ruleID, sourceID, environmentID, resourceID, state, severity sql.NullString,
	evidenceStartUS, evidenceEndUS, startedAtUS, updatedAtUS, acknowledgedAtUS, resolvedAtUS sql.NullInt64,
	summaryCode sql.NullString,
	synthetic sql.NullInt64,
) (domain.Alert, error) {
	if !id.Valid || !ruleID.Valid || !sourceID.Valid || !state.Valid || !severity.Valid ||
		!evidenceStartUS.Valid || !evidenceEndUS.Valid || !startedAtUS.Valid || !updatedAtUS.Valid ||
		!summaryCode.Valid || !synthetic.Valid || synthetic.Int64 < 0 || synthetic.Int64 > 1 {
		return domain.Alert{}, domain.ErrRepositoryUnavailable
	}
	alert := domain.Alert{
		ID: domain.AlertID(id.String), RuleID: domain.AlertRuleID(ruleID.String),
		SourceID: domain.SourceID(sourceID.String), State: domain.AlertState(state.String),
		Severity: domain.Severity(severity.String),
		EvidenceRange: domain.TimeRange{
			Start: time.UnixMicro(evidenceStartUS.Int64).UTC(), End: time.UnixMicro(evidenceEndUS.Int64).UTC(),
		},
		StartedAt: time.UnixMicro(startedAtUS.Int64).UTC(), UpdatedAt: time.UnixMicro(updatedAtUS.Int64).UTC(),
		SummaryCode: domain.ReasonCode(summaryCode.String), Synthetic: synthetic.Int64 == 1,
	}
	if environmentID.Valid {
		value := domain.EnvironmentID(environmentID.String)
		alert.EnvironmentID = &value
	}
	if resourceID.Valid {
		value := domain.ResourceID(resourceID.String)
		alert.ResourceID = &value
	}
	if acknowledgedAtUS.Valid {
		value := time.UnixMicro(acknowledgedAtUS.Int64).UTC()
		alert.AcknowledgedAt = &value
	}
	if resolvedAtUS.Valid {
		value := time.UnixMicro(resolvedAtUS.Int64).UTC()
		alert.ResolvedAt = &value
	}
	if err := alert.Validate(); err != nil {
		return domain.Alert{}, err
	}
	return alert, nil
}
