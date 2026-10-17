# ADR-0013: Publish scoped inventory authority and isolate demonstration lifecycle

- Status: Accepted
- Date: 2026-10-17
- Owners: Project maintainers
- Clarifies: ADR-0002, ADR-0007, ADR-0011

## Context

A source-wide completion marker cannot retire deleted Pods while an unrelated
optional kind returns 403. Treating a partial source as complete deletes denied
cache; retaining every old generation forever makes deleted objects look live.
Operator namespace or profile changes can also revoke access to cached objects
before a new successful collection exists.

The packaged demonstration runs in ordinary authenticated installations as well
as explicitly isolated deployments. Ordinary same-version restarts must preserve
acknowledgements and administrator choices. Public demonstrations need a smaller
trust surface and repeatable reset without manufacturing an administrator login.

## Decision

Collectors emit at most 2,048 disjoint inventory coverage partitions. Each binds
source, environment, resource kind, all scopes or one exact scope, and a typed
observation status. Empty successful lists still carry coverage. Only available,
complete, fully paginated partitions can advance their completion generation
and retire unseen resources. Missing, bounded, denied or failed partitions never
prove deletion. Worker truncation downgrades completion before persistence.

PostgreSQL publishes all partition outcomes and the source publication generation
atomically behind the active configuration/capture fence. Reads match resource
identity against the published authority. Denied, unsupported or omitted scopes
are hidden. Cached failures are stale; partial health is unknown. Retirement is
bounded separately from publication, so visibility does not depend on finishing
all cleanup batches. Legacy full captures remain supported for the packaged
demonstration and repository contract fixtures.

Operator policy changes persist only a digest of reviewed configuration, advance
the source version and close cached reads. A failed collection cannot reopen
them. A fresh complete capture or explicit partition publication must establish
the new authority. Disabled global Kubernetes observation is omitted from scoped
authority; it cannot expose previously observed Nodes through a stale fallback.

Provider facts use closed typed metadata. No raw maps, provider payloads,
environment values, Secret values, host mount paths or credentials are accepted.
Stored metadata is validated and bounded; the HTTP contract and browser repeat
that boundary.

An isolated demonstration requires its own empty private runtime directory and
a fixture-version-marked PostgreSQL deployment. Directory and database modes
cannot be switched between ordinary and isolated use. Startup rejects real
sources, non-synthetic history and provider/outbound credential configuration.
The ordinary database mode remains ordinary even if it currently holds only
packaged synthetic resources.

In ordinary installations, same-version restarts preserve durable state. A
packaged fixture upgrade replaces its derived runtime history while preserving
accounts, protected audit, administrator rule configuration and the source's
enable/disable choice. Isolated demonstration startup resets its scenario and
derived runtime facts before serving; explicit reset uses a process restart.
Fixture versions are never merged. Cleanup is bounded and resumable through a
pending marker. Identity and replay sequences are retained; old replay positions
are invalidated with the durable watermark. Protected audit has its separate
lifecycle and is never part of fixture cleanup.

Guest viewing is a separate explicit setting within isolated mode. It creates
no user, session, authentication proof or idle renewal. Only named product GET
routes are public. Account, audit, settings and every write route are denied on
the server. Streams retain global/per-peer queues, limits, keepalive, lifetime
and cleanup. The fixture's rule is system-owned without a user foreign-key
placeholder when no account exists. Its alert still comes through the normal
rule, evidence, alert persistence and replay boundaries. Outbound delivery stays
disabled independently of fixture alert creation.

## Verification

Real PostgreSQL tests cover independently denied partitions, empty successful
lists, bounded partial collections, protected unseen cache, policy changes and
outages, mode/root mismatch, account-free guest seeding, process reset and audit
preservation. Provider wire tests cover SDK requests, permission isolation,
bounded reads and omission of forbidden endpoints. Real disposable Engine and
cluster checks have separate required gates; their definitions alone are not
execution evidence. The active plan records exact outcomes.

## References

- [Provider-neutral boundary](0002-provider-neutral-boundary.md)
- [Deterministic isolated demo](0007-deterministic-isolated-demo.md)
- [PostgreSQL persistence](0011-postgresql-primary-persistence.md)
- [Active evidence](../implementation-plan.md)
