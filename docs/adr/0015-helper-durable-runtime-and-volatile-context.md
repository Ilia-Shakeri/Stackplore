# ADR-0015: Persist typed helper state and keep working text volatile

- Status: Accepted
- Date: 2026-10-19
- Owners: Project maintainers
- Scope: Version 2.0 target design; accepted during roadmap stage 0
- Clarifies: ADR-0004, ADR-0011, ADR-0012
- Implementation status: Decision accepted; helper schema, worker and recovery proof are not shipped by this record

## Context

Process-local tasks cannot safely account for accepted work, cancellations or
potentially charged calls after a crash. Persisting all conversation and tool
text instead would create a large sensitive archive and backup surface.
Provider timeouts can leave both response and charge unknown. Retrying such a
call is not the same as replaying a completed idempotent HTTP request.

Existing PostgreSQL lease and operation recovery implementations provide
patterns, but helper work needs its own states, ownership fences, usage ledger
and privacy lifecycle. A transient stream ring is not the durable authority.

## Decision

### Durable metadata and volatile content

PostgreSQL stores only bounded typed goals/recipe identifiers, owner user IDs,
selected source/resource/window identifiers, profile and policy versions,
state/step/event metadata, evidence references, usage reservations and safe
audit metadata. Closed typed fields must not become arbitrary text or raw
payload storage in disguise.

Raw question text, conversation messages, log bodies, tool payloads and
inference responses are not persisted by default. Working text stays in
bounded private process memory for at most 30 minutes or eight turns, whichever
comes first. It is not copied into database columns, audit, telemetry, crash
reports, offline caches or error responses. Each conversation requires its
owner's current session authority on every access.

If restart, expiry, restore or ownership loss removes text required by a run,
that run becomes `interrupted` and requires explicit user re-entry. The worker
must not reconstruct or invent missing text. A queued typed recipe may resume
only after fresh actor, policy, source authority and remaining-budget checks.
It must have all required inputs in permitted typed fields.

### State machine, claims and cancellation

Use explicit `queued`, `running`, `completed`, `failed`, `cancelled`, `expired`,
`interrupted` and `uncertain` states. Transitions use repository methods shaped
around complete use cases, short atomic transactions, version checks and
constraints. Store a lease owner, lease expiry, attempt number and fencing
generation; a worker with an old lease or restore generation cannot complete
or publish a result.

Claims use bounded pages and short `FOR UPDATE SKIP LOCKED` transactions.
Reserve admission and per-call usage before dispatch. No transaction remains
open across inference or provider I/O. A service ownership-loss signal stops
admission and fences active work. A fresh process does not make an expired
external call safe to resend.

Closing a stream releases that subscription only. Explicit cancellation stores
intent, cancels bounded I/O and fences completion. Session revocation, actor
disable/role change, source denial, changed policy and restore generation are
checked before each step, result publication and replay. On observed
revocation, fence/discard queued output before cancelling the producer.
Already transmitted network bytes cannot be recalled.

Durable, authorized typed status events are the replay authority. Stream event
positions are scoped to owner/run and restore generation. Use bounded keyset
replay with a retention-gap reset and a REST status refetch. The transient
manager provides fan-out, not durable completion. Do not persist answer text
merely to replay it; after volatile output expiry expose only truthful safe
status and evidence references.

### Idempotency and potentially charged calls

Bind a keyed idempotency digest to owner, conversation, typed goal, scope,
profile/policy versions and all material parameters. A matching repeated POST
returns the same run. A changed fingerprint conflicts. A user-requested new
attempt has a new run and reservation, and must not covertly reuse or refund
the old one.

Record a durable dispatch intent and usage reservation before each inference
call. After an uncertain or partial response, do not retry the call
automatically. A possibly charged call retains `unknown` usage rather than
zero usage. Do not claim exactly-once execution or assume that a timeout
means no provider work occurred.

Unknown-charge reservations are exempt from ordinary conversation expiry.
Keep a minimal safe ledger or tombstone until supported usage evidence
reconciles it, or an explicit audited operator process conservatively settles
the exposure. Retention, quota resets, conversation deletion and restore must
not release that reservation or create a fresh spending allowance from it.
Concurrent reconciliation must be one atomic, idempotent ledger transition.

An old backup can omit later dispatches. After restore, paid admission stays
disabled until the affected interval is reconciled or its maximum exposure is
conservatively reserved under the new generation. The restored balance alone
does not prove available quota. Free local operation must still honor its
capacity, concurrency and time reservations.

### Initial hard limits and lifecycle

These are accepted starting design caps, not measured capacity or a service
guarantee. Enforce the tighter existing v1 policy when limits overlap.

