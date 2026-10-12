# ADR-0011: Use PostgreSQL as the Primary Persistence System

- Status: Accepted
- Date: 2026-08-16
- Supersedes: [ADR-0003](0003-sqlite-single-instance-and-repositories.md)

## Context

Stackplore 1.0 stores authentication state, provider configuration metadata, normalized inventory, health history, metric samples, infrastructure events, relationships, alerts, notification deliveries, audit records, and application settings. Discovery and historical observation create concurrent write and time-range workloads. The product also needs tested schema upgrades, bounded retention, reliable transactional invariants, connection readiness, operator backup and restore, and a deployment path that is not limited to one process owning one local file.

The earlier SQLite decision optimized for a small single-instance release. That constraint no longer matches the accepted Stackplore 1.0 product target.

## Decision

PostgreSQL is the only supported primary persistence system for Stackplore 1.0.

The Go service uses a bounded PostgreSQL connection pool. All database I/O is context-aware and parameterized. Use cases retain domain-owned repository ports and transaction-shaped methods; SQL and driver types remain inside the PostgreSQL adapter.

Schema changes are ordered immutable migrations. Startup serializes migration work with a PostgreSQL advisory lock, validates the migration ledger, applies pending migrations transactionally where PostgreSQL permits, and refuses changed or unknown history.

The schema uses foreign keys, unique constraints, check constraints, and indexes aligned to keyset pagination, retention, and time-range paths. High-volume history is pruned in bounded batches. Audit retention remains separate from generic cleanup.

Readiness includes a bounded database round trip and migration state. Shutdown stops new work, cancels background work, drains bounded HTTP work, and closes the pool.

Production backup and restore use PostgreSQL-native tools and an operator runbook. A restore is not trusted until a disposable target passes schema, integrity, application-readiness, and representative read checks.

## Consequences

- A PostgreSQL service or managed PostgreSQL database is required.
- Local development and self-hosted evaluation use the same database family as production.
- Real PostgreSQL integration tests are required for migrations, repositories, concurrency invariants, and retention behavior.
- The service can later support more than one API replica only after background-work ownership, stream delivery, and provider collection coordination are designed and tested. PostgreSQL alone does not make the application horizontally safe.
- SQLite code, dependencies, licenses, file ownership locks, file backup/restore commands, configuration, and documentation are removed after PostgreSQL parity is proven.
- Existing development SQLite data has no automatic in-place upgrade promise. If an import tool is offered, it must be explicit, offline, bounded, tested, and documented; it cannot keep SQLite as a runtime dependency.

## Rejected alternatives

### Keep SQLite for local mode

Two persistence implementations double migration and repository proof, create environment drift, and weaken clean-install confidence. One PostgreSQL contract is simpler to operate honestly.

### Add Redis beside PostgreSQL

No measured requirement currently justifies another stateful dependency. Bounded in-process queues and PostgreSQL remain the default. A future addition needs its own evidence and decision record.

### Couple services directly to PostgreSQL queries

This would leak persistence rules into use cases and providers. Domain repository ports remain the boundary.

## Verification

Acceptance requires the Milestone 1 evidence in [the active implementation plan](../implementation-plan.md), including clean installation, real PostgreSQL tests, concurrency invariants, readiness failure and recovery, reviewed hot-path query plans, and a disposable backup/restore drill.
