> Historical snapshot. Current state is in [the active evidence plan](../implementation-plan.md).

# Stackplore 1.0 Implementation Plan

Status: active master plan. This plan supersedes the old stop-after-each-phase workflow while retaining completed foundation work and its tests.

## Evidence baseline — 2026-08-21

The repository now has PostgreSQL server/authentication wiring and native maintenance operations, provider-neutral domain ports, deterministic Demo data, Docker and Kubernetes read adapters, bounded realtime feeds, connected product read APIs, protected audit pagination, a durable bounded alert claim/evaluation worker, atomic alert transition persistence, all declared alert predicate families including normalized Kubernetes unavailable-replica evidence, administrator-only audited alert-rule CRUD APIs and a responsive English/Persian management interface, a linked in-application notification center, and an opt-in PostgreSQL-leased generic webhook worker with fixed signed payloads and bounded retries. Its administrator settings surface exposes only safe runtime status and queues durable one-attempt tests through an exact one-use recent-auth grant. Notification target mutation, guarded operations, and target-environment proof remain open.

Unrelated untracked files observed and preserved:

- `code_dumper.py`
- `full_Stackplore_project_code.txt`

Full local gate command:

```powershell
$repoRoot = (Get-Location).Path
$env:PATH = "$repoRoot\.tools\node-v24.18.0-win-x64;$repoRoot\.tools\go\bin;$env:PATH"
$env:CI = 'true'
$env:GOCACHE = "$repoRoot\.tmp\go-build-v1"
npm.cmd run verify
```

Current local result on 2026-08-21: `npm.cmd run verify` passed with 328 locale messages, 181 bounded OpenAPI schemas, 36 API-client tests, 140 web tests, all Go package tests, lint, type checks, and production builds. PostgreSQL integration tests, including the six-migration schema and notification test-result audit paths, were present but skipped because `STACKPLORE_TEST_POSTGRES_URL` was not configured. Docker and Helm executables were absent. This host still does not prove containers, browser journeys, live Docker, live Kubernetes, Helm, webhook receipt, backup/restore, or production infrastructure.

## Delivery rules

Each milestone stays buildable and includes implementation, focused tests, contract updates, English/Persian strings, security review, performance bounds, and current documentation. A milestone exits only on named direct evidence. A later broad gate cannot replace missing real-environment proof.

## Milestone 1 — PostgreSQL primary persistence

State: in progress.

Progress on 2026-08-16: the API server now opens a bounded PostgreSQL pool,
serializes and verifies migrations, seeds and validates installation metadata,
uses the PostgreSQL authentication repository, monitors the full migration
ledger in readiness, and closes the pool during shutdown. The env-gated real
database test covers clean schema creation, idempotent reopen, foreign-schema
rejection, installation seeding, pre-auth consumption and replay rejection, and
filtered user-list SQL. The full local gate passes, but this host has no Docker,
PostgreSQL client, or PostgreSQL server, so the real test has not run here.
Local recovery and backup/restore commands now use PostgreSQL. Native client
tool and disposable restore drills remain open evidence gates.

Dependency order:

1. Accept the PostgreSQL ADR and mark the SQLite ADR superseded.
2. Define database URL, pool, timeout, TLS, migration, readiness, and shutdown configuration.
3. Convert the schema to PostgreSQL types and syntax, with pagination and retention indexes.
4. Implement migration advisory locking and immutable migration-ledger checks.
5. Replace SQLite store, transactions, error mapping, repositories, and auth storage with PostgreSQL adapters while preserving domain ports.
6. Run PostgreSQL-native backup and empty-target restore verification on disposable infrastructure.
7. Add real PostgreSQL migration, repository, auth-race, retention, and readiness tests.
8. Add local Compose PostgreSQL, CI service PostgreSQL, and clean-install proof.
9. Remove SQLite code, dependencies, licenses, config, docs, and claims after parity proof.

Exit evidence:

- a clean PostgreSQL database migrates and starts;
- all repository and authentication tests run against real PostgreSQL;
- concurrent first setup and final-administrator invariants hold;
- readiness fails when PostgreSQL is unavailable and recovers when it returns;
- retention and keyset hot paths have reviewed query plans;
- backup, restore, and restore verification are exercised on disposable data;
- no active SQLite implementation or supported-deployment claim remains.

## Milestone 2 — Deterministic demo and core read APIs

State: in progress. Environments, stable inventory, health, topology, bounded logs, current and historical metrics, event history, Demo alerts, protected audit history, a PostgreSQL-claimed provider-backed alert worker, all declared predicate families, and audited alert-rule CRUD APIs with an administrator interface are wired. Demo fixture depth remains open.

Build a deterministic multi-environment provider with reverse proxy, frontend, API, worker, PostgreSQL, Redis, and monitoring resources. Its fixed timeline tells one degradation story across health, CPU/memory, restart evidence, events, logs, topology, and a firing alert. Add service/use-case layers and versioned paginated endpoints for sources, environments, resources, health, relationships, current and historical metrics, events, logs, alerts, and audit history.

Exit evidence: stable fixture snapshots, provider contract tests, API authorization and bound tests, OpenAPI/client parity, deterministic screenshot states, and end-to-end navigation through the failure story.

## Milestone 3 — Product exploration surfaces

State: in progress. Overview, resource inventory/search, resource detail/health, graph topology with an accessible list, metric charts and tables, logs, events, alerts, global search, audit history, notification center, readiness, and read-only connection status use API calls in both locales. Rich log controls, full inventory/log pagination, onboarding, preferences, and browser/performance proof remain.

Replace planned shells with:

- overview summaries, trends, active failure focus, provider state, alerts, events, and top consumers;
- URL-backed, paginated, sortable, filterable resource inventory and stable detail routes;
- evidence-aware topology with search, filters, grouping, minimap, focus paths, detail panel, and equivalent table view;
- bounded log explorer with live tail, pause, search, severity, copy, download, wrapping, and container selection;
- downsampled metric charts with time ranges, tooltips, unavailable states, and tables;
- correlated event timeline, alerts, notification center, global search, settings, onboarding, and audit views.

Use route-level splitting, list/log virtualization, memoized graph work, bounded chart points, canceled stale requests, coherent motion, reduced-motion behavior, and responsive layouts.

Exit evidence: component, routing, locale, accessibility, topology, large-fixture performance, and browser journey tests in both directions and themes.

## Milestone 4 — Realtime, alerts, and notifications

State: in progress. Bounded authenticated SSE, replay IDs, keepalives, backpressure limits, cleanup tests, stored alert-rule validation, all declared predicates, pending/firing/acknowledged/resolved lifecycle cores, PostgreSQL `SKIP LOCKED` rule claims, provider-backed bounded evaluation, normalized Kubernetes unavailable-replica counts, safe shutdown, signed durable alert and notification reads, atomic alert/transition/in-app-notification writes, administrator-only CSRF-protected alert acknowledgement and alert-rule CRUD with durable audit and an English/Persian interface, a linked notification center, and a hardened PostgreSQL-leased webhook worker with fixed payloads, stable idempotency keys, expired-lease recovery, bounded retry, terminal outcomes exist. The settings API and UI expose secret-free operator-owned status and guarded durable test delivery; each new test links its intent to a terminal success, failure, or uncertain audit result in the delivery transaction. Notification target mutation and live receiver proof remain.

Implement authenticated SSE for infrastructure events, health, metrics, logs, and alert transitions. Give each client a bounded queue, event IDs, reconnect support, keepalive, backpressure disconnect policy, client and lifetime limits, cancellation, and cleanup. Add rule evaluation for the required conditions, pending/firing/acknowledged/resolved lifecycle, in-app delivery, and generic webhooks with stable delivery IDs, bounded retry, safe signing, and complete SSRF controls.

Exit evidence: slow-client isolation, reconnect/resume, cleanup, alert transition, at-least-once delivery, SSRF, redirect, timeout, and secret-redaction tests.

## Milestone 5 — Docker and Kubernetes observation

State: in progress. Docker and Kubernetes inventory, health, topology, logs, metrics, and events have bounded normalized adapters and deterministic tests. Live provider proof and remaining fidelity gaps are open.

Implement Docker Engine and Compose discovery, health, bounded logs, stats, events, and evidence-aware relations. Implement namespace-scoped Kubernetes discovery for supported workloads and resources, conditions, readiness, restarts, events, current/previous logs, Metrics API, and topology. Permission denied and unsupported capabilities remain explicit. Secret object values, exec credential plugins, arbitrary paths, and arbitrary endpoints remain forbidden.

Exit evidence: deterministic SDK tests plus separately labeled optional live Docker and Kubernetes checks. Mock results never count as live integration proof.

## Milestone 6 — Guarded operations and audit

State: in progress. Protected administrator-only audit pagination is wired from PostgreSQL through the API and web view with signed keyset cursors. Guarded restart and scale operations, audit export, and audit retention execution remain open.

Add only reviewed restart and scale operations. Every path enforces the complete gate in `AGENTS.md`, writes durable intent before provider access, handles uncertain timeout, and writes a separate outcome. Add protected audit pagination, export bounds, and audit-specific retention.

Exit evidence: authorization matrix, CSRF, recent-auth, stale target, replay, idempotency, audit-before-call, timeout, provider-denial, and guest-demo prohibition tests.

## Milestone 7 — Deployment and operations

State: in progress. Non-root API/web Dockerfiles, a persistent loopback Compose profile, edge proxy, external-PostgreSQL cluster chart, deployment runbook, and CI gates exist. This host lacks Docker and Helm, so image, Compose, chart, cluster, and restore execution remain unproven.

Add reproducible non-root container images, a complete Compose install with PostgreSQL and durable volumes, liveness/readiness checks, safe secrets, limits, and graceful shutdown. Add Kubernetes manifests and Helm support with external/operator-managed PostgreSQL configuration. Finish migrations, backup/restore, diagnostics, configuration reference, troubleshooting, and upgrade paths.

Exit evidence: clean Compose install, restart persistence, health behavior, migration upgrade, backup/restore drill, image scan, manifest validation, Helm render/lint, and disposable Kubernetes smoke proof.

## Milestone 8 — Release hardening

State: pending on all product milestones.

Measure large inventory, topology, log, metric, API, SQL, and bundle hot paths. Fix high-impact results. Complete WCAG 2.2 AA checks, responsive and RTL review, theme and motion polish, PWA behavior, dependency and secret scans, CI parity, release artifacts, versioning, and accurate English/Persian docs with screenshots from the real demo.

Exit evidence: full local gate, real PostgreSQL integration gate, browser suite, Compose gate, manifest/Helm gate, security gates, measured performance report, and a requirement-by-requirement final audit with no major production TODO.

## Final independent audit

Before release, audit architecture, correctness, security, PostgreSQL and query plans, concurrency, API design, frontend boundaries, accessibility, motion, performance, responsive design, RTL, tests, Docker, Kubernetes, deployment, and documentation. Confirm every definition-of-done item with direct current evidence, fix confirmed gaps, then rerun every applicable gate.
