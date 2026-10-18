# Changelog

## [1.0.1] - 2026-10-18

### Security correction

- Defines the corrective PostgreSQL 17.11 derivative `stackplore-postgres:1.0.1`
  from the same immutable upstream Alpine digest. The first upstream-only
  candidate was rejected after its actual strict scan found one CRITICAL and
  24 HIGH gosu standard-library findings; none is suppressed or ignored.
- Rebuilds unchanged official gosu 1.19 source with Go 1.26.9, retaining the
  upstream PostgreSQL engine and entrypoint. Actual derivative build,
  helper/engine/root-drop probes and complete image scan are still pending.
- One canonical database policy checks Compose and hosted fixtures for drift.
  Release packaging requires the engine/helper probes, full database image scan
  and bound SPDX, plus the database Docker-save archive and seventh SBOM.
  Expected counts are 39 assets, 30 checksums, all 31 provenance subjects and
  seven SBOM attestations; these are pending proof. Compose/CI build owned
  patched fixtures; Helm continues to use external PostgreSQL.
- Server and native maintenance tools require PostgreSQL 17.11 or later within
  major 17. Upgrade guidance distinguishes upstream minor-version advice from
  the application schema and the tested backup/restore path.

### Acceptance

- Corrective candidate. Exact local, browser, PostgreSQL, provider, deployment,
  artifact and recovery results belong to the [1.0.1 receipt](docs/evidence/1.0.1/README.md).
- Pushed `a252718efffbdbb03c8f4552c25e998f481c77bb` passed local gate B and all
  14 browser cases, then trial `39119787671` failed the strict database-image
  scan after the exact pull/engine probe passed. It is rejected and untagged;
  [failed receipt](docs/evidence/1.0.1/failed-database-image.json). Current review
  score remains 8/10; new derivative-tree full gate/runtime/scan/recovery/artifact
  evidence is required.
- The immutable `v1.0.0` checkpoint remains available as source history. Its
  publication was canceled after the database gap was found; no 1.0.0 release
  assets were published. The 0.9.0 prerelease also has the old database default.
  Use the corrected release once its acceptance receipt is complete.

## [1.0.0] - 2026-10-18

> Withheld checkpoint: publication canceled after finding the vulnerable database
> default and missing database-image scan. Superseded by the 1.0.1 correction.

### Release scope

- Completes the nine-stage PostgreSQL-only infrastructure exploration product:
  local accounts and guarded administration, read-only Docker/Kubernetes,
  durable observations/alerts, bilingual exploration, safe PWA, and bounded
  opt-in recovery actions with protected audit.
- Ships the single non-root application image, native maintenance tools, external
  PostgreSQL Helm chart, English/Persian user and operator guides, legal notices,
  checksums, SPDX inventories and signed provenance.
- Final review removes sensitive authentication fixture values from failure
  diagnostics and reconciles stale project, installation and security policy text.

### Verification and support

- Final acceptance requires the exact 1.0.0 local, browser, PostgreSQL, provider,
  Compose/Helm, artifact scan, signature and recovery gates. Results and remaining
  gates are recorded in the [1.0 evidence](docs/evidence/1.0.0/README.md).
- Runtime proof targets Windows amd64, Linux amd64 and the Linux amd64 image.
  Linux arm64 and macOS archives are cross-built only. PostgreSQL 17 is the tested
  database major. Customer deployment acceptance remains environment-specific.

## [0.9.0] - 2026-10-17

### Added

- Public offline shell and install manifest with explicit update acceptance; private API, pages and infrastructure data stay network-only.
- Measured frontend and backend core coverage, additive log field redaction, safe kubeconfig profiles and atomic safe timeline outcomes.
- Bounded streamed log download with an explicit completion receipt; URL-backed environment/severity/text event filters; retained resource health history, metric trends and loaded-subset overview evidence.
- Coalesced inventory, topology and overview refresh with canceled-response fences; atomic once-only source creation audit and real PostgreSQL 14-to-16 forward-upgrade checks.
- Pinned release build, secret/dependency/image scans, native packages, SBOMs, checksums and signed build attestations.

### Fixed

- Deep inventory/alert keysets now use matching bounds/indexes. Measured warm SQL fixtures are recorded with their scale and limits.
- Initial overview source-clock/stream messages no longer duplicate full read batches and exhaust the default request budget.
- PWA install uses raster icons, and immediate acceptance of an installed update handles the browser's waiting-worker transition safely.
- Mounted Kubernetes profiles pass the closed source-settings startup allowlist and full server composition tests.
- Pinned transitive dependency updates remove the verified HIGH/CRITICAL source-scan findings; exact upstream legal bytes ship in web/native/image packages.
- Go is pinned to `1.26.9`; native scans use executable-aware rootfs analysis and reject missing binary or SBOM coverage, rather than accepting an empty successful report.

