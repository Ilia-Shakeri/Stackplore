# Stackplore active implementation evidence

Review dates: 2026-10-12–17. Target: 1.0.0. Stages 1–4 are verified development
checkpoints through `0.4.0`. Stage 4 final local and browser gates passed; exact
remote publication outcomes are recorded separately. Current work is Stage 5
provider fidelity. Stages 5–9 remain open.
[Roadmap](roadmap.md) ([فارسی](roadmap.fa.md)) defines ordered exit gates.
[Audit](codebase-audit.md) records coverage and defects.
[Historical evidence](history/implementation-plan-2026-08.md) retains prior results.

On 2026-10-17 the maintainer explicitly renewed authorization to test,
version, commit and push each remaining stage. This supersedes the earlier
no-push instruction for this task. The unchanged `v0.3.0` implementation
`7815e52a2e71b8452e1863f798beec5b7b0c8fb9` was pushed with its existing tag.
[Main run 39007896683](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39007896683)
and [tag run 39007898666](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39007898666)
completed successfully. Code/full real-PostgreSQL/race and image-build/chart
jobs passed; neither run proves a Compose start or a cluster install.

## Review entry and boundaries

Entry commit `08028f4` had substantial uncommitted PostgreSQL/provider/product/notification/deployment work. It was preserved, reviewed and integrated. An existing unreferenced dashboard image stays outside this release because capture provenance was not established. No contributor/runtime data was reset or deleted.

Review covered domain/auth/HTTP/providers/realtime/alerts/notifications/operation scaffold; both persistence generations, composition roots and tests; web/interface/client/contracts; scripts/deployment/workflows and docs. Legacy storage was removed only after real PostgreSQL parity and native restore proof. The system PostgreSQL service was not changed; all SQL proof used a separate repo-local cluster and disposable databases.

Before the first tag, untracked SQL was normalized once to LF with one final
newline, so local embedded bytes match the Git index/fresh checkout. The final
SQL and native gates were rerun on new disposable data after this step. Existing
development database history was not changed; see [upgrade constraints](operations/upgrade-rollback.md).

## Correction loop

| Pass                     | Scoped score | Remaining baseline problems                                                                                      | Correction                                                                                 |
| ------------------------ | ------------ | ---------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| Entry                    | 5/10         | Auth/ownership/outbox races, log leaks, invalid success replies, source races, first boot and contradictory docs | Whole-code review and ordered roadmap                                                      |
| 1                        | 7/10         | Key lifetime race, restore ownership/scope/durability, chart YAML/mount/forwarded scheme and false metric states | Fixes plus real SQL/HTTP/provider/frontend regressions                                     |
| 2                        | 8/10         | New version check import/lint, web test formatting, incomplete setup quick start and unchecked final gates       | Fix scripts/docs/format and rerun relevant gates                                           |
| Independent scope review | 9/10         | No known severe defect in delivered stage-1 fixes                                                                | Backend/frontend/PostgreSQL review; final gate and release diff required before completion |

These historical scores judge stage-1 infrastructure and corrected code, not
measured coverage or whole-product completion. At that baseline, stages 2–9 and
target proof were open. The stage 2 candidate and remaining 1.0 scope are
recorded separately below. The final architecture still needs accepted target
alignment.

## Stage 1 changes

- Common auth transaction serialization preserves setup/current actor/final-admin invariants. Queries and whole transactions use deadlines.
- Ownership precedes migrations; dedicated same-session probes detect loss, seal storage and stop service/jobs. Pool minimum capacity, shutdown and ledger reads are bounded.
- Notification allocation/enqueue share a transaction lock; a late earlier sequence cannot be stranded. Leases/retries/terminal audits remain.
- 18 real auth/product repository parity contracts replace legacy storage behavior. SQLite code/migrations/dependencies/licenses and busy-timeout setting are removed. Key/path/single-link checks remain in `runtimefiles`.
- Bounded non-audit PostgreSQL prune exists. Scheduled quotas and separately protected audit expiry remain future work.
- Bad input returns a problem, denied/outage codes stay truthful, metric replies cap at 200, source/health summaries mark partial/unknown, exact resource reads work beyond the first page.
- Shared plain/structured log scrub masks bearer/basic/cookies/keys; repeated pod container names remain distinct; webhook attempts close idle sockets.
- Explicit web source requests, stable details/history/search, absent metric states, strict runtime guards, real predicate translations and Persian log direction have regressions.
- Setup environment/Secret inputs, pod mount ownership, chart annotation YAML, HTTPS forwarding, edge probe separation and Compose filesystem/resource bounds are corrected. Real source profiles default off.
- Native backup/restore covers public schema, has a 15-minute bound and maintenance ownership monitor; publication syncs and cannot overwrite.
- Current guides/roadmap/audit, consistent 0.1.0 metadata, document/link/version gates, changelog and pinned workflow actions are included.

