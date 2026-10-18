# Repository review — 2026-10-12–18

Scope: all first-party backend, frontend, interface/client packages, contracts,
migrations, deployment, workflows, scripts, configuration, tests, and documents.
Generated types were checked against their source rather than treated as an
independent design. Third-party bundles and local caches are excluded. The
review began at `08028f4` with a substantial uncommitted implementation; those
files were preserved and reviewed as part of the candidate baseline.

## Fresh audit of the delivered baseline

The 2026-10-18 review restarted from documentation delivery `657a600`.
Accepted corrective source is `376ed0379900ab870dd8eb8af6b55ef3e5ec389b`,
immutable `v1.0.2`, schema 17. All nine stages are complete; architecture/code
and release acceptance score **9/10** after verified corrections and full local,
exact main/tag, signed trial/publication and independent public/archive proof.
The first tag browser failure, blocked release and unchanged-tag retry keep
their actual outcomes. The [fresh audit](evidence/1.0.2/fresh-audit.md) records
all negative-to-positive loops, four-context rate diagnosis and the separate
main-only harness follow-up, including failed live retry-coverage guards.
[Public acceptance](evidence/1.0.2/published-release.json). Customer capacity
and production-rollout evidence remain operator responsibilities.

## Historical accepted 1.0.1 review

Initial quality was 5/10. Final architecture/code review is **9/10** at
`73ece4754554cca548c242768b195d21664d37b8`, immutable `v1.0.1`.
That review accepted the nine stages within its then-recorded scope. The fresh
audit above supersedes its no-open-defect conclusion for the changed source. Review scores are judgments, not measured coverage.

Full local D passed 28 release/policy, 40 client and 453 web tests; fresh browser
B passed 14 cases. All six exact main/tag CI jobs, strict signed trial and public
publication passed. Independent consumers verified all 39 public files, 30
checksums, all 31 provenance subjects and seven SBOM attestations. Public
Windows bytes match the actual recovery-tested trial; the different public
database archive passed independent identity, engine/helper/root-drop,
read-only non-root SQL, license and owned cleanup checks.
[Published receipt](evidence/1.0.1/published-release.json).

The correction loop retained each failed candidate. PostgreSQL 17.6 and missing
image scans withheld immutable 1.0.0; unsafe 0.9.0 is superseded. Upstream-only
`a252718` passed local/browser/main CI but trial `39119787671` failed on one
CRITICAL and 24 HIGH gosu findings. Derivative `f046d523` passed actual image
build/probes and strict scan but trial `39121495195` rejected a valid
versionless pinned-source main-module package URL; five other CI jobs passed
while code was canceled. The narrow format fix preserved dependency versions
and full inventory/graph identity before local D and the successful final trial.
[Failed image receipt](evidence/1.0.1/failed-database-image.json),
[report-format checkpoint](evidence/1.0.1/database-runtime-checkpoint.json).
Empty-coverage helper output and interrupted public transfers were never
relabeled as passing analysis or artifact failures. Successful resumed transfers
preserved normal certificate and digest verification.

Earlier keyset, audit, clock, locale, legal/dependency and sensitive-diagnostic
fixes remain below. The [requirement matrix](evidence/0.9.0/requirements.md)
maps unchanged product scope; [final review](evidence/1.0.1/final-review.md) and
[release evidence](evidence/1.0.1/README.md) retain current results. Historical
pending statements below describe only their dated checkpoints. Customer,
air-gapped and cross-built platform limits remain explicit.

## Verified defects at review entry

