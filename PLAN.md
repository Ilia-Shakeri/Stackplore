# Stackplore delivery plan

Target: **1.0.1**. **All nine stages are functionally implemented; corrective
runtime, deployment, scan and publication acceptance remain open.**

Withheld checkpoint: `a424d9f4c9f23d25960b93482b3096a6d05d0e1f`, immutable `v1.0.0`.
[Exact main CI](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39116327391)
and [exact tag CI](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39117205748)
passed all six jobs. The signed trial passed independent consumer verification
and actual Windows archive recovery. These passing receipts retain their scope.
The
[release run](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39117205810)
was canceled after finding PostgreSQL 17.6 in deployment/CI and no database-image
scan. The release API confirmed no `v1.0.0` GitHub release. The tag will not move.
The published 0.9.0 prerelease also has the unsafe database default and is
superseded. [Corrective 1.0.1 acceptance](docs/evidence/1.0.1/README.md) now governs
delivery; the [1.0.0 receipt](docs/evidence/1.0.0/README.md) preserves history.

| Stage | Version | Delivered outcome |
| --- | --- | --- |
| 1 | 0.1.0 | Complete: safe ownership/migrations, final administrator, bounded calls, invalid-input handling, masking, source selection and reproducible baseline. |
| 2 | 0.2.0 | Complete: durable PostgreSQL observations and safe source profiles, bounded collection/retention, truthful authority and audited source settings. |
| 3 | 0.3.0 | Complete: covered alert evidence/lifecycle, durable replay and fenced opt-in webhook/Matrix delivery. |
| 4 | 0.4.0 | Complete: paginated exploration, URL filters, log controls, resource details, metric truth, accessible topology and bilingual browser journeys. |
| 5 | 0.5.0 | Complete: official scoped Docker/Kubernetes adapters, restricted mTLS, independent coverage, safe typed facts, stable log/metric identity and isolated deterministic guest demo. |
| 6 | 0.6.0 | Complete: narrowly guarded restart/scale, durable intent/result audit, settings, preferences, quota/archive-bound audit retention and bounded cleanup. |
| 7 | 0.7.0 | Complete: one non-root embedded application, actual Compose setup/restart, Helm installation against external TLS PostgreSQL and bilingual operator runbooks. |
| 8 | 0.9.0 | Functionally complete: PWA, redacted logs, health history, filters, measured interface/performance and signed publication passed their recorded checks. Superseded because its database default is unsafe and database-image scanning was absent. |
| 9 | 1.0.1 | Functionality implemented. The immutable 1.0.0 checkpoint passed its recorded local/browser/main/tag/trial/recovery checks but was withheld. Corrective runtime, deployment, full scan and final artifact acceptance are pending. |

The current correction-loop score is **8/10**. Earlier scoped 9/10 reviews
predate the database-image gap. The corrective tree pins PostgreSQL 17.11 using
the canonical [database policy](deploy/postgres-image.json) and adds engine,
OS-package and SPDX gates; configured gates are not observed proof. Return to
9/10 only after the exact corrective real-PostgreSQL, browser, hosted deployment
and strict scan gates pass. The [requirement matrix](docs/evidence/0.9.0/requirements.md)
maps accepted areas to source, tests, guides and proof limits. Scores remain
review judgments, separate from measured coverage and runtime evidence.

Remaining gates include real PostgreSQL 17.11 and native backup/empty-target
restore proof, the full changed-tree local/browser matrix, all six hosted CI jobs,
database engine/digest and OS/SPDX scans, the signed trial and actual Windows
1.0.1 recovery. Then tag the checked corrective source, publish, and independently
verify all public assets and exact identity. The new expected counts are 37
assets, 29 checksums, 15 provenance subjects and six archive/image SBOM
attestations; none is yet a 1.0.1 receipt. No accepted feature is deferred into
these acceptance gates.

Customer production acceptance remains an operator task. Credentials, provider
rights, ingress/TLS, receiver reachability, backups and capacity require the
customer environment's own checks. A published release does not certify that
installation. Runtime evidence covers Windows amd64, Linux amd64 and the Linux
amd64 image with PostgreSQL 17; other native outputs are cross-built only.

- [Completion roadmap](docs/roadmap.md) ([فارسی](docs/roadmap.fa.md)) records scope and exit gates.
- [Active evidence](docs/implementation-plan.md) retains exact commands, outcomes, failures and corrections.
- [Repository audit](docs/codebase-audit.md) and [corrective review](docs/evidence/1.0.1/final-review.md) record reviewed boundaries and the open score.
- [Documentation inventory](docs/README.md) links current user, development and operator guides.
- [Historical phase ledger](docs/history/phase-plan.md) preserves superseded foundation planning.

The maintainer authorized stage-by-stage testing, versioning, commits and pushes
on 2026-10-17. Existing tags and failed receipts remain intact. Unrelated user
artifacts stay outside release commits.