## Exact local context

Windows PowerShell; Node.js 24.18.0, Corepack 0.35.0, pnpm 11.17.0, Go 1.26.5. PostgreSQL server/native clients 17.11 from the existing installation ran separately with repo-local data on loopback55473. Password-free trust is limited to this disposable test cluster and is not production guidance. Helm 3.17.3 was downloaded into ignored tools and verified against its published SHA-256. No global tool install occurred.

Docker/Compose, live providers/receiver, a cluster and a local C race compiler are absent from this proof. Linux CI has a race command; its outcome is separate from plain local Go tests.

```powershell
$repoRoot = (Get-Location).Path
$env:PATH = "$repoRoot\.tools\node-v24.18.0-win-x64;$repoRoot\.tools\go\bin;$env:PATH"
$env:CI = 'true'
$env:GOCACHE = "$repoRoot\.tmp\go-build-v1"
$env:STACKPLORE_TEST_POSTGRES_URL = 'postgresql://stackplore_test@127.0.0.1:55473/stackplore_test?sslmode=disable'
npm.cmd run bootstrap
npm.cmd run verify
```

The test URL contains no password and names only a disposable test database. Remove its test-only variable before runtime commands. Fixtures use unique schemas and a shared test advisory gate. Native restore uses fresh whole databases and private runtime-key directories.

## Historical 0.1 checks

| Command/check                                              | Result                                                                                                                                                    |
| ---------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `npm.cmd run bootstrap`                                    | Passed pinned tools/frozen lock                                                                                                                           |
| Focused HTTP/provider/notification Go suite                | Passed; real PostgreSQL HTTP setup/login/session/grant/user/logout and protected-data scan ran                                                            |
| `go -C apps/api test ./...` with real URL                  | Passed after legacy removal; latest ledger/maintenance regressions also passed                                                                            |
| PostgreSQL parity/concurrency                              | 18 auth/product contracts plus ownership, query deadlines, migration rollback/unknown/changed history, outbox commit order and protected retention passed |
| Web tests/types/lint                                       | 160 tests passed, types/lint passed, locale parity 345                                                                                                    |
| `check:contracts`                                          | 181 bounded schemas and generated output parity passed                                                                                                    |
| `check:docs`, `check:version`                              | Inventory/217 local links and consistent 0.1.0 metadata passed after final document edits                                                                 |
| Helm lint/default/setup+cluster+ingress/annotation renders | Actual Helm 3.17.3 passed; no pod/cluster claim                                                                                                           |
| Native backup/archive-list/empty-target restore            | Passed17.11; ledger6, users1, sessions0, preauth0, generation1 checked                                                                                    |
| Restored runtime                                           | Ready200, old session401, fresh admin login/source/audit reads passed                                                                                     |
| Own ownership-session termination                          | Runtime stopped with failure within monitor bound; latest marker `NATIVE_BACKUP_RESTORE_SESSION_REVOCATION_AND_OWNERSHIP_STOP_PASS`                       |
| Final `npm.cmd run verify`                                 | Passed exit0 with real PostgreSQL URL: all Go packages,36 client tests,160 web tests,345 locale messages,181 schemas, lint/types/builds                   |
| Remote workflow/race/images                                | Passed for tagged implementation `72f8e466cbf81d5e00c541ce09e60d5e886a9e81`; links below                                                                  |

Remote evidence for the immutable 0.1.0 implementation revision:

- [Tag run 38262601532](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/38262601532): completed successfully; code and deploy jobs passed.
- [Main run 38262599330](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/38262599330): completed successfully; code and deploy jobs passed.
- The code job ran the full gate against real PostgreSQL and `go -C apps/api test -race ./...` on Linux.
- The deploy job validated the Compose model, linted/rendered the reviewed chart profiles, and built both images. It did not start Compose or install a cluster.

The follow-up evidence documentation commit does not change runtime code or
retag the tested implementation. It records these results after the runs ended.

Failures were repaired: entry Go operation formatting; later web test formatting; version-check URL import/regex lint; empty annotation-map Helm YAML. Native helper field-name errors were corrected using fresh disposable databases. An interrupted validation session stopped tool jobs; the disposable PostgreSQL cluster recovered its own WAL, then final checks/native drill were rerun. This does not prove a production recovery.

## Remaining 1.0 scope

Stage 2 core source/observation/collection/history work is in the verified local
checkpoint; its final evidence follows. Stage 3: alert window/rate/outage/event semantics,
durable live alert/replay, Matrix and receiver proof. Stage 4: full
paging/detail/log/topology, idle session refresh and browser journeys. Stage 5:
vendor-client alignment, restricted production Docker, namespace/partial rights,
provider fidelity and full multi-environment demo/guest isolation. Stage 6:
operator audit quotas/minimum retention/reviewed cleanup, narrow operation gates
and settings/export/cleanup. Stage 7: built-web serving alignment, real
containers/cluster/TLS/upgrade/restore. Stage 8/9: safe PWA,
accessibility/performance/security, artifacts/SBOM/provenance, complete bilingual
docs and independent final 1.0 review.

## Stage 2 checkpoint — 2026-10-13

The durable observation/source checkpoint adds domain capture/read/retention
ports, PostgreSQL adapters and migrations 7–8, bounded collection and fair
retention jobs, cached provider reads, fixed operator-owned profile registration,
and exact administrator source toggles with one-use purpose-bound recent auth.
All six released 0.1 migrations remain byte-for-byte unchanged. Profile and
observation writes have source/version/generation checks and bounded batches;
partial capture cannot retire unseen records. Logs remain live and are not
persisted. See the [English](observations.md) and [Persian](observations.fa.md)
behavior documents for bounds and supported workflows.

The review covers every new backend/domain/SQL/job/transport/source UI module
and affected contracts, manifests, tests and documentation. Retention preserves
the current metric per resource/name, synthetic fixture history, pending and
audit-linked delivery rows and all audit records. Audit retain-all protection is
implemented; operator audit quotas and reviewed expiry remain stage 6. Namespace
profiles and restricted production Docker remain stage 5, with their live proof
separate. The static demo alarm is preserved through a narrowly validated
compatibility wrapper until the durable alert stage.

| Correction pass        | Scoped score | Remaining issue                                                                                                   | Applied correction                                                                                                                              |
| ---------------------- | ------------ | ----------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Initial implementation | 7/10         | Missing runtime job wiring, incomplete source UI/contract and safe mutation proof                                 | Composition lifecycle, closed routes/types, bilingual UI, exact transaction tests                                                               |
| First self review      | 8/10         | Mutable cursor anchors, denied/stale cache truth, retention starvation/current deletion, unfinished capture truth | Immutable signed sort values, source/port status overlays, fair quotas/latest protection, partial in-progress status                            |
| Independent review     | 8/10         | Old authorization timestamp, multi-read snapshot race, cross-source event IDs, demo metadata/alarm regression     | Decode then prepare, database clock after auth lock, coherent bounded SQL read snapshot, source/revision IDs and explicit fixture compatibility |
| Final scope review     | 9/10         | No known severe defect within the delivered checkpoint; broader 1.0 and target gates remain open                  | Independent backend/SQL/frontend/docs review, complete local gate and exact-tree native drill                                                   |

These scores apply only to the delivered checkpoint. Whole 1.0 completion,
accepted target architecture alignment and live deployment evidence are still
open and cannot inherit this score.

Historical candidate checks before the final gate:

- Source SQL transaction/replay/expiry/wrong target/state/purpose/revoked actor
  and failed-audit rollback regressions passed against real PostgreSQL 17.11.
- PostgreSQL capture/restart/6-to-current upgrade/keyset/long Unicode/status/
  retention tests passed; final coherent-read regressions awaited the gate then.
- Web source UI tests, strict types, lint and build passed; both catalogs have
  391 keys. Long infrastructure cursors accept 4,096 and reject 4,097 bytes;
  account input remains 1,024.
- OpenAPI policy and regenerated transport output passed for 186 bounded schemas.
- A disposable API proved first capture, exact source disable, replay rejection,
  stale cached reads and persisted state after process restart. Native backup
  and restore ran, but the helper expected 18 metric samples while collection
  correctly records all nine supported names across six resources (54 samples).
  The fixture assertion was corrected and the fresh native drill then awaited rerun.

Local warm-cache `EXPLAIN (ANALYZE, BUFFERS)` used 2,100 synthetic/native
resources, 21,000 metric points and 10,000 events in isolated fixtures. Reviewed
inventory/name, event keyset, current metric, bounded prune and audit paths use
their relevant indexes. Example timings reported by the test were approximately
0.160ms inventory/51 rows, 0.044ms events/51 rows, 0.366ms current metrics/100 rows
and 0.867ms prune/250 rows. The initial full-table latest-metric check was
replaced with an indexed newest-row probe. These are fixture measurements,
not production capacity, cold-cache or latency/SLO proof. Exact reproducible
commands and final outcomes are recorded below after the completed gate.

## Final 0.2 local evidence

| Exact command/check                                                                                                    | Observed result                                                                                                                                                                                                                |
| ---------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `npm.cmd run bootstrap`                                                                                                | Passed pinned tools and frozen lock; four workspace manifests agree at 0.2.0                                                                                                                                                   |
| `npm.cmd run verify` with the same disposable real PostgreSQL URL above                                                | Exit 0: format/docs/version/locales/contracts/lint/types/all Go tests/client/web tests/API and web builds                                                                                                                      |
| PostgreSQL within the full gate                                                                                        | Passed 46.013s; no environment-gated SQL skip in this run                                                                                                                                                                      |
| Client/web                                                                                                             | 36 client tests and 206 web tests passed; 391 locale keys matched                                                                                                                                                              |
| Contract output                                                                                                        | 186 bounded schemas and regenerated TypeScript output matched                                                                                                                                                                  |
| SQL focused upgrade/capture/read/retention suite                                                                       | 21 new test functions; full package 41.759s, final focused snapshot/history/progress checks 4.066s                                                                                                                             |
| `go -C apps/api test ./internal/postgres -run TestObservationHotPathQueryPlansOnRealPostgreSQL -count=1 -v`            | Passed; five plans reviewed, final log 3.270s. Exact seed: 2,103 resources, 2,003 snapshots, 21,003 metric samples, 10,003 events and 10,000 audit rows                                                                        |
| Helm 3.17.3 `lint` and `template` with `api.setup.enabled=true`, `api.kubernetes.enabled=true`, `ingress.enabled=true` | Passed; actual cluster binding appeared. No cluster-install claim                                                                                                                                                              |
| Latest compiled API/storage native drill on new whole disposable databases                                             | Passed `STAGE2_CAPTURE_TOGGLE_REPLAY_RESTART_NATIVE_RESTORE_OWNERSHIP_PASS`                                                                                                                                                    |
| Native invariants                                                                                                      | Ledger 8, users 1, resources 6, metrics 54, disabled source/version 2 retained; sessions/preauth 0 and restore generation 1                                                                                                    |
| Actual HTTP/lifecycle drill                                                                                            | First persisted capture/fixture metadata, exact authenticated toggle, replay 409, stale reads, process restart, native backup/list/restore, old-session 401, fresh login/source/audit reads and terminal ownership loss passed |

The final native drill used PostgreSQL/native tools 17.11 and fresh private
test databases `stackplore_stage2_native_d` and `stackplore_stage2_restore_d`
on the same isolated loopback cluster. It did not touch the system service or
user data. All raw authentication values were generated in process memory and
not printed or committed. Runtime keys, archives, helper executables and logs
remain ignored private test artifacts.

