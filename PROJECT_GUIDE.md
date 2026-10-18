# Stackplore project guide

Product rules live in [SPEC.md](SPEC.md), accepted [ADRs](docs/adr/README.md), and [AGENTS.md](AGENTS.md). The [roadmap](docs/roadmap.md) sets current delivery order; the [evidence plan](docs/implementation-plan.md) records actual results. [Conventions](docs/conventions.md) define formatting and review.

## Current state

The current code is the 1.0.0 final candidate. Earlier checkpoints implement
secure local accounts, PostgreSQL observations, covered alerts and durable
delivery, full exploration, scoped provider fidelity, guarded actions, unified
installation and safe PWA behavior. Stage 8's 0.9.0 tag identifies
`20f8c2e30cb2ccdfc16aa266f0b0d5647c429881`; its exact main CI and signed artifact
trial passed. Its exact tag CI, publication and independent public artifact
verification also passed. Final 1.0 gates are recorded separately in the
[evidence plan](docs/implementation-plan.md). No customer deployment follows
from a tag. [Audit](docs/codebase-audit.md) and [roadmap](docs/roadmap.md) govern
current delivery state.

## Delivery contract

1. Inspect Git status and preserve existing contributor files.
2. Read the current source authorities and stage exit criteria.
3. Keep the change cohesive; update behavior, relevant negative tests, contract/client types, both locales and documents together.
4. Use pinned tools; run focused regressions, then the full gate and named real-environment checks.
5. Review the complete diff and untracked files for secrets, runtime artifacts, wrong line endings and unrelated work.
6. Record accurate commands, failures and fixes. Score the current work, list remaining defects and repeat corrections until the quality bar is justified. Keep product completeness distinct.
7. Set one consistent stage version, update CHANGELOG, commit and annotate the
   exact version tag only after the milestone gate passes. The maintainer's
   explicit 2026-10-17 request authorizes testing, versioning, commits and pushes
   for every remaining stage and supersedes the earlier no-push task rule.
   Inspect exact main/tag checks and signed artifacts. Never rewrite release
   history or claim an unobserved remote result.

The old phase stop/maintainer-commit procedure is historical; use the active plan and current instructions. Tools may be prepared locally in ignored directories; do not change unrelated system installations.

## Architecture and data

- One modular Go service, with transport/jobs -> use cases/services -> domain ports <- provider/PostgreSQL adapters. Domain imports no HTTP, SQL driver, provider or frontend packages.
- PostgreSQL is the only supported persistence layer. Use bounded context-aware calls/pools/transactions, ownership before migration, advisory serialized immutable migrations, UTC microseconds, parameters and real SQL tests.
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
