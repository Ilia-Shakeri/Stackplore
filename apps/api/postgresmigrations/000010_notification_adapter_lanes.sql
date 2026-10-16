ALTER TABLE notification_delivery_cursors
    ADD COLUMN adapter_kind TEXT NOT NULL DEFAULT 'webhook'
        CHECK (adapter_kind IN ('webhook', 'matrix'));

ALTER TABLE notification_delivery_cursors
    DROP CONSTRAINT notification_delivery_cursors_pkey,
    ADD PRIMARY KEY (adapter_kind, destination_reference_id);

CREATE INDEX notification_deliveries_scoped_runnable_idx
    ON notification_deliveries (adapter_kind, destination_reference_id, next_attempt_at_us, id)
    WHERE state = 'pending';

CREATE INDEX notification_deliveries_scoped_lease_idx
    ON notification_deliveries (adapter_kind, destination_reference_id, lease_expires_at_us, id)
    WHERE state = 'in_flight';

WITH cancellation_clock AS MATERIALIZED (
    SELECT (EXTRACT(EPOCH FROM clock_timestamp()) * 1000000)::BIGINT AS now_us
)
UPDATE notification_delivery_attempts AS attempt
SET finished_at_us = GREATEST(attempt.started_at_us, cancellation_clock.now_us),
    outcome = 'canceled', response_status = NULL, error_code = NULL
FROM notification_deliveries AS delivery, notifications AS notification,
     cancellation_clock
WHERE attempt.delivery_id = delivery.id
  AND delivery.notification_id = notification.id
  AND notification.synthetic = 1
  AND attempt.finished_at_us IS NULL;

WITH cancellation_clock AS MATERIALIZED (
    SELECT (EXTRACT(EPOCH FROM clock_timestamp()) * 1000000)::BIGINT AS now_us
)
UPDATE notification_deliveries AS delivery
SET state = 'canceled', next_attempt_at_us = NULL, lease_owner = NULL,
    lease_expires_at_us = NULL,
    terminal_at_us = GREATEST(delivery.updated_at_us, cancellation_clock.now_us),
    updated_at_us = GREATEST(delivery.updated_at_us, cancellation_clock.now_us),
    error_code = NULL, row_version = delivery.row_version + 1
FROM notifications AS notification, cancellation_clock
WHERE delivery.notification_id = notification.id
  AND notification.synthetic = 1
  AND delivery.state IN ('pending', 'in_flight');
