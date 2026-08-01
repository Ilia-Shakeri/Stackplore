# Development

Stackplore uses one pnpm workspace for browser and shared TypeScript packages
and one Go module for the API. Root tasks keep both parts on the same check
path.

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

The server recognizes only the exact names below. An unknown or wrong-case
`STACKPLORE_*` name, duplicate process entry, blank value, surrounding space,
non-canonical value, or out-of-range value stops startup. Errors identify only
the field and failed rule; they do not copy its value.

| Variable | Default when absent | Allowed value or range |
| --- | --- | --- |
| `STACKPLORE_ENVIRONMENT` | `development` | `development`, `production`, or `test` |
| `STACKPLORE_HTTP_ADDRESS` | `127.0.0.1:8080` | Canonical lowercase host or IP plus port `1` through `65535` |
| `STACKPLORE_PUBLIC_ORIGIN` | `http://` plus `STACKPLORE_HTTP_ADDRESS` | One canonical `http` or `https` origin with no user information, path, query, fragment, or redundant default port |
| `STACKPLORE_TRUSTED_PROXY_PREFIXES` | No trusted proxy | Canonical comma-separated IPv4 or IPv6 CIDR prefixes, with no spaces, duplicates, host bits, or `/0` |
| `STACKPLORE_METRICS_ALLOWED_PREFIXES` | `127.0.0.0/8,::1/128` | Canonical comma-separated client CIDR prefixes under the same list rules |
| `STACKPLORE_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `STACKPLORE_LOG_FORMAT` | `json` | `json` or `text` |
| `STACKPLORE_HTTP_READ_HEADER_TIMEOUT` | `5s` | `1s` through `30s`; cannot exceed the read timeout |
| `STACKPLORE_HTTP_READ_TIMEOUT` | `15s` | `1s` through `2m` |
| `STACKPLORE_HTTP_WRITE_TIMEOUT` | `30s` | `1s` through `5m` |
| `STACKPLORE_HTTP_IDLE_TIMEOUT` | `60s` | `1s` through `10m` |
| `STACKPLORE_HTTP_REQUEST_TIMEOUT` | `15s` | `100ms` through `2m`; must be strictly shorter than the write timeout |
| `STACKPLORE_HTTP_SHUTDOWN_TIMEOUT` | `15s` | `1s` through `2m` |
| `STACKPLORE_HTTP_MAX_HEADER_BYTES` | `32768` | `4096` through `1048576` bytes |
| `STACKPLORE_HTTP_MAX_REQUEST_BODY_BYTES` | `262144` | `1024` through `16777216` bytes for `/api/v1` request bodies |
| `STACKPLORE_HTTP_MAX_RESPONSE_BYTES` | `16777216` | `65536` through `67108864` buffered response bytes |
| `STACKPLORE_HTTP_MAX_CONNECTIONS` | `512` | `1` through `16384` accepted HTTP connections |
| `STACKPLORE_HTTP_MAX_IN_FLIGHT` | `256` | `1` through `4096` active handlers |
| `STACKPLORE_RATE_LIMIT_PER_SECOND` | `20` | `0.1` through `10000` tokens per second per verified client on non-operational paths |
| `STACKPLORE_RATE_LIMIT_BURST` | `40` | `1` through `10000` tokens per client bucket |
| `STACKPLORE_RATE_LIMIT_MAX_CLIENTS` | `4096` | `1` through `65536` tracked client buckets before a shared overflow bucket is used |
| `STACKPLORE_DEV_API_ORIGIN` | `http://127.0.0.1:8080` | Canonical Vite-only `http` or `https` upstream origin under the public-origin syntax rules; not used as server trust configuration |

Do not commit `.env`. The example contains no credential or private
infrastructure value. A non-loopback production listener or origin additionally
requires an HTTPS public origin and at least one trusted-proxy prefix. The
server rejects production startup before listening when either gate is absent.

Forwarding headers are not a general deployment escape hatch. On
non-operational paths and `/metrics`, a direct peer outside
`STACKPLORE_TRUSTED_PROXY_PREFIXES` may not send `Forwarded` or any
`X-Forwarded-*` field. The terse health and readiness probes do not use
forwarding identity. A trusted peer still may not send `Forwarded`; it may send
one canonical forwarded scheme, one unambiguous forwarded host, and an IP-only
client chain of at most 16 hops. Its non-operational requests must yield one
verified untrusted client from that chain. Every `/api/v1` request must also
normalize to the exact configured public scheme and authority.

