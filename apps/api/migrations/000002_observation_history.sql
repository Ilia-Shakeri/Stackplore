CREATE TABLE environments (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    source_id TEXT NOT NULL,
    provider TEXT NOT NULL CHECK (provider IN ('demo', 'docker', 'kubernetes')),
    display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 1 AND 256),
    availability TEXT NOT NULL
        CHECK (availability IN ('available', 'stale', 'unavailable', 'unsupported', 'permission_denied')),
    completeness TEXT NOT NULL CHECK (completeness IN ('complete', 'partial')),
    observed_at_us INTEGER NOT NULL CHECK (observed_at_us >= 0),
    reason_code TEXT CHECK (reason_code IS NULL OR length(reason_code) BETWEEN 3 AND 96),
    synthetic INTEGER NOT NULL DEFAULT 0 CHECK (synthetic IN (0, 1)),
    first_seen_at_us INTEGER NOT NULL CHECK (first_seen_at_us >= 0),
    last_seen_at_us INTEGER NOT NULL CHECK (last_seen_at_us >= first_seen_at_us),
    retired_at_us INTEGER CHECK (retired_at_us IS NULL OR retired_at_us >= last_seen_at_us),
    row_version INTEGER NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    FOREIGN KEY (source_id, provider)
        REFERENCES infrastructure_sources (id, provider) ON UPDATE RESTRICT ON DELETE RESTRICT,
    UNIQUE (id, source_id),
    UNIQUE (id, source_id, provider),
    CHECK (observed_at_us BETWEEN first_seen_at_us AND last_seen_at_us),
    CHECK (completeness <> 'partial' OR availability IN ('available', 'stale')),
    CHECK (reason_code IS NOT NULL OR (availability = 'available' AND completeness = 'complete')),
    CHECK (
        (provider = 'demo' AND synthetic = 1)
        OR (provider <> 'demo' AND synthetic = 0)
    )
) STRICT;

CREATE INDEX environments_source_active_idx
    ON environments (source_id, retired_at_us, id);

CREATE INDEX environments_source_state_idx
    ON environments (source_id, availability, completeness, id);

CREATE INDEX environments_retired_lifecycle_idx
    ON environments (retired_at_us, id)
    WHERE retired_at_us IS NOT NULL;

CREATE TABLE resources (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    source_id TEXT NOT NULL,
    provider TEXT NOT NULL CHECK (provider IN ('demo', 'docker', 'kubernetes')),
    environment_id TEXT NOT NULL CHECK (length(environment_id) BETWEEN 1 AND 128),
    kind TEXT NOT NULL CHECK (length(kind) BETWEEN 3 AND 96),
    scope_kind TEXT NOT NULL CHECK (scope_kind IN ('global', 'project', 'namespace')),
    scope_name TEXT NOT NULL CHECK (length(scope_name) <= 256),
    native_id TEXT NOT NULL
        CHECK (length(CAST(native_id AS BLOB)) BETWEEN 1 AND 512),
    synthetic INTEGER NOT NULL DEFAULT 0 CHECK (synthetic IN (0, 1)),
    first_seen_at_us INTEGER NOT NULL CHECK (first_seen_at_us >= 0),
    last_seen_at_us INTEGER NOT NULL CHECK (last_seen_at_us >= first_seen_at_us),
    retired_at_us INTEGER CHECK (retired_at_us IS NULL OR retired_at_us >= last_seen_at_us),
    row_version INTEGER NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    FOREIGN KEY (source_id, provider)
        REFERENCES infrastructure_sources (id, provider) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (environment_id, source_id, provider)
        REFERENCES environments (id, source_id, provider) ON UPDATE RESTRICT ON DELETE RESTRICT,
    UNIQUE (id, source_id),
    UNIQUE (id, source_id, provider),
    UNIQUE (id, source_id, provider, environment_id),
    UNIQUE (source_id, environment_id, kind, scope_kind, scope_name, native_id),
    CHECK (
        (scope_kind = 'global' AND scope_name = '')
        OR (scope_kind IN ('project', 'namespace') AND length(scope_name) BETWEEN 1 AND 256)
    ),
    CHECK (
        (provider = 'demo' AND synthetic = 1)
        OR (provider <> 'demo' AND synthetic = 0)
    )
) STRICT;

CREATE INDEX resources_source_kind_active_idx
    ON resources (source_id, kind, retired_at_us, id);

CREATE INDEX resources_environment_active_idx
    ON resources (environment_id, retired_at_us, id);

CREATE INDEX resources_scope_idx
    ON resources (source_id, environment_id, scope_kind, scope_name, id);

