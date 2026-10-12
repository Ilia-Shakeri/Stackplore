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

const auditCursorVersion = 1

type auditCursorState struct {
	Version      int    `json:"v"`
	OccurredAtUS int64  `json:"t"`
	ID           string `json:"id"`
}

func (store *Store) ListAuditEntries(ctx context.Context, request domain.PageRequest) (domain.AuditPage, error) {
	if !store.available() || ctx == nil || request.Validate() != nil {
		return domain.AuditPage{}, domain.ErrRepositoryInvalidInput
	}
	if ctx.Err() != nil {
		return domain.AuditPage{}, errors.Join(domain.ErrRepositoryUnavailable, ctx.Err())
	}
	cursor, err := store.decodeAuditCursor(request.Cursor)
	if err != nil {
		return domain.AuditPage{}, err
	}
	queryContext, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	const columns = `SELECT id, occurred_at_us, actor_kind, actor_user_id, operator_id,
       action, target_kind, target_id, outcome, request_id, reason_code
FROM audit_entries`
	const order = ` ORDER BY occurred_at_us DESC, id ASC LIMIT $1`
	statement := columns + order
	arguments := []any{request.Limit + 1}
	if cursor != nil {
		statement = columns + `
WHERE occurred_at_us < $1 OR (occurred_at_us = $1 AND id > $2)
ORDER BY occurred_at_us DESC, id ASC LIMIT $3`
		arguments = []any{cursor.OccurredAtUS, cursor.ID, request.Limit + 1}
	}
	rows, err := store.pool.Query(queryContext, statement, arguments...)
	if err != nil {
		return domain.AuditPage{}, mapRepositoryReadError(queryContext, err)
	}
	defer rows.Close()
	entries := make([]domain.AuditEntry, 0, request.Limit+1)
	for rows.Next() {
		var entry domain.AuditEntry
		var occurredAtUS int64
		var actorKind, action, targetKind, outcome, requestID, reasonCode string
		var actorUserID, operatorID, targetID sql.NullString
		if err := rows.Scan(
			&entry.ID, &occurredAtUS, &actorKind, &actorUserID, &operatorID,
			&action, &targetKind, &targetID, &outcome, &requestID, &reasonCode,
		); err != nil {
			return domain.AuditPage{}, mapRepositoryReadError(queryContext, err)
		}
		entry.OccurredAt = time.UnixMicro(occurredAtUS).UTC()
		entry.Actor = domain.AuditActor{Kind: domain.AuditActorKind(actorKind)}
		switch entry.Actor.Kind {
		case domain.AuditActorUser:
			entry.Actor.ID = actorUserID.String
		case domain.AuditActorOperator:
			entry.Actor.ID = operatorID.String
		}
		entry.Action = domain.AuditAction(action)
		entry.Target = domain.AuditTarget{Kind: domain.AuditTargetKind(targetKind), ID: targetID.String}
		entry.Outcome = domain.AuditOutcome(outcome)
		entry.RequestID = domain.RequestID(requestID)
		entry.ReasonCode = domain.ReasonCode(reasonCode)
		if entry.Validate() != nil {
			return domain.AuditPage{}, domain.ErrRepositoryUnavailable
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return domain.AuditPage{}, mapRepositoryReadError(queryContext, err)
	}
	page := domain.PageInfo{Limit: request.Limit}
	if len(entries) > request.Limit {
		entries = entries[:request.Limit]
		page.HasMore = true
		page.NextCursor, err = store.encodeAuditCursor(entries[len(entries)-1])
		if err != nil {
			return domain.AuditPage{}, err
		}
	}
	result := domain.AuditPage{Entries: entries, Page: page}
	if err := result.ValidateFor(request); err != nil {
		return domain.AuditPage{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func (store *Store) encodeAuditCursor(entry domain.AuditEntry) (domain.PageCursor, error) {
	state := auditCursorState{Version: auditCursorVersion, OccurredAtUS: entry.OccurredAt.UnixMicro(), ID: string(entry.ID)}
	payload, err := json.Marshal(state)
	if err != nil {
		return "", domain.ErrRepositoryUnavailable
	}
	digestKey := store.snapshotAuthDigestKey()
	defer clear(digestKey[:])
	mac := hmac.New(sha256.New, digestKey[:])
	_, _ = mac.Write([]byte("stackplore/audit-cursor/v1\x00"))
	_, _ = mac.Write(payload)
	signed := append(append(make([]byte, 0, len(payload)+sha256.Size), payload...), mac.Sum(nil)...)
	encoded := base64.RawURLEncoding.EncodeToString(signed)
	if len(encoded) > domain.MaxPageCursorBytes {
		return "", domain.ErrRepositoryUnavailable
	}
	return domain.PageCursor(encoded), nil
}

func (store *Store) decodeAuditCursor(value domain.PageCursor) (*auditCursorState, error) {
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
	_, _ = mac.Write([]byte("stackplore/audit-cursor/v1\x00"))
	_, _ = mac.Write(payload)
	if subtle.ConstantTimeCompare(signature, mac.Sum(nil)) != 1 {
		return nil, domain.ErrRepositoryInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var state auditCursorState
	if err := decoder.Decode(&state); err != nil {
		return nil, domain.ErrRepositoryInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) || state.Version != auditCursorVersion ||
		state.OccurredAtUS < 0 || domain.AuditEntryID(state.ID).Validate() != nil {
		return nil, domain.ErrRepositoryInvalidInput
	}
	return &state, nil
}

func mapRepositoryReadError(ctx context.Context, err error) error {
	if ctx != nil && ctx.Err() != nil {
		return errors.Join(domain.ErrRepositoryUnavailable, ctx.Err())
	}
	return domain.ErrRepositoryUnavailable
}

var _ domain.AuditEntryReader = (*Store)(nil)
