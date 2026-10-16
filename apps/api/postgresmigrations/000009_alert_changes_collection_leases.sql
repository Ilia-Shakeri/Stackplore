ALTER TABLE alert_rules
    ADD COLUMN evaluation_generation BIGINT NOT NULL DEFAULT 0 CHECK (evaluation_generation >= 0),
    ADD COLUMN evaluation_lease_until_us BIGINT CHECK (evaluation_lease_until_us IS NULL OR evaluation_lease_until_us >= 0);

CREATE TABLE alert_changes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES infrastructure_sources(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    rule_id TEXT NOT NULL REFERENCES alert_rules(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    alert_id TEXT REFERENCES alerts(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    change_kind TEXT NOT NULL CHECK (change_kind IN ('alert.changed','rule.created','rule.changed','rule.deleted')),
    occurred_at_us BIGINT NOT NULL CHECK (occurred_at_us >= 0),
    CHECK ((change_kind='alert.changed')=(alert_id IS NOT NULL))
);
CREATE INDEX alert_changes_source_replay_idx ON alert_changes(source_id,id);
CREATE INDEX alert_changes_retention_idx ON alert_changes(occurred_at_us,id);

CREATE TABLE alert_change_retention_positions (
    source_id TEXT PRIMARY KEY REFERENCES infrastructure_sources(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    pruned_through_id BIGINT NOT NULL CHECK (pruned_through_id >= 0)
);

CREATE TABLE source_collection_outcomes (
    source_id TEXT NOT NULL REFERENCES infrastructure_sources(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    configuration_version BIGINT NOT NULL CHECK (configuration_version >= 1),
    generation BIGINT NOT NULL CHECK (generation >= 1),
    collected_at_us BIGINT NOT NULL CHECK (collected_at_us >= 0),
    availability TEXT NOT NULL CHECK (availability IN ('available','stale','unavailable','unsupported','permission_denied')),
    completeness TEXT NOT NULL CHECK (completeness IN ('complete','partial')),
    observed_at_us BIGINT NOT NULL CHECK (observed_at_us >= 0),
    reason_code TEXT CHECK (length(reason_code) BETWEEN 3 AND 96),
    metric_availability TEXT NOT NULL CHECK (metric_availability IN ('available','stale','unavailable','unsupported','permission_denied')),
    metric_completeness TEXT NOT NULL CHECK (metric_completeness IN ('complete','partial')),
    metric_observed_at_us BIGINT NOT NULL CHECK (metric_observed_at_us >= 0),
    metric_reason_code TEXT CHECK (length(metric_reason_code) BETWEEN 3 AND 96),
    event_availability TEXT NOT NULL CHECK (event_availability IN ('available','stale','unavailable','unsupported','permission_denied')),
    event_completeness TEXT NOT NULL CHECK (event_completeness IN ('complete','partial')),
    event_observed_at_us BIGINT NOT NULL CHECK (event_observed_at_us >= 0),
    event_reason_code TEXT CHECK (length(event_reason_code) BETWEEN 3 AND 96),
    PRIMARY KEY (source_id,generation)
);
CREATE INDEX source_collection_outcomes_range_idx ON source_collection_outcomes(source_id,configuration_version,collected_at_us DESC,generation);
CREATE INDEX source_collection_outcomes_current_idx ON source_collection_outcomes(source_id,configuration_version,generation DESC);
CREATE INDEX source_collection_outcomes_retention_idx ON source_collection_outcomes(collected_at_us,source_id,generation);

CREATE TABLE demo_alert_seeds (
    source_id TEXT PRIMARY KEY REFERENCES infrastructure_sources(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    fixture_version TEXT NOT NULL CHECK (length(fixture_version) BETWEEN 1 AND 128),
    seeded_at_us BIGINT NOT NULL CHECK (seeded_at_us >= 0)
);

CREATE TABLE workload_replica_history (
    resource_id TEXT NOT NULL REFERENCES resources(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    capture_generation BIGINT NOT NULL CHECK (capture_generation >= 1),
    sampled_at_us BIGINT NOT NULL CHECK (sampled_at_us >= 0),
    desired_replicas BIGINT NOT NULL CHECK (desired_replicas BETWEEN 0 AND 9007199254740991),
    ready_replicas BIGINT NOT NULL CHECK (ready_replicas BETWEEN 0 AND desired_replicas),
    availability TEXT NOT NULL CHECK (availability IN ('available','stale','unavailable','unsupported','permission_denied')),
    completeness TEXT NOT NULL CHECK (completeness IN ('complete','partial')),
    reason_code TEXT CHECK (length(reason_code) BETWEEN 3 AND 96),
    PRIMARY KEY(resource_id,sampled_at_us)
);
CREATE INDEX workload_replica_history_retention_idx ON workload_replica_history(sampled_at_us,resource_id);
