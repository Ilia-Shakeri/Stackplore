# Stackplore delivery plan

Target: **1.0.0**. Locally verified checkpoints: **Stages 1–5, through 0.5.0**. Current gate: **Stage 5 publication and exact CI pending; Stages 6–9 open**.
Stage 1 is verified and tagged. Stage 2's core changes passed the full local,
real PostgreSQL, restart and native restore gates; tagged CI is recorded separately.
Stage 3 adds covered windows, durable alert changes and isolated outbound lanes.
Its full local gate with real PostgreSQL, disposable native restart/restore and
HTTPS receiver checks, and Helm lint/render passed. Exact proof is recorded in
the evidence plan; its unchanged implementation and tag were pushed on 2026-10-17
and both exact CI runs passed. Stage 4 passed its final local, real PostgreSQL, native restore and actual browser gates. [Browser evidence](docs/evidence/0.4.0/README.md) is retained.
The Stage 5 candidate adds official provider clients, restricted Docker mTLS,
namespace-scoped coverage and cache authority, typed safe facts, immutable Pod
log/metric checks and the nine-resource, two-environment isolated guest demo.
Real disposable Engine and kind receipts, PostgreSQL regressions and 24 loaded
guest browser views are recorded in the evidence plan. Final review found an
environment-name authority gap and an optional metric-port clock regression;
environment and clock fixes passed real SQL tests. A final restricted-profile
startup allowlist omission was reproduced and corrected with scoped tests.
The final changed-tree gate D passed with real PostgreSQL, 36 client tests,
350 web tests and both builds. Independent scoped review is 9/10. Stage 5 is a
locally verified checkpoint; its tag, push and remote CI are not yet claimed.
Stages 6–9 remain open. This checkpoint does not finish 1.0.

- [Completion roadmap](docs/roadmap.md) ([فارسی](docs/roadmap.fa.md)) gives ordered versions, scope and exit gates.
- [Active evidence plan](docs/implementation-plan.md) records current commands, outcomes and correction loops.
- [Repository audit](docs/codebase-audit.md) records reviewed defects and coverage.
- [Historical phase ledger](docs/history/phase-plan.md) preserves old foundation records; its SQLite order and stop-before-commit flow are superseded.

The maintainer authorized stage-by-stage implementation, versioning and commits.
The maintainer explicitly authorized test/version/commit/push of each remaining
stage on 2026-10-17. This supersedes the earlier task no-push restriction.
Preserve unrelated work.
Each stage must pass its own named checks and the full local gate before it is
called complete. Local, PostgreSQL, browser, container and live-target proof stay
distinct. No 1.0 completion claim follows from a development checkpoint.
