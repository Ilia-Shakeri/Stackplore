# Changelog

All notable changes to Stackplore will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioned releases will follow [Semantic Versioning](https://semver.org/) once release work begins.

## [Unreleased]

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

### Project status

- The repository contains the Phase 7 local account, session, authorization,
  and user-management flows; the Phase 6 storage foundation; the Phase 5
  service foundation; provider-neutral domain and contract foundations; and a
  runnable bilingual application shell with API, browser-client, web,
  contract, and locale checks. The versioned API has protected authentication,
  user-administration, and system-readiness paths but no product data workflow.
- Product workflows, provider integrations, container images, deployment
  packages, a demo, and a versioned release do not exist yet.
