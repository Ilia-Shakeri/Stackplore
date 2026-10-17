# Runtime configuration

The Go loader in `apps/api/internal/config/config.go` is authoritative. Unknown,
duplicate, blank, non-canonical or out-of-range `STACKPLORE_*` names fail startup
with a safe code, without echoing the rejected value. The service reads process
environment; it does not load `.env` itself. Compose loads its project `.env`.
Never print a resolved Compose environment containing real secrets.

The development launcher `scripts/dev.mjs` loads the root `.env` into process
environment before starting Go and Vite. Use `.env.example` for that workflow.
A Compose `.env` contains `STACKPLORE_COMPOSE_*` names that the strict Go loader
rejects; it cannot be reused unchanged for development. Direct Go/operator
commands require their values in process environment.

The Go loader's `PUBLIC_ORIGIN` default below is the API listener. The development
launcher supplies `http://127.0.0.1:5173` when the operator has not set it, matching
the browser-facing Vite origin. Vite preserves Host and Origin when forwarding.
When starting each process directly, set `STACKPLORE_PUBLIC_ORIGIN` to the web
origin and `STACKPLORE_DEV_API_ORIGIN` to the API listener. Explicit operator values
take precedence; an authority mismatch is rejected rather than bypassed with CORS.

`STACKPLORE_` prefixes every name in the tables. Keep private values in the
operator secret system. [`.env.example`](../.env.example) is a development
template, not production secrets or a promise that every feature is complete.

## Application, trust and logging

| Name                       | Default                               | Accepted rule                                                                     |
| -------------------------- | ------------------------------------- | --------------------------------------------------------------------------------- |
| `ENVIRONMENT`              | `development`                         | `development`, `production`, `test`                                               |
| `HTTP_ADDRESS`             | `127.0.0.1:8080`                      | Canonical listener host and nonzero port                                          |
| `PUBLIC_ORIGIN`            | HTTP plus listener                    | Exact origin, no path/query/fragment; HTTPS for production beyond loopback        |
| `TRUSTED_PROXY_PREFIXES`   | none                                  | Canonical comma-separated CIDRs, no spaces; only actual trusted peers             |
| `METRICS_ALLOWED_PREFIXES` | loopback IPv4/IPv6                    | Canonical CIDRs; exact public origin is also required                             |
| `LOG_LEVEL`                | `info`                                | `debug`, `info`, `warn`, `error`                                                  |
| `LOG_FORMAT`               | `json`                                | `json` or `text`                                                                  |
| `DATA_DIRECTORY`           | `data`                                | Private local runtime-key directory; production requires a non-root absolute path |
| `DEV_API_ORIGIN`           | `http://127.0.0.1:8080` in dev script | Canonical development proxy target; browser development only                      |

Do not trust whole private network ranges merely because they are private.
Chart example proxy CIDRs must be narrowed to the actual edge network. Browser
request values cannot select filesystem locations or provider endpoints.

## Provider and demonstration profiles

`STACKPLORE_` prefixes these names. Every endpoint and file reference is selected
by the operator at startup; browser requests cannot supply one.

| Name | Default | Accepted rule |
| --- | --- | --- |
| `DOCKER_ACCESS_PROFILE` | `docker_socket_dev` | Development socket or `docker_restricted_proxy`; the socket profile requires loopback development |
| `DOCKER_PROXY_ORIGIN` | absent | One exact HTTPS origin for the restricted proxy; no user info, path, query or fragment |
| `DOCKER_PROXY_CA_FILE` | absent | Absolute operator CA reference; required with the restricted proxy |
| `DOCKER_PROXY_CERT_FILE` | absent | Absolute operator client certificate reference; required with the restricted proxy |
| `DOCKER_PROXY_KEY_FILE` | absent | Absolute private client key reference; required with the restricted proxy |
| `KUBERNETES_NAMESPACES` | absent | Required when Kubernetes is enabled: one to 64 explicit DNS namespace labels, comma-separated without spaces |
| `KUBERNETES_CLUSTER_OBSERVATION` | `false` | Separate opt-in global observation; namespace lists still stay inside the allowlist |
| `KUBERNETES_ACCESS_PROFILE` | `kubernetes_incluster` | In-cluster service account or read-only `kubernetes_kubeconfig` |
| `KUBERNETES_KUBECONFIG_FILE` | absent | Absolute mounted kubeconfig path below one approved reference root; required for kubeconfig profile |
| `KUBERNETES_CONTEXT` | kubeconfig current context | Optional exact context name; maximum 256 bytes |
| `KUBERNETES_REFERENCE_ROOTS` | absent | One to eight absolute mounted directories; OS path-list separator (`:` on Linux, `;` on Windows); no filesystem root |
| `DEMO_DEPLOYMENT` | `false` | Isolated synthetic deployment; rejects real providers, provider credentials, outbound settings and infrastructure writes |
| `GUEST_DEMO` | `false` | Public named product GET routes only; requires isolated demo deployment |
| `DEMO_DATA_DIRECTORY` | absent | Required isolated absolute private directory, separate from `DATA_DIRECTORY`; an ordinary or unmarked nonempty directory is refused |

