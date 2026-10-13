package postgres

import (
	"context"
	"time"
)

const MaximumRetentionBatch = 1000

// Current metric reads select the newest point, with the lowest ID as the tie break.
// Keep that point when pruning history so an old cutoff cannot erase current data.
const historicalMetricPredicate = `item.id<>(SELECT newest.id FROM metric_samples AS newest
WHERE newest.resource_id=item.resource_id AND newest.metric_name=item.metric_name
ORDER BY newest.sampled_at_us DESC,newest.id COLLATE "C" LIMIT 1)`

// RetentionClass names a fixed observation cleanup target. Audit is excluded.
type RetentionClass string

const (
	RetentionMetricSamples                  RetentionClass = "metric_samples"
	RetentionHealthHistory                  RetentionClass = "health_history"
	RetentionInfrastructureEvents           RetentionClass = "infrastructure_events"
	RetentionTerminalNotificationDeliveries RetentionClass = "terminal_notification_deliveries"
)

var retentionStatements = map[RetentionClass]string{
	RetentionMetricSamples: `WITH expired AS (
    SELECT item.id FROM metric_samples AS item WHERE item.sampled_at_us < $1 AND ` + historicalMetricPredicate + `
    ORDER BY item.sampled_at_us, item.id LIMIT $2 FOR UPDATE OF item SKIP LOCKED
) DELETE FROM metric_samples USING expired WHERE metric_samples.id = expired.id`,
	RetentionHealthHistory: `WITH expired AS (
    SELECT id FROM health_history WHERE observed_at_us < $1
    ORDER BY observed_at_us, id LIMIT $2 FOR UPDATE SKIP LOCKED
) DELETE FROM health_history USING expired WHERE health_history.id = expired.id`,
	RetentionInfrastructureEvents: `WITH expired AS (
    SELECT id FROM infrastructure_events WHERE occurred_at_us < $1
    ORDER BY occurred_at_us, id LIMIT $2 FOR UPDATE SKIP LOCKED
) DELETE FROM infrastructure_events USING expired WHERE infrastructure_events.id = expired.id`,
	RetentionTerminalNotificationDeliveries: `WITH expired AS (
    SELECT id FROM notification_deliveries WHERE terminal_at_us < $1 AND audit_intent_id IS NULL
    ORDER BY terminal_at_us, id LIMIT $2 FOR UPDATE SKIP LOCKED
) DELETE FROM notification_deliveries USING expired WHERE notification_deliveries.id = expired.id`,
}

// Prune removes one bounded batch of eligible observations; it never prunes audit.
func (store *Store) Prune(ctx context.Context, class RetentionClass, cutoff time.Time, batchLimit int) (int64, error) {
	if !store.available() {
		return 0, ErrStoreClosed
	}
	statement, allowed := retentionStatements[class]
	if ctx == nil || ctx.Err() != nil || !allowed || cutoff.IsZero() || cutoff.Location() != time.UTC ||
		cutoff.Before(time.Unix(0, 0)) || cutoff.Nanosecond()%int(time.Microsecond) != 0 ||
		batchLimit < 1 || batchLimit > MaximumRetentionBatch {
		return 0, ErrInvalidOptions
	}
	var removed int64
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		result, err := transaction.transaction.Exec(ctx, statement, cutoff.UnixMicro(), batchLimit)
		if err != nil {
			return err
		}
		removed = result.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return removed, nil
}
