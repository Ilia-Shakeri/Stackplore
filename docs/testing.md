# Testing and evidence

Use the pinned tools in [development](development.md). `npm.cmd run verify`
checks format, documentation links/inventory, release metadata, locale parity,
OpenAPI policy/derived types, lint, TypeScript, local tests with their measured
coverage gates, legal-copy checks and builds. Browser, provider, installation and
release workflows have additional commands and real-runtime gates.
Run the smallest regression for a change, then the full gate for a milestone.
Do not weaken tests or hide skips to claim completion.

Current code is corrective 1.0.1, with all nine stages functionally implemented
and a whole-release review score of 8/10. Corrective full local/browser, real
PostgreSQL/native restore, hosted-deployment and strict scan results remain
pending until recorded in the [active plan](implementation-plan.md) and
[acceptance checklist](evidence/1.0.1/README.md). A running check has no passing
result yet. Immutable 1.0.0 passed its recorded main/tag/trial/recovery checks,
but its publication was canceled after finding the vulnerable database default
and missing database-image scan; the release API confirmed no release.
The published 0.9.0 prerelease has the unsafe default and is superseded.
[Withheld receipt](evidence/1.0.0/publication-withheld.json).
Historical receipts below describe their own revisions. Project acceptance and
a customer's production deployment are separate; these tests do not deploy it.

| Boundary                                   | Required check                                              | Proof limitation                                                                        |
| ------------------------------------------ | ----------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| Domain/auth/provider/service/HTTP          | `npm.cmd run test:api`                                      | Most provider tests use fixtures.                                                       |
| Browser client                             | `npm.cmd run test:client`                                   | Network mocks do not prove a deployment.                                                |
| React/locale/interface flows               | `npm.cmd run test:web`                                      | DOM tests do not prove geometry, contrast or browsers.                                  |
| HTTP model/client drift                    | `npm.cmd run check:contracts`                               | Does not alone compare every runtime parser bound.                                      |
| English/Persian key parity                 | `npm.cmd run check:locales`                                 | Natural language quality needs review.                                                  |
| Documents/version metadata                 | `npm.cmd run check:docs`, `npm.cmd run check:version`       | Link/file checks do not certify factual accuracy.                                       |
| PostgreSQL SQL/migrations/auth concurrency | Test URL plus `npm.cmd run test:api`                        | Must run against a disposable real server.                                              |
| PostgreSQL image identity and scan | Exact engine/image metadata, OS vulnerability report and SPDX inventory | Application-image scans and manifest pins do not prove the separate database image. |
| Native archive/restore                     | `cmd/storage` on separate fresh databases                   | Successful archive listing alone is not a restore.                                      |
| Compose/model/images                       | Compose config, build, start, probes and restart            | Definitions or image build alone do not prove startup.                                  |
| Helm/cluster                               | Lint, render, install, probes, RBAC and TLS                 | Rendering alone does not prove a pod runs.                                              |
| Webhook/Matrix                             | Persisted retries against a disposable HTTPS receiver       | Local receipt does not prove an operator receiver, public DNS or a complete homeserver. |
| Browser/accessibility/performance          | Critical journeys in both locales/themes at required widths | Each needs saved current evidence.                                                      |

## Real database

The test role needs schema creation and the permissions used by concurrency
regressions. Use a disposable database with no user data. Tests create unique
schemas and remove only their own fixtures. Never reuse a live application URL.