Certificate and credential files are bounded regular files. Projected credential
symlinks may resolve within their selected mount directory; escaping links,
directories and special files are refused. TLS verifies the exact configured
hostname and CA; proxy environment variables and redirects are disabled.
Projected Kubernetes tokens refresh without persisting their values.

The mounted kubeconfig profile rejects all symlinks, including internal links,
and permits only file references under the explicit roots. The kubeconfig and
CA/certificate/key files are capped at 1 MiB each; token files at 16 KiB. It accepts
one exact HTTPS cluster origin and either a token-file or paired client
certificate/key references. Inline credential/CA data, authentication plugins,
basic authentication, proxy/impersonation settings, extensions and insecure TLS
are refused, including in unused contexts. The same namespace and transport
endpoint allowlists still apply. This observation profile cannot enable a writer;
do not reuse its reader credential for infrastructure operations. Restart after
changing a certificate, CA, selected context or endpoint. Selected token-file
credentials use bounded refresh. See the [mounted-profile example](kubernetes-provider.md#mounted-kubeconfig-and-context).

`STACKPLORE_LOG_REDACT_FIELDS` optionally adds up to 32 comma-separated structured
log field names to built-in masking. Names start with an ASCII letter and contain
at most 64 letters/digits/underscore/hyphen bytes, without spaces. Matching and
duplicate detection ignore case, underscores and hyphens. The policy applies to
pages, downloads and live logs; it cannot disable built-in credential masking.
Leave the key absent for the built-in policy. These field names are configuration,
never values to redact or credentials.
Compose passes this optional operator environment variable through without
inventing an empty value. The Helm equivalent is `api.logRedactFields`, an
array of the same field names; an empty array omits the setting. Restart or
roll out the application after changing the policy.

The application has no Docker socket in the restricted profile. Its mTLS proxy
admits only reviewed bounded observation requests. Write permission remains a
separate deployment concern and cannot be enabled through that observation
proxy. [Docker](docker-provider.md) and [Kubernetes](kubernetes-provider.md) give
the exact supported endpoints, limits and configuration examples.

An isolated demo must use a fresh separate PostgreSQL database. The persisted
deployment mode rejects an ordinary database even if its current resources are
synthetic. Never reuse an ordinary Compose environment for guest mode: forbidden
credential keys are rejected even when empty. Guest mode needs no bootstrap
proof and offers no account setup, session or mutation route. The
[demo guide](demo.md) explains the launch and reset contract.

## PostgreSQL pool

| Name                         | Default                                   | Bounds                                                                           |
| ---------------------------- | ----------------------------------------- | -------------------------------------------------------------------------------- |
| `DATABASE_URL`               | Password-free local `stackplore` database | PostgreSQL URL ≤4096 bytes; production explicitly requires `sslmode=verify-full` |
| `DATABASE_MIN_CONNECTIONS`   | `2`                                       | At least 1 and no greater than max                                               |
| `DATABASE_MAX_CONNECTIONS`   | `32`                                      | 2–256; one connection is reserved for ownership                                  |
| `DATABASE_CONNECT_TIMEOUT`   | `5s`                                      | 1s–30s                                                                           |
| `DATABASE_QUERY_TIMEOUT`     | `10s`                                     | 100ms–2m                                                                         |
| `DATABASE_MIGRATION_TIMEOUT` | `2m`                                      | 5s–15m                                                                           |
| `DATABASE_HEALTH_TIMEOUT`    | `2s`                                      | 100ms–10s                                                                        |

The pool uses UTC, bounded lifetime/idle settings, immutable migrations and
one active runtime. Ownership loss is terminal and needs controlled restart;
a successful ordinary pool ping does not restore ownership. SQLite runtime
configuration is unsupported after the old adapter is retired.

## HTTP limits

| Name                          | Default    | Bounds                                        |
| ----------------------------- | ---------- | --------------------------------------------- |
| `HTTP_READ_HEADER_TIMEOUT`    | `5s`       | 1s–30s                                        |
| `HTTP_READ_TIMEOUT`           | `15s`      | 1s–2m                                         |
| `HTTP_WRITE_TIMEOUT`          | `30s`      | 1s–5m                                         |
| `HTTP_IDLE_TIMEOUT`           | `60s`      | 1s–10m                                        |
| `HTTP_REQUEST_TIMEOUT`        | `15s`      | 100ms–2m; strictly shorter than write timeout |
| `HTTP_SHUTDOWN_TIMEOUT`       | `15s`      | 1s–2m                                         |
| `HTTP_MAX_HEADER_BYTES`       | `32768`    | 4096–1048576                                  |
| `HTTP_MAX_REQUEST_BODY_BYTES` | `262144`   | 1024–16777216                                 |
| `HTTP_MAX_RESPONSE_BYTES`     | `16777216` | 65536–67108864                                |
| `HTTP_MAX_CONNECTIONS`        | `512`      | 1–16384                                       |
| `HTTP_MAX_IN_FLIGHT`          | `256`      | 1–4096                                        |
| `RATE_LIMIT_PER_SECOND`       | `20`       | 0.1–10000, finite                             |
| `RATE_LIMIT_BURST`            | `40`       | 1–10000                                       |
| `RATE_LIMIT_MAX_CLIENTS`      | `4096`     | 1–65536                                       |

Authenticated SSE has additional fixed queue, replay, keepalive, client,
lifetime and write bounds in the realtime/HTTP composition. The ordinary
request-buffer deadline does not apply to these bounded streams. Probe and
metrics requests have their documented operational rate-limit exemption.

## Setup and provider authority

| Name                         | Default                             | Rule                                                                                                |
| ---------------------------- | ----------------------------------- | --------------------------------------------------------------------------------------------------- |
| `SETUP_BOOTSTRAP_PROOF`      | absent                              | Canonical unpadded base64url 32-byte one-use proof; paired expiry required                          |
| `SETUP_BOOTSTRAP_EXPIRES_AT` | absent                              | Canonical UTC RFC3339, at most one hour ahead; must be future on first setup                        |
| `DOCKER_ENABLED`             | `false`                             | Direct socket is restricted to the explicit loopback development profile                            |
| `DOCKER_SOCKET_PATH`         | `/var/run/docker.sock`              | Operator-owned absolute local Unix socket path; no browser path input                               |
| `KUBERNETES_ENABLED`         | `false`                             | Opt-in mounted service-account collector with explicit namespaces; cluster reads are separate opt-in |
| `KUBERNETES_TOKEN_FILE`      | Standard service-account token path | Operator-owned absolute local mount                                                                 |
| `KUBERNETES_CA_FILE`         | Standard service-account CA path    | Operator-owned absolute local mount                                                                 |
| `MUTATIONS_ENABLED`          | `false`                             | Enables only the fixed guarded operation set; all other gates still apply                          |
| `DOCKER_WRITE_ENABLED`       | `false`                             | Requires Docker observation and global mutation flag                                                |
| `KUBERNETES_WRITE_ENABLED`   | `false`                             | Requires Kubernetes observation and global mutation flag                                            |
| `DOCKER_WRITER_ORIGIN`       | absent                              | Separate HTTPS origin for the narrowly guarded Docker writer                                        |
| `DOCKER_WRITER_CA_FILE`      | absent                              | Operator-owned CA file reference for that writer                                                    |
| `DOCKER_WRITER_CERT_FILE`    | absent                              | Separate writer mTLS certificate file reference                                                     |
| `DOCKER_WRITER_KEY_FILE`     | absent                              | Separate writer private-key file reference                                                          |
| `KUBERNETES_WRITER_TOKEN_FILE` | absent                            | Separate writer token file, distinct from the observer identity                                     |
| `KUBERNETES_WRITER_CA_FILE`  | absent                              | Operator-owned writer trust file reference                                                          |
| `KUBERNETES_WRITER_NAMESPACES` | absent                            | One to 64 unique explicit namespaces, all already enabled for observation                           |
| `AUDIT_RETENTION_DAYS`       | `365`                               | Minimum 90 days; accepted range 90–3650                                                              |
| `AUDIT_QUOTA_ROWS`           | `1000000`                           | Protected capacity range 1,000–10,000,000; reserve permits bounded completion audits                |
| `AUDIT_ARCHIVE_VERIFIED_THROUGH` | absent                          | UTC cutoff only after an operator independently verifies a protected archive                        |

Generate setup material using [account recovery/setup](operations/account-recovery.md).
Do not store it in tracked files, command arguments or logs. Mounted provider
credentials are never returned by the product. Namespace-only collection and
restricted Docker access have the profiles described above; their executed
proof is recorded in the [active plan](implementation-plan.md).
Policy flags never replace role, CSRF, recent auth, preview, exact version,
one-use confirmation, durable intent and outcome audit checks.

Registered profiles and enabled state persist in PostgreSQL. Startup adds only
missing profiles and preserves enabled state. Reviewed startup Docker profile
selection and authority changes advance the version, append protected audit and
fence old cache until fresh coverage publishes. Unchanged startup is a no-op. Administrator
source toggles need their own CSRF, exact-version preview, one-use confirmation
and `source.update` recent-auth grant. They do not depend on infrastructure write
flags and do not grant provider write authority. Browsers cannot create profiles,
provide endpoints/paths or edit credentials. An absent operator profile can be
disabled, but cannot be enabled.

Collection and ordinary history retention use fixed runtime policy rather than
environment settings: bounded polling/backoff, 64 sources, four concurrent
collections and fair deadline-bound retention. Metrics older than seven days
and eligible health/events/terminal deliveries older than 30 days can be pruned;
the newest metric per resource/name and protected records are retained. Audit
uses a protected operator policy with at least 90 days, a bounded quota and a
separate verified-archive cutoff. Browser cleanup never touches audit records.
Operator credential references remain deployment-owned and are rotated through
the [guarded action runbook](operations/guarded-actions.md); the browser never
accepts paths or credential values. See [observations](observations.md) for
ordinary-history limits and exclusions.

## Generic webhook

| Name                               | Default | Rule                                                                     |
| ---------------------------------- | ------- | ------------------------------------------------------------------------ |
| `WEBHOOK_ENABLED`                  | `false` | Must be true when other webhook values are supplied                      |
| `WEBHOOK_DESTINATION_REFERENCE_ID` | absent  | Safe stable bounded logical label, not a URL                             |
| `WEBHOOK_URL`                      | absent  | Operator-owned HTTPS URL with strict destination policy on every attempt |
| `WEBHOOK_SIGNING_KEY`              | absent  | Canonical unpadded base64url 32-byte key                                 |

When enabled, all destination fields are required. Browser settings expose
only safe status and guarded one-attempt test delivery. Targets/signing keys
remain operator owned. DNS, IP, redirect, timeout and response-size policy
applies per attempt. Delivery is at-least-once; receivers must deduplicate.
Host case, host syntax, ports, fragments and empty queries must be canonical at
startup. Runtime destination checks still repeat for every attempt.

## Matrix

| Name                              | Default | Rule                                                                     |
| --------------------------------- | ------- | ------------------------------------------------------------------------ |
| `MATRIX_ENABLED`                  | `false` | Reject nonempty destination fields while disabled                        |
| `MATRIX_DESTINATION_REFERENCE_ID` | absent  | Safe bounded reference, distinct from webhook reference                  |
| `MATRIX_ORIGIN`                   | absent  | One canonical HTTPS origin; no user information, path, query or fragment |
| `MATRIX_ROOM_ID`                  | absent  | Exact bounded operator room ID; never returned to the browser            |
| `MATRIX_ACCESS_TOKEN`             | absent  | Operator-owned bearer token; never logged or exposed                     |

All five values use the `STACKPLORE_` prefix. Enabling Matrix requires all four
destination fields. There is no homeserver discovery and no redirect following.
Each attempt validates and pins DNS/IP destinations under the same outbound
policy as webhook. Configured public HTTPS receivers must provide valid TLS.
The canonical origin has no trailing slash or explicit default `:443`. Room IDs
are bounded to 255 printable ASCII bytes, start with `!`, and exclude path/query
delimiters and percent escapes. Tokens are bounded to 4,096 bearer-token bytes.
Settings expose only enabled state and logical references. The guarded test
selects `?adapter=matrix`; omission selects the generic webhook. Read
[alert and delivery semantics](alerts-notifications.md) before enabling either lane.

## Deployment-only values

Compose reads `STACKPLORE_COMPOSE_DATABASE_PASSWORD`,
`STACKPLORE_COMPOSE_DATABASE_URL`, and `STACKPLORE_COMPOSE_PORT` (default 8080).
These values belong to Compose, not the Go process loader. The two setup values
pass through from the operator shell; unset values remain absent. Webhook and
Matrix configuration pass through explicitly. Do not use `docker compose config`
without `--quiet` when real private values are present.

Helm reads [values/schema](../deploy/helm/stackplore/values.schema.json).
`api.setup.enabled=true` reads proof and expiry keys from `api.existingSecret`
for a fresh installation. Disable it after setup. The chart defaults real
Kubernetes reads off; enabling them requests the current cluster reader role.
The external database URL needs verified TLS. For a private database CA, set
`api.databaseTLS.enabled=true`, create the named CA Secret with the configured
certificate key, and add `sslrootcert=/var/run/stackplore/database-ca/ca.crt` to
the URL. Only the CA certificate is mounted. API replicas stay exactly one.
