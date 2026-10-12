# Development

## Toolchain

- Node.js `24.18.0` from `.node-version`
- pnpm `11.17.0` through Corepack
- Go `1.26.5` from `apps/api/go.mod`
- PostgreSQL 17 or newer for live persistence checks

Do not substitute a globally different toolchain when reviewing reproducibility. On Windows, invoke npm as `npm.cmd` when npm is needed.

## Bootstrap

```sh
node scripts/bootstrap.mjs
```

Bootstrap checks the pinned tools and installs the locked workspace. It does not create a database, mutate an existing database, or install system packages.

## Local PostgreSQL

The API requires `STACKPLORE_DATABASE_URL`. Development accepts a local password-free role or `sslmode=disable`; production requires an explicit HTTPS public origin and PostgreSQL `sslmode=verify-full`.

Use a disposable database for tests. Never point `STACKPLORE_TEST_POSTGRES_URL` at development, staging, or production data. Integration tests create and drop isolated schemas inside the named database.

Required development values are documented in `.env.example`. Do not commit a populated `.env`.

The Go process does not load `.env` itself. Set `STACKPLORE_DATABASE_URL` in the
operator process before running development. See the complete
[configuration reference](configuration.md). Remove the test-only
`STACKPLORE_TEST_POSTGRES_URL` before starting the application, because that
name belongs to the test runner, not the strict runtime loader.

On Windows, prepare the pinned tools before repository commands:

```powershell
$repoRoot = (Get-Location).Path
$env:PATH = "$repoRoot\.tools\node-v24.18.0-win-x64;$repoRoot\.tools\go\bin;$env:PATH"
$env:CI = 'true'
$env:GOCACHE = "$repoRoot\.tmp\go-build-v1"
npm.cmd run bootstrap
```

## Run

```sh
corepack pnpm run dev
```

This starts:

- the Go API on `http://127.0.0.1:8080`;
- the Vite web server on `http://127.0.0.1:5173`;
- a same-origin development proxy from the web server to the API.

The API exits if configuration, migration history, database ownership, the local digest key, or first-run bootstrap policy is invalid. `/healthz` proves the process can answer. `/readyz` additionally proves current database readiness.

## Verification

The full local gate is:

```sh
corepack pnpm run verify
```

It runs formatting, document inventory/links and version metadata checks, locale parity, OpenAPI policy and derived-client drift, web and Go lint, strict TypeScript checks, Go/client/web tests, and production builds.

Focused commands:

```sh
corepack pnpm run format:check
corepack pnpm run check:locales
corepack pnpm run check:contracts
corepack pnpm run lint
corepack pnpm run typecheck
corepack pnpm run test
corepack pnpm run build
```

Real PostgreSQL integration:

```sh
STACKPLORE_TEST_POSTGRES_URL='postgresql://test-role@127.0.0.1:5432/stackplore_test?sslmode=disable' \
  go -C apps/api test ./...
```

SQL and PostgreSQL-backed HTTP tests skip when the variable is absent. A skip
is not PostgreSQL proof. Fixtures use a common test advisory gate to prevent
different Go packages from contending for the installation ownership lock.
See [the testing matrix](testing.md) and [current evidence](implementation-plan.md).

## Contracts

`contracts/openapi.yaml` is the HTTP authority. After an intentional contract change:

```sh
corepack pnpm run derive:api-client
corepack pnpm run check:contracts
```

The policy accepts only reviewed routes, methods, response codes, security requirements, components, and bounded schemas. Generated types are committed. Runtime browser adapters still parse unknown data because generated static types do not validate network input.

## Backend rules

- Keep domain packages free of transport, database, and provider client imports.
- Validate provider results at the adapter boundary and again against the exact query.
- Bind cursors to normalized query state and guarantee forward progress.
- Keep external errors, raw provider payloads, credentials, and secret values out of public problems and logs.
- Use parameterized SQL, bounded transactions, forward-only migrations, and the shared PostgreSQL advisory locks.
- Run Go tests from `apps/api` or use `go -C apps/api ...`.

## Frontend rules

- Keep strict TypeScript and runtime response guards.
- Add English and Persian keys together; locale parity is a hard gate.
- Test keyboard use, focus, semantic names, LTR and RTL, technical-text isolation, and narrow widths.
- Do not represent a network mutation as successful before its API result.
- Keep synthetic data visibly marked.

## Deployment checks

Static assets can be parsed locally without container tools, but real deployment evidence requires the named tool:

```sh
docker compose --env-file deploy/compose.env.example config --quiet
helm lint deploy/helm/stackplore --strict
```

The CI workflow also builds both images and runs the PostgreSQL integration suite. A workflow file is not proof until a run passes. See [deployment](operations/deployment.md).

## Operator commands

Account recovery:

```sh
go -C apps/api run ./cmd/operator recover-admin --login-name operator --display-name 'Primary Operator'
```

The new password must arrive through standard input. Backup and restore use `cmd/storage`; see [backup/restore](operations/backup-restore.md). Never put passwords or PostgreSQL URLs with credentials in command arguments, shell history, logs, tickets, or commits.

## Evidence labels

Report each proof at its real level:

- unit or static local check;
- local real PostgreSQL integration;
- container build or Compose run;
- cluster chart render/install;
- target environment backup/restore and rollback;
- browser accessibility and performance run.

Do not use one level as proof of another.
