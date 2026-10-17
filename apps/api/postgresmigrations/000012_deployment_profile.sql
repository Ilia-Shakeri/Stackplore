CREATE TABLE deployment_profile (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    mode TEXT NOT NULL CHECK (mode IN ('normal','demo')),
    guest_demo BOOLEAN NOT NULL DEFAULT FALSE CHECK (NOT guest_demo OR mode='demo'),
    data_root_digest TEXT NOT NULL CHECK (data_root_digest ~ '^[a-f0-9]{64}$'),
    fixture_version TEXT NOT NULL CHECK (length(fixture_version) BETWEEN 1 AND 64),
    reset_pending BOOLEAN NOT NULL DEFAULT FALSE,
    reset_epoch BIGINT NOT NULL DEFAULT 0 CHECK (reset_epoch >= 0)
);

ALTER TABLE alert_rules
    ADD COLUMN system_owned BOOLEAN NOT NULL DEFAULT FALSE,
    ALTER COLUMN created_by_user_id DROP NOT NULL,
    ALTER COLUMN updated_by_user_id DROP NOT NULL;
ALTER TABLE alert_rules ADD CONSTRAINT alert_rules_ownership_check CHECK (
    (system_owned AND source_id='source_demo' AND created_by_user_id IS NULL AND updated_by_user_id IS NULL)
    OR (NOT system_owned AND created_by_user_id IS NOT NULL AND updated_by_user_id IS NOT NULL)
);
