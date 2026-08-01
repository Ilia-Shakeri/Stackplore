CREATE TABLE alert_rules (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    source_id TEXT NOT NULL
        REFERENCES infrastructure_sources (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    environment_id TEXT CHECK (environment_id IS NULL OR length(environment_id) BETWEEN 1 AND 128),
    resource_id TEXT CHECK (resource_id IS NULL OR length(resource_id) BETWEEN 1 AND 128),
    display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 1 AND 256),
    rule_kind TEXT NOT NULL CHECK (
        rule_kind IN (
            'resource_unavailable',
            'resource_unhealthy',
            'restart_threshold',
            'cpu_threshold',
            'memory_threshold',
            'workload_unavailable_replicas',
            'event_severity_match',
            'provider_connection_lost'
        )
    ),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    severity TEXT NOT NULL CHECK (severity IN ('debug', 'info', 'warning', 'error', 'critical')),
    comparison_operator TEXT CHECK (
        comparison_operator IS NULL
        OR comparison_operator IN ('greater_than', 'greater_than_or_equal', 'less_than', 'less_than_or_equal', 'equal')
    ),
    threshold_value REAL CHECK (
        threshold_value IS NULL
        OR (
            threshold_value = threshold_value
            AND threshold_value BETWEEN 0.0 AND 9007199254740991.0
        )
    ),
    threshold_count INTEGER CHECK (threshold_count IS NULL OR threshold_count >= 1),
    window_seconds INTEGER CHECK (window_seconds IS NULL OR window_seconds BETWEEN 1 AND 86400),
    minimum_event_severity TEXT CHECK (
        minimum_event_severity IS NULL
        OR minimum_event_severity IN ('debug', 'info', 'warning', 'error', 'critical')
    ),
    evaluation_interval_seconds INTEGER NOT NULL
        CHECK (evaluation_interval_seconds BETWEEN 1 AND 86400),
    pending_for_seconds INTEGER NOT NULL DEFAULT 0
        CHECK (pending_for_seconds BETWEEN 0 AND 86400),
    last_evaluated_at_us INTEGER CHECK (last_evaluated_at_us IS NULL OR last_evaluated_at_us >= 0),
    next_evaluation_at_us INTEGER CHECK (next_evaluation_at_us IS NULL OR next_evaluation_at_us >= 0),
    created_by_user_id TEXT NOT NULL
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    updated_by_user_id TEXT NOT NULL
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    created_at_us INTEGER NOT NULL CHECK (created_at_us >= 0),
    updated_at_us INTEGER NOT NULL CHECK (updated_at_us >= created_at_us),
    deleted_at_us INTEGER CHECK (deleted_at_us IS NULL OR deleted_at_us >= created_at_us),
    row_version INTEGER NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    FOREIGN KEY (environment_id, source_id)
        REFERENCES environments (id, source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (resource_id, source_id)
        REFERENCES resources (id, source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    UNIQUE (id, source_id),
    CHECK (last_evaluated_at_us IS NULL OR last_evaluated_at_us >= created_at_us),
    CHECK (next_evaluation_at_us IS NULL OR next_evaluation_at_us >= created_at_us),
    CHECK (deleted_at_us IS NULL OR (enabled = 0 AND deleted_at_us <= updated_at_us)),
    CHECK (
        (
            rule_kind IN ('resource_unavailable', 'resource_unhealthy')
            AND resource_id IS NOT NULL
            AND comparison_operator IS NULL
            AND threshold_value IS NULL
            AND threshold_count IS NULL
            AND window_seconds IS NULL
            AND minimum_event_severity IS NULL
        )
        OR (
            rule_kind IN ('cpu_threshold', 'memory_threshold')
            AND resource_id IS NOT NULL
            AND comparison_operator IS NOT NULL
            AND threshold_value IS NOT NULL
            AND threshold_count IS NULL
            AND window_seconds IS NOT NULL
            AND minimum_event_severity IS NULL
        )
        OR (
            rule_kind IN ('restart_threshold', 'workload_unavailable_replicas')
            AND resource_id IS NOT NULL
            AND comparison_operator IS NOT NULL
            AND threshold_value IS NULL
            AND threshold_count IS NOT NULL
            AND window_seconds IS NOT NULL
            AND minimum_event_severity IS NULL
        )
        OR (
            rule_kind = 'event_severity_match'
            AND comparison_operator IS NULL
            AND threshold_value IS NULL
            AND threshold_count IS NULL
            AND window_seconds IS NULL
            AND minimum_event_severity IS NOT NULL
        )
        OR (
            rule_kind = 'provider_connection_lost'
            AND environment_id IS NULL
            AND resource_id IS NULL
            AND comparison_operator IS NULL
            AND threshold_value IS NULL
            AND threshold_count IS NULL
            AND window_seconds IS NULL
            AND minimum_event_severity IS NULL
        )
    )
) STRICT;

CREATE INDEX alert_rules_scheduler_idx
    ON alert_rules (enabled, next_evaluation_at_us, id)
    WHERE deleted_at_us IS NULL;

CREATE INDEX alert_rules_source_kind_idx
    ON alert_rules (source_id, rule_kind, enabled, id)
    WHERE deleted_at_us IS NULL;

CREATE INDEX alert_rules_resource_idx
    ON alert_rules (resource_id, enabled, id)
    WHERE resource_id IS NOT NULL AND deleted_at_us IS NULL;

CREATE INDEX alert_rules_deleted_lifecycle_idx
    ON alert_rules (deleted_at_us, id)
    WHERE deleted_at_us IS NOT NULL;

CREATE TABLE alerts (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    rule_id TEXT NOT NULL CHECK (length(rule_id) BETWEEN 1 AND 128),
    source_id TEXT NOT NULL CHECK (length(source_id) BETWEEN 1 AND 128),
    environment_id TEXT CHECK (environment_id IS NULL OR length(environment_id) BETWEEN 1 AND 128),
    resource_id TEXT CHECK (resource_id IS NULL OR length(resource_id) BETWEEN 1 AND 128),
    dedupe_key TEXT NOT NULL CHECK (length(dedupe_key) BETWEEN 1 AND 128),
    state TEXT NOT NULL CHECK (state IN ('pending', 'firing', 'acknowledged', 'resolved')),
    severity TEXT NOT NULL CHECK (severity IN ('debug', 'info', 'warning', 'error', 'critical')),
    evidence_start_at_us INTEGER NOT NULL CHECK (evidence_start_at_us >= 0),
    evidence_end_at_us INTEGER NOT NULL CHECK (evidence_end_at_us > evidence_start_at_us),
    started_at_us INTEGER NOT NULL CHECK (started_at_us >= 0),
    updated_at_us INTEGER NOT NULL CHECK (updated_at_us >= started_at_us),
    acknowledged_at_us INTEGER
        CHECK (acknowledged_at_us IS NULL OR acknowledged_at_us >= started_at_us),
    acknowledged_by_user_id TEXT
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    resolved_at_us INTEGER CHECK (resolved_at_us IS NULL OR resolved_at_us >= started_at_us),
    summary_code TEXT NOT NULL CHECK (length(summary_code) BETWEEN 3 AND 96),
    synthetic INTEGER NOT NULL DEFAULT 0 CHECK (synthetic IN (0, 1)),
    row_version INTEGER NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    FOREIGN KEY (rule_id, source_id)
        REFERENCES alert_rules (id, source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (environment_id, source_id)
        REFERENCES environments (id, source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (resource_id, source_id)
        REFERENCES resources (id, source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    UNIQUE (id, source_id),
    CHECK (evidence_end_at_us <= updated_at_us),
    CHECK (acknowledged_at_us IS NULL OR acknowledged_at_us <= updated_at_us),
    CHECK (resolved_at_us IS NULL OR resolved_at_us <= updated_at_us),
    CHECK (
        acknowledged_at_us IS NULL
        OR resolved_at_us IS NULL
        OR acknowledged_at_us <= resolved_at_us
    ),
    CHECK (
        (acknowledged_at_us IS NULL AND acknowledged_by_user_id IS NULL)
        OR (acknowledged_at_us IS NOT NULL AND acknowledged_by_user_id IS NOT NULL)
    ),
    CHECK (
        (state IN ('pending', 'firing') AND acknowledged_at_us IS NULL AND resolved_at_us IS NULL)
        OR (state = 'acknowledged' AND acknowledged_at_us IS NOT NULL AND resolved_at_us IS NULL)
        OR (state = 'resolved' AND resolved_at_us IS NOT NULL)
    )
) STRICT;

CREATE UNIQUE INDEX alerts_live_dedupe_uq
    ON alerts (rule_id, dedupe_key)
    WHERE resolved_at_us IS NULL;

CREATE INDEX alerts_source_state_idx
    ON alerts (source_id, state, updated_at_us DESC, id);

CREATE INDEX alerts_resource_state_idx
    ON alerts (resource_id, state, updated_at_us DESC, id)
    WHERE resource_id IS NOT NULL;

CREATE INDEX alerts_rule_time_idx
    ON alerts (rule_id, started_at_us DESC, id);

CREATE INDEX alerts_resolved_history_idx
    ON alerts (resolved_at_us, id)
    WHERE resolved_at_us IS NOT NULL;

CREATE TABLE alert_transitions (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    alert_id TEXT NOT NULL
        REFERENCES alerts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    from_state TEXT CHECK (
        from_state IS NULL OR from_state IN ('pending', 'firing', 'acknowledged', 'resolved')
    ),
    to_state TEXT NOT NULL CHECK (to_state IN ('pending', 'firing', 'acknowledged', 'resolved')),
    occurred_at_us INTEGER NOT NULL CHECK (occurred_at_us >= 0),
    actor_kind TEXT NOT NULL CHECK (actor_kind IN ('system', 'user')),
    actor_user_id TEXT
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    reason_code TEXT NOT NULL CHECK (length(reason_code) BETWEEN 3 AND 96),
    request_id TEXT CHECK (request_id IS NULL OR length(request_id) BETWEEN 1 AND 128),
    evidence_start_at_us INTEGER CHECK (evidence_start_at_us IS NULL OR evidence_start_at_us >= 0),
    evidence_end_at_us INTEGER CHECK (evidence_end_at_us IS NULL OR evidence_end_at_us >= 0),
    CHECK (from_state IS NULL OR from_state <> to_state),
    CHECK (
        (actor_kind = 'system' AND actor_user_id IS NULL)
        OR (actor_kind = 'user' AND actor_user_id IS NOT NULL)
    ),
    CHECK (
        (evidence_start_at_us IS NULL AND evidence_end_at_us IS NULL)
        OR (
            evidence_start_at_us IS NOT NULL
            AND evidence_end_at_us IS NOT NULL
            AND evidence_end_at_us > evidence_start_at_us
            AND evidence_end_at_us <= occurred_at_us
        )
    )
) STRICT;

CREATE INDEX alert_transitions_alert_time_idx
    ON alert_transitions (alert_id, occurred_at_us, id);

CREATE INDEX alert_transitions_time_idx
    ON alert_transitions (occurred_at_us, id);

CREATE TABLE notifications (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    source_id TEXT
        REFERENCES infrastructure_sources (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    alert_id TEXT CHECK (alert_id IS NULL OR length(alert_id) BETWEEN 1 AND 128),
    resource_id TEXT CHECK (resource_id IS NULL OR length(resource_id) BETWEEN 1 AND 128),
    notification_kind TEXT NOT NULL CHECK (length(notification_kind) BETWEEN 3 AND 96),
    severity TEXT NOT NULL CHECK (severity IN ('debug', 'info', 'warning', 'error', 'critical')),
    summary_code TEXT NOT NULL CHECK (length(summary_code) BETWEEN 3 AND 96),
    created_at_us INTEGER NOT NULL CHECK (created_at_us >= 0),
    synthetic INTEGER NOT NULL DEFAULT 0 CHECK (synthetic IN (0, 1)),
    FOREIGN KEY (alert_id, source_id)
        REFERENCES alerts (id, source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (resource_id, source_id)
        REFERENCES resources (id, source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CHECK ((alert_id IS NULL AND resource_id IS NULL) OR source_id IS NOT NULL),
    CHECK (source_id IS NOT NULL OR synthetic = 0)
) STRICT;

CREATE INDEX notifications_source_time_idx
    ON notifications (source_id, created_at_us DESC, id)
    WHERE source_id IS NOT NULL;

CREATE INDEX notifications_time_idx
    ON notifications (created_at_us DESC, id);

CREATE INDEX notifications_alert_idx
    ON notifications (alert_id, created_at_us, id)
    WHERE alert_id IS NOT NULL;

CREATE INDEX notifications_resource_time_idx
    ON notifications (resource_id, created_at_us DESC, id)
    WHERE resource_id IS NOT NULL;

CREATE TABLE notification_deliveries (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    notification_id TEXT NOT NULL
        REFERENCES notifications (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    adapter_kind TEXT NOT NULL CHECK (adapter_kind IN ('in_app', 'webhook', 'matrix')),
    delivery_purpose TEXT NOT NULL CHECK (delivery_purpose IN ('alert', 'test')),
    destination_reference_id TEXT
        CHECK (destination_reference_id IS NULL OR length(destination_reference_id) BETWEEN 1 AND 128),
    render_revision INTEGER NOT NULL DEFAULT 1 CHECK (render_revision >= 1),
    canonical_payload_bytes BLOB NOT NULL CHECK (length(canonical_payload_bytes) BETWEEN 2 AND 65536),
    payload_sha256 BLOB NOT NULL CHECK (length(payload_sha256) = 32),
    state TEXT NOT NULL CHECK (state IN ('pending', 'in_flight', 'succeeded', 'failed', 'uncertain', 'canceled')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count BETWEEN 0 AND 100),
    max_attempts INTEGER NOT NULL CHECK (max_attempts BETWEEN 1 AND 100),
    next_attempt_at_us INTEGER CHECK (next_attempt_at_us IS NULL OR next_attempt_at_us >= 0),
    lease_owner TEXT CHECK (lease_owner IS NULL OR length(lease_owner) BETWEEN 1 AND 128),
    lease_expires_at_us INTEGER CHECK (lease_expires_at_us IS NULL OR lease_expires_at_us >= 0),
    created_at_us INTEGER NOT NULL CHECK (created_at_us >= 0),
    updated_at_us INTEGER NOT NULL CHECK (updated_at_us >= created_at_us),
    terminal_at_us INTEGER CHECK (terminal_at_us IS NULL OR terminal_at_us >= created_at_us),
    error_code TEXT CHECK (error_code IS NULL OR length(error_code) BETWEEN 3 AND 96),
    row_version INTEGER NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    CHECK (attempt_count <= max_attempts),
    CHECK (next_attempt_at_us IS NULL OR next_attempt_at_us >= updated_at_us),
    CHECK (lease_expires_at_us IS NULL OR lease_expires_at_us > updated_at_us),
    CHECK (
        (adapter_kind = 'in_app' AND destination_reference_id IS NULL)
        OR (adapter_kind IN ('webhook', 'matrix') AND destination_reference_id IS NOT NULL)
    ),
    CHECK (
        (state = 'pending' AND next_attempt_at_us IS NOT NULL AND lease_owner IS NULL AND lease_expires_at_us IS NULL)
        OR (state = 'in_flight' AND next_attempt_at_us IS NULL AND lease_owner IS NOT NULL AND lease_expires_at_us IS NOT NULL)
        OR (
            state IN ('succeeded', 'failed', 'uncertain', 'canceled')
            AND next_attempt_at_us IS NULL
            AND lease_owner IS NULL
            AND lease_expires_at_us IS NULL
        )
    ),
    CHECK (
        (state IN ('pending', 'in_flight') AND terminal_at_us IS NULL)
        OR (state IN ('succeeded', 'failed', 'uncertain', 'canceled') AND terminal_at_us IS NOT NULL)
    ),
    CHECK (terminal_at_us IS NULL OR terminal_at_us <= updated_at_us),
    CHECK (state IN ('failed', 'uncertain') OR error_code IS NULL)
) STRICT;

CREATE INDEX notification_deliveries_runnable_idx
    ON notification_deliveries (next_attempt_at_us, id)
    WHERE state = 'pending';

CREATE INDEX notification_deliveries_notification_idx
    ON notification_deliveries (notification_id, created_at_us, id);

CREATE INDEX notification_deliveries_terminal_retention_idx
    ON notification_deliveries (terminal_at_us, id)
    WHERE terminal_at_us IS NOT NULL;

CREATE TABLE notification_delivery_attempts (
    delivery_id TEXT NOT NULL
        REFERENCES notification_deliveries (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    attempt_number INTEGER NOT NULL CHECK (attempt_number BETWEEN 1 AND 100),
    started_at_us INTEGER NOT NULL CHECK (started_at_us >= 0),
    finished_at_us INTEGER CHECK (finished_at_us IS NULL OR finished_at_us >= started_at_us),
    outcome TEXT CHECK (outcome IS NULL OR outcome IN ('success', 'failure', 'uncertain', 'canceled')),
    response_status INTEGER CHECK (response_status IS NULL OR response_status BETWEEN 100 AND 599),
    error_code TEXT CHECK (error_code IS NULL OR length(error_code) BETWEEN 3 AND 96),
    PRIMARY KEY (delivery_id, attempt_number),
    CHECK (
        (finished_at_us IS NULL AND outcome IS NULL AND response_status IS NULL AND error_code IS NULL)
        OR (finished_at_us IS NOT NULL AND outcome IS NOT NULL)
    ),
    CHECK (outcome IN ('failure', 'uncertain') OR error_code IS NULL)
) STRICT;

CREATE INDEX notification_delivery_attempts_finished_idx
    ON notification_delivery_attempts (finished_at_us, delivery_id, attempt_number)
    WHERE finished_at_us IS NOT NULL;

CREATE TABLE audit_entries (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    occurred_at_us INTEGER NOT NULL CHECK (occurred_at_us >= 0),
    actor_kind TEXT NOT NULL CHECK (actor_kind IN ('user', 'operator', 'system')),
    actor_user_id TEXT
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    operator_id TEXT CHECK (operator_id IS NULL OR length(operator_id) BETWEEN 1 AND 128),
    action TEXT NOT NULL CHECK (length(action) BETWEEN 3 AND 96),
    target_kind TEXT NOT NULL CHECK (length(target_kind) BETWEEN 3 AND 96),
    target_id TEXT CHECK (target_id IS NULL OR length(target_id) BETWEEN 1 AND 128),
    outcome TEXT NOT NULL CHECK (outcome IN ('intent', 'success', 'failure', 'denied', 'uncertain')),
    request_id TEXT NOT NULL CHECK (length(request_id) BETWEEN 1 AND 128),
    reason_code TEXT NOT NULL CHECK (length(reason_code) BETWEEN 3 AND 96),
    source_id TEXT
        REFERENCES infrastructure_sources (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CHECK (
        (actor_kind = 'user' AND actor_user_id IS NOT NULL AND operator_id IS NULL)
        OR (actor_kind = 'operator' AND actor_user_id IS NULL)
        OR (actor_kind = 'system' AND actor_user_id IS NULL AND operator_id IS NULL)
    )
) STRICT;

CREATE INDEX audit_entries_time_idx
    ON audit_entries (occurred_at_us DESC, id);

CREATE INDEX audit_entries_actor_time_idx
    ON audit_entries (actor_kind, actor_user_id, operator_id, occurred_at_us DESC, id);

CREATE INDEX audit_entries_action_time_idx
    ON audit_entries (action, occurred_at_us DESC, id);

CREATE INDEX audit_entries_target_time_idx
    ON audit_entries (target_kind, target_id, occurred_at_us DESC, id);

CREATE INDEX audit_entries_request_idx
    ON audit_entries (request_id, occurred_at_us, id);

CREATE INDEX audit_entries_source_time_idx
    ON audit_entries (source_id, occurred_at_us DESC, id)
    WHERE source_id IS NOT NULL;

CREATE INDEX audit_entries_dedicated_retention_idx
    ON audit_entries (occurred_at_us, id);

CREATE TABLE audit_attributes (
    audit_entry_id TEXT NOT NULL
        REFERENCES audit_entries (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    attribute_key TEXT NOT NULL CHECK (length(attribute_key) BETWEEN 3 AND 96),
    value_kind TEXT NOT NULL
        CHECK (value_kind IN ('identifier', 'code', 'count', 'flag', 'safe_label', 'masked_reference')),
    value_text TEXT NOT NULL CHECK (length(value_text) BETWEEN 1 AND 512),
    PRIMARY KEY (audit_entry_id, attribute_key),
    CHECK (value_kind <> 'flag' OR value_text IN ('true', 'false')),
    CHECK (
        value_kind <> 'count'
        OR (
            value_text NOT GLOB '*[^0-9]*'
            AND length(value_text) BETWEEN 1 AND 19
        )
    )
) STRICT;

CREATE INDEX audit_attributes_key_idx
    ON audit_attributes (attribute_key, audit_entry_id);
