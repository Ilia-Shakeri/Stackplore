package postgres

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

const alertRuleCursorVersion = 1

type alertRuleCursorState struct {
	Version     int    `json:"v"`
	UpdatedAtUS int64  `json:"t"`
	ID          string `json:"id"`
}

const alertRuleColumns = `id, source_id, environment_id, resource_id, display_name, rule_kind,
       enabled, severity, comparison_operator, threshold_value, threshold_count, window_seconds,
       minimum_event_severity, evaluation_interval_seconds, pending_for_seconds,
       created_by_user_id, updated_by_user_id, created_at_us, updated_at_us, row_version,system_owned`

func (store *Store) ListAlertRules(ctx context.Context, request domain.PageRequest) (domain.AlertRulePage, error) {
	if !store.available() || ctx == nil || request.Validate() != nil {
		return domain.AlertRulePage{}, domain.ErrRepositoryInvalidInput
	}
	cursor, err := store.decodeAlertRuleCursor(request.Cursor)
	if err != nil {
		return domain.AlertRulePage{}, err
	}
	queryContext, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	statement := `SELECT ` + alertRuleColumns + ` FROM alert_rules
WHERE deleted_at_us IS NULL
ORDER BY updated_at_us DESC, id COLLATE "C" ASC LIMIT $1`
	arguments := []any{request.Limit + 1}
	if cursor != nil {
		statement = `SELECT ` + alertRuleColumns + ` FROM alert_rules
WHERE deleted_at_us IS NULL AND (updated_at_us < $1 OR (updated_at_us = $1 AND id COLLATE "C" > $2 COLLATE "C"))
ORDER BY updated_at_us DESC, id COLLATE "C" ASC LIMIT $3`
		arguments = []any{cursor.UpdatedAtUS, cursor.ID, request.Limit + 1}
	}
	rows, err := store.pool.Query(queryContext, statement, arguments...)
	if err != nil {
		return domain.AlertRulePage{}, mapRepositoryReadError(queryContext, err)
	}
	defer rows.Close()
	rules := make([]domain.AlertRuleRecord, 0, request.Limit+1)
	for rows.Next() {
		record, err := scanAlertRuleRecord(rows)
		if err != nil {
			return domain.AlertRulePage{}, domain.ErrRepositoryUnavailable
		}
		rules = append(rules, record)
	}
	if err := rows.Err(); err != nil {
		return domain.AlertRulePage{}, mapRepositoryReadError(queryContext, err)
	}
	page := domain.PageInfo{Limit: request.Limit}
	if len(rules) > request.Limit {
		rules = rules[:request.Limit]
		page.HasMore = true
		page.NextCursor, err = store.encodeAlertRuleCursor(rules[len(rules)-1])
		if err != nil {
			return domain.AlertRulePage{}, err
		}
	}
	result := domain.AlertRulePage{Rules: rules, Page: page}
	if result.ValidateFor(request) != nil {
		return domain.AlertRulePage{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func (store *Store) GetAlertRule(ctx context.Context, ruleID domain.AlertRuleID) (domain.AlertRuleRecord, error) {
	if !store.available() || ctx == nil || ruleID.Validate() != nil {
		return domain.AlertRuleRecord{}, domain.ErrRepositoryInvalidInput
	}
	queryContext, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	record, err := scanAlertRuleRecord(store.pool.QueryRow(queryContext,
		`SELECT `+alertRuleColumns+` FROM alert_rules WHERE id = $1 AND deleted_at_us IS NULL`, string(ruleID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AlertRuleRecord{}, domain.ErrRepositoryNotFound
	}
	if err != nil {
		return domain.AlertRuleRecord{}, mapRepositoryReadError(queryContext, err)
	}
	return record, nil
}

func (store *Store) CreateAlertRule(ctx context.Context, command domain.AlertRuleCreate) error {
	if store == nil || ctx == nil || command.Validate() != nil {
		return domain.ErrRepositoryInvalidInput
	}
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if err := lockAlertChanges(ctx, transaction); err != nil {
			return err
		}
		record := command.Record
		nextEvaluation := any(nil)
		if record.Rule.Enabled {
			nextEvaluation = record.CreatedAt.UnixMicro()
		}
		result, err := transaction.transaction.Exec(ctx, `INSERT INTO alert_rules (
    id, source_id, environment_id, resource_id, display_name, rule_kind, enabled, severity,
    comparison_operator, threshold_value, threshold_count, window_seconds, minimum_event_severity,
    evaluation_interval_seconds, pending_for_seconds, next_evaluation_at_us,
    created_by_user_id, updated_by_user_id, created_at_us, updated_at_us, row_version
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,1)`,
			string(record.Rule.ID), string(record.Rule.SourceID), optionalEnvironmentID(record.Rule.EnvironmentID),
			optionalResourceID(record.Rule.ResourceID), record.Rule.DisplayName, string(record.Rule.Kind),
			boolInteger(record.Rule.Enabled), string(record.Rule.Severity), optionalComparison(record.Rule.Comparison),
			optionalFloat(record.Rule.ThresholdValue), optionalCount(record.Rule.ThresholdCount), optionalDuration(record.Rule.Window),
			optionalSeverity(record.Rule.MinimumSeverity), int64(record.Rule.EvaluationInterval/time.Second),
			int64(record.Rule.PendingFor/time.Second), nextEvaluation, string(record.CreatedByUserID), string(record.UpdatedByUserID),
			record.CreatedAt.UnixMicro(), record.UpdatedAt.UnixMicro())
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return err
		}
		if err := appendAlertChange(ctx, transaction, record.Rule.SourceID, record.Rule.ID, nil, domain.AlertChangeRuleCreated, record.UpdatedAt); err != nil {
			return err
		}
		return (&transactionRepositories{transaction: transaction}).AppendAuditEntries(ctx, []domain.AuditEntry{command.Audit})
	})
	return mapAlertRuleWriteResult(err)
}

func (store *Store) ReplaceAlertRule(ctx context.Context, command domain.AlertRuleReplace) error {
	if store == nil || ctx == nil || command.Validate() != nil {
		return domain.ErrRepositoryInvalidInput
	}
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if err := lockAlertChanges(ctx, transaction); err != nil {
			return err
		}
		record := command.Record
		var previousSource domain.SourceID
		if err := transaction.transaction.QueryRow(ctx, `SELECT source_id FROM alert_rules WHERE id=$1 AND row_version=$2 AND deleted_at_us IS NULL FOR UPDATE`, string(record.Rule.ID), command.ExpectedVersion).Scan(&previousSource); err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return mapRepositoryWriteError(ctx, err)
			}
			return domain.ErrRepositoryConflict
		}
		if previousSource != record.Rule.SourceID {
			return domain.ErrRepositoryInvalidInput
		}
		nextEvaluation := any(nil)
		if record.Rule.Enabled {
			nextEvaluation = record.UpdatedAt.UnixMicro()
		}
		result, err := transaction.transaction.Exec(ctx, `UPDATE alert_rules SET
    source_id=$1, environment_id=$2, resource_id=$3, display_name=$4, rule_kind=$5,
    enabled=$6, severity=$7, comparison_operator=$8, threshold_value=$9, threshold_count=$10,
    window_seconds=$11, minimum_event_severity=$12, evaluation_interval_seconds=$13,
    pending_for_seconds=$14, last_evaluated_at_us=NULL, next_evaluation_at_us=$15,
    updated_by_user_id=$16, updated_at_us=$17, row_version=row_version+1,
    evaluation_generation=evaluation_generation+1,evaluation_lease_until_us=NULL
WHERE id=$18 AND row_version=$19 AND deleted_at_us IS NULL`,
			string(record.Rule.SourceID), optionalEnvironmentID(record.Rule.EnvironmentID), optionalResourceID(record.Rule.ResourceID),
			record.Rule.DisplayName, string(record.Rule.Kind), boolInteger(record.Rule.Enabled), string(record.Rule.Severity),
			optionalComparison(record.Rule.Comparison), optionalFloat(record.Rule.ThresholdValue), optionalCount(record.Rule.ThresholdCount),
			optionalDuration(record.Rule.Window), optionalSeverity(record.Rule.MinimumSeverity),
			int64(record.Rule.EvaluationInterval/time.Second), int64(record.Rule.PendingFor/time.Second), nextEvaluation,
			string(record.UpdatedByUserID), record.UpdatedAt.UnixMicro(), string(record.Rule.ID), command.ExpectedVersion)
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return err
		}
		if err := resolveRuleAlerts(ctx, transaction, record.Rule.ID, record.UpdatedAt); err != nil {
			return err
		}
		if err := appendAlertChange(ctx, transaction, previousSource, record.Rule.ID, nil, domain.AlertChangeRuleChanged, record.UpdatedAt); err != nil {
			return err
		}
		return (&transactionRepositories{transaction: transaction}).AppendAuditEntries(ctx, []domain.AuditEntry{command.Audit})
	})
	return mapAlertRuleWriteResult(err)
}

