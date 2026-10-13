# Stackplore delivery plan

Target: **1.0.0**. Current checkpoint: **Stage 2, durable observations, 0.2.0**.
Stage 1 is verified and tagged. Stage 2's core changes passed the full local,
real PostgreSQL, restart and native restore gates; tagged CI is recorded separately.
Remaining least-privilege profiles and audit controls stay named
in the roadmap. This checkpoint does not finish 1.0.

- [Completion roadmap](docs/roadmap.md) ([فارسی](docs/roadmap.fa.md)) gives ordered versions, scope and exit gates.
- [Active evidence plan](docs/implementation-plan.md) records current commands, outcomes and correction loops.
- [Repository audit](docs/codebase-audit.md) records reviewed defects and coverage.
- [Historical phase ledger](docs/history/phase-plan.md) preserves old foundation records; its SQLite order and stop-before-commit flow are superseded.

The maintainer authorized stage-by-stage implementation, versioning, commits and pushes on 2026-10-12. Preserve unrelated work. Each stage must pass its own named checks and the full local gate before it is called complete. Local, PostgreSQL, browser, container and live-target proof stay distinct. No 1.0 completion claim follows from a development checkpoint.
