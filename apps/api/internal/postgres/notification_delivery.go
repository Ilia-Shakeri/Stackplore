package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
)

const selectUnqueuedWebhookNotifications = `SELECT notification.id, notification.source_id,
       notification.alert_id, notification.resource_id, notification.notification_kind,
       notification.severity, notification.summary_code, notification.created_at_us,
       notification.synthetic, notification.outbox_sequence
FROM notifications AS notification
WHERE notification.outbox_sequence > $1
ORDER BY notification.outbox_sequence
LIMIT $2`

const insertWebhookDelivery = `INSERT INTO notification_deliveries (
    id, notification_id, adapter_kind, delivery_purpose, audit_intent_id, destination_reference_id,
    render_revision, canonical_payload_bytes, payload_sha256, state, attempt_count,
    max_attempts, next_attempt_at_us, created_at_us, updated_at_us, row_version
) VALUES ($1, $2, $10, $3, $4, $5, 1, $6, $7, 'pending', 0, $8, $9, $9, $9, 1)
ON CONFLICT (id) DO NOTHING`

const recoverExpiredWebhookLeases = `WITH expired AS (
    SELECT id, attempt_count, max_attempts, updated_at_us
    FROM notification_deliveries
    WHERE adapter_kind = $3
      AND ($4 = '' OR destination_reference_id = $4)
      AND state = 'in_flight'
      AND lease_expires_at_us <= $1
    ORDER BY lease_expires_at_us, id
    FOR UPDATE SKIP LOCKED
    LIMIT $2
), closed_attempts AS (
    UPDATE notification_delivery_attempts AS attempt SET
        finished_at_us = GREATEST($1, attempt.started_at_us),
        outcome = 'uncertain',
        error_code = 'notification.lease_expired'
    FROM expired
    WHERE attempt.delivery_id = expired.id
      AND attempt.attempt_number = expired.attempt_count
      AND attempt.finished_at_us IS NULL
    RETURNING attempt.delivery_id
), recovered AS (
UPDATE notification_deliveries AS delivery SET
    state = CASE WHEN expired.attempt_count < expired.max_attempts THEN 'pending' ELSE 'uncertain' END,
    next_attempt_at_us = CASE WHEN expired.attempt_count < expired.max_attempts
        THEN GREATEST($1, expired.updated_at_us) ELSE NULL END,
    lease_owner = NULL,
    lease_expires_at_us = NULL,
    updated_at_us = GREATEST($1, expired.updated_at_us),
    terminal_at_us = CASE WHEN expired.attempt_count < expired.max_attempts
        THEN NULL ELSE GREATEST($1, expired.updated_at_us) END,
    error_code = CASE WHEN expired.attempt_count < expired.max_attempts
        THEN NULL ELSE 'notification.lease_expired' END,
    row_version = delivery.row_version + 1
FROM expired
WHERE delivery.id = expired.id
  AND EXISTS (SELECT 1 FROM closed_attempts WHERE closed_attempts.delivery_id = expired.id)
RETURNING delivery.id, delivery.audit_intent_id, delivery.state, delivery.updated_at_us
)
INSERT INTO audit_entries (
    id, occurred_at_us, actor_kind, actor_user_id, operator_id, action,
    target_kind, target_id, outcome, request_id, reason_code, source_id
)
SELECT 'audit_delivery_lease_' || recovered.id, recovered.updated_at_us,
       intent.actor_kind, intent.actor_user_id, intent.operator_id, intent.action,
       intent.target_kind, intent.target_id, 'uncertain', intent.request_id,
       'notification.test_delivery_uncertain', NULL
FROM recovered
JOIN audit_entries AS intent ON intent.id = recovered.audit_intent_id
WHERE recovered.state = 'uncertain'
  AND intent.action = 'notification.test'
  AND intent.outcome = 'intent'`