func (store *Store) DeleteAlertRule(ctx context.Context, command domain.AlertRuleDelete) error {
	if store == nil || ctx == nil || command.Validate() != nil {
		return domain.ErrRepositoryInvalidInput
	}
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if err := lockAlertChanges(ctx, transaction); err != nil {
			return err
		}
		var sourceID domain.SourceID
		if err := transaction.transaction.QueryRow(ctx, `SELECT source_id FROM alert_rules WHERE id=$1 AND row_version=$2 AND deleted_at_us IS NULL FOR UPDATE`, string(command.RuleID), command.ExpectedVersion).Scan(&sourceID); err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return mapRepositoryWriteError(ctx, err)
			}
			return domain.ErrRepositoryConflict
		}
		result, err := transaction.transaction.Exec(ctx, `UPDATE alert_rules SET enabled=0,
    last_evaluated_at_us=NULL, next_evaluation_at_us=NULL, updated_by_user_id=$1,
    updated_at_us=$2, deleted_at_us=$2, row_version=row_version+1,
    evaluation_generation=evaluation_generation+1,evaluation_lease_until_us=NULL
WHERE id=$3 AND row_version=$4 AND deleted_at_us IS NULL`,
			string(command.UpdatedByUserID), command.DeletedAt.UnixMicro(), string(command.RuleID), command.ExpectedVersion)
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return err
		}
		if err := resolveRuleAlerts(ctx, transaction, command.RuleID, command.DeletedAt); err != nil {
			return err
		}
		if err := appendAlertChange(ctx, transaction, sourceID, command.RuleID, nil, domain.AlertChangeRuleDeleted, command.DeletedAt); err != nil {
			return err
		}
		return (&transactionRepositories{transaction: transaction}).AppendAuditEntries(ctx, []domain.AuditEntry{command.Audit})
	})
	return mapAlertRuleWriteResult(err)
}

