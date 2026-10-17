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

## 0.1.0 to 0.2.0

The first six released migration bytes are unchanged. The new runtime adds
migration 7 for collection generations, typed snapshot fields and query indexes,
then migration 8 for digest-only source confirmations. Real PostgreSQL tests
verify the forward upgrade from the exact six-version ledger. Existing users,
sources and observations are retained. Startup inserts only missing operator
profiles; it does not overwrite an existing administrator's source enabled state.

Restore an older archive with the storage command from its matching release.
The 0.2 command expects the exact eight-version ledger and rejects a six-version
0.1 archive. After restoring and verifying it with 0.1, stop that runtime and
perform the normal single-owner 0.2 forward upgrade against the disposable copy.
Do not change ledger digests to bypass this gate. Repeat representative reads
before applying the reviewed upgrade procedure to an actual installation.

## 0.2.0 to 0.3.0

The first eight released migration bytes remain unchanged. Migration 9 adds
configuration-version-bound collection history, replica history, fenced alert
claims, durable change replay and versioned demo seed markers. Migration 10
separates outbound adapter/destination cursors and lanes. Existing webhook cursor
positions are preserved. Legacy synthetic pending/in-flight deliveries become
canceled, with unfinished attempts closed; they must not reach external targets.
Sources, accounts, existing alerts and delivery history remain stored. Earlier static demo alerts
become durable after an enabled administrator and resource capture are present.
Seeding does not resurrect acknowledged or deleted state after its marker exists.

The 0.3 storage tool expects its exact ten-version ledger. Restore an eight-version
archive with the matching 0.2 tool, verify it, stop that runtime, then run the 0.3
forward upgrade on the disposable copy. Never rewrite migration digests or start
both runtimes against one database. Old runtime rollback requires the separately
verified pre-upgrade backup; there are no reverse migrations.

## Later PostgreSQL checkpoints and the 0.9.0 candidate

Earlier released migration files remain byte-identical. The later ledger adds:

| Migration | Change |
| --- | --- |
| 11 | Current inventory partition coverage and authority fences |
| 12 | Separate normal/guest deployment identity |
| 13 | Durable guarded-operation preview, intent and result state |
| 14 | Versioned application settings and protected audit policy |
| 15 | Permits the operator-owned `kubernetes_kubeconfig` mounted profile |
| 16 | Adds the source alert feed's matching update-time/opaque-ID keyset index |

Migrations 15–16 are forward-only additions. The mounted profile stores a logical
profile reference, never kubeconfig text, credentials or browser-selected paths.
The index changes the query plan, not alert identity or cursor authorization.
Measure index work and startup/lock timing on the restored representative copy.
Use the archive's matching storage build to restore its exact ledger, then stop
it before upgrading that copy through the reviewed forward path. A newer storage
tool is not a generic importer for older migration histories.

The single-service deployment embeds the UI in Go. An upgrade from the older
separate web proxy must preserve PostgreSQL/runtime-key volumes and route the
same-origin ingress to the API service. The chart's non-root initialization
prepares a private PVC child and atomically moves a validated legacy root key
without overwriting it. Ambiguous duplicate keys or unsafe files fail closed;
do not delete a key to get past this check. See [deployment](deployment.md).

## Before an upgrade

1. Read the exact target changelog and migration/contract changes.
2. Verify the tagged source, signed checksum manifest, artifact identity and
   provenance using [release verification](release.md) ([فارسی](release.fa.md)).
   A workflow definition or an unobserved signing run is not a verified artifact.
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
The current candidate's final acceptance is recorded in the [active plan](../implementation-plan.md).
Project acceptance does not authorize or prove an unrequested customer rollout.
For recovery from a suspected compromise, preserve evidence and use
[incident response](incident-response.md).
