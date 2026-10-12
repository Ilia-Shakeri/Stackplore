# Stackplore completion roadmap

Review date: 2026-10-12. Target: 1.0.0. This is the delivery order for work after
the repository-wide review. Existing foundations are reused; the old numbered
phase ledger is historical. [Implementation evidence](implementation-plan.md)
records commands and results. [Audit](codebase-audit.md) records defects and
coverage. A source definition, test fixture, or deployment manifest is not proof
that its feature works in a live target.

## Version and delivery rules

Each completed stage receives one version in the root and workspace manifests,
HTTP contract metadata, chart metadata, and changelog. A release check rejects
drift. Commit reviewed source, tests, generated transport types, and documents;
create the matching annotated `vVERSION` tag; push the commit and tag without
force. Keep existing contributor work and secrets out of accidental edits.
The maintainer authorized commits, tags, and pushes for this delivery on
2026-10-12. No 1.0 tag is made before all final evidence exists.

Prerelease stages are development checkpoints, with limitations stated in the
changelog and README. No production support promise follows from a 0.x tag.

## Stage 1 — Safe and reproducible baseline — 0.1.0

Fix confirmed security and correctness blockers in the current read-only
implementation: PostgreSQL ownership and last-administrator serialization,
bounded storage calls, bad-input HTTP replies, source selection isolation,
provider log masking, repeated alert episodes, and Kubernetes container identity.
Repair first-install bootstrap and edge/pod configuration. Verify the existing
implementation before replacing any obsolete storage code. Preserve runtime
key/path protection in a focused package; remove the SQLite adapter, migrations,
dependencies, and license notices only after the real PostgreSQL parity checks
pass. Update contradictory architecture, project, security, and contribution
documents. Add documentation and version checks to the normal local gate.

Exit: full local gate; real PostgreSQL migrations, auth concurrency and repository
checks; native disposable backup/restore; direct regression tests for fixed
defects; reviewed release diff; truthful baseline evidence. Compose, browser,
Helm install, and live infrastructure evidence remain separate named gates.

## Stage 2 — Durable observations and connections — 0.2.0

Add use cases and repository ports for persistent safe source references,
environment/resource snapshots, relationship evidence, health and metric history,
event ingestion, and settings. Add bounded discovery jobs, cancellation,
reconnect, backoff, retention batches, and protected audit lifecycle. Never
accept browser-supplied paths/endpoints or persist credential values. Namespace
selection uses reviewed operator-owned profiles. Restrict production Docker
access through the accepted narrow proxy/authorization design.

Exit: real PostgreSQL repository/transaction tests, restart persistence,
retention boundary tests, audited source changes and one-use recent-auth gates,
reviewed `EXPLAIN (ANALYZE, BUFFERS)` for inventory/history/audit hot paths,
least-privilege fixtures and separately labeled live provider checks.

## Stage 3 — Correct alert evidence and live delivery — 0.3.0

Honor alert windows and restart rate semantics; connection failures must produce
connection-loss evidence. Search the whole bounded event window. Stream durable
alert state rather than provider fixture alerts. Complete replay/reset refetch,
signed webhook attempts, notification setting policy, and receiver verification.
Add the required Matrix adapter through the same outbound destination, secret,
size, timeout, retry and audit controls, with separate disposable receiver proof.
Keep deployment-owned notification targets until an audited safe mutation flow
has an accepted contract. Do not let a UI overwrite hidden operator secrets.

Exit: episode re-fire, unavailable evidence, time window, state transition,
acknowledgement stream, slow-client/replay/reset tests; real PostgreSQL worker
leases and recovery; disposable HTTPS receiver with signature/idempotency,
redirect/SSRF/timeout and uncertain outcome checks.

## Stage 4 — Complete exploration journeys — 0.4.0

Paginate inventory, topology, alerts/rules, logs, and search; show partial results
honestly. Synchronize URL source/filter state with navigation. Finish overview
unavailable/partial metric states, source counts, time range correlation, log
tail/pause/wrap/copy/download/container controls, and detail links. Split the
large product/API modules by feature while retaining runtime transport guards.
Complete topology grouping, evidence, correct directions and accessible node/
edge navigation, including isolated nodes. Name all real alert predicates in
both locales. Keep sessions alive while valid activity extends their idle limit.

