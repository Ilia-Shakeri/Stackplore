# Stackplore delivery plan

Target: **1.0.0**. Verified checkpoints: **Stages 1–4, through 0.4.0**. Current work: **Stage 5 provider fidelity, 0.5.0**.
Stage 1 is verified and tagged. Stage 2's core changes passed the full local,
real PostgreSQL, restart and native restore gates; tagged CI is recorded separately.
Stage 3 adds covered windows, durable alert changes and isolated outbound lanes.
Its full local gate with real PostgreSQL, disposable native restart/restore and
HTTPS receiver checks, and Helm lint/render passed. Exact proof is recorded in
the evidence plan; its unchanged implementation and tag were pushed on 2026-10-17
and both exact CI runs passed. Stage 4 passed its final local, real PostgreSQL, native restore and actual browser gates. [Browser evidence](docs/evidence/0.4.0/README.md) is retained. Stages 5–9 remain open. Remaining provider profiles and audit controls stay named
in the roadmap. This checkpoint does not finish 1.0.

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