const claimWebhookDeliveries = `WITH due AS (
    SELECT id
    FROM notification_deliveries
    WHERE adapter_kind = $5
      AND ($6 = '' OR destination_reference_id = $6)
      AND EXISTS (SELECT 1 FROM notifications WHERE notifications.id = notification_deliveries.notification_id AND synthetic = 0)
      AND state = 'pending'
      AND next_attempt_at_us <= $1
      AND attempt_count < max_attempts
    ORDER BY next_attempt_at_us, id
    FOR UPDATE SKIP LOCKED
    LIMIT $2
), claimed AS (
    UPDATE notification_deliveries AS delivery SET
        state = 'in_flight',
        attempt_count = delivery.attempt_count + 1,
        next_attempt_at_us = NULL,
        lease_owner = $3,
        updated_at_us = GREATEST(delivery.updated_at_us, $1),
        lease_expires_at_us = GREATEST(delivery.updated_at_us, $1) + $4,
        row_version = delivery.row_version + 1
    FROM due
    WHERE delivery.id = due.id
    RETURNING delivery.id, delivery.notification_id, delivery.destination_reference_id,
        delivery.canonical_payload_bytes, delivery.attempt_count, delivery.max_attempts,
        delivery.lease_owner, delivery.lease_expires_at_us
)
SELECT id, notification_id, destination_reference_id, canonical_payload_bytes,
       attempt_count, max_attempts, lease_owner, lease_expires_at_us
FROM claimed
ORDER BY lease_expires_at_us, id`

var _ notification.DeliveryRepository = (*Store)(nil)
var _ notification.AdapterDeliveryRepository = (*Store)(nil)
var _ notification.TestRepository = (*Store)(nil)
var _ notification.AdapterTestRepository = (*Store)(nil)

type webhookPayload struct {
	Version        int    `json:"version"`
	NotificationID string `json:"notification_id"`
	SourceID       string `json:"source_id,omitempty"`
	AlertID        string `json:"alert_id,omitempty"`
	ResourceID     string `json:"resource_id,omitempty"`
	Kind           string `json:"kind"`
	Severity       string `json:"severity"`
	SummaryCode    string `json:"summary_code"`
	CreatedAt      string `json:"created_at"`
	Synthetic      bool   `json:"synthetic"`
}

func (store *Store) EnqueueWebhookDeliveries(
	ctx context.Context,
	destinationReferenceID string,
	maximumAttempts int,
	enqueuedAt time.Time,
	limit int,
) (int, error) {
	return store.EnqueueDeliveries(ctx, notification.DeliveryAdapterWebhook, destinationReferenceID, maximumAttempts, enqueuedAt, limit)
}

func (store *Store) EnqueueDeliveries(ctx context.Context, adapter notification.DeliveryAdapter, destinationReferenceID string, maximumAttempts int, enqueuedAt time.Time, limit int) (int, error) {
	if !validDeliveryRepositoryInput(store, ctx, destinationReferenceID, enqueuedAt, limit) ||
		maximumAttempts < 1 || maximumAttempts > 100 || !adapter.Valid() {
		return 0, domain.ErrRepositoryInvalidInput
	}
	created := 0
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if _, err := transaction.transaction.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, notificationOutboxLockKey); err != nil {
			return mapRepositoryWriteError(ctx, err)
		}
		if _, err := transaction.transaction.Exec(ctx, `INSERT INTO notification_delivery_cursors (
    destination_reference_id, last_notification_sequence, created_at_us, updated_at_us, adapter_kind
) VALUES ($1, 0, $2, $2, $3)
ON CONFLICT (adapter_kind, destination_reference_id) DO NOTHING`, destinationReferenceID, enqueuedAt.UnixMicro(), string(adapter)); err != nil {
			return mapRepositoryWriteError(ctx, err)
		}
		var lastSequence int64
		if err := transaction.transaction.QueryRow(ctx, `SELECT last_notification_sequence
FROM notification_delivery_cursors
WHERE destination_reference_id = $1 AND adapter_kind = $2
FOR UPDATE`, destinationReferenceID, string(adapter)).Scan(&lastSequence); err != nil {
			return mapRepositoryReadError(ctx, err)
		}
		rows, err := transaction.transaction.Query(ctx, selectUnqueuedWebhookNotifications, lastSequence, limit)
		if err != nil {
			return mapRepositoryReadError(ctx, err)
		}
		notifications := make([]sequencedNotification, 0, limit)
		for rows.Next() {
			item, err := scanSequencedNotification(rows)
			if err != nil {
				rows.Close()
				return domain.ErrRepositoryUnavailable
			}
			notifications = append(notifications, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return mapRepositoryReadError(ctx, err)
		}
		rows.Close()
		for _, item := range notifications {
			if item.Notification.Synthetic || item.Notification.Kind == notification.TestNotificationKind {
				continue
			}
			payload, deliveryID, digest, err := renderAdapterDelivery(adapter, destinationReferenceID, item.Notification)
			if err != nil {
				return domain.ErrRepositoryUnavailable
			}
			result, err := transaction.transaction.Exec(
				ctx, insertWebhookDelivery, deliveryID, string(item.Notification.ID), "alert", nil, destinationReferenceID,
				payload, digest[:], maximumAttempts, enqueuedAt.UnixMicro(), string(adapter),
			)
			if err != nil {
				return mapRepositoryWriteError(ctx, err)
			}
			if result.RowsAffected() == 1 {
				created++
			}
		}
		if len(notifications) > 0 {
			lastSequence = notifications[len(notifications)-1].Sequence
			result, err := transaction.transaction.Exec(ctx, `UPDATE notification_delivery_cursors SET
    last_notification_sequence = $1,
    updated_at_us = GREATEST(updated_at_us, $2)
WHERE destination_reference_id = $3 AND adapter_kind = $4`, lastSequence, enqueuedAt.UnixMicro(), destinationReferenceID, string(adapter))
			if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, mapRepositoryWriteError(ctx, err)
	}
	return created, nil
}

