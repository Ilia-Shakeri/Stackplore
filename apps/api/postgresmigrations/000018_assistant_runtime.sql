CREATE TABLE assistant_control (
    singleton_id BIGINT PRIMARY KEY CHECK (singleton_id = 1),
    row_version BIGINT NOT NULL CHECK (row_version > 0),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    paid_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    profile_paid BOOLEAN NOT NULL DEFAULT FALSE,
    paid_generation BIGINT NOT NULL DEFAULT -1 CHECK (paid_generation >= -1),
    paid_receipt_digest BYTEA CHECK (paid_receipt_digest IS NULL OR octet_length(paid_receipt_digest)=32),
    quota_period BIGINT NOT NULL DEFAULT 1 CHECK (quota_period > 0),
    policy_version BIGINT NOT NULL CHECK (policy_version > 0),
    profile_id TEXT NOT NULL CHECK (length(profile_id) BETWEEN 1 AND 128),
    profile_version BIGINT NOT NULL CHECK (profile_version > 0),
    user_quota BIGINT NOT NULL CHECK (user_quota BETWEEN 1 AND 1000000000),
    installation_quota BIGINT NOT NULL CHECK (installation_quota BETWEEN user_quota AND 1000000000)
);
INSERT INTO assistant_control(singleton_id,row_version,enabled,paid_enabled,policy_version,profile_id,profile_version,user_quota,installation_quota)
VALUES (1,1,FALSE,FALSE,1,'disabled',1,1000000,10000000);

CREATE TABLE assistant_usage_accounts (
    account_key TEXT PRIMARY KEY CHECK (length(account_key) BETWEEN 1 AND 140),
    used_units BIGINT NOT NULL DEFAULT 0 CHECK (used_units >= 0),
    reserved_units BIGINT NOT NULL DEFAULT 0 CHECK (reserved_units >= 0),
    period_version BIGINT NOT NULL DEFAULT 1 CHECK (period_version > 0)
);
INSERT INTO assistant_usage_accounts (account_key) VALUES ('installation');

CREATE TABLE assistant_runs (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 128),
    conversation_id TEXT NOT NULL CHECK (length(conversation_id) BETWEEN 1 AND 128),
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    session_digest BYTEA NOT NULL CHECK (octet_length(session_digest)=32),
    digest_key_id TEXT NOT NULL CHECK (length(digest_key_id) BETWEEN 1 AND 128),
    auth_revision BIGINT NOT NULL CHECK (auth_revision > 0),
    idempotency_digest BYTEA NOT NULL CHECK (octet_length(idempotency_digest)=32),
    fingerprint BYTEA NOT NULL CHECK (octet_length(fingerprint)=32),
    goal TEXT NOT NULL CHECK (goal IN ('explain_resource','explain_alert','investigate_incident')),
    source_id TEXT NOT NULL REFERENCES infrastructure_sources(id) ON DELETE RESTRICT,
    source_version BIGINT NOT NULL CHECK (source_version > 0),
    resource_id TEXT CHECK (resource_id IS NULL OR length(resource_id) BETWEEN 1 AND 128),
    from_us BIGINT NOT NULL CHECK (from_us >= 0),
    until_us BIGINT NOT NULL CHECK (until_us >= from_us AND until_us-from_us <= 86400000000),
    restore_generation BIGINT NOT NULL CHECK (restore_generation >= 0),
    configuration_version BIGINT NOT NULL CHECK (configuration_version > 0),
    policy_version BIGINT NOT NULL CHECK (policy_version > 0),
    profile_id TEXT NOT NULL CHECK (length(profile_id) BETWEEN 1 AND 128),
    profile_version BIGINT NOT NULL CHECK (profile_version > 0),
    volatile BOOLEAN NOT NULL,
    memory_owner TEXT,
    paid BOOLEAN NOT NULL,
    usage_unknown BOOLEAN NOT NULL DEFAULT FALSE,
    usage_conservative BOOLEAN NOT NULL DEFAULT FALSE,
    state TEXT NOT NULL CHECK (state IN ('queued','running','completed','failed','cancelled','expired','interrupted','uncertain')),
    row_version BIGINT NOT NULL DEFAULT 1 CHECK (row_version BETWEEN 1 AND 64),
    claim_attempts BIGINT NOT NULL DEFAULT 0 CHECK (claim_attempts BETWEEN 0 AND 3),
    fence BIGINT NOT NULL DEFAULT 0 CHECK (fence >= 0),
    lease_owner TEXT CHECK (lease_owner IS NULL OR length(lease_owner) BETWEEN 1 AND 128),
    lease_until_us BIGINT,
    tool_calls BIGINT NOT NULL DEFAULT 0 CHECK (tool_calls BETWEEN 0 AND 12),
    inference_calls BIGINT NOT NULL DEFAULT 0 CHECK (inference_calls BETWEEN 0 AND 3),
    reserved_units BIGINT NOT NULL CHECK (reserved_units BETWEEN 1 AND 1000000),
    used_units BIGINT NOT NULL DEFAULT 0 CHECK (used_units BETWEEN 0 AND reserved_units),
    created_at_us BIGINT NOT NULL CHECK (created_at_us >= 0),
    updated_at_us BIGINT NOT NULL CHECK (updated_at_us >= created_at_us),
    deadline_us BIGINT NOT NULL CHECK (deadline_us > created_at_us AND deadline_us-created_at_us <= 120000000),
    UNIQUE (owner_id,idempotency_digest),
    CHECK ((volatile AND memory_owner IS NOT NULL AND length(memory_owner) BETWEEN 1 AND 128) OR (NOT volatile AND memory_owner IS NULL)),
    CHECK ((state='running' AND lease_owner IS NOT NULL AND lease_until_us IS NOT NULL) OR (state<>'running' AND lease_owner IS NULL AND lease_until_us IS NULL))
);
CREATE UNIQUE INDEX assistant_one_running_user ON assistant_runs(owner_id) WHERE state='running';
CREATE INDEX assistant_queue ON assistant_runs(state,created_at_us,id);
CREATE INDEX assistant_owner_queue ON assistant_runs(owner_id,state,created_at_us,id);
CREATE INDEX assistant_expired_lease ON assistant_runs(lease_until_us,id) WHERE state='running';
CREATE INDEX assistant_retention ON assistant_runs(updated_at_us,id) WHERE state NOT IN ('queued','running');

