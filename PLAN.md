# Stackplore delivery plan

Target: **1.0.0**. Current gate: **Stage 9 final candidate**. Stages 1–8 are
published checkpoints. Stage 8 `v0.9.0` is on
`20f8c2e30cb2ccdfc16aa266f0b0d5647c429881`; exact main/tag CI, the signed artifact
trial/publication, consumer verification and Windows archive recovery passed.
All 34 public assets were verified; the Windows archive matches the recovery
trial. The final 1.0 gates remain tracked in the [active evidence](docs/implementation-plan.md).

The Stage 9 review corrects stale project/security/development conventions and
removes sensitive authentication fixture values from test failure messages.
Runtime/assertion guards remain unchanged. Current scope, proof and acceptance
are in [the 1.0 evidence](docs/evidence/1.0.0/README.md). Earlier paragraphs below
are chronological checkpoints; this status takes precedence.

Stage 7 is tagged at `64fd4391113cebc53e99bdd09a3d81bbff6054ce`.
[Exact main CI](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39031584134)
passed all six jobs, including actual Compose setup/restart and Helm installation
against separate TLS PostgreSQL. [Exact tag CI](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39032396854)
also passed all six jobs.
The 0.9.0 candidate adds safe offline/update behavior, browser accessibility
checks, measured keyset corrections, mounted Kubernetes profiles, streamed log
downloads, health history, system events and a signed release pipeline. These
changes remain candidates until their exact checks and publication pass.

The paragraphs below retain earlier stage evidence; the status above governs.
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
published checkpoint; corrected exact-revision main CI run 39018504287 passed.
Both exact-revision Stage 5 runs passed. Stage 6 exact main and tag CI passed,
including disposable provider-write jobs. Stage 7 unifies built-web serving and
deployment; its exact main installation gate passed. Stages 8–9 remain open. No checkpoint
finishes 1.0.

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
