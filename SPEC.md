# Stackplore Product Specification

- **Product:** Stackplore
- **Tagline:** Explore every service in your stack.
- **Target release:** 1.0.0
- **Specification status:** Accepted for implementation
- **Implementation status:** Final `1.0.0` candidate after the tagged `0.9.0`
  checkpoint. The accepted scope is mapped to code, tests, documentation and
  evidence in the [requirement matrix](docs/evidence/0.9.0/requirements.md).
  Final exact-tag verification, signed publication and consumer acceptance
  remain distinct gates in the [active evidence plan](docs/implementation-plan.md)
  and [1.0 receipt](docs/evidence/1.0.0/README.md).

This document defines the required behavior and quality bar for Stackplore 1.0.
It describes a target, not current shipped functionality. Delivery order and
current stage status live in [PLAN.md](PLAN.md) and the [evidence plan](docs/implementation-plan.md). Engineering rules live in
[PROJECT_GUIDE.md](PROJECT_GUIDE.md).

## 1. Product summary

Stackplore is a self-hosted infrastructure explorer for developers, operations
engineers, and small infrastructure teams. It will present Docker and
Kubernetes resources through one responsive web interface with live topology,
health, logs, metrics, events, alerts, and tightly guarded recovery actions.

Stackplore is an exploration and troubleshooting tool. It is read-only by
default and does not try to become an infrastructure control plane.

## 2. Product goals

Version 1.0 must:

- make a mixed infrastructure estate understandable from one interface;
- preserve evidence and confidence for every inferred relationship;
- show useful failure, permission, disconnected, and unavailable states;
- support English and Persian as equal product languages;
- work as a responsive website and installable progressive web application;
- provide a deterministic demonstration experience without live providers;
- support Docker and Kubernetes through one provider-neutral domain model;
- make all infrastructure mutations explicit, limited, confirmed, and audited;
- use secure defaults suitable for a self-hosted infrastructure-facing service;
- remain maintainable as an open-source project;
- document real behavior, limitations, deployment, and security boundaries.

## 3. Non-goals

Version 1.0 is not:

- an application performance monitoring suite;
- a replacement for Prometheus or Grafana;
- a log archive or search cluster;
- a Docker management suite;
- a Kubernetes administration console;
- a deployment or continuous delivery platform;
- a secret manager;
- a remote terminal or shell;
- a multi-tenant hosted service;
- a native desktop or mobile application.

The first release supports one self-hosted installation. PostgreSQL is the
primary persistence system. The first production topology uses one active API
process until provider collection, background jobs, and realtime fan-out have
explicit multi-process coordination; repository boundaries keep that runtime
limit out of domain rules.

## 4. Users and roles

### 4.1 Administrator

An administrator can:

- complete first-run setup;
- create, disable, and manage local users;
- configure provider connections through safe references;
- configure retention, alert, notification, and application settings;
- inspect audit history;
- acknowledge alerts;
- use the narrow supported recovery actions when mutation mode is enabled.

### 4.2 Viewer

A viewer can:

- inspect environments, resources, topology, health, logs, metrics, and events;
- search and filter visible infrastructure data;
- inspect alerts and notification state;
- save personal language, theme, and view preferences.

A viewer cannot change users, sources, global settings, alert rules, or
infrastructure resources.

V1 roles are installation-wide. A viewer can inspect every configured source
and all normalized data that the application exposes; v1 has no per-source,
per-environment, or tenant access control. Administrators must not grant the
viewer role to someone who may not inspect the whole installation.

## 5. Required user journeys

### 5.1 First-run setup

1. The user selects English or Persian.
2. When the deployment is reachable beyond loopback, the user supplies the
   short-lived bootstrap proof configured by the operator.
3. The user creates the first administrator.
4. The user selects the deterministic demo or configures a provider source.
5. The application validates the choice without exposing credentials.
6. The user reaches the overview.

First administrator creation must be atomic and available only while the user
table is empty. Once setup succeeds, the network setup endpoint permanently
closes. A deployment exposed beyond loopback must require an operator-supplied,
short-lived, one-use bootstrap proof or an equivalent out-of-band gate.
Bootstrap proof values must never enter persistent storage, logs, errors, or
audit metadata.

### 5.2 Demonstration