### Verification

- Release candidate. Local, browser, deployment, scan and exact tagged artifact results have distinct receipts in the [active evidence plan](docs/implementation-plan.md). Publishing requires complete exact-tag checks, scan completeness, checksums and signed provenance/SBOMs.


## [0.7.0] - 2026-10-17

### Added

- One same-origin Go application image serves the UI, API, health, readiness and
  metrics endpoints; browser routes use a bounded static-asset fallback.
- Compose first-run/restart smoke gate and Helm routing to the unified app
  Service, with bilingual install guidance.

### Limits

- Exact main and tag CI passed all six jobs, including actual Compose
  setup/restart and Helm installation against external TLS PostgreSQL. The
  ingress Service route is an object check; a customer's controller, hostname,
  certificate and real HTTPS edge require deployment-specific probes.
  Native restore and upgrade/rollback constraints have separate receipts.

## [0.6.0] - 2026-10-17

### Added

- Purpose-bound recent authentication and durable, idempotent previews and
  execution records for narrowly scoped Docker and Kubernetes operations.
- Administrator settings for bounded audit retention, history cleanup and safe
  configuration export, plus personal locale and number/date preferences.
- PostgreSQL-backed operation and settings records, forward-only migrations,
  guarded provider writers, and matching English/Persian UI and API contracts.

### Limits

- Local full gate passes. Provider writes stay disabled by default; no live
  operator infrastructure or deployment is claimed.
- Exact tagged disposable Docker and Kubernetes writer integration jobs passed.
  Safe mounted-profile expansion and installation/release gates follow in later
  versions; credentials remain operator-managed references.

## [0.5.0] - 2026-10-17

### Added

- Official versioned Docker and Kubernetes clients behind bounded observation
  transports, a separate mTLS Docker observation proxy, explicit Kubernetes
  namespace authority and separately enabled cluster observation.
- Closed safe container, condition, port, mount-target, label and resource-limit
  metadata with strict storage, contract and browser validation.
- Atomic partition coverage and bounded independent retirement. Optional denied
  kinds preserve other observations; changed operator authority closes old cache
  until fresh coverage publishes. Projected Kubernetes credentials refresh safely.
- Versioned nine-resource/two-environment deterministic fixture with stable facts,
  controlled scenario steps and initial synthetic metric history.
- Explicit isolated deployment/database/directory modes and public read-only guest
  viewing, without manufactured accounts or sessions. Synthetic alert evidence
  persists through the normal engine; outbound configuration and writes are denied.
- Provider, guest, lifecycle and deployment guides in English and Persian, plus
  required real disposable Engine/cluster test jobs.

### Fixed

- Docker CPU uses a prior sample; missing/reset evidence is unavailable. Event
  history is partial unless its unfiltered retained ring proves the requested range.
- Kubernetes optional-kind permissions, previous/container log identity, stable
  bounded log pagination, byte budgets, UID recreation and rolling replica surges.
- Resource identity/detail reads survive unavailable optional health observations.
- Ordinary same-fixture restarts preserve administrator choices; isolated resets
  and fixture upgrades preserve accounts and protected audit history.
- Omitted environments cannot disclose old names through paginated cached reads.
  Current metrics respect their independent collection clock, preserve the last
  valid retained point and label observations older than five minutes as stale.
- Restricted Docker observation profiles pass source-settings startup validation
  and use the same guarded enable/disable flow as other reviewed profiles.

### Limits

- This is a development checkpoint. Stages 6–9 retain guarded actions/settings,
  installation, measured hardening and final release requirements.
- Historical live-provider logs/events remain bounded and explicitly partial
  where upstream retention, permissions or request limits cannot prove coverage.
- Local, browser, real-provider and remote evidence are recorded separately in
  the active plan; test definitions alone do not prove a target deployment.


## [0.4.0] - 2026-10-17

### Added

- Paginated exploration with URL-backed inventory, time, log and topology filters;
  bounded loaded-line search, selection, copy and download, previous-container
  targeting and pause/resume. Source counts use a complete PostgreSQL aggregate.
- Relationship evidence, correct directions, grouping and keyboard graph controls,
  paginated accessible node/edge lists and independent timestamp-based metric series.
- Activity-based session checks use the actual server deadline, bounded requests
  and stale-response fences. Product pages and runtime transport guards are split
  by feature; all production text has English and Persian parity.
