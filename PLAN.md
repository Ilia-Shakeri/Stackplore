# Stackplore Delivery Plan

- **Target:** 1.0.0
- **Current phase:** Phase 2 — complete, awaiting maintainer commit
- **Delivery model:** One numbered phase, one reviewed commit

This plan is ordered. A later phase does not begin until the current phase is
verified, committed by the maintainer, and the worktree is clean.

## Delivery gates

### Entry gate

Before each phase:

- read `PROJECT_GUIDE.md`, `SPEC.md`, and this file;
- verify the current branch and latest commit;
- verify the worktree is clean;
- check required local tools;
- confirm the exact phase boundary;
- inspect relevant architecture decisions.

### Exit gate

Before each phase is offered for commit:

- format all changed files;
- run phase checks and existing regression checks;
- inspect every changed and untracked file, including UTF-8, declared
  line-ending, trailing-whitespace, and final-newline checks for untracked text;
- run `git diff --check`;
- resolve failures caused by the phase;
- update accurate documentation and `CHANGELOG.md`;
- mark the phase complete in this file;
- report observed results and environment limits;
- provide one Conventional Commit subject;
- stop before the next phase.

No check is reported as passing unless its command ran successfully. Mock-backed
provider tests and live provider validation are reported separately.

## Current tool notes

Phase 2 validation on 2026-07-29 reconfirmed:

| Tool | System state | First required phase | Validation state |
| --- | --- | --- | --- |
| Git | 2.53.0 | 0 | Available |
| Node.js | 22.23.0 | 1 | 24.18.0 LTS verified from a checksummed repo-local archive |
| Corepack | 0.34.6 | 1 | Bundled 0.35.0 verified through the pinned Node.js archive |
| pnpm | Missing | 1 | 11.17.0 verified through repo-local Corepack cache |
| Go | Missing | 1 | 1.26.5 verified from a checksummed repo-local archive |
| Make | Missing | 1 | PowerShell, POSIX shell, and direct root equivalents verified |
| Docker and Compose | Missing | 28 | Require local or external validation host |
| kubectl | Missing | 29 | Require cluster validation host |
| Helm | Missing | 29 | Install only with explicit approval |

Repo-local validation tools live under ignored `.tools`; Phase 1 installed no
global tool. Make itself remains unavailable on this Windows host.

## Architecture baseline

- Go API with Chi, `log/slog`, explicit constructors, and domain ports.
- React, strict TypeScript, Vite, and pnpm workspace.
- TanStack Router and Query, React Hook Form, Zod, Tailwind CSS, Radix
  primitives behind project components, Lucide icons, xyflow, and Recharts.
- OpenAPI 3.1 contract under `/api/v1`.
- Pure-Go SQLite, embedded forward-only migrations, an exclusive database
  process lock, and one API process.
- Demo, Docker, and Kubernetes behind one provider capability contract.
- Authenticated SSE with bounded queues for all browser live streams.
- One-origin production serving from the Go process behind required HTTPS
  outside loopback.
- Read-only operation by default.
- Docker access through a restricted proxy in the recommended deployment.
- Kubernetes namespace-scoped read-only RBAC by default, with a separate
  cluster-observation profile and no Secret object reads.
- English and Persian from the start.

Exact dependency patches are pinned when their owning phase first adds them.

## Phase progress

- [x] Phase 0 — Product specification and repository governance
- [x] Phase 1 — Monorepo and developer toolchain
- [x] Phase 2 — Design system and application shell
- [ ] Phase 3 — English, Persian, RTL, and localization
- [ ] Phase 4 — Domain contracts and OpenAPI foundation
- [ ] Phase 5 — Production-ready backend foundation
- [ ] Phase 6 — SQLite storage and migrations
- [ ] Phase 7 — Authentication, users, and authorization
- [ ] Phase 8 — Deterministic demo provider
- [ ] Phase 9 — Overview dashboard
- [ ] Phase 10 — Interactive topology explorer
- [ ] Phase 11 — Resource inventory and detail pages
- [ ] Phase 12 — Realtime streaming and infrastructure timeline
- [ ] Phase 13 — Unified log exploration
- [ ] Phase 14 — Metrics and historical charts
- [ ] Phase 15 — Docker discovery and health
- [ ] Phase 16 — Docker logs, stats, and events
- [ ] Phase 17 — Docker topology inference
- [ ] Phase 18 — Kubernetes discovery and health
- [ ] Phase 19 — Kubernetes logs, metrics, and events
- [ ] Phase 20 — Kubernetes topology inference
- [ ] Phase 21 — Global search, filtering, and command palette
- [ ] Phase 22 — Alerts and notifications
- [ ] Phase 23 — Guarded operations and audit history
- [ ] Phase 24 — Onboarding, connections, and settings
- [ ] Phase 25 — Progressive web application and future app boundaries
- [ ] Phase 26 — Accessibility, performance, and resilience
- [ ] Phase 27 — Comprehensive testing
- [ ] Phase 28 — Container images and Docker Compose deployment
- [ ] Phase 29 — Kubernetes deployment and Helm chart
- [ ] Phase 30 — Continuous integration, security, and release workflow
- [ ] Phase 31 — Complete English and Persian documentation
- [ ] Phase 32 — Final 1.0 quality review and release preparation