1. The user enters an explicitly configured demonstration deployment.
2. A visible badge identifies all data as synthetic.
3. Topology, logs, metrics, events, health, and alert states are useful and
   deterministic.
4. Real provider configuration is unavailable in guest demonstration mode.
5. Every infrastructure mutation is unavailable.
6. Outbound notification delivery is unavailable.

Selecting the demo provider inside a normal authenticated installation does not
disable authentication. Only the dedicated deployment mode may enable a
read-only guest, and that mode must disable real sources and mutations.

### 5.3 Docker troubleshooting

1. An administrator adds a Docker source.
2. Stackplore discovers Engine, Compose, container, service, network, image,
   port, label, health, restart, limit, and mount metadata.
3. A user follows topology from a Compose service to a container.
4. The user inspects health, recent events, logs, stats, and safe metadata.
5. An administrator may restart the exact container only when mutation mode is
   enabled and all authorization and confirmation checks pass.

### 5.4 Kubernetes troubleshooting

1. An administrator adds an in-cluster or mounted kubeconfig source.
2. Stackplore discovers allowed cluster resources and reports missing rights.
3. A user follows ownership, selector, route, endpoint, storage, and
   configuration references.
4. The user inspects conditions, events, logs, metrics, and relationships.
5. An administrator may restart a supported workload or change a supported
   replica count only when all mutation gates pass.

### 5.5 Bilingual use

1. The user changes language without a page reload.
2. The document language and direction change immediately.
3. Layout, navigation, dialogs, charts, topology, and tables remain usable.
4. Dates and numbers follow locale preferences.
5. Resource names, image names, addresses, logs, and technical identifiers keep
   their original content and use left-to-right isolation where needed.

### 5.6 End-to-end troubleshooting

1. A user starts from an unhealthy or unavailable overview summary.
2. The user follows a stable resource link into inventory or topology.
3. Relationship evidence identifies the relevant upstream and downstream
   resources without presenting inference as fact.
4. The user correlates bounded logs, metrics, health evidence, and events on one
   time range.
5. The user reaches an alert or records a clear unavailable or denied state.
6. If a supported action is needed, an administrator sees the exact impact,
   completes every mutation gate, and returns to the same resource evidence.

## 6. Functional requirements

### 6.1 Authentication and authorization

Stackplore must provide:

- local first-run administrator creation;
- out-of-band bootstrap proof for setup exposed beyond loopback;
- Argon2id password hashing with versioned cost parameters;
- random opaque server-side sessions;
- only a digest of each session token at rest;
- HttpOnly session cookies;
- SameSite protection and Secure cookies under HTTPS;
- CSRF protection and origin validation for unsafe requests;
- short-lived, consumed pre-auth CSRF proof for login and first-run setup;
- login throttling without user enumeration;
- session expiry, logout, and administrator revocation;
- current user status and role resolution on every authorized request;
- administrator and viewer authorization;
- administrator user management;
- protection against disabling, deleting, or demoting the last enabled
  administrator;
- a local operator-only administrator recovery command that never reopens
  network setup;
- session revocation after password, role, enabled-state, or recovery changes;
- audit events for authentication and account changes;
- recent-authentication gates for high-risk account, source, notification,
  export, and cleanup changes;
- current-password reauthentication that creates a short-lived, one-use
  server-side grant bound to user, session, and exact purpose;
- current-session rotation and all-other-session revocation after self password
  change, and full target-session revocation after administrator reset;
- no default production credentials;
- no production authentication bypass.

OAuth, SSO, external identity providers, and password recovery mail are outside
the first release.

### 6.2 Infrastructure sources

All sources implement one provider contract with explicit capabilities.

Supported providers:

- deterministic demo;
- Docker Engine;
- Kubernetes.

Docker connections may use:

- a restricted authorization proxy in production;
- a direct local socket only in an explicit loopback development profile, with
  a clear host-control warning;
- an operator-defined TLS TCP target only when hostname verification, mutual
  TLS, and an equivalently narrow daemon-side API allowlist are enforced.

Kubernetes connections may use:

- an in-cluster service account;
- a read-only mounted kubeconfig;
- an explicit kubeconfig context.

Kubeconfig loading accepts only certificate, key, token-file, and CA-file
references mapped by the operator inside configured read-only roots. It rejects
credential `exec` and auth-provider plug-ins, proxy settings, impersonation,
inline credential data, insecure TLS, and references outside those roots.

