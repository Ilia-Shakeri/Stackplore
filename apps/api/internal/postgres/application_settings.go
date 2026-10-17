package postgres

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/appsettings"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

var _ appsettings.Repository = (*Store)(nil)

const applicationColumns = `row_version,updated_at_us,metric_retention_hours,health_retention_hours,event_retention_hours,delivery_retention_hours,log_line_limit,default_alert_window_seconds,default_alert_pending_seconds`

func scanApplicationSettings(row rowScanner) (domain.ApplicationSettingsRecord, error) {
	var record domain.ApplicationSettingsRecord
	var at int64
	err := row.Scan(&record.Version, &at, &record.Values.MetricRetentionHours, &record.Values.HealthRetentionHours, &record.Values.EventRetentionHours, &record.Values.DeliveryRetentionHours, &record.Values.LogLineLimit, &record.Values.DefaultAlertWindowSeconds, &record.Values.DefaultAlertPendingSeconds)
	if err != nil {
		return domain.ApplicationSettingsRecord{}, err
	}
	record.UpdatedAt = time.UnixMicro(at).UTC()
	if record.Validate() != nil {
		return domain.ApplicationSettingsRecord{}, domain.ErrRepositoryUnavailable
	}
	return record, nil
}

func (store *Store) ReadApplicationSettings(ctx context.Context) (domain.ApplicationSettingsRecord, error) {
	if ctx == nil || !store.available() {
		return domain.ApplicationSettingsRecord{}, appsettings.ErrUnavailable
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	record, err := scanApplicationSettings(store.pool.QueryRow(ctx, `SELECT `+applicationColumns+` FROM application_settings WHERE singleton=1`))
	if err != nil {
		return domain.ApplicationSettingsRecord{}, applicationError(err)
	}
	return record, nil
}

func (store *Store) ReadAuditPolicyStatus(ctx context.Context) (domain.AuditPolicyStatus, error) {
	if ctx == nil || !store.available() {
		return domain.AuditPolicyStatus{}, appsettings.ErrUnavailable
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	status := domain.AuditPolicyStatus{MinimumRetentionDays: domain.MinimumAuditRetentionDays}
	var archived *int64
	err := store.pool.QueryRow(ctx, `SELECT retention_days,quota_rows,retained_rows,archive_verified_through_us FROM audit_lifecycle_state WHERE singleton=1`).Scan(&status.RetentionDays, &status.QuotaRows, &status.StoredRows, &archived)
	if err != nil || status.RetentionDays < 90 || status.RetentionDays > 3650 || status.QuotaRows < 1000 || status.QuotaRows > 10000000 || status.StoredRows < 0 {
		return domain.AuditPolicyStatus{}, appsettings.ErrUnavailable
	}
	status.QuotaExceeded = status.StoredRows >= status.QuotaRows
	if archived != nil {
		value := time.UnixMicro(*archived).UTC()
		status.ArchiveVerifiedThrough = &value
	}
	return status, nil
}

func (store *Store) ConfigureAuditLifecyclePolicy(ctx context.Context, policy domain.AuditLifecyclePolicy) error {
	if ctx == nil || policy.Validate() != nil || !store.available() {
		return appsettings.ErrInvalid
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	err := store.withinAuthTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		tx := transaction.transaction
		var days int
		var quota int64
		var archived *int64
		var err error
		err = tx.QueryRow(ctx, `SELECT retention_days,quota_rows,archive_verified_through_us FROM audit_lifecycle_state WHERE singleton=1 FOR UPDATE`).Scan(&days, &quota, &archived)
		if err != nil {
			return err
		}
		var nextArchive *int64
		if !policy.ArchiveVerifiedThrough.IsZero() {
			v := policy.ArchiveVerifiedThrough.UnixMicro()
			nextArchive = &v
		}
		if archived != nil && (nextArchive == nil || *nextArchive < *archived) {
			return appsettings.ErrInvalid
		}
		changed := days != policy.RetentionDays || quota != policy.QuotaRows || !sameOptionalInt64(archived, nextArchive)
		if changed {
			if _, err = tx.Exec(ctx, `UPDATE audit_lifecycle_state SET retention_days=$1,quota_rows=$2,archive_verified_through_us=$3,updated_at_us=(extract(epoch FROM clock_timestamp())*1000000)::BIGINT WHERE singleton=1`, policy.RetentionDays, policy.QuotaRows, nextArchive); err != nil {
				return err
			}
			entry, genErr := systemAuditEntry("audit.policy_update", "audit.policy_reconciled")
			if genErr != nil {
				return genErr
			}
			if err = insertAuthAuditTx(ctx, tx, entry); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return appsettings.ErrUnavailable
	}
	return nil
}

func sameOptionalInt64(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (store *Store) PruneAuditHistory(ctx context.Context, limit int) (int64, error) {
	if ctx == nil || limit < 1 || limit > 1000 || !store.available() {
		return 0, appsettings.ErrInvalid
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	var removed int64
	err := store.withinAuthTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		tx := transaction.transaction
		var days int
		var archive *int64
		var err error
		if err = tx.QueryRow(ctx, `SELECT retention_days,archive_verified_through_us FROM audit_lifecycle_state WHERE singleton=1 FOR UPDATE`).Scan(&days, &archive); err != nil {
			return err
		}
		if archive == nil {
			return nil
		}
		var ids []string
		err = tx.QueryRow(ctx, `SELECT COALESCE(array_agg(id),ARRAY[]::TEXT[]) FROM (SELECT a.id FROM audit_entries a WHERE a.occurred_at_us <= LEAST((extract(epoch FROM clock_timestamp())*1000000)::BIGINT-$1::BIGINT*86400000000,$2) AND NOT EXISTS(SELECT 1 FROM operation_executions o WHERE o.intent_audit_id=a.id OR o.result_audit_id=a.id) AND NOT EXISTS(SELECT 1 FROM notification_deliveries n WHERE n.audit_intent_id=a.id) ORDER BY a.occurred_at_us,a.id LIMIT $3 FOR UPDATE OF a SKIP LOCKED) candidates`, int64(days), *archive, limit).Scan(&ids)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		entry, genErr := systemAuditEntry("audit.retention", "audit.retention_pruned")
		if genErr != nil {
			return genErr
		}
		if err = insertAuthAuditTx(ctx, tx, entry); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `SELECT set_config('stackplore.audit_lifecycle','on',true)`); err != nil {
			return err
		}
		result, err := tx.Exec(ctx, `DELETE FROM audit_entries WHERE id=ANY($1::TEXT[])`, ids)
		if err != nil {
			return err
		}
		removed = result.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, appsettings.ErrUnavailable
	}
	return removed, nil
}

func systemAuditEntry(action, reason string) (domain.AuditEntry, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return domain.AuditEntry{}, fmt.Errorf("audit id unavailable")
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	entryID, requestID := hex.EncodeToString(random[:16]), hex.EncodeToString(random[16:])
	entry := domain.AuditEntry{ID: domain.AuditEntryID("audit_" + entryID), OccurredAt: at, Actor: domain.AuditActor{Kind: domain.AuditActorSystem}, Action: domain.AuditAction(action), Target: domain.AuditTarget{Kind: "audit.history", ID: "lifecycle"}, Outcome: domain.AuditSuccess, RequestID: domain.RequestID("req_" + requestID), ReasonCode: domain.ReasonCode(reason)}
	if entry.Validate() != nil {
		return domain.AuditEntry{}, fmt.Errorf("audit entry invalid")
	}
	return entry, nil
}

type applicationMaterial struct {
	Values  domain.ApplicationValues `json:"values"`
	Classes []domain.HistoryClass    `json:"classes"`
}

func (store *Store) CreateApplicationPreview(ctx context.Context, command appsettings.PreviewCommand) (appsettings.StoredPreview, error) {
	if ctx == nil || command.Validate() != nil {
		return appsettings.StoredPreview{}, appsettings.ErrInvalid
	}
	var result appsettings.StoredPreview
	err := store.withinAuthTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		tx := transaction.transaction
		if err := requireLivePrincipalTx(ctx, tx, command.Actor, command.CreatedAt, true); err != nil {
			return err
		}
		settings, err := scanApplicationSettings(tx.QueryRow(ctx, `SELECT `+applicationColumns+` FROM application_settings WHERE singleton=1 FOR UPDATE`))
		if err != nil {
			return err
		}
		if settings.Version != command.ExpectedVersion {
			return appsettings.ErrVersion
		}
		impact := domain.HistoryImpact{Classes: []domain.HistoryImpactClass{}, MaximumRows: domain.MaximumHistoryCleanupRows, BackupRequired: command.Kind == appsettings.PreviewCleanup}
		classes := command.Classes
		if command.Kind == appsettings.PreviewSettings {
			classes = []domain.HistoryClass{}
			for _, class := range domain.HistoryClasses() {
				if command.Values.RetentionHours(class) < settings.Values.RetentionHours(class) {
					classes = append(classes, class)
				}
			}
			impact.ExpiryPolicyChanged = len(classes) > 0
			impact.BackupRequired = impact.ExpiryPolicyChanged
		}
		for _, class := range classes {
			values := settings.Values
			if command.Kind == appsettings.PreviewSettings {
				values = command.Values
			}
			cutoff := command.CreatedAt.Add(-time.Duration(values.RetentionHours(class)) * time.Hour)
			var lower *time.Time
			if command.Kind == appsettings.PreviewSettings {
				value := command.CreatedAt.Add(-time.Duration(settings.Values.RetentionHours(class)) * time.Hour)
				lower = &value
			}
			count, err := countEligibleHistory(ctx, tx, class, cutoff, lower)
			if err != nil {
				return err
			}
			impact.Classes = append(impact.Classes, domain.HistoryImpactClass{Name: class, Cutoff: cutoff, EligibleRows: count, EligibleRowsCapped: count == domain.HistoryImpactCountCap})
		}
		if impact.Validate() != nil {
			return appsettings.ErrInvalid
		}
		material, err := json.Marshal(applicationMaterial{Values: command.Values, Classes: command.Classes})
		if err != nil {
			return err
		}
		encodedImpact, err := json.Marshal(impact)
		if err != nil {
			return err
		}
		if len(material) > 4096 || len(encodedImpact) > 4096 {
			return appsettings.ErrInvalid
		}
		if _, err := tx.Exec(ctx, `WITH expired AS (SELECT session_id,change_kind FROM application_change_confirmations WHERE expires_at_us<=$1 ORDER BY expires_at_us,session_id,change_kind LIMIT 1000 FOR UPDATE SKIP LOCKED)
DELETE FROM application_change_confirmations AS item USING expired WHERE item.session_id=expired.session_id AND item.change_kind=expired.change_kind`, command.CreatedAt.UnixMicro()); err != nil {
			return err
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM (SELECT 1 FROM application_change_confirmations WHERE NOT(session_id=$1 AND change_kind=$2) LIMIT 4096) AS bounded`, string(command.Actor.SessionID), string(command.Kind)).Scan(&count); err != nil {
			return err
		}
		if count >= 4096 {
			return appsettings.ErrUnavailable
		}
		_, err = tx.Exec(ctx, `INSERT INTO application_change_confirmations(session_id,user_id,change_kind,expected_version,token_digest,requested,impact,created_at_us,expires_at_us)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(session_id,change_kind) DO UPDATE SET user_id=EXCLUDED.user_id,expected_version=EXCLUDED.expected_version,token_digest=EXCLUDED.token_digest,requested=EXCLUDED.requested,impact=EXCLUDED.impact,created_at_us=EXCLUDED.created_at_us,expires_at_us=EXCLUDED.expires_at_us`, string(command.Actor.SessionID), string(command.Actor.UserID), string(command.Kind), command.ExpectedVersion, command.Digest[:], material, encodedImpact, command.CreatedAt.UnixMicro(), command.ExpiresAt.UnixMicro())
		if err != nil {
			return err
		}
		result = appsettings.StoredPreview{Settings: settings, Impact: impact, ExpiresAt: command.ExpiresAt}
		return nil
	})
	if err != nil {
		return appsettings.StoredPreview{}, applicationError(err)
	}
	return result, nil
}

func historyTarget(class domain.HistoryClass) (table, column, guard string, err error) {
	switch class {
	case domain.HistoryMetricSamples:
		return "metric_samples", "sampled_at_us", "item.synthetic=0 AND " + historicalMetricPredicate, nil
	case domain.HistoryHealth:
		return "health_history", "observed_at_us", "item.synthetic=0", nil
	case domain.HistoryEvents:
		return "infrastructure_events", "occurred_at_us", "item.synthetic=0", nil
	case domain.HistoryTerminalDeliveries:
		return "notification_deliveries", "terminal_at_us", "item.audit_intent_id IS NULL", nil
	default:
		return "", "", "", appsettings.ErrInvalid
	}
}

func countEligibleHistory(ctx context.Context, tx pgx.Tx, class domain.HistoryClass, cutoff time.Time, lower *time.Time) (int, error) {
	table, column, guard, err := historyTarget(class)
	if err != nil {
		return 0, err
	}
	if cutoff.Location() != time.UTC || cutoff.UnixMicro() < 0 || cutoff.Nanosecond()%int(time.Microsecond) != 0 {
		return 0, appsettings.ErrInvalid
	}
	arguments := []any{cutoff.UnixMicro(), domain.HistoryImpactCountCap}
	if lower != nil {
		guard += ` AND item.` + column + `>=$3`
		arguments = append(arguments, lower.UnixMicro())
	}
	var count int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM (SELECT 1 FROM `+table+` AS item WHERE item.`+column+`<$1 AND `+guard+` LIMIT $2) AS bounded`, arguments...).Scan(&count)
	return count, err
}

func applicationCommitTime(ctx context.Context, tx pgx.Tx, command *appsettings.CommitCommand) error {
	var at time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
		return err
	}
	at = at.UTC().Truncate(time.Microsecond)
	if command.Grant.ConsumedAt.After(at) {
		at = command.Grant.ConsumedAt
	}
	command.Grant.ConsumedAt = at
	command.Audit.OccurredAt = at
	return requireLivePrincipalTx(ctx, tx, command.Actor, at, true)
}

func consumeApplicationPreview(ctx context.Context, tx pgx.Tx, command appsettings.CommitCommand, kind appsettings.PreviewKind) (applicationMaterial, domain.HistoryImpact, error) {
	var materialBytes, impactBytes []byte
	err := tx.QueryRow(ctx, `DELETE FROM application_change_confirmations WHERE session_id=$1 AND user_id=$2 AND change_kind=$3 AND expected_version=$4 AND token_digest=$5 AND created_at_us<=$6 AND expires_at_us>$6 RETURNING requested,impact`, string(command.Actor.SessionID), string(command.Actor.UserID), string(kind), command.ExpectedVersion, command.ConfirmationDigest[:], command.Grant.ConsumedAt.UnixMicro()).Scan(&materialBytes, &impactBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return applicationMaterial{}, domain.HistoryImpact{}, appsettings.ErrConfirmation
	}
	if err != nil {
		return applicationMaterial{}, domain.HistoryImpact{}, err
	}
	var material applicationMaterial
	var impact domain.HistoryImpact
	if decodeApplicationJSON(materialBytes, &material) != nil || decodeApplicationJSON(impactBytes, &impact) != nil || impact.Validate() != nil {
		return applicationMaterial{}, domain.HistoryImpact{}, appsettings.ErrUnavailable
	}
	if impact.BackupRequired && !command.BackupAcknowledged {
		return applicationMaterial{}, domain.HistoryImpact{}, appsettings.ErrBackup
	}
	return material, impact, nil
}

func decodeApplicationJSON(value []byte, destination any) error {
	if len(value) > 4096 {
		return appsettings.ErrUnavailable
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return appsettings.ErrUnavailable
	}
	return nil
}

func (store *Store) CommitApplicationSettings(ctx context.Context, command appsettings.CommitCommand) (domain.ApplicationSettingsRecord, error) {
	if ctx == nil || command.Values.Validate() != nil || command.Validate(auth.RecentAuthPurposeSettingsUpdate, "settings.update") != nil || !validRecentGrantUse(command.Grant, auth.RecentAuthPurposeSettingsUpdate, command.Grant.ConsumedAt) {
		return domain.ApplicationSettingsRecord{}, appsettings.ErrInvalid
	}
	var result domain.ApplicationSettingsRecord
	err := store.withinAuthTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		tx := transaction.transaction
		if err := applicationCommitTime(ctx, tx, &command); err != nil {
			return err
		}
		current, err := scanApplicationSettings(tx.QueryRow(ctx, `SELECT `+applicationColumns+` FROM application_settings WHERE singleton=1 FOR UPDATE`))
		if err != nil {
			return err
		}
		if current.Version != command.ExpectedVersion {
			return appsettings.ErrVersion
		}
		material, _, err := consumeApplicationPreview(ctx, tx, command, appsettings.PreviewSettings)
		if err != nil {
			return err
		}
		if material.Values != command.Values || len(material.Classes) != 0 {
			return appsettings.ErrConfirmation
		}
		if err := consumeRecentGrantTx(ctx, tx, command.Grant, auth.RecentAuthPurposeSettingsUpdate); err != nil {
			return err
		}
		values := command.Values
		result, err = scanApplicationSettings(tx.QueryRow(ctx, `UPDATE application_settings SET row_version=row_version+1,updated_at_us=$1,metric_retention_hours=$2,health_retention_hours=$3,event_retention_hours=$4,delivery_retention_hours=$5,log_line_limit=$6,default_alert_window_seconds=$7,default_alert_pending_seconds=$8 WHERE singleton=1 RETURNING `+applicationColumns, command.Grant.ConsumedAt.UnixMicro(), values.MetricRetentionHours, values.HealthRetentionHours, values.EventRetentionHours, values.DeliveryRetentionHours, values.LogLineLimit, values.DefaultAlertWindowSeconds, values.DefaultAlertPendingSeconds))
		if err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx, command.Audit)
	})
	if err != nil {
		return domain.ApplicationSettingsRecord{}, applicationError(err)
	}
	return result, nil
}

