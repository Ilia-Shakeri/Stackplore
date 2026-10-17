CREATE TABLE application_settings (
    singleton SMALLINT PRIMARY KEY CHECK (singleton=1),
    row_version BIGINT NOT NULL CHECK (row_version BETWEEN 1 AND 9007199254740991),
    updated_at_us BIGINT NOT NULL CHECK (updated_at_us>=0),
    metric_retention_hours INTEGER NOT NULL CHECK (metric_retention_hours BETWEEN 1 AND 8760),
    health_retention_hours INTEGER NOT NULL CHECK (health_retention_hours BETWEEN 1 AND 8760),
    event_retention_hours INTEGER NOT NULL CHECK (event_retention_hours BETWEEN 1 AND 8760),
    delivery_retention_hours INTEGER NOT NULL CHECK (delivery_retention_hours BETWEEN 1 AND 8760),
    log_line_limit INTEGER NOT NULL CHECK (log_line_limit BETWEEN 10 AND 200),
    default_alert_window_seconds INTEGER NOT NULL CHECK (default_alert_window_seconds BETWEEN 60 AND 86400),
    default_alert_pending_seconds INTEGER NOT NULL CHECK (default_alert_pending_seconds BETWEEN 0 AND 3600)
);
INSERT INTO application_settings VALUES(1,1,(extract(epoch FROM clock_timestamp())*1000000)::BIGINT,168,720,720,720,200,300,0);

CREATE TABLE application_change_confirmations (
    session_id TEXT NOT NULL REFERENCES sessions(id),
    user_id TEXT NOT NULL REFERENCES users(id),
    change_kind TEXT NOT NULL CHECK (change_kind IN ('settings','cleanup')),
    expected_version BIGINT NOT NULL CHECK (expected_version BETWEEN 1 AND 9007199254740991),
    token_digest BYTEA NOT NULL UNIQUE CHECK (octet_length(token_digest)=32),
    requested JSONB NOT NULL CHECK (jsonb_typeof(requested)='object' AND octet_length(requested::text)<=4096),
    impact JSONB NOT NULL CHECK (jsonb_typeof(impact)='object' AND octet_length(impact::text)<=4096),
    created_at_us BIGINT NOT NULL CHECK (created_at_us>=0),
    expires_at_us BIGINT NOT NULL CHECK (expires_at_us-created_at_us=120000000),
    PRIMARY KEY(session_id,change_kind)
);
CREATE INDEX application_change_confirmations_expiry_idx ON application_change_confirmations(expires_at_us,session_id,change_kind);

CREATE TABLE audit_lifecycle_state (
    singleton SMALLINT PRIMARY KEY CHECK (singleton=1),
    retention_days INTEGER NOT NULL CHECK (retention_days BETWEEN 90 AND 3650),
    quota_rows BIGINT NOT NULL CHECK (quota_rows BETWEEN 1000 AND 10000000),
    retained_rows BIGINT NOT NULL CHECK (retained_rows>=0),
    archive_verified_through_us BIGINT CHECK (archive_verified_through_us>=0),
    updated_at_us BIGINT NOT NULL CHECK (updated_at_us>=0)
);
INSERT INTO audit_lifecycle_state
SELECT 1,365,1000000,count(*),NULL,(extract(epoch FROM clock_timestamp())*1000000)::BIGINT FROM audit_entries;

CREATE FUNCTION protect_audit_lifecycle() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    rows_now BIGINT;
    quota BIGINT;
    completion BOOLEAN;
BEGIN
    IF TG_OP='UPDATE' THEN
        RAISE EXCEPTION USING ERRCODE='42501', MESSAGE='audit history is immutable';
    END IF;
    IF TG_OP='DELETE' THEN
        IF current_setting('stackplore.audit_lifecycle',TRUE) IS DISTINCT FROM 'on' THEN
            RAISE EXCEPTION USING ERRCODE='42501', MESSAGE='audit lifecycle required';
        END IF;
        UPDATE audit_lifecycle_state SET retained_rows=retained_rows-1 WHERE singleton=1;
        RETURN OLD;
    END IF;
    SELECT retained_rows,quota_rows INTO rows_now,quota FROM audit_lifecycle_state WHERE singleton=1 FOR UPDATE;
    completion := (NEW.actor_kind IN ('system','operator') AND NEW.action IN ('audit.retention','audit.policy_update'))
        OR (NEW.action IN ('infrastructure.operation','notification.test') AND NEW.outcome IN ('success','failure','uncertain')
            AND EXISTS(SELECT 1 FROM audit_entries AS intent
                WHERE intent.action=NEW.action AND intent.outcome='intent' AND intent.request_id=NEW.request_id
                AND intent.target_kind=NEW.target_kind AND intent.target_id IS NOT DISTINCT FROM NEW.target_id LIMIT 1));
    IF rows_now>=quota AND (NOT completion OR rows_now>=quota+256) THEN
        RAISE EXCEPTION USING ERRCODE='P0001', MESSAGE='protected audit quota reached';
    END IF;
    UPDATE audit_lifecycle_state SET retained_rows=retained_rows+1 WHERE singleton=1;
    RETURN NEW;
END;
$$;
CREATE TRIGGER audit_lifecycle_guard BEFORE INSERT OR UPDATE OR DELETE ON audit_entries
FOR EACH ROW EXECUTE FUNCTION protect_audit_lifecycle();
