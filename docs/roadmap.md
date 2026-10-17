# Stackplore completion roadmap

Review date: 2026-10-12. Target: 1.0.0. This is the delivery order for work after
the repository-wide review. Existing foundations are reused; the old numbered
phase ledger is historical. [Implementation evidence](implementation-plan.md)
records commands and results. [Audit](codebase-audit.md) records defects and
coverage. A source definition, test fixture, or deployment manifest is not proof
that its feature works in a live target.

Current status (2026-10-17): stages 1–5 are locally verified development checkpoints
through `0.5.0`. Exact local and remote outcomes are separate in the evidence
plan. Stage 4 passed its local, PostgreSQL, native and actual browser gates;
[retained browser evidence](evidence/0.4.0/README.md) states the scope. Stage 5
is locally verified at `0.5.0` with named disposable Engine/kind,
PostgreSQL and guest-browser receipts and a passing final changed-tree gate.
Exact commit/tag/CI are pending. Stages 6–9 remain open. [Exploration](exploration.md)
([فارسی](exploration.fa.md)), [observations](observations.md) and
[alerts and delivery](alerts-notifications.md) describe current boundaries.

## Version and delivery rules

Each completed stage receives one version in the root and workspace manifests,
HTTP contract metadata, chart metadata, and changelog. A release check rejects
drift. Commit reviewed source, tests, generated transport types, and documents;
create the matching annotated `vVERSION` tag. Keep existing contributor work and
secrets out of accidental edits. The maintainer's explicit
2026-10-17 test/version/commit/push request supersedes the earlier task no-push
restriction. Push only reviewed, tested milestones without rewriting history,
and inspect the exact remote revision's CI. No 1.0 tag is made before all final evidence exists.

Prerelease stages are development checkpoints, with limitations stated in the
changelog and README. No production support promise follows from a 0.x tag.

## Stage 1 — Safe and reproducible baseline — 0.1.0

Status: complete development baseline; exact local/CI proof is in the evidence plan.

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

Status: core checkpoint passed full local and disposable PostgreSQL gates.
Live target and remaining 1.0 work stay named below.

Add use cases and repository ports for persistent safe source references,
environment/resource snapshots, relationship evidence, health and metric history,
event ingestion and fixed operator profile settings. Add bounded discovery jobs,
cancellation, reconnect/backoff and fair ordinary-history retention. Audit has a
protected retain-all lifecycle at this checkpoint. Administrator enable/disable
uses exact target/version preview, expiring one-use confirmation, a `source.update`
recent-auth grant and atomic configuration/audit commit. Browser profile creation
is outside this checkpoint; browser paths/endpoints and credential values are
never accepted. Production restricted
Docker access and namespace-scoped Kubernetes fidelity belong to stage 5;
operator quotas, protected audit cleanup/export and broader settings belong to
stage 6.

Exit: real PostgreSQL repository/transaction tests, restart persistence,
retention boundary tests, audited source changes and one-use recent-auth gates,
reviewed `EXPLAIN (ANALYZE, BUFFERS)` for inventory/history/audit hot paths,
source isolation, interrupted/partial capture, fair retention and authority/
replay fixtures. Keep live provider/target checks separately labeled and open
until their owning stage proves them.

## Stage 3 — Correct alert evidence and live delivery — 0.3.0

Status: complete locally verified development checkpoint. Full local/real
PostgreSQL, disposable native restart/restore, HTTPS receiver and Helm lint/render
proof is recorded in the active evidence plan. Instructions prohibited push
at the time of the local checkpoint. On 2026-10-17 the unchanged tag was
pushed under renewed authorization and both exact CI runs passed. Stage 4 is
also verified; Stage 5 publication/CI and Stages 6–9 remain open.

Delivered covered alert windows and observed restart increments per window,
raw connection-loss evidence and the whole bounded scoped event window. Alert
streams read durable state with replay/reset refetch. Webhook and Matrix lanes
share outbound destination, secret, size, timeout, retry and audit controls,
with separate disposable receiver proof. Targets remain deployment-owned until
an audited safe mutation flow has an accepted contract; the UI cannot overwrite
hidden operator secrets.

