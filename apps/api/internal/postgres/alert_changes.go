package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const alertChangeLockKey int64 = 0x535441434B414C43

func lockAlertChanges(ctx context.Context, transaction *transaction) error {
	_, err := transaction.transaction.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, alertChangeLockKey)
	return err
}

func appendAlertChange(ctx context.Context, transaction *transaction, sourceID domain.SourceID, ruleID domain.AlertRuleID, alertID *domain.AlertID, kind domain.AlertChangeKind, at time.Time) error {
	_, err := transaction.transaction.Exec(ctx, `INSERT INTO alert_changes(source_id,rule_id,alert_id,change_kind,occurred_at_us) VALUES($1,$2,$3,$4,$5)`, string(sourceID), string(ruleID), optionalAlertID(alertID), string(kind), at.UnixMicro())
	return err
}

func (store *Store) AlertChangePosition(ctx context.Context, sourceID domain.SourceID) (int64, error) {
	if !store.available() || ctx == nil || sourceID.Validate() != nil {
		return 0, domain.ErrRepositoryInvalidInput
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	var exists bool
	var id int64
	err := store.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM infrastructure_sources WHERE id=$1 AND deleted_at_us IS NULL),COALESCE((SELECT MAX(id) FROM alert_changes WHERE source_id=$1),0)`, string(sourceID)).Scan(&exists, &id)
	if err != nil {
		return 0, mapRepositoryReadError(ctx, err)
	}
	if !exists {
		return 0, domain.ErrRepositoryNotFound
	}
	return id, nil
}

func (store *Store) ListAlertChanges(ctx context.Context, sourceID domain.SourceID, after int64, limit int) (domain.AlertChangePage, error) {
	if !store.available() || ctx == nil || sourceID.Validate() != nil || after < 0 || limit < 1 || limit > 200 {
		return domain.AlertChangePage{}, domain.ErrRepositoryInvalidInput
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	// One snapshot makes pruning and replay-position checks agree with returned rows.
	var result domain.AlertChangePage
	result.Changes = []domain.AlertChange{}
	result.NextID = after
	_, err := readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (struct{}, error) {
		var exists, cursorExists bool
		var oldest, newest, prunedThrough int64
		if err := reader.queries.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM infrastructure_sources WHERE id=$1 AND deleted_at_us IS NULL),COALESCE(MIN(id),0),COALESCE(MAX(id),0),COALESCE((SELECT pruned_through_id FROM alert_change_retention_positions WHERE source_id=$1),0),EXISTS(SELECT 1 FROM alert_changes AS cursor_row WHERE cursor_row.source_id=$1 AND cursor_row.id=$2) FROM alert_changes WHERE source_id=$1`, string(sourceID), after).Scan(&exists, &oldest, &newest, &prunedThrough, &cursorExists); err != nil {
			return struct{}{}, err
		}
		if !exists {
			return struct{}{}, domain.ErrRepositoryNotFound
		}
		if after > newest || after > 0 && (!cursorExists || after < prunedThrough || oldest == 0 || after < oldest) {
			result.Reset = true
			result.NextID = newest
			return struct{}{}, nil
		}
		rows, err := reader.queries.Query(ctx, `SELECT id,source_id,rule_id,alert_id,change_kind,occurred_at_us FROM alert_changes WHERE source_id=$1 AND id>$2 ORDER BY id LIMIT $3`, string(sourceID), after, limit)
		if err != nil {
			return struct{}{}, err
		}
		defer rows.Close()
		for rows.Next() {
			var change domain.AlertChange
			var alertID sql.NullString
			var kind string
			var at int64
			if err := rows.Scan(&change.ID, &change.SourceID, &change.RuleID, &alertID, &kind, &at); err != nil {
				return struct{}{}, err
			}
			change.Kind = domain.AlertChangeKind(kind)
			change.OccurredAt = time.UnixMicro(at).UTC()
			if alertID.Valid {
				value := domain.AlertID(alertID.String)
				change.AlertID = &value
			}
			result.Changes = append(result.Changes, change)
			result.NextID = change.ID
		}
		return struct{}{}, rows.Err()
	})
	if err != nil {
		return domain.AlertChangePage{}, mapRepositoryReadError(ctx, err)
	}
	return result, nil
}

// PruneAlertHistory protects latest source positions and never prunes audit history.
func (store *Store) PruneAlertHistory(ctx context.Context, before time.Time, limit int) (int64, error) {
	if ctx == nil || before.IsZero() || before.Location() != time.UTC || before.UnixMicro() < 0 || before.Nanosecond()%int(time.Microsecond) != 0 || limit < 3 || limit > MaximumRetentionBatch {
		return 0, domain.ErrRepositoryInvalidInput
	}
	var removed int64
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		if err := lockAlertChanges(ctx, transaction); err != nil {
			return err
		}
		quota := limit / 3
		var changesRemoved int64
		if err := transaction.transaction.QueryRow(ctx, `WITH expired AS(SELECT item.id FROM alert_changes AS item WHERE occurred_at_us<$1 AND item.id<>(SELECT MAX(newest.id) FROM alert_changes AS newest WHERE newest.source_id=item.source_id) ORDER BY occurred_at_us,id LIMIT $2 FOR UPDATE SKIP LOCKED),
removed AS(DELETE FROM alert_changes AS item USING expired WHERE item.id=expired.id RETURNING item.source_id,item.id),
positions AS(INSERT INTO alert_change_retention_positions(source_id,pruned_through_id) SELECT source_id,MAX(id) FROM removed GROUP BY source_id ON CONFLICT(source_id) DO UPDATE SET pruned_through_id=GREATEST(alert_change_retention_positions.pruned_through_id,EXCLUDED.pruned_through_id) RETURNING source_id)
SELECT count(*) FROM removed`, before.UnixMicro(), quota).Scan(&changesRemoved); err != nil {
			return err
		}
		removed += changesRemoved
		result, err := transaction.transaction.Exec(ctx, `WITH expired AS(SELECT item.source_id,item.generation FROM source_collection_outcomes AS item WHERE collected_at_us<$1 AND item.generation<>(SELECT MAX(newest.generation) FROM source_collection_outcomes AS newest WHERE newest.source_id=item.source_id) ORDER BY collected_at_us,source_id,generation LIMIT $2 FOR UPDATE SKIP LOCKED) DELETE FROM source_collection_outcomes AS item USING expired WHERE item.source_id=expired.source_id AND item.generation=expired.generation`, before.UnixMicro(), quota)
		if err != nil {
			return err
		}
		removed += result.RowsAffected()
		result, err = transaction.transaction.Exec(ctx, `WITH expired AS(SELECT item.resource_id,item.sampled_at_us FROM workload_replica_history AS item JOIN resources AS resource ON resource.id=item.resource_id WHERE resource.synthetic=0 AND item.sampled_at_us<$1 AND item.sampled_at_us<>(SELECT MAX(newest.sampled_at_us) FROM workload_replica_history AS newest WHERE newest.resource_id=item.resource_id) ORDER BY item.sampled_at_us,item.resource_id LIMIT $2 FOR UPDATE OF item SKIP LOCKED) DELETE FROM workload_replica_history AS item USING expired WHERE item.resource_id=expired.resource_id AND item.sampled_at_us=expired.sampled_at_us`, before.UnixMicro(), limit-2*quota)
		if err != nil {
			return err
		}
		removed += result.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, mapRepositoryWriteError(ctx, err)
	}
	return removed, nil
}

var _ domain.AlertChangeReader = (*Store)(nil)