Connection records hold non-secret metadata and operator-created logical
references to mounted files, environment entries, and remote targets. Browser
requests may select only those allowlisted identifiers; they cannot name an
arbitrary process environment variable, host path, or remote endpoint. The
browser must never receive provider credentials, private keys, raw kubeconfig
content, or Secret values.

### 6.3 Overview

The overview must show:

- environment count and source status;
- service or workload count;
- healthy, degraded, unhealthy, and unknown counts;
- active alert count;
- recent infrastructure events;
- resource usage summaries;
- restart trends;
- top resource consumers;
- environment cards;
- links to topology, resources, logs, events, and alerts.

Every summary must state unavailable data rather than invent a value.
Resource health uses only healthy, degraded, unhealthy, and unknown. Source and
data availability are reported separately as available, stale, unavailable,
unsupported, or permission denied.

### 6.4 Topology

The topology explorer must support:

- zoom, pan, fit, minimap, and full-screen view;
- environment, scope, resource-type, and health filtering;
- search and grouping;
- selected-resource detail;
- neighbor, upstream, and downstream highlighting;
- directional and typed edges;
- edge evidence, provenance, and confidence;
- an accessible list view with equivalent navigation;
- responsive controls.

Every node shows a provider or resource-type icon, name, type, provider, scope,
health, and important state.

Every relationship contains source, target, type, direction, provider,
provenance, confidence, and safe metadata. Inferred edges must be visibly
different from explicit edges. Shared network membership is evidence of
co-location, not proof of an application dependency.

### 6.5 Docker discovery

Docker discovery covers:

- Engine identity and version;
- Compose projects and services;
- containers and state;
- images;
- networks and membership;
- exposed and published ports;
- labels;
- health checks;
- restart count, start time, and exit code;
- mount metadata;
- resource requests or limits when available.

Compose labels form the main grouping source. Explicit
`stackplore.io/depends-on` labels and optional read-only Compose file import may
add relationships. Environment-variable values must not be inspected for
dependency inference.

Compose import parses only the selected operator-mapped file as inert data. It
does not resolve includes, `extends`, build contexts, environment files,
configs, secrets, referenced files, or host environment interpolation. Only
allowlisted service names and declared relationship fields may become topology
evidence.

### 6.6 Kubernetes discovery

Kubernetes discovery covers:

- cluster identity and namespaces;
- nodes;
- Deployments, StatefulSets, DaemonSets, ReplicaSets, Jobs, and CronJobs;
- Pods and container status;
- Services, Ingresses, EndpointSlices or Endpoints;
- PersistentVolumeClaims;
- ConfigMap references;
- Secret references derived from workload specifications, without reading
  Secret objects;
- readiness, restarts, conditions, requests, and limits.

The default source is namespace-scoped and discovers only configured
namespaces. Cluster identity, nodes, and other cluster-scoped data are marked
unavailable in that mode. An explicit cluster-observation profile may grant
the additional read-only rights; it remains separate from mutation rights and
still cannot read Secret objects.

Relationships include ownership, selectors, routes, endpoints, storage,
configuration references, and explicit `stackplore.io/depends-on` annotations.
Missing rights must degrade only the affected capability.

### 6.7 Resource inventory and details

The inventory must support:

- search;
- provider, environment, type, health, and scope filters;
- server-side pagination when data size warrants it;
- sorting;
- column visibility;
- saved local view preferences;
- stable deep links;
- loading, empty, error, denied, and disconnected states;
- responsive table or card presentation.

Resource pages use only relevant tabs from:

- Overview;
- Health;
- Metrics;
- Logs;
- Events;
- Relationships;
- Configuration metadata;
- Alerts;
- Audit history.

### 6.8 Logs

The log experience must support:

- demo, Docker container, and Kubernetes pod/container sources;
- previous-container logs where supported;
- live tail over authenticated SSE;
- pause and resume;
- search and detectable severity emphasis;
- timestamp visibility and wrapping;
- selected-line copy;
- bounded, streamed download;
- multi-container selection;
- configurable redaction;
- bounded browser and server buffers;
- clear unavailable and permission-denied states.

Logs are not persisted as a general archive. The same redaction and access
rules apply to display and download. There is no shell or terminal.

### 6.9 Metrics

Normalized metrics include:

- CPU use;
- memory use;
- network receive and transmit;
- restart count;
- health history;
- requests and limits when available.

