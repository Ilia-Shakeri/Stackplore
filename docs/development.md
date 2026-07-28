# Development

Stackplore uses one pnpm workspace for browser code and one Go module for the
API. Root tasks keep both parts on the same check path.

## Prerequisites

Install these exact tool versions with a local version manager or an official
archive:

| Tool | Version | Purpose |
| --- | --- | --- |
| Node.js | 24.18.0 LTS | JavaScript runtime and root task runner |
| Corepack | 0.35.0, bundled with Node.js | Fetches the pinned pnpm release |
| pnpm | 11.17.0 | Workspace package manager |
| Go | 1.26.5 | API build and test toolchain |
| Make | Optional | Short aliases for root tasks |

Do not install repository tools globally. Corepack reads the exact pnpm version
from `package.json`. Go reads the preferred patch from `apps/api/go.mod`.

The web package pins TypeScript 6.0.3. TypeScript 7.0.2 does not yet expose the
code API required by the current lint parser, whose supported range ends before
6.1. Keeping one supported compiler avoids a split lint and type-check path.
Revisit this pin in a focused dependency update when the parser supports the
new compiler.

## Bootstrap

Clone the repository, enter its root, and run one command for the current
shell.

PowerShell:

```powershell
node .\scripts\bootstrap.mjs
```

POSIX shell:

```sh
sh scripts/run.sh bootstrap
```

With Make:

```sh
make bootstrap
```

Bootstrap stops when Node.js, pnpm, or Go differs from the pinned version. It
then installs the locked workspace packages and downloads the checksummed Go
modules.

## Environment

Defaults bind both services to loopback. To override them, copy
`.env.example` to `.env` and edit only local values. The root development task
loads `.env` when it exists.

| Variable | Default | Use |
| --- | --- | --- |
| `STACKPLORE_HTTP_ADDRESS` | `127.0.0.1:8080` | Go API listener |
| `STACKPLORE_DEV_API_ORIGIN` | `http://127.0.0.1:8080` | Vite proxy target |

Do not commit `.env`. The example contains no credential or private
infrastructure value. The Phase 1 server rejects a non-loopback listener.

## Run

Start the API and web development servers together:

```powershell
corepack pnpm run dev
```

The web page is available at `http://127.0.0.1:5173`. Vite proxies
`/healthz` and future `/api` paths to the Go service, so the browser keeps one
development origin. The API liveness response is available through either
origin:

```powershell
Invoke-WebRequest http://127.0.0.1:8080/healthz
Invoke-WebRequest http://127.0.0.1:5173/healthz
```

Run one service when focused work needs it:

```powershell
go -C apps/api run ./cmd/server
corepack pnpm --filter @stackplore/web dev
```

## Tasks

Make, PowerShell, and shell commands all delegate to the root package scripts.

| Task | Make | PowerShell | POSIX shell |
| --- | --- | --- | --- |
| Bootstrap | `make bootstrap` | `node .\scripts\bootstrap.mjs` | `sh scripts/run.sh bootstrap` |
| Develop | `make dev` | `corepack pnpm run dev` | `sh scripts/run.sh dev` |
| Full check | `make verify` | `corepack pnpm run verify` | `sh scripts/run.sh verify` |
| All tests | `make test` | `corepack pnpm run test` | `sh scripts/run.sh test` |
| API tests | `make test-api` | `corepack pnpm run test:api` | `sh scripts/run.sh test-api` |
| Web tests | `make test-web` | `corepack pnpm run test:web` | `sh scripts/run.sh test-web` |
| Build | `make build` | `corepack pnpm run build` | `sh scripts/run.sh build` |
| Lint | `make lint` | `corepack pnpm run lint` | `sh scripts/run.sh lint` |
| Type check | `make typecheck` | `corepack pnpm run typecheck` | `sh scripts/run.sh typecheck` |
| Format | `make format` | `corepack pnpm run format` | `sh scripts/run.sh format` |
| Format check | `make format-check` | `corepack pnpm run format:check` | `sh scripts/run.sh format-check` |

The optional PowerShell wrapper maps the same names. Local execution policy may
block direct script files. This process-only form does not change machine or
user policy:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\run.ps1 verify
```

`verify` checks formatting, Go and TypeScript lint rules, strict TypeScript
types, API and web tests, and both builds. Unit tests make no network request.
