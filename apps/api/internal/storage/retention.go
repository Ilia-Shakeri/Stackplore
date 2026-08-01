package storage

import (
	"context"
	"time"
)

const MaximumRetentionBatch = 1_000

// RetentionClass identifies a fixed, non-audit cleanup target.
type RetentionClass string

const (
	RetentionMetricSamples                  RetentionClass = "metric_samples"
	RetentionHealthHistory                  RetentionClass = "health_history"
	RetentionInfrastructureEvents           RetentionClass = "infrastructure_events"
	RetentionTerminalNotificationDeliveries RetentionClass = "terminal_notification_deliveries"
)

var retentionStatements = map[RetentionClass]string{
	RetentionMetricSamples: `DELETE FROM metric_samples
WHERE id IN (
    SELECT id FROM metric_samples
    WHERE sampled_at_us < ?
    ORDER BY sampled_at_us, id
    LIMIT ?
)`,
	RetentionHealthHistory: `DELETE FROM health_history
WHERE id IN (
    SELECT id FROM health_history
    WHERE observed_at_us < ?
    ORDER BY observed_at_us, id
    LIMIT ?
)`,
	RetentionInfrastructureEvents: `DELETE FROM infrastructure_events
WHERE id IN (
    SELECT id FROM infrastructure_events
    WHERE occurred_at_us < ?
    ORDER BY occurred_at_us, id
    LIMIT ?
)`,
	RetentionTerminalNotificationDeliveries: `DELETE FROM notification_deliveries
WHERE id IN (
    SELECT id FROM notification_deliveries
    WHERE terminal_at_us IS NOT NULL AND terminal_at_us < ?
    ORDER BY terminal_at_us, id
    LIMIT ?
)`,
}

// Prune removes at most batchLimit rows from one fixed retention class.
func (store *Store) Prune(ctx context.Context, class RetentionClass, cutoff time.Time, batchLimit int) (int64, error) {
	if store == nil || store.db == nil || store.closed.Load() {
		return 0, ErrStoreClosed
	}
	statement, allowed := retentionStatements[class]
	if ctx == nil || ctx.Err() != nil || !allowed || cutoff.IsZero() || cutoff.Before(time.Unix(0, 0)) ||
		batchLimit < 1 || batchLimit > MaximumRetentionBatch {
		return 0, ErrRetentionRejected
	}

	var removed int64
	err := store.withinTransaction(ctx, func(transaction *transaction) error {
		result, err := transaction.transaction.ExecContext(ctx, statement, cutoff.UTC().UnixMicro(), batchLimit)
		if err != nil {
			return ErrRetentionRejected
		}
		removed, err = result.RowsAffected()
		if err != nil || removed < 0 || removed > int64(batchLimit) {
			return ErrRetentionRejected
		}
		return nil
	})
	if err != nil {
		return 0, ErrRetentionRejected
	}
	return removed, nil
}