Sources are Docker stats, the Kubernetes Metrics API when present, and the demo
provider. Samples use bounded PostgreSQL retention. Charts require accessible table
alternatives. Missing sources produce an unavailable state.

### 6.10 Events

The unified timeline includes:

- Docker events;
- Kubernetes events;
- Stackplore system events;
- safe operational action outcomes without actor or session data;
- alert state changes.

It supports live updates, time range, severity, provider, environment,
resource, search, and deep links.
Authentication, account, configuration, and denied-action events remain in the
administrator-only audit view and never enter the viewer timeline.

### 6.11 Alerts and notifications

Initial rule types:

- resource unavailable;
- resource unhealthy;
- restart threshold;
- CPU threshold;
- memory threshold;
- Kubernetes workload unavailable replicas;
- event severity match;
- provider connection lost.

Rules support create, edit, enable, disable, validation, and an evaluation
interval. Alert states are pending, firing, acknowledged, and resolved.

CPU and memory thresholds compare covered time-weighted gauge means. Workload
unavailable replicas compare the covered time-weighted desired-minus-ready
deficit. Restart thresholds count observed positive counter increments within
the configured window; a reset contributes its new count, never an inflated
delta. Missing, stale, future, partial or unsupported evidence is unknown and
does not clear or fire. Event rules examine their complete bounded evaluation-
interval history with exact scope. Connection-loss rules require raw provider
outage/timeout evidence independent of cached stale observations. Rule source
is immutable after creation; replacement closes the old active episode and
invalidates its claim atomically. A later match after resolution opens a new
episode. Pending periods require continuous proven matches.

Evaluation completion must reject an expired/replaced claim, a changed rule,
newer episode state or a changed/disabled source configuration. Raw collection
evidence must belong to the current source configuration. Durable alert replay
uses committed ordered positions and a protected prune watermark, resets stale
positions to a bounded snapshot, and refetches authoritative paginated state.

Notification delivery includes:

- in-application notifications;
- a generic webhook adapter;
- an optional Matrix adapter;
- test delivery;
- bounded retry and backoff;
- secret-safe errors and audit events.

Alert history links to the affected resource and evidence time range. The
notification center links each notice back to its alert and resource when one
exists.

Outbound notification targets require strict URL, DNS, address, redirect,
timeout, and response-size controls for every adapter. Full webhook URLs,
Matrix access tokens, signing keys, and other notification credentials are
operator-injected secret references, not browser-submitted values. Responses,
logs, audit events, diagnostics, and configuration exports show only safe
labels and masked references.

Matrix homeserver discovery is disabled in v1. The operator maps one exact
canonical HTTPS origin, and the access token is never sent to a different
origin or across a redirect.

Delivery is at-least-once, not exactly-once. Each persisted delivery has one
stable identifier and canonical payload bytes across retries. Webhooks send the
identifier as an idempotency key and bind it into the signature; Matrix uses
the same stable value as its transaction identifier. A timeout is reported as
an uncertain outcome, and destination operators remain responsible for
deduplication.
Matrix transaction identity is scoped to the device and endpoint. Operators must
reconcile pending deliveries before changing destination or credential context;
reusing a transaction ID across a different context does not establish deduplication.

### 6.12 Guarded operations

Stackplore is read-only unless `STACKPLORE_MUTATIONS_ENABLED=true`.

Every mutation also requires:

- exact write permission in a separate deployment profile;
- administrator role;
- a valid CSRF check;
- recent authentication;
- an exact stable target, expected current version, provider capability, and
  server allowlist match;
- an impact preview and a short-lived, single-use confirmation bound to actor,
  source, target, action, and material parameters;
- an idempotency key where retry could repeat an accepted effect;
- a durable audit-intent write before any provider call;
- a safe timeout;
- result feedback;
- a separate audit outcome for success, failure, or uncertain timeout;
- prohibition in guest demonstration mode.

Allowed operations:

- restart one Docker container;
- restart a Kubernetes Deployment, StatefulSet, or DaemonSet with a rollout
  annotation update;
- scale a Kubernetes Deployment or StatefulSet.

Delete, force-delete, exec, shell, arbitrary API forwarding, arbitrary provider
commands, Secret editing, image deployment, and cluster administration are
forbidden.

### 6.13 Audit