Use PostgreSQL server and native `psql`, `pg_dump` and `pg_restore` clients at
17.11 or a later 17.x patch. The [canonical image policy](../deploy/postgres-image.json)
defines the approved digest. Corrective proof needs real database execution,
readiness/ledger checks and native backup/empty-target restore, plus exact
database engine/digest metadata, OS scan and matching SPDX package inventory.
Retain observed tool/engine versions and identity in each receipt. The
[existing-installation minor-update guidance](operations/upgrade-rollback.md#postgresql-17-patch-baseline)
follows upstream; operators validate their own backup/restore and maintenance.
No specific 17.6-to-17.11 runtime experiment is claimed or substituted for that
deployment-specific work.

```powershell
$env:STACKPLORE_TEST_POSTGRES_URL = 'postgresql://test-role@127.0.0.1:5432/stackplore_test?sslmode=disable'
npm.cmd run test:api
```

## Historical Stage 6 guarded operations and settings

The full Windows gate passed on the changed tree with pinned Node 24.18.0, Go,
and a disposable PostgreSQL 17 cluster. It includes real migration, repository,
and audit-lifecycle transaction tests, API/web type checks, 36 API-client tests,
402 web tests, and production builds. The audit-specific test proves expiry is
disabled without an archive cutoff, then removes one eligible old row under a
one-row batch and confirms the stored cutoff. This local gate does not prove
real Docker or Kubernetes writes; CI has separate opt-in disposable Engine and
kind jobs for those effects.

Without the URL, environment-gated SQL and HTTP integration tests skip. Report
that skip. The [evidence plan](implementation-plan.md) records exact versions,
commands, failures, fixes and proof levels for each delivered version.

The 0.2 observation suite includes real forward upgrades, restart persistence,
generation/version races, coherent logical read snapshots, immutable signed
keysets, source/port permission truth and protected fair retention. Focused SQL
plan output can be reproduced with the same disposable test URL:

```powershell
go -C apps/api test ./internal/postgres -run TestObservationHotPathQueryPlansOnRealPostgreSQL -count=1 -v
```

The fixture uses synthetic and invented native data; warm-cache SQL timings
are not end-to-end API, browser, production or capacity proof.

## Verified local alert and delivery checkpoint

The 0.3.0 targeted suites cover gauge/replica weighted windows, positive restart
increments and resets, missing/future/partial coverage, raw connection outcomes,
scoped event history, continuous pending periods, new episodes and unknown-state
preservation. Real PostgreSQL regressions cover source configuration fences,
database-clock lease expiry after lock contention, acknowledgement/edit races,
atomic state/audit writes, source-scoped durable replay, retention watermarks and
legacy eight-/nine-version forward upgrades.

Focused delivery proof uses real PostgreSQL with an actual disposable TLS
receiver. Resolver/dial seams direct checked fixture addresses to that private
receiver; this does not contact public DNS or a live Matrix homeserver. Tests
verify persisted stable IDs/bytes across retries, independent adapter/reference
lanes, webhook HMAC/idempotency headers, Matrix PUT transaction IDs and plain
notices, TLS host verification, redirects, DNS rebinding, special-address policy,
response caps/timeouts, lease recovery, synthetic exclusion and one-attempt test
intent/result audit. Missing SQL configuration still skips the database portion.

```powershell
go -C apps/api test ./internal/alerting ./internal/notification ./internal/config -count=1
go -C apps/api test ./internal/postgres -run 'TestNotificationAdapter|TestNotificationCompletion|TestPersistedAdapterRetries|TestMatrixTestIntent' -count=1 -v
go -C apps/api test ./internal/postgres -run TestAlertHistoryHotPathQueryPlansOnRealPostgreSQL -count=1 -v
```

These regressions supplement the full local gate. The final 0.3.0 local gate with
real PostgreSQL, disposable native restart/restore and replay drill, HTTPS receiver
checks, and Helm lint/render passed. Exact commands and outcomes are in the active
evidence plan. Later exploration and provider receipts are recorded below;
complete Matrix homeserver and target installation proof remain separate. Under renewed authorization on
2026-10-17, exact unchanged 0.3.0 main/tag CI passed code/race/image-build/chart
jobs. Older tagged results do not certify the 1.0.1 candidate; current results
are recorded in the evidence plan.

## Historical provider and isolated-demo checkpoint

The `0.5.0` checkpoint has scoped Docker/Kubernetes, PostgreSQL and guest-browser
receipts. Final complete changed-tree gate D passed with real PostgreSQL,
36 client tests, 350 web tests and both builds. Exact publication/CI identities
are retained in the active plan; do not report early failed full gates as successful. The active
plan records the 8.5/10 findings, fixes and final independent scoped 9/10 review.
Those counts are historical and are not current candidate acceptance.

With a disposable real PostgreSQL URL, reproduce the narrow authority and clock
checks after preparing the pinned tools:

```powershell
go -C apps/api test ./internal/postgres -run 'TestEnvironmentNamesRequireCurrentPartitionAuthorityBeforePagination|TestInventoryPartitions|TestOperatorAuthorityFence|TestNormalFixtureUpgrade|TestSyntheticWorkerKeepsScenarioClock' -count=1 -v
go -C apps/api test ./internal/provider/kubernetes -run 'TestSDKMetrics|TestSDKMixedPost|TestProviderRejectsUnboundUsage|TestProviderKnownUsageDenial|TestProviderMissingUsage|TestSDKLogReadRequiresSamePodUID' -count=1 -v
go -C apps/api test ./internal/provider/demo ./internal/observation ./internal/domain -count=1
npm.cmd run verify
```

SQL regressions cover published environment/kind/scope authority before
pagination, empty complete lists, denied/omitted partitions, policy narrowing,
protected custom rules/disabled-source choices and separate synthetic/operational
clocks. Wire tests cover stable Pod UID before/after log and metric reads, usage
windows wholly inside Pod lifetime, independent bounds, any-usage-denial fencing
and stale mixed results when CPU/memory is missing. These mocks do not replace
real provider proof.

The [Docker guide](docker-provider.md) gives the opt-in disposable restricted-mTLS
Engine test and safety guards. The [Kubernetes guide](kubernetes-provider.md)
gives isolated kind setup, missing-API and positive-usage tests, image checksums
and exact final test-binary hash. Local proof covers named Engine/Kubernetes
versions and separate namespace profiles; it is not a Stackplore Compose/chart
installation or a general compatibility claim. Kubelet TLS relaxation belongs
only to the disposable metrics fixture; the observation adapter still verifies
the API server CA.

The [guest matrix](evidence/0.5.0/browser-matrix.json) records 24 loaded route
states in both locales across 360px light and 1280px dark profiles. The
[runtime receipt](evidence/0.5.0/guest-runtime.json) records restart reset, fixed
fixture clock, zero manufactured users/sessions/grants, zero deliveries and
server-side private/write route denial. It does not claim every viewport/theme
combination, native screen-reader execution or target installation acceptance.
Native restore receipts and earlier focused static/web checks remain distinct
from the final complete gate D, which passed after the latest corrections.

## Coverage and performance

The 0.4 exploration regressions add fixed full-set inventory filtering and cursor
binding, complete source counts beyond transport limits, capability-aware health,
stable paired UTC ranges, safe current/previous container targets, all-source
topology/events and truthful empty reads. Run the real PostgreSQL cases with
the disposable URL before claiming SQL behavior:

```powershell
go -C apps/api test ./internal/postgres -run TestInventory -count=1 -v
go -C apps/api test ./internal/httpapi -count=1
npm.cmd run test:web
npm.cmd run verify
```

New web checks cover late/cancelled page responses, replayed cursors, the 1,000-item
bound, URL history and compound filters, log pause/range/export, graph 200-node
bound and accessible pagination/evidence, timestamp-separated metric series,
fractional rules and activity-driven server-confirmed session extension.
The [retained 0.4 browser matrix](evidence/0.4.0/README.md) records 120 loaded
product states, 40 authentication form states and 20 final topology rechecks.
Actual keyboard and accessibility-tree checks prove semantic behavior; no
Narrator/NVDA process execution is claimed. The final evidence plan distinguishes
component/DOM checks, actual browser journeys and broader release acceptance. Do not replace a
missing browser or deployment result with the presence of a test definition.

The specification requires measured meaningful coverage; do not estimate it from
test counts. `test:api` measures statement coverage of `internal/domain` and
`internal/alerting` and requires at least 80% in each. `test:web` collects feature
coverage and requires at least 70% statements, branches, functions and lines.
These are defined gates; record actual changed-tree measurements before calling
them passed. Race tests need a supported Go race toolchain. SQL hot
paths require representative data and `EXPLAIN (ANALYZE, BUFFERS)` before a
performance claim. Browser measurements require actual rendering and large
fixtures; bundle byte counts alone are insufficient.

## Current candidate regressions and runtime gates

Focused cases cover exact-resource health history (200 samples, 30-day lookback,
current source authority), server event filters before pagination and cursor
binding, atomic once-only source creation audit under capacity failure, mounted
kubeconfig profile migration/fencing, and representative keyset plans at 1,000
and 20,000 rows. With the disposable real PostgreSQL URL, run:

```powershell
go -C apps/api test ./internal/postgres -run 'TestHealthHistoryReadsRetainedExactResourceAndClosesRevokedAuthority|TestEventFiltersApplyBeforePaginationAndBindCursor|TestOperatorSourceCreationIsAuditedOnceAndAtomically|TestMountedKubernetesProfileMigrationAndFencePreserveAdministratorChoice|TestReleaseRepresentativeKeysetPlansOnRealPostgreSQL' -count=1 -v
npm.cmd run test:release
npm.cmd run check:go-notices
npm.cmd run check:web-notices
```

Read saved `EXPLAIN (ANALYZE, BUFFERS)` work, index names, row counts and timing
conditions before making a performance claim. Warm fixture latency does not
certify hardware-independent throughput. Migration 15 adds the mounted profile;
migration 16 adds the source alert feed's matching keyset index. Tests must prove
their forward path without changing any earlier migration bytes.

Log download tests cover fixed filters across pages, additive redaction/export
policy, 10-second/1-MiB/1,000-output/10,000-scan/100-page limits, malformed or
incomplete streams, 401/403 handling, cancellation and late-response suppression.
PWA tests assert the four-public-file allowlist, no API/auth caching, public-only
offline fallback and explicit update activation. Live-view tests check five-second
coalescing and preservation of loaded page depth. Local DOM checks supplement
real browser journeys; they do not replace them.

Mixed event timeline checks retain the permission boundary: denied native
`EventHistory` means a denied empty feed even when safe operation/alert facts
exist. A separate system subfeed is not part of the current HTTP contract.

Build the embedded production UI, then run `npm.cmd run test:browser` with
`SP_BROWSER_ADMIN_URL` loaded privately for a disposable loopback PostgreSQL
administrator. The harness creates/removes only its own databases and starts
isolated guest, normal English and normal Persian application processes. It
needs `psql` and pinned browser tooling; `SP_BROWSER_EXECUTABLE` may select a
trusted installed browser. Never use a customer's database or publish that URL.
Record actual screenshots, RTL/LTR, themes, widths, keyboard, accessibility and
network/offline measurements. Do not label unexecuted scenarios passed.

The native validation matrix targets Windows amd64 and Linux amd64 and the image
matrix Linux amd64. Each artifact requires its own startup/readiness/auth/read
and storage probes. macOS/arm64 cross-build success is extra packaging evidence,
not native compatibility proof. Compose fresh/restart and Helm TLS installation
are separate CI lanes. Packaging, strict dependency/native/image/secret scans,
SPDX, checksums, signatures and independent consumer verification follow
[release operations](operations/release.md) ([فارسی](operations/release.fa.md)).

## Safety and review

Fixtures use invented values. Logs and stored evidence contain no live secrets.
Restore/action tests target disposable resources only. Auth tests may generate
opaque values internally but never print them. Inspect the complete tagged diff
and current documentation together before claiming a release gate passed.
