# ADR-0001: Use a Monorepo and One Production Origin

- Status: Accepted
- Date: 2026-07-28
- Owners: Project maintainers

## Context

Stackplore v1 is planned to have a Go service, a React application, a
TypeScript API client, shared UI primitives, an OpenAPI contract, and deployment
assets. These pieces must evolve together because one feature often changes the
domain model, HTTP contract, browser client, and interface in the same release.

Authentication uses cookies and live updates use SSE. Serving the browser
application and API from different origins would add CORS, credential, CSRF,
proxy, and local-development complexity without a v1 requirement for
independent deployment.

## Decision

Keep all product, contract, deployment, and documentation sources in one
repository.

Use a pnpm workspace for `apps/web`, `packages/api-client`, and `packages/ui`.
Keep the Go module in `apps/api` and expose root commands that coordinate Go and
pnpm checks without hiding their native commands.

Use `contracts/openapi.yaml` as the OpenAPI 3.1 source for `/api/v1`. The
TypeScript client must be reproducibly derived from that contract and include a
small project-owned fetch layer for cookie, request-ID, cancellation, and
problem-response behavior.

In production, the Go service serves the built frontend and the API from one
origin and one port. API routes remain below `/api/v1`; `/healthz`, `/readyz`,
and `/metrics` remain operational endpoints. The SPA fallback must never
convert an API or operational 404 into HTML.

In development, Vite proxies the same paths to the Go service. General CORS
support is not enabled by default.

## Consequences

### Benefits

- Contract, service, client, interface, and deployment changes can be reviewed
  and released together.
- Cookie sessions and SSE use normal same-origin browser behavior.
- Production needs one exposed application endpoint and no extra web runtime.
- Local checks can detect contract drift before changes merge.
- A future desktop wrapper can reuse the same API client and frontend features.

### Costs and risks

- Repository checks span Go and JavaScript toolchains.
- The production build must order contract checks, frontend build, and server
  packaging correctly.
- Static asset caching and SPA fallback rules become server responsibilities.
- Independent frontend scaling is not available in v1.

## Alternatives considered

### Separate repositories

This would increase release coordination and make cross-layer contract changes
harder to review. The v1 team and deployment model do not justify that cost.

### Separate production origins

This would require a formal cross-origin credential and CSRF design. There is no
v1 consumer that needs direct cross-origin API access.

### A dedicated production web server

A reverse proxy may still sit in front for TLS, but requiring another runtime
only to serve the application shell would add configuration and health
boundaries. The Go service can serve immutable assets and correct cache headers.

## Implementation and verification

Later phases must:

- create the workspace and Go module without application behavior in Phase 1;
- validate `contracts/openapi.yaml` and client compatibility from Phase 4;
- test that API and operational misses return structured or plain 404 responses,
  never the SPA shell;
- verify cookie login and SSE through the development proxy and production
  server;
- prove that private API and stream responses are absent from PWA caches;
- document any future cross-origin mode in a superseding record.

## References

- [Architecture overview](../architecture.md)
- [ADR-0005: Local sessions and CSRF](0005-local-sessions-and-csrf.md)