## Run

Start the API and web development servers together:

```powershell
corepack pnpm run dev
```

The web page is available at `http://127.0.0.1:5173`. Vite proxies `/api`,
`/healthz`, `/readyz`, and `/metrics` to the Go service and rewrites the
upstream authority to the configured development API origin. This preserves
the Phase 5 exact-origin gate while the browser uses one development origin.
Probe the direct service with:

```powershell
Invoke-WebRequest http://127.0.0.1:8080/healthz
Invoke-WebRequest http://127.0.0.1:8080/readyz
Invoke-WebRequest http://127.0.0.1:8080/metrics
curl.exe -i http://127.0.0.1:8080/api/v1/system/readiness
```

The final call returns `401` by design. Phase 5 has no user or session system,
so no caller can receive detailed readiness yet.

Run one service when focused work needs it:

```powershell
go -C apps/api run ./cmd/server
corepack pnpm --filter @stackplore/web dev
```

### Operational endpoint meaning

| Path | Current result | Gate and meaning |
| --- | --- | --- |
| `/healthz` | `200` and `ok` | Terse process liveness. It does not check storage, providers, or product data. |
| `/readyz` | `503` and `not ready` before service readiness or during stop; otherwise `200` and `ready` | Terse local readiness. Phase 5 covers validated configuration and the serving lifecycle. Phase 6 adds database readiness. |
| `/metrics` | `200` for an exact-origin, allowed verified client; plain `404` otherwise | Process-only, low-cardinality metrics. The default network allowlist permits IPv4 and IPv6 loopback. |
| `/api/v1/system/readiness` | `401` after all outer request gates pass | Reserved for an enabled administrator. Until Phase 7, every caller is denied and no header, query value, cookie, or proxy claim is a bypass. |

The closed readiness operation documents every reachable Phase 5 problem:
`400` for invalid network metadata or public origin, `401` at the closed
administrator handler, `413` for a declared body over the limit, `429` for a
client-rate denial, `500` for a safely contained internal response failure, and
`503` for boundary unavailability such as request-ID failure, cancellation, or
in-flight overload. It has no success response.

All current responses receive a safe request ID, `no-store`, and fixed security
headers. Every non-operational request requires verified network metadata and
passes the per-client token bucket. Versioned API requests additionally require
the exact public origin. Those outer gates may reject a request before the
closed readiness handler runs.

If secure request-ID generation is unavailable, the operational endpoints keep
serving with the fixed safe fallback `req_unavailable`. Other paths fail closed
with `503`. Each failure increments a metric, while one bounded process log
signals the repeated source fault without creating a log flood.

### Deadline and lifecycle meaning

The server has separate read-header, read, write, idle, request, and shutdown
timeouts plus a ceiling on accepted connections. Current versioned handlers run
behind an in-flight ceiling and write to a bounded response buffer. A panic,
bad status, overflow, cancellation, or total request deadline discards the
partial buffer and returns a safe problem. A known API `Content-Length` over
the body ceiling returns `413` before the handler; any future body-reading
handler is also capped while reading. Client buckets are capped; once the
tracked-client cap is full, new clients share one bounded overflow bucket.
Tracked identities expire after 10 minutes without a request.

An interrupt or termination signal marks readiness false and requests graceful
HTTP drain within the shutdown timeout. It then cancels base request contexts
and force-closes the server if the drain timed out. There is no long-lived HTTP
stream in Phase 5. A future SSE or log-tail route must define its own
connection, idle, write, lifetime, concurrency, queue, and byte limits before
it can receive a narrow exemption from the ordinary buffered request deadline.

Phase 5 checks use deterministic unit tests and local in-process listeners.
They do not prove a deployed TLS edge, external proxy, container image,
orchestrator health check, or public-network installation; those deployment
assets and live checks belong to their owning later phases.

The root web route redirects to `/overview`. Every product route currently
shows an explicit planned or no-data state; it does not simulate provider
results. In development only, `/design-system` exposes the shared controls and
their states for focused browser review.

## Interface development

Shared interface components and semantic CSS tokens live in
`packages/ui/src`. Feature code imports controls from `@stackplore/ui` and
icons from `@stackplore/ui/icons`; it does not import the wrapped primitive
packages directly. Tailwind consumes the same semantic tokens for application
layout.

