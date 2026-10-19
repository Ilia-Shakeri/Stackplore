# ADR-0014: Keep the troubleshooting helper read-only and evidence-bound

- Status: Accepted
- Date: 2026-10-19
- Owners: Project maintainers
- Scope: Version 2.0 target design; accepted during roadmap stage 0
- Clarifies: ADR-0001, ADR-0002, ADR-0006, ADR-0010, ADR-0011, ADR-0013
- Implementation status: Decision accepted; helper runtime and release proof are not shipped by this record

## Context

The published 1.0.3 release exposes normalized infrastructure observations and
narrow guarded recovery operations. The next product version adds a
user-triggered troubleshooting helper. A free-form executor would expand the
application's authority, bypass existing provider boundaries, and confuse a
plausible explanation with an observed infrastructure fact.

Existing inventory, health, topology, metric, event, log, alert and workload
history ports provide useful evidence. Existing roles are installation-wide;
a selected source or environment is a query scope, not a new permission model.
The optional helper must preserve ordinary exploration when inference is off,
unavailable, slow or out of quota.

## Decision

### Product and authority boundary

The 2.0 helper is user-triggered and read-only. It explains observations,
investigates an explicitly selected scope and time window, states missing or
conflicting evidence, and suggests safe next reads. It does not schedule
autonomous investigations or call any infrastructure mutation port.

Do not inject `domain.OperationProvider`, operation services, shell, exec,
arbitrary SQL, generic provider proxies, arbitrary network clients, host file
readers or Secret readers into the helper tool registry. Tool names and closed
argument schemas are server-owned. Browser input and returned text cannot
register tools, widen capabilities, select endpoints or supply host paths.

V1 guarded operations remain available only through their existing contract and
all gates in ADR-0006. A possible later proposal handoff is separate work; it
cannot execute a generated action, extend the allowlist, or block 2.0 completion.
There is no automatic undo or rollback promise.

### Runtime shape and dependency direction

Keep one modular Go service and PostgreSQL. Use bounded in-process workers and
PostgreSQL claims instead of adding a queue service. No vector database,
separate orchestrator, framework or second service is required. A later
departure needs a new accepted decision supported by measurements.

Transport and jobs call helper use cases. Use cases depend on domain-owned
inference, evidence, policy and repository ports. PostgreSQL and inference
adapters implement those ports. Provider SDK types, HTTP types and SQL drivers
remain outside domain packages. Significant I/O accepts `context.Context`.

Compose the helper under `cmd/server` with the same service lifetime and
installation ownership-loss fence as observation workers. The inference
gateway must not run before durable admission and the shared evidence policy
are ready. Feature enablement and external transmission are off by default.
Paid or external inference is forbidden in public guest mode.

Preserve the same-origin React interface, strict TypeScript, shared interface
package, existing motion system and English/Persian parity. Private helper
responses and streams are excluded from service-worker and offline caches.

### Contracts and evidence

Add HTTP operations under `/api/v1/assistant` and `/api/v1/analysis` only when
their owning implementation stage supplies tested handlers. Product version 2
does not require a new HTTP major version. OpenAPI remains canonical and
derived transport types change with the implemented contract.

Evidence uses normalized read ports, including `domain.ObservationReader`,
separate log reads and existing alert/workload history readers. Every evidence
item carries a server-issued reference, source and resource identity, observed
time or exact query window, relevant configuration/capture generation, and
availability/completeness/synthetic state. References never grant authority.

Validate source authority before reads and before publication or replay.
Distinguish unavailable, unsupported, denied, partial and stale evidence. Missing
metrics are not zero. Observations gathered across time or sources are not
advertised as one atomic infrastructure snapshot.

Output contains validated sections for observed facts, qualified hypotheses,
contrary evidence, gaps and safe next steps. A temporal correlation is not
proof of cause. Cite only server-issued references that pass current authority
checks. Reject invented citation IDs, arbitrary links, images, active markup,
output-supplied tool actions and raw token streams.

### Roles and history

Administrator and viewer roles retain installation-wide observation rights.
Scope filters do not implement per-source, per-environment or tenant access
control. Conversation metadata is owner-private; knowing its identifier does
not grant another user access. Administrator audit access remains distinct
from conversation content access.

A runbook corpus is inert, operator-approved and versioned. Approving a corpus
also defines which existing roles may read it. Use bounded PostgreSQL full-text
search; no embeddings or automatic learning from conversations are required.
No browser-supplied path or URL crawler is added. Corpus text does not enter a
remote context under the default transmission policy.

## Consequences

### Benefits

- The helper reuses tested observation boundaries without gaining write power.
- Existing exploration remains useful when the optional inference path fails.
- Grounded output, typed tools and explicit gaps make results reviewable.
- One service and one persistence family preserve the supported deployment shape.

### Costs and risks

- A qualified hypothesis can still be wrong; evaluation and honest wording are required.
- Installation-wide viewer access remains broad and must be understood by operators.
- Evidence freshness and availability can change while a run is in progress.
- Corpus approval, role review, citation resolution and privacy policy add operational work.
- Read-only requests can still consume provider, database and inference capacity.

## Alternatives considered

### Autonomous executor or generic tool framework

This would expand authority beyond the existing product boundary and make
one-use confirmation, scoped permissions and audit intent harder to prove.
The 2.0 helper receives only narrow read ports.

### Separate worker service and additional state stores

No measurement currently demonstrates that a second process, broker or vector
database is needed. They would add ownership, recovery and deployment paths.

### Treat query scope as access control

The existing roles cannot enforce that claim. A finer access model needs a
separate accepted decision, schema, policy and end-to-end proof.

### Publish future routes immediately

This would describe planned behavior as a live contract. Follow ADR-0010 and
publish each operation with its implementation and negative tests.

## Implementation and verification

Roadmap stages 1–3 establish the gateway, durable runtime and shared privacy
policy before dispatch is exposed. Stages 4–7 add conversation, bounded reads,
incident workflows and curated knowledge. Later hardening and release stages
provide independent runtime and artifact evidence.

Verify absence of write ports in helper wiring; unknown tool rejection;
source/identity/version mismatch; missing and denied evidence; per-owner route
and stream access; returned instruction attacks; safe rendering; bilingual
keyboard and reduced-motion journeys; profile failure; and unchanged v1
exploration with the feature off. Real provider, database, browser and
deployment claims need their own source-bound receipts.

Acceptance of this design does not implement a handler, migration, profile,
tool, user interface, corpus or release. The active plan records those states.

## References

- [Provider-neutral boundary](0002-provider-neutral-boundary.md)
- [Guarded operations](0006-read-only-and-guarded-mutations.md)
- [Versioned contract](0010-versioned-contract-and-derived-client.md)
- [PostgreSQL persistence](0011-postgresql-primary-persistence.md)
- [Scoped inventory authority](0013-provider-coverage-and-demo-lifecycle.md)
- [Durable helper runtime](0015-helper-durable-runtime-and-volatile-context.md)
- [Authority and egress policy](0016-helper-authority-and-egress-policy.md)
- [Version 2 roadmap](../roadmap-v2.md)
