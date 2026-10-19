# Testing and evidence

Use the pinned tools in [development](development.md). `npm.cmd run verify`
checks format, documentation links/inventory, release metadata, locale parity,
OpenAPI policy/derived types, lint, TypeScript, local tests with their measured
coverage gates, legal-copy checks and builds. Browser, provider, installation and
release workflows have additional commands and real-runtime gates.
Run the smallest regression for a change, then the full gate for a milestone.
Do not weaken tests or hide skips to claim completion.

Corrective **1.0.3** is published and independently accepted, schema **17**, immutable `v1.0.3` at `656581cf05d4774358d5644a6e1af3c69a210f81`. [Public acceptance](evidence/1.0.3/published-release.json) binds the exact source and artifacts. Customer installation, provider rights, receiver delivery and capacity require separate operator proof. Historical 1.0.2 or 1.0.1 results do not certify this source.

The public-worker update test's maintainer follow-up treats its extra script GET
as a bounded control read: status/MIME, at most 64 KiB and one marker are checked;
only 429 may retry within four attempts/15 seconds using bounded `Retry-After`.
Safe status/count/size metadata is attached. The artifact uploader includes hidden
reports only through its explicit sanitized filename whitelist. Real page/API
errors stay failures; no product rate is raised. This later main-only follow-up
is separate from immutable release `376ed03`; the
[diagnostic](evidence/1.0.2/browser-fixture-diagnostic.json) retains both the failed
first tag attempt and separate worker/reload-logo 429 observations.

The load test's unmeasured setup has four additional 2500-ms refills between
validated route/query setup and the next route/search. Initial/preload refill
remains; setup is excluded from measured read timing. The workload stays exactly
80 reads, eight parallel, ten rounds and minimum 600-ms spacing, with all status/
envelope assertions unchanged and no measured-read retry. Fixed public-document
status diagnostics distinguish HTML navigation from setup API failures without
recording bodies. Maintainer CI `39138645362` on `7545339` failed before load
with a recorded setup API 429; the document status was unknown. The exact revised
callback later passed against the signed `376ed03` runtime: 86 setup responses
and 80 measured reads were all 200, with valid envelopes, ten in-memory diagnostic
checks and owned cleanup. This local focused proof does not replace full browser/
six-job CI on the containing review commit. [Evidence](evidence/1.0.2/browser-fixture-diagnostic.json).

The historical accepted corrective release is **1.0.2**, immutable `v1.0.2` at
`376ed0379900ab870dd8eb8af6b55ef3e5ec389b`, schema **17**. All **nine stages**
are complete; architecture/code and release review score **9/10**. Full local D,
all six exact-source main/tag jobs, signed trial/publication and independent
public checksum/provenance/SBOM/archive checks pass. Failed first tag/browser
and blocked-release attempts retain their outcomes; unchanged-tag retry passed.
[Published proof and runtime limits](evidence/1.0.2/published-release.json).

The immutable 1.0.1 source and its [published receipt](evidence/1.0.1/published-release.json)
retain their original results and scope. Those older checks do not certify a different source. Customer production and air-gapped installation require separate
operator evidence.

Historical 1.0.1 evidence only: Full local D passed 28 release/policy, 40 client and 453 web tests with real
PostgreSQL 17.11; fresh browser B passed all 14 cases. Web coverage is 81.61%
statements, 77.8% branches, 85.48% functions and 83.11% lines. The final eight
public vulnerability reports each have zero HIGH/CRITICAL findings; full-history
secret scan has zero findings. Signed Linux amd64 guest/normal smoke passed.
The public Windows archive equals the recovery-tested trial. Because the public
database archive differs from trial, it was independently loaded and executed;
identity, PostgreSQL 17.11, gosu 1.19/Go 1.26.9, UID/GID 70 root-drop,
read-only/network-none/non-root SQL, license and owned cleanup passed.
Historical failed candidates remain failed; project checks do not prove a
customer rollout or a full air-gapped installation.