## Phase details

### Phase 0 — Product specification and repository governance

Deliver:

- `SPEC.md`, `PROJECT_GUIDE.md`, and `PLAN.md`;
- ADR directory and initial accepted decisions;
- initial architecture and security model;
- Apache-2.0 license;
- editor, line-ending, and ignore rules;
- honest English and Persian README skeletons;
- contribution, security reporting, and conduct policies;
- issue forms and pull request template;
- initial `CHANGELOG.md`;
- repository and Conventional Commit conventions;
- full phase checklist.

Hard boundary:

- no application scaffold;
- no package manifest;
- no build, runtime, or deployment claim.

Phase 0 checklist:

- [x] Product scope and non-goals are complete.
- [x] User journeys and acceptance rules are complete.
- [x] Architecture boundaries and accepted decisions are documented.
- [x] Threat model covers infrastructure-facing risks.
- [x] Repository and commit conventions are documented.
- [x] English and Persian README files state current truth.
- [x] Community and security reporting files are present.
- [x] Issue and pull request templates are present.
- [x] Line endings, editor rules, ignore rules, and license are present.
- [x] Relative documentation links resolve.
- [x] Sensitive and private values are absent.
- [x] `git diff --check` and repository inspection pass.

Commit:

```text
docs: define Stackplore product architecture and delivery plan
```

### Phase 1 — Monorepo and developer toolchain

Deliver:

- pnpm workspace and root scripts;
- pinned Node.js, Corepack boot path, pnpm, and Go versions;
- React and strict TypeScript web scaffold;
- Go API scaffold and initial `/healthz`;
- root Makefile plus direct shell and PowerShell command equivalents;
- `.env.example` with no secrets;
- formatting, linting, type checks, and basic tests;
- reproducible bootstrap instructions;
- frontend smoke screen.

Phase 1 checklist:

- [x] pnpm workspace, root tasks, exact manifests, lockfile, and checksum files
  are present.
- [x] Node.js, Corepack, pnpm, Go, and direct dependencies use exact stable
  pins compatible with the selected toolchain.
- [x] The strict TypeScript React smoke screen passes lint, type, test, and
  production build checks.
- [x] The Go API uses Chi, binds only to loopback, and tests the terse
  `/healthz` contract and method boundary.
- [x] Make, direct PowerShell, PowerShell wrapper, and POSIX shell task paths
  are documented; both wrappers were exercised.
- [x] `.env.example` contains development-only non-secret values.
- [x] Bootstrap verifies tool versions, installs the frozen workspace lock,
  and downloads checksummed Go modules.
- [x] The root verification task passes.
- [x] A live development boot served the smoke page and proxied `/healthz` to
  the API on a configurable loopback port.

Commit:

```text
chore: scaffold Stackplore monorepo and developer tooling
```

### Phase 2 — Design system and application shell

Deliver:

- CSS design tokens;
- light, dark, and system themes;
- original replaceable logo, favicon, and icon placeholders;
- bundled and licensed typography;
- project UI wrappers for button, input, select, dialog, drawer, dropdown,
  tooltip, tabs, badge, card, table, toast, and skeleton;
- error boundary;
- responsive sidebar, top bar, mobile navigation, and route shell;
- component showcase when it adds verification value.

Phase 2 checklist:

- [x] Semantic CSS tokens use one project prefix and drive Tailwind, project
  wrappers, shell styles, and both color themes.
- [x] Light, dark, and system preferences resolve before paint, persist
  locally, follow system changes, and update browser theme metadata.
- [x] Original replaceable logo, favicon, PWA icon, wordmark, and social
  preview sources are present and parse as safe SVG.
- [x] Inter and Vazirmatn variable fonts bundle locally with exact package
  pins, notices, and shipped license copies.