func (store *Store) CreateWebhookTest(ctx context.Context, command notification.WebhookTestCommand) error {
	return store.CreateNotificationTest(ctx, notification.DeliveryAdapterWebhook, command)
}

func (store *Store) CreateNotificationTest(ctx context.Context, adapter notification.DeliveryAdapter, command notification.WebhookTestCommand) error {
	if store == nil || ctx == nil || ctx.Err() != nil || command.Validate() != nil || !adapter.Valid() {
		return domain.ErrRepositoryInvalidInput
	}
	item := command.Notification
	payload, deliveryID, digest, err := renderAdapterDelivery(adapter, command.DestinationReferenceID, item)
	if err != nil {
		return domain.ErrRepositoryInvalidInput
	}
	err = store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if _, err := transaction.transaction.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, notificationOutboxLockKey); err != nil {
			return mapRepositoryWriteError(ctx, err)
		}
		result, err := transaction.transaction.Exec(ctx, insertNotificationStatement,
			string(item.ID), nil, nil, nil, string(item.Kind), string(item.Severity),
			string(item.SummaryCode), item.CreatedAt.UnixMicro(), 0,
		)
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return err
		}
		audit := command.Audit
		actorUserID, operatorID := auditActorColumns(audit.Actor)
		result, err = transaction.transaction.Exec(ctx, insertAuditEntryStatement,
			string(audit.ID), audit.OccurredAt.UnixMicro(), string(audit.Actor.Kind), actorUserID, operatorID,
			string(audit.Action), string(audit.Target.Kind), optionalString(audit.Target.ID), string(audit.Outcome),
			string(audit.RequestID), string(audit.ReasonCode),
		)
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return err
		}
		result, err = transaction.transaction.Exec(ctx, insertWebhookDelivery,
			deliveryID, string(item.ID), "test", string(audit.ID), command.DestinationReferenceID,
			payload, digest[:], command.MaximumAttempts, item.CreatedAt.UnixMicro(), string(adapter),
		)
		return exactInsertResult(ctx, result.RowsAffected(), err)
	})
	return mapRepositoryWriteError(ctx, err)
}

type sequencedNotification struct {
	Notification domain.Notification
	Sequence     int64
}

