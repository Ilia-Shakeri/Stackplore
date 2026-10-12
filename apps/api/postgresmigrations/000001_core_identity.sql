CREATE TABLE installation_metadata (
    singleton_id BIGINT PRIMARY KEY CHECK (singleton_id = 1),
    installation_id TEXT NOT NULL UNIQUE
        CHECK (length(installation_id) BETWEEN 1 AND 128),
    data_mode TEXT NOT NULL
        CHECK (data_mode IN ('normal', 'guest_demo')),
    demo_fixture_version TEXT
        CHECK (demo_fixture_version IS NULL OR length(demo_fixture_version) BETWEEN 1 AND 128),
    created_at_us BIGINT NOT NULL CHECK (created_at_us >= 0),
    setup_completed_at_us BIGINT CHECK (setup_completed_at_us IS NULL OR setup_completed_at_us >= created_at_us),
    network_setup_closed_at_us BIGINT
        CHECK (network_setup_closed_at_us IS NULL OR network_setup_closed_at_us >= created_at_us),
    restored_at_us BIGINT CHECK (restored_at_us IS NULL OR restored_at_us >= created_at_us),
    restore_generation BIGINT NOT NULL DEFAULT 0 CHECK (restore_generation >= 0),
    CHECK (
        (data_mode = 'normal' AND demo_fixture_version IS NULL)
        OR (data_mode = 'guest_demo' AND demo_fixture_version IS NOT NULL)
    ),
    CHECK (
        (setup_completed_at_us IS NULL AND network_setup_closed_at_us IS NULL)
        OR (
            setup_completed_at_us IS NOT NULL
            AND network_setup_closed_at_us IS NOT NULL
            AND network_setup_closed_at_us >= setup_completed_at_us
        )
    ),
    CHECK (
        (restore_generation = 0 AND restored_at_us IS NULL)
        OR (restore_generation > 0 AND restored_at_us IS NOT NULL)
    )
);

CREATE TABLE users (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    login_name TEXT NOT NULL CHECK (length(login_name) BETWEEN 1 AND 256),
    canonical_login_name TEXT NOT NULL CHECK (length(canonical_login_name) BETWEEN 1 AND 256),
    display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 1 AND 256),
    role TEXT NOT NULL CHECK (role IN ('administrator', 'viewer')),
    enabled BIGINT NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    auth_revision BIGINT NOT NULL DEFAULT 1 CHECK (auth_revision >= 1),
    created_at_us BIGINT NOT NULL CHECK (created_at_us >= 0),
    updated_at_us BIGINT NOT NULL CHECK (updated_at_us >= created_at_us),
    disabled_at_us BIGINT CHECK (disabled_at_us IS NULL OR disabled_at_us >= created_at_us),
    deleted_at_us BIGINT CHECK (deleted_at_us IS NULL OR deleted_at_us >= created_at_us),
    row_version BIGINT NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    CHECK (
        (enabled = 1 AND disabled_at_us IS NULL AND deleted_at_us IS NULL)
        OR (enabled = 0 AND disabled_at_us IS NOT NULL)
    ),
    CHECK (disabled_at_us IS NULL OR disabled_at_us <= updated_at_us),
    CHECK (deleted_at_us IS NULL OR deleted_at_us BETWEEN disabled_at_us AND updated_at_us)
);

CREATE UNIQUE INDEX users_canonical_login_active_uq
    ON users (canonical_login_name)
    WHERE deleted_at_us IS NULL;

CREATE INDEX users_role_enabled_idx
    ON users (role, enabled, deleted_at_us, id);

CREATE INDEX users_deleted_lifecycle_idx
    ON users (deleted_at_us, id)
    WHERE deleted_at_us IS NOT NULL;