Passed exit checks: episode re-fire, unavailable evidence, time window, state transition,
acknowledgement stream, slow-client/replay/reset tests; real PostgreSQL worker
leases and recovery; disposable HTTPS receiver with signature/idempotency,
redirect/SSRF/timeout and uncertain outcome checks.

Implemented semantics and explicit limits are in [alerts and notifications](alerts-notifications.md).
Later exploration/provider receipts are recorded below. Operator receiver,
installation and release acceptance remain separate; disposable HTTPS is not
an operator production claim.

## Stage 4 — Complete exploration journeys — 0.4.0

Status: verified development checkpoint; final local, PostgreSQL, native and
actual-browser gates passed. [Evidence](evidence/0.4.0/README.md) states the scope;
exact publication outcomes are recorded separately in the active plan.

Paginate inventory, topology, alerts/rules, logs, and search; show partial results
honestly. Extend the existing URL source/search synchronization to all shareable
filters. Preserve truthful unavailable/partial metric states and finish source
counts, time range correlation, log tail/pause/wrap/copy/download/container
controls and detail links. Split the
large product/API modules by feature while retaining runtime transport guards.
Complete topology grouping, evidence, correct directions and accessible node/
edge navigation, including isolated nodes. Retain the existing English/Persian
names for all real alert predicates. Keep sessions alive while valid activity
extends their idle limit.

Exit: component/client regressions and real browser setup/login/resource/log/
topology/alert journeys in English/Persian, LTR/RTL, both themes, and all required
widths; keyboard and screen-reader checks; large-fixture budgets.

## Stage 5 — Provider fidelity and deterministic demo — 0.5.0

Locally verified checkpoint; exact publication/CI pending. Official vendor
clients now back restricted mTLS Docker observation and namespace-scoped
Kubernetes discovery, safe typed metadata, health/topology, metrics/events and
bounded historic/current/previous logs. Independently published kind/scope
coverage preserves allowed inventory while hiding denied, unsupported or omitted
cache; environment names use the same current authority before pagination.
Pod logs require stable UID checks around the read, and usage samples require a
stable UID and a full sample window inside the Pod lifetime. Known usage denial
conservatively closes the entire metric port; missing usage with known
configuration is stale, while configuration remains visible in detail metadata.

The `demo-v2` story has nine stable resources in two environments. Isolated guest
mode has separate marked storage, no accounts/sessions, write-route denial and
no outbound delivery. Ordinary same-version Demo restarts preserve durable
acknowledgements; isolated startup resets derived facts and fixture upgrades
preserve protected history and normal administrator choices. See [Docker](docker-provider.md),
[Kubernetes](kubernetes-provider.md) and [Demo](demo.md).

Exit: provider contract suites; real disposable Engine and cluster read proof;
permission-denied and missing Metrics API behavior; stable fixture snapshots,
story browser journey, and guest mode rejection of real sources and writes.
Those scoped receipts are recorded in the active plan. Environment authority,
optional-port clock and restricted-profile startup fixes have scoped passing
regressions and the complete changed-tree gate D passed. Final independent scope
review is 9/10 after the 8.5/10 correction loop. Exact publication/CI are pending;
Stage 6–9 work is not complete.

## Stage 6 — Narrow guarded actions and settings — 0.6.0

Complete the operation repository, confirmation and idempotency records,
purpose-bound one-use reauthentication, durable intent before provider calls,
and separate result audit. Add only the accepted container restart, workload
rollout restart, and supported replica scaling. All deployment permission,
administrator, CSRF, target/version, preview, expiry, replay, timeout and
synthetic-source gates are mandatory. Finish non-secret settings/export/cleanup
previews and personal preferences with bounded audited lifecycle.

Replace audit retain-all with an explicitly reviewed operator quota/minimum
retention/archive policy and guarded cleanup that preserves protected history.
Complete safe operator-owned profile/credential-reference rotation without
storing raw provider credential values.

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