| Resource                 | Initial cap                                            |
| ------------------------ | ------------------------------------------------------ |
| Active runs              | One per user; two per installation                     |
| Queued runs              | Two per user; 16 per installation                      |
| Run and inference time   | 120 seconds per run; three calls, 25 seconds each      |
| Read tools               | 12 calls per run; two parallel reads                   |
| Context                  | 64 KiB decoded text and 16,000 verified input tokens   |
| Output                   | 64 KiB decoded output and 4,096 verified output tokens |
| Validated stream frame   | 16 KiB, within the existing v1 cap                     |
| Ordinary safe metadata   | Seven days, excluding unresolved usage ledger          |
| Authorized status replay | 24 hours                                               |
| Volatile working context | 30 minutes or eight turns, whichever comes first       |
| Locally opted-in logs    | 200 lines, 32 KiB and a 15-minute window               |

Byte and token limits both apply. A profile must provide a verified compatible
pre-dispatch counter through a domain-owned counting port. An approved local
tokenizer must match the profile's exact tokenizer revision, request formatting,
chat template, tool/schema envelope and special-token policy. Test its counts
against the actual runtime before enabling that profile. A runtime's usage
count returned after generation does not prove the pre-dispatch input bound.
Uncheckable required token bounds reject dispatch; a byte estimate is not a
verified token count. A profile without that capability remains unavailable
for production dispatch even if a development connectivity probe succeeds.
Reserve installation/user usage atomically and
apply fair admission and a global kill switch. Currency ceilings use the
selected profile's reviewed actual terms, not a guessed conversion.

Prune ordinary metadata and replay in bounded batches using the effective
policy. Deleting runs or conversations cannot cascade into protected audit
history or unresolved usage rows. Corpus, audit, usage and backup lifecycles
remain explicit and separate. Application deletion does not erase prior
backups or PostgreSQL write-ahead logs.

## Consequences

### Benefits

- Accepted work, quota and cancellation survive process failure without storing raw text.
- Late completions cannot revive cancelled or replaced work.
- Ordinary expiry cannot erase an unknown charge or silently renew its allowance.
- PostgreSQL remains the only durable dependency.

### Costs and risks

- Free-form work cannot transparently resume after its volatile input is lost.
- Answers cannot be replayed after expiry unless a separately accepted retention policy permits it.
- Conservative charge settlement can reduce available quota until operator action.
- Backups and external processing retain their own privacy and billing consequences.
- Single-process ownership still applies; PostgreSQL claims alone do not prove multi-replica safety.

## Alternatives considered

### Persist every message and tool response

This would ease replay but create a default sensitive archive with a wider
retention, access, export and backup surface. It is not selected for 2.0.

### Keep all jobs and usage only in memory

A crash would forget accepted work and potentially charged dispatches. It
cannot provide honest recovery, idempotency or quota invariants.

### Retry every network failure automatically

A timeout can follow accepted processing. An automatic resend can duplicate
cost and cannot be justified by a missing response.

### Reuse the notification queue or transient stream ring as the run store

The lease pattern is reusable, but helper state, privacy and usage have
different lifecycles. Separate repository methods and tables make those
invariants explicit.

## Implementation and verification

Stage 2 must test real PostgreSQL migration from schema 17, rollback of failed
transitions, concurrent admission/claim/reconcile, quota reset, unknown-charge
retention, stale leases, expired context, cancellation, restore generations,
ownership loss, idempotency conflict and crash at every dispatch/commit
boundary. Inspect hot-path query plans and prove bounded retention.

Stage 4 must test status-only replay, owner/session isolation, lost-text
re-entry, reconnect and slow clients. Later gates measure cancellation within
two seconds of explicit local cancel or observed revocation, a 60-minute soak
and 100 lifecycle cycles without orphan tasks, unbounded queues or accepted
late output. This does not measure unobserved provider permission-change delay.

New policy/runtime packages retain all existing floors and target at least
90% branch coverage. Native restore proof includes unresolved usage and paid
admission blocking. No claim above is implemented by accepting this record.

## References

- [PostgreSQL persistence](0011-postgresql-primary-persistence.md)
- [Durable alert replay](0012-durable-alert-change-replay.md)
- [Helper scope](0014-read-only-helper-scope-and-boundaries.md)
- [Authority and egress policy](0016-helper-authority-and-egress-policy.md)
- [Existing notification leases](../../apps/api/internal/postgres/notification_delivery.go)
- [Existing uncertain-operation recovery](../../apps/api/internal/postgres/operation_recovery.go)
- [Version 2 roadmap](../roadmap-v2.md)