CREATE TABLE user_password_credentials (
    user_id TEXT PRIMARY KEY
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    algorithm TEXT NOT NULL DEFAULT 'argon2id' CHECK (algorithm = 'argon2id'),
    algorithm_version BIGINT NOT NULL CHECK (algorithm_version >= 1),
    time_cost BIGINT NOT NULL CHECK (time_cost >= 1),
    memory_kib BIGINT NOT NULL CHECK (memory_kib >= 1),
    parallelism BIGINT NOT NULL CHECK (parallelism >= 1),
    salt BYTEA NOT NULL CHECK (length(salt) BETWEEN 16 AND 64),
    password_hash BYTEA NOT NULL CHECK (length(password_hash) BETWEEN 16 AND 128),
    changed_at_us BIGINT NOT NULL CHECK (changed_at_us >= 0)
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    user_id TEXT NOT NULL
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    token_digest BYTEA NOT NULL UNIQUE CHECK (length(token_digest) = 32),
    token_digest_algorithm TEXT NOT NULL DEFAULT 'hmac-sha256'
        CHECK (token_digest_algorithm = 'hmac-sha256'),
    token_digest_key_id TEXT NOT NULL CHECK (length(token_digest_key_id) BETWEEN 1 AND 128),
    csrf_digest BYTEA NOT NULL UNIQUE CHECK (length(csrf_digest) = 32),
    csrf_digest_algorithm TEXT NOT NULL DEFAULT 'hmac-sha256'
        CHECK (csrf_digest_algorithm = 'hmac-sha256'),
    csrf_digest_key_id TEXT NOT NULL CHECK (length(csrf_digest_key_id) BETWEEN 1 AND 128),
    user_auth_revision BIGINT NOT NULL CHECK (user_auth_revision >= 1),
    created_at_us BIGINT NOT NULL CHECK (created_at_us >= 0),
    last_seen_at_us BIGINT NOT NULL CHECK (last_seen_at_us >= created_at_us),
    idle_expires_at_us BIGINT NOT NULL CHECK (idle_expires_at_us > created_at_us),
    absolute_expires_at_us BIGINT NOT NULL CHECK (absolute_expires_at_us > created_at_us),
    revoked_at_us BIGINT CHECK (revoked_at_us IS NULL OR revoked_at_us >= created_at_us),
    revoke_reason_code TEXT
        CHECK (revoke_reason_code IS NULL OR length(revoke_reason_code) BETWEEN 3 AND 96),
    rotated_from_session_id TEXT,
    FOREIGN KEY (rotated_from_session_id, user_id)
        REFERENCES sessions (id, user_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    UNIQUE (id, user_id),
    CHECK (last_seen_at_us <= absolute_expires_at_us),
    CHECK (last_seen_at_us <= idle_expires_at_us),
    CHECK (idle_expires_at_us <= absolute_expires_at_us),
    CHECK (
        (revoked_at_us IS NULL AND revoke_reason_code IS NULL)
        OR (revoked_at_us IS NOT NULL AND revoke_reason_code IS NOT NULL)
    ),
    CHECK (rotated_from_session_id IS NULL OR rotated_from_session_id <> id)
);

CREATE INDEX sessions_user_active_idx
    ON sessions (user_id, absolute_expires_at_us, id)
    WHERE revoked_at_us IS NULL;

CREATE INDEX sessions_expiry_retention_idx
    ON sessions (absolute_expires_at_us, id);

CREATE INDEX sessions_revoked_retention_idx
    ON sessions (revoked_at_us, id)
    WHERE revoked_at_us IS NOT NULL;

CREATE TABLE preauth_challenges (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    purpose TEXT NOT NULL CHECK (purpose IN ('login', 'setup')),
    cookie_digest BYTEA NOT NULL UNIQUE CHECK (length(cookie_digest) = 32),
    header_digest BYTEA NOT NULL UNIQUE CHECK (length(header_digest) = 32),
    digest_algorithm TEXT NOT NULL DEFAULT 'hmac-sha256'
        CHECK (digest_algorithm = 'hmac-sha256'),
    digest_key_id TEXT NOT NULL CHECK (length(digest_key_id) BETWEEN 1 AND 128),
    created_at_us BIGINT NOT NULL CHECK (created_at_us >= 0),
    expires_at_us BIGINT NOT NULL CHECK (expires_at_us > created_at_us),
    consumed_at_us BIGINT
        CHECK (consumed_at_us IS NULL OR consumed_at_us BETWEEN created_at_us AND expires_at_us)
);

CREATE INDEX preauth_challenges_expiry_retention_idx
    ON preauth_challenges (expires_at_us, id);

CREATE TABLE recent_auth_grants (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    user_id TEXT NOT NULL
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    session_id TEXT NOT NULL,
    purpose TEXT NOT NULL CHECK (length(purpose) BETWEEN 3 AND 96),
    grant_digest BYTEA NOT NULL UNIQUE CHECK (length(grant_digest) = 32),
    digest_algorithm TEXT NOT NULL DEFAULT 'hmac-sha256'
        CHECK (digest_algorithm = 'hmac-sha256'),
    digest_key_id TEXT NOT NULL CHECK (length(digest_key_id) BETWEEN 1 AND 128),
    created_at_us BIGINT NOT NULL CHECK (created_at_us >= 0),
    expires_at_us BIGINT NOT NULL CHECK (expires_at_us > created_at_us),
    consumed_at_us BIGINT
        CHECK (consumed_at_us IS NULL OR consumed_at_us BETWEEN created_at_us AND expires_at_us),
    revoked_at_us BIGINT CHECK (revoked_at_us IS NULL OR revoked_at_us >= created_at_us),
    revoke_reason_code TEXT
        CHECK (revoke_reason_code IS NULL OR length(revoke_reason_code) BETWEEN 3 AND 96),
    FOREIGN KEY (session_id, user_id)
        REFERENCES sessions (id, user_id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CHECK (
        (revoked_at_us IS NULL AND revoke_reason_code IS NULL)
        OR (revoked_at_us IS NOT NULL AND revoke_reason_code IS NOT NULL)
    )
);

CREATE INDEX recent_auth_grants_session_idx
    ON recent_auth_grants (session_id, expires_at_us, id);

CREATE INDEX recent_auth_grants_expiry_retention_idx
    ON recent_auth_grants (expires_at_us, id);

CREATE TABLE infrastructure_sources (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    provider TEXT NOT NULL CHECK (provider IN ('demo', 'docker', 'kubernetes')),
    display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 1 AND 256),
    synthetic BIGINT NOT NULL DEFAULT 0 CHECK (synthetic IN (0, 1)),
    enabled BIGINT NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    connection_kind TEXT NOT NULL CHECK (length(connection_kind) BETWEEN 1 AND 96),
    config_revision BIGINT NOT NULL DEFAULT 1 CHECK (config_revision >= 1),
    created_by_user_id TEXT
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    created_at_us BIGINT NOT NULL CHECK (created_at_us >= 0),
    updated_at_us BIGINT NOT NULL CHECK (updated_at_us >= created_at_us),
    deleted_at_us BIGINT CHECK (deleted_at_us IS NULL OR deleted_at_us >= created_at_us),
    row_version BIGINT NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    CHECK (
        (provider = 'demo' AND synthetic = 1)
        OR (provider <> 'demo' AND synthetic = 0)
    ),
    CHECK (deleted_at_us IS NULL OR (enabled = 0 AND deleted_at_us <= updated_at_us)),
    UNIQUE (id, provider)
);

CREATE INDEX infrastructure_sources_provider_active_idx
    ON infrastructure_sources (provider, enabled, deleted_at_us, id);

CREATE INDEX infrastructure_sources_deleted_lifecycle_idx
    ON infrastructure_sources (deleted_at_us, id)
    WHERE deleted_at_us IS NOT NULL;

CREATE TABLE source_connection_references (
    source_id TEXT NOT NULL
        REFERENCES infrastructure_sources (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    purpose TEXT NOT NULL CHECK (length(purpose) BETWEEN 1 AND 96),
    reference_kind TEXT NOT NULL CHECK (length(reference_kind) BETWEEN 1 AND 96),
    reference_id TEXT NOT NULL CHECK (length(reference_id) BETWEEN 1 AND 128),
    display_label TEXT CHECK (display_label IS NULL OR length(display_label) BETWEEN 1 AND 256),
    created_at_us BIGINT NOT NULL CHECK (created_at_us >= 0),
    updated_at_us BIGINT NOT NULL CHECK (updated_at_us >= created_at_us),
    PRIMARY KEY (source_id, purpose)
);

CREATE INDEX source_connection_references_reference_idx
    ON source_connection_references (reference_kind, reference_id, source_id);

CREATE TABLE source_status (
    source_id TEXT PRIMARY KEY
        REFERENCES infrastructure_sources (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    availability TEXT NOT NULL
        CHECK (availability IN ('available', 'stale', 'unavailable', 'unsupported', 'permission_denied')),
    completeness TEXT NOT NULL CHECK (completeness IN ('complete', 'partial')),
    observed_at_us BIGINT NOT NULL CHECK (observed_at_us >= 0),
    reason_code TEXT CHECK (reason_code IS NULL OR length(reason_code) BETWEEN 3 AND 96),
    row_version BIGINT NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    CHECK (completeness <> 'partial' OR availability IN ('available', 'stale')),
    CHECK (reason_code IS NOT NULL OR (availability = 'available' AND completeness = 'complete'))
);

CREATE INDEX source_status_state_idx
    ON source_status (availability, completeness, observed_at_us, source_id);

CREATE TABLE source_capabilities (
    source_id TEXT NOT NULL
        REFERENCES infrastructure_sources (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    capability_name TEXT NOT NULL CHECK (
        capability_name IN (
            'inventory.list',
            'inventory.detail',
            'health.read',
            'topology.read',
            'log.read',
            'log.tail',
            'metric.current',
            'metric.history',
            'event.history',
            'event.watch'
        )
    ),
    availability TEXT NOT NULL
        CHECK (availability IN ('available', 'stale', 'unavailable', 'unsupported', 'permission_denied')),
    completeness TEXT NOT NULL CHECK (completeness IN ('complete', 'partial')),
    observed_at_us BIGINT NOT NULL CHECK (observed_at_us >= 0),
    reason_code TEXT CHECK (reason_code IS NULL OR length(reason_code) BETWEEN 3 AND 96),
    row_version BIGINT NOT NULL DEFAULT 1 CHECK (row_version >= 1),
    PRIMARY KEY (source_id, capability_name),
    CHECK (completeness <> 'partial' OR availability IN ('available', 'stale')),
    CHECK (reason_code IS NOT NULL OR (availability = 'available' AND completeness = 'complete'))
);

CREATE INDEX source_capabilities_state_idx
    ON source_capabilities (capability_name, availability, completeness, source_id);
