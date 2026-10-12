# Runtime configuration

The Go loader in `apps/api/internal/config/config.go` is authoritative. Unknown,
duplicate, blank, non-canonical or out-of-range `STACKPLORE_*` names fail startup
with a safe code, without echoing the rejected value. The service reads process
environment; it does not load `.env` itself. Compose loads its project `.env`.
Never print a resolved Compose environment containing real secrets.

`STACKPLORE_` prefixes every name in the tables. Keep private values in the
operator secret system. [`.env.example`](../.env.example) is a development
template, not production secrets or a promise that every feature is complete.

## Application, trust and logging

| Name | Default | Accepted rule |
| --- | --- | --- |
| `ENVIRONMENT` | `development` | `development`, `production`, `test` |
| `HTTP_ADDRESS` | `127.0.0.1:8080` | Canonical listener host and nonzero port |
| `PUBLIC_ORIGIN` | HTTP plus listener | Exact origin, no path/query/fragment; HTTPS for production beyond loopback |
| `TRUSTED_PROXY_PREFIXES` | none | Canonical comma-separated CIDRs, no spaces; only actual trusted peers |
| `METRICS_ALLOWED_PREFIXES` | loopback IPv4/IPv6 | Canonical CIDRs; exact public origin is also required |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json` or `text` |
| `DATA_DIRECTORY` | `data` | Private local runtime-key directory; production requires a non-root absolute path |
| `DEV_API_ORIGIN` | `http://127.0.0.1:8080` in dev script | Canonical development proxy target; browser development only |

Do not trust whole private network ranges merely because they are private.
Chart example proxy CIDRs must be narrowed to the actual edge network. Browser
request values cannot select filesystem locations or provider endpoints.

## PostgreSQL

| Name | Default | Bounds |
| --- | --- | --- |
| `DATABASE_URL` | Password-free local `stackplore` database | PostgreSQL URL ≤4096 bytes; production explicitly requires `sslmode=verify-full` |
| `DATABASE_MIN_CONNECTIONS` | `2` | At least 1 and no greater than max |
| `DATABASE_MAX_CONNECTIONS` | `32` | 2–256; one connection is reserved for ownership |
| `DATABASE_CONNECT_TIMEOUT` | `5s` | 1s–30s |
| `DATABASE_QUERY_TIMEOUT` | `10s` | 100ms–2m |
| `DATABASE_MIGRATION_TIMEOUT` | `2m` | 5s–15m |
| `DATABASE_HEALTH_TIMEOUT` | `2s` | 100ms–10s |

The pool uses UTC, bounded lifetime/idle settings, immutable migrations and
one active runtime. Ownership loss is terminal and needs controlled restart;
a successful ordinary pool ping does not restore ownership. SQLite runtime
configuration is unsupported after the old adapter is retired.

## HTTP limits

| Name | Default | Bounds |
| --- | --- | --- |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | 1s–30s |
| `HTTP_READ_TIMEOUT` | `15s` | 1s–2m |
| `HTTP_WRITE_TIMEOUT` | `30s` | 1s–5m |
| `HTTP_IDLE_TIMEOUT` | `60s` | 1s–10m |
| `HTTP_REQUEST_TIMEOUT` | `15s` | 100ms–2m; strictly shorter than write timeout |
| `HTTP_SHUTDOWN_TIMEOUT` | `15s` | 1s–2m |
| `HTTP_MAX_HEADER_BYTES` | `32768` | 4096–1048576 |
| `HTTP_MAX_REQUEST_BODY_BYTES` | `262144` | 1024–16777216 |
| `HTTP_MAX_RESPONSE_BYTES` | `16777216` | 65536–67108864 |
| `HTTP_MAX_CONNECTIONS` | `512` | 1–16384 |
| `HTTP_MAX_IN_FLIGHT` | `256` | 1–4096 |
| `RATE_LIMIT_PER_SECOND` | `20` | 0.1–10000, finite |
| `RATE_LIMIT_BURST` | `40` | 1–10000 |
| `RATE_LIMIT_MAX_CLIENTS` | `4096` | 1–65536 |

Authenticated SSE has additional fixed queue, replay, keepalive, client,
lifetime and write bounds in the realtime/HTTP composition. The ordinary
request-buffer deadline does not apply to these bounded streams. Probe and
metrics requests have their documented operational rate-limit exemption.

## Setup and provider authority

| Name | Default | Rule |
| --- | --- | --- |
| `SETUP_BOOTSTRAP_PROOF` | absent | Canonical unpadded base64url 32-byte one-use proof; paired expiry required |
| `SETUP_BOOTSTRAP_EXPIRES_AT` | absent | Canonical UTC RFC3339, at most one hour ahead; must be future on first setup |
| `DOCKER_ENABLED` | `false` | Direct socket is restricted to the explicit loopback development profile |
| `DOCKER_SOCKET_PATH` | `/var/run/docker.sock` | Operator-owned absolute local Unix socket path; no browser path input |
| `KUBERNETES_ENABLED` | `false` | Opt-in mounted service-account collector; current reads require explicit cluster observation rights |
| `KUBERNETES_TOKEN_FILE` | Standard service-account token path | Operator-owned absolute local mount |
| `KUBERNETES_CA_FILE` | Standard service-account CA path | Operator-owned absolute local mount |
| `MUTATIONS_ENABLED` | `false` | Policy flag only; no current live operation route |
| `DOCKER_WRITE_ENABLED` | `false` | Requires Docker observation and global mutation flag |
| `KUBERNETES_WRITE_ENABLED` | `false` | Requires Kubernetes observation and global mutation flag |

Generate setup material using [account recovery/setup](operations/account-recovery.md).
Do not store it in tracked files, command arguments or logs. Mounted provider
credentials are never returned by the product. Namespace-only collection and
the production restricted Docker access profile remain [roadmap](roadmap.md) work.
Policy flags never replace role, CSRF, recent auth, preview, exact version,
one-use confirmation, durable intent and outcome audit checks.

## Generic webhook

| Name | Default | Rule |
| --- | --- | --- |
| `WEBHOOK_ENABLED` | `false` | Must be true when other webhook values are supplied |
| `WEBHOOK_DESTINATION_REFERENCE_ID` | absent | Safe stable bounded logical label, not a URL |
| `WEBHOOK_URL` | absent | Operator-owned HTTPS URL with strict destination policy on every attempt |
| `WEBHOOK_SIGNING_KEY` | absent | Canonical unpadded base64url 32-byte key |

When enabled, all destination fields are required. Browser settings expose
only safe status and guarded one-attempt test delivery. Targets/signing keys
remain operator owned. DNS, IP, redirect, timeout and response-size policy
applies per attempt. Delivery is at-least-once; receivers must deduplicate.

## Deployment-only values

Compose reads `STACKPLORE_COMPOSE_DATABASE_PASSWORD`,
`STACKPLORE_COMPOSE_DATABASE_URL`, and `STACKPLORE_COMPOSE_PORT` (default 8080).
These values belong to Compose, not the Go process loader. The two setup values
pass through from the operator shell; unset values remain absent. Webhook
configuration passes through explicitly. Do not use `docker compose config`
without `--quiet` when real private values are present.

Helm reads [values/schema](../deploy/helm/stackplore/values.schema.json).
`api.setup.enabled=true` reads proof and expiry keys from `api.existingSecret`
for a fresh installation. Disable it after setup. The chart defaults real
Kubernetes reads off; enabling them requests the current cluster reader role.
The external database URL needs verified TLS. API replicas stay exactly one.