Audit events cover:

- login success and failure;
- logout;
- user create, change, disable, and session revoke;
- source create, change, test, and delete;
- settings changes;
- alert rule changes;
- alert acknowledgement;
- operational actions;
- notification tests;
- configuration export;
- data cleanup.

Each event includes timestamp, actor, action, target, outcome, request ID, and
small safe metadata. Passwords, tokens, session identifiers, credentials,
private keys, raw kubeconfig content, and Secret values are forbidden. An
infrastructure action cannot reach its provider until its audit intent is
durably recorded.

Audit history has its own operator-configured quota and retention policy with a
documented minimum. Expired rows are pruned only in bounded batches by the
dedicated audit lifecycle, which writes a cutoff-and-count summary record.
Generic data cleanup cannot delete audit history.

### 6.14 Settings

Settings cover:

- connections and their status;
- users;
- language, theme, date, and number preferences;
- metric retention and log limits;
- alert defaults;
- notification integrations;
- mutation mode status;
- version and application information;
- diagnostics;
- non-secret configuration export;
- bounded data cleanup.

Cleanup must show scope and impact, require confirmation, create an audit event,
and warn about backup needs. Generic cleanup may prune only named,
retention-eligible history such as metric samples, health history,
infrastructure events, and terminal notification deliveries. It cannot delete
users, sessions, sources, current settings, or audit records. Those records
follow separate, explicit lifecycle rules.

Lowering a retention setting does not silently expire existing rows. If the
new value would make stored data newly eligible, it uses the same preview,
purpose-bound recent authentication, hard confirmation, backup warning, and
audit path as explicit cleanup.

Configuration export is administrator-only, requires recent authentication,
uses an explicit field allowlist and size cap, returns `Cache-Control: no-store`,
and creates an audit event. It masks logical reference identifiers and provider
hosts and excludes every credential-bearing value.

### 6.15 Deterministic demo

The demo dataset contains at least two environments and includes a frontend,
API, worker, PostgreSQL, Redis, reverse proxy, and monitoring service. It
contains healthy and degraded resources, exactly identified unhealthy
resources, realistic logs and metrics, meaningful events, one firing alert, and
evidence-backed topology edges.

Values, timestamps relative to the demo clock, and identifiers must be stable
across runs and tests. Synthetic data must always be labeled.

In an explicitly isolated demonstration deployment, process startup resets the
scenario and derived facts to the versioned initial step before serving. An
explicit reset uses that deployment's process restart; the browser has no reset
or scenario-advance control. A fixture-version change replaces prior packaged
synthetic runtime history; different fixture versions are never merged. Reset
is bounded and resumable after interruption and preserves protected audit and
authentication history.

Selecting Demo in an ordinary authenticated installation does not activate that
reset or guest mode. Same-version alert acknowledgements survive ordinary
restarts. A packaged fixture upgrade preserves accounts, audit, administrator
rule configuration, real observations and the Demo source's enable/disable choice
while replacing its packaged synthetic runtime facts. These lifecycle rules are
clarified in [ADR-0013](docs/adr/0013-provider-coverage-and-demo-lifecycle.md).

A guest demo deployment requires a separate data root and database marked with
the active fixture version. Startup fails if it finds a real provider row or
non-synthetic observation history. A database from a normal installation can never be mounted
into guest demo mode.

Guest viewing is a separate explicit setting within the isolated deployment.
It creates no account, session or CSRF proof; only named read routes are public.
The server rejects account, audit, settings and all write routes. Real provider
and outbound notification configuration are forbidden even when unused. The
packaged alert rule is system owned and needs no placeholder user. Observation
facts use the trusted fixed scenario clock; collection receipts and scheduling
use the operational clock. The current `demo-v2` fixture has nine resources in
two environments; [the demo guide](docs/demo.md) records its stable story and bounds.

### 6.16 Progressive web application

The web application must provide:

- a manifest and installable metadata;
- replaceable icons;
- a cached application shell;
- a truthful offline state;
- safe version update notice;
- responsive layouts and touch controls.

The service worker must not cache API responses, sessions, SSE streams, logs,
metrics, or other infrastructure data.
Cacheable shell HTML uses a static hash-based content policy. Nonce-bearing or
personalized HTML is `no-store`, network-only, and never replayed by the service
worker.

## 7. Domain model

Provider-neutral domain types include:

