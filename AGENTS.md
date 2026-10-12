# Stackplore Repository Rules

## Product boundary

- Stackplore is a self-hosted infrastructure exploration and troubleshooting product.
- Keep observation read-only by default. Never add shell access, exec, an arbitrary Docker or Kubernetes proxy, Secret value reads, or browser-supplied host paths and endpoints.
- Docker, Kubernetes, and Demo providers implement provider-neutral domain ports. Provider SDK values do not cross the transport boundary.
- PostgreSQL is the only supported persistence layer for Stackplore 1.0. Do not add new SQLite behavior or preserve SQLite-only deployment assumptions.

## Source order

When sources disagree, use this order:

1. accepted ADRs that are not superseded;
2. `SPEC.md` product and security rules;
3. `contracts/openapi.yaml` for HTTP behavior;
4. `PLAN.md` and `docs/implementation-plan.md` for delivery state;
5. architecture, security, development, and operations documents;
6. implementation and tests.

Update all affected layers in the same change. Never describe planned behavior as shipped.

## Architecture boundaries

- Backend dependency direction is transport and jobs -> services/use cases -> domain ports <- provider and PostgreSQL adapters.
- Domain packages must not import HTTP, SQL drivers, Docker, Kubernetes, or frontend code.
- Keep business transactions behind use-case-shaped repository methods or explicit units of work.
- All significant I/O accepts `context.Context`. Bound concurrency, queues, streams, reads, pages, retries, and retention batches.
- Keep one modular Go service unless measured evidence proves another process is needed.
- OpenAPI under `/api/v1` is canonical. Regenerate and check TypeScript transport types after contract changes.
- Keep realtime clients isolated with bounded per-client queues, event IDs, keepalive, backpressure policy, lifetime limits, and cleanup.

## PostgreSQL rules

- Use a bounded connection pool with explicit connect, query, transaction, and shutdown timeouts.
- Use parameterized queries, context-aware calls, short transactions, foreign keys, unique constraints, and indexes matched to real query paths.
- Serialize schema migration with a PostgreSQL advisory lock. Refuse unknown or changed migration history.
- Store UTC timestamps with microsecond precision. Preserve opaque identifiers and repository/domain types.
- Test migrations and repositories against real PostgreSQL. Mocks do not prove SQL behavior.
- Use keyset pagination for large mutable sets. Inspect `EXPLAIN (ANALYZE, BUFFERS)` on hot paths before performance claims.
- Prune high-volume history in bounded batches. Audit history has a separate protected lifecycle.
- Production backup and restore use PostgreSQL-native tools and a restore-verification runbook.

## Security rules

- Never print, log, persist, return, copy into tests, or commit passwords, raw tokens, session IDs, provider credentials, private keys, kubeconfig content, Secret values, or full notification targets.
- Preserve Argon2id password hashing, opaque keyed session digests, secure cookie rules, CSRF, origin checks, rate limits, RBAC, session revocation, and final-administrator protection.
- High-risk changes require a purpose-bound one-use recent-auth grant.
- Guarded infrastructure changes require all gates: deployment enablement, separate provider write permission, administrator role, CSRF, recent auth, exact target and version, impact preview, one-use confirmation, durable audit intent, bounded provider call, and result audit.
- Outbound notifications require strict scheme, DNS, IP, redirect, timeout, and response-size checks on every attempt.
- Use safe fixed error codes and bounded metadata. Do not pass browser values to a shell.

## Frontend and interface rules

- Keep React and strict TypeScript. Do not use `any` to bypass a boundary.
- Use the shared interface package and one coherent motion system. Respect `prefers-reduced-motion`.
- Every production feature ships with natural English and Persian strings, locale parity, RTL/LTR checks, and LTR isolation for technical text.
- Meet WCAG 2.2 AA: keyboard access, visible focus, names, semantic regions, live states, non-color status cues, reduced motion, and accessible alternatives for charts and topology.
- Use URL-backed filters for shareable views. Paginate or virtualize large lists and logs. Lazy-load graph and chart code.
- Show loading, empty, stale, unavailable, unsupported, permission-denied, and error states truthfully.

## Verification

On Windows PowerShell, prepare the pinned tools before repository commands:

```powershell
$repoRoot = (Get-Location).Path
$env:PATH = "$repoRoot\.tools\node-v24.18.0-win-x64;$repoRoot\.tools\go\bin;$env:PATH"
$env:CI = 'true'
$env:GOCACHE = "$repoRoot\.tmp\go-build-v1"
```

Use `npm.cmd`, not the PowerShell npm shim.

- Bootstrap: `npm.cmd run bootstrap`
- Development: `npm.cmd run dev`
- Full local gate: `npm.cmd run verify`
- Go tests: `npm.cmd run test:api`
- Web tests: `npm.cmd run test:web`
- Contract drift: `npm.cmd run check:contracts`
- Locale parity: `npm.cmd run check:locales`

Add the smallest relevant tests with each behavior change, then run the full gate before a milestone is called complete. PostgreSQL, container, browser, Docker, Kubernetes, Helm, and external integration claims require their own real proof; the local gate does not imply them.

## Change discipline

- Inspect `git status` first. Preserve unrelated tracked and untracked user work.
- Do not push, rewrite history, force-reset, hide failures, or weaken tests to make a gate green.
- Prefer small cohesive packages and components. Avoid giant services, hidden global state, duplicate libraries, magic values, dead code, and unresolved production TODOs.
- Update docs when behavior or architecture changes. Record exact commands and honest results in the active plan.
