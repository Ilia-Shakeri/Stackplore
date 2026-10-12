CREATE TABLE authentication_throttles (
    scope TEXT NOT NULL CHECK (scope IN ('login_source', 'login_account', 'setup_source', 'recent_user')),
    key_digest BYTEA NOT NULL CHECK (length(key_digest) = 32),
    window_started_at_us BIGINT NOT NULL CHECK (window_started_at_us >= 0),
    failure_count BIGINT NOT NULL CHECK (failure_count BETWEEN 0 AND 1000000),
    blocked_until_at_us BIGINT CHECK (
        blocked_until_at_us IS NULL OR blocked_until_at_us >= window_started_at_us
    ),
    expires_at_us BIGINT NOT NULL CHECK (expires_at_us > window_started_at_us),
    PRIMARY KEY (scope, key_digest),
    CHECK (blocked_until_at_us IS NULL OR blocked_until_at_us <= expires_at_us)
);

CREATE INDEX authentication_throttles_expiry_idx
    ON authentication_throttles (expires_at_us, scope, key_digest);
