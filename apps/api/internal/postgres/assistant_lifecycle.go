package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/jackc/pgx/v5"
)

const assistantReplaySQL = `SELECT sequence,run_version,state,occurred_at_us FROM assistant_status_events
WHERE run_id=$1 AND sequence>$2 AND occurred_at_us>=$3 ORDER BY sequence LIMIT $4`

const assistantPruneEventsSQL = `DELETE FROM assistant_status_events WHERE (run_id,sequence) IN
(SELECT run_id,sequence FROM assistant_status_events WHERE occurred_at_us<$1 ORDER BY occurred_at_us,run_id,sequence LIMIT $2)`

const assistantPruneStepsSQL = `DELETE FROM assistant_steps WHERE (run_id,step_number) IN
(SELECT step.run_id,step.step_number FROM assistant_steps AS step JOIN assistant_runs AS run ON run.id=step.run_id
WHERE run.state NOT IN ('queued','running') AND run.updated_at_us<$1 ORDER BY run.updated_at_us,step.run_id,step.step_number LIMIT $2)`

const assistantPruneRunsSQL = `DELETE FROM assistant_runs WHERE id IN
(SELECT run.id FROM assistant_runs AS run WHERE state NOT IN ('queued','running') AND updated_at_us<$1
AND NOT EXISTS(SELECT 1 FROM assistant_steps AS step WHERE step.run_id=run.id)
AND NOT EXISTS(SELECT 1 FROM assistant_status_events AS event WHERE event.run_id=run.id) ORDER BY updated_at_us,id LIMIT $2)`

const assistantPruneUsageSQL = `DELETE FROM assistant_usage_ledger WHERE reservation_id IN
(SELECT reservation_id FROM assistant_usage_ledger WHERE state='settled' AND updated_at_us<$1 ORDER BY updated_at_us,reservation_id LIMIT $2)`

func (store *Store) CancelAssistant(ctx context.Context, actor assistant.Actor, id string, at time.Time) (assistant.Run, error) {
	if !actor.Valid() || !assistant.Identifier(id) || !assistant.Timestamp(at) {
		return assistant.Run{}, assistant.ErrInvalid
	}
	var result assistant.Run
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := assistantLock(ctx, tx); err != nil {
			return err
		}
		if err := assistantActor(ctx, tx, actor, at); err != nil {
			return err
		}
		r, err := assistantRead(ctx, tx, id)
		if err != nil {
			return err
		}
		if r.OwnerID != actor.UserID {
			return assistant.ErrDenied
		}
		at, err = assistantNow(ctx, tx)
		if err != nil {
			return err
		}
		if !r.State.Terminal() {
			if err = assistantTerminal(ctx, tx, &r, assistant.Cancelled, at); err != nil {
				return err
			}
		}
		result = r.Run
		return nil
	})
	return result, err
}

func (store *Store) GetAssistant(ctx context.Context, actor assistant.Actor, id string, at time.Time) (assistant.Run, error) {
	if !actor.Valid() || !assistant.Identifier(id) || !assistant.Timestamp(at) {
		return assistant.Run{}, assistant.ErrInvalid
	}
	var result assistant.Run
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		control, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		if err = assistantActor(ctx, tx, actor, at); err != nil {
			return err
		}
		r, err := assistantRead(ctx, tx, id)
		if err != nil {
			return err
		}
		if r.OwnerID != actor.UserID {
			return assistant.ErrDenied
		}
		if err = assistantAuthority(ctx, tx, control, r, at); err != nil {
			return err
		}
		result = r.Run
		return nil
	})
	return result, err
}

