CREATE TABLE operation_confirmations (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    token_digest BYTEA NOT NULL UNIQUE CHECK (octet_length(token_digest)=32),
    fingerprint BYTEA NOT NULL CHECK (octet_length(fingerprint)=32),
    user_id TEXT NOT NULL REFERENCES users(id),
    session_id TEXT NOT NULL REFERENCES sessions(id),
    source_id TEXT NOT NULL REFERENCES infrastructure_sources(id),
    resource_id TEXT NOT NULL REFERENCES resources(id),
    request JSONB NOT NULL CHECK (jsonb_typeof(request)='object' AND octet_length(request::text)<=8192),
    created_at_us BIGINT NOT NULL CHECK (created_at_us>=0),
    expires_at_us BIGINT NOT NULL CHECK (expires_at_us-created_at_us=120000000),
    UNIQUE(session_id,source_id,resource_id)
);
CREATE INDEX operation_confirmations_expiry_idx ON operation_confirmations(expires_at_us,id);

CREATE TABLE operation_executions (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    user_id TEXT NOT NULL REFERENCES users(id),
    session_id TEXT NOT NULL REFERENCES sessions(id),
    source_id TEXT NOT NULL REFERENCES infrastructure_sources(id),
    resource_id TEXT NOT NULL REFERENCES resources(id),
    confirmation_id TEXT NOT NULL UNIQUE,
    idempotency_digest BYTEA NOT NULL CHECK (octet_length(idempotency_digest)=32),
    fingerprint BYTEA NOT NULL CHECK (octet_length(fingerprint)=32),
    request JSONB NOT NULL CHECK (jsonb_typeof(request)='object' AND octet_length(request::text)<=8192),
    request_id TEXT NOT NULL CHECK (length(request_id) BETWEEN 1 AND 128),
    state TEXT NOT NULL CHECK (state IN ('in_progress','succeeded','failed','uncertain')),
    result JSONB CHECK (result IS NULL OR (jsonb_typeof(result)='object' AND octet_length(result::text)<=4096)),
    reason_code TEXT CHECK (reason_code IS NULL OR reason_code ~ '^[a-z][a-z0-9_]*[.][a-z0-9_.-]+$'),
    created_at_us BIGINT NOT NULL CHECK (created_at_us>=0),
    updated_at_us BIGINT NOT NULL CHECK (updated_at_us>=created_at_us),
    lease_until_us BIGINT NOT NULL CHECK (lease_until_us>created_at_us AND lease_until_us-created_at_us<=120000000),
    intent_audit_id TEXT NOT NULL UNIQUE REFERENCES audit_entries(id),
    result_audit_id TEXT UNIQUE REFERENCES audit_entries(id),
    UNIQUE(user_id,idempotency_digest),
    CHECK ((state='in_progress' AND result IS NULL AND reason_code IS NULL AND result_audit_id IS NULL AND updated_at_us=created_at_us)
        OR (state='succeeded' AND result IS NOT NULL AND reason_code='operation.succeeded' AND result_audit_id IS NOT NULL)
        OR (state IN ('failed','uncertain') AND result IS NULL AND reason_code IS NOT NULL AND result_audit_id IS NOT NULL))
);
CREATE UNIQUE INDEX operation_executions_target_active_idx ON operation_executions(source_id,resource_id) WHERE state='in_progress';
CREATE INDEX operation_executions_orphan_idx ON operation_executions(lease_until_us,id) WHERE state='in_progress';
CREATE INDEX operation_executions_order_idx ON operation_executions(created_at_us DESC,id COLLATE "C" DESC);
CREATE INDEX operation_executions_source_order_idx ON operation_executions(source_id,created_at_us DESC,id COLLATE "C" DESC);
CREATE INDEX operation_executions_active_user_idx ON operation_executions(user_id) WHERE state='in_progress';
