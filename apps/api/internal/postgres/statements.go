package postgres

const insertAuditEntryStatement = `INSERT INTO audit_entries (
    id,
    occurred_at_us,
    actor_kind,
    actor_user_id,
    operator_id,
    action,
    target_kind,
    target_id,
    outcome,
    request_id,
    reason_code,
    source_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL)`