Exit: component/client regressions and real browser setup/login/resource/log/
topology/alert journeys in English/Persian, LTR/RTL, both themes, and all required
widths; keyboard and screen-reader checks; large-fixture budgets.

## Stage 5 — Provider fidelity and deterministic demo — 0.5.0

Complete Docker Engine/Compose inventory, current stats, bounded historic logs,
event collection, and evidence-backed relationships. Align collectors with the accepted vendor-client decision. Complete namespace-scoped
Kubernetes discovery, partial/denied capabilities, current/previous container
logs, metrics/events, and topology. Do not fail a whole source when one optional
resource type is denied. Build at least two deterministic demo environments and
the required proxy/frontend/API/worker/PostgreSQL/Redis/monitoring story. Add
explicit guest-demo isolation, fixture version/reset, and no-outbound proof.

Exit: provider contract suites; real disposable Engine and cluster read proof;
permission-denied and missing Metrics API behavior; stable fixture snapshots,
story browser journey, and guest mode rejection of real sources and writes.

## Stage 6 — Narrow guarded actions and settings — 0.6.0

Complete the operation repository, confirmation and idempotency records,
purpose-bound one-use reauthentication, durable intent before provider calls,
and separate result audit. Add only the accepted container restart, workload
rollout restart, and supported replica scaling. All deployment permission,
administrator, CSRF, target/version, preview, expiry, replay, timeout and
synthetic-source gates are mandatory. Finish non-secret settings/export/cleanup
previews and personal preferences with bounded audited lifecycle.

Exit: complete allow/deny matrix, concurrent/replayed operation tests, real
PostgreSQL transaction tests, uncertain timeout reconciliation, viewer/guest
denial, and opt-in disposable action proof. Writes remain disabled by default.

## Stage 7 — Verified installation and operator support — 0.7.0

Align built-web serving with ADR-0001 before release. Run non-root container builds, Compose fresh install and persistence, safe
bootstrap, readiness failure/recovery, native backup/empty-target restore and
upgrade drills. Validate Helm render/lint/schema and install on a disposable
cluster with external PostgreSQL TLS. Finish configuration reference,
troubleshooting, monitoring, least-privilege provider profiles, upgrade,
rollback and incident response runbooks in both documentation languages.

Exit: saved sanitized evidence for images/scans, Compose restart and recovery,
cluster ingress/probes/security/RBAC, upgrade and rollback constraints, database
restore with revoked sessions and representative reads. A render is not an
install; a local database is not a target-production drill.

## Stage 8 — Accessibility, performance and release candidate — 0.9.0

Complete safe PWA shell/offline/update behavior without private API caching.
Measure SQL/API concurrency, inventory, graph, charts, logs and bundles; bound
work and fix measured regressions. Complete WCAG 2.2 AA, responsive/RTL/theme,
reduced motion and failure-state review. Add pinned release workflows,
dependency/secret/image scans, SBOM, checksums and provenance. Capture
screenshots from the real running demo. Reconcile every requirement with code,
tests, docs and current evidence.

Exit: all preceding stages plus browser/accessibility/performance/security and
deployment gates; reviewed release artifacts; no unresolved severe defects or
unimplemented 1.0 requirement hidden behind a completion claim.

## Stage 9 — Final independent review and 1.0.0

Repeat the whole-repository architecture/security/correctness/operations review.
Score architecture and implementation separately from product completion and
target evidence. List remaining defects, fix them, and rerun applicable gates
until the quality score is justified at 9 or 10. Validate the exact tagged tree,
supported versions, complete English/Persian documents, upgrade constraints,
release notes, signatures, SBOM and checksums. Publish only with all evidence.

## Documentation inventory

Required now: specification, active roadmap/evidence plan, architecture and ADRs,
security model/policy, development/contribution guide, API contract, testing
matrix, configuration reference, deployment, account recovery, backup/restore,
troubleshooting, upgrade/rollback, release procedure, changelog, license/notices,
and equivalent English/Persian entry points. Feature/user tutorials and final
screenshots expand alongside their owning stages; future behavior is labeled.