- [x] Every planned Phase 2 control is exported behind the private shared
  interface package and shown on the development-only showcase route.
- [x] The route shell covers setup, login, product, resource detail, settings,
  about, and not-found paths without inventing provider data.
- [x] Direct routes, exact path parameters, unknown routes, theme state,
  primitive behavior, accessible structure, and mobile focus behavior have
  automated regression tests.
- [x] Browser review at 360, 768, 1024, 1440, and 1920 CSS pixels found no
  page-level horizontal overflow in light or dark presentation.
- [x] The frozen bootstrap, root verification task, Go module verification,
  production builds, and live loopback web-to-API proxy check pass.

Commit:

```text
feat(web): establish the responsive design system and application shell
```

### Phase 3 — English, Persian, RTL, and localization

Deliver:

- English and Persian feature catalogs;
- locale provider and runtime switch;
- persisted language;
- document language and direction updates;
- locale-aware dates, numbers, and plural forms;
- direction-aware primitives and typography;
- missing-key and catalog parity checks;
- complete shell copy in both languages.

Commit:

```text
feat(i18n): add complete English and Persian localization foundations
```

### Phase 4 — Domain contracts and OpenAPI foundation

Deliver:

- provider-neutral identity, resource, health, relationship, metric, event,
  alert, and audit contracts;
- focused inventory, health, topology, log, current-metric, event, and
  capability provider ports;
- versioned OpenAPI 3.1 structure;
- consistent problem, pagination, filter, and sorting models;
- typed frontend API package;
- reproducible contract validation.

Commit:

```text
feat(api): define versioned API and shared domain contracts
```

### Phase 5 — Production-ready backend foundation

Deliver:

- validated configuration;
- structured logging and request IDs;
- panic recovery, safe errors, and security headers;
- server and request timeouts;
- bounded rate-limit foundation;
- validated HTTPS public-origin and trusted-proxy configuration with
  non-loopback production startup rejection;
- graceful start and stop;
- `/healthz`, `/readyz`, `/metrics`, and `/api/v1`;
- terse root probes, administrator-only `/api/v1/system/readiness`, and
  protected low-cardinality metrics;
- middleware and unit tests.

Commit:

```text
feat(api): add the production-ready service foundation
```

### Phase 6 — SQLite storage and migrations

Deliver:

- SQLite initialization with WAL, foreign keys, and busy timeout;
- exclusive process-ownership lock;
- embedded migration runner;
- transaction helpers and repository interfaces;
- user, session, source, resource snapshot, metric, event, alert, notification,
  and audit tables;
- retention helpers and indexes;
- empty and upgrade migration tests;
- local operator-only SQLite online-backup command;
- offline restore, integrity, permission, and session-revocation procedure;
- second-process, backup, and restore tests.

Commit:

```text
feat(storage): add SQLite persistence and database migrations
```

### Phase 7 — Authentication, users, and authorization

Deliver:

- atomic empty-user first-run detection and permanently closing administrator
  setup;
- short-lived, one-use out-of-band bootstrap proof for setup exposed beyond
  loopback;
- Argon2id password records;
- login, logout, and non-enumerating throttling;
- server-side session digest storage;
- secure cookies, CSRF, expiry, rotation, and revocation;
- bounded pre-auth CSRF cookie-and-header proof for login and setup;
- current-role lookup and session revocation after account security changes;
- installation-wide viewer and administrator policy;
- last-enabled-administrator protection and local operator recovery;
- user administration API and interface;
- setup and login interfaces;
- current-password reauthentication with short-lived, one-use, user-, session-,
  and purpose-bound server grants for high-risk changes;
- self-password session rotation plus other-session revocation, and full target
  session revocation after administrator reset;
- authentication audit events and security tests.

Commit:

```text
feat(auth): add secure sessions and role-based access control
```

### Phase 8 — Deterministic demo provider

Deliver:

- Demo adapter for the Phase 4 provider ports and capabilities;
- deterministic environments, scopes, resources, relationships, log fixtures,
  metric-sample fixtures, event fixtures, and health transitions;
- synthetic-data marker and visible demo badge;
- separate guest-demo data root, fixture-version marker, mixed-data startup
  rejection, deterministic reset, and fixture-upgrade cleanup;
- reusable provider and browser-test fixtures;
- provider API tests.

Logs, metric samples, and events are fixed provider evidence only. Phases 12,
13, and 14 own their full query, persistence, live-flow, and interface behavior.
Phase 22 owns alert evaluation, history, and delivery from demo evidence.