Two helper mistakes were corrected without relaxing product checks: the sample
assertion was changed to the nine-name fixture contract (54), and removing
already absent setup environment names became idempotent. A fixture for old
authorization time also needed an earlier installation creation time so its
invented historical setup respected database constraints. The complete gate
and final fresh native drill passed after these corrections.

The 0.2 scope score is 9/10 after independent SQL/backend/frontend/document
review. The first six migration byte diffs remain empty. Real live provider,
browser geometry/accessibility, local race tooling, Compose startup, cluster
install/TLS and target production evidence remain open. Remote CI must be
checked for the exact tagged implementation before claiming that boundary.

## Final 0.2 remote evidence

Implementation commit `b8448bf2016a466e522263ae32983e1f2c6af232` and annotated
tag `v0.2.0` were pushed without force. The remote tag peels to that exact
implementation revision. Both workflows completed successfully on 2026-10-13:

- [Tag run 38381333991](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/38381333991): code and deploy jobs passed.
- [Main run 38381330095](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/38381330095): code and deploy jobs passed.
- Code ran the complete gate with real PostgreSQL and the Linux Go concurrency
  gate (`go -C apps/api test -race ./...`). Local Windows race tooling remains
  absent; the Linux result is actual separate proof.
- Deploy validated the Compose model, linted/rendered reviewed Helm profiles
  and built both images. It did not start the application through Compose,
  install a cluster or prove live infrastructure/receiver/browser flows.

This follow-up documentation records completed remote results. It changes no
runtime behavior and does not rewrite the implementation tag. The unknown
dashboard image remains unstaged; private test cluster/data/archive artifacts
remain ignored. The disposable PostgreSQL process was stopped after proof;
the system PostgreSQL service was not changed.

## Stage 3 — 0.3.0 alert evidence and delivery

Date: 2026-10-16. Scope includes covered alert evaluation, fresh claim fencing,
durable source-scoped replay, idempotent demo seed, webhook/Matrix lanes,
secret-free settings/test transport, bilingual interface and operator documents.
The first eight released migrations remain byte-identical; new migrations 9–10
are canonical LF before final native/SQL verification. Existing unknown
`artifacts/stackplore-dashboard-example.png` remains outside the change.

### Correction loop

| Pass                              | Scoped score | Remaining issue                                                                                                          | Correction                                                                                                                                      |
| --------------------------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Initial implementation            | 7/10         | Latest-only windows, incomplete event scope, cache/outage confusion, transient replay and missing Matrix lane            | Covered raw evidence ports, durable changes, shared safe outbound and separate lanes                                                            |
| Review                            | 8/10         | Claims age in serial queues, configuration changes reuse evidence, backdated middle-prune gap, stale lease/result writes | Fresh one-at-a-time claims, configuration/version fences, durable prune watermark and atomic episode closure                                    |
| Integration review                | 8/10         | Unchecked stream metadata/query, reset freshness, late interface writes and open-stream shutdown                         | Closed typed metadata/query, reset/change refetch, recovered snapshot publication, interface generation fences and stream shutdown cancellation |
| Final independent review and gate | 9/10         | No known severe defect in the delivered stage; target and later-stage limits stay open                                   | All local gates, real PostgreSQL, disposable HTTPS, final native C and reviewed migration bytes passed                                          |

Scores judge this delivered stage, not whole-project completion, measured test
coverage or production capacity. Stage 3 is a completed local development
checkpoint. Stages 4–9 remain open; no whole-product or target-readiness claim is made.

### Executed focused proof