CREATE INDEX resources_last_seen_idx
    ON resources (last_seen_at_us, id);

CREATE INDEX resources_retired_lifecycle_idx
    ON resources (retired_at_us, id)
    WHERE retired_at_us IS NOT NULL;

CREATE TABLE resource_snapshots (
    resource_id TEXT PRIMARY KEY
        REFERENCES resources (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 1 AND 256),
    availability TEXT NOT NULL
        CHECK (availability IN ('available', 'stale', 'unavailable', 'unsupported', 'permission_denied')),
    completeness TEXT NOT NULL CHECK (completeness IN ('complete', 'partial')),
    observed_at_us INTEGER NOT NULL CHECK (observed_at_us >= 0),
    reason_code TEXT CHECK (reason_code IS NULL OR length(reason_code) BETWEEN 3 AND 96),
    row_version INTEGER NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    CHECK (completeness <> 'partial' OR availability IN ('available', 'stale')),
    CHECK (reason_code IS NOT NULL OR (availability = 'available' AND completeness = 'complete'))
) STRICT;

CREATE INDEX resource_snapshots_state_idx
    ON resource_snapshots (availability, completeness, observed_at_us, resource_id);

CREATE INDEX resource_snapshots_observed_idx
    ON resource_snapshots (observed_at_us, resource_id);

