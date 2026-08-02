# Local SQLite Backup and Restore

Phase 6 provides local, operator-only storage commands. There is no browser or
HTTP backup or restore operation. Run each command from the repository root
with the same validated `STACKPLORE_*` environment used by the API service.

## Safety boundary

- `STACKPLORE_DATA_DIRECTORY` names one local, non-root directory. Production
  requires an absolute path. Network shares, UNC paths, device paths, and one
  data directory shared by several machines or API processes are unsupported.
- The database name is fixed as `stackplore.db`. The same directory contains
  `.stackplore.owner.lock`, `.stackplore.maintenance.lock`, and the private
  32-byte `.stackplore.auth-key` digest key.
- The online database backup does not copy `.stackplore.auth-key`. A same-data-
  directory restore keeps the current key. A fresh data directory creates a
  new key on first service start; restored sessions and short-lived proofs are
  already invalidated by restore. Do not copy a key from another live
  installation or expose it with a backup.
- A missing key is regenerated and makes bearer state produced by the old key
  unusable. A malformed, symlinked, linked, or POSIX-permission-exposed key
  file blocks storage startup. Windows operators must verify its ACL. Password
  hashes do not depend on this key.
- The main database and single-file restore source must each have exactly one
  filesystem link. Do not hard-link them across data or backup directories.
- SQLite may create `stackplore.db-wal`, `stackplore.db-shm`, or
  `stackplore.db-journal` while the service runs. Never copy the live main file
  as a backup and never treat one file from that set as the complete database.
- Backup takes a shared maintenance lock and may run while the one owning API
  process is live. Restore takes both the exclusive process-ownership lock and
  the exclusive maintenance lock, so the API must be stopped and no backup may
  be running.
- A backup destination must be a new absolute local file outside the data
  directory. Its existing parent must be private. The command never overwrites
  a destination.
- A restore source must be an existing absolute local regular file in a private
  parent directory. It must be a single SQLite file with no WAL, shared-memory,
  or rollback-journal sidecar.
- The commands write bounded status logs to standard error and no database
  content to standard output. Success is exit status `0`.

## Directory permissions

On POSIX systems, use owner-only directories and files. Phase 6 creates data
directories with mode `0700` and files with mode `0600`; it rejects group or
other access on final storage directories and files.

```sh
install -d -m 0700 /var/lib/stackplore
install -d -m 0700 /var/backups/stackplore
chmod 0600 /var/backups/stackplore/restore-source.db
```

Windows file mode bits do not describe NTFS access-control entries. Phase 6 can
reject symlinks, non-regular files, UNC paths, and device paths on Windows, but
it cannot prove that an ACL is private. The operator must restrict both the
data and backup directories to the service and approved maintenance accounts.
For example, replace the account and paths below, then inspect the final ACLs:

```powershell
$runAsAccount = 'HOST\stackplore-service'
$maintenanceAccount = 'HOST\stackplore-operator'
$systemAccount = 'NT AUTHORITY\SYSTEM'
$dataDirectory = 'C:\Stackplore\data'
$backupDirectory = 'C:\Stackplore-backups'

icacls.exe $dataDirectory /grant:r "${runAsAccount}:(OI)(CI)F"
icacls.exe $dataDirectory /grant:r "${maintenanceAccount}:(OI)(CI)F"
icacls.exe $dataDirectory /grant:r "${systemAccount}:(OI)(CI)F"
icacls.exe $backupDirectory /grant:r "${runAsAccount}:(OI)(CI)F"
icacls.exe $backupDirectory /grant:r "${maintenanceAccount}:(OI)(CI)F"
icacls.exe $backupDirectory /grant:r "${systemAccount}:(OI)(CI)F"
icacls.exe $dataDirectory /inheritance:r
icacls.exe $backupDirectory /inheritance:r
icacls.exe $dataDirectory
icacls.exe $backupDirectory
```

Keep only required service, system-recovery, operator, and backup identities.
Stop if the final listing contains an unexpected trustee.
Storage has no built-in encryption; use encrypted local media and protect any
copy exported by an operator.

## Live backup

Create the private backup parent first. Keep the API running, choose a new
destination name, and run one of these commands. The examples show only the two
storage settings; a production command must retain every other validated value
from the service environment.

PowerShell:

```powershell
$env:STACKPLORE_DATA_DIRECTORY = 'C:\Stackplore\data'
$env:STACKPLORE_DATABASE_BUSY_TIMEOUT = '5s'
go -C apps/api run ./cmd/storage backup --destination 'C:\Stackplore-backups\stackplore-2026-08-02.db'
```