func scanSequencedNotification(row rowScanner) (sequencedNotification, error) {
	var result sequencedNotification
	var sourceID, alertID, resourceID sql.NullString
	var kind, severity, summaryCode string
	var createdAtUS, synthetic int64
	if err := row.Scan(
		&result.Notification.ID, &sourceID, &alertID, &resourceID, &kind,
		&severity, &summaryCode, &createdAtUS, &synthetic, &result.Sequence,
	); err != nil {
		return sequencedNotification{}, err
	}
	if synthetic < 0 || synthetic > 1 || result.Sequence < 1 {
		return sequencedNotification{}, domain.ErrRepositoryUnavailable
	}
	if sourceID.Valid {
		value := domain.SourceID(sourceID.String)
		result.Notification.SourceID = &value
	}
	if alertID.Valid {
		value := domain.AlertID(alertID.String)
		result.Notification.AlertID = &value
	}
	if resourceID.Valid {
		value := domain.ResourceID(resourceID.String)
		result.Notification.ResourceID = &value
	}
	result.Notification.Kind = domain.ReasonCode(kind)
	result.Notification.Severity = domain.Severity(severity)
	result.Notification.SummaryCode = domain.ReasonCode(summaryCode)
	result.Notification.CreatedAt = time.UnixMicro(createdAtUS).UTC()
	result.Notification.Synthetic = synthetic == 1
	if result.Notification.Validate() != nil {
		return sequencedNotification{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func (store *Store) ClaimWebhookDeliveries(
	ctx context.Context,
	leaseOwner string,
	claimedAt time.Time,
	leaseDuration time.Duration,
	limit int,
) ([]notification.DeliveryTask, error) {
	return store.claimDeliveries(ctx, notification.DeliveryAdapterWebhook, "", leaseOwner, claimedAt, leaseDuration, limit)
}

func (store *Store) ClaimDeliveries(ctx context.Context, adapter notification.DeliveryAdapter, reference, leaseOwner string, claimedAt time.Time, leaseDuration time.Duration, limit int) ([]notification.DeliveryTask, error) {
	if !validDeliveryToken(reference) {
		return nil, domain.ErrRepositoryInvalidInput
	}
	return store.claimDeliveries(ctx, adapter, reference, leaseOwner, claimedAt, leaseDuration, limit)
}

func (store *Store) claimDeliveries(ctx context.Context, adapter notification.DeliveryAdapter, reference, leaseOwner string, claimedAt time.Time, leaseDuration time.Duration, limit int) ([]notification.DeliveryTask, error) {
	if !validDeliveryRepositoryInput(store, ctx, leaseOwner, claimedAt, limit) ||
		leaseDuration < time.Second || leaseDuration > 10*time.Minute || leaseDuration%time.Microsecond != 0 || !adapter.Valid() {
		return nil, domain.ErrRepositoryInvalidInput
	}
	tasks := make([]notification.DeliveryTask, 0, limit)
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if _, err := transaction.transaction.Exec(ctx, recoverExpiredWebhookLeases, claimedAt.UnixMicro(), limit, string(adapter), reference); err != nil {
			return mapRepositoryWriteError(ctx, err)
		}
		rows, err := transaction.transaction.Query(
			ctx, claimWebhookDeliveries, claimedAt.UnixMicro(), limit, leaseOwner, leaseDuration.Microseconds(), string(adapter), reference,
		)
		if err != nil {
			return mapRepositoryReadError(ctx, err)
		}
		for rows.Next() {
			var task notification.DeliveryTask
			var leaseExpiresUS int64
			if err := rows.Scan(
				&task.ID, &task.NotificationID, &task.DestinationReferenceID, &task.Payload,
				&task.AttemptNumber, &task.MaximumAttempts, &task.LeaseOwner, &leaseExpiresUS,
			); err != nil {
				rows.Close()
				return domain.ErrRepositoryUnavailable
			}
			task.LeaseExpiresAt = time.UnixMicro(leaseExpiresUS).UTC()
			if task.Validate(claimedAt) != nil {
				rows.Close()
				return domain.ErrRepositoryUnavailable
			}
			tasks = append(tasks, task)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return mapRepositoryReadError(ctx, err)
		}
		rows.Close()
		for _, task := range tasks {
			result, err := transaction.transaction.Exec(ctx, `INSERT INTO notification_delivery_attempts (
    delivery_id, attempt_number, started_at_us
) VALUES ($1, $2, $3)`, task.ID, task.AttemptNumber, claimedAt.UnixMicro())
			if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, mapRepositoryWriteError(ctx, err)
	}
	return tasks, nil
}

func (store *Store) CompleteWebhookDelivery(ctx context.Context, completion notification.DeliveryCompletion) error {
	return store.completeDelivery(ctx, notification.DeliveryAdapterWebhook, "", completion)
}

func (store *Store) CompleteDelivery(ctx context.Context, adapter notification.DeliveryAdapter, reference string, completion notification.DeliveryCompletion) error {
	if !validDeliveryToken(reference) {
		return domain.ErrRepositoryInvalidInput
	}
	return store.completeDelivery(ctx, adapter, reference, completion)
}

func (store *Store) completeDelivery(ctx context.Context, adapter notification.DeliveryAdapter, reference string, completion notification.DeliveryCompletion) error {
	if !store.available() || ctx == nil || ctx.Err() != nil || completion.Validate() != nil || !adapter.Valid() {
		return domain.ErrRepositoryInvalidInput
	}
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		var state, purpose, storedAdapter, storedReference string
		var owner sql.NullString
		var leaseExpiresUS sql.NullInt64
		var auditIntentID sql.NullString
		var attemptCount, maximumAttempts int
		var claimedAtUS int64
		if err := transaction.transaction.QueryRow(ctx, `SELECT state, attempt_count, lease_owner, delivery_purpose, audit_intent_id, adapter_kind, destination_reference_id, lease_expires_at_us, max_attempts, updated_at_us
FROM notification_deliveries
WHERE id = $1
FOR UPDATE`, completion.DeliveryID).Scan(&state, &attemptCount, &owner, &purpose, &auditIntentID, &storedAdapter, &storedReference, &leaseExpiresUS, &maximumAttempts, &claimedAtUS); err != nil {
			return mapRepositoryReadError(ctx, err)
		}
		if state != "in_flight" || attemptCount != completion.AttemptNumber || !owner.Valid || owner.String != completion.LeaseOwner || storedAdapter != string(adapter) || reference != "" && storedReference != reference || !leaseExpiresUS.Valid || completion.FinishedAt.UnixMicro() >= leaseExpiresUS.Int64 || completion.FinishedAt.UnixMicro() < claimedAtUS || completion.RetryAt != nil && attemptCount >= maximumAttempts {
			return domain.ErrRepositoryConflict
		}
		result, err := transaction.transaction.Exec(ctx, `UPDATE notification_delivery_attempts SET
    finished_at_us = GREATEST($1, started_at_us), outcome = $2, response_status = $3, error_code = $4
WHERE delivery_id = $5 AND attempt_number = $6 AND finished_at_us IS NULL`,
			completion.FinishedAt.UnixMicro(), string(completion.Outcome), optionalInt(completion.ResponseStatus),
			optionalString(completion.ErrorCode), completion.DeliveryID, completion.AttemptNumber,
		)
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return err
		}
		terminalState := string(completion.Outcome)
		if completion.Outcome == notification.DeliverySucceeded {
			terminalState = "succeeded"
		} else if completion.Outcome == notification.DeliveryFailed {
			terminalState = "failed"
		}
		nextAttempt := optionalTime(completion.RetryAt)
		terminalAt := any(completion.FinishedAt.UnixMicro())
		errorCode := optionalString(completion.ErrorCode)
		if completion.RetryAt != nil {
			terminalState = "pending"
			terminalAt = nil
			errorCode = nil
		}
		result, err = transaction.transaction.Exec(ctx, `UPDATE notification_deliveries SET
    state = $1,
    next_attempt_at_us = CASE WHEN $2::bigint IS NULL THEN NULL
        ELSE GREATEST($2, updated_at_us, $3) END,
    lease_owner = NULL, lease_expires_at_us = NULL,
    updated_at_us = GREATEST(updated_at_us, $3),
    terminal_at_us = CASE WHEN $4::bigint IS NULL THEN NULL
        ELSE GREATEST($4, updated_at_us, $3) END,
    error_code = $5, row_version = row_version + 1
WHERE id = $6 AND state = 'in_flight' AND attempt_count = $7 AND lease_owner = $8`,
			terminalState, nextAttempt, completion.FinishedAt.UnixMicro(), terminalAt, errorCode,
			completion.DeliveryID, completion.AttemptNumber, completion.LeaseOwner,
		)
		if err := exactInsertResult(ctx, result.RowsAffected(), err); err != nil {
			return err
		}
		if completion.RetryAt == nil && purpose == "test" && auditIntentID.Valid {
			return insertNotificationTestResultAudit(ctx, transaction, auditIntentID.String, completion)
		}
		return nil
	})
	return mapRepositoryWriteError(ctx, err)
}

