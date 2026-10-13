# Architecture

Status: active 1.0 implementation

## System shape

Stackplore uses a same-origin web edge, one Go API process, PostgreSQL, and provider adapters. The browser never receives provider credentials and never calls infrastructure endpoints directly.

```text
Browser
  |
  | HTTPS, / and /api/v1
  v
Web edge (static React files + reverse proxy)
  |
  | trusted private HTTP hop
  v
Go API
  |-- local auth and policy
  |-- product services
  |-- provider registry
  |-- alert and notification workers
  |
  +--> PostgreSQL
  +--> Demo provider
  +--> Docker read provider (opt-in)
  +--> Kubernetes read provider (opt-in)
```

The server wires Demo plus operator-enabled Docker and Kubernetes read adapters. Sources are runtime configuration; persistent browser connection workflows and discovery scheduling are unfinished. Docker direct socket access is limited to loopback development. The current Kubernetes collector requires an explicit cluster observation profile; namespace-only/partial permission behavior remains open.

## Repository layout

```text
apps/api/                     Go service, domain, adapters, commands
apps/web/                     React application
packages/api-client/          OpenAPI-derived types and same-origin client
packages/ui/                  shared accessible interface components
contracts/openapi.yaml        version 1 HTTP authority
apps/api/postgresmigrations/  embedded forward-only PostgreSQL migrations
deploy/                       container, Compose, edge, and cluster assets
docs/                         decisions, security, development, operations
```

## Backend boundaries

The backend follows inward dependencies:

1. `internal/domain` owns identities, normalized models, query limits, validation, provider ports, and repository ports.
2. Provider packages translate external data into domain results. They cannot expose raw provider payloads.
3. PostgreSQL and authentication adapters implement domain/service ports.
4. HTTP handlers parse bounded wire input, call typed services/providers, and map results into closed response shapes.
5. `cmd/server`, `cmd/operator`, and `cmd/storage` compose runtime boundaries.

Domain code does not import HTTP, PostgreSQL, Docker, Kubernetes, or UI packages. Stable resource and relationship IDs are digests of normalized identity fields, not display names.

## Persistence

PostgreSQL is the primary store. Startup:

1. validates a canonical connection URL and bounded pool settings;
2. opens a pool with query and connection deadlines;
3. takes the installation ownership lock before any migration;
4. takes the migration advisory lock and verifies the immutable ledger;
5. applies embedded forward-only migrations;
6. retains ownership on a dedicated session and fails closed on loss;
7. initializes the installation row and local digest-key boundary;
8. reports database readiness.

The schema covers installation identity, users, credentials, sessions, pre-auth and recent-auth grants, source and resource identity, observations, topology, health, metrics, events, alerts, notifications, audit history, and authentication throttles. The live server wires authentication, product reads, alert evaluation and management, in-app notification reads, and the opt-in PostgreSQL-leased generic webhook worker. Discovery scheduling and remaining product settings remain 1.0 work.

PostgreSQL backup uses custom-format `pg_dump`, validates the artifact with `pg_restore --list`, and publishes it through an atomic no-overwrite hard link. Restore accepts an empty target only, runs in one transaction, verifies the exact migration ledger, advances the restore generation, and invalidates sessions and pre-auth state.

## Provider model

Every provider reports all known capabilities as `available`, `stale`, `unavailable`, `unsupported`, or `permission_denied`, with completeness and a machine reason code when required. A missing capability is never simulated.

Core read ports are:

- environments, resource inventory, and resource detail;
- health evidence;
- topology relationships with provenance and confidence;
- bounded historical logs and cancelable tails;
- current normalized metrics;
- event history and cancelable watches;
- capability discovery.

Queries own hard limits, time windows, sorting, and opaque cursors bound to normalized filter state. Result validation rejects cross-source data, duplicate identities, unstable order, unrequested records, oversized payloads, invalid time ranges, and data attached to unavailable status.

The Demo provider is synthetic and deterministic. It exposes six resources, health states, five relationships, bounded logs, three current metrics per resource, and event history. Provider tail/watch capabilities are explicitly unsupported. Authenticated HTTP SSE sends bounded polled snapshots; it does not prove a native provider watch or durable alert-change feed.

## HTTP and authentication

The versioned API lives at `/api/v1`. `contracts/openapi.yaml` is linted by a closed policy and generates browser types. Browser adapters still validate unknown runtime data before use.

