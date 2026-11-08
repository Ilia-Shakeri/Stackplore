CREATE TABLE runbook_catalog (
    singleton SMALLINT PRIMARY KEY CHECK(singleton=1),
    row_version BIGINT NOT NULL CHECK(row_version BETWEEN 1 AND 9007199254740991)
);
INSERT INTO runbook_catalog VALUES(1,1);

CREATE TABLE runbook_corpora (
    id TEXT PRIMARY KEY CHECK(id ~ '^[a-z][a-z0-9_-]{0,63}$'),
    version BIGINT NOT NULL CHECK(version BETWEEN 1 AND 9007199254740991),
    digest BYTEA NOT NULL CHECK(octet_length(digest)=32),
    audience TEXT NOT NULL CHECK(audience IN ('administrators','all_users')),
    approved BOOLEAN NOT NULL,
    restore_generation BIGINT NOT NULL CHECK(restore_generation>=0),
    document_count INTEGER NOT NULL CHECK(document_count BETWEEN 0 AND 32),
    content_bytes INTEGER NOT NULL CHECK(content_bytes BETWEEN 0 AND 524288),
    updated_at_us BIGINT NOT NULL CHECK(updated_at_us>=0),
    CHECK((approved AND document_count>0 AND content_bytes>0) OR
          (NOT approved AND document_count=0 AND content_bytes=0))
);

CREATE TABLE runbook_documents (
    corpus_id TEXT NOT NULL REFERENCES runbook_corpora(id) ON DELETE CASCADE,
    id TEXT NOT NULL CHECK(id ~ '^[a-z][a-z0-9_-]{0,63}$'),
    locale TEXT NOT NULL CHECK(locale IN ('en','fa')),
    title TEXT NOT NULL CHECK(octet_length(title) BETWEEN 1 AND 256),
    body TEXT NOT NULL CHECK(octet_length(body) BETWEEN 1 AND 16384),
    search_terms TSVECTOR GENERATED ALWAYS AS
        (to_tsvector('pg_catalog.simple'::regconfig,title || ' ' || body)) STORED,
    PRIMARY KEY(corpus_id,id,locale)
);
CREATE INDEX runbook_documents_search_idx ON runbook_documents USING GIN(search_terms);
CREATE INDEX runbook_documents_locale_identity_idx ON runbook_documents(locale,corpus_id,id);

CREATE TABLE runbook_confirmations (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id),
    corpus_id TEXT NOT NULL CHECK(corpus_id ~ '^[a-z][a-z0-9_-]{0,63}$'),
    expected_version BIGINT NOT NULL CHECK(expected_version BETWEEN 0 AND 9007199254740991),
    restore_generation BIGINT NOT NULL CHECK(restore_generation>=0),
    candidate_digest BYTEA NOT NULL CHECK(octet_length(candidate_digest)=32),
    revoke BOOLEAN NOT NULL,
    token_digest BYTEA NOT NULL UNIQUE CHECK(octet_length(token_digest)=32),
    created_at_us BIGINT NOT NULL CHECK(created_at_us>=0),
    expires_at_us BIGINT NOT NULL CHECK(expires_at_us-created_at_us=120000000)
);
CREATE INDEX runbook_confirmations_expiry_idx ON runbook_confirmations(expires_at_us,session_id);
