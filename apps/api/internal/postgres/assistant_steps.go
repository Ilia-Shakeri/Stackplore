package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/jackc/pgx/v5"
)

func assistantRead(ctx context.Context, tx pgx.Tx, id string) (assistantRecord, error) {
	r, err := assistantScan(tx.QueryRow(ctx, `SELECT `+assistantRunColumns+` FROM assistant_runs WHERE id=$1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, assistant.ErrDenied
	}
	return r, err
}
func assistantEarlier(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
func assistantVerifyLease(ctx context.Context, tx pgx.Tx, r assistantRecord, l assistant.Lease, checkVersion bool) (time.Time, error) {
	now, err := assistantNow(ctx, tx)
	if err != nil {
		return time.Time{}, err
	}
	if r.State != assistant.Running || checkVersion && r.Version != l.Version || r.Fence != l.Fence || r.LeaseOwner != l.Worker || !r.LeaseUntil.After(now) || !r.Deadline.After(now) {
		return now, assistant.ErrConflict
	}
	return now, nil
}

func assistantUsage(ctx context.Context, tx pgx.Tx, r assistantRecord, unknown bool, at time.Time) error {
	if unknown {
		_, err := tx.Exec(ctx, `UPDATE assistant_usage_ledger SET state='unknown',updated_at_us=$2,minimum_units=$3 WHERE reservation_id=$1 AND state='held'`, r.ID, at.UnixMicro(), r.UsedUnits)
		return err
	}
	command, err := tx.Exec(ctx, `UPDATE assistant_usage_ledger SET state='settled',settled_units=$2,updated_at_us=$3,settlement_kind='observed' WHERE reservation_id=$1 AND state='held'`, r.ID, r.UsedUnits, at.UnixMicro())
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return assistant.ErrConflict
	}
	for _, key := range []string{"installation", "user:" + r.OwnerID} {
		command, err = tx.Exec(ctx, `UPDATE assistant_usage_accounts SET reserved_units=reserved_units-$1,used_units=(CASE WHEN period_version=(SELECT quota_period FROM assistant_control WHERE singleton_id=1) THEN used_units ELSE 0 END)+$2,period_version=(SELECT quota_period FROM assistant_control WHERE singleton_id=1) WHERE account_key=$3 AND reserved_units>=$1`, r.ReservedUnits, r.UsedUnits, key)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return assistant.ErrUnavailable
		}
	}
	return nil
}

func assistantTerminal(ctx context.Context, tx pgx.Tx, r *assistantRecord, state assistant.State, at time.Time) error {
	var pendingInference bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM assistant_steps WHERE run_id=$1 AND kind='inference' AND state='running')`, r.ID).Scan(&pendingInference); err != nil {
		return err
	}
	unknown := pendingInference || state == assistant.Uncertain
	r.UsageUncertain = unknown
	if _, err := tx.Exec(ctx, `UPDATE assistant_steps SET state=CASE WHEN kind='inference' THEN 'uncertain' ELSE 'cancelled' END,finished_at_us=$2 WHERE run_id=$1 AND state='running'`, r.ID, at.UnixMicro()); err != nil {
		return err
	}
	r.State = state
	r.Version++
	r.Fence++
	r.UpdatedAt = at
	r.LeaseOwner = ""
	r.LeaseUntil = time.Time{}
	if _, err := tx.Exec(ctx, `UPDATE assistant_runs SET state=$2,row_version=$3,fence=$4,updated_at_us=$5,lease_owner=NULL,lease_until_us=NULL,used_units=$6,usage_unknown=$7 WHERE id=$1`, r.ID, r.State, r.Version, r.Fence, at.UnixMicro(), r.UsedUnits, r.UsageUncertain); err != nil {
		return err
	}
	if err := assistantUsage(ctx, tx, *r, unknown, at); err != nil {
		return err
	}
	return assistantEvent(ctx, tx, r.Run, at)
}

func (store *Store) ClaimAssistant(ctx context.Context, request assistant.Claim) (assistant.Run, bool, error) {
	if !assistant.Identifier(request.Worker) || !assistant.Identifier(request.ProcessID) || !assistant.Timestamp(request.At) || request.Lease < assistant.MinimumLeaseDuration || request.Lease > assistant.MaximumLeaseDuration {
		return assistant.Run{}, false, assistant.ErrInvalid
	}
	var result assistant.Run
	claimed := false
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		control, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		if !control.Enabled {
			return assistant.ErrDenied
		}
		request.At, err = assistantNow(ctx, tx)
		if err != nil {
			return err
		}
		var active int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM assistant_runs WHERE state='running'`).Scan(&active); err != nil {
			return err
		}
		if active >= 2 {
			return nil
		}
		rows, err := tx.Query(ctx, `SELECT id FROM assistant_runs AS candidate WHERE state='queued' AND created_at_us<=$1 AND NOT EXISTS(SELECT 1 FROM assistant_runs AS active WHERE active.owner_id=candidate.owner_id AND active.state='running') ORDER BY created_at_us,id LIMIT 16`, request.At.UnixMicro())
		if err != nil {
			return err
		}
		ids := make([]string, 0, 16)
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, id := range ids {
			r, err := assistantRead(ctx, tx, id)
			if err != nil {
				return err
			}
			denied := assistantAuthority(ctx, tx, control, r, request.At)
			if denied != nil && !errors.Is(denied, assistant.ErrDenied) {
				return denied
			}
			request.At, err = assistantNow(ctx, tx)
			if err != nil {
				return err
			}
			if denied != nil || !r.Deadline.After(request.At) || r.Attempts >= assistant.MaximumClaims || r.Volatile && r.memoryOwner != request.ProcessID {
				state := assistant.Interrupted
				if !r.Deadline.After(request.At) {
					state = assistant.Expired
				}
				if err = assistantTerminal(ctx, tx, &r, state, request.At); err != nil {
					return err
				}
				continue
			}
			r.State = assistant.Running
			r.Attempts++
			r.Version++
			r.Fence++
			r.LeaseOwner = request.Worker
			r.LeaseUntil = assistantEarlier(request.At.Add(request.Lease), r.Deadline)
			r.UpdatedAt = request.At
			_, err = tx.Exec(ctx, `UPDATE assistant_runs SET state='running',row_version=$2,fence=$3,lease_owner=$4,lease_until_us=$5,updated_at_us=$6,claim_attempts=$7 WHERE id=$1`, r.ID, r.Version, r.Fence, r.LeaseOwner, r.LeaseUntil.UnixMicro(), request.At.UnixMicro(), r.Attempts)
			if err != nil {
				return err
			}
			if err = assistantEvent(ctx, tx, r.Run, request.At); err != nil {
				return err
			}
			result = r.Run
			claimed = true
			return nil
		}
		return nil
	})
	return result, claimed, err
}

func (store *Store) BeginAssistantStep(ctx context.Context, request assistant.StepStart) (assistant.Run, assistant.Step, error) {
	if !request.Lease.Valid() || !request.Kind.Valid() || request.MaximumUnits < 0 || request.MaximumUnits > assistant.MaximumReservationUnits || request.Kind == assistant.Inference && request.MaximumUnits == 0 || request.Kind != assistant.Inference && request.MaximumUnits != 0 {
		return assistant.Run{}, assistant.Step{}, assistant.ErrInvalid
	}
	var result assistant.Run
	var step assistant.Step
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		control, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		r, err := assistantRead(ctx, tx, request.Lease.RunID)
		if err != nil {
			return err
		}
		if err = assistantAuthority(ctx, tx, control, r, request.Lease.At); err != nil {
			return err
		}
		request.Lease.At, err = assistantVerifyLease(ctx, tx, r, request.Lease, true)
		if err != nil {
			return err
		}
		if request.MaximumUnits > r.ReservedUnits-r.UsedUnits {
			return assistant.ErrQuota
		}
		if request.Kind == assistant.Inference && r.InferenceCalls >= assistant.MaximumInferenceCalls || request.Kind != assistant.Inference && r.ToolCalls >= assistant.MaximumToolCalls {
			return assistant.ErrQuota
		}
		var pending bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM assistant_steps WHERE run_id=$1 AND state='running')`, r.ID).Scan(&pending); err != nil {
			return err
		}
		if pending {
			return assistant.ErrConflict
		}
		step = assistant.Step{Number: r.ToolCalls + r.InferenceCalls + 1, Kind: request.Kind, State: "running", StartedAt: request.Lease.At, MaximumUnits: request.MaximumUnits}
		_, err = tx.Exec(ctx, `INSERT INTO assistant_steps(run_id,step_number,kind,state,fence,started_at_us,maximum_units) VALUES($1,$2,$3,'running',$4,$5,$6)`, r.ID, step.Number, step.Kind, r.Fence, step.StartedAt.UnixMicro(), step.MaximumUnits)
		if err != nil {
			return err
		}
		if request.Kind == assistant.Inference {
			r.InferenceCalls++
		} else {
			r.ToolCalls++
		}
		r.Version++
		r.UpdatedAt = request.Lease.At
		r.LeaseUntil = assistantEarlier(request.Lease.At.Add(assistant.MaximumLeaseDuration), r.Deadline)
		_, err = tx.Exec(ctx, `UPDATE assistant_runs SET tool_calls=$2,inference_calls=$3,row_version=$4,updated_at_us=$5,lease_until_us=$6 WHERE id=$1`, r.ID, r.ToolCalls, r.InferenceCalls, r.Version, r.UpdatedAt.UnixMicro(), r.LeaseUntil.UnixMicro())
		if err != nil {
			return err
		}
		result = r.Run
		return assistantEvent(ctx, tx, result, r.UpdatedAt)
	})
	return result, step, err
}

