package postgres

import (
	"context"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (store *Store) PruneObservationHistory(ctx context.Context, policy domain.ObservationRetentionPolicy, limit int) (int64, error) {
	if ctx == nil || limit < 4 || limit > MaximumRetentionBatch {
		return 0, domain.ErrRepositoryInvalidInput
	}
	targets := []struct {
		table, column string
		cutoff        time.Time
	}{
		{"metric_samples", "sampled_at_us", policy.MetricsBefore},
		{"health_history", "observed_at_us", policy.HealthBefore},
		{"infrastructure_events", "occurred_at_us", policy.EventsBefore},
		{"notification_deliveries", "terminal_at_us", policy.DeliveriesBefore},
	}
	for _, target := range targets {
		if target.cutoff.IsZero() || target.cutoff.Location() != time.UTC || target.cutoff.UnixMicro() < 0 || target.cutoff.Nanosecond()%int(time.Microsecond) != 0 {
			return 0, domain.ErrRepositoryInvalidInput
		}
	}
	var removed int64
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		for index, target := range targets {
			quota := limit / len(targets)
			if index < limit%len(targets) {
				quota++
			}
			guard := ` AND item.synthetic=0`
			if target.table == "metric_samples" {
				guard += ` AND ` + historicalMetricPredicate
			}
			if target.table == "notification_deliveries" {
				guard = ` AND item.audit_intent_id IS NULL`
			}
			statement := `WITH expired AS (SELECT item.id FROM ` + target.table + ` AS item WHERE item.` + target.column + `<$1` + guard + `
ORDER BY item.` + target.column + `,item.id LIMIT $2 FOR UPDATE OF item SKIP LOCKED)
DELETE FROM ` + target.table + ` AS item USING expired WHERE item.id=expired.id`
			result, err := transaction.transaction.Exec(ctx, statement, target.cutoff.UnixMicro(), quota)
			if err != nil {
				return err
			}
			removed += result.RowsAffected()
		}
		return nil
	})
	if err != nil {
		return 0, mapRepositoryWriteError(ctx, err)
	}
	return removed, nil
}

var _ domain.ObservationRetentionRepository = (*Store)(nil)
