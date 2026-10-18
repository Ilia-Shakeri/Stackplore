# Stackplore project guide

Product rules live in [SPEC.md](SPEC.md), accepted [ADRs](docs/adr/README.md), and [AGENTS.md](AGENTS.md). The [roadmap](docs/roadmap.md) sets current delivery order; the [evidence plan](docs/implementation-plan.md) records actual results. [Conventions](docs/conventions.md) define formatting and review.

## Current state

The active target remains untagged corrective `1.0.1`. Pushed candidate
`a252718efffbdbb03c8f4552c25e998f481c77bb` is rejected: local gate B and all 14
browser cases passed, but trial `39119787671` failed the strict database-image
scan after its PostgreSQL 17.11 engine probe passed. Fresh analysis found one
CRITICAL and 24 HIGH gosu Go-library findings; all remain blocking. Keep the
[failed receipt](docs/evidence/1.0.1/failed-database-image.json). A new derivative
candidate and its exact acceptance proof remain pending in the
[current release evidence](docs/evidence/1.0.1/README.md).
All nine stages are functionally implemented: secure local accounts, PostgreSQL observations, covered
alerts and durable delivery, full exploration, scoped provider fidelity, guarded
actions, unified installation and safe PWA behavior. Current review score is
8/10 until real corrective PostgreSQL/browser/hosted-deployment and scan gates pass.

Immutable `v1.0.0` remains at `a424d9f4c9f23d25960b93482b3096a6d05d0e1f`.
Its exact main/tag CI, signed trial and Windows consumer recovery passed, but its
release run was canceled after discovering the vulnerable PostgreSQL 17.6
default and omitted database-image scan. The release API confirmed no 1.0.0
GitHub release. The earlier published 0.9.0 prerelease has the same unsafe
database default and is superseded. Keep those receipts and tags intact; they
do not prove the corrective tree. See [checkpoint history](docs/evidence/1.0.0/README.md).
No customer deployment follows from a tag. The [audit](docs/codebase-audit.md)
and [roadmap](docs/roadmap.md) govern delivery scope and review history.

## Delivery contract

1. Inspect Git status and preserve existing contributor files.
2. Read the current source authorities and stage exit criteria.
3. Keep the change cohesive; update behavior, relevant negative tests, contract/client types, both locales and documents together.
4. Use pinned tools; run focused regressions, then the full gate and named real-environment checks.
5. Review the complete diff and untracked files for secrets, runtime artifacts, wrong line endings and unrelated work.
6. Record accurate commands, failures and fixes. Score the current work, list remaining defects and repeat corrections until the quality bar is justified. Keep product completeness distinct.
7. Keep version metadata and CHANGELOG consistent. Follow the maintainer's
   delivery authorization for commits and pushes; tag only the checked source.
   Inspect exact main/tag checks and independently verify signed artifacts.
   Never rewrite published release history or claim an unobserved result.

The old phase stop/maintainer-commit procedure is historical; use the active plan and current instructions. Tools may be prepared locally in ignored directories; do not change unrelated system installations.

## Architecture and data

- One modular Go service, with transport/jobs -> use cases/services -> domain ports <- provider/PostgreSQL adapters. Domain imports no HTTP, SQL driver, provider or frontend packages.
- PostgreSQL is the only supported persistence layer. Use bounded context-aware calls/pools/transactions, ownership before migration, advisory serialized immutable migrations, UTC microseconds, parameters and real SQL tests.
- Require PostgreSQL server/native clients 17.11 or a later 17.x patch. The canonical database image is in `deploy/postgres-image.json`; prove its exact base/derivative and engine/helper identity, full image scan and bound SPDX alongside application/native/source/history scans.
- Build the intended `stackplore-postgres:1.0.1` derivative from the pinned upstream base with unchanged official gosu 1.19 rebuilt using Go 1.26.9. Prove engine/helper versions, root-drop behavior, full-image scan and bound SPDX; package its Docker-save archive and signed SBOM. Compose/CI use owned patched fixtures; Helm retains external PostgreSQL. No image finding may be suppressed or ignored to pass acceptance.
- Keep single-runtime ownership; do not claim horizontal safety from PostgreSQL alone.
- Use provider-neutral identities, explicit capability/completeness states, typed safe metadata, evidence and confidence. Display names are not identity keys.
- Bound requests, pagination, history, streams, queues, retries and retention. Protect audit lifecycle separately.
- Keep observation read-only and credential values server-side. No arbitrary proxy, host-path input, shell, exec, Secret reads or unrestricted manifest/SQL operations.
- Infrastructure writes need every accepted deployment/provider/role/CSRF/recent-auth/target/version/preview/confirmation/audit/timeout gate before they are wired.

## Web and contracts

- OpenAPI under `/api/v1` is canonical; derive transport types after intentional changes and validate unknown runtime responses. Never hand-edit generated types.
- Keep strict TypeScript and shared accessible controls; do not bypass boundaries with `any`.
- Use explicit source-scoped calls, URL-backed filters, cancellable stale reads, paging/virtualization and lazy heavy views.
- Ship English/Persian together, technical-text LTR isolation, keyboard/focus/semantic/live states, graph/chart alternatives, RTL/LTR and reduced-motion checks.
- Do not turn unavailable observations into healthy/zero values or fill real sources with synthetic fixtures.

## Verification and operations

Use [development](docs/development.md), [configuration](docs/configuration.md), [testing](docs/testing.md) and [release procedure](docs/releasing.md). Every deployment, restore, browser and live-provider claim requires its own direct evidence. [Document inventory](docs/README.md) links current operator and user guides. Secrets never enter logs, commits, test output, issue examples or public audit metadata.