func insertNotificationTestResultAudit(
	ctx context.Context,
	transaction *transaction,
	auditIntentID string,
	completion notification.DeliveryCompletion,
) error {
	outcome, reason, ok := notificationTestAuditResult(completion.Outcome)
	if !ok {
		return domain.ErrRepositoryInvalidInput
	}
	digest := sha256.Sum256([]byte("stackplore-notification-test-result/v1\x00" + completion.DeliveryID))
	auditID := "audit_delivery_" + base64.RawURLEncoding.EncodeToString(digest[:])
	result, err := transaction.transaction.Exec(ctx, `INSERT INTO audit_entries (
    id, occurred_at_us, actor_kind, actor_user_id, operator_id, action,
    target_kind, target_id, outcome, request_id, reason_code, source_id
)
SELECT $1, $2, actor_kind, actor_user_id, operator_id, action,
       target_kind, target_id, $3, request_id, $4, NULL
FROM audit_entries
WHERE id = $5
  AND action = 'notification.test'
  AND outcome = 'intent'`, auditID, completion.FinishedAt.UnixMicro(), string(outcome), string(reason), auditIntentID)
	return exactInsertResult(ctx, result.RowsAffected(), err)
}

func notificationTestAuditResult(outcome notification.DeliveryOutcome) (domain.AuditOutcome, domain.ReasonCode, bool) {
	switch outcome {
	case notification.DeliverySucceeded:
		return domain.AuditSuccess, "notification.test_delivery_succeeded", true
	case notification.DeliveryFailed:
		return domain.AuditFailure, "notification.test_delivery_failed", true
	case notification.DeliveryUncertain:
		return domain.AuditUncertain, "notification.test_delivery_uncertain", true
	default:
		return "", "", false
	}
}

