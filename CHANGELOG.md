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

### Project status

- The repository contains the Phase 5 service foundation, provider-neutral
  domain and contract foundations, a runnable bilingual application shell, and
  API, browser-client, web, contract, and locale checks. The versioned API has
  one deliberately closed system-readiness path and no product data workflow.
- Product workflows, provider integrations, storage, authentication, container
  images, deployment packages, a demo, and a versioned release do not exist yet.