Retained 1.0.2 correction coverage includes generated-marker URL/userinfo and clipped
text, real HTTP 405 Allow sets, later-page event denial, actual non-byte database
collation, ledger 14/16-to-17 upgrades and native percent/equals/quote/backslash
database names. Interface regressions cover all seven log-severity deep links,
English/Persian labels, deleted-rule drafts, node facts and role-aware keyboard
navigation. Focused checks do not replace the full gate or fresh production
browser; exact commands and failed attempts are in the
[fresh audit](evidence/1.0.2/fresh-audit.md).

| Boundary                                   | Required check                                                                                                            | Proof limitation                                                                                           |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| Domain/auth/provider/service/HTTP          | `npm.cmd run test:api`                                                                                                    | Most provider tests use fixtures.                                                                          |
| Browser client                             | `npm.cmd run test:client`                                                                                                 | Network mocks do not prove a deployment.                                                                   |
| React/locale/interface flows               | `npm.cmd run test:web`                                                                                                    | DOM tests do not prove geometry, contrast or browsers.                                                     |
| HTTP model/client drift                    | `npm.cmd run check:contracts`                                                                                             | Does not alone compare every runtime parser bound.                                                         |
| English/Persian key parity                 | `npm.cmd run check:locales`                                                                                               | Natural language quality needs review.                                                                     |
| Documents/version metadata                 | `npm.cmd run check:docs`, `npm.cmd run check:version`                                                                     | Link/file checks do not certify factual accuracy.                                                          |
| PostgreSQL SQL/migrations/auth concurrency | Test URL plus `npm.cmd run test:api`                                                                                      | Must run against a disposable real server.                                                                 |
| PostgreSQL derivative identity and scan    | Exact base/derivative and engine/helper metadata, root-drop probe, full image scan, bound SPDX and Docker-save/SBOM proof | Application-image scans, engine-only probes and clean OS results do not prove the complete database image. |
| Native archive/restore                     | `cmd/storage` on separate fresh databases                                                                                 | Successful archive listing alone is not a restore.                                                         |
| Compose/model/images                       | Compose config, build, start, probes and restart                                                                          | Definitions or image build alone do not prove startup.                                                     |
| Helm/cluster                               | Lint, render, install, probes, RBAC and TLS                                                                               | Rendering alone does not prove a pod runs.                                                                 |
| Webhook/Matrix                             | Persisted retries against a disposable HTTPS receiver                                                                     | Local receipt does not prove an operator receiver, public DNS or a complete homeserver.                    |
| Browser/accessibility/performance          | Critical journeys in both locales/themes at required widths                                                               | Each needs saved current evidence.                                                                         |

## Real database

The test role needs schema creation and the permissions used by concurrency
regressions. Use a disposable database with no user data. Tests create unique
schemas and remove only their own fixtures. Never reuse a live application URL.

Use PostgreSQL server and native `psql`, `pg_dump` and `pg_restore` clients at
17.11 or a later 17.x patch. The [canonical image policy](../deploy/postgres-image.json)
defines the approved digest. Corrective proof needs real database execution,
readiness/ledger checks and native backup/empty-target restore, plus exact
database base/derivative and engine/helper metadata, root-drop behavior, full
image scan and matching SPDX inventory. The released `stackplore-postgres:1.0.3`
rebuilds unchanged official gosu 1.19 with Go 1.26.9 on the same pinned upstream
17.11 base, retaining its engine and entrypoint. Build owned patched fixtures
before Compose/CI tests; Helm remains external-PostgreSQL based. Require actual
postgres 17.11, gosu 1.19/Go 1.26.9 and root-to-database-user probes. All image
findings remain blocking, including those inherited from the base.
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
jobs. Older tagged results do not certify a different 1.0.3 source; current results
are recorded in the evidence plan.

## Historical provider and isolated-demo checkpoint

The `0.5.0` checkpoint has scoped Docker/Kubernetes, PostgreSQL and guest-browser
receipts. Final complete changed-tree gate D passed with real PostgreSQL,
36 client tests, 350 web tests and both builds. Exact publication/CI identities
are retained in the active plan; do not report early failed full gates as successful. The active
plan records the 8.5/10 findings, fixes and final independent scoped 9/10 review.
Those counts are historical and are not current release counts.

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

## Candidate 1.0.3 focused corrections