| Priority | Entry reference                                                                            | Defect and consequence                                                                                                         | Delivery  |
| -------- | ------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------ | --------- |
| P1       | `internal/postgres/auth_repository.go:1167`                                                | Final-admin check lacks a shared transaction lock; concurrent changes can remove both admins.                                  | Stage 1   |
| P1       | `internal/postgres/store.go:85`                                                            | Migrations run before process ownership; rejected contender can change live schema.                                            | Stage 1   |
| P1       | `internal/postgres/store.go:117`                                                           | Pool health misses lost ownership connection; old runtime can outlive its lock.                                                | Stage 1   |
| P1       | `internal/httpapi/product_handlers.go:171`                                                 | Invalid query/body paths return without a problem, producing empty 200 replies.                                                | Stage 1   |
| P1       | `internal/provider/docker/provider.go:442`, `internal/provider/kubernetes/provider.go:648` | Space-delimited log mask can leave bearer/structured secret values visible.                                                    | Stage 1   |
| P1       | `internal/provider/kubernetes/provider.go:1020`                                            | Repeated container names across pods collide and reject inventory.                                                             | Stage 1   |
| P1       | `compose.yaml:32`, `cmd/server/main.go:279`                                                | Fresh non-loopback API bind requires bootstrap proof absent in deployment inputs.                                              | Stage 1   |
| P1       | `internal/provider/kubernetes/api_collector.go:67`                                         | Global mandatory reads prevent namespace-only and partial-permission observation.                                              | Stage 2/5 |
| P2       | `features/product/product-context.tsx:39`, `ProductPages.tsx:996`                          | Deferred mutable source change races child requests/direct details.                                                            | Stage 1   |
| P2       | `internal/alerting/worker.go:224`                                                          | Resolved-current worker contract lacks candidate ID; the current PostgreSQL claim excludes resolved rows and avoids this path. | Stage 1   |
| P2       | `internal/notification/webhook.go:136`                                                     | Per-attempt HTTP transport leaves idle sockets behind.                                                                         | Stage 1   |
| P2       | `internal/postgres/store.go:223`                                                           | One connection is accepted though ownership reserves it; startup waits for itself.                                             | Stage 1   |
| P2       | `internal/postgres/auth_repository.go:49`, `transaction.go:51`                             | Parent context bypasses configured total repository/transaction bounds.                                                        | Stage 1   |
| P2       | `internal/postgresops/operations.go:149`                                                   | Native backup/restore has no command deadline.                                                                                 | Stage 1   |
| P2       | `deploy/helm/stackplore/templates/web-deployment.yaml:29`                                  | Root-owned writable mounts prevent non-root Nginx start.                                                                       | Stage 1   |
| P2       | `deploy/helm/stackplore/templates/web-configmap.yaml:19`                                   | TLS ingress hop becomes forwarded HTTP and fails public-origin checks.                                                         | Stage 1   |
| P2       | `internal/alerting/provider_evidence.go:74,91,120`                                         | Connection loss skips evaluation; windows/rates ignored; first event hides matching history.                                   | Stage 3   |
| P2       | `internal/httpapi/product_stream_handlers.go:319`                                          | Alert stream uses fixture alerts instead of durable state.                                                                     | Stage 3   |
| P2       | `features/product/product-api.ts:966`, `AlertRulesSettingsPage.tsx:208`                    | Inventory/rules stop at first page; topology also truncates.                                                                   | Stage 4   |
| P2       | `features/product/ProductPages.tsx:1200`                                                   | Missing metrics become zero totals.                                                                                            | Stage 1/4 |
| P2       | `features/auth/auth-store.ts:104`                                                          | Local idle timer ignores backend activity extension.                                                                           | Stage 4   |
| P2       | `features/product/product-stream.ts:37`                                                    | Replay reset marks stale without refetch.                                                                                      | Stage 3   |
| P2       | `features/product/TopologyGraph.tsx:216`                                                   | Bidirectional/undirected edges are drawn as one-way.                                                                           | Stage 4   |
| P2       | `features/product/ProductPages.tsx:250,1317`                                               | Accessible graph omits node links/isolated nodes; real predicate labels fall back.                                             | Stage 4   |
| P1       | `internal/postgres/notification_delivery.go:15`                                            | Outbox sequence allocation can commit below an advanced cursor and lose delivery.                                              | Stage 1   |
| P2       | `features/product/product-api.ts:957`                                                      | Backend up to300 metric items conflicts with max200 transport guard.                                                           | Stage 1   |
| P2       | `internal/httpapi/product_observability_handlers.go:179,325`                               | Live log/event next-page time range shifts and invalidates the cursor.                                                         | Stage 2/4 |
| P2       | `docs/adr/0001:serving`, `docs/adr/0002:clients`                                           | Separate web runtime/custom collectors differ from accepted built-web/vendor-client targets.                                   | Stage 5/7 |
| P2       | `PROJECT_GUIDE.md:9`, `SPEC.md:runtime`, `CHANGELOG.md`                                    | Current implementation is described as old phases/SQLite or missing.                                                           | Stage 1   |

