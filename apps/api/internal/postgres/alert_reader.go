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

const alertCursorVersion = 1

type alertCursorState struct {
	Version     int    `json:"v"`
	SourceID    string `json:"source"`
	UpdatedAtUS int64  `json:"t"`
	ID          string `json:"id"`
}

func (store *Store) ListAlerts(ctx context.Context, sourceID domain.SourceID, request domain.PageRequest) (domain.AlertPage, error) {
	if !store.available() || ctx == nil || sourceID.Validate() != nil || request.Validate() != nil {
		return domain.AlertPage{}, domain.ErrRepositoryInvalidInput
	}
	if ctx.Err() != nil {
		return domain.AlertPage{}, errors.Join(domain.ErrRepositoryUnavailable, ctx.Err())
	}
	cursor, err := store.decodeAlertCursor(sourceID, request.Cursor)
	if err != nil {
		return domain.AlertPage{}, err
	}
	queryContext, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	const columns = `SELECT id, rule_id, source_id, environment_id, resource_id, state, severity,
       evidence_start_at_us, evidence_end_at_us, started_at_us, updated_at_us,
       acknowledged_at_us, resolved_at_us, summary_code, synthetic
FROM alerts`
	statement := columns + ` WHERE source_id = $1 ORDER BY updated_at_us DESC, id COLLATE "C" ASC LIMIT $2`
	arguments := []any{string(sourceID), request.Limit + 1}
	if cursor != nil {
		statement = columns + `
WHERE source_id = $1 AND updated_at_us <= $2 AND (updated_at_us < $2 OR (updated_at_us = $2 AND id COLLATE "C" > $3 COLLATE "C"))
ORDER BY updated_at_us DESC, id COLLATE "C" ASC LIMIT $4`
		arguments = []any{string(sourceID), cursor.UpdatedAtUS, cursor.ID, request.Limit + 1}
	}
	rows, err := store.pool.Query(queryContext, statement, arguments...)
	if err != nil {
		return domain.AlertPage{}, mapRepositoryReadError(queryContext, err)
	}
	defer rows.Close()
	alerts := make([]domain.Alert, 0, request.Limit+1)
	for rows.Next() {
		alert, err := scanAlert(rows)
		if err != nil {
			return domain.AlertPage{}, mapRepositoryReadError(queryContext, err)
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return domain.AlertPage{}, mapRepositoryReadError(queryContext, err)
	}
	page := domain.PageInfo{Limit: request.Limit}
	if len(alerts) > request.Limit {
		alerts = alerts[:request.Limit]
		page.HasMore = true
		page.NextCursor, err = store.encodeAlertCursor(sourceID, alerts[len(alerts)-1])
		if err != nil {
			return domain.AlertPage{}, err
		}
	}
	result := domain.AlertPage{Alerts: alerts, Page: page}
	if err := result.ValidateFor(sourceID, request); err != nil {
		return domain.AlertPage{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanAlert(row rowScanner) (domain.Alert, error) {
	var alert domain.Alert
	var environmentID, resourceID sql.NullString
	var evidenceStartUS, evidenceEndUS, startedAtUS, updatedAtUS int64
	var acknowledgedAtUS, resolvedAtUS sql.NullInt64
	var state, severity, summaryCode string
	var synthetic int64
	if err := row.Scan(
		&alert.ID, &alert.RuleID, &alert.SourceID, &environmentID, &resourceID, &state, &severity,
		&evidenceStartUS, &evidenceEndUS, &startedAtUS, &updatedAtUS,
		&acknowledgedAtUS, &resolvedAtUS, &summaryCode, &synthetic,
	); err != nil {
		return domain.Alert{}, err
	}
	if synthetic != 0 && synthetic != 1 {
		return domain.Alert{}, domain.ErrRepositoryUnavailable
	}
	if environmentID.Valid {
		value := domain.EnvironmentID(environmentID.String)
		alert.EnvironmentID = &value
	}
	if resourceID.Valid {
		value := domain.ResourceID(resourceID.String)
		alert.ResourceID = &value
	}
	alert.State = domain.AlertState(state)
	alert.Severity = domain.Severity(severity)
	alert.EvidenceRange = domain.TimeRange{Start: time.UnixMicro(evidenceStartUS).UTC(), End: time.UnixMicro(evidenceEndUS).UTC()}
	alert.StartedAt = time.UnixMicro(startedAtUS).UTC()
	alert.UpdatedAt = time.UnixMicro(updatedAtUS).UTC()
	if acknowledgedAtUS.Valid {
		value := time.UnixMicro(acknowledgedAtUS.Int64).UTC()
		alert.AcknowledgedAt = &value
	}
	if resolvedAtUS.Valid {
		value := time.UnixMicro(resolvedAtUS.Int64).UTC()
		alert.ResolvedAt = &value
	}
	alert.SummaryCode = domain.ReasonCode(summaryCode)
	alert.Synthetic = synthetic == 1
	if err := alert.Validate(); err != nil {
		return domain.Alert{}, domain.ErrRepositoryUnavailable
	}
	return alert, nil
}

func (store *Store) encodeAlertCursor(sourceID domain.SourceID, alert domain.Alert) (domain.PageCursor, error) {
	state := alertCursorState{
		Version: alertCursorVersion, SourceID: string(sourceID),
		UpdatedAtUS: alert.UpdatedAt.UnixMicro(), ID: string(alert.ID),
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return "", domain.ErrRepositoryUnavailable
	}
	digestKey := store.snapshotAuthDigestKey()
	defer clear(digestKey[:])
	mac := hmac.New(sha256.New, digestKey[:])
	_, _ = mac.Write([]byte("stackplore/alert-cursor/v1\x00"))
	_, _ = mac.Write(payload)
	signed := append(append(make([]byte, 0, len(payload)+sha256.Size), payload...), mac.Sum(nil)...)
	encoded := base64.RawURLEncoding.EncodeToString(signed)
	if len(encoded) > domain.MaxPageCursorBytes {
		return "", domain.ErrRepositoryUnavailable
	}
	return domain.PageCursor(encoded), nil
}

func (store *Store) decodeAlertCursor(sourceID domain.SourceID, value domain.PageCursor) (*alertCursorState, error) {
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
	_, _ = mac.Write([]byte("stackplore/alert-cursor/v1\x00"))
	_, _ = mac.Write(payload)
	if subtle.ConstantTimeCompare(signature, mac.Sum(nil)) != 1 {
		return nil, domain.ErrRepositoryInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var state alertCursorState
	if err := decoder.Decode(&state); err != nil {
		return nil, domain.ErrRepositoryInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) || state.Version != alertCursorVersion ||
		state.SourceID != string(sourceID) || state.UpdatedAtUS < 0 || domain.AlertID(state.ID).Validate() != nil {
		return nil, domain.ErrRepositoryInvalidInput
	}
	return &state, nil
}

var _ domain.AlertReader = (*Store)(nil)