Fresh PostgreSQL 17.11 tests ran on separate owned disposable databases and
verified cleanup/absence. The retention control expires an eight-day metric at
the default 168 hours; 8760 hours preserves it and all four older history
classes. One-hour and mixed class policies expire only eligible rows. Real
lock contention proves that an already pending policy increase is read before
deletion, and that an update cannot pass the shared policy lock while deletion
is blocked. Another case changes policy between worker batches and verifies
the next batch reloads it. Existing fair quotas, newest samples, current health,
synthetic observations and protected audit/deliveries remain covered.

Direct application transaction tests exercise all seven settings in a concurrent
one-winner commit; stale version, changed material, wrong/expired confirmation,
missing backup acknowledgement and wrong-purpose rejection; one-use grants;
and failed-audit rollback. Cleanup preview/commit removes at most 1,000 rows,
retains its frozen cutoffs, reports remaining work and rolls back deletion when
audit insertion fails. Export checks effective settings/source summaries,
masking, audit rollback and one-use authentication. A failed additional fixture
attempt used a nonexistent `interval_us` column; it remains failed. The corrected
fixture uses `interval_seconds` and passed. Preserve both outcomes in the active
plan rather than relabeling the fixture error as a product failure or success.

With a disposable real PostgreSQL test URL and pinned tools:

```powershell
go -C apps/api test ./internal/postgres -run 'TestScheduledRetention|TestRetentionSerializes|TestRetentionHolds|TestObservationRetention|TestApplicationSettings|TestApplicationCleanup|TestApplicationConfigurationExport' -count=1 -v
go -C apps/api test ./internal/observation -run TestRetention -count=1 -v
go -C apps/api test ./internal/httpapi -run 'TestStreamsRejectDeniedReplayAndRecoverWithFreshFeed|TestStreamsPollDenialClosesActiveClientsAndAllowsFreshRecovery|TestStreamsRejectInventoryAndTypedPermissionDenials|TestStreamSnapshotsKeepStalePartialAndOtherAvailability' -count=1 -v
go -C apps/api test ./internal/realtime -run TestManagerInvalidation -count=1 -v
```

The transient SSE tests use real HTTP connections with controlled provider
snapshots, two active clients, denied handshakes and poll-time revocation across
health, metrics, events and logs. Recovery must use fresh authorized data, with
old replay and queued frames fenced. Stale/partial/unavailable/unsupported states
retain their existing semantics; denial is a separate 403 boundary. These tests
do not contact a live provider or prove deployment revocation latency. New-rule
default tests use the effective settings read, delayed-response edit preservation
and failure/retry states; they do not change existing stored rules.

An additional real HTTP regression reproduced a legal 100-event Persian REST
page (1,669,440 bytes, HTTP 200) followed by an SSE HTTP 200 response with no
frame: the old transient hub rejected the page above its 256-KiB limit. The
corrected producer emits a bounded `snapshot_overflow` reset and stays open;
event and log HTTP cases recover to normal small snapshots. Actual producer/hub
tests cover changed and unchanged large pages, subsequent small-page recovery,
permission revocation and exact frame-budget boundaries for every transient
topic. The initial fixture-only failure used the generic test helper's 64-KiB
REST ceiling; its corrected fixture uses the production 16-MiB default. Keep that
failed setup separate from the reproduced silent-stream defect.

```powershell
go -C apps/api test ./internal/httpapi -run TestStreamBudget -count=1 -v
```

## Retained 1.0.2 regressions and runtime gates

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
migration 16 adds the source alert feed's matching keyset index. Migration 17
adds notification/rule byte-collation indexes; real non-byte-collation and
ledger-14/16-to-17 checks must preserve data and canonical paging without
changing earlier migration bytes. Native special-name backup/restore requires
the explicitly enabled disposable native-tool fixture; no customer database
may be used for those tests.

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

Retained-event transport regressions use a fake persisted reader to bypass
provider ingestion filtering. They cover REST, initial SSE and a real later SSE
poll, deployment policy, safe text/metadata, Persian 8,192-rune bounds, mask
expansion and reader immutability. They prove output masking, not a historical
SQL/backups rewrite. Run
`go -C apps/api test ./internal/httpapi -run 'TestRetainedEventSummariesAreRedactedAtHTTPBoundaries|TestRetainedEventStreamPollKeepsDeploymentRedactionPolicy' -count=1 -v`.

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