- `User`;
- `Session`;
- `InfrastructureSource`;
- `Environment`;
- `Scope`;
- `Resource`;
- `ResourceSnapshot`;
- `Relationship`;
- `HealthSnapshot`;
- `MetricSample`;
- `InfrastructureEvent`;
- `AlertRule`;
- `Alert`;
- `NotificationDelivery`;
- `AuditEntry`.

Stable identifiers combine source identity with provider-native immutable
identity. Provider-specific metadata lives in bounded typed structures. Raw SDK
objects must never cross the HTTP boundary.

Provider capabilities identify support for discovery, logs, metrics, events,
and each allowed mutation. Unsupported and permission-denied are distinct
states.

## 8. HTTP and realtime contracts

The API lives under `/api/v1`. The canonical OpenAPI 3.1 contract lives in
`contracts/openapi.yaml`.

Phase 4 established the shared provider-neutral schemas without claiming
runtime operations. Phase 5 first published
`GET /api/v1/system/readiness` as a closed error-only gate. Phase 7 replaces
that temporary gate with the real local-session administrator check and extends
the contract to 13 implemented path forms and 16 operations for
pre-authentication, setup, sessions, password changes, recent-auth grants, user
administration, and detailed readiness. The contract declares the actual
session-cookie and CSRF schemes. Detailed readiness returns success only for an
enabled administrator, `401` without a valid session, and `403` for an enabled
viewer. Root operational endpoints remain outside the `/api/v1` contract.
Each later handler phase adds only paths, safe examples, limits, authorization,
and problem responses that exist in the same implementation phase.

API groups:

- auth;
- users;
- setup;
- sources;
- environments;
- resources;
- topology;
- logs;
- metrics;
- events;
- alerts;
- notifications;
- actions;
- audit;
- settings;
- system.

Requirements:

- one consistent problem response;
- request IDs;
- strict request validation;
- authorization at the handler and service boundary;
- stable resource identifiers;
- pagination, filtering, and sorting conventions;
- documented response status;
- no secret-bearing response;
- same-origin cookies;
- bounded request and response sizes;
- safe request examples where a body exists and safe examples for every
  documented response outcome.

SSE carries resource status, events, metrics, logs, and alerts. Streams require
authentication, event IDs, keepalive, reconnection, bounded buffers,
backpressure behavior, per-client cancellation, and an explicit disconnected
state. Credentials must not appear in URLs.

## 9. Interface and experience

### 9.1 Required routes

- `/setup`
- `/login`
- `/overview`
- `/topology`
- `/resources`
- `/resources/:provider/:kind/:id`
- `/logs`
- `/events`
- `/alerts`
- `/audit`
- `/settings`
- `/settings/connections`
- `/settings/users`
- `/settings/notifications`
- `/settings/preferences`
- `/about`
- `/not-found`

### 9.2 Application shell

Desktop uses a collapsible sidebar, top bar, environment selector, global
search, command palette, notification center, language and theme controls, user
menu, breadcrumbs, main area, and optional detail panel.

Mobile uses a compact top bar, drawer navigation, touch-sized controls,
topology-specific controls, and table alternatives without page-level
horizontal overflow.

### 9.3 Visual direction

The visual system uses:

- graphite or ink dark surfaces;
- clean neutral light surfaces;
- indigo-violet primary, cyan information, emerald success, amber warning, and
  rose danger tokens;
- restrained gradients, borders, elevation, spacing, and motion;
- one icon family;
- original replaceable brand placeholders;
- bundled Inter and Vazirmatn, subject to license verification.

The design avoids excessive glass effects, gradients, neon, oversized rounding,
template-like composition, random colors, and emoji icons.

### 9.4 Internationalization

English (`en`) and Persian (`fa`) are complete product locales. Requirements:

- runtime switching and persistence;
- `document.lang` and `document.dir` updates;
- left-to-right English and right-to-left Persian;
- natural professional Persian;
- feature-owned keys;
- locale-aware dates, numbers, and plurals;
- parity checks between catalogs;
- direction-aware placement and icons;
- isolation for technical left-to-right text.

### 9.5 Accessibility

The target is WCAG 2.2 AA practice. The application requires:

- semantic HTML and correct headings;
- full keyboard use and visible focus;
- accessible names and status text;
- dialog focus containment and Escape behavior;
- reduced-motion support;
- live-region announcements where useful;
- color-independent health indicators;
- graph and chart list or table alternatives.

