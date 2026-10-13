ALTER TABLE infrastructure_sources
    ADD COLUMN operator_profile_id TEXT,
    ADD COLUMN capture_generation BIGINT NOT NULL DEFAULT 0 CHECK (capture_generation >= 0),
    ADD COLUMN completed_generation BIGINT NOT NULL DEFAULT 0 CHECK (completed_generation >= 0),
    ADD COLUMN relationship_completed_generation BIGINT NOT NULL DEFAULT 0 CHECK (relationship_completed_generation >= 0),
    ADD COLUMN capture_started_at_us BIGINT CHECK (capture_started_at_us IS NULL OR capture_started_at_us >= 0),
    ADD COLUMN capture_finished_at_us BIGINT CHECK (capture_finished_at_us IS NULL OR capture_finished_at_us >= 0),
    ADD CONSTRAINT infrastructure_sources_generation_progress_check CHECK (
        completed_generation <= capture_generation AND relationship_completed_generation <= capture_generation
    );

UPDATE infrastructure_sources SET operator_profile_id = CASE provider
    WHEN 'demo' THEN 'demo_fixture'
    WHEN 'docker' THEN 'docker_socket_dev'
    WHEN 'kubernetes' THEN 'kubernetes_incluster'
END;

ALTER TABLE infrastructure_sources ALTER COLUMN operator_profile_id SET NOT NULL;
ALTER TABLE infrastructure_sources ALTER COLUMN operator_profile_id SET DEFAULT 'demo_fixture';
ALTER TABLE infrastructure_sources ADD CONSTRAINT infrastructure_sources_operator_profile_check CHECK (
    (provider = 'demo' AND operator_profile_id = 'demo_fixture')
    OR (provider = 'docker' AND operator_profile_id = 'docker_socket_dev')
    OR (provider = 'kubernetes' AND operator_profile_id = 'kubernetes_incluster')
);

ALTER TABLE environments ADD COLUMN capture_generation BIGINT NOT NULL DEFAULT 0 CHECK (capture_generation >= 0);
ALTER TABLE resources ADD COLUMN capture_generation BIGINT NOT NULL DEFAULT 0 CHECK (capture_generation >= 0);
ALTER TABLE resource_snapshots ADD COLUMN resource_version TEXT NOT NULL DEFAULT '' CHECK (length(resource_version) <= 128);
ALTER TABLE resource_snapshots
    ADD COLUMN desired_replicas BIGINT CHECK (desired_replicas BETWEEN 0 AND 9007199254740991),
    ADD COLUMN ready_replicas BIGINT CHECK (ready_replicas BETWEEN 0 AND desired_replicas),
    ADD COLUMN replicas_availability TEXT CHECK (replicas_availability IN ('available','stale','unavailable','unsupported','permission_denied')),
    ADD COLUMN replicas_completeness TEXT CHECK (replicas_completeness IN ('complete','partial')),
    ADD COLUMN replicas_observed_at_us BIGINT CHECK (replicas_observed_at_us >= 0),
    ADD COLUMN replicas_reason_code TEXT CHECK (length(replicas_reason_code) BETWEEN 3 AND 96),
    ADD CONSTRAINT resource_snapshots_replicas_presence_check CHECK (
        (desired_replicas IS NULL AND ready_replicas IS NULL AND replicas_availability IS NULL
          AND replicas_completeness IS NULL AND replicas_observed_at_us IS NULL AND replicas_reason_code IS NULL)
        OR (desired_replicas IS NOT NULL AND ready_replicas IS NOT NULL AND replicas_availability IS NOT NULL
          AND replicas_completeness IS NOT NULL AND replicas_observed_at_us IS NOT NULL)
    );
ALTER TABLE relationships ADD COLUMN capture_generation BIGINT NOT NULL DEFAULT 0 CHECK (capture_generation >= 0);

ALTER TABLE source_capabilities DROP CONSTRAINT source_capabilities_capability_name_check;
ALTER TABLE source_capabilities ADD CONSTRAINT source_capabilities_capability_name_check CHECK (
    capability_name IN ('inventory.list','inventory.detail','health.read','topology.read','log.read','log.tail',
        'metric.current','metric.history','event.history','event.watch',
        'operation.docker_restart','operation.workload_restart','operation.workload_scale')
);

CREATE INDEX environments_capture_idx ON environments (source_id, capture_generation, id) WHERE retired_at_us IS NULL;
CREATE INDEX resources_capture_idx ON resources (source_id, capture_generation, id) WHERE retired_at_us IS NULL;
CREATE INDEX relationships_capture_idx ON relationships (source_id, capture_generation, id) WHERE retired_at_us IS NULL;
CREATE INDEX resource_snapshots_name_idx ON resource_snapshots (display_name COLLATE "C", resource_id);
CREATE INDEX resource_snapshots_observation_order_idx ON resource_snapshots (observed_at_us, resource_id);
CREATE INDEX resources_source_kind_order_idx ON resources (source_id, kind COLLATE "C", id) WHERE retired_at_us IS NULL;
CREATE INDEX infrastructure_events_source_order_idx ON infrastructure_events (source_id, occurred_at_us DESC, id COLLATE "C");
CREATE INDEX audit_entries_observation_order_idx ON audit_entries (occurred_at_us DESC, id COLLATE "C");
CREATE INDEX metric_samples_capture_dedupe_idx ON metric_samples (resource_id, metric_name, sampled_at_us DESC, id COLLATE "C");
CREATE INDEX metric_samples_observation_retention_idx ON metric_samples (sampled_at_us, id) WHERE synthetic=0;
CREATE INDEX health_history_observation_retention_idx ON health_history (observed_at_us, id) WHERE synthetic=0;
CREATE INDEX infrastructure_events_observation_retention_idx ON infrastructure_events (occurred_at_us, id) WHERE synthetic=0;
CREATE INDEX notification_deliveries_observation_retention_idx ON notification_deliveries (terminal_at_us, id)
    WHERE audit_intent_id IS NULL AND terminal_at_us IS NOT NULL;
