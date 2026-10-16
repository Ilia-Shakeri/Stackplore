# ADR-0012: Replay committed alert changes across restarts

- Status: Accepted
- Date: 2026-10-16
- Scope: Refines the alert replay scope of [ADR-0004](0004-server-sent-events.md).

## Context

Alert acknowledgement and rule edits are durable PostgreSQL transactions.
Process-local stream counters cannot replay their committed changes after a
restart. Publishing before commit can expose rolled-back state; allocating a
sequence without commit serialization can strand a late lower sequence.

## Decision

Write bounded source/rule/episode change metadata in each alert/rule transaction.
Serialize change ID allocation through a transaction advisory lock before the
mutation so visible sequence order follows commit order. Scope replay to the
installation and source. Poll bounded pages with an ordered keyset and bounded
per-client queue. SSE IDs come from committed change IDs; large IDs stay decimal
strings in JSON. Clients refetch authoritative paginated state after changes.

For alerts, the durable bounded-query replay log replaces the process-local
in-memory replay ring required by ADR-0004. The other observation streams keep
their transient ring. No additional process, broker, provider command port or
cross-source global feed is introduced. Queue, read/write, lifetime, stream and
authorization limits still apply. Seven-day eligible retention has bounded
prune quotas and keeps the newest position per source. A per-source highest-pruned
ID detects removed facts even when older rows remain; cursors before that watermark
reset to a snapshot. Missing cursors request an initial snapshot without reset.
A snapshot at zero position emits an empty SSE ID to clear a prior cursor.

## Alternatives

- Keep process-local IDs: cheap, but restart loses replay meaning.
- Publish full durable snapshots per change: unnecessarily duplicates paginated
  product records and expands transport/retention load.
- Add a broker: adds deployment and coordination cost without measured need.

## Consequences and verification

Replay now survives restart while changes remain retained. Each connection polls
under a fixed global cap; this is not proof of multi-replica support or target
capacity. Snapshot cursor must precede the page to avoid missed concurrent commits.
Commit-order, rollback, source isolation, acknowledgement, stale lease, retention
gap, slow-client and cancellation tests are required. Real PostgreSQL and native
HTTP restart evidence are recorded in the active evidence plan.
