-- The source alert feed orders all states together by update time and opaque ID.
-- The state-leading index serves state filters and cannot supply this feed order.
CREATE INDEX alerts_source_order_idx ON alerts (source_id, updated_at_us DESC, id COLLATE "C");