Manual responsive checks cover 360, 768, 1024, 1440, and 1920 pixel widths.

Every data screen must represent loading, empty, error, denied, disconnected,
and retry states where applicable.

## 10. Security and data rules

Security rules are expanded in [docs/security.md](docs/security.md).

Core rules:

- authenticate production users;
- require an HTTPS public origin for every production deployment reachable
  beyond loopback;
- deny mutations by default;
- use least privilege;
- expose no provider credentials to the browser;
- read no Kubernetes Secret object values;
- log no secret values;
- send no telemetry or analytics outside the installation by default;
- make no hidden outbound request;
- use safe HTTP headers and a restrictive content policy;
- cap concurrency, streams, input size, and retention;
- record consequential actions without recording credentials.

Stackplore must not persist raw passwords, raw session tokens, private keys,
raw kubeconfig content, Secret values, or complete environment dumps.

## 11. Runtime and deployment

Development uses a Vite server, Go API server, and same-origin development
proxy.

The current runtime uses PostgreSQL for the API, account recovery and native
backup/restore. It checks process ownership before immutable migrations and
uses bounded pools, queries and transactions. Local real PostgreSQL tests and
a disposable native restore drill provide current baseline evidence; target
production drills remain separate. A non-root Go application image embeds the
web bundle and serves it with the API from one same-origin service. Compose CI
builds and starts a fresh stack, checks assets/readiness and API restart
persistence. Helm routes ingress to the same application Service and receives
lint/render checks; cluster installation, external PostgreSQL TLS, backup/restore,
rollback and production acceptance remain separate gates. Disposable provider
fixtures have their own named receipts.
The 0.2 checkpoint stores safe operator profile references and normalized
observations, runs bounded collection/backoff and fair ordinary-history
retention, and serves cached product reads; logs remain bounded live reads.
Administrator source toggles require exact-version preview, one-use confirmation,
purpose-bound recent authentication and atomic audit. The `0.5.0` candidate adds
restricted mTLS Docker observation, explicit Kubernetes namespaces, independently
published kind/scope authority, typed safe metadata, Pod UID/lifetime checks and
guest isolation. Provider target receipts have named version bounds; the final
local gate passed and exact publication CI passed, including disposable Engine
and Kubernetes guarded-writer jobs. Profile/reference rotation and 1.0 deployment
gates remain. Audit expiry requires the independently verified archive cutoff
described in the current settings guide. Stage 7 aligns production packaging
with ADR-0001 and adds Compose installation/restart proof; the exact 0.7.0 gates
remain open until its changed tree is verified. See [current observation
boundaries](docs/observations.md) and the roadmap for remaining work; these status
notes do not reduce the required 1.0 behavior above.
The locally verified 0.3.0 development checkpoint adds current-configuration raw collection evidence,
covered alert windows, source/rule/lease-fenced completion, committed replay with
a prune watermark and isolated webhook/Matrix lanes. Its local gate and separate
disposable/Helm proof are recorded in the evidence plan. Stage 4 adds recorded
actual-browser/proxy exploration, and Stage 5 has named disposable provider
receipts; an operator receiver, full installation and release acceptance remain
separate target work. See [alert and delivery
boundaries](docs/alerts-notifications.md).
Configuration is validated before listen. Unknown names and non-canonical,
blank or out-of-bound values fail without logging values. The current reference
is [configuration](docs/configuration.md).

Production uses one origin. The Go process serves the API and static web build.
TLS may terminate at a trusted edge, but a non-loopback production profile must
reject startup unless its configured public origin is HTTPS. Only explicitly
trusted proxy peers may establish the original scheme, host, and client
address. Forwarding data from any other peer is rejected. Versioned API
requests must resolve to the exact configured public scheme and authority.

The production image must:

- run as non-root;
- use a read-only root filesystem where practical;
- drop all Linux capabilities, prevent privilege gain, and use the runtime
  default seccomp profile;
- use durable PostgreSQL storage and only reviewed mounted runtime files;
- handle `SIGTERM`;
- expose `/healthz`, `/readyz`, and low-cardinality `/metrics`;
- use health checks;
- provide PostgreSQL-native backup, restore, and disposable restore-verification
  procedures;
- avoid CORS complexity.