func (store *Store) FinishAssistantStep(ctx context.Context, request assistant.StepFinish) (assistant.Run, error) {
	if !request.Lease.Valid() || request.Number < 1 || request.Number > 15 || request.Units < 0 || request.Units > assistant.MaximumReservationUnits {
		return assistant.Run{}, assistant.ErrInvalid
	}
	var result assistant.Run
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		control, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		r, err := assistantRead(ctx, tx, request.Lease.RunID)
		if err != nil {
			return err
		}
		if err = assistantAuthority(ctx, tx, control, r, request.Lease.At); err != nil {
			return err
		}
		request.Lease.At, err = assistantVerifyLease(ctx, tx, r, request.Lease, true)
		if err != nil {
			return err
		}
		var kind assistant.StepKind
		var maximumUnits int64
		err = tx.QueryRow(ctx, `SELECT kind,maximum_units FROM assistant_steps WHERE run_id=$1 AND step_number=$2 AND state='running' AND fence=$3 FOR UPDATE`, r.ID, request.Number, r.Fence).Scan(&kind, &maximumUnits)
		if errors.Is(err, pgx.ErrNoRows) {
			return assistant.ErrConflict
		}
		if err != nil {
			return err
		}
		if kind != assistant.Inference && (request.Units != 0 || request.Uncertain) {
			return assistant.ErrInvalid
		}
		if request.Units > maximumUnits || r.UsedUnits+request.Units > r.ReservedUnits {
			return assistant.ErrQuota
		}
		if request.Uncertain {
			r.UsedUnits += request.Units
			if _, err = tx.Exec(ctx, `UPDATE assistant_steps SET units=$3 WHERE run_id=$1 AND step_number=$2`, r.ID, request.Number, request.Units); err != nil {
				return err
			}
			if err = assistantTerminal(ctx, tx, &r, assistant.Uncertain, request.Lease.At); err != nil {
				return err
			}
			result = r.Run
			return nil
		}
		_, err = tx.Exec(ctx, `UPDATE assistant_steps SET state='completed',units=$3,finished_at_us=$4 WHERE run_id=$1 AND step_number=$2`, r.ID, request.Number, request.Units, request.Lease.At.UnixMicro())
		if err != nil {
			return err
		}
		r.UsedUnits += request.Units
		r.Version++
		r.UpdatedAt = request.Lease.At
		r.LeaseUntil = assistantEarlier(request.Lease.At.Add(assistant.MaximumLeaseDuration), r.Deadline)
		_, err = tx.Exec(ctx, `UPDATE assistant_runs SET used_units=$2,row_version=$3,updated_at_us=$4,lease_until_us=$5 WHERE id=$1`, r.ID, r.UsedUnits, r.Version, r.UpdatedAt.UnixMicro(), r.LeaseUntil.UnixMicro())
		if err != nil {
			return err
		}
		result = r.Run
		return assistantEvent(ctx, tx, result, r.UpdatedAt)
	})
	return result, err
}