CREATE TABLE relationships (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    source_id TEXT NOT NULL,
    from_resource_id TEXT NOT NULL CHECK (length(from_resource_id) BETWEEN 1 AND 128),
    to_resource_id TEXT NOT NULL CHECK (length(to_resource_id) BETWEEN 1 AND 128),
    kind TEXT NOT NULL CHECK (
        kind IN ('owns', 'selects', 'routes_to', 'depends_on', 'mounts', 'references', 'member_of')
    ),
    direction TEXT NOT NULL CHECK (direction IN ('source_to_target', 'bidirectional', 'undirected')),
    observed_at_us INTEGER NOT NULL CHECK (observed_at_us >= 0),
    first_seen_at_us INTEGER NOT NULL CHECK (first_seen_at_us >= 0),
    last_seen_at_us INTEGER NOT NULL CHECK (last_seen_at_us >= first_seen_at_us),
    retired_at_us INTEGER CHECK (retired_at_us IS NULL OR retired_at_us >= last_seen_at_us),
    row_version INTEGER NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    FOREIGN KEY (source_id)
        REFERENCES infrastructure_sources (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (from_resource_id, source_id)
        REFERENCES resources (id, source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (to_resource_id, source_id)
        REFERENCES resources (id, source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    UNIQUE (id, source_id),
    UNIQUE (source_id, from_resource_id, to_resource_id, kind, direction),
    CHECK (from_resource_id <> to_resource_id),
    CHECK (observed_at_us BETWEEN first_seen_at_us AND last_seen_at_us),
    CHECK (direction = 'source_to_target' OR from_resource_id < to_resource_id)
) STRICT;

CREATE INDEX relationships_from_active_idx
    ON relationships (from_resource_id, retired_at_us, kind, id);

CREATE INDEX relationships_to_active_idx
    ON relationships (to_resource_id, retired_at_us, kind, id);

CREATE INDEX relationships_source_active_idx
    ON relationships (source_id, retired_at_us, id);

CREATE INDEX relationships_retired_lifecycle_idx
    ON relationships (retired_at_us, id)
    WHERE retired_at_us IS NOT NULL;

CREATE TABLE relationship_evidence (
    relationship_id TEXT NOT NULL CHECK (length(relationship_id) BETWEEN 1 AND 128),
    evidence_id TEXT NOT NULL CHECK (length(evidence_id) BETWEEN 1 AND 128),
    provider TEXT NOT NULL CHECK (provider IN ('demo', 'docker', 'kubernetes')),
    source_id TEXT NOT NULL CHECK (length(source_id) BETWEEN 1 AND 128),
    provenance TEXT NOT NULL CHECK (
        provenance IN (
            'ownership',
            'selector',
            'route',
            'annotation',
            'declared_dependency',
            'storage_reference',
            'network_membership',
            'endpoint'
        )
    ),
    certainty TEXT NOT NULL CHECK (certainty IN ('explicit', 'inferred')),
    confidence REAL NOT NULL CHECK (confidence = confidence AND confidence BETWEEN 0.0 AND 1.0),
    observed_at_us INTEGER NOT NULL CHECK (observed_at_us >= 0),
    synthetic INTEGER NOT NULL DEFAULT 0 CHECK (synthetic IN (0, 1)),
    PRIMARY KEY (relationship_id, evidence_id),
    FOREIGN KEY (relationship_id, source_id)
        REFERENCES relationships (id, source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (source_id, provider)
        REFERENCES infrastructure_sources (id, provider) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CHECK (
        (provider = 'demo' AND synthetic = 1)
        OR (provider <> 'demo' AND synthetic = 0)
    )
) STRICT;

CREATE INDEX relationship_evidence_source_observed_idx
    ON relationship_evidence (source_id, observed_at_us, relationship_id, evidence_id);

CREATE TABLE resource_health_current (
    resource_id TEXT PRIMARY KEY
        REFERENCES resources (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    health_state TEXT NOT NULL CHECK (health_state IN ('healthy', 'degraded', 'unhealthy', 'unknown')),
    availability TEXT NOT NULL
        CHECK (availability IN ('available', 'stale', 'unavailable', 'unsupported', 'permission_denied')),
    completeness TEXT NOT NULL CHECK (completeness IN ('complete', 'partial')),
    observed_at_us INTEGER NOT NULL CHECK (observed_at_us >= 0),
    reason_code TEXT CHECK (reason_code IS NULL OR length(reason_code) BETWEEN 3 AND 96),
    row_version INTEGER NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    CHECK (completeness <> 'partial' OR availability IN ('available', 'stale')),
    CHECK (reason_code IS NOT NULL OR (availability = 'available' AND completeness = 'complete')),
    CHECK (availability IN ('available', 'stale') OR health_state = 'unknown')
) STRICT;

CREATE INDEX resource_health_current_state_idx
    ON resource_health_current (health_state, availability, observed_at_us, resource_id);

CREATE TABLE health_evidence (
    resource_id TEXT NOT NULL
        REFERENCES resource_health_current (resource_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    evidence_id TEXT NOT NULL CHECK (length(evidence_id) BETWEEN 1 AND 128),
    code TEXT NOT NULL CHECK (length(code) BETWEEN 3 AND 96),
    source_code TEXT NOT NULL CHECK (length(source_code) BETWEEN 3 AND 96),
    status TEXT NOT NULL CHECK (status IN ('positive', 'negative', 'unknown')),
    observed_at_us INTEGER NOT NULL CHECK (observed_at_us >= 0),
    synthetic INTEGER NOT NULL DEFAULT 0 CHECK (synthetic IN (0, 1)),
    PRIMARY KEY (resource_id, evidence_id)
) STRICT;

CREATE INDEX health_evidence_observed_idx
    ON health_evidence (observed_at_us, resource_id, evidence_id);

CREATE TABLE health_history (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    resource_id TEXT NOT NULL
        REFERENCES resources (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    health_state TEXT NOT NULL CHECK (health_state IN ('healthy', 'degraded', 'unhealthy', 'unknown')),
    availability TEXT NOT NULL
        CHECK (availability IN ('available', 'stale', 'unavailable', 'unsupported', 'permission_denied')),
    completeness TEXT NOT NULL CHECK (completeness IN ('complete', 'partial')),
    observed_at_us INTEGER NOT NULL CHECK (observed_at_us >= 0),
    reason_code TEXT CHECK (reason_code IS NULL OR length(reason_code) BETWEEN 3 AND 96),
    synthetic INTEGER NOT NULL DEFAULT 0 CHECK (synthetic IN (0, 1)),
    CHECK (completeness <> 'partial' OR availability IN ('available', 'stale')),
    CHECK (reason_code IS NOT NULL OR (availability = 'available' AND completeness = 'complete')),
    CHECK (availability IN ('available', 'stale') OR health_state = 'unknown')
) STRICT;

CREATE INDEX health_history_resource_time_idx
    ON health_history (resource_id, observed_at_us DESC, id);

CREATE INDEX health_history_retention_idx
    ON health_history (observed_at_us, id);

CREATE TABLE metric_samples (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    resource_id TEXT NOT NULL
        REFERENCES resources (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    metric_name TEXT NOT NULL CHECK (
        metric_name IN (
            'cpu.usage.cores',
            'memory.usage.bytes',
            'network.receive.bytes_total',
            'network.send.bytes_total',
            'restart.count',
            'cpu.request.cores',
            'cpu.limit.cores',
            'memory.request.bytes',
            'memory.limit.bytes'
        )
    ),
    unit TEXT NOT NULL CHECK (unit IN ('cores', 'bytes', 'count')),
    value REAL NOT NULL CHECK (
        value = value
        AND value BETWEEN 0.0 AND 9007199254740991.0
        AND (unit = 'cores' OR value = CAST(value AS INTEGER))
    ),
    sampled_at_us INTEGER NOT NULL CHECK (sampled_at_us >= 0),
    interval_seconds INTEGER NOT NULL DEFAULT 0
        CHECK (interval_seconds = 0 OR interval_seconds BETWEEN 1 AND 86400),
    synthetic INTEGER NOT NULL DEFAULT 0 CHECK (synthetic IN (0, 1)),
    CHECK (
        (metric_name IN ('cpu.usage.cores', 'cpu.request.cores', 'cpu.limit.cores') AND unit = 'cores')
        OR (
            metric_name IN (
                'memory.usage.bytes',
                'network.receive.bytes_total',
                'network.send.bytes_total',
                'memory.request.bytes',
                'memory.limit.bytes'
            )
            AND unit = 'bytes'
        )
        OR (metric_name = 'restart.count' AND unit = 'count')
    )
) STRICT;

CREATE INDEX metric_samples_resource_metric_time_idx
    ON metric_samples (resource_id, metric_name, sampled_at_us DESC, id);

CREATE INDEX metric_samples_retention_idx
    ON metric_samples (sampled_at_us, id);

CREATE TABLE infrastructure_events (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    origin_kind TEXT NOT NULL DEFAULT 'provider' CHECK (origin_kind IN ('provider', 'system')),
    provider TEXT CHECK (provider IS NULL OR provider IN ('demo', 'docker', 'kubernetes')),
    source_id TEXT CHECK (source_id IS NULL OR length(source_id) BETWEEN 1 AND 128),
    environment_id TEXT CHECK (environment_id IS NULL OR length(environment_id) BETWEEN 1 AND 128),
    resource_id TEXT CHECK (resource_id IS NULL OR length(resource_id) BETWEEN 1 AND 128),
    kind TEXT NOT NULL CHECK (length(kind) BETWEEN 3 AND 96),
    severity TEXT NOT NULL CHECK (severity IN ('debug', 'info', 'warning', 'error', 'critical')),
    reason_code TEXT NOT NULL CHECK (length(reason_code) BETWEEN 3 AND 96),
    summary TEXT NOT NULL CHECK (length(summary) <= 8192),
    occurred_at_us INTEGER NOT NULL CHECK (occurred_at_us >= 0),
    observed_at_us INTEGER NOT NULL CHECK (observed_at_us >= occurred_at_us),
    synthetic INTEGER NOT NULL DEFAULT 0 CHECK (synthetic IN (0, 1)),
    FOREIGN KEY (source_id, provider)
        REFERENCES infrastructure_sources (id, provider) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (environment_id, source_id, provider)
        REFERENCES environments (id, source_id, provider) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (resource_id, source_id, provider)
        REFERENCES resources (id, source_id, provider) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (resource_id, source_id, provider, environment_id)
        REFERENCES resources (id, source_id, provider, environment_id)
        ON UPDATE RESTRICT ON DELETE RESTRICT,
    CHECK (origin_kind <> 'provider' OR source_id IS NOT NULL),
    CHECK (
        (
            provider IS NULL
            AND source_id IS NULL
            AND environment_id IS NULL
            AND resource_id IS NULL
            AND synthetic = 0
        )
        OR (
            provider IS NOT NULL
            AND source_id IS NOT NULL
            AND (
                (provider = 'demo' AND synthetic = 1)
                OR (provider <> 'demo' AND synthetic = 0)
            )
        )
    )
) STRICT;

CREATE INDEX infrastructure_events_source_time_idx
    ON infrastructure_events (source_id, occurred_at_us DESC, id)
    WHERE source_id IS NOT NULL;

CREATE INDEX infrastructure_events_origin_time_idx
    ON infrastructure_events (origin_kind, occurred_at_us DESC, id);

CREATE INDEX infrastructure_events_provider_time_idx
    ON infrastructure_events (provider, occurred_at_us DESC, id)
    WHERE provider IS NOT NULL;

CREATE INDEX infrastructure_events_environment_time_idx
    ON infrastructure_events (environment_id, occurred_at_us DESC, id)
    WHERE environment_id IS NOT NULL;

CREATE INDEX infrastructure_events_resource_time_idx
    ON infrastructure_events (resource_id, occurred_at_us DESC, id)
    WHERE resource_id IS NOT NULL;

CREATE INDEX infrastructure_events_severity_time_idx
    ON infrastructure_events (severity, occurred_at_us DESC, id);

CREATE INDEX infrastructure_events_retention_idx
    ON infrastructure_events (occurred_at_us, id);
