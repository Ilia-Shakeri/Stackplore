# Incident response

[راهنمای فارسی](incident-response.fa.md)

Use this procedure for a suspected account, provider, outbound destination or
storage incident. Preserve the database, runtime key and existing evidence.
Never paste tokens, cookies, kubeconfig content, log payloads, backup contents or
notification addresses into an issue or incident report.

1. Record the UTC interval, deployed version, artifact digest, affected logical
   source identifiers and safe request IDs. Separate API readiness, provider
   availability and notification delivery failures.
2. Stop admission at the operator's ingress if public access is unsafe. Disable
   infrastructure mutations at deployment level and suspend affected outbound
   adapters. Drain the application before changing provider authority. These
   actions do not revoke or repair external credentials by themselves.
3. Preserve a PostgreSQL-native backup and the matching private runtime key under
   the existing encrypted custody procedure. Retain the failing deployment and
   sanitized probe/audit records. Do not rewrite migration or audit history.
4. Revoke affected sessions/accounts through administrator controls or the
   [offline recovery procedure](account-recovery.md). Preserve a recoverable
   administrator; the final administrator guard remains active. Rotate provider
   and notification credentials at their issuing systems, then replace the
   operator-owned mounted references and restart the application.
5. Test the reviewed repair against an owned disposable installation. Require
   readiness, fresh login, relevant read journeys, source permission boundaries,
   denied writes and an independently verified
   [restore](backup-restore.md). A running process alone is insufficient.
6. Restore service in a controlled single-owner rollout. Check
   [monitoring](monitoring.md) and [upgrade constraints](upgrade-rollback.md).
   Retain failed receipts and record residual target-specific risks. Report
   product vulnerabilities through [the security policy](../../SECURITY.md).

Audit expiry stays bounded by the operator-verified archive cutoff and minimum
retention. An incident is not permission to delete audit history. Readiness
failure does not justify bypassing TLS, bootstrap, authorization or migration
history checks.