Commit:

```text
feat(demo): add a deterministic infrastructure demo provider
```

### Phase 9 — Overview dashboard

Deliver:

- dashboard aggregation;
- environment and health summaries;
- source availability and recent infrastructure event summaries;
- restart, usage, and top-consumer summaries;
- loading, empty, and error states;
- responsive English and Persian dashboard;
- component tests.

Commit:

```text
feat(dashboard): add the infrastructure overview experience
```

### Phase 10 — Interactive topology explorer

Deliver:

- topology API and graph normalization;
- typed nodes and evidence-aware edges;
- health, grouping, filters, search, minimap, fit, and full-screen controls;
- neighbor, upstream, and downstream highlights;
- detail panel and mobile controls;
- accessible list fallback;
- RTL and interaction tests.

Commit:

```text
feat(topology): add the interactive service topology explorer
```

### Phase 11 — Resource inventory and detail pages

Deliver:

- resource API with search, filters, pagination, and sorting;
- URL-backed filters and column preferences;
- stable resource routes and detail header;
- overview, health, relationship, and safe metadata tabs;
- deep links and responsive table alternatives;
- English and Persian states and tests.

Commit:

```text
feat(resources): add searchable resource inventory and detail views
```

### Phase 12 — Realtime streaming and infrastructure timeline

Deliver:

- authenticated SSE hub;
- event IDs, keepalive, replay, reconnection, and bounded queues;
- cancellation, client limits, overflow rules, and cleanup;
- unified event API and timeline;
- filters, search, severity, links, and disconnected state;
- lifecycle and leak tests.

Commit:

```text
feat(events): add realtime infrastructure streaming and timeline
```

### Phase 13 — Unified log exploration

Deliver:

- demo log service against the Phase 4 provider port;
- bounded query, download, and live SSE APIs;
- pause, resume, search, severity, timestamp, wrap, copy, and download;
- redaction and safe filename rules;
- multi-container and permission states;
- log page and resource tab;
- buffer, cancellation, and interface tests.

Commit:

```text
feat(logs): add unified live log exploration
```

### Phase 14 — Metrics and historical charts

Deliver:

- demo metric-sample service against the Phase 4 provider port;
- bounded sampling, persistence, and retention cleanup;
- metric API and live SSE;
- CPU, memory, network, restart, and health charts;
- request and limit display;
- accessible table alternatives and unavailable states;
- dashboard and resource integration;
- sampling, retention, and UI tests.

Commit:

```text
feat(metrics): add live and historical resource metrics
```

### Phase 15 — Docker discovery and health

Deliver:

- safe Docker source configuration through operator-mapped targets and
  connection test;
- adapter transport support and validation for operator-mapped restricted
  proxies and verified mutual-TLS targets, plus the loopback-only direct-socket
  development guard;
- supported Moby client module with API version negotiation;
- Engine, Compose, container, service, network, port, image, label, health,
  restart, mount, and limit discovery;
- typed safe metadata and provider status;
- mock-backed tests and separate live-check instructions.

Commit:

```text
feat(docker): add Docker discovery and health integration
```

### Phase 16 — Docker logs, stats, and events

Deliver:

- bounded Docker logs and live tail;
- CPU, memory, and network stat normalization;
- Docker events and reconnect behavior;
- cancellation, permission, and error handling;
- integration with existing log, metric, and event ports;
- mock-backed and optional live tests.

Commit:

```text
feat(docker): add Docker logs stats and event streaming
```

### Phase 17 — Docker topology inference

Deliver:

- Compose grouping and declared dependency relationships;
- explicit `stackplore.io/depends-on` labels;
- inert optional Compose file import inside mounted roots with no include,
  `extends`, build-context, environment-file, config, secret, referenced-file,
  or host-environment resolution;
- network membership, container-to-service, and port metadata relationships;
- provenance, confidence, deduplication, and cycle-safe graph behavior;
- ambiguous and incomplete metadata tests.

Commit:

```text
feat(docker): infer Compose and container network topology
```

### Phase 18 — Kubernetes discovery and health

Deliver:

- in-cluster and operator-mapped mounted kubeconfig source configuration;
- rejection of credential execution, auth-provider plug-ins, proxy settings,
  impersonation, inline credentials, insecure TLS, and outside-root references;
- context selection and connection test;
- namespace, node, workload, pod, service, ingress, endpoint, PVC, ConfigMap
  reference, and Secret reference discovery;
