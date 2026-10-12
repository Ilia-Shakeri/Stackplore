ALTER TABLE notification_deliveries
    ADD COLUMN audit_intent_id TEXT
        REFERENCES audit_entries (id) ON UPDATE RESTRICT ON DELETE RESTRICT;

ALTER TABLE notification_deliveries
    ADD CONSTRAINT notification_deliveries_audit_intent_purpose_check CHECK (
        (delivery_purpose = 'alert' AND audit_intent_id IS NULL)
        OR delivery_purpose = 'test'
    );

CREATE UNIQUE INDEX notification_deliveries_audit_intent_idx
    ON notification_deliveries (audit_intent_id)
    WHERE audit_intent_id IS NOT NULL;
