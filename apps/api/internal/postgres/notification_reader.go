package postgres

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const notificationCursorVersion = 1

type notificationCursorState struct {
	Version     int    `json:"v"`
	CreatedAtUS int64  `json:"t"`
	ID          string `json:"id"`
}

func (store *Store) ListNotifications(ctx context.Context, request domain.PageRequest) (domain.NotificationPage, error) {
	if !store.available() || ctx == nil || request.Validate() != nil {
		return domain.NotificationPage{}, domain.ErrRepositoryInvalidInput
	}
	if ctx.Err() != nil {
		return domain.NotificationPage{}, errors.Join(domain.ErrRepositoryUnavailable, ctx.Err())
	}
	cursor, err := store.decodeNotificationCursor(request.Cursor)
	if err != nil {
		return domain.NotificationPage{}, err
	}
	queryContext, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	const columns = `SELECT id, source_id, alert_id, resource_id, notification_kind,
       severity, summary_code, created_at_us, synthetic
FROM notifications`
	statement := columns + ` ORDER BY created_at_us DESC, id COLLATE "C" ASC LIMIT $1`
	arguments := []any{request.Limit + 1}
	if cursor != nil {
		statement = columns + `
WHERE created_at_us < $1 OR (created_at_us = $1 AND id COLLATE "C" > $2 COLLATE "C")
ORDER BY created_at_us DESC, id COLLATE "C" ASC LIMIT $3`
		arguments = []any{cursor.CreatedAtUS, cursor.ID, request.Limit + 1}
	}
	rows, err := store.pool.Query(queryContext, statement, arguments...)
	if err != nil {
		return domain.NotificationPage{}, mapRepositoryReadError(queryContext, err)
	}
	defer rows.Close()
	notifications := make([]domain.Notification, 0, request.Limit+1)
	for rows.Next() {
		notification, err := scanNotification(rows)
		if err != nil {
			return domain.NotificationPage{}, mapRepositoryReadError(queryContext, err)
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return domain.NotificationPage{}, mapRepositoryReadError(queryContext, err)
	}
	page := domain.PageInfo{Limit: request.Limit}
	if len(notifications) > request.Limit {
		notifications = notifications[:request.Limit]
		page.HasMore = true
		page.NextCursor, err = store.encodeNotificationCursor(notifications[len(notifications)-1])
		if err != nil {
			return domain.NotificationPage{}, err
		}
	}
	result := domain.NotificationPage{Notifications: notifications, Page: page}
	if err := result.ValidateFor(request); err != nil {
		return domain.NotificationPage{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func scanNotification(row rowScanner) (domain.Notification, error) {
	var notification domain.Notification
	var sourceID, alertID, resourceID sql.NullString
	var kind, severity, summaryCode string
	var createdAtUS, synthetic int64
	if err := row.Scan(
		&notification.ID, &sourceID, &alertID, &resourceID, &kind,
		&severity, &summaryCode, &createdAtUS, &synthetic,
	); err != nil {
		return domain.Notification{}, err
	}
	if synthetic != 0 && synthetic != 1 {
		return domain.Notification{}, domain.ErrRepositoryUnavailable
	}
	if sourceID.Valid {
		value := domain.SourceID(sourceID.String)
		notification.SourceID = &value
	}
	if alertID.Valid {
		value := domain.AlertID(alertID.String)
		notification.AlertID = &value
	}
	if resourceID.Valid {
		value := domain.ResourceID(resourceID.String)
		notification.ResourceID = &value
	}
	notification.Kind = domain.ReasonCode(kind)
	notification.Severity = domain.Severity(severity)
	notification.SummaryCode = domain.ReasonCode(summaryCode)
	notification.CreatedAt = time.UnixMicro(createdAtUS).UTC()
	notification.Synthetic = synthetic == 1
	if err := notification.Validate(); err != nil {
		return domain.Notification{}, domain.ErrRepositoryUnavailable
	}
	return notification, nil
}

func (store *Store) encodeNotificationCursor(notification domain.Notification) (domain.PageCursor, error) {
	state := notificationCursorState{
		Version: notificationCursorVersion, CreatedAtUS: notification.CreatedAt.UnixMicro(), ID: string(notification.ID),
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return "", domain.ErrRepositoryUnavailable
	}
	digestKey := store.snapshotAuthDigestKey()
	defer clear(digestKey[:])
	mac := hmac.New(sha256.New, digestKey[:])
	_, _ = mac.Write([]byte("stackplore/notification-cursor/v1\x00"))
	_, _ = mac.Write(payload)
	signed := append(append(make([]byte, 0, len(payload)+sha256.Size), payload...), mac.Sum(nil)...)
	encoded := base64.RawURLEncoding.EncodeToString(signed)
	if len(encoded) > domain.MaxPageCursorBytes {
		return "", domain.ErrRepositoryUnavailable
	}
	return domain.PageCursor(encoded), nil
}

func (store *Store) decodeNotificationCursor(value domain.PageCursor) (*notificationCursorState, error) {
	if value == "" {
		return nil, nil
	}
	signed, err := base64.RawURLEncoding.Strict().DecodeString(string(value))
	if err != nil || base64.RawURLEncoding.EncodeToString(signed) != string(value) || len(signed) <= sha256.Size {
		return nil, domain.ErrRepositoryInvalidInput
	}
	payload := signed[:len(signed)-sha256.Size]
	signature := signed[len(signed)-sha256.Size:]
	digestKey := store.snapshotAuthDigestKey()
	defer clear(digestKey[:])
	mac := hmac.New(sha256.New, digestKey[:])
	_, _ = mac.Write([]byte("stackplore/notification-cursor/v1\x00"))
	_, _ = mac.Write(payload)
	if subtle.ConstantTimeCompare(signature, mac.Sum(nil)) != 1 {
		return nil, domain.ErrRepositoryInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var state notificationCursorState
	if err := decoder.Decode(&state); err != nil {
		return nil, domain.ErrRepositoryInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) || state.Version != notificationCursorVersion ||
		state.CreatedAtUS < 0 || domain.NotificationID(state.ID).Validate() != nil {
		return nil, domain.ErrRepositoryInvalidInput
	}
	return &state, nil
}

var _ domain.NotificationReader = (*Store)(nil)