`/healthz` returns only process liveness. `/readyz` is `503` until the database
has passed identity, migration-ledger, integrity, and installation checks and
the listener is serving. It becomes `200` while local service work may be
accepted, returns to not-ready when storage health is lost, and returns to
not-ready before shutdown. Loss of one observed provider later changes that
source status but does not make the whole API unready.

The operator command uses PostgreSQL-native custom archives and empty-target
restore. The old development SQLite adapter is retired following real repository
parity and restore checks; no automatic old-development-data import is supported.
See [the native runbook](docs/operations/backup-restore.md).

Health and readiness responses expose only terse process state. Detailed
readiness is administrator-only at `/api/v1/system/readiness`; Phase 7 enforces
the live session, enabled-account, and administrator-role checks. No header,
query value, bootstrap proof, or proxy assertion bypasses that guard.
`/metrics` requires both the exact configured public scheme and authority and a
verified client-network allowlist match. Phase 5 defaults the allowlist to
loopback clients and makes a denied endpoint look absent with `404`.
Operational authentication for `/metrics` remains a later option. Metric names
and labels cannot contain user, source, provider-native, resource, or
log-derived identifiers.

All HTTP work uses server header, read, write, idle, and accepted-connection
ceilings. Every non-operational request additionally uses client-rate and
rate-client ceilings; the terse probes and metrics endpoint are exempt from the
client token bucket. Versioned API work also uses a total request deadline, a
declared-body precheck, capped handler body reads, and buffered-response and
in-flight ceilings. A termination signal marks the service not ready and asks
the server to drain accepted requests within the shutdown bound. It then
cancels the base request context and forces the server closed if the bound
expired. Future SSE and log-tail routes must define separate connection, idle,
write, lifetime, concurrency, queue, and byte limits before they can bypass the
ordinary buffered request deadline.

The recommended Docker deployment uses a restricted authorization proxy.
Kubernetes deployment uses namespace-scoped read-only RBAC by default, a
separate optional cluster-observation profile, and separate optional mutation
permissions. It disables service-account token automount unless the explicit
in-cluster provider profile needs its dedicated identity. No profile requires
cluster-admin.

## 12. Quality and verification

Each phase adds tests with the behavior it introduces. The complete suite will
include:

- Go unit, service, repository, migration, handler, provider, SSE, security,
  integration, and race tests where practical;
- frontend component, hook, form, route, localization, RTL, accessibility,
  query-state, failure-state, and topology tests;
- browser tests for setup, login, demo, language, RTL, themes, topology,
  resources, logs, alerts, authorization, mutation-off behavior, and mobile
  navigation.

Coverage is measured, never estimated. Targets are at least 80 percent for core
backend domain and alert logic and at least 70 percent meaningful frontend
feature logic, with full browser coverage of critical journeys.

Live Docker and Kubernetes checks run only where suitable environments exist.
Mock-backed provider tests do not prove a live integration.

## 13. Definition of done

Stackplore 1.0 is complete only when:

- the complete verification command passes;
- production frontend and backend builds pass;
- the documented demo command starts a meaningful deterministic application;
- English, Persian, left-to-right, and right-to-left behavior pass review;
- local authentication, session security, and role boundaries work;
- Docker and Kubernetes provider contracts have tests and honest live
  validation records where environments exist;
- topology always shows provenance and confidence;
- demo and provider abstractions support logs, metrics, and events;
- alerts evaluate and transition correctly;
- mutations are off by default and require every guard;
- audit records cover required actions without secret data;
- no arbitrary execution path exists;
- Compose deployment passes validation and smoke checks;
- the Helm chart passes lint and render checks;
- continuous checks match documented local commands;
- English and Persian documentation describe only real behavior;
- screenshots come from the final running demo;
- no production TODO remains;
- version is `1.0.0`;
- release artifacts have checksums and a software bill of materials;
- the user authorizes final tag and release publication, and the exact tagged
  source and published artifacts pass their release gates.

## 14. Deferred work

Possible later work includes:

- horizontal API scaling;
- coordinated multi-process provider collection and realtime fan-out;
- external identity providers;
- additional notification adapters;
- a desktop wrapper;
- additional provider types;
- user-defined dashboards;
- longer-term external metric or log integrations.

Deferred work must not weaken the first release boundaries or appear as shipped
functionality before implementation and validation.