- Bilingual exploration guides and current stage evidence.

### Fixed

- Development proxy preserves browser authority and uses the web public origin.
- Source-wide topology and events no longer inherit a first-inventory-page filter;
  empty target reads preserve availability, and bounded selections report partial.
- Health filtering respects denied/partial observations and source capabilities.
- Exact time windows remain stable through pagination; demonstration clock anchors
  include observations at their exclusive upper boundary.
- Rule edits retain event targets and accept fractional thresholds.

### Limits

- This development checkpoint does not complete 1.0. Stages 5–9 retain provider,
  guarded action/settings, installation, performance and final release requirements.
- Log download includes only the loaded, redacted lines (at most 1,000). Graphs
  show at most 200 nodes, with paginated alternatives and explicit loaded-edge bounds.
- Real provider/container/cluster and final assistive-technology acceptance remain
  separate from local browser/DOM and PostgreSQL evidence.


## [0.3.0] - 2026-10-16

### Added

- Covered CPU/memory and unavailable-replica time-weighted windows, observed
  restart increments, complete scoped event evaluation and raw source-outcome
  evidence. Unknown/stale/partial/future evidence never fabricates clear or fire.
- Fenced fresh evaluation claims, continuous pending windows, new episode IDs,
  atomic rule-edit episode closure, and durable idempotent synthetic demo seeding.
- Committed source-scoped alert change replay across restarts, bounded SSE
  snapshots/change metadata, explicit backpressure/reset and browser refetch.
- Optional Matrix `m.notice` delivery through the same guarded HTTPS boundary,
  independent adapter/reference outbox lanes, stable retry bodies and transaction
  IDs, secret-free bilingual status and protected per-lane test delivery.
- Forward migrations 9–10, bounded alert-evidence retention, real PostgreSQL and
  disposable HTTPS regressions, deployment Secret inputs and bilingual operator
  references. The first eight released migrations remain unchanged.

### Limits

This is a development checkpoint; current proof is recorded in the evidence
plan. Full browser, real provider, container/cluster, live operator receiver,
production scale and remaining roadmap gates stay open. Matrix deduplication
assumes stable access-token/device context and transaction ID; timeout can leave
an uncertain accepted message. Source is immutable when replacing a rule; use
a new rule to move source. The replacement repository instruction prohibits push,
so local version/commit/tag does not imply new remote CI or image publication.

## [0.2.0] - 2026-10-13

### Added

- Persistent operator-owned source registry and typed PostgreSQL inventory,
  topology, health, replica, metric and event capture/read ports; migrations 7–8
  extend the immutable six-migration 0.1 baseline.
- Bounded collection jobs, capped provider-neutral aggregate capture, versioned
  generation reconciliation, reconnect backoff and cached reads. Logs remain live
  and bounded; provider bodies and credential values are not persisted.
- Administrator source toggle UI in English/Persian with exact preview,
  two-minute one-use confirmation, current-password `source.update` grant,
  version conflict checks and atomic configuration/grant/confirmation/audit.
- Fair bounded history cleanup, newest-metric protection, unchanged-health
  deduplication, synthetic fixture protection and a separate retain-all audit
  lifecycle. Audit quota/expiry controls remain planned.
- Signed query-bound keysets carrying immutable sort values, a 4,096-byte
  infrastructure cursor ceiling and separate 1,024-byte account cursor input.
- Real PostgreSQL capture/restart/upgrade/keyset/authority/retention regressions,
  reviewed disposable query plans and bilingual observation documentation.

### Corrected

- Cached reads preserve stale/partial/denied/unsupported status by source and
  optional port; an unfinished capture cannot claim a complete fresh estate.
- Repeated event revisions preserve immutable facts and first stored observation
  time. Event text uses the shared credential scrubber.
- Source authorization time is refreshed after request decoding and after the
  PostgreSQL auth lock, preventing an old prepared time from extending expiry.

### Limits

This is a development checkpoint. Namespace-scoped/partial-rights Kubernetes,
restricted production Docker access, operator audit quotas, complete historical
alert semantics and durable alert streams, live browser/provider/container/cluster
proof and the remaining roadmap stages are open. Demo alarm compatibility remains
an explicitly static fixture until the durable alert stage. Exact checks and
remote outcomes are recorded in the active evidence plan; no production claim is
made by source versioning.

Remote follow-up: main and `v0.2.0` workflows passed for implementation
`b8448bf`, including the complete real-PostgreSQL gate, Linux race tests,
image builds, Compose model and Helm gates. This proves the named checks;
container startup, cluster install and live provider/browser proof remain open.

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