func (store *Store) AssistantEvents(ctx context.Context, actor assistant.Actor, id string, after int64, limit int, at time.Time) ([]assistant.Event, error) {
	if !actor.Valid() || !assistant.Identifier(id) || after < 0 || limit < 1 || limit > 100 || !assistant.Timestamp(at) {
		return nil, assistant.ErrInvalid
	}
	result := make([]assistant.Event, 0, limit)
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		control, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		if err = assistantActor(ctx, tx, actor, at); err != nil {
			return err
		}
		r, err := assistantRead(ctx, tx, id)
		if err != nil {
			return err
		}
		if r.OwnerID != actor.UserID {
			return assistant.ErrDenied
		}
		if err = assistantAuthority(ctx, tx, control, r, at); err != nil {
			return err
		}
		at, err = assistantNow(ctx, tx)
		if err != nil {
			return err
		}
		var first int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(min(sequence),0) FROM assistant_status_events WHERE run_id=$1 AND occurred_at_us>=$2`, id, at.Add(-assistant.ReplayRetention).UnixMicro()).Scan(&first); err != nil {
			return err
		}
		if first == 0 || after > r.Version || after > 0 && after < first-1 {
			return assistant.ErrConflict
		}
		rows, err := tx.Query(ctx, assistantReplaySQL, id, after, at.Add(-assistant.ReplayRetention).UnixMicro(), limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var event assistant.Event
			var atUS int64
			if err = rows.Scan(&event.Sequence, &event.RunVersion, &event.State, &atUS); err != nil {
				return err
			}
			event.At = time.UnixMicro(atUS).UTC()
			result = append(result, event)
		}
		return rows.Err()
	})
	return result, err
}

func (store *Store) RecoverAssistant(ctx context.Context, at time.Time, limit int) (int, error) {
	if !assistant.Timestamp(at) || limit < 1 || limit > assistant.MaximumRetentionBatch {
		return 0, assistant.ErrInvalid
	}
	changed := 0
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		control, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		at, err = assistantNow(ctx, tx)
		if err != nil {
			return err
		}
		// Admission permits at most sixteen queued plus two running rows. Scan
		// that whole bounded set so a healthy head cannot hide expired work.
		const maximumActiveRuns = 18
		rows, err := tx.Query(ctx, `SELECT id FROM assistant_runs WHERE state IN ('queued','running') AND updated_at_us<=$1 ORDER BY updated_at_us,id LIMIT $2`, at.UnixMicro(), maximumActiveRuns)
		if err != nil {
			return err
		}
		ids := make([]string, 0, maximumActiveRuns)
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
			if changed >= limit {
				break
			}
			r, err := assistantRead(ctx, tx, id)
			if err != nil {
				return err
			}
			authority := assistantAuthority(ctx, tx, control, r, at)
			if authority != nil && !errors.Is(authority, assistant.ErrDenied) {
				return authority
			}
			if authority == nil && r.Deadline.After(at) && (r.State != assistant.Running || r.LeaseUntil.After(at)) {
				continue
			}
			var pending bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM assistant_steps WHERE run_id=$1 AND kind='inference' AND state='running')`, r.ID).Scan(&pending); err != nil {
				return err
			}
			state := assistant.Interrupted
			if pending {
				state = assistant.Uncertain
			} else if !r.Deadline.After(at) {
				state = assistant.Expired
			} else if authority == nil && !r.Volatile && r.Attempts < assistant.MaximumClaims {
				var queued, userQueued int
				if err = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE owner_id=$1) FROM assistant_runs WHERE state='queued'`, r.OwnerID).Scan(&queued, &userQueued); err != nil {
					return err
				}
				if queued < 16 && userQueued < 2 {
					state = assistant.Queued
				}
			}
			if state == assistant.Queued {
				if _, err = tx.Exec(ctx, `UPDATE assistant_steps SET state='cancelled',finished_at_us=$2 WHERE run_id=$1 AND state='running'`, r.ID, at.UnixMicro()); err != nil {
					return err
				}
				r.State = state
				r.Version++
				r.Fence++
				r.UpdatedAt = at
				if _, err = tx.Exec(ctx, `UPDATE assistant_runs SET state='queued',row_version=$2,fence=$3,lease_owner=NULL,lease_until_us=NULL,updated_at_us=$4 WHERE id=$1`, r.ID, r.Version, r.Fence, at.UnixMicro()); err != nil {
					return err
				}
				if err = assistantEvent(ctx, tx, r.Run, at); err != nil {
					return err
				}
			} else if err = assistantTerminal(ctx, tx, &r, state, at); err != nil {
				return err
			}
			changed++
		}
		return nil
	})
	return changed, err
}

func (store *Store) PruneAssistant(ctx context.Context, at time.Time, limit int) (int, error) {
	if !assistant.Timestamp(at) || limit < 1 || limit > assistant.MaximumRetentionBatch {
		return 0, assistant.ErrInvalid
	}
	removed := 0
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := assistantLock(ctx, tx); err != nil {
			return err
		}
		for _, statement := range []struct {
			sql    string
			cutoff time.Time
		}{
			{assistantPruneEventsSQL, at.Add(-assistant.ReplayRetention)},
			{assistantPruneStepsSQL, at.Add(-assistant.MetadataRetention)},
			{assistantPruneRunsSQL, at.Add(-assistant.MetadataRetention)},
			{assistantPruneUsageSQL, at.Add(-assistant.MetadataRetention)},
		} {
			if removed >= limit {
				break
			}
			result, err := tx.Exec(ctx, statement.sql, statement.cutoff.UnixMicro(), limit-removed)
			if err != nil {
				return err
			}
			removed += int(result.RowsAffected())
		}
		return nil
	})
	return removed, err
}
