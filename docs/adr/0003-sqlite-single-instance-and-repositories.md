# ADR-0003: Use SQLite with One Active API Instance

- Status: Accepted
- Date: 2026-07-28
- Owners: Project maintainers

## Context

Stackplore is a self-hosted, single-installation v1 product. It needs durable
users, sessions, source metadata, preferences, resource snapshots, bounded
metric history, events, alerts, notification delivery, and audit records. The
default deployment should remain small and require no external database.

SQLite fits that deployment, but a metrics and events workload can create write
contention and disk growth. SQLite also does not provide the coordination
needed for multiple active API replicas. Treating it as if it did would create
unsafe migrations, inconsistent sessions, duplicate background work, and split
SSE state.

## Decision

Use SQLite as the v1 system of record and support exactly one active API process
per database.

Use `modernc.org/sqlite` as the pure-Go driver candidate, subject to an explicit
compatibility check when storage is implemented. Pin the selected release in
`go.mod`.

Access persistence through focused repository ports owned by the domain or
application boundary. SQL rows and driver types stay in the storage adapter.
Use an explicit transaction abstraction only for use cases that must commit
multiple repository changes atomically.

Configure and enforce:

- an exclusive process-ownership lock acquired before migrations or background
  work;
- embedded ordered migrations before readiness;
- a migration table and exclusive migration ownership;
- foreign keys on every connection;
- WAL mode;
- a bounded busy timeout;
- short transactions and a controlled write path;
- bounded read ranges, pagination, and suitable indexes;
- retention quotas and batch pruning for high-volume history;
- a separate audit quota, documented minimum retention, bounded expiry pruning,
  and prune-summary record;
- UTC storage and locale conversion only at presentation boundaries;
- a local operator-only online-backup command and an offline,
  integrity-checked restore procedure that revokes restored sessions.

Credential records prefer environment or mounted-file references. Raw
passwords, raw session tokens, provider private keys, kubeconfig content,
Kubernetes Secret values, and complete environment dumps are not persisted.

Do not claim horizontal availability for v1. Helm and Compose deployments set
one replica, use a persistent volume suitable for SQLite, and avoid concurrent
writers from another process. Kubernetes uses a `Recreate` update strategy so
an update does not overlap old and new API processes.

Repository interfaces must be shaped so a later PostgreSQL adapter can
implement the same domain operations. SQL-dialect details and auto-increment
identifiers must not become public API contracts.

## Consequences

### Benefits

- Default deployment has one service and one data volume.
- Transactions, backups, and local troubleshooting are straightforward.
- A pure-Go service build avoids a native SQLite toolchain.
- Domain tests use repository fakes and storage tests focus on adapter behavior.

### Costs and risks

- V1 cannot run multiple active API replicas.
- Sampling, alert evaluation, and event ingestion must keep write transactions
  short and bounded.
- Operators must monitor disk space and configure retention.
- WAL and database files require a correct persistent filesystem and backup
  procedure.
- Repository ports reduce but do not eliminate work needed for a later
  PostgreSQL migration.

## Alternatives considered

### PostgreSQL as a required dependency

PostgreSQL is a strong option for a later high-availability mode, but requiring
it now increases installation and operational cost for the primary
single-installation use case.

### Multiple API replicas sharing SQLite

Shared-disk locking does not solve migration ownership, background scheduler
duplication, session coordination, or live-stream fan-out. This mode is
unsupported.

### Direct SQL from application services

This would couple domain behavior to schema and driver details and make another
storage adapter much harder.

### Store all observed provider data indefinitely

Infrastructure data is high volume and often reconstructable. Unbounded
retention creates an avoidable disk-exhaustion failure.

## Implementation and verification

Later phases must:

- verify driver compatibility, WAL behavior, busy timeout, and cancellation;
- run migrations against empty and previous-version databases;
- run repository contract tests against real temporary SQLite databases;
- test contention, rollback, foreign keys, pruning, and bounded queries;
- test session-token digest storage and audit field filtering;
- test that a second process cannot acquire database ownership;
- test Kubernetes render output for one replica and `Recreate`;
- document consistent backup, restore, retention, and disk-full recovery;
- make readiness fail on migration or required local database failure;
- reject or document any deployment configuration above one replica.

## References

- [Architecture overview](../architecture.md)
- [ADR-0004: Server-sent events](0004-server-sent-events.md)
- [ADR-0005: Local sessions and CSRF](0005-local-sessions-and-csrf.md)
