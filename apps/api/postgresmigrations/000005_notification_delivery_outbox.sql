ALTER TABLE notifications
    ADD COLUMN outbox_sequence BIGINT GENERATED ALWAYS AS IDENTITY;

CREATE UNIQUE INDEX notifications_outbox_sequence_idx
    ON notifications (outbox_sequence);

CREATE TABLE notification_delivery_cursors (
    destination_reference_id TEXT PRIMARY KEY
        CHECK (length(destination_reference_id) BETWEEN 1 AND 128),
    last_notification_sequence BIGINT NOT NULL DEFAULT 0
        CHECK (last_notification_sequence >= 0),
    created_at_us BIGINT NOT NULL CHECK (created_at_us >= 0),
    updated_at_us BIGINT NOT NULL CHECK (updated_at_us >= created_at_us)
);