func renderWebhookDelivery(destinationReferenceID string, item domain.Notification) ([]byte, string, [sha256.Size]byte, error) {
	if item.Validate() != nil || !validDeliveryToken(destinationReferenceID) {
		return nil, "", [sha256.Size]byte{}, domain.ErrRepositoryInvalidInput
	}
	payload, err := json.Marshal(webhookPayload{
		Version: 1, NotificationID: string(item.ID), SourceID: optionalSourceText(item.SourceID),
		AlertID: optionalAlertText(item.AlertID), ResourceID: optionalResourceText(item.ResourceID),
		Kind: string(item.Kind), Severity: string(item.Severity), SummaryCode: string(item.SummaryCode),
		CreatedAt: item.CreatedAt.Format(time.RFC3339Nano), Synthetic: item.Synthetic,
	})
	if err != nil || len(payload) < 2 || len(payload) > 64*1024 {
		return nil, "", [sha256.Size]byte{}, domain.ErrRepositoryUnavailable
	}
	idInput := sha256.Sum256([]byte("stackplore-webhook/v1\x00" + destinationReferenceID + "\x00" + string(item.ID)))
	deliveryID := "delivery_" + base64.RawURLEncoding.EncodeToString(idInput[:])
	payloadDigest := sha256.Sum256(payload)
	return payload, deliveryID, payloadDigest, nil
}

func renderAdapterDelivery(adapter notification.DeliveryAdapter, reference string, item domain.Notification) ([]byte, string, [sha256.Size]byte, error) {
	if !adapter.Valid() || !validDeliveryToken(reference) || item.Validate() != nil || item.Synthetic {
		return nil, "", [sha256.Size]byte{}, domain.ErrRepositoryInvalidInput
	}
	if adapter == notification.DeliveryAdapterWebhook {
		return renderWebhookDelivery(reference, item)
	}
	payload, err := notification.RenderMatrixPayload(item)
	if err != nil {
		return nil, "", [sha256.Size]byte{}, domain.ErrRepositoryInvalidInput
	}
	idInput := sha256.Sum256([]byte("stackplore-matrix/v1\x00" + reference + "\x00" + string(item.ID)))
	deliveryID := "delivery_" + base64.RawURLEncoding.EncodeToString(idInput[:])
	digest := sha256.Sum256(payload)
	return payload, deliveryID, digest, nil
}

func validDeliveryRepositoryInput(store *Store, ctx context.Context, token string, at time.Time, limit int) bool {
	return store != nil && store.pool != nil && !store.closed.Load() && ctx != nil && ctx.Err() == nil &&
		validDeliveryToken(token) && !at.IsZero() && at.Location() == time.UTC &&
		at.Nanosecond()%int(time.Microsecond) == 0 && at.UnixMicro() >= 0 &&
		limit >= 1 && limit <= notification.MaximumDeliveryBatch
}

func validDeliveryToken(value string) bool {
	if len(value) < 1 || len(value) > 128 || !deliveryASCIILetter(value[0]) {
		return false
	}
	for _, char := range value[1:] {
		if char != '_' && char != '-' && (char < '0' || char > '9') &&
			(char < 'A' || char > 'Z') && (char < 'a' || char > 'z') {
			return false
		}
	}
	return true
}

func deliveryASCIILetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func optionalSourceText(value *domain.SourceID) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func optionalAlertText(value *domain.AlertID) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func optionalResourceText(value *domain.ResourceID) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func optionalInt(value *int) any {
	if value == nil {
		return nil
	}
	return int64(*value)
}
