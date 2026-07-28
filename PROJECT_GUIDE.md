# Stackplore Project Guide

This guide defines how Stackplore is changed. Product behavior belongs in
[SPEC.md](SPEC.md), delivery order belongs in [PLAN.md](PLAN.md), and detailed
repository rules belong in [docs/conventions.md](docs/conventions.md).

## 1. Current state

Phase 0 product, architecture, security, and governance documents are
committed. Phase 1 provides a runnable web and API developer scaffold with
basic tests and builds. Product workflows remain planned. Available commands
and their verified scope are documented in
[docs/development.md](docs/development.md).

## 2. Delivery contract

Work proceeds through the numbered phases in `PLAN.md`.

For each phase:

1. Read `PROJECT_GUIDE.md`, `SPEC.md`, and `PLAN.md`.
2. When a previous phase exists, confirm it is committed and the worktree is
   clean.
3. Work only on the current phase.
4. Preserve unrelated contributor work.
5. Add or update tests with behavior.
6. Format every changed source file.
7. Run all checks relevant to the phase and all existing regression checks.
8. Inspect the complete phase diff and repository status.
9. Fix regressions caused by the phase.
10. Update truthful documentation and the phase checklist.
11. Prepare one coherent Conventional Commit.
12. Stop at the commit gate before beginning another phase.

Phases are not merged or skipped without a recorded project decision.

Implementation work must not stage, commit, push, tag, rebase, hard-reset, or
rewrite repository history. It must not install global tools. At the commit
gate, report the exact suggested commit and push commands, then wait for the
maintainer to reply `committed, continue`. Repository state is checked again
before the next phase starts.

## 3. Repository layout

The target layout is:

```text
.
├── apps/
│   ├── api/
│   │   ├── cmd/server/
│   │   ├── internal/
│   │   │   ├── auth/
│   │   │   ├── config/
│   │   │   ├── domain/
│   │   │   ├── httpapi/
│   │   │   ├── providers/
│   │   │   │   ├── demo/
│   │   │   │   ├── docker/
│   │   │   │   └── kubernetes/
│   │   │   ├── realtime/
│   │   │   ├── services/
│   │   │   ├── storage/
│   │   │   └── telemetry/
│   │   ├── migrations/
│   │   └── tests/
│   └── web/
│       ├── public/
│       └── src/
│           ├── app/
│           ├── components/
│           ├── features/
│           ├── hooks/
│           ├── i18n/
│           ├── lib/
│           ├── routes/
│           ├── styles/
│           └── test/
├── packages/
│   ├── api-client/
│   └── ui/
├── contracts/
├── deploy/
│   ├── compose/
│   └── helm/stackplore/
├── docs/
│   └── adr/
├── scripts/
└── .github/
```

Directories are introduced only by the phase that owns them. Phase 0 creates
documentation and repository policy only.

## 4. Architecture rules

### 4.1 Backend

- Use Go and the standard library where practical.
- Use Chi only as a thin HTTP router.
- Use `log/slog` for structured logs.
- Pass `context.Context` through I/O and service boundaries.
- Construct dependencies explicitly.
- Keep domain rules independent from HTTP, SQLite, Docker, and Kubernetes.
- Put provider-specific behavior behind capability-aware interfaces.
- Return typed errors and map them once at the HTTP boundary.
- Bound concurrency, queue depth, response size, and retention.
- Close response bodies, streams, watchers, rows, and transactions promptly.
- Do not serialize provider SDK structures directly to clients.

### 4.2 Frontend

- Use strict TypeScript.
- Organize product code by feature.
- Use TanStack Query for server state.
- Use URL state for shareable filters.
- Use component state for local behavior.
- Add a small shared UI state store only after a concrete cross-cutting need.
- Keep API transport inside `packages/api-client`.
- Keep platform access behind small interfaces so a future wrapper does not
  alter domain or feature components.
- Use one icon family and one chart library.
- Wrap accessibility primitives in project-owned UI components.
- Keep route components focused on composition.

### 4.3 Contract

- The versioned HTTP API starts at `/api/v1`.
- `contracts/openapi.yaml` is the canonical transport contract after Phase 4.
- Backend DTOs and the TypeScript client must agree with the contract.
- Provider-neutral fields stay stable.
- Provider details live only in typed and size-bounded metadata.
- Contract-derived output must be reproducible from a documented command.
- A verification check fails when committed contract output is stale.

### 4.4 Storage

- SQLite supports one active API process in version 1.
- Use a repository interface at each domain boundary.
- Keep transactions short.
- Use WAL, a busy timeout, foreign keys, and explicit indexes.
- Run embedded forward-only migrations before serving requests.
- Put retention behind services, not ad hoc SQL in handlers.
- Use SQLite's online backup mechanism rather than copying a live database file.
- Never persist provider credentials or Secret values.

### 4.5 Realtime

- Use SSE for server-to-browser streams.
- Authenticate every stream with the normal session.
- Never put credentials in query strings.
- Use event IDs, keepalive, bounded queues, explicit overflow behavior, and
  immediate cancellation.
- Show disconnected and retry states in the interface.
- A slow client must not block provider collection or another client.

