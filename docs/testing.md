# Testing and evidence

Use the pinned tools in [development](development.md). `npm.cmd run verify`
checks format, documentation links/inventory, release metadata, locale parity,
OpenAPI policy/derived types, lint, TypeScript, all local tests, and builds.
Run the smallest regression for a change, then the full gate for a milestone.
Do not weaken tests or hide skips to claim completion.

| Boundary | Required check | Proof limitation |
| --- | --- | --- |
| Domain/auth/provider/service/HTTP | `npm.cmd run test:api` | Most provider tests use fixtures. |
| Browser client | `npm.cmd run test:client` | Network mocks do not prove a deployment. |
| React/locale/interface flows | `npm.cmd run test:web` | DOM tests do not prove geometry, contrast or browsers. |
| HTTP model/client drift | `npm.cmd run check:contracts` | Does not alone compare every runtime parser bound. |
| English/Persian key parity | `npm.cmd run check:locales` | Natural language quality needs review. |
| Documents/version metadata | `npm.cmd run check:docs`, `npm.cmd run check:version` | Link/file checks do not certify factual accuracy. |
| PostgreSQL SQL/migrations/auth concurrency | Test URL plus `npm.cmd run test:api` | Must run against a disposable real server. |
| Native archive/restore | `cmd/storage` on separate fresh databases | Successful archive listing alone is not a restore. |
| Compose/model/images | Compose config, build, start, probes and restart | Definitions or image build alone do not prove startup. |
| Helm/cluster | Lint, render, install, probes, RBAC and TLS | Rendering alone does not prove a pod runs. |
| Webhook | Disposable HTTPS receiver | Signing/SSRF fixtures do not prove receipt. |
| Browser/accessibility/performance | Critical journeys in both locales/themes at required widths | Each needs saved current evidence. |

## Real database

The test role needs schema creation and the permissions used by concurrency
regressions. Use a disposable database with no user data. Tests create unique
schemas and remove only their own fixtures. Never reuse a live application URL.

```powershell
$env:STACKPLORE_TEST_POSTGRES_URL = 'postgresql://test-role@127.0.0.1:5432/stackplore_test?sslmode=disable'
npm.cmd run test:api
```

Without the URL, environment-gated SQL and HTTP integration tests skip. Report
that skip. The [evidence plan](implementation-plan.md) records exact versions,
commands, failures, fixes and proof levels for each delivered version.

The 0.2 observation suite includes real forward upgrades, restart persistence,
generation/version races, coherent logical read snapshots, immutable signed
keysets, source/port permission truth and protected fair retention. Focused SQL
plan output can be reproduced with the same disposable test URL:

```powershell
go -C apps/api test ./internal/postgres -run TestObservationHotPathQueryPlansOnRealPostgreSQL -count=1 -v
```

The fixture uses synthetic and invented native data; warm-cache SQL timings
are not end-to-end API, browser, production or capacity proof.

## Coverage and performance

The specification requires measured meaningful coverage; do not estimate it from
test counts. Collect Go coverage and frontend feature coverage with the owning
release-hardening work. Race tests need a supported Go race toolchain. SQL hot
paths require representative data and `EXPLAIN (ANALYZE, BUFFERS)` before a
performance claim. Browser measurements require actual rendering and large
fixtures; bundle byte counts alone are insufficient.

## Safety and review

Fixtures use invented values. Logs and stored evidence contain no live secrets.
Restore/action tests target disposable resources only. Auth tests may generate
opaque values internally but never print them. Inspect the complete tagged diff
and current documentation together before claiming a release gate passed.
