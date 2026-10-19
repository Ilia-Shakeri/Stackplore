package postgres

import (
	"bytes"
	"context"
	"fmt"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/jackc/pgx/v5"
)

func (store *Store) SettleAssistantUsage(ctx context.Context, request assistant.UsageSettlement) error {
	if !assistant.Identifier(request.RunID) || request.Units < 0 || request.Units > assistant.MaximumQuotaUnits || !assistant.Digest(request.ReceiptDigest) || !request.Audit.Valid() {
		return assistant.ErrInvalid
	}
	return store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := assistantLock(ctx, tx); err != nil {
			return err
		}
		var owner, state, kind string
		settlementKind := "observed"
		if request.Conservative {
			settlementKind = "conservative"
		}
		var reserved, settled, minimum int64
		var receipt []byte
		err := tx.QueryRow(ctx, `SELECT COALESCE(owner_id,''),state,reserved_units,COALESCE(settled_units,-1),receipt_digest,minimum_units,COALESCE(settlement_kind,'') FROM assistant_usage_ledger WHERE reservation_id=$1 FOR UPDATE`, request.RunID).Scan(&owner, &state, &reserved, &settled, &receipt, &minimum, &kind)
		if err == pgx.ErrNoRows {
			return assistant.ErrDenied
		}
		if err != nil {
			return err
		}
		if state == "settled" {
			if settled == request.Units && kind == settlementKind && bytes.Equal(receipt, request.ReceiptDigest[:]) {
				return nil
			}
			return assistant.ErrConflict
		}
		if state != "unknown" || request.Units < minimum || request.Units > reserved || request.Conservative && request.Units != reserved {
			return assistant.ErrConflict
		}
		if _, err = tx.Exec(ctx, `UPDATE assistant_usage_ledger SET state='settled',settled_units=$2,receipt_digest=$3,updated_at_us=GREATEST(updated_at_us,$4),settlement_kind=$5 WHERE reservation_id=$1`, request.RunID, request.Units, request.ReceiptDigest[:], request.Audit.At.UnixMicro(), settlementKind); err != nil {
			return err
		}
		keys := []string{"installation"}
		if owner != "" {
			keys = append(keys, "user:"+owner)
		}
		for _, key := range keys {
			command, err := tx.Exec(ctx, `UPDATE assistant_usage_accounts SET reserved_units=reserved_units-$1,used_units=(CASE WHEN period_version=(SELECT quota_period FROM assistant_control WHERE singleton_id=1) THEN used_units ELSE 0 END)+$2,period_version=(SELECT quota_period FROM assistant_control WHERE singleton_id=1) WHERE account_key=$3 AND reserved_units>=$1`, reserved, request.Units, key)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return assistant.ErrUnavailable
			}
		}
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM assistant_runs WHERE id=$1)`, request.RunID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			r, err := assistantRead(ctx, tx, request.RunID)
			if err != nil {
				return err
			}
			if r.UsageUncertain {
				now, err := assistantNow(ctx, tx)
				if err != nil {
					return err
				}
				r.UsageUncertain = false
				r.UsageConservative = request.Conservative
				r.UsedUnits = request.Units
				r.Version++
				r.UpdatedAt = now
				if _, err = tx.Exec(ctx, `UPDATE assistant_runs SET usage_unknown=FALSE,row_version=$2,updated_at_us=$3,used_units=$4,usage_conservative=$5 WHERE id=$1`, r.ID, r.Version, now.UnixMicro(), r.UsedUnits, r.UsageConservative); err != nil {
					return err
				}
				if err = assistantEvent(ctx, tx, r.Run, now); err != nil {
					return err
				}
			}
		}
		action := "assistant.usage_settle"
		if request.Conservative {
			action = "assistant.usage_conservative"
		}
		return assistantAudit(ctx, tx, request.Audit, action, request.RunID)
	})
}

func (store *Store) ResetAssistantQuota(ctx context.Context, a assistant.OperatorAudit) error {
	if !a.Valid() {
		return assistant.ErrInvalid
	}
	return store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := assistantLock(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE assistant_control SET quota_period=quota_period+1 WHERE singleton_id=1`); err != nil {
			return err
		}
		return assistantAudit(ctx, tx, a, "assistant.quota_reset", "installation")
	})
}

func (store *Store) SettleAssistantRestore(ctx context.Context, request assistant.RestoreSettlement) error {
	if request.Generation < 0 || request.ExposureUnits < 0 || request.ExposureUnits > assistant.MaximumQuotaUnits || !assistant.Digest(request.ReceiptDigest) || !request.Audit.Valid() {
		return assistant.ErrInvalid
	}
	return store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		control, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		var generation int64
		if err = tx.QueryRow(ctx, `SELECT restore_generation FROM installation_metadata WHERE singleton_id=1 FOR SHARE`).Scan(&generation); err != nil {
			return err
		}
		if generation != request.Generation {
			return assistant.ErrConflict
		}
		if control.paidGeneration == generation {
			var previous []byte
			if err = tx.QueryRow(ctx, `SELECT paid_receipt_digest FROM assistant_control WHERE singleton_id=1`).Scan(&previous); err != nil {
				return err
			}
			if bytes.Equal(previous, request.ReceiptDigest[:]) {
				return nil
			}
			return assistant.ErrConflict
		}
		if request.ExposureUnits > 0 {
			id := fmt.Sprintf("restore_%d_%d", generation, control.Version)
			_, err = tx.Exec(ctx, `INSERT INTO assistant_usage_ledger(reservation_id,reserved_units,state,restore_generation,created_at_us,updated_at_us,receipt_digest) VALUES($1,$2,'unknown',$3,$4,$4,$5)`, id, request.ExposureUnits, generation, request.Audit.At.UnixMicro(), request.ReceiptDigest[:])
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE assistant_usage_accounts SET reserved_units=reserved_units+$1 WHERE account_key='installation'`, request.ExposureUnits); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE assistant_control SET paid_generation=$1,paid_receipt_digest=$2 WHERE singleton_id=1`, generation, request.ReceiptDigest[:]); err != nil {
			return err
		}
		return assistantAudit(ctx, tx, request.Audit, "assistant.restore_settle", "installation")
	})
}