func (store *Store) FinishAssistant(ctx context.Context, request assistant.Finish) (assistant.Run, error) {
	if !request.Lease.Valid() || (request.State != assistant.Completed && request.State != assistant.Failed && request.State != assistant.Interrupted && request.State != assistant.Expired) {
		return assistant.Run{}, assistant.ErrInvalid
	}
	var result assistant.Run
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		control, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		r, err := assistantRead(ctx, tx, request.Lease.RunID)
		if err != nil {
			return err
		}
		if request.State == assistant.Interrupted || request.State == assistant.Expired {
			now, err := assistantNow(ctx, tx)
			if err != nil {
				return err
			}
			if r.State != assistant.Running || r.Version != request.Lease.Version || r.Fence != request.Lease.Fence || r.LeaseOwner != request.Lease.Worker || request.State == assistant.Expired && now.Before(r.Deadline) {
				return assistant.ErrConflict
			}
			if err = assistantTerminal(ctx, tx, &r, request.State, now); err != nil {
				return err
			}
			result = r.Run
			return nil
		}
		if err = assistantAuthority(ctx, tx, control, r, request.Lease.At); err != nil {
			return err
		}
		request.Lease.At, err = assistantVerifyLease(ctx, tx, r, request.Lease, true)
		if err != nil {
			return err
		}
		var pending bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM assistant_steps WHERE run_id=$1 AND state='running')`, r.ID).Scan(&pending); err != nil {
			return err
		}
		if pending {
			return assistant.ErrConflict
		}
		if err = assistantTerminal(ctx, tx, &r, request.State, request.Lease.At); err != nil {
			return err
		}
		result = r.Run
		return nil
	})
	return result, err
}

func (store *Store) RenewAssistantLease(ctx context.Context, l assistant.Lease, duration time.Duration) (assistant.Run, error) {
	if !l.Valid() || duration < assistant.MinimumLeaseDuration || duration > assistant.MaximumLeaseDuration {
		return assistant.Run{}, assistant.ErrInvalid
	}
	var result assistant.Run
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		control, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		r, err := assistantRead(ctx, tx, l.RunID)
		if err != nil {
			return err
		}
		if err = assistantAuthority(ctx, tx, control, r, l.At); err != nil {
			return err
		}
		now, err := assistantVerifyLease(ctx, tx, r, l, false)
		if err != nil {
			return err
		}
		r.LeaseUntil = assistantEarlier(now.Add(duration), r.Deadline)
		if _, err = tx.Exec(ctx, `UPDATE assistant_runs SET lease_until_us=$2 WHERE id=$1`, r.ID, r.LeaseUntil.UnixMicro()); err != nil {
			return err
		}
		result = r.Run
		return nil
	})
	return result, err
}