- no Secret object read;
- namespace-scoped default plus explicit read-only cluster-observation profile;
- health, conditions, restarts, requests, and limits;
- permission-degraded states;
- mock-backed tests and version-skew notes.

Commit:

```text
feat(kubernetes): add cluster discovery and workload health
```

### Phase 19 — Kubernetes logs, metrics, and events

Deliver:

- pod logs, container choice, previous logs, and live tail;
- optional read-only `metrics.k8s.io/v1beta1` support and missing-server state;
- Kubernetes events and watches;
- resource-version recovery, cancellation, and permission handling;
- integration with shared live interfaces;
- mock-backed and optional live tests.

Commit:

```text
feat(kubernetes): add logs metrics and event streaming
```

### Phase 20 — Kubernetes topology inference

Deliver:

- workload and pod ownership;
- Service selectors and endpoint relationships;
- Ingress routes;
- PVC, ConfigMap, and safe Secret references;
- explicit dependency annotations;
- provenance, confidence, deduplication, and cycle safety;
- multiple-selector and incomplete-resource tests.

Commit:

```text
feat(kubernetes): infer workload service and ingress topology
```

### Phase 21 — Global search, filtering, and command palette

Deliver:

- global search API across resources and environments;
- keyboard command palette and navigation commands;
- recent items and shortcuts;
- URL-backed filters and saved local preferences;
- highlighting and full empty, error, and keyboard states;
- English and Persian commands;
- tests.

Commit:

```text
feat(search): add global search filtering and command palette
```

### Phase 22 — Alerts and notifications

Deliver:

- alert rule storage, validation, evaluation, and scheduling;
- pending, firing, acknowledged, and resolved transitions;
- history, affected-resource and evidence-time links, APIs, and management
  interface;
- linked notification center and in-application notifications;
- versioned demo alert-rule seed, proof of the required firing alert, and active
  alert-count integration in the overview;
- generic webhook and optional-runtime Matrix adapters;
- shared egress guard for every adapter and operator-created secret references
  for full webhook URLs, Matrix tokens, and signing keys;
- exact operator-mapped Matrix origin with discovery disabled and no
  cross-origin or redirect token forwarding;
- recent authentication for notification configuration and test delivery;
- at-least-once delivery, stable webhook idempotency keys, fixed retry payloads,
  Matrix transaction IDs, uncertain timeout state, and safe logging;
- DNS, address, redirect, secret-canary, SSRF, state transition, retry, and
  permission tests.

Commit:

```text
feat(alerts): add rule evaluation and notification delivery
```

### Phase 23 — Guarded operations and audit history

Deliver:

- provider mutation capability model and global feature flag;
- exact deployment write permission, current administrator authorization,
  CSRF, same-origin, and recent-authentication gates;
- expected resource version, impact preview, and short-lived single-use
  confirmation bound to actor, source, target, action, and parameters;
- idempotency for retryable effects and durable audit intent before provider
  access;
- Docker restart;
- Kubernetes rollout annotation restart and supported scaling;
- success and failure feedback;
- audit records, API, and interface;
- dedicated audit quota, minimum retention, bounded pruning, and prune-summary
  records outside generic cleanup;
- destructive gates for an audit-retention decrease that newly expires rows;
- disabled and demo-prohibited states;
- proof that no arbitrary execution path exists;
- audit-write failure, stale-version, confirmation reuse, idempotency, timeout,
  security, and race tests.

Commit:

```text
feat(actions): add guarded operational controls and audit history
```

### Phase 24 — Onboarding, connections, and settings

Deliver:

- complete first-run flow and demo choice;
- Docker and Kubernetes source forms and tests;
- recent-authenticated source changes, connection status, safe edit, and
  confirmed deletion;
- personal theme and language preferences;
- retention, alert, and notification settings;
- diagnostics and version details;
- recent-authenticated, audited, no-store, size-capped configuration export
  from an explicit safe-field allowlist with masked references and hosts;
- previewed, hard-confirmed, backup-warned, audited cleanup limited to named
  retention-eligible history and barred from users, active sessions, sources,
  settings, and audit;
- the same destructive path for any retention decrease that newly expires
  stored rows;
- recent-authentication negative tests for source, export, and cleanup changes;
- tests.

Commit:

```text
feat(settings): add onboarding connections and application preferences
```

### Phase 25 — Progressive web application and future app boundaries

Deliver:

- manifest, install metadata, and replaceable raster and maskable icons;
- offline application shell and truthful offline state;
- static-only cache rules and logout cleanup;
- hash-based policy for cached shell HTML and network-only handling for
  nonce-bearing or personalized HTML;