References above are entry line numbers, not permanent post-fix locations.
Backend paths are relative to `apps/api`; frontend paths to `apps/web/src`.

## Coverage and gaps

Backend review covers auth/hash/token/bootstrap/RBAC/recovery; domain validation,
ports and query/result bounds; HTTP middleware, request/response parsing and
streams; Demo/Docker/Kubernetes collection and normalization; realtime hub;
alert lifecycle, evidence and workers; notification settings/delivery/SSRF;
operation scaffold; PostgreSQL schema, migration, repository, auth, maintenance,
native operator commands and tests; legacy storage and every composition root.

Frontend review covers shell/routing/session/user management; product calls,
runtime guards, pages, graph and streams; catalogs and formatters; UI primitives,
styles/motion/theme/direction; client transport/generated types; tests and build
configuration. Operations review covers Compose, images, edge, Helm, workflow,
toolchain and scripts, governance, all ADRs and current documentation.

Historical unfinished scope at the 0.2 checkpoint: source creation through reviewed
operator profiles, audit quota/cleanup controls, full demo/guest profile,
namespace least privilege, historical fidelity,
complete paging/filter/navigation/log controls, durable live alert changes,
guarded actions, settings/export/cleanup, safe PWA, browser/live provider/
deployment/performance/accessibility/security/release evidence. Tests currently
mock many of these boundaries; their existence does not prove target behavior.

## Stage 2 follow-up review — 2026-10-13

The registry, observation domain ports, PostgreSQL adapters, bounded collection
and fair retention jobs, cached provider facade, source management use case/HTTP,
generated contracts, bilingual source UI and all affected documents were reviewed.
The implementation is a development checkpoint; it does not close the separate
least-privilege/live-target or audit quota requirements.

Corrections found during review included mutable cursor anchor rereads, source
and optional-port denial leaking old available status, incomplete capture showing
fresh complete data, metric pruning deleting current values, one retention class
starving another, repeated health refresh expanding history, and authorization
time captured before a slow request body or transaction lock. These have targeted
real SQL or boundary regressions. At that checkpoint, static demo alarm
compatibility was explicit and durable alert seeding/reads/streams were the next stage.

## Stage 3 follow-up review — 2026-10-16

The locally verified 0.3.0 development checkpoint replaces static alert transport with durable seeded
state, committed replay and reset/refetch. Covered gauge/replica windows, observed
restart increments, raw source outcomes and all bounded scoped events replace
latest-only evidence. Claim generation, rule/source versions, episode state and
database-clock expiry fence completion. Rule edits/deletion resolve the old
episode atomically. A retained prune watermark detects replay holes despite
out-of-order timestamps. Webhook and Matrix lanes preserve canonical bytes and
delivery IDs, exclude synthetic notices and use the same strict outbound policy.

Targeted real PostgreSQL and disposable HTTPS regressions, the final changed-tree
full local gate, disposable native restart/restore and replay drill, and Helm
lint/render passed. Exact results are recorded in the active evidence plan.
Independent reviews score the stage's infrastructure and code quality 9/10;
this score covers the delivered scope. This review does not close full exploration,
least-privilege provider, operator receiver, browser, deployment, guarded-action,
protected audit or release-hardening work. On 2026-10-17 the unchanged 0.3.0
implementation/tag was pushed under renewed authorization and both exact remote
CI runs passed; links and limits are in the evidence plan.

