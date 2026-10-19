# Upgrade and rollback

## 1.0.2 to corrective 1.0.3

Schema remains **17**. Keep PostgreSQL, the runtime digest key, source references
and volumes; take a verified native backup before replacing the application.
Automatic history retention now follows the effective saved settings on every
bounded batch. Review those values before startup: the prior worker incorrectly
used fixed defaults. Increasing a retention period cannot recover already deleted
history. Lowering it remains behind the guarded preview and confirmation path.

Normal short Helm API names and runtime PVC names retain their identities. Long
names that previously exceeded the Service limit now use stable hashed API names;
Deployment, Service and Ingress references agree. The chart does not rename the
runtime PVC. Compare the rendered upgrade before applying it to an installation.
Recheck source permissions, stream denial/recovery and effective alert defaults.
Candidate verification is in [1.0.3 evidence](../evidence/1.0.3/README.md);
customer upgrade and rollback still need the installation's own acceptance.

The 0.x baseline has no supported automatic upgrade from old development
SQLite data. PostgreSQL is the only runtime store. Keep any earlier development
data untouched; an import would need a separate offline, bounded, tested tool.

The first 0.1.0 tag canonicalizes its previously untracked SQL files to LF before
publication. Raw migration bytes determine the ledger digest. An archive from
an earlier unversioned CRLF development build therefore needs that matching
build; it is not an automatic 0.1.0 upgrade. Do not edit ledger digests. Use a
new verified target or a separately reviewed data migration if retaining that
development database is required.

## 1.0.1 to corrective 1.0.2

Corrective `v1.0.2` at `376ed03` advances ledger **16 to 17**. Migration 17 adds matching
byte-collation indexes for notification and alert-rule keysets; query seek and
ordering now agree under non-byte database defaults. Existing identifiers,
authorization and the first sixteen migration bytes remain unchanged. Real
PostgreSQL checks cover both ledger-14 and ledger-16 forward upgrades, source
data retention and canonical paging. Full exact-source acceptance remains in
[current evidence](../evidence/1.0.2/README.md).

Take and verify a native backup before the controlled single-owner upgrade.
Budget startup/index work on a restored representative copy. A 1.0.2 storage
command requires its exact 17-entry ledger; restore a 1.0.1 archive with the
matching 1.0.1 tool, verify it, stop that runtime, then upgrade the restored copy.
Do not run an older binary on ledger 17 or edit historical ledger checksums.
Rollback needs the separately verified pre-upgrade backup.

### Retained event text and exposure response

An older runtime may have stored native-event summaries containing credential
text. The corrected REST and initial/subsequent SSE response mapper applies the
deployment redaction policy and an 8,192-rune output limit to those retained
summaries. Migration 17 adds indexes; it does not rewrite these rows or backups.
Focused transport tests pass, while full corrected-source acceptance remains in
the [current evidence](../evidence/1.0.2/README.md).

1. Restrict access to or stop an exposed older runtime. Verify the patched
   release/source and its gates before reopening event/API access.
2. Review the actual exposure through a private operator process. Rotate
   affected credentials when exposure occurred; do not paste sensitive rows,
   provider logs or downloads into a report.
3. Protect the retained database, native archives and earlier exports with the
   operator's access, encryption and archive policy. Output masking leaves their
   stored bytes unchanged.
4. If expiration is appropriate, use the existing
   [confirmed bounded history cleanup](../settings.md#explicit-bounded-history-cleanup).
   It requires an exact preview, purpose-bound recent authentication, one-use
   confirmation and backup acknowledgement; one action removes at most 1,000
   eligible rows before the current retention cutoff. Younger rows remain.
   Deletion does not sanitize WAL, backups or physical media. Preserve protected
   audit history; do not run a blanket SQL purge or unbounded migration scrub.

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

| Migration | Change                                                                   |
| --------- | ------------------------------------------------------------------------ |
| 11        | Current inventory partition coverage and authority fences                |
| 12        | Separate normal/guest deployment identity                                |
| 13        | Durable guarded-operation preview, intent and result state               |
| 14        | Versioned application settings and protected audit policy                |
| 15        | Permits the operator-owned `kubernetes_kubeconfig` mounted profile       |
| 16        | Adds the source alert feed's matching update-time/opaque-ID keyset index |

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

## PostgreSQL 17 patch baseline

Stackplore 1.0.x requires PostgreSQL **17.11 or a later 17.x patch** for the
server and native clients (`psql`, `pg_dump`, `pg_restore`). Version 17.6 is below
the supported security baseline. Review the upstream
[PostgreSQL 17 security list](https://www.postgresql.org/support/security/17/);
server-only updates leave client fixes unapplied.

The upstream [17.11 release notes](https://www.postgresql.org/docs/17/release-17-11.html)
do not require dump/restore for an existing 17.x cluster. This does not remove
these operator gates:

1. Validate a fresh native backup and a restore to a separate empty target with
   patched clients, following [backup/restore](backup-restore.md).
2. Plan maintenance, stop the API, and review intervening upstream release notes.
   Check `output_plugin_libraries`, affected `pgcrypto` data cleanup, and
   `btree_gist`/`ltree` reindex requirements where applicable. Stackplore does not
   create or use those optional extensions or custom logical decoding plugins;
   inspect operator-added components separately. Review scripted `COPY FROM
STDIN` data terminators with the patched `psql`.
3. Update the approved server image and native clients, preserving the existing
   database volume. Record their exact versions and image digest. Never remove
   the volume to make an update pass.
4. Check database readiness, ledger integrity, login, representative reads and
   retained data after restart. Keep the validated backup for recovery; do not
   resume service with the vulnerable 17.6 server or clients.

Application migration tests do not prove a 17.6-to-17.11 server update. Require a
separate disposable patch-upgrade rehearsal and record its exact receipt in the
[active plan](../implementation-plan.md) before claiming that path verified.

## Before an upgrade

1. Read the exact target changelog and migration/contract changes.
2. Verify the tagged source, signed checksum manifest, artifact identity and
   provenance using [release verification](release.md) ([فارسی](release.fa.md)).
   A workflow definition or an unobserved signing run is not a verified artifact.
3. Record current release, database server/client patch versions, deployment settings and
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
Current 1.0.2 release acceptance is recorded in the [active plan](../implementation-plan.md)
and [public proof](../evidence/1.0.2/published-release.json).
Project acceptance does not authorize or prove an unrequested customer rollout.
For recovery from a suspected compromise, preserve evidence and use
[incident response](incident-response.md).