| Command/check                                                                              | Result                                                                                                                                                                                                                                                                                                          |
| ------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `npm.cmd run bootstrap`                                                                    | Passed pinned tools and frozen lock at 0.3.0                                                                                                                                                                                                                                                                    |
| `go -C apps/api test -count=1 ./internal/alerting ./internal/observation` and scoped `vet` | Passed 1.227s / 1.031s; raw freshness, coverage, resets, replicas/events, pending continuity and fresh claims                                                                                                                                                                                                   |
| `go -C apps/api test -count=1 ./internal/notification ./internal/config` and scoped `vet`  | Passed 5.308s / 0.706s; DNS/IP/redirect/TLS/size/timeout policy and closed Matrix settings                                                                                                                                                                                                                      |
| Real PostgreSQL focused alert/collection suite                                             | Passed 24.625s: eight-ledger upgrade, configuration fences, seed/ack/restart/delete, ordered replay, middle-prune reset, edit/delete lease invalidation, audit rollback and concurrent acknowledgement                                                                                                          |
| Real PostgreSQL cursor follow-up                                                           | Passed 5.613s: a nonzero cursor must be a committed row in the same source; foreign-source and rolled-back sequence gaps reset without hiding a later valid change                                                                                                                                              |
| Real PostgreSQL delivery suite                                                             | Seven focused tests passed 11.551s: upgrade from 8 and 9, legacy synthetic cancellation, adapter/reference isolation, stale ownership, lease expiry/recovery, retry exhaustion and Matrix test intent/result audits                                                                                             |
| Persisted disposable HTTPS receiver                                                        | Passed actual TLS server with injected checked-address/dial seam: webhook signature/idempotency and canonical retry bytes; Matrix PUT transaction ID and plain `m.notice` retry after repository reopen. No public endpoint or complete homeserver claim                                                        |
| `npm.cmd run check:contracts`                                                              | Final gate passed 192 bounded schemas, contract validation and generated parity                                                                                                                                                                                                                                 |
| Web/interface focused run                                                                  | 208 tests passed before final response-race regressions; final gate reruns the final tree                                                                                                                                                                                                                       |
| Native API/storage drill B                                                                 | Passed seed -> firing -> acknowledgement -> SSE change -> process restart -> replay/ack preservation -> native backup/list/empty-target restore -> old session401/new login -> ownership-loss failure stop. Initial helper A had a mistyped fixture filter; corrected on fresh B, no application failure hidden |
| Restored native invariants                                                                 | Ledger10, users1, resources6, metrics54, demo seed1, acknowledged alert1, outbound deliveries0, disabled source version2, sessions0/preauth0 and restore generation1                                                                                                                                            |
| Helm 3.17.3 lint and configured render                                                     | Passed actual local strict lint/render with setup, Kubernetes read profile, ingress, webhook and Matrix. Matrix token references the existing Secret; no cluster install claim                                                                                                                                  |
| Final `npm.cmd run verify` with real PostgreSQL                                            | Passed exit0 on frozen runtime tree: PostgreSQL package68.899s, all Go packages,36 client tests,211 web tests,394 locale messages,192 bounded schemas, format/lint/types and both builds. Log `.tmp/stage3-verify.log`                                                                                          |
| Final native drill after last runtime correction                                           | Fresh C passed on final compiled API/storage: `STAGE3_SEED_ACK_SSE_REPLAY_RESTART_NATIVE_RESTORE_OWNERSHIP_PASS`; source `stackplore_stage3_native_c`, restore `stackplore_stage3_restore_c`, private key directory `.tools/stage3-native-key-c`, ignored archive `.tools/stage3-review/native-backup-c.dump`   |
| Remote CI/race/images/Compose/cluster/browser/live operator receiver                   | Initially local under then-current instructions; on 2026-10-17 exact unchanged main/tag CI passed code/race and image/chart builds. Compose start, cluster, browser and live operator receiver were not part of those runs                                                                                                                                                                                                                                  |

Native helpers and private data remain under ignored `.tools/stage3-review` and
repo-local test directories. Local proof logs stay in ignored `.tmp`; no credential,
cookie, full destination, private key or notification response body is copied
into committed evidence. Windows has no local C race toolchain or Docker/Compose
runtime; no new race/image/startup claim is made.

After all proof finished, the separate repo-local PostgreSQL process shut down
cleanly; the system service was untouched. Version metadata, documents and
reviewed release paths were rechecked after the final evidence update. The
checkpoint was committed and tagged locally as `v0.3.0`. The same immutable
implementation/tag was pushed on 2026-10-17 under renewed authorization, with
the exact successful CI results linked at the top of this plan.

