# Architecture

The current working target is **1.0.2**, a corrective candidate after a fresh
whole-repository audit of the 1.0.1 delivery. The review reopened quality at
**8/10**. Confirmed defects and an accepted navigation gap have corrections and
focused regressions. The complete local gate and fresh production-browser suite pass; exact-source
hosted, signed publication and independent public-consumer gates remain pending.
[Fresh review and current proof](evidence/1.0.2/README.md).

The immutable 1.0.1 source and its [published receipt](evidence/1.0.1/published-release.json)
retain their original results and scope. Those checks do not certify this new
candidate. Customer production and air-gapped installation require separate
operator evidence.

## System shape

Stackplore uses one Go application serving the embedded same-origin web bundle and API, PostgreSQL, and provider adapters. An optional ingress may provide public TLS. The browser never receives provider credentials and never calls infrastructure endpoints directly.

```text
Browser
  |
  | HTTPS, / and /api/v1
  v
Go application (embedded React assets + API + health/readiness/metrics)
  |
  |-- local auth and policy
  |-- product services
  |-- provider registry
  |-- collection, retention, alert and notification workers
  |
  +--> PostgreSQL
  +--> Demo provider
  +--> Docker read provider (opt-in)
  +--> Kubernetes read provider (opt-in)
```

The server wires Demo plus operator-enabled Docker and Kubernetes read adapters.
Startup registers missing safe source profiles in PostgreSQL and preserves
existing enabled state and versions. Administrators can preview and toggle
registered profiles; they cannot create profiles or supply endpoints, host
paths or credentials. Docker direct socket access is limited to loopback
development. The restricted profile uses a private mutually authenticated
HTTPS proxy and a fixed read allowlist. Kubernetes requires an explicit bounded
namespace list; optional cluster observation adds only nodes and named namespace
reads. Official vendor clients stay inside the adapters. See the
[Docker](docker-provider.md) and [Kubernetes](kubernetes-provider.md) profiles.

## Repository layout

```text
apps/api/                     Go service, domain, adapters, commands
apps/web/                     React application
packages/api-client/          OpenAPI-derived types and same-origin client
packages/ui/                  shared accessible interface components
contracts/openapi.yaml        version 1 HTTP authority
apps/api/postgresmigrations/  embedded forward-only PostgreSQL migrations
deploy/                       application image, Compose, and cluster assets
docs/                         decisions, security, development, operations
```

## Fresh corrective boundaries

Schema 17 adds canonical byte-order notification/rule indexes; their keyset
seek predicates and sort order share that collation. Later-page native event
permission denial closes the collection result instead of retaining earlier data. Provider
log/event text shares credential-userinfo masking, and native database tools
preserve literal database identity through one URI decode and escaped libpq
selection. The interface adds fixed role-aware keyboard navigation and presents
localized severities and observed node facts. See the scoped
[fresh audit](evidence/1.0.2/fresh-audit.md) for proof and pending acceptance.

## Backend boundaries

The backend follows inward dependencies:

1. `internal/domain` owns identities, normalized models, query limits, validation, provider ports, and repository ports.
2. Provider packages translate external data into domain results. They cannot expose raw provider payloads.
3. PostgreSQL and authentication adapters implement domain/service ports.
4. HTTP handlers and jobs call typed services and domain ports; handlers map bounded wire input and domain results into closed response shapes. Provider adapters remain behind those ports.
5. `cmd/server`, `cmd/operator`, and `cmd/storage` compose runtime boundaries.

Domain code does not import HTTP, PostgreSQL, Docker, Kubernetes, or UI packages. Stable resource and relationship IDs are digests of normalized identity fields, not display names.

## Persistence

PostgreSQL is the only supported store. Server and native `psql`, `pg_dump` and
`pg_restore` clients require 17.11 or a later patch within major 17. The canonical
[database image policy](../deploy/postgres-image.json) pins the upstream base
and reviewed `stackplore-postgres:1.0.2` derivative. It rebuilds unchanged
official gosu 1.19 with Go 1.26.9 while retaining the PostgreSQL engine and
entrypoint. Actual helper/engine/root-drop behavior, complete scans, bound SPDX
and independent archive SQL startup passed for historical 1.0.1; the current
1.0.2 candidate requires its own exact-source gates. The release must include the database Docker-save archive
and signed SBOM. Existing 17.x minor-update guidance follows upstream and requires
operator maintenance and validated backup/restore; no specific 17.6-to-17.11
runtime experiment is claimed. Startup:

1. validates a canonical connection URL and bounded pool settings;
2. opens a pool with query and connection deadlines;
3. takes the installation ownership lock before any migration;
4. takes the migration advisory lock and verifies the immutable ledger;
5. applies embedded forward-only migrations;
6. retains ownership on a dedicated session and fails closed on loss;
7. initializes the installation row and local digest-key boundary;
8. reports database readiness.