- update notification;
- responsive and touch polish;
- platform boundary for a future desktop wrapper;
- practical PWA tests.

Commit:

```text
feat(pwa): add installable responsive web application support
```

### Phase 26 — Accessibility, performance, and resilience

Deliver:

- keyboard, focus, screen-reader, RTL, and reduced-motion audit;
- route-level splitting and query cache review;
- large-topology and large-table performance review;
- error boundaries, retries, and disconnected states;
- memory and SSE cleanup review;
- browser console cleanup;
- local audit where available;
- fixes and regression tests.

Commit:

```text
refactor(web): harden accessibility performance and resilience
```

### Phase 27 — Comprehensive testing

Deliver:

- complete backend unit and integration suites;
- frontend component and feature suites;
- storage and migration tests;
- provider fixture and security tests;
- browser suite for English, Persian RTL, desktop, and mobile;
- full troubleshooting journey from unhealthy overview through resource and
  topology evidence, correlated logs, metrics and events, linked alert, and the
  guarded-action boundary;
- measured coverage reports;
- race test configuration;
- deterministic data and flake review;
- documented commands.

Commit:

```text
test: complete unit integration and end-to-end coverage
```

### Phase 28 — Container images and Docker Compose deployment

Deliver:

- multi-stage minimal non-root production image;
- health checks, persistent data, HTTPS-edge guidance, and production
  environment example;
- isolated demo data and Docker provider Compose profiles;
- restricted Docker socket proxy;
- read-only default and separate optional mutation profile;
- dropped capabilities, no-new-privileges, default seccomp, read-only root, and
  reviewed writable paths;
- local backup and offline restore operations;
- reasonable limits and restart policy;
- one-command demo and Docker observation setup;
- Compose validation, container smoke test, and deployment guide.

Commit:

```text
build: add secure container images and Compose deployment
```

### Phase 29 — Kubernetes deployment and Helm chart

Deliver:

- Helm 4 chart with values, Deployment, Service, optional Ingress, PVC, and
  ConfigMap;
- safe Secret references;
- security context, probes, and resource bounds;
- non-root, no privilege escalation, dropped capabilities, `RuntimeDefault`
  seccomp, read-only root, and conditional service-account token automount;
- exactly one replica, `Recreate` update strategy, and exclusive database
  process-lock validation;
- namespace-scoped read-only service account and RBAC by default;
- separate optional cluster-observation RBAC;
- separate optional mutation RBAC;
- HTTPS public-origin and trusted-edge guidance;
- practical network policy;
- lint, render tests, and install, upgrade, and uninstall guides.

Commit:

```text
build(kubernetes): add Helm deployment and least-privilege RBAC
```

### Phase 30 — Continuous integration, security, and release workflow

Deliver:

- frontend and backend checks;
- formatting, linting, type checks, tests, and builds;
- OpenAPI and locale parity validation;
- Compose and Helm validation;
- secret, dependency, source, and container scans;
- software bill of materials;
- release artifacts and checksums;
- dependency update policy;
- status badges backed by real checks.

Commit:

```text
ci: add quality security and release workflows
```

### Phase 31 — Complete English and Persian documentation

Deliver:

- final equivalent English and Persian README files;
- real screenshots from the final demo;
- architecture diagram and quick start;
- Docker, Kubernetes, configuration, security, alerts, API, development, and
  troubleshooting guides;
- contribution guide, accurate roadmap, feature list, and limitations;
- internal link checks;
- clearly replaceable final placeholder brand assets.

Commit:

```text
docs: publish complete English and Persian documentation
```

### Phase 32 — Final 1.0 quality review and release preparation

Perform independent read-only review of:

- security;
- backend correctness;
- frontend correctness;
- accessibility;
- performance;
- test gaps;
- documentation consistency;
- deployment correctness.

Then:

- fix confirmed findings;
- run the complete verification suite and production builds;
- run the Compose demo smoke test;
- lint and render the Helm chart;
- verify no secrets, unfinished work, or false README claims;
- update `CHANGELOG.md`;
- set version `1.0.0`;
- create the release checklist;
- leave tag and release publication to the maintainer.

Commit:

```text
chore: prepare Stackplore v1.0.0 release
```

## Release acceptance

The final release gate is the definition of done in `SPEC.md`. Phase completion
does not waive a final requirement. Any unavailable live environment remains a
visible release blocker or an explicitly documented validation task; it is
never converted into a passing claim.
