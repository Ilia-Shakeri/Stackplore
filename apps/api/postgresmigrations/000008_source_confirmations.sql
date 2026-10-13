CREATE TABLE source_update_confirmations (
    source_id TEXT NOT NULL REFERENCES infrastructure_sources (id) ON DELETE CASCADE,
    session_id TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_digest BYTEA NOT NULL CHECK (octet_length(token_digest) = 32),
    expected_version BIGINT NOT NULL CHECK (expected_version >= 1),
    requested_enabled BIGINT NOT NULL CHECK (requested_enabled IN (0, 1)),
    created_at_us BIGINT NOT NULL CHECK (created_at_us >= 0),
    expires_at_us BIGINT NOT NULL CHECK (expires_at_us > created_at_us),
    PRIMARY KEY (source_id, session_id),
    UNIQUE (token_digest),
    CHECK (expires_at_us - created_at_us = 120000000)
);

CREATE INDEX source_update_confirmations_expiry_idx ON source_update_confirmations (expires_at_us, source_id, session_id);
