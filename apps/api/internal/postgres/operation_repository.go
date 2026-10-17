package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/operation"
	"github.com/jackc/pgx/v5"
	"time"
)

var _ operation.Repository = (*Store)(nil)

func operationTransactionTime(ctx context.Context, tx pgx.Tx) (time.Time, error) {
	var at time.Time
	err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at)
	return at.UTC().Truncate(time.Microsecond), err
}
func operationActor(ctx context.Context, tx pgx.Tx, actor auth.Principal, at time.Time, admin bool) error {
	if err := requireLivePrincipalTx(ctx, tx, actor, at, admin); err != nil {
		if errors.Is(err, auth.ErrAuthRepositoryRejected) {
			return auth.ErrAuthorizationDenied
		}
		return err
	}
	return nil
}
func lockOperationTarget(ctx context.Context, tx pgx.Tx, request domain.OperationRequest) error {
	var provider, source, environment, kind, scopeKind, scopeName, native, version string
	var enabled, synthetic int64
	err := tx.QueryRow(ctx, `SELECT resource.provider,resource.source_id,resource.environment_id,resource.kind,resource.scope_kind,resource.scope_name,resource.native_id,snapshot.resource_version,source.enabled,source.synthetic FROM resources resource JOIN infrastructure_sources source ON source.id=resource.source_id JOIN resource_snapshots snapshot ON snapshot.resource_id=resource.id WHERE resource.id=$1 AND source.id=$2 AND source.deleted_at_us IS NULL AND resource.retired_at_us IS NULL FOR SHARE OF source,resource,snapshot`, string(request.Resource.ID), string(request.Source.SourceID)).Scan(&provider, &source, &environment, &kind, &scopeKind, &scopeName, &native, &version, &enabled, &synthetic)
	if errors.Is(err, pgx.ErrNoRows) {
		return operation.ErrNotFound
	}
	if err != nil {
		return err
	}
	if enabled != 1 || synthetic != 0 {
		return operation.ErrDisabled
	}
	identity, err := domain.NewResourceIdentity(domain.ProviderKind(provider), domain.SourceID(source), domain.EnvironmentID(environment), domain.ResourceKind(kind), domain.Scope{Kind: domain.ScopeKind(scopeKind), Name: scopeName}, domain.NativeResourceID(native))
	if err != nil {
		return operation.ErrUnavailable
	}
	if request.Source.Provider != identity.Provider || request.Resource.Identity != identity || request.ExpectedVersion != version {
		return operation.ErrStale
	}
	return nil
}
func (s *Store) CreateConfirmation(ctx context.Context, actor auth.Principal, c operation.Confirmation) error {
	if actor.Validate() != nil || actor.Role != auth.RoleAdministrator || c.Validate() != nil || actor.UserID != c.UserID || actor.SessionID != c.SessionID {
		return operation.ErrInvalid
	}
	data, err := encodeOperationRequest(c.Request)
	if err != nil {
		return err
	}
	err = s.withinAuthTransaction(ctx, func(ctx context.Context, t *transaction) error {
		tx := t.transaction
		at, err := operationTransactionTime(ctx, tx)
		if err != nil {
			return err
		}
		if !at.Before(c.ExpiresAt) || c.CreatedAt.After(at.Add(time.Minute)) {
			return operation.ErrConfirmation
		}
		if err = operationActor(ctx, tx, actor, at, true); err != nil {
			return err
		}
		if err = lockOperationTarget(ctx, tx, c.Request); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `WITH expired AS (SELECT id FROM operation_confirmations WHERE expires_at_us<=$1 ORDER BY expires_at_us,id LIMIT 1000 FOR UPDATE SKIP LOCKED) DELETE FROM operation_confirmations c USING expired WHERE c.id=expired.id`, at.UnixMicro()); err != nil {
			return err
		}
		var count int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM (SELECT 1 FROM operation_confirmations LIMIT 4096) bounded`).Scan(&count); err != nil {
			return err
		}
		if count >= 4096 {
			return operation.ErrAdmission
		}
		_, err = tx.Exec(ctx, `INSERT INTO operation_confirmations(id,token_digest,fingerprint,user_id,session_id,source_id,resource_id,request,created_at_us,expires_at_us) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(session_id,source_id,resource_id) DO UPDATE SET id=EXCLUDED.id,token_digest=EXCLUDED.token_digest,fingerprint=EXCLUDED.fingerprint,request=EXCLUDED.request,created_at_us=EXCLUDED.created_at_us,expires_at_us=EXCLUDED.expires_at_us`, c.ID, c.Digest[:], c.ParametersFingerprint[:], string(c.UserID), string(c.SessionID), string(c.Request.Source.SourceID), string(c.Request.Resource.ID), data, c.CreatedAt.UnixMicro(), c.ExpiresAt.UnixMicro())
		return err
	})
	return operationRepositoryError(err)
}
func findOperationTx(ctx context.Context, tx pgx.Tx, actor auth.Principal, key [32]byte, request domain.OperationRequest) (operation.ExecutionRecord, bool, error) {
	record, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+` FROM operation_executions WHERE user_id=$1 AND idempotency_digest=$2`, string(actor.UserID), key[:]))
	if errors.Is(err, pgx.ErrNoRows) {
		return operation.ExecutionRecord{}, false, nil
	}
	if err != nil {
		return record, false, err
	}
	if record.SessionID != actor.SessionID || operation.RequestFingerprint(record.Request) != operation.RequestFingerprint(request) {
		return operation.ExecutionRecord{}, false, operation.ErrIdempotencyConflict
	}
	return record, true, nil
}
func (s *Store) FindExecution(ctx context.Context, actor auth.Principal, key [32]byte, request domain.OperationRequest) (operation.ExecutionRecord, bool, error) {
	var record operation.ExecutionRecord
	var found bool
	if actor.Validate() != nil || actor.Role != auth.RoleAdministrator || request.Validate() != nil {
		return record, false, operation.ErrInvalid
	}
	err := s.withinAuthTransaction(ctx, func(ctx context.Context, t *transaction) error {
		at, err := operationTransactionTime(ctx, t.transaction)
		if err != nil {
			return err
		}
		if err = operationActor(ctx, t.transaction, actor, at, true); err != nil {
			return err
		}
		record, found, err = findOperationTx(ctx, t.transaction, actor, key, request)
		return err
	})
	return record, found, operationRepositoryError(err)
}
func (s *Store) StartExecution(ctx context.Context, c operation.StartExecutionCommand) (operation.ExecutionRecord, bool, error) {
	var record operation.ExecutionRecord
	started := false
	if c.Validate() != nil {
		return record, false, operation.ErrInvalid
	}
	data, err := encodeOperationRequest(c.Request)
	if err != nil {
		return record, false, err
	}
	err = s.withinAuthTransaction(ctx, func(ctx context.Context, t *transaction) error {
		tx := t.transaction
		at, err := operationTransactionTime(ctx, tx)
		if err != nil {
			return err
		}
		if err = operationActor(ctx, tx, c.Actor, at, true); err != nil {
			return err
		}
		var found bool
		record, found, err = findOperationTx(ctx, tx, c.Actor, c.IdempotencyDigest, c.Request)
		if err != nil || found {
			return err
		}
		if c.CreatedAt.After(at.Add(time.Minute)) {
			return operation.ErrInvalid
		}
		if err = lockOperationTarget(ctx, tx, c.Request); err != nil {
			return err
		}
		var busy bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operation_executions WHERE source_id=$1 AND resource_id=$2 AND state='in_progress')`, string(c.Request.Source.SourceID), string(c.Request.Resource.ID)).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return operation.ErrInProgress
		}
		var global, user int
		if err = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE user_id=$1) FROM (SELECT user_id FROM operation_executions WHERE state='in_progress' LIMIT 65) bounded`, string(c.Actor.UserID)).Scan(&global, &user); err != nil {
			return err
		}
		if global >= operation.MaximumActiveExecutions || user >= operation.MaximumUserExecutions {
			return operation.ErrAdmission
		}
		var confirmationID string
		err = tx.QueryRow(ctx, `DELETE FROM operation_confirmations WHERE token_digest=$1 AND fingerprint=$2 AND user_id=$3 AND session_id=$4 AND source_id=$5 AND resource_id=$6 AND created_at_us<=$7 AND expires_at_us>$7 RETURNING id`, c.Digest[:], c.Fingerprint[:], string(c.Actor.UserID), string(c.Actor.SessionID), string(c.Request.Source.SourceID), string(c.Request.Resource.ID), at.UnixMicro()).Scan(&confirmationID)
		if errors.Is(err, pgx.ErrNoRows) {
			return operation.ErrConfirmation
		}
		if err != nil {
			return err
		}
		c.Grant.ConsumedAt = at
		if err = consumeRecentGrantTx(ctx, tx, c.Grant, auth.RecentAuthPurposeOperationExecute); err != nil {
			if errors.Is(err, auth.ErrAuthRepositoryRejected) {
				return auth.ErrRecentAuthRejected
			}
			return err
		}
		c.AuditIntent.OccurredAt = at
		if err = insertAuthAuditTx(ctx, tx, c.AuditIntent); err != nil {
			return err
		}
		lease := at.Add(c.LeaseUntil.Sub(c.CreatedAt))
		record, err = scanOperation(tx.QueryRow(ctx, `INSERT INTO operation_executions(id,user_id,session_id,source_id,resource_id,confirmation_id,idempotency_digest,fingerprint,request,request_id,state,created_at_us,updated_at_us,lease_until_us,intent_audit_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'in_progress',$11,$11,$12,$13) RETURNING `+operationColumns, c.ExecutionID, string(c.Actor.UserID), string(c.Actor.SessionID), string(c.Request.Source.SourceID), string(c.Request.Resource.ID), confirmationID, c.IdempotencyDigest[:], c.Fingerprint[:], data, string(c.RequestID), at.UnixMicro(), lease.UnixMicro(), string(c.AuditIntent.ID)))
		started = err == nil
		return err
	})
	if err != nil {
		started = false
	}
	return record, started, operationRepositoryError(err)
}
func (s *Store) CompleteExecution(ctx context.Context, c operation.CompletionCommand) (operation.ExecutionRecord, error) {
	var record operation.ExecutionRecord
	err := s.withinTransaction(ctx, func(ctx context.Context, t *transaction) error {
		tx := t.transaction
		var err error
		record, err = scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+` FROM operation_executions WHERE id=$1 FOR UPDATE`, c.ExecutionID))
		if err != nil {
			return err
		}
		if record.State != operation.ExecutionInProgress {
			return operation.ErrUncertain
		}
		if c.CompletedAt.Before(record.CreatedAt) {
			c.CompletedAt = record.CreatedAt
			c.AuditResult.OccurredAt = c.CompletedAt
		}
		if c.ValidateFor(record) != nil {
			return operation.ErrInvalid
		}
		if err = insertAuthAuditTx(ctx, tx, c.AuditResult); err != nil {
			return err
		}
		var result any
		if c.Result != nil {
			data, err := json.Marshal(c.Result)
			if err != nil {
				return operation.ErrInvalid
			}
			result = data
		}
		record, err = scanOperation(tx.QueryRow(ctx, `UPDATE operation_executions SET state=$1,result=$2,reason_code=$3,updated_at_us=$4,result_audit_id=$5 WHERE id=$6 AND state='in_progress' RETURNING `+operationColumns, string(c.State), result, string(c.ReasonCode), c.CompletedAt.UnixMicro(), string(c.AuditResult.ID), c.ExecutionID))
		return err
	})
	return record, operationRepositoryError(err)
}
func (s *Store) AppendOperationDenial(ctx context.Context, actor auth.Principal, entry domain.AuditEntry) error {
	if actor.Validate() != nil || entry.Validate() != nil || entry.Actor.Kind != domain.AuditActorUser || entry.Actor.ID != string(actor.UserID) || entry.Action != "infrastructure.operation" || entry.Outcome != domain.AuditDenied {
		return operation.ErrInvalid
	}
	return operationRepositoryError(s.withinAuthTransaction(ctx, func(ctx context.Context, t *transaction) error {
		at, err := operationTransactionTime(ctx, t.transaction)
		if err != nil {
			return err
		}
		if err = operationActor(ctx, t.transaction, actor, at, false); err != nil {
			return err
		}
		entry.OccurredAt = at
		return insertAuthAuditTx(ctx, t.transaction, entry)
	}))
}
func operationRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	for _, known := range []error{operation.ErrInvalid, operation.ErrDisabled, operation.ErrForbidden, operation.ErrStale, operation.ErrConfirmation, operation.ErrIdempotencyConflict, operation.ErrInProgress, operation.ErrUncertain, operation.ErrNotFound, operation.ErrAdmission, auth.ErrAuthorizationDenied, auth.ErrRecentAuthRejected} {
		if errors.Is(err, known) {
			return known
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return operation.ErrNotFound
	}
	return operation.ErrUnavailable
}
