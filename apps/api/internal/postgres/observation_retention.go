package postgres

import (
	"context"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (store *Store) PruneObservationHistory(ctx context.Context, at time.Time, limit int) (int64, error) {
	if ctx == nil || limit < 4 || limit > MaximumRetentionBatch || at.IsZero() || at.Location() != time.UTC || at.UnixMicro() < 0 || at.Nanosecond()%int(time.Microsecond) != 0 {
		return 0, domain.ErrRepositoryInvalidInput
	}
	var removed int64
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		// Hold the policy row through deletion so a policy change and this batch
		// have one order, including an increase that preserves older history.
		settings, err := scanApplicationSettings(transaction.transaction.QueryRow(ctx, `SELECT `+applicationColumns+` FROM application_settings WHERE singleton=1 FOR SHARE`))
		if err != nil {
			return err
		}
		targets := []struct {
			table, column string
			hours         int
		}{
			{"metric_samples", "sampled_at_us", settings.Values.MetricRetentionHours},
			{"health_history", "observed_at_us", settings.Values.HealthRetentionHours},
			{"infrastructure_events", "occurred_at_us", settings.Values.EventRetentionHours},
			{"notification_deliveries", "terminal_at_us", settings.Values.DeliveryRetentionHours},
		}
		for index, target := range targets {
			cutoff := at.Add(-time.Duration(target.hours) * time.Hour)
			if cutoff.UnixMicro() < 0 {
				return domain.ErrRepositoryInvalidInput
			}
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
			result, err := transaction.transaction.Exec(ctx, statement, cutoff.UnixMicro(), quota)
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
