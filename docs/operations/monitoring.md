# Monitoring and incident response

Stackplore distinguishes process liveness, application readiness and per-source
availability. A healthy process can still be unready because PostgreSQL,
migration history, or single-owner state is unavailable. One provider outage is
reported in that source's status; it need not fail global readiness.

## Routine checks

- Probe `/healthz` for process liveness and `/readyz` before routing requests.
- Scrape `/metrics` only from `STACKPLORE_METRICS_ALLOWED_PREFIXES` and with the
  configured exact public origin. Keep that endpoint off the public ingress.
- Collect structured application logs from the runtime. Log fields are bounded
  and scrubbed, but logs and provider diagnostics may still contain sensitive
  operational context. Restrict readers and retention.
- Watch readiness transitions, source freshness/coverage, notification terminal
  outcomes, database capacity and audit growth. Do not interpret missing or stale
  provider data as a zero value or a healthy source.
- Test native backups and restore to an empty database on a schedule owned by the
  operator. A successful backup command without a restore check is not recovery
  proof.

## Incident response

1. Record UTC time, release, commit, deployment type, safe error code and request
   ID. Keep credentials, full URLs, session cookies, payloads and provider secrets
   out of the incident report.
2. Check `/healthz`, `/readyz`, database connection/TLS, migration ledger and
   runtime ownership. Preserve the exact status and bounded log excerpt.
3. For a source outage, keep that source disabled or isolated until its fixed
   operator profile and permissions are verified. Never enable Secret reads,
   arbitrary endpoints or host paths to restore visibility.
4. For an uncertain guarded action, inspect its durable intent/result record and
   provider state before retrying. A one-use confirmation or recent-auth grant
   must not be replayed.
5. For possible data loss, stop writes through the controlled single-owner
   rollout, preserve the failed database, and use the reviewed
   [backup/restore](backup-restore.md) path against a new empty target.
6. For suspected credential exposure, rotate the credential in its operator-owned
   secret system, reconcile pending deliveries/actions, and restart the owning
   service through the documented procedure. Never paste the old or new value in
   chat or issue reports.
7. Record recovery checks separately: readiness, sign-in, representative reads,
   source freshness, audit state, restored-session revocation and backup checksum.

Do not use `docker compose down --volumes`, remove a PVC, edit migration history,
or start a second API as an incident shortcut. Escalate suspected security issues
through the private path in [SECURITY.md](../../SECURITY.md).

See [troubleshooting](troubleshooting.md), [upgrade/rollback](upgrade-rollback.md),
and the [testing matrix](../testing.md) for proof boundaries.
