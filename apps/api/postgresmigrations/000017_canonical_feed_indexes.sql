-- Opaque identifiers use byte order in domain validation and continuation keys.
-- Match that order even when the installation uses a language-aware collation.
DROP INDEX notifications_time_idx;
CREATE INDEX notifications_time_idx ON notifications (created_at_us DESC, id COLLATE "C");

CREATE INDEX alert_rules_updated_order_idx ON alert_rules (updated_at_us DESC, id COLLATE "C")
    WHERE deleted_at_us IS NULL;
