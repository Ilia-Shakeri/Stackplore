# Changelog

All notable changes to Stackplore will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioned releases will follow [Semantic Versioning](https://semver.org/) once release work begins.

## [Unreleased]

No later roadmap stage is complete.

## [0.1.0] - 2026-10-12

### Added

- Reviewed PostgreSQL-backed read-only baseline with secure local auth, Demo/Docker/Kubernetes adapters, product APIs/pages, bounded SSE, rule management, audit/notification reads and leased webhook delivery. Much of this implementation existed uncommitted at review entry and is integrated after verification.
- Real PostgreSQL auth/product repository parity, concurrency, ownership, migration rollback/drift, outbox, retention and HTTP tests; native backup/restore and session revocation proof on disposable local data.
- Nine-stage completion roadmap, codebase audit, current English/Persian user guides and roadmap, document index, configuration/testing/troubleshooting/upgrade/release guides; document-link and version gates.
- Versioned metadata at 0.1.0, pinned workflow action revisions and real Helm default/setup/cluster/annotation render checks.

### Fixed

- Final-administrator mutation race, migration-before-ownership, ownership loss, query/transaction bounds, undersized pools, outbox commit ordering, digest-key close races and migration-ledger read bounds.
- Invalid read input returning empty success, provider denied/outage mapping, metric reply bounds/completeness, source summary health/count truth and exact resource lookup beyond the first page.
- Bearer/structured secret log masking, repeated Kubernetes container names and per-delivery idle HTTP connection cleanup.
- Mutable frontend source races, direct resource links, URL source/search history, missing metric totals, runtime guards, predicate translations and technical log direction.
- Fresh-install setup environment/Secret gates, Nginx writable pod mounts, chart annotation YAML and forwarded HTTPS, separate edge liveness/readiness and Compose API filesystem/resource limits.
- Native jobs15-minute bound, restore ownership monitoring/public schema scope, archive sync and atomic no-overwrite publication.

### Removed

- Legacy SQLite runtime/migrations/dependencies/licenses and busy-timeout configuration after real PostgreSQL parity and native restore checks. Private runtime key/path protection remains in a focused package. Old development data is not converted automatically.

### Known limits

- This is a development checkpoint. The full 1.0 product is not complete and has no production support promise.
- Persistent discovery/source workflows, correct historical alert evidence/durable alert streams, complete paging/detail/log/topology journeys, namespace fidelity, full demo/guest isolation, guarded actions/settings/cleanup/PWA and release artifacts remain in the roadmap.
- Accepted built-web serving and vendor-client architecture need alignment before 1.0.
- Local PostgreSQL17.11 and Helm3.17.3 proof does not prove containers, browser accessibility/performance, a live provider/receiver, a cluster or a production restore. Linux race/remote CI outcomes are recorded separately in the evidence plan.

Remote follow-up: both main/tag workflows passed for the implementation revision,
including Linux race tests, real PostgreSQL, image builds, Compose model and
chart gates. This does not prove container startup or a cluster install.

## [0.0.0] - Historical foundation record

The entries below preserve the earlier phase snapshot; retired storage and planned-product descriptions are historical, not current support claims.

### Added

- Initial product specification, project guide, and phased delivery plan
- Initial architecture, security, and architecture-decision documentation
- English and Persian project overview documents
- Contribution, conduct, and security policies
- Issue forms and a phase-scoped pull request template
- pnpm workspace with exact Node.js, Corepack, pnpm, and frontend tool pins
- Strict TypeScript and React smoke screen with lint, test, and build checks
- Go API scaffold with a loopback-only listener and terse `/healthz` endpoint
- Root Make, PowerShell, POSIX shell, format, lint, type, test, and build tasks
- Reproducible development bootstrap and environment documentation
- Semantic interface tokens with light, dark, and system theme behavior
- Bundled Inter and Vazirmatn typography with shipped font license notices
- Original replaceable logo, favicon, PWA icon, wordmark, and social preview assets
- Project-owned accessible interface wrappers and an application error boundary
- Responsive route shell with desktop sidebar, top bar, mobile drawer, and
  component showcase
- Paired feature-owned English and Persian catalogs with exact parity,
  placeholder, and plural-form checks
- Persisted runtime language switching with pre-paint document language and
  direction resolution
- Locale-aware number, Persian-calendar date, and plural formatting foundations
- Direction-aware shared primitives, technical-text isolation, and complete
  bilingual shell copy
- Provider-neutral Go domain contracts with stable versioned identifiers,
  bounded validation, typed provider outcomes, and focused provider ports
- Canonical OpenAPI 3.1 schema foundation under `/api/v1` with sealed
  transport models and paths added only with tested runtime handlers
- Contract-derived TypeScript transport types plus a small same-origin browser
  client with safe problem parsing and request-ID propagation
- Reproducible contract policy, derived-type drift, client test, and root
  verification tasks
- Strict startup configuration with canonical origin, listener, trusted-proxy,
  metrics-network, logging, timeout, size, concurrency, and rate-limit bounds
- Structured redacting process logs, safe request IDs and problems, fixed
  security headers, panic containment, and bounded buffered responses
- Terse `/healthz` and `/readyz` probes, origin-and-network-guarded
  low-cardinality `/metrics`, and a deliberately closed
  `/api/v1/system/readiness` route
- Accepted-connection and in-flight bounds, per-client rate limiting for
  non-operational requests, request deadlines, and signal-driven graceful
  shutdown with a forced-stop ceiling
- The first implemented OpenAPI path and reproducibly derived browser error
  types for its closed Phase 5 behavior
- Pure-Go SQLite storage with WAL, foreign keys, a bounded busy timeout,
  exclusive process ownership, checked embedded migrations, and database-backed
  readiness
- Strict indexed schema for installation, identity, source, observation,
  alert, notification, and audit state plus bounded non-audit retention helpers
- Domain-owned metric, event, and audit repository ports with atomic bounded
  SQLite adapters and transaction handling
- Local operator command for live SQLite online backup and offline,
  integrity-checked restore with schema upgrade and restored-session revocation
- Fail-closed restore markers, maintenance locking, private local-path checks,
  second-process coverage, and a platform-aware backup and restore procedure
- Atomic first-run administrator setup with permanent closure and a short-lived,
  one-use operator proof when setup is exposed beyond loopback
- Versioned Argon2id credentials, non-enumerating durable login throttles,
  opaque server-side sessions, exact-origin CSRF checks, expiry, rotation, and
  revocation
- Administrator and viewer authorization, one-use recent-auth grants,
  final-administrator protection, password rotation and reset, and audited
  local account recovery
- Canonical authentication and user-administration HTTP contracts, derived
  browser types, setup and login screens, session-aware navigation, and a
  responsive bilingual user-management interface

### Historical project status

- The repository contains the Phase 7 local account, session, authorization,
  and user-management flows; the Phase 6 storage foundation; the Phase 5
  service foundation; provider-neutral domain and contract foundations; and a
  runnable bilingual application shell with API, browser-client, web,
  contract, and locale checks. The versioned API has protected authentication,
  user-administration, and system-readiness paths but no product data workflow.
- Product workflows, provider integrations, container images, deployment
  packages, a demo, and a versioned release do not exist yet.