The schema covers installation identity, users, account credentials, sessions,
pre-auth and recent-auth grants, safe source profile references, resource identity,
observations, topology, health, metrics, events, alerts, notifications, audit
history, source confirmation digests and authentication throttles. The server
wires authentication, cached product reads, collection and retention workers,
source settings, alert evaluation/management, in-app notification reads and the
opt-in PostgreSQL-leased webhook and Matrix workers. Raw collection outcomes,
covered metric/replica windows, fenced evaluation claims and committed alert
change metadata have explicit domain ports. Versioned application settings,
masked export, confirmed history cleanup and protected audit lifecycle have
separate service and repository boundaries.
The versioned demo seed uses the evaluator/lifecycle and durable fixture marker.
See [alert and delivery boundaries](alerts-notifications.md) and
[ADR-0012](adr/0012-durable-alert-change-replay.md).

Collection is bounded by source, concurrency, page, item and aggregate deadline
limits, with cancellation and retry backoff. A successful inventory partition
advances its kind/scope completion generation even when another kind is denied.
Partial partitions cannot retire unseen resources. Denied, unsupported and
omitted partitions cannot expose prior cached identities. Operator authority
changes fence all cached reads until fresh coverage publishes; an outage cannot
clear that fence. Source version changes reject old capture commits. Product inventory,
detail, health, topology, metrics, history and events use PostgreSQL read ports;
HTTP/SSE reads do not trigger collection. Logs remain bounded live provider
reads. Optional ports retain separate status and observation clocks. See the
[collection and retention reference](observations.md) for fixed limits and stale,
disabled, interrupted and synthetic behavior.

Retention runs bounded transactions with fair quotas across eligible metrics,
health, events and terminal ordinary deliveries. It preserves the newest metric
per resource/name, synthetic facts, current state, pending/audit-linked deliveries
and audit history. Protected audit quota and minimum-retention
policy, verified-archive cutoff, and separately authenticated settings export
and ordinary-history cleanup. Audit rows remain outside ordinary cleanup.

PostgreSQL backup uses custom-format `pg_dump`, validates the artifact with `pg_restore --list`, and publishes it through an atomic no-overwrite hard link. Restore accepts an empty target only, runs in one transaction, verifies the exact migration ledger, advances the restore generation, and invalidates sessions and pre-auth state.

## Provider model

Every provider reports all known capabilities as `available`, `stale`, `unavailable`, `unsupported`, or `permission_denied`, with completeness and a machine reason code when required. A missing capability is never simulated.

Core read ports are:

- environments, resource inventory, and resource detail;
- health evidence and bounded retained health history;
- topology relationships with provenance and confidence;
- bounded historical logs and cancelable tails;
- current normalized metrics and bounded metric history;
- event history and cancelable watches;
- capability discovery.

Queries own hard limits, time windows, sorting, and opaque cursors bound to normalized filter state. Result validation rejects cross-source data, duplicate identities, unstable order, unrequested records, oversized payloads, invalid time ranges, and data attached to unavailable status.

The versioned Demo provider exposes nine resources across shop and operations
environments, with declared route/dependency/monitoring evidence, bounded logs
and events. Collection stores 972 stable metric facts: nine names, nine resources
and twelve five-minute samples ending at the controlled scenario clock. Provider
tail/watch capabilities are explicitly unsupported. Authenticated HTTP SSE sends
bounded polled snapshots; it does not prove a native provider watch or durable
provider event feed. Alert SSE is a separate PostgreSQL-backed committed change
feed, including demo alerts; it does not depend on provider watch support.

Cached metric status is conservative at source/port level. A mix of supported,
unsupported, denied or failed resource metric sets can produce partial metrics.
Retained samples and an aggregate port status do not prove every resource's
current provider availability; samples keep their own observation timestamps.
The source-wide conservative status remains explicit; it is not a persisted
per-name resource status. Current samples older than five minutes are stale,
using the controlled clock for Demo. Future samples cannot shadow valid current
facts. History retains each actual timestamp. Partitioned native event history
requires a resource identity with current authority; unscoped native events are
hidden because their former namespace cannot be proven. Fixed safe operation
and alert-transition events can remain source-wide and carry no account or
audit identifiers. Denied native event-history capability closes the combined
feed; the contract exposes no separate system subfeed.

An explicit isolated demonstration binds a separate empty PostgreSQL database
and private data directory to a persisted deployment mode. Restart resets only
its derived fixture state; account/security/audit history is protected. Normal
fixture upgrades preserve administrator rules and source enablement. Guest mode
has no account, session, CSRF proof or fabricated user identity, allows only named
read routes, and rejects real providers, mutations and outbound configuration.
See [ADR-0013](adr/0013-provider-coverage-and-demo-lifecycle.md) and the
[demonstration guide](demo.md).

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