### Disposable query plans

`go -C apps/api test -count=1 ./internal/postgres -run TestAlertHistoryHotPathQueryPlansOnRealPostgreSQL -v`
captured `EXPLAIN (ANALYZE, BUFFERS)` in the scoped test (13.928s). The corpus
has one source, 10,002 changes, 10,001 collection outcomes, 10,001 replica facts
and 10,003 metrics. Warm SQL-only rows/execution milliseconds:

| Query                  | Rows  | ms    |
| ---------------------- | ----- | ----- |
| Ordered replay         | 100   | 0.122 |
| Collection window      | 1,361 | 1.553 |
| Current collection     | 1     | 0.040 |
| Replica window         | 1,362 | 1.597 |
| Metric window          | 1,362 | 2.615 |
| Replay prune quota     | 33    | 0.295 |
| Collection prune quota | 33    | 0.452 |

This one-source corpus used primary-key/retention indexes; it did not demonstrate
selection of the added range indexes. These numbers are not end-to-end latency,
target scale, multi-source planner evidence or a service-level objective. The
source, caps, predicates and aggregate deadlines are part of the reviewed query
boundary. Broader measured capacity remains later-stage work.

## Stage 4 — 0.4.0 verified exploration checkpoint — 2026-10-17

The frozen runtime passed the full local gate, real PostgreSQL regressions, a
fresh native restart/restore drill and actual browser journeys. This completes
the scoped development stage; it does not complete 1.0 or target acceptance.

### Scope and correction loop

- Inventory filters run against the complete PostgreSQL set before keyset
  pagination. Provider/environment/scope/health/availability, sorting and source
  selection have closed query validation and cursor bindings. Counts aggregate
  the active snapshot rather than the first 200 resources/100 health targets.
- Source-wide topology/events no longer silently filter to an inventory prefix.
  Exact-resource requests still validate identity. Relationship transport carries
  bounded evidence; graph/list views preserve directions and isolated nodes.
- Logs use exact bounded ranges and resource/container/previous selection.
  Loaded-line search/severity and export remain bounded to loaded redacted data.
  Empty source reads preserve availability without calling target-only adapters.
- Product transport and pages are split by feature. Lists cap at 1,000 loaded
  items; the graph caps at 200 nodes with 50-node/50-edge list pages. Late replies,
  repeated cursors, cancellation and cap truncation have regressions.
- Pointer/key activity requests a server session context at most once per 60
  seconds, with a 10-second deadline, single-flight and session-generation fences.
  There is no invented expiry or idle background extension.
- Metric series use actual sample timestamps and keep separate resource curves.
  Tables expose identity/time/value; charts do not invent synchronized fleet sums.

| Pass | Scoped score | Findings and correction |
| --- | --- | --- |
| Initial feature review | 8/10 | First-page source counts and topology/event scoping; live default range cursor drift; partial/denied health; empty target reads; incomplete pagination and stale session deadlines. Fixed in domain, PostgreSQL, HTTP and web boundaries with regressions. |
| Independent and real-browser review | 8/10 | Missing partial-summary reason, source capability stale filter order, fractional rule threshold validation, irregular metric spacing and resource aggregation, last-page cap truncation, fixed demonstration-clock boundary, slow tail read starvation, query patch races and mobile toolbar layout. Fixed and directly verified. |

These are scoped architecture/implementation scores. Product completeness and
target evidence remain separate; stages 5–9 retain their exit requirements.

### Executed checks