CREATE TABLE assistant_steps (
    run_id TEXT NOT NULL REFERENCES assistant_runs(id) ON DELETE CASCADE,
    step_number BIGINT NOT NULL CHECK (step_number BETWEEN 1 AND 15),
    kind TEXT NOT NULL CHECK (kind IN ('inventory','health','metrics','events','topology','logs','inference')),
    state TEXT NOT NULL CHECK (state IN ('running','completed','uncertain','cancelled')),
    fence BIGINT NOT NULL CHECK (fence > 0),
    units BIGINT NOT NULL DEFAULT 0 CHECK (units BETWEEN 0 AND 1000000),
    maximum_units BIGINT NOT NULL CHECK (maximum_units BETWEEN 0 AND 1000000),
    started_at_us BIGINT NOT NULL CHECK (started_at_us >= 0),
    finished_at_us BIGINT CHECK (finished_at_us IS NULL OR finished_at_us >= started_at_us),
    PRIMARY KEY (run_id,step_number)
);
CREATE UNIQUE INDEX assistant_one_pending_step ON assistant_steps(run_id) WHERE state='running';

CREATE TABLE assistant_status_events (
    run_id TEXT NOT NULL REFERENCES assistant_runs(id) ON DELETE CASCADE,
    sequence BIGINT NOT NULL CHECK (sequence BETWEEN 1 AND 64),
    run_version BIGINT NOT NULL CHECK (run_version BETWEEN 1 AND 64),
    state TEXT NOT NULL CHECK (state IN ('queued','running','completed','failed','cancelled','expired','interrupted','uncertain')),
    occurred_at_us BIGINT NOT NULL CHECK (occurred_at_us >= 0),
    PRIMARY KEY (run_id,sequence)
);
CREATE INDEX assistant_status_retention ON assistant_status_events(occurred_at_us,run_id,sequence);

CREATE TABLE assistant_usage_ledger (
    reservation_id TEXT PRIMARY KEY CHECK (length(reservation_id) BETWEEN 1 AND 128),
    run_reference TEXT CHECK (run_reference IS NULL OR length(run_reference) BETWEEN 1 AND 128),
    owner_id TEXT REFERENCES users(id) ON DELETE RESTRICT,
    idempotency_digest BYTEA CHECK (idempotency_digest IS NULL OR octet_length(idempotency_digest)=32),
    fingerprint BYTEA CHECK (fingerprint IS NULL OR octet_length(fingerprint)=32),
    reserved_units BIGINT NOT NULL CHECK (reserved_units BETWEEN 0 AND 1000000000),
    minimum_units BIGINT NOT NULL DEFAULT 0 CHECK (minimum_units BETWEEN 0 AND reserved_units),
    settled_units BIGINT CHECK (settled_units BETWEEN 0 AND reserved_units),
    state TEXT NOT NULL CHECK (state IN ('held','unknown','settled')),
    restore_generation BIGINT NOT NULL CHECK (restore_generation >= 0),
    created_at_us BIGINT NOT NULL CHECK (created_at_us >= 0),
    updated_at_us BIGINT NOT NULL CHECK (updated_at_us >= created_at_us),
    receipt_digest BYTEA CHECK (receipt_digest IS NULL OR octet_length(receipt_digest)=32),
    settlement_kind TEXT CHECK (settlement_kind IN ('observed','conservative')),
    CHECK ((state='settled' AND settlement_kind IS NOT NULL) OR (state<>'settled' AND settlement_kind IS NULL)),
    CHECK ((state='settled' AND settled_units IS NOT NULL) OR (state<>'settled' AND settled_units IS NULL))
);
CREATE INDEX assistant_usage_retention ON assistant_usage_ledger(updated_at_us,reservation_id) WHERE state='settled';
CREATE UNIQUE INDEX assistant_usage_idempotency ON assistant_usage_ledger(owner_id,idempotency_digest) WHERE idempotency_digest IS NOT NULL;
