ALTER TABLE infrastructure_sources
    ADD COLUMN partition_generation BIGINT NOT NULL DEFAULT 0 CHECK (partition_generation BETWEEN 0 AND capture_generation),
    ADD COLUMN operator_policy_digest TEXT CHECK (operator_policy_digest IS NULL OR operator_policy_digest ~ '^[a-f0-9]{64}$'),
    ADD COLUMN operator_policy_pending BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE inventory_partitions (
    source_id TEXT NOT NULL REFERENCES infrastructure_sources(id),
    environment_id TEXT NOT NULL REFERENCES environments(id),
    kind TEXT NOT NULL CHECK (length(kind) BETWEEN 3 AND 96),
    scope_mode TEXT NOT NULL CHECK (scope_mode IN ('all_scopes','exact_scope')),
    scope_kind TEXT NOT NULL,
    scope_name TEXT NOT NULL CHECK (length(scope_name) <= 256),
    coverage_generation BIGINT NOT NULL CHECK (coverage_generation > 0),
    completed_generation BIGINT NOT NULL CHECK (completed_generation BETWEEN 0 AND coverage_generation),
    availability TEXT NOT NULL CHECK (availability IN ('available','stale','unavailable','unsupported','permission_denied')),
    completeness TEXT NOT NULL CHECK (completeness IN ('complete','partial')),
    observed_at_us BIGINT NOT NULL CHECK (observed_at_us >= 0),
    reason_code TEXT CHECK (length(reason_code) BETWEEN 3 AND 96),
    PRIMARY KEY (source_id,environment_id,kind,scope_mode,scope_kind,scope_name),
    CHECK ((scope_mode='all_scopes' AND scope_kind='' AND scope_name='')
        OR (scope_mode='exact_scope' AND ((scope_kind='global' AND scope_name='')
            OR (scope_kind IN ('project','namespace') AND length(scope_name)>0))))
);

CREATE INDEX inventory_partitions_published ON inventory_partitions
    (source_id,coverage_generation,environment_id,kind,scope_mode,scope_kind,scope_name)
    INCLUDE (completed_generation,availability,completeness);

ALTER TABLE resource_snapshots ADD COLUMN metadata JSONB
    CHECK (metadata IS NULL OR (jsonb_typeof(metadata)='object' AND octet_length(metadata::text)<=65536));

ALTER TABLE infrastructure_sources DROP CONSTRAINT infrastructure_sources_operator_profile_check;
ALTER TABLE infrastructure_sources ADD CONSTRAINT infrastructure_sources_operator_profile_check CHECK (
    (provider='demo' AND operator_profile_id='demo_fixture')
    OR (provider='docker' AND operator_profile_id IN ('docker_socket_dev','docker_restricted_proxy'))
    OR (provider='kubernetes' AND operator_profile_id='kubernetes_incluster')
);

ALTER TABLE resource_snapshots DROP CONSTRAINT resource_snapshots_check2;
ALTER TABLE resource_snapshots ADD CONSTRAINT resource_snapshots_ready_replicas_check
    CHECK (ready_replicas BETWEEN 0 AND 9007199254740991);
ALTER TABLE workload_replica_history DROP CONSTRAINT workload_replica_history_check;
ALTER TABLE workload_replica_history ADD CONSTRAINT workload_replica_history_ready_replicas_check
    CHECK (ready_replicas BETWEEN 0 AND 9007199254740991);