The theme control supports `light`, `dark`, and `system`. Its preference is
stored locally under `stackplore.theme`, and a small head script resolves it
before the application renders. Inter and Vazirmatn are bundled through the
workspace packages, so the interface makes no runtime font request. Font
credits and shipped license copies are listed in
[`THIRD_PARTY_NOTICES.md`](../THIRD_PARTY_NOTICES.md).

The language control switches between English and Persian without a reload.
Its local preference uses `stackplore.locale`; a separate head script applies
`lang` and `dir` before paint. Paired catalogs live beside the application,
shell, and route features. Add both messages together, use stable namespaced
keys, and run `corepack pnpm run check:locales` to verify key, plural, and
placeholder parity.

Visible components use the locale provider rather than direct
`toLocaleString` calls or assembled sentence fragments. The provider exposes
the shared number, date, and plural formatters. Technical values use the shared
left-to-right isolation component so Persian layout never mutates resource
names, image names, addresses, or identifiers.

When changing a shared component, check keyboard behavior, visible focus,
accessible naming, both color themes, both writing directions, reduced motion,
and layouts at 360, 768, 1024, 1440, and 1920 CSS pixels. Prefer logical CSS
properties. Directional icons and keyboard order must follow the active
writing direction without component forks.

## Contract development

The canonical transport contract is [`contracts/openapi.yaml`](../contracts/openapi.yaml).
It uses OpenAPI 3.1 and the `/api/v1` server prefix. Phase 4 defined the shared
schemas. Phase 5 adds only `GET /system/readiness`, matching the deliberately
closed runtime route. It has no success shape or security scheme until the real
session and administrator guard exist; the contract records `400`, `401`,
`413`, `429`, `500`, and `503`, and the derived output contains request and
problem types but no success type for this operation.

After changing the contract, re-derive the committed browser types and run the
drift gate:

```powershell
corepack pnpm run derive:api-client
corepack pnpm run check:contracts
corepack pnpm run test:client
```

Contract-derived output lives at
`packages/api-client/src/derived/types.derived.ts`. Do not edit it by hand.
`check:contracts` applies the contract linter, repository-specific policy, and
derived-output comparison. Handwritten request policy, problem parsing, and
request-ID handling stay outside the derived directory in
`packages/api-client/src`.

The contract derivation tool is pinned exactly at `0.99.0`. It is a stable
pre-1.0 release, so even a minor update may change its configuration or derived
shape. Refresh it only in a focused dependency change and review the complete
derived diff. The contract linter is also pinned exactly so every environment
applies the same rule set.

## Tasks

Make, PowerShell, and shell commands all delegate to the root package scripts.

| Task | Make | PowerShell | POSIX shell |
| --- | --- | --- | --- |
| Bootstrap | `make bootstrap` | `node .\scripts\bootstrap.mjs` | `sh scripts/run.sh bootstrap` |
| Develop | `make dev` | `corepack pnpm run dev` | `sh scripts/run.sh dev` |
| Full check | `make verify` | `corepack pnpm run verify` | `sh scripts/run.sh verify` |
| All tests | `make test` | `corepack pnpm run test` | `sh scripts/run.sh test` |
| API tests | `make test-api` | `corepack pnpm run test:api` | `sh scripts/run.sh test-api` |
| Client tests | `make test-client` | `corepack pnpm run test:client` | `sh scripts/run.sh test-client` |
| Web tests | `make test-web` | `corepack pnpm run test:web` | `sh scripts/run.sh test-web` |
| Build | `make build` | `corepack pnpm run build` | `sh scripts/run.sh build` |
| Locale check | `make check-locales` | `corepack pnpm run check:locales` | `sh scripts/run.sh check-locales` |
| Contract check | `make check-contracts` | `corepack pnpm run check:contracts` | `sh scripts/run.sh check-contracts` |
| Derive API client | `make derive-api-client` | `corepack pnpm run derive:api-client` | `sh scripts/run.sh derive-api-client` |
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

`verify` checks formatting, catalog parity, contract policy and derived
output, Go and TypeScript lint rules, strict TypeScript types, API, client, and
web tests, and both builds. Unit tests make no network request. The web suite
also covers English and Persian direct routes,
runtime language and theme persistence, locale formatting, bidirectional
keyboard behavior, unknown routes, exact isolated resource parameters,
mobile-drawer focus return, shared primitive behavior, and representative
automated accessibility rules.
