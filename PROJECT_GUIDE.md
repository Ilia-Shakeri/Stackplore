# Stackplore project guide

Product rules live in [SPEC.md](SPEC.md), accepted [ADRs](docs/adr/README.md), and [AGENTS.md](AGENTS.md). The [roadmap](docs/roadmap.md) sets current delivery order; the [evidence plan](docs/implementation-plan.md) records actual results. [Conventions](docs/conventions.md) define formatting and review.

## Current state

The accepted corrective release is **1.0.2**, immutable `v1.0.2` at
`376ed0379900ab870dd8eb8af6b55ef3e5ec389b`, schema **17**. All **nine stages**
are complete; architecture/code and release review score **9/10**. Full local D,
all six exact-source main/tag jobs, signed trial/publication and independent
public checksum/provenance/SBOM/archive checks pass. Failed first tag/browser
and blocked-release attempts retain their outcomes; unchanged-tag retry passed.
[Published proof and runtime limits](docs/evidence/1.0.2/published-release.json).

The immutable 1.0.1 source and its [published receipt](docs/evidence/1.0.1/published-release.json)
retain their original results and scope. Those older checks do not certify a different source. Customer production and air-gapped installation require separate
operator evidence.

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
- Build the reviewed `stackplore-postgres:1.0.2` derivative from the pinned upstream base with unchanged official gosu 1.19 rebuilt using Go 1.26.9. Prove engine/helper versions, root-drop behavior, full-image scan and bound SPDX; package its Docker-save archive and signed SBOM. Compose/CI use owned patched fixtures; Helm retains external PostgreSQL. No image finding may be suppressed or ignored to pass acceptance.
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
