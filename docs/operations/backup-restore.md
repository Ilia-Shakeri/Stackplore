# PostgreSQL backup and restore

Stackplore exposes no HTTP backup or restore route. The local storage command
wraps `pg_dump` and `pg_restore`; the tools must be installed on the operator
host and compatible with the PostgreSQL server major version.

Accepted 1.0.2 is immutable source `376ed03`, ledger 17. Its actual Windows
native backup/empty-target restore passed, and the public archive is byte-identical
to that tested trial. The different public database archive was independently
loaded/executed; [published proof](../evidence/1.0.2/published-release.json) keeps
both identities. Use the matching release storage tool; operator backup custody
and a restore check on the actual installation remain required.

After [verifying a native release package](release.md), run its
`./stackplore-storage` executable (`.\stackplore-storage.exe` on Windows).
Replace the `go -C apps/api run ./cmd/storage` prefix in the examples below
with that executable; subcommands, arguments and protected environment inputs
are identical. The Go form is for a source checkout with the pinned toolchain.
The application container contains only the server, so run storage maintenance
from the operator host with compatible PostgreSQL-native tools.

The database URL is read from `STACKPLORE_DATABASE_URL`. Its password is passed
only through the child-process environment, never through command arguments or
logs. Keep the URL and every archive secret.

Native operations cover the application `public` schema only, use a total
15-minute deadline, and never import other schemas from an archive. Use a new
empty database as the restore target. Maintenance ownership is checked on the
same session every 500 ms while the native job runs; loss cancels that job.
Backup syncs the archive before no-overwrite publication and syncs the parent
directory on supported platforms; Windows cannot supply that directory-sync
guarantee. This is not a cluster-wide PostgreSQL server backup.

## Literal database names

Use a valid PostgreSQL URL and percent-encode its database path exactly once.
The storage tool preserves the decoded database name literally through native
`psql`, `pg_dump` and `pg_restore` calls. Percent, equals, quote and backslash
characters in a database name do not become another decoded name or connection
options. Do not pre-decode the path, hand-build extra tool connection options,
or use a different target as a workaround. Passwords remain outside command
arguments. The 1.0.2 regression uses unique owned databases and actual native
backup/empty-target restore; it is not a customer restore receipt.

This build expects ledger **17**. Restore an older archive using its matching
storage release, verify that target, then perform the controlled forward upgrade.
Historical 1.0.1 receipts continue to describe ledger 16.

## Safety contract

- Backup may run while the API is live. It uses PostgreSQL custom archive
  format, validates the archive with `pg_restore --list`, writes mode `0600`
  where supported, and publishes through an atomic no-overwrite hard link.
- Destination must be a new absolute local path. The command never overwrites
  an existing path.
- Restore is offline and only targets an empty database. Never point restore at
  the current production database.
- Restore acquires the same PostgreSQL advisory ownership lock as the API. A
  running API makes restore fail closed.
- `pg_restore --single-transaction --exit-on-error` leaves the empty target
  unchanged when archive installation fails.
- After install, Stackplore verifies the exact migration ledger, revokes all
  restored sessions and pre-authentication proofs, and increments the restore
  generation. A failed final check makes the target untrusted; do not use it.
- Restore uses the storage executable from the archive's matching release.
  An older archive is not upgraded by the restore command. Restore and verify
  first, then use the controlled [forward upgrade](upgrade-rollback.md).
- Archive encryption, remote replication, retention, and PostgreSQL server
  backups are operator duties. Do not rely on one local archive.

## Live backup

Create a private backup directory first. On Linux:

```sh
install -d -m 0700 /var/backups/stackplore
export STACKPLORE_DATABASE_URL='postgresql://stackplore@db.example:5432/stackplore?sslmode=verify-full&sslrootcert=/run/secrets/postgres-ca.crt'
go -C apps/api run ./cmd/storage backup \
  --destination /var/backups/stackplore/stackplore-2026-08-16.dump
sha256sum /var/backups/stackplore/stackplore-2026-08-16.dump
```

On PowerShell:

```powershell
$env:STACKPLORE_DATABASE_URL = 'postgresql://stackplore@db.example:5432/stackplore?sslmode=verify-full&sslrootcert=C%3A%5CStackplore%5Cpostgres-ca.crt'
go -C apps/api run ./cmd/storage backup --destination 'C:\Stackplore-backups\stackplore-2026-08-16.dump'
Get-FileHash -Algorithm SHA256 'C:\Stackplore-backups\stackplore-2026-08-16.dump'
```

Store the checksum away from the archive. A nonzero exit means no published
backup is trusted.

## Offline restore to a new database

1. Verify the archive checksum.
2. Create a new empty PostgreSQL database with a least-privilege owner. Do not
   create application tables in it.
3. Stop all Stackplore processes that could use the target database.
4. Set `STACKPLORE_DATABASE_URL` to the new empty database, not the old one.
5. Run restore.

Linux example:

```sh
export STACKPLORE_DATABASE_URL='postgresql://stackplore_restore@db.example:5432/stackplore_restore_20260816?sslmode=verify-full&sslrootcert=/run/secrets/postgres-ca.crt'
go -C apps/api run ./cmd/storage restore \
  --source /var/backups/stackplore/stackplore-2026-08-16.dump
```

PowerShell example:

```powershell
$env:STACKPLORE_DATABASE_URL = 'postgresql://stackplore_restore@db.example:5432/stackplore_restore_20260816?sslmode=verify-full&sslrootcert=C%3A%5CStackplore%5Cpostgres-ca.crt'
go -C apps/api run ./cmd/storage restore --source 'C:\Stackplore-backups\stackplore-2026-08-16.dump'
```

The target owner needs rights to create objects in `public`. The command rejects
a target that already contains any base table.

## Verify before cutover

Start one API instance against the restored database on a private address. Check
`/healthz`, `/readyz`, administrator login with a known current credential, user
and audit reads, and representative resource/history queries. Restored sessions
must not work. Run the application integration suite against this disposable
target when possible.

Only after checks pass, update the deployment secret to the restored database
URL and perform the normal controlled rollout. Keep the old database read-only
until the rollback window closes. A database switch, DNS change, archive delete,
or old-database removal is outside this command and needs its own approval.

## Failure handling

- `tool_unavailable`: install compatible `pg_dump` and `pg_restore` tools.
- `ownership_unavailable`: stop the API or other maintenance task for the
  target and retry.
- `restore_rejected`: keep the target out of service. Inspect tool versions,
  archive integrity, target emptiness, and migration compatibility.
- `migration_rejected`: the archive ledger does not exactly match this build.
  Use the matching application release, then follow its documented upgrade path.

Never add `--clean`, edit migration ledger rows, copy authentication keys between
installations, or bypass the empty-target check.