## Correction loop

Stage 4 final review on 2026-10-17 extends the audit across feature-split
transport guards, all exploration pages, PostgreSQL filters/aggregate counts,
exact observation ranges, topology evidence/list navigation, metric time
spacing, bounded paging and activity-based sessions. Regression and real-browser
checks found count truncation, whole-source prefix filtering, denied/stale health
classification, empty target adapter calls, cap truncation, fixed demonstration
time exclusion, query patch races and a narrow-screen toolbar defect. The active
evidence plan records corrections and passed local, PostgreSQL, native and
actual-browser gates. Final graph review also found misleading paths through
intermediate cards; bounded routing, distinct lanes and card-interior regression
checks corrected them. The delivered stage scores 9/10 for architecture and
implementation. This scoped score does not close stages 5–9 or target acceptance.

Each loop records score, remaining defects, fixes and commands in the active
evidence plan. Architecture quality, feature completeness and target deployment
proof are separate. The 1.0 review cannot borrow a narrow stage score to claim
the whole product is finished.

## Stage 1 review result

The corrected baseline received independent scoped9/10 backend, frontend and
PostgreSQL reviews. This score covers the fixes delivered in stage1; whole
product completion and target evidence remain open. Native restore passed on
disposable local PostgreSQL, and the final local gate is recorded in the active
evidence plan. The resolved-current worker robustness item is a future
contract edge case at that checkpoint, not a verified current PostgreSQL recurrence
defect. Stage 3 also supplies a fresh candidate identifier for resolved-current
worker input.

## Stage 5 follow-up review — 2026-10-17

The `0.5.0` candidate adds official Docker/Kubernetes clients behind bounded
adapters, restricted mTLS Docker observation, explicit namespace inventory,
independent coverage publication, typed safe metadata and a deterministic
nine-resource/two-environment guest deployment. Named disposable Engine and
kind tests, guest browser/runtime receipts and real PostgreSQL regressions are
recorded in the active evidence plan. They do not alone close the milestone.

Review found and corrected normal fixture upgrades erasing administrator rules
or re-enabling disabled sources; optional health reads erasing resource identity;
revoked partition evidence entering alert history; partial replica evidence
claiming complete coverage; policy changes exposing cached scopes during outage;
derived reference names surviving a denied parent kind; and logs/usage joining
recreated Pods by reusable names. Log reads now check UID before and after, and
usage requires stable UID and a full sample window inside the observed lifetime.
Any known usage denial closes the metric port; missing usage with configured
facts is stale. Detail metadata keeps separately observed configuration.

Final review lowered the stage score to **8.5/10** for an environment-name
authority gap and an optional metric-port clock regression. The environment
failure was reproduced against real PostgreSQL before correction; the shared
predicate now hides omitted/denied/unsupported names before pagination and in
initial-port inference, while retaining authorized empty environments. Five
focused SQL tests passed. Synthetic collection separately preserves its fixture
clock through in-progress writes and failures, with operational collection
receipts kept distinct. Three final clock/retention/future-sample SQL regressions
passed in 6.295s. A final startup review also reproduced the source-settings
allowlist rejecting the valid restricted Docker profile; its narrow fix passed
source-settings/server tests in 1.581s/6.208s. Final changed-tree gate D then
passed with real PostgreSQL, 36 client tests, 350 web tests and both builds.
Final independent index/scope review assigns **9/10** after these fixes and the
complete gate. Exact publication/CI remain pending; no whole-product score follows.

Stages 6–9 remain open: guarded operations, settings/export/cleanup and protected
audit lifecycle; built-web serving and actual installation/upgrade/restore;
release accessibility/performance/security/PWA; final packaging and independent
1.0 review. Target-version proof, local validation and remote CI remain distinct.
