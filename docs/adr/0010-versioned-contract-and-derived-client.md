# ADR-0010: Version the HTTP Contract and Derive the Browser Client

- Status: Accepted
- Date: 2026-08-01
- Owners: Project maintainers

## Context

Stackplore needs one reviewable transport boundary between the Go service and
browser code. Handwritten request and response shapes on both sides would drift
as resource, topology, event, alert, and audit work arrives. Deriving the
entire server from a transport document would instead make domain code depend
on HTTP tooling and weaken the provider-neutral boundary.

Phase 4 establishes schemas before feature endpoints exist. Publishing
speculative operations would create a false runtime contract and make later
service work preserve behavior that was never implemented.

Phase 5 introduces one versioned runtime path before local sessions and roles
exist. The detailed readiness path must be reserved for administrators without
inventing a temporary credential, header, or successful response.

## Decision

Use `contracts/openapi.yaml` as the canonical OpenAPI 3.1 transport contract for
the versioned `/api/v1` surface.

- Keep `paths` empty until an owning feature phase implements and tests an HTTP
  operation.
- In Phase 5, publish only `GET /system/readiness`. Document the reachable
  `400`, `401`, `413`, `429`, `500`, and `503` problems, including the closed
  `401` handler, but no success response or security scheme. This rule permits
  a later success shape only with real local-session and administrator checks;
  Phase 7 supplies them.
- Define shared provider-neutral schemas for identity, resources, health,
  relationships, metrics, events, alerts, audit records, capabilities,
  pagination, filtering, sorting, request IDs, and problem responses.
- Seal transport objects. Do not expose raw provider payloads or an unrestricted
  metadata map.
- Use fixed enums for states whose behavior is controlled by Stackplore. Use
  length- and pattern-bounded code strings only where providers may add safe
  values that Stackplore must preserve.
- Keep Go domain types independent from the contract derivation tool. HTTP adapters
  will map between domain and transport types when operations arrive.
- Derive browser transport types into
  `packages/api-client/src/derived/types.derived.ts`. Derived files are never
  edited by hand.
- Keep request construction, same-origin policy, problem parsing, and request-ID
  handling in reviewed handwritten modules inside `@stackplore/api-client`.
- Make linting, repository policy checks, and derived-output drift detection
  part of the root verification task.

The derivation tool is pinned exactly at `0.99.0`. It is a stable release, but its
pre-1.0 public configuration and output may change in a minor release. Updates
therefore happen only in a focused dependency change that re-derives and
reviews the committed output. The contract linter is also pinned exactly so
local and automated checks apply the same rules.

Phase 7 extends the contract to 13 path forms and 16 operations. It publishes
the implemented pre-authentication, session, password, recovery-grant, user
administration, and readiness shapes; declares the real session-cookie and
CSRF schemes; and exposes a readiness success only after administrator
authorization. The historical Phase 5 closed-gate rule above remains the
reason no speculative success contract existed before that implementation.

## Consequences

### Benefits

- Reviewers can see transport compatibility changes in one versioned document.
- Browser types cannot silently drift from the committed contract.
- Domain and provider ports remain independent from HTTP and derivation details.
- Paths state current runtime truth: shared schemas matured before handlers,
  and operations appear only with tested authorization and response behavior.
- Bounded provider extension codes preserve useful values without allowing raw
  provider objects through the boundary.

### Costs and risks

- A contract change may update the source document, derived output, wrappers,
  mappings, and tests in one change.
- Committed derived output adds review noise and must be checked for drift.
- Domain and transport models still require explicit mapping and parity review.
- Exact tool pins require deliberate maintenance rather than unattended minor
  updates.

## Alternatives considered

### Handwrite browser types

This avoids derived output but permits backend, documentation, and browser
types to disagree without a deterministic failure.

### Derive the Go server and domain model

This reduces transport boilerplate but makes transport tooling shape core
domain and provider ports. Stackplore keeps those boundaries explicit instead.

### Publish planned operations now

This would make documentation look complete before handlers, authorization,
limits, and failure behavior exist. Operations are added only with their owning
implementation phase.

### Use an unversioned root API

This shortens paths but removes the explicit compatibility boundary needed for
a long-lived self-hosted client and server.

## Verification

The repository exposes:

```text
corepack pnpm run derive:api-client
corepack pnpm run check:contracts
corepack pnpm run test:client
corepack pnpm run verify
```

`check:contracts` applies contract lint and repository policy, derives into a
temporary location, and fails when committed derived types are stale.

## References

- [Product specification](../../SPEC.md)
- [Project guide](../../PROJECT_GUIDE.md)
- [Architecture overview](../architecture.md)
- [ADR-0001: Monorepo and same origin](0001-monorepo-and-same-origin.md)
- [ADR-0002: Provider-neutral boundary](0002-provider-neutral-boundary.md)