POSIX shell:

```sh
STACKPLORE_DATA_DIRECTORY=/var/lib/stackplore \
STACKPLORE_DATABASE_BUSY_TIMEOUT=5s \
go -C apps/api run ./cmd/storage backup \
  --destination /var/backups/stackplore/stackplore-2026-08-02.db
```

The command checks the source identity, complete migration ledger,
installation metadata, foreign keys, and SQLite health. It uses the SQLite
online-backup mechanism, publishes one new `0600` file atomically, and performs
full integrity, foreign-key, and migration checks on that completed file. A
nonzero exit is not a usable backup result. If a destination is present after a
failure, quarantine it, inspect the failure, and use a different new name for
the next attempt.

Retain more than one known-good generation. Record a checksum after success,
store it separately from the artifact, and test restore on a disposable local
data directory before relying on the backup.

## Offline restore

1. While the current API is still healthy, make a new live backup as a rollback
   point.
2. Stop the API and wait for it to exit. Do not continue merely because HTTP is
   unreachable; restore must be able to acquire the process-ownership lock.
3. Confirm the restore source is the intended private single-file backup and
   verify its operator-recorded checksum.
4. Run the restore command with the same data-directory and busy-timeout
   settings used by the service.

PowerShell:

```powershell
$env:STACKPLORE_DATA_DIRECTORY = 'C:\Stackplore\data'
$env:STACKPLORE_DATABASE_BUSY_TIMEOUT = '5s'
go -C apps/api run ./cmd/storage restore --source 'C:\Stackplore-backups\stackplore-2026-08-02.db'
```

POSIX shell:

```sh
STACKPLORE_DATA_DIRECTORY=/var/lib/stackplore \
STACKPLORE_DATABASE_BUSY_TIMEOUT=5s \
go -C apps/api run ./cmd/storage restore \
  --source /var/backups/stackplore/stackplore-2026-08-02.db
```

Restore rejects another product's database, changed or unknown migration
history, failed integrity, foreign-key violations, unsafe permissions, and a
source with sidecars. It stages the candidate, applies known missing embedded
migrations, performs full checks, revokes every active restored session,
deletes pending pre-authentication challenges and recent-authorization grants,
and increments the restore generation before installing `stackplore.db`.

After exit status `0`, start the API and require `GET /readyz` to return `200`
before reopening access. Every session from the backup is invalid by design;
each user must sign in again. Verify that `.stackplore.auth-key` is still a
32-byte regular file with a private POSIX mode or Windows ACL when reusing a
data directory; on a fresh data directory, the service creates it during
startup.

## Interrupted restore and WAL evidence

Restore uses these durable artifacts inside the data directory:

- `.stackplore.restore.pending`;
- `.stackplore.restore.installed`;
- `.stackplore.restore.stage.db`;
- `.stackplore.restore.rollback.db`.

The stage and rollback names can also have `-wal`, `-shm`, or `-journal`
sidecars. Each such sidecar is restore state and must stay with its base file.

The API and backup command fail closed while restore state remains. If the
restore command is interrupted or reports failure, keep the API stopped and
rerun the exact same restore command with the same source. Recovery either
rolls back an incomplete replacement or validates and finalizes an installed
candidate. Do not delete, rename, or edit a restore marker, stage, or rollback
file by hand. If the same command still fails, preserve the full data directory
and its ACL or mode evidence before diagnosis.

Final installed-marker removal happens only after the checked target and prior
cleanup are safe. If a filesystem metadata-sync fault or reboot makes that
marker reappear after a reported success, startup remains closed. Keep the API
stopped and rerun the same restore command; recovery repeats finalization
without installing the source or incrementing the restore generation again.

A clean shutdown normally checkpoints the current WAL. Restore may normalize a
healthy stopped database set before replacement. If the current database is
corrupt and any `stackplore.db-wal`, `stackplore.db-shm`, or
`stackplore.db-journal` file remains, preserve the main file and every sidecar
as one forensic set after stopping all writers. Do not remove a sidecar merely
to make restore proceed, and never take a main-file-only copy from a live
database. Place the complete evidence set in a separate protected local
directory and verify that copy. To proceed, move the entire stopped database
set, not selected sidecars, out of the active data directory; do not move lock
or restore-marker files. Then use a known-good single-file backup as the restore
source.
