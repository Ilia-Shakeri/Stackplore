# Repository review — 2026-10-12

Scope: all first-party backend, frontend, interface/client packages, contracts,
migrations, deployment, workflows, scripts, configuration, tests, and documents.
Generated types were checked against their source rather than treated as an
independent design. Third-party bundles and local caches are excluded. The
review began at `08028f4` with a substantial uncommitted implementation; those
files were preserved and reviewed as part of the candidate baseline.

Initial quality: 5/10. This is a review score, not measured coverage. Version 1.0
completion is blocked by the unfinished stages in [the roadmap](roadmap.md).

## Verified defects at review entry

| Priority | Entry reference | Defect and consequence | Delivery |
| --- | --- | --- | --- |
| P1 | `internal/postgres/auth_repository.go:1167` | Final-admin check lacks a shared transaction lock; concurrent changes can remove both admins. | Stage 1 |
| P1 | `internal/postgres/store.go:85` | Migrations run before process ownership; rejected contender can change live schema. | Stage 1 |
| P1 | `internal/postgres/store.go:117` | Pool health misses lost ownership connection; old runtime can outlive its lock. | Stage 1 |
| P1 | `internal/httpapi/product_handlers.go:171` | Invalid query/body paths return without a problem, producing empty 200 replies. | Stage 1 |
| P1 | `internal/provider/docker/provider.go:442`, `internal/provider/kubernetes/provider.go:648` | Space-delimited log mask can leave bearer/structured secret values visible. | Stage 1 |
| P1 | `internal/provider/kubernetes/provider.go:1020` | Repeated container names across pods collide and reject inventory. | Stage 1 |
| P1 | `compose.yaml:32`, `cmd/server/main.go:279` | Fresh non-loopback API bind requires bootstrap proof absent in deployment inputs. | Stage 1 |
| P1 | `internal/provider/kubernetes/api_collector.go:67` | Global mandatory reads prevent namespace-only and partial-permission observation. | Stage 2/5 |
| P2 | `features/product/product-context.tsx:39`, `ProductPages.tsx:996` | Deferred mutable source change races child requests/direct details. | Stage 1 |
| P2 | `internal/alerting/worker.go:224` | Resolved-current worker contract lacks candidate ID; the current PostgreSQL claim excludes resolved rows and avoids this path. | Stage 1 |
| P2 | `internal/notification/webhook.go:136` | Per-attempt HTTP transport leaves idle sockets behind. | Stage 1 |
| P2 | `internal/postgres/store.go:223` | One connection is accepted though ownership reserves it; startup waits for itself. | Stage 1 |
| P2 | `internal/postgres/auth_repository.go:49`, `transaction.go:51` | Parent context bypasses configured total repository/transaction bounds. | Stage 1 |
| P2 | `internal/postgresops/operations.go:149` | Native backup/restore has no command deadline. | Stage 1 |
| P2 | `deploy/helm/stackplore/templates/web-deployment.yaml:29` | Root-owned writable mounts prevent non-root Nginx start. | Stage 1 |
| P2 | `deploy/helm/stackplore/templates/web-configmap.yaml:19` | TLS ingress hop becomes forwarded HTTP and fails public-origin checks. | Stage 1 |
| P2 | `internal/alerting/provider_evidence.go:74,91,120` | Connection loss skips evaluation; windows/rates ignored; first event hides matching history. | Stage 3 |
| P2 | `internal/httpapi/product_stream_handlers.go:319` | Alert stream uses fixture alerts instead of durable state. | Stage 3 |
| P2 | `features/product/product-api.ts:966`, `AlertRulesSettingsPage.tsx:208` | Inventory/rules stop at first page; topology also truncates. | Stage 4 |
| P2 | `features/product/ProductPages.tsx:1200` | Missing metrics become zero totals. | Stage 1/4 |
| P2 | `features/auth/auth-store.ts:104` | Local idle timer ignores backend activity extension. | Stage 4 |
| P2 | `features/product/product-stream.ts:37` | Replay reset marks stale without refetch. | Stage 3 |
| P2 | `features/product/TopologyGraph.tsx:216` | Bidirectional/undirected edges are drawn as one-way. | Stage 4 |
| P2 | `features/product/ProductPages.tsx:250,1317` | Accessible graph omits node links/isolated nodes; real predicate labels fall back. | Stage 4 |
| P1 | `internal/postgres/notification_delivery.go:15` | Outbox sequence allocation can commit below an advanced cursor and lose delivery. | Stage 1 |
| P2 | `features/product/product-api.ts:957` | Backend up to300 metric items conflicts with max200 transport guard. | Stage 1 |
| P2 | `internal/httpapi/product_observability_handlers.go:179,325` | Live log/event next-page time range shifts and invalidates the cursor. | Stage 2/4 |
| P2 | `docs/adr/0001:serving`, `docs/adr/0002:clients` | Separate web runtime/custom collectors differ from accepted built-web/vendor-client targets. | Stage 5/7 |
| P2 | `PROJECT_GUIDE.md:9`, `SPEC.md:runtime`, `CHANGELOG.md` | Current implementation is described as old phases/SQLite or missing. | Stage 1 |

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

Major unfinished scope after the 0.2 checkpoint: source creation through reviewed
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
real SQL or boundary regressions. Static demo alarm compatibility is explicit;
durable alert seeding/reads/streams remain the next stage.

## Correction loop

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
contract edge case, not a verified current PostgreSQL recurrence defect.