func scanAlertRuleRecord(row rowScanner) (domain.AlertRuleRecord, error) {
	var record domain.AlertRuleRecord
	var environmentID, resourceID, comparison, minimumSeverity sql.NullString
	var createdBy, updatedBy sql.NullString
	var thresholdValue sql.NullFloat64
	var thresholdCount, windowSeconds sql.NullInt64
	var kind, severity string
	var enabled, evaluationSeconds, pendingSeconds, createdAtUS, updatedAtUS int64
	if err := row.Scan(
		&record.Rule.ID, &record.Rule.SourceID, &environmentID, &resourceID, &record.Rule.DisplayName,
		&kind, &enabled, &severity, &comparison, &thresholdValue, &thresholdCount, &windowSeconds,
		&minimumSeverity, &evaluationSeconds, &pendingSeconds, &createdBy,
		&updatedBy, &createdAtUS, &updatedAtUS, &record.Version, &record.SystemOwned,
	); err != nil {
		return domain.AlertRuleRecord{}, err
	}
	if enabled < 0 || enabled > 1 {
		return domain.AlertRuleRecord{}, domain.ErrRepositoryUnavailable
	}
	record.Rule.Kind = domain.AlertRuleKind(kind)
	record.CreatedByUserID, record.UpdatedByUserID = domain.UserID(createdBy.String), domain.UserID(updatedBy.String)
	record.Rule.Enabled = enabled == 1
	record.Rule.Severity = domain.Severity(severity)
	record.Rule.EvaluationInterval = time.Duration(evaluationSeconds) * time.Second
	record.Rule.PendingFor = time.Duration(pendingSeconds) * time.Second
	record.CreatedAt = time.UnixMicro(createdAtUS).UTC()
	record.UpdatedAt = time.UnixMicro(updatedAtUS).UTC()
	if environmentID.Valid {
		value := domain.EnvironmentID(environmentID.String)
		record.Rule.EnvironmentID = &value
	}
	if resourceID.Valid {
		value := domain.ResourceID(resourceID.String)
		record.Rule.ResourceID = &value
	}
	if comparison.Valid {
		value := domain.ComparisonOperator(comparison.String)
		record.Rule.Comparison = &value
	}
	if thresholdValue.Valid {
		value := thresholdValue.Float64
		record.Rule.ThresholdValue = &value
	}
	if thresholdCount.Valid {
		value := uint64(thresholdCount.Int64)
		record.Rule.ThresholdCount = &value
	}
	if windowSeconds.Valid {
		record.Rule.Window = time.Duration(windowSeconds.Int64) * time.Second
	}
	if minimumSeverity.Valid {
		value := domain.Severity(minimumSeverity.String)
		record.Rule.MinimumSeverity = &value
	}
	if record.Validate() != nil {
		return domain.AlertRuleRecord{}, domain.ErrRepositoryUnavailable
	}
	return record, nil
}