Local authentication uses:

- Argon2id password records;
- opaque random session tokens stored only as keyed digests;
- host-only `HttpOnly`, `SameSite=Lax` cookies;
- exact same-origin enforcement and a session-bound CSRF proof on unsafe requests;
- one-use pre-auth proofs for sign-in and setup;
- one-use recent-auth proofs bound to exact high-risk purposes;
- current user role and enabled state resolution on every authenticated call;
- permanent first-run setup closure and an operator recovery command.

HTTP middleware applies request IDs, fixed security headers, public-origin and proxy checks, bounded request and response bodies, concurrency limits, timeouts, per-client rate limiting, safe access logs, and safe problem responses. `/healthz` is process liveness; `/readyz` includes database readiness. `/metrics` requires both the exact public origin and an allowed client network.

## Frontend

The React application uses strict TypeScript, TanStack Router, a small same-origin client, runtime response guards, and shared interface components. Authentication and product APIs enter through explicit contexts, which keeps routes testable.

English and Persian catalogs must have exact key parity. Locale changes update `lang`, `dir`, typography, number/date formatting, navigation, dialogs, and technical-text isolation. Provider IDs, resource IDs, and log text keep explicit LTR isolation inside Persian RTL pages.

Connected pages include overview, resources, resource detail, topology, logs, events, alerts, audit, search, notification center, connection status and alert/notification settings. Connection mutation, preferences, full detail tabs, log controls and guarded infrastructure actions remain planned. Requests carry explicit source identity; direct resource links use their route source. Missing metric samples are not summed into fake zero totals.

## Deployment

The local Compose profile has three services:

- PostgreSQL with a named data volume and health check;
- a non-root API with a named runtime-key volume and readiness gate;
- a read-only non-root Nginx edge serving the web app and proxying same-origin API traffic.

Only the edge binds to host loopback. The private bridge uses an explicit subnet so proxy trust is narrow and reviewable. `docker compose down` preserves both named volumes.

The cluster chart uses an external PostgreSQL Secret, one API replica with a ReadWriteOnce runtime-key claim and `Recreate` strategy, multiple stateless web replicas, probes, resource bounds, restricted security contexts, optional TLS ingress, and a web disruption budget. The API remains single-replica until digest-key distribution and multi-process ownership are redesigned.

CI defines locked package installation, full code verification, a real PostgreSQL integration service, Compose model validation, chart lint, and both image builds. These definitions are not target-environment proof until they run successfully in the release system.

Both code/deploy jobs passed for the 0.1.0 tagged revision, including Linux race
tests and image builds; see [exact run evidence](implementation-plan.md).
Compose startup, cluster installation and live provider checks remain open.

## Security and reliability rules

- Read-only provider access is the default.
- Provider credentials remain server-side and secret values are dropped at adapter boundaries.
- External errors and raw payloads do not cross the public API.
- No operational change exists without explicit capability support, administrator authorization, recent authentication, confirmation, idempotency, timeout, result verification, and audit.
- Synthetic data stays marked through domain, HTTP, and UI layers.
- Pagination, histories, stream buffers, line sizes, requests, responses, and retention are bounded.
- Database migrations are additive and forward-only; backup and restore are operator workflows, not request handlers.
- Local, CI, Compose, PostgreSQL, browser, and target-cluster evidence are reported as separate proof levels.

## Current limits

- Persistent provider configuration, discovery/reconnect and native live collection are not complete.
- Source configuration and provider credentials are not managed by the live API.
- Product observations are not yet persisted or scheduled.
- Generic webhook delivery is opt-in and locally verified; a real receiver run remains target-environment proof.
- Guarded operations are not live.
- Real local PostgreSQL 17.11 tests and a native backup/restore drill ran on a disposable private cluster. Helm 3.17.3 lint/render ran locally. Container startup, real browser geometry, live Docker/Kubernetes and target production drills are still unproved.
- The current separate static web edge and custom fixed HTTP collectors differ from ADR-0001 built-web serving and ADR-0002 vendor clients. Stage 5/7 must align these implementations with accepted decisions, or obtain a justified superseding decision before 1.0.

See [ADR-0011](adr/0011-postgresql-primary-persistence.md), [security](security.md), [deployment](operations/deployment.md), and [backup/restore](operations/backup-restore.md).