| Check | Result |
| --- | --- |
| Real PostgreSQL inventory regression | Three tests passed: 223-resource aggregate, 200+20 filtered continuation, all new filter cursor bindings, denied health, disabled-source stale and capability stale/denied classification. Latest focused run 9.073s. |
| All Go packages with `STACKPLORE_TEST_POSTGRES_URL` | Passed after test-process recovery; PostgreSQL package 74.422s, HTTP 4.755s. Log `.tmp/stage4-api-all-recovered.log`. |
| HTTP exploration query regressions | Eleven tests with subcases passed: closed filters, exact microsecond UTC ranges and bounds, previous/container targets, evidence mapping, all-source versus exact-resource pages, and 15 empty-source/status combinations. |
| Runtime client/form checks | Focused API/rule, paging, chart, session and routing tests, strict types and lint passed; the final full gate reran the frozen runtime. |
| Development proxy | Real Vite/upstream probe passed Host and Origin preservation for API, health, ready and metrics paths. Marker `DEVELOPMENT_PROXY_HOST_ORIGIN_FOUR_ROUTES_PASS`. |
| Browser | Actual disposable PostgreSQL/API/Vite: 120 data-loaded product states, 40 authentication form states and 20 final topology rechecks passed, with real login/logout, inventory filter/reload/detail/log links, redacted log download, events, grouping/evidence/keyboard and persisted alert acknowledgement. [Retained receipts](evidence/0.4.0/README.md) state semantic accessibility and clipboard limits. |
| Documents/version | Final document inventory/links, formatting, version and diff checks passed after the evidence update; all metadata matches 0.4.0. |
| Full `npm.cmd run verify` | Passed exit 0 on the frozen runtime: all Go packages with the real disposable PostgreSQL URL (cached after the uncached 74.422s package proof), 36 client tests, 284 web tests in 26 files, 509 locale messages, 192 bounded schemas, format/lint/types and both builds (web 10.64s). Log `.tmp/stage4-final-verify.log`. |
| Exact 0.4.0 commit/tag/push/CI | Not created yet; only record after final named gates pass. |

An initial full web test run reported 267 passing and one cold lazy-route timing
failure; the route regression was corrected without weakening assertions.
The first new PostgreSQL probe used the wrong test variable and skipped; it is
not SQL proof. Corrected real-URL runs above executed the tests. Denied-health
fixture setup initially violated the database's enforced unknown-state invariant;
the fixture was corrected, and the actual regression then passed.

A tool/session interruption stopped the disposable processes during a full Go
run. Its incomplete result was discarded. The separate repo-local cluster
performed normal WAL recovery, reached readiness, and the complete Go suite was
rerun successfully. The system PostgreSQL service was not changed. Local browser
fixtures, private runtime keys and raw temporary logs remain ignored; committed
evidence must contain no authentication material.


### Final correction and native proof

Final real-image review lowered the graph score from 9 to 8: a long edge crossed
an unrelated card and implied a false link. Routing now uses bounded gutters and
card-boundary ports; a halo separates crossings and geometry is memoized across
pan/zoom. Thirteen focused tests passed, including 1,000 distinct paths among
200 cards with every segment outside every card interior and inside world
bounds. All 20 locale/theme/width topology states were then repeated with actual
loaded data. Scoped architecture and implementation scores returned to 9/10.

Fresh compiled 0.4 API/storage native drill D passed:
`STAGE4_SEED_ACK_SSE_REPLAY_RESTART_NATIVE_RESTORE_OWNERSHIP_PASS`

The source `stackplore_stage4_native_d` and restored
`stackplore_stage4_restore_d` are disposable databases. Seed firing, one-use
source version changes, durable SSE snapshot/change/replay, acknowledgement,
API restart and preserved state passed. Native backup/list/empty-target restore
verified ledger 10, one user, six resources, 54 metric facts, one seed and one
acknowledged alert, zero outbound deliveries, disabled source version 2, zero
sessions/preauth and restore generation 1. The old session returned 401 and a
new login worked. Ownership loss terminated only the fixture process within the
bounded failure window. Private keys, dump and helper logs remain ignored.

The actual inventory query-plan drill also passed (5.735s package, 2.19s test)
with 2,003 demonstration resources, 100 native resources, 21,003 metric facts,
10,003 events and 10,000 audit records. Warm SQL-only observations were about
0.19ms for 51 name-sorted resources, 17.80ms for the full aggregate, 5.98ms for
101 health-filtered resources, 0.05ms for 51 events, 0.41ms for 100 current
metrics, 1.42ms for a 250-row retention quota and 0.10ms for 51 audit rows.
These are fixture SQL plans, not end-to-end latency, target scale or an SLO.
