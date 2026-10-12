# Upgrade and rollback

The 0.x baseline has no supported automatic upgrade from old development
SQLite data. PostgreSQL is the only runtime store. Keep any earlier development
data untouched; an import would need a separate offline, bounded, tested tool.

The first 0.1.0 tag canonicalizes its previously untracked SQL files to LF before
publication. Raw migration bytes determine the ledger digest. An archive from
an earlier unversioned CRLF development build therefore needs that matching
build; it is not an automatic 0.1.0 upgrade. Do not edit ledger digests. Use a
new verified target or a separately reviewed data migration if retaining that
development database is required.

## Before an upgrade

1. Read the exact target changelog and migration/contract changes.
2. Verify artifact checksums and the tagged source. Production artifacts and
   provenance are delivered by the later release-hardening stage.
3. Record current release, database major version, deployment settings and
   private runtime-key custody without copying secret values into reports.
4. Run PostgreSQL-native backup, validate its checksum, and restore to an empty
   disposable target using [backup/restore](backup-restore.md).
5. Prove the target build against that copy: migrations, readiness, login,
   representative reads and session invalidation after restore.
6. Arrange a controlled single-API rollout. Do not start an upgrade candidate
   against the live database while another runtime owns it.

## Apply and verify

Stop/drain the old API, deploy the reviewed build, then check probes, login,
users, source capabilities, read journeys and audit/notification state. Preserve
PostgreSQL and runtime-key volumes. Keep current backups and the old deployment
definition until the rollback window closes. Container/chart rollout and
live-provider checks require the separate evidence described in
[deployment](deployment.md) and [testing](../testing.md).

## Rollback constraints

Migrations are forward-only and the application refuses unknown/changed ledger
history. Replacing the binary with an older version against a newer schema is
not a supported rollback. `helm rollback` or an old image tag alone does not
undo database changes.

If the target build cannot safely read the new state, keep it offline and
restore the pre-upgrade archive into a new empty database. Validate it with the
matching old build before a controlled deployment-secret/database switch. This
loses writes after the archive cutoff; assess that impact before cutover.
Retain the failed/new database for private investigation and do not delete old
data or archives as part of the application restore command.

Restores revoke sessions and pre-auth state; users must sign in again. The
operator owns archive encryption/retention, secret changes and target cleanup.
Every target upgrade/rollback rehearsal is recorded separately from local tests.