func mapAlertRuleWriteResult(err error) error {
	if err == nil || errors.Is(err, domain.ErrRepositoryInvalidInput) || errors.Is(err, domain.ErrRepositoryConflict) {
		return err
	}
	return errors.Join(domain.ErrRepositoryUnavailable, err)
}

func optionalComparison(value *domain.ComparisonOperator) any {
	if value == nil {
		return nil
	}
	return string(*value)
}
func optionalFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}
func optionalCount(value *uint64) any {
	if value == nil {
		return nil
	}
	return int64(*value)
}
func optionalDuration(value time.Duration) any {
	if value == 0 {
		return nil
	}
	return int64(value / time.Second)
}
func optionalSeverity(value *domain.Severity) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func (store *Store) encodeAlertRuleCursor(record domain.AlertRuleRecord) (domain.PageCursor, error) {
	payload, err := json.Marshal(alertRuleCursorState{Version: alertRuleCursorVersion, UpdatedAtUS: record.UpdatedAt.UnixMicro(), ID: string(record.Rule.ID)})
	if err != nil {
		return "", domain.ErrRepositoryUnavailable
	}
	digestKey := store.snapshotAuthDigestKey()
	defer clear(digestKey[:])
	mac := hmac.New(sha256.New, digestKey[:])
	_, _ = mac.Write([]byte("stackplore/alert-rule-cursor/v1\x00"))
	_, _ = mac.Write(payload)
	signed := append(append(make([]byte, 0, len(payload)+sha256.Size), payload...), mac.Sum(nil)...)
	return domain.PageCursor(base64.RawURLEncoding.EncodeToString(signed)), nil
}

func (store *Store) decodeAlertRuleCursor(value domain.PageCursor) (*alertRuleCursorState, error) {
	if value == "" {
		return nil, nil
	}
	signed, err := base64.RawURLEncoding.Strict().DecodeString(string(value))
	if err != nil || base64.RawURLEncoding.EncodeToString(signed) != string(value) || len(signed) <= sha256.Size {
		return nil, domain.ErrRepositoryInvalidInput
	}
	payload, signature := signed[:len(signed)-sha256.Size], signed[len(signed)-sha256.Size:]
	digestKey := store.snapshotAuthDigestKey()
	defer clear(digestKey[:])
	mac := hmac.New(sha256.New, digestKey[:])
	_, _ = mac.Write([]byte("stackplore/alert-rule-cursor/v1\x00"))
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, domain.ErrRepositoryInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var state alertRuleCursorState
	if err := decoder.Decode(&state); err != nil {
		return nil, domain.ErrRepositoryInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) || state.Version != alertRuleCursorVersion || state.UpdatedAtUS < 0 || domain.AlertRuleID(state.ID).Validate() != nil {
		return nil, domain.ErrRepositoryInvalidInput
	}
	return &state, nil
}