## 5. Security rules

The complete model is in [docs/security.md](docs/security.md).

Minimum rules for every change:

- read-only behavior is the default;
- authorization is enforced by the server;
- unsafe requests require CSRF protection;
- all input is validated at a trust boundary;
- raw provider objects never enter API responses or audit metadata;
- logs and errors reveal no credentials;
- sensitive file references stay inside configured mount roots;
- Kubernetes Secret objects are not read;
- Docker access uses the smallest practical API surface;
- every outbound notification adapter receives the shared SSRF controls;
- PWA caching excludes private and infrastructure data;
- new outbound network behavior is documented;
- security-sensitive behavior has negative tests.

Do not add a default account, credential, bypass, arbitrary command path, or
general provider proxy.

## 6. Dependency policy

- Add a dependency only when it removes more risk or complexity than it adds.
- Use maintained stable releases; no preview channels.
- A stable `v0` module is allowed when the upstream project intentionally uses
  a pre-v1 version line for supported releases. Record the reason and compatible
  release line when first adding it.
- Pin exact frontend tool versions through the workspace and lockfile.
- Pin exact Go modules in `go.mod` and `go.sum`.
- Refresh official metadata immediately before first introducing a dependency.
- Record major architecture dependencies in an ADR.
- Do not add two libraries for one responsibility.
- Review license, release activity, runtime size, transitive tree, security
  history, and platform support.
- Isolate official provider clients behind project interfaces because their
  public APIs may change between minor lines.
- Update dependencies in focused changes with full regression checks.

Initial Phase 1 baseline:

- Node.js 24.18.0 LTS;
- Corepack 0.35.0;
- pnpm 11.17.0;
- Go 1.26.5.

Frontend dependency patches are exact in the workspace manifests and
`pnpm-lock.yaml`. Go dependency patches and checksums are exact in `go.mod` and
`go.sum`.

## 7. Code quality

- Prefer clear names and small cohesive modules.
- Keep functions focused and side effects explicit.
- Avoid hidden global state.
- Avoid `any` outside a documented boundary.
- Avoid giant utility modules and catch-all services.
- Keep handwritten files near 500 lines or less where practical.
- Keep UI components near 250 lines or less where practical.
- Explain constraints and trade-offs, not obvious syntax.
- Delete dead and commented-out code.
- Replace magic values with named configuration or constants.
- Leave no unresolved production TODO in version 1.
- Treat warnings as work, not background noise.

## 8. Testing rules

- Test behavior in the phase that introduces it.
- Prefer deterministic fixtures and controlled clocks.
- Test success, failure, permission, timeout, cancellation, and boundary cases.
- Use mock-backed provider tests for repeatability.
- Mark live provider checks separately.
- Do not claim a live provider works from mock tests.
- Measure coverage with a real command before reporting a value.
- Add a regression test for every confirmed defect when practical.
- Keep browser tests isolated and repeatable.

Available root commands:

```text
make bootstrap
make dev
make verify
make test
make test-api
make test-web
make build
make lint
make typecheck
make format
make format-check
```

Direct PowerShell and POSIX shell equivalents are in
[docs/development.md](docs/development.md). Later phases own end-to-end,
contract, demo, and deployment commands; those commands are not available yet.

## 9. Internationalization and accessibility

- Add English and Persian strings together.
- Keep both locale catalogs in parity.
- Do not hardcode production copy in components.
- Update document language and direction at runtime.
- Isolate technical left-to-right content in Persian layouts.
- Keep status understandable without color.
- Verify keyboard use, focus, labels, headings, dialogs, and live regions.
- Provide list or table alternatives for topology and charts.
- Respect reduced motion.
- Test 360, 768, 1024, 1440, and 1920 pixel widths when a phase changes layout.

## 10. Documentation

- Describe current behavior, not intent, in user-facing feature claims.
- Mark planned features clearly.
- Update both English and Persian documentation when user behavior changes.
- Keep examples free of real credentials and private infrastructure data.
- Keep links relative where practical.
- Record architecture decisions in `docs/adr`.
- Update `CHANGELOG.md` under `Unreleased` during each phase.
- Capture screenshots only from a running deterministic demo.

## 11. Git and review

Follow [docs/conventions.md](docs/conventions.md).

Each phase produces one Conventional Commit. A phase diff must contain only the
phase's work. Before staging:

```text
git status --short
git diff --check
git diff
```

Also inspect every untracked text file for invalid UTF-8, a byte-order mark,
line endings that violate `.gitattributes`, trailing whitespace, and a missing
final newline. After the maintainer stages explicit phase paths, run:

```text
git diff --cached --check
git diff --cached --stat
```

If unrelated work exists, stage only explicit phase paths. Never hide, replace,
or fold unrelated work into a phase commit.

## 12. Phase completion record

A phase report contains:

- implemented behavior;
- each validation command and observed result;
- important files;
- accurate limitations;
- one exact Conventional Commit message;
- exact user-run staging, commit, and push commands;
- a hard stop before the next phase.

Failed checks remain visible until fixed or clearly identified as an external
environment limit.