Connected pages include overview, resources, resource detail, topology, logs,
events, alerts, audit, search, notification center, source settings and
alert/notification settings. Source settings support an administrator-only exact
enable/disable preview and confirmation; unknown operator profiles cannot be
enabled. Paginated exploration has URL filters, bounded log target/range/tail,
copy and streamed matching-log export, evidence-based topology with accessible
lists, and independent timestamp-based metric and health histories. Product pages and transport guards
are feature-owned modules. Lists cap at 1,000 loaded items; graph rendering caps
at 200 nodes with 50-node/50-edge list pages. Activity checks use actual server
session deadlines and bounded, single-flight renewal. Local format preferences,
reviewed detail metadata, guarded restart/scale and mounted Kubernetes profiles
are implemented; browser-created provider profiles remain unsupported.
See [exploration](exploration.md). Requests carry explicit
source identity; direct resource links use their route source. Missing metric
samples are not summed into fake zero totals.

The production service worker caches only the four public offline assets.
Application HTML, API responses, account data, logs, streams and metrics stay
network-only. Offline navigation shows a public unavailable page, and a waiting
worker requires the user's update action before activation and reload.

## Deployment

The local Compose profile has two services:

- PostgreSQL with a named data volume and health check;
- one non-root Go application serving embedded assets and same-origin API routes, with a named runtime-key volume and readiness gate.

The application binds to host loopback. The private bridge uses an explicit subnet for metrics access. `docker compose down` preserves both named volumes.

The cluster chart uses an external PostgreSQL Secret, one Go application replica with a ReadWriteOnce runtime-key claim and `Recreate` strategy, probes, resource bounds, restricted security contexts, and optional TLS ingress. The application remains single-replica until digest-key distribution and multi-process ownership are redesigned.

CI defines six separate jobs: code/real PostgreSQL/race/browser verification,
image/chart checks, fresh Compose setup/UI/restart, actual kind Helm installation
with external TLS PostgreSQL, restricted Docker and namespace-scoped Kubernetes.
The two provider jobs exercise both read and explicitly enabled guarded writer
fixtures. Exact run receipts establish which revision passed. Helm requests use
a Service port-forward; customer ingress-controller HTTPS remains separate.

Release verification must cover the separately deployed PostgreSQL derivative in
addition to native executables, the application image, source dependencies and
history. It requires exact base/derivative and engine/helper metadata, root-drop
probes, a complete image vulnerability scan and an SPDX package inventory bound
to that identity. Compose/CI first build owned patched database fixtures; Helm
continues to use external PostgreSQL under the server/client policy. The historical 1.0.1
main/tag/trial/public/archive gates passed; 1.0.2 requires fresh recorded proof. A scan of the application image alone does not prove the database image.

Both code/deploy jobs passed for the 0.1.0 and 0.2.0 tagged revisions, including
Linux race tests and image builds; see [exact run evidence](implementation-plan.md).
The verified unchanged 0.3.0 tag was pushed under renewed authorization on
2026-10-17. Both exact remote code/deploy runs passed; the evidence plan links
them and distinguishes these builds from target startup.
The 0.4.0 exploration checkpoint also passed its exact remote jobs. Stage 5 has
separate real disposable Engine and Kubernetes observation receipts; these do
not prove a full application installation. At that checkpoint, Compose startup,
chart installation and final acceptance remained later-stage gates. Their
replacement results and current open gates are in the active evidence plan.

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

- Fixed operator profiles, persisted enabled state, collection/retry scheduling and cached observation reads are implemented. Local/PostgreSQL/native and exact tagged CI results are recorded separately in the active evidence plan.
- Browser source creation and endpoint/path/credential editing remain unsupported. Operator credential references are deployment-owned and rotated through the protected restart procedure in [guarded actions](operations/guarded-actions.md).
- Fixed guarded restart/scale operations and application settings are implemented. Deployment and provider write permissions remain false by default; live target operation is not inferred from source tests.
- Restricted Docker and namespace-scoped Kubernetes use official clients and bounded partition authority; the active plan records their separate live proof.
- Covered alert evidence and durable replay are implemented in the locally verified 0.3 checkpoint; unsupported or incomplete windows stay unknown. Full local and separately named disposable/Helm proof passed; exact outcomes live in the evidence plan.
- Webhook and Matrix are opt-in. Disposable local HTTPS proof and live operator receiver proof are separate; no live production receiver claim follows from unit or SQL tests.
- Guarded operations have unit and PostgreSQL coverage plus opt-in disposable Engine/kind jobs; their exact remote execution is recorded in the active plan.
- Real PostgreSQL, native backup/restore, browser geometry, Compose startup and chart installation have separately named gates and receipts. A provider fixture does not prove an application deployment; a checked release does not prove a customer's production installation.
- Stage 7 embeds built assets in the single Go application, following ADR-0001. CI Compose startup is still distinct from cluster installation and target acceptance. Provider clients follow ADR-0002.

See [ADR-0011](adr/0011-postgresql-primary-persistence.md), [observations](observations.md), [security](security.md), [deployment](operations/deployment.md), and [backup/restore](operations/backup-restore.md).
