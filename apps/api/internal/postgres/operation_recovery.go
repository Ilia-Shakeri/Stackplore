package postgres

import (
	"context"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/operation"
	"time"
)

// Expired dispatches are never resent. The daemon may already have applied them.
func (s *Store) RecoverOrphanedOperations(ctx context.Context, now time.Time, limit int) (int, error) {
	if ctx == nil || now.IsZero() || now.Location() != time.UTC || now.Nanosecond()%int(time.Microsecond) != 0 || limit < 1 || limit > 100 {
		return 0, operation.ErrInvalid
	}
	count := 0
	err := s.withinTransaction(ctx, func(ctx context.Context, t *transaction) error {
		tx := t.transaction
		rows, err := tx.Query(ctx, `SELECT `+operationColumns+` FROM operation_executions WHERE state='in_progress' AND lease_until_us<=$1 ORDER BY lease_until_us,id LIMIT $2 FOR UPDATE SKIP LOCKED`, now.UnixMicro(), limit)
		if err != nil {
			return err
		}
		records := make([]operation.ExecutionRecord, 0, limit)
		for rows.Next() {
			record, err := scanOperation(rows)
			if err != nil {
				rows.Close()
				return err
			}
			records = append(records, record)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, record := range records {
			audit := domain.AuditEntry{ID: domain.AuditEntryID("audit_orphan_" + record.ID), OccurredAt: now, Actor: domain.AuditActor{Kind: domain.AuditActorSystem}, Action: "infrastructure.operation", Target: domain.AuditTarget{Kind: "infrastructure.resource", ID: string(record.Request.Resource.ID)}, Outcome: domain.AuditUncertain, RequestID: record.RequestID, ReasonCode: "operation.orphaned_uncertain"}
			if err = insertAuthAuditTx(ctx, tx, audit); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE operation_executions SET state='uncertain',reason_code='operation.orphaned_uncertain',updated_at_us=$1,result_audit_id=$2 WHERE id=$3 AND state='in_progress'`, now.UnixMicro(), string(audit.ID), record.ID); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	if err != nil {
		return 0, operationRepositoryError(err)
	}
	return count, nil
}