func (store *Store) CommitHistoryCleanup(ctx context.Context, command appsettings.CommitCommand) (appsettings.CleanupResult, error) {
	if ctx == nil || command.Values != (domain.ApplicationValues{}) || command.Validate(auth.RecentAuthPurposeDataCleanup, "data.cleanup") != nil || !validRecentGrantUse(command.Grant, auth.RecentAuthPurposeDataCleanup, command.Grant.ConsumedAt) {
		return appsettings.CleanupResult{}, appsettings.ErrInvalid
	}
	var result appsettings.CleanupResult
	err := store.withinAuthTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		tx := transaction.transaction
		if err := applicationCommitTime(ctx, tx, &command); err != nil {
			return err
		}
		current, err := scanApplicationSettings(tx.QueryRow(ctx, `SELECT `+applicationColumns+` FROM application_settings WHERE singleton=1 FOR UPDATE`))
		if err != nil {
			return err
		}
		if current.Version != command.ExpectedVersion {
			return appsettings.ErrVersion
		}
		material, impact, err := consumeApplicationPreview(ctx, tx, command, appsettings.PreviewCleanup)
		if err != nil {
			return err
		}
		if domain.ValidateHistoryClasses(material.Classes) != nil || material.Values != (domain.ApplicationValues{}) || len(material.Classes) != len(impact.Classes) || impact.ExpiryPolicyChanged || !impact.BackupRequired {
			return appsettings.ErrConfirmation
		}
		if err := consumeRecentGrantTx(ctx, tx, command.Grant, auth.RecentAuthPurposeDataCleanup); err != nil {
			return err
		}
		result = appsettings.CleanupResult{PolicyVersion: current.Version, Scope: append([]domain.HistoryClass{}, material.Classes...), Cutoffs: []domain.HistoryCutoff{}}
		for index, class := range material.Classes {
			if impact.Classes[index].Name != class {
				return appsettings.ErrConfirmation
			}
			cutoff := impact.Classes[index].Cutoff
			table, column, guard, err := historyTarget(class)
			if err != nil {
				return err
			}
			quota := domain.MaximumHistoryCleanupRows / len(material.Classes)
			if index < domain.MaximumHistoryCleanupRows%len(material.Classes) {
				quota++
			}
			tag, err := tx.Exec(ctx, `WITH expired AS (SELECT item.id FROM `+table+` AS item WHERE item.`+column+`<$1 AND `+guard+` ORDER BY item.`+column+`,item.id LIMIT $2 FOR UPDATE OF item SKIP LOCKED) DELETE FROM `+table+` AS item USING expired WHERE item.id=expired.id`, cutoff.UnixMicro(), quota)
			if err != nil {
				return err
			}
			result.RemovedRows += tag.RowsAffected()
			result.Cutoffs = append(result.Cutoffs, domain.HistoryCutoff{Name: class, Cutoff: cutoff})
			remaining, err := countEligibleHistory(ctx, tx, class, cutoff, nil)
			if err != nil {
				return err
			}
			result.RemainingRows = result.RemainingRows || remaining > 0
		}
		if err := insertAuthAuditTx(ctx, tx, command.Audit); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_attributes(audit_entry_id,attribute_key,value_kind,value_text) VALUES($1,'cleanup.removed_rows','count',$2)`, string(command.Audit.ID), strconv.FormatInt(result.RemovedRows, 10))
		return err
	})
	if err != nil {
		return appsettings.CleanupResult{}, applicationError(err)
	}
	return result, nil
}

func (store *Store) CommitConfigurationExport(ctx context.Context, command appsettings.ExportCommand) (appsettings.ConfigurationExport, error) {
	if ctx == nil || command.Actor.Validate() != nil || command.Actor.Role != auth.RoleAdministrator || command.Grant.Actor != command.Actor || !validRecentGrantUse(command.Grant, auth.RecentAuthPurposeConfigurationExport, command.Grant.ConsumedAt) || command.Audit.Validate() != nil || command.Audit.Action != "configuration.export" || command.Audit.Actor.ID != string(command.Actor.UserID) || command.Audit.Outcome != domain.AuditSuccess || command.Audit.OccurredAt != command.Grant.ConsumedAt || len(command.ApplicationVersion) < 1 || len(command.ApplicationVersion) > 64 || len(command.NotificationIntegrations) > 2 {
		return appsettings.ConfigurationExport{}, appsettings.ErrInvalid
	}
	seen := map[string]bool{}
	for _, integration := range command.NotificationIntegrations {
		if (integration.Kind != "webhook" && integration.Kind != "matrix") || seen[integration.Kind] || integration.Target != "[masked]" || integration.CredentialReference != "[masked]" {
			return appsettings.ConfigurationExport{}, appsettings.ErrInvalid
		}
		seen[integration.Kind] = true
	}
	var result appsettings.ConfigurationExport
	err := store.withinAuthTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		tx := transaction.transaction
		var at time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
			return err
		}
		at = at.UTC().Truncate(time.Microsecond)
		if command.Grant.ConsumedAt.After(at) {
			at = command.Grant.ConsumedAt
		}
		command.Grant.ConsumedAt = at
		command.Audit.OccurredAt = at
		if err := requireLivePrincipalTx(ctx, tx, command.Actor, at, true); err != nil {
			return err
		}
		settings, err := scanApplicationSettings(tx.QueryRow(ctx, `SELECT `+applicationColumns+` FROM application_settings WHERE singleton=1 FOR SHARE`))
		if err != nil {
			return err
		}
		result = appsettings.ConfigurationExport{ApplicationVersion: command.ApplicationVersion, Settings: settings.Values, Sources: []appsettings.ExportSource{}, NotificationIntegrations: append([]appsettings.ExportIntegration{}, command.NotificationIntegrations...)}
		rows, err := tx.Query(ctx, `SELECT provider,display_name,enabled,operator_profile_id FROM infrastructure_sources WHERE deleted_at_us IS NULL ORDER BY id COLLATE "C" LIMIT 101`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var source appsettings.ExportSource
			var enabled int64
			if err := rows.Scan(&source.Provider, &source.Name, &enabled, &source.Profile); err != nil {
				rows.Close()
				return err
			}
			source.Enabled = enabled == 1
			source.CredentialReference = "[masked]"
			source.Host = "[masked]"
			result.Sources = append(result.Sources, source)
		}
		readErr := rows.Err()
		rows.Close()
		if readErr != nil {
			return readErr
		}
		if len(result.Sources) > 100 {
			return appsettings.ErrUnavailable
		}
		encoded, err := json.Marshal(result)
		if err != nil || len(encoded) > 65536 {
			return appsettings.ErrUnavailable
		}
		if err := consumeRecentGrantTx(ctx, tx, command.Grant, auth.RecentAuthPurposeConfigurationExport); err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx, command.Audit)
	})
	if err != nil {
		return appsettings.ConfigurationExport{}, applicationError(err)
	}
	return result, nil
}

func applicationError(err error) error {
	for _, known := range []error{appsettings.ErrInvalid, appsettings.ErrForbidden, appsettings.ErrVersion, appsettings.ErrConfirmation, appsettings.ErrBackup, auth.ErrAuthorizationDenied, auth.ErrRecentAuthRejected} {
		if errors.Is(err, known) {
			return known
		}
	}
	if errors.Is(err, auth.ErrAuthRepositoryRejected) {
		return auth.ErrRecentAuthRejected
	}
	return appsettings.ErrUnavailable
}
