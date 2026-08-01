# Security Architecture

## Status and purpose

This document defines the security requirements and delivery boundary for
Stackplore. Through Phase 5, validated startup configuration, safe structured
logging, strict origin and proxy handling, bounded short HTTP requests,
operational probes, protected process metrics, and graceful shutdown are
implemented and tested. Authentication, storage, provider access, streams,
notifications, deployment controls, and guarded actions remain later-phase
requirements. No control described here is a claim of an independent audit.

Security vulnerabilities should be reported through the private process in [SECURITY.md](../SECURITY.md).

## Security objectives

Stackplore must:

- expose only the minimum infrastructure data needed for its documented views;
- keep credentials, sensitive provider configuration, and secret values out of
  response bodies, client-readable storage, logs, events, exports, and audit
  details; the raw session token appears only in a protected `Set-Cookie`
  header;
- require server-side authorization for every protected read and mutation;
- make infrastructure-changing actions explicit, narrow, reviewable, and attributable;
- remain bounded when providers, clients, logs, metadata, or remote endpoints behave maliciously;
- fail closed when authentication, authorization, provider permissions, or security configuration is missing;
- state operational limitations honestly instead of implying protection the product cannot provide.

Availability is important, but it does not justify widening provider access, bypassing authorization, or returning stale private data as if it were current.

## Threat model

The design assumes possible attacks from:

- an unauthenticated network client;
- an authenticated user attempting to exceed assigned permissions;
- a compromised browser session;
- malicious or malformed infrastructure names, labels, annotations, status messages, metrics, or log lines;
- a slow client attempting to exhaust stream resources;
- an outbound notification destination attempting redirects, DNS rebinding, or
  response amplification;
- a compromised dependency, build input, or container base;
- accidental operator misconfiguration.

A process with host administrator access can read or alter local files, replace the running binary, tamper with SQLite, and often reach provider credentials. Docker socket access is effectively host-root access. These are critical deployment boundaries; the application cannot make a compromised host trustworthy.

## Trust boundaries

| Boundary | Untrusted input | Required controls |
| --- | --- | --- |
| Browser to same-origin HTTP service | URLs, headers, cookies, forms, filters, identifiers, stream connections | TLS at the deployment edge, authentication, authorization, CSRF defense, origin checks, input bounds, safe response encoding, rate limits |
| HTTP service to SQLite | User-controlled fields, provider results, concurrent writes | Parameterized queries, typed validation, short transactions, migration checks, quotas, file permissions |
| HTTP service to Demo provider | Fixed fixtures and simulated state | Read-only fixture boundary, no real credentials, no route to live providers |
| HTTP service to Docker boundary | Untrusted daemon data and a root-equivalent control plane | Restricted proxy, exact method and path allowlist, response limits, separate mutation profile |
| HTTP service to Kubernetes API | Cluster metadata and a privileged control plane | Dedicated identity, namespace scope by default, exact resources and verbs, no Secret object reads |
| HTTP service to mounted files | Host paths, symlinks, credential files | Operator-defined roots, canonical path checks, read-only mounts, minimal file permissions |
| HTTP service to outbound notification destination | DNS, redirects, certificates, response bodies | Destination policy, address checks on every connection, time and size bounds, controlled egress |
| HTTP service to clients over SSE | Slow readers, reconnect storms, replay requests | Authenticated same-origin streams, bounded queues and replay, connection caps, cancellation |
| Service worker to browser cache | Private responses and stale application state | Static shell caching only, explicit exclusions, versioned eviction, no private offline data |

Provider output remains untrusted even when it comes from a locally managed daemon or cluster.

## Authentication, sessions, and CSRF

### First-run bootstrap

Initial administrator creation is a security-sensitive state transition:

- network setup is available only while the user table is empty;
- the empty-state check and first administrator insert occur in one serialized database transaction;
- exactly one competing request may succeed, and all later requests fail closed;
- deployments exposed beyond loopback must require an operator-supplied,
  short-lived, one-use bootstrap proof or an equivalent out-of-band gate;
- successful setup permanently closes the network bootstrap path;
- setup attempts are rate-limited and recorded without storing passwords or bootstrap secrets;
- responses do not disclose more setup state than required by the first-run interface.

This prevents two simultaneous requests from both claiming initial ownership.
The last enabled administrator cannot be disabled, deleted, or demoted. A local
operator-only recovery command may reset or create an administrator after
filesystem-level access; it never reopens network setup, revokes affected
sessions, and emits a safe recovery record.

### Passwords and login

- Passwords are hashed with Argon2id using versioned parameters selected from current guidance.
- Password length and resource limits prevent trivial passwords and hash-cost denial of service.
- Login responses are generic for unknown users, wrong passwords, disabled accounts, and invalid sessions.
- Per-account and per-source rate limits use bounded storage and do not allow a client to lock out arbitrary users indefinitely.
- Password changes and account recovery rotate or revoke sessions as defined
  below.
- There is no default password, shipped credential, or hidden authentication bypass.

### Sessions

- Session tokens contain at least 256 bits of cryptographically secure randomness.
- Only a keyed or cryptographic hash of each token is stored; a raw token is
  returned once through `Set-Cookie` and never in a response body.
- Cookies use `Secure`, `HttpOnly`, an appropriate `SameSite` mode, path
  `/api/v1`, and no broad domain attribute.
- Sessions have idle and absolute expiration, rotate after authentication and privilege changes, and are revoked on logout.
- Privilege changes are checked against current server-side state, not only data captured when the session was created.
- Role and enabled-state changes revoke every session for the affected user.
- Authentication material is never stored in browser local storage, placed in URLs, or sent in SSE query strings.

### CSRF and origin validation

Every state-changing browser request requires a CSRF token bound to the session
and a valid same-origin `Origin` check. A missing or invalid `Origin` is
rejected. `SameSite` cookies are defense in depth, not the sole CSRF control.

Before a session exists, an auth preflight endpoint creates a short-lived
server-side nonce record, sets an HttpOnly SameSite pre-auth cookie, and returns
a separate header token bound to that record. Login and first-run setup require
the cookie, header token, and valid `Origin`, then consume the nonce. Pre-auth
material grants no account or setup authority and is rate- and storage-bounded.

The normal deployment is one origin. Cross-origin credentialed access is disabled unless a future, documented deployment mode defines an exact allowlist and test coverage.
Every non-loopback production profile requires an HTTPS configured public
origin and rejects startup otherwise. TLS may terminate at a trusted edge; only
explicitly configured proxy addresses may supply forwarding headers used to
recover the original scheme or host.

## Authorization and guarded actions

Authorization is enforced in the service layer for both HTTP and non-HTTP entry points. Hiding a button is not authorization.

Each infrastructure-changing action must:

1. require `STACKPLORE_MUTATIONS_ENABLED=true` from process startup;
2. require the exact write permission in the active deployment profile;
3. resolve a valid session, current enabled state, administrator role, and a
   purpose-bound recent-auth grant;
4. pass CSRF and same-origin checks;
5. reject demo deployment mode and synthetic targets;
6. require the source's exact action capability;
7. match a typed action and resource-kind allowlist and the expected current
   provider or resource version;
8. show an impact preview and consume a short-lived, single-use confirmation
   bound to actor, source, stable target, action, and material parameters;
9. use a request-scoped idempotency record for any retryable effect;
10. durably commit audit intent before contacting the provider, failing closed
    if that write fails;
11. apply a deadline, output bound, and cancellation;
12. record the final outcome, including uncertain timeout, with safe metadata.

Cleanup actions also require an exact preview, an explicit hard confirmation, a backup warning, and bounded selection criteria. Paths, shell fragments, provider arguments, and selectors supplied by a browser must never be passed to a shell.

Read-only provider access is the default. Mutating Docker or Kubernetes access requires a separate, explicit deployment profile and narrower credentials. A denied provider operation remains denied; the service must not retry it with broader credentials.

Recent authentication is also required to create, promote, demote, disable, or
delete a user; change one's own password; reset another user's password; create,
change, or delete a provider source; create, change, test, or delete
notification delivery; export configuration; or run retention cleanup. Each
route has negative tests for missing, expired, and wrong-user recent-auth proof.

Recent authentication verifies the current password through the normal bounded
hash path, then stores a short-lived, one-use server-side grant bound to user,
current session, and exact purpose. The protected operation consumes the grant
atomically. Session revocation and password, role, or enabled-state changes
remove every outstanding grant. Tests cover expiry, replay, wrong purpose,
wrong user, and wrong session.

A successful self password change rotates the current session and revokes every
other session and grant for that user. An administrative reset revokes every
session and grant for the target user.

V1 authorization is installation-wide. A viewer can read all normalized
provider data, logs, metrics, and infrastructure events exposed by every
configured source. Authentication, account, denied-action, configuration, and
full audit events remain administrator-only and are not copied into the viewer
timeline. V1 provides no source-level or tenant isolation.

## Demo isolation

Demo mode is an explicit deployment choice, not an authentication shortcut.

- Demo provider results come only from fixed, non-sensitive fixtures.
- Guest access, if enabled, is read-only and can reach only demo data.
- All infrastructure mutations are disabled.
- Live Docker and Kubernetes providers, mounted credentials, all outbound
  notification delivery, and sensitive exports are unavailable to the guest
  path.
- Configuration that combines guest demo access with live provider access fails startup validation.
- Guest demo deployment uses a separate data root and database marked for the
  demo fixture version. Startup fails if that database contains non-demo source
  or history rows; a normal installation database can never be reused.
- The interface labels simulated state clearly.

Production mode never silently falls back to demo data when a real provider fails.

## Docker boundary

The Docker socket grants control comparable to host administrator access. It must not be mounted directly into an Internet-facing application process.

- Production uses a dedicated authorization proxy that exposes an exact
  allowlist of required HTTP methods and daemon API paths.
- The default profile permits only documented read operations.
- Container creation, execution, file copy, archive extraction, image build, plugin control, swarm control, and unrestricted event or log access are denied unless a later feature has a separate reviewed design.
- Mutations, if introduced, use a separate opt-in proxy policy and deployment identity.
- The proxy is isolated on a private network, has no public port, and applies request, response, concurrency, and time limits.
- Daemon responses are normalized before use; raw JSON, environment arrays, mounts, and labels are not forwarded to the browser.
- The application does not infer safety from membership in a Docker group.

A direct socket is allowed only in an explicit loopback development profile
with a host-control warning. A remote TLS target must be mapped by the operator,
use mutual TLS with hostname and certificate verification, and terminate at an
authorization layer with an equivalent method-and-path allowlist. Browser input
cannot supply its address. Production keeps the Docker provider disabled unless
one of those restricted authorization boundaries exists.

## Kubernetes boundary

Kubernetes access uses a dedicated service account or dedicated kubeconfig identity. Namespace-scoped access is the default.

- RBAC lists exact GA API groups, resources, and verbs where available. The
  optional read-only `metrics.k8s.io/v1beta1` Resource Metrics API is the sole
  named pre-GA exception.
- Read mode uses only required `get`, `list`, and `watch` permissions.
- Wildcards, privilege escalation, impersonation, token creation, non-resource URL access, and broad cluster-admin bindings are prohibited.
- Secret objects are never read, listed, or watched.
- The product may report the name of a referenced Secret only when that reference is already present in a Pod or workload specification; it never resolves or returns the Secret value.
- Pod execution, attach, port-forward, eviction, proxy, and mutation subresources are absent from the read-only role.
- Cluster-wide discovery is an explicit operator choice with a separately reviewed role.
- A permissions error is displayed as a denied or unavailable state and never triggers credential escalation.
- Kubeconfig sources may use only operator-mapped certificate, key, token-file,
  and CA-file references inside configured read-only roots.
- Kubeconfigs containing credential `exec` or auth-provider plug-ins, proxy
  settings, impersonation, inline credential data, insecure TLS, or
  outside-root file references are rejected before a client is created.

Provider clients must cap watch buffers, recover from expired resource versions with a bounded relist, stop on request cancellation, and avoid persisting raw API objects.

## Metadata and provider normalization

Infrastructure metadata can contain credentials or attacker-controlled content.

- Provider adapters return typed, normalized domain objects rather than raw provider payloads.
- Only documented metadata keys are eligible for browser display or storage.
- Labels and annotations use an explicit allowlist; keys merely absent from a denylist are not safe.
- Environment variables, command arguments, full mount definitions, auth configuration, managed fields, last-applied configuration, and provider credential fields are excluded.
- Names, values, collection counts, nesting, and total response sizes are bounded.
- Text is treated as plain text and encoded by the rendering layer. Provider content is never trusted markup.

Allowlist additions require a security review and leak-focused tests.

## Mounted files and path handling

- Mount roots are chosen by the operator and are read-only by default.
- The operator maps stable logical identifiers to approved files, environment
  entries, and remote targets. Clients can select only those identifiers and
  cannot probe arbitrary host paths or process environment names.
- Server-side resolution canonicalizes paths, rejects traversal and device paths, resolves symlinks safely, and proves the final target remains within an allowed root.
- File type, size, permissions, and ownership are checked before use.
- Credential files are opened only by the provider component that needs them and are never returned, logged, or exposed through download endpoints.
- New file writes use restrictive permissions, safe temporary files in the destination directory, and atomic replacement where supported.
- Optional Compose import parses one selected file as inert data. It does not
  follow includes, `extends`, build contexts, environment files, configs,
  secrets, referenced files, or host environment interpolation, and it extracts
  only allowlisted relationship fields.

## Logs and redaction

Application logs are structured and use bounded fixed fields. The Phase 5
logger redacts sensitive field and group names, replaces raw byte and
unsupported values, neutralizes control and format characters, bounds text and
keys, and omits source paths. Request logs use fixed route and method classes;
configuration failures log the field and rule but never its rejected value.
Authorization headers, cookies, session tokens, CSRF tokens, passwords,
bootstrap secrets, provider credentials, webhook signatures, URL user
information, request bodies, and raw environment data are never logged.
Request-ID source failure uses one fixed safe fallback, increments a counter for
every occurrence, and emits only one bounded process log for the repeated
failure.

Infrastructure logs are hostile input:

- redaction runs before persistence, streaming, search indexing, export, or audit inclusion;
- each line, event, query, response, and total stream has a size bound;
- control characters and terminal escape sequences are neutralized for display;
- sensitive-key patterns and configured secret canaries are removed;
- truncation is explicit so an operator does not mistake partial output for complete output;
- correlation identifiers are random, bounded, and non-secret.

Tests must seed recognizable secret canaries across headers, query strings, metadata, provider errors, and log bodies, then prove they do not reach any output sink. Redaction lowers accidental exposure risk; it cannot guarantee detection of every secret embedded in arbitrary text.

## SQLite security and reliability

SQLite stores authentication state, configuration, history, and audit records and must be treated as sensitive.

- The database and backup directory use restrictive operating-system permissions.
- SQL uses parameters; identifiers and sort expressions come from fixed allowlists.
- Schema changes are versioned, transactional where supported, and checked before serving traffic.
- WAL mode, a busy timeout, short transactions, and one controlled writer path limit lock contention.
- The initial deployment supports one active API process per database. Shared-network filesystems and active-active use are unsupported.
- History, events, sessions, idempotency records, and audit data have explicit quotas and retention policies.
- Disk-space checks and bounded pruning prevent unbounded growth.
- Integrity checks and a tested SQLite backup operation are part of maintenance.

Copying a live database file without the SQLite backup mechanism is not a supported backup. Encryption at rest is an operator responsibility in the initial architecture; use encrypted storage when local-disk disclosure is in scope.

## Server-sent events

SSE endpoints use the existing same-origin session and never accept credentials in a URL.

- Authorization is checked before connection and re-evaluated when relevant permissions or sessions change.
- Per-user, per-source, and global connection limits prevent connection floods.
- The hub uses bounded event sizes, bounded subscriber queues, and a bounded replay ring.
- Events carry monotonically usable identifiers. A reconnect outside the replay window receives a reset signal and must fetch a fresh snapshot.
- Slow clients are dropped or told to resynchronize; they cannot grow server memory without bound.
- Keepalives, write deadlines, shutdown signals, and request cancellation release resources promptly.
- The Phase 5 versioned API path buffers its response and has a short total
  deadline. A stream cannot merely disable that guard; its owning phase must
  add explicit connection, idle, write, lifetime, concurrency, queue, and byte
  ceilings first.
- Responses use `Cache-Control: no-store`, disable intermediary buffering where supported, and are never stored by the service worker.
- Stream payloads contain normalized, authorized data only.

## Notification egress and SSRF

All outbound notification adapters are disabled until configured by an
administrator. Webhook and Matrix test and delivery calls use one shared egress
policy and dialer. Matrix homeserver discovery is disabled in v1; the operator
maps one exact canonical HTTPS origin, and its token is never sent to another
origin or across a redirect.

- HTTPS is required by default. Any plain HTTP exception is explicit, narrow, and visibly unsafe.
- URLs reject embedded credentials, fragments, unsupported schemes, ambiguous host syntax, and unexpected ports.
- DNS results are checked before every connection. Loopback, link-local, private, multicast, unspecified, reserved, and cloud metadata address ranges are denied for both IPv4 and IPv6.
- Redirects are disabled by default. If later enabled, every hop repeats the complete URL, DNS, address, and port policy.
- Connection, TLS, request, and response deadlines are short and bounded.
- Request bodies, response bodies, headers, retries, and concurrent deliveries are bounded.
- Hostname validation and certificate verification remain enabled.
- An operator may allow a private destination only through an exact deployment-level allowlist that ordinary users cannot change.
- A deployment may enforce these rules again with an outbound proxy or network policy.

Notification payloads contain the minimum event fields and no session,
provider, or database credentials. Full webhook URLs, Matrix access tokens,
signing keys, and other credential-bearing values are supplied through
operator-created secret references. They are rotated outside the browser and
never returned in full, logged, audited, diagnosed, or exported. The database
stores only safe adapter metadata, masked reference identifiers, and bounded
delivery state.

Delivery is at-least-once. A persisted stable delivery identifier, canonical
payload bytes, and signed idempotency field remain unchanged across webhook
retries. Matrix uses the stable identifier as its transaction ID. Timeouts are
recorded as uncertain, and tests prove retry envelopes do not drift.

## Audit trail

Security-relevant events include login, logout, session revocation, setup attempts, account and role changes, configuration changes, provider mutations, denied actions, exports, backups, cleanup, and notification changes.

Audit records include UTC time, actor identifier, request identifier, source,
action, target, result, and redacted before-and-after summaries. Session
identifiers, raw tokens, passwords, credentials, arbitrary request bodies, and
raw logs are excluded.

Audit intent and any related local state change commit in the same database
transaction before an external effect begins. If that write fails, the
external action does not run. A separate outcome record follows the external
effect. Audit access is separately authorized and retention is explicit.
Generic cleanup cannot remove audit rows. Local records are tamper-evident only
if a later design adds external sealing; a host administrator can alter a local
SQLite audit trail, so the product must not call it immutable.

Audit storage has a separate operator-configured quota and retention policy
with a documented minimum. A dedicated worker prunes only expired rows in
bounded batches and writes a summary record containing the cutoff and count.
Generic cleanup cannot invoke or widen this lifecycle.

Generic cleanup can prune only explicitly named retention classes: metric
samples, infrastructure events, resolved notification deliveries, and expired
idempotency records. It cannot delete users, active sessions, sources, current
settings, or audit records. Configuration export is administrator-only,
requires recent authentication, uses an explicit field allowlist and response
size cap, sets `Cache-Control: no-store`, records an audit event, masks
reference identifiers and provider hosts, and excludes every credential value.

A retention decrease that would make stored rows newly eligible for deletion
is a destructive cleanup, not an ordinary setting update. It requires a count
and time-range preview, purpose-bound recent authentication, hard confirmation,
a backup warning, and an audit record before the new cutoff can take effect.
The same rule applies to the separate audit-retention policy.

## HTTP and browser controls

The Phase 5 service accepts one canonical public origin. A non-loopback
production listener or origin rejects startup unless that public origin uses
HTTPS and at least one trusted-proxy network prefix is configured. The origin
may contain no user information, path, query, fragment, non-canonical host, or
redundant default port.

Only a direct peer inside an explicit canonical prefix may supply forwarding
data used for routing or client identity. Forwarding fields from an untrusted
peer are rejected on every non-operational path and on `/metrics`; the terse
health and readiness probes do not use forwarding identity. The standardized
`Forwarded` field is rejected wherever forwarding data is evaluated in Phase
5. Trusted peers may supply one bounded `X-Forwarded-Proto`, one bounded
`X-Forwarded-Host`, and an IP-only `X-Forwarded-For` chain of at most 16 hops.
The service walks that chain from the trusted edge toward the client and
rejects malformed, ambiguous, or unverified network state. A non-operational
request through a trusted peer must yield one verified untrusted client from
that chain. Every `/api/v1` request must also resolve to the exact configured
public scheme and authority. This is origin and proxy validation, not user
authorization.

Responses apply route-appropriate controls, including:

- a hash-based Content Security Policy for cacheable application-shell HTML,
  with no unsafe script execution;
- `frame-ancestors 'none'`;
- `X-Content-Type-Options: nosniff`;
- a strict referrer policy;
- a minimal Permissions Policy;
- HTTPS-only Strict Transport Security when the deployment is fully HTTPS;
- no cache storage for authentication, API, log, metric, audit, export, backup, or stream responses.

Phase 5 applies `Cache-Control: no-store`, a deny-all non-HTML content policy,
same-origin resource policy, no-referrer policy, frame denial, content-type
sniffing denial, a minimal permissions policy, and request IDs to every current
response. It removes server, hop-by-hop, content-length, and CORS response
fields supplied by handlers. HSTS is emitted only when the configured public
origin is HTTPS.

Dynamic content is encoded by context. Untrusted HTML is not rendered. Error responses do not expose stack traces, SQL, filesystem paths, provider credentials, or internal request bodies. Server banners and unnecessary diagnostic endpoints are disabled.

`/healthz` and `/readyz` return only terse process state on the public listener.
Phase 5 readiness covers configuration and HTTP lifecycle, not database or
provider health. `GET /api/v1/system/readiness` is reserved for an authenticated
administrator, but each request reaching its handler receives `401` until Phase
7 implements local sessions and roles. No header, query value, cookie, proxy
assertion, or environment setting is a bypass.

`/metrics` requires the exact configured public scheme and authority plus a
verified client IP in an explicit network allowlist. Its default is IPv4 and
IPv6 loopback; denied clients receive a plain `404`. Phase 5 has no
operational-account alternative. Metric names and labels never contain client,
user, source, provider-native, resource, log, or other infrastructure
identifiers. Only fixed route, method, and status classes are used.

All requests have server read, write, and idle timeouts plus an accepted-socket
ceiling. Every non-operational path has a capped per-client token bucket.
Versioned API requests additionally have one total request deadline, a declared
body-length precheck, capped handler body reads, bounded buffered responses, and
a fixed in-flight ceiling. Their panic, timeout, cancellation, overflow, and
overload paths replace any partial result with one safe bounded problem.

## PWA cache boundary

The service worker may cache only the versioned application shell and immutable
public static assets. Cacheable shell HTML uses a static hash-based CSP. Any
HTML that carries a response nonce or personalized boot data is `no-store` and
network-only; a nonce-bearing response is never cached or replayed.

- `/api/`, authentication paths, SSE, logs, metrics, audit data, exports, and any personalized response are always network-only and never cached.
- Offline screens contain no previously viewed private data.
- Service-worker upgrades remove obsolete named caches.
- Logout clears application-owned transient state and any cache that could contain user-specific data.
- A failed API request is shown as unavailable; it is not replaced with an old private response.

## Supply-chain and build controls

- Runtime and build dependencies use exact versions in committed lock and checksum files.
- Dependencies come from official registries or explicitly reviewed sources.
- New dependencies require purpose, maintenance, license, and vulnerability review.
- Automated checks cover known vulnerabilities, license policy, secret leakage, and static analysis.
- Build jobs use minimal permissions, isolated credentials, and pinned third-party task revisions.
- Release artifacts include a software bill of materials and provenance when the release process is introduced.
- Container images use a minimal pinned base, a non-root runtime user, read-only filesystems where practical, and no build-time secrets.
- Deployment manifests use immutable image digests for releases.
- Compose drops all Linux capabilities, sets no-new-privileges, uses the runtime
  default seccomp profile, and adds only reviewed writable mounts or temporary
  filesystems.
- Kubernetes containers set `runAsNonRoot`, disallow privilege escalation, drop
  all capabilities, use `RuntimeDefault` seccomp, and keep the root filesystem
  read-only.
- Kubernetes disables service-account token automount unless the explicit
  in-cluster provider profile requires its dedicated least-privilege identity.

No signature, provenance, scan, or audit claim is made until the corresponding release evidence exists.

## Backup, restore, and incident limits

V1 exposes no browser or HTTP backup or restore path. A local operator-only
command uses the SQLite online-backup mechanism, accepts an explicit
operator-owned destination, refuses unsafe file permissions, and emits no
database contents to standard output. Backups are encrypted and access-
controlled outside the application. Restore is an offline procedure: the
service is stopped, the candidate is integrity-checked in isolation, files are
replaced with restrictive permissions, and all restored sessions are revoked
before service resumes. Both procedures are tested before release.

An incident runbook should cover:

1. isolate the affected service and preserve relevant evidence;
2. revoke sessions and bootstrap material;
3. rotate database, provider, webhook, deployment, and signing credentials that may be exposed;
4. inspect host, proxy, provider, audit, and deployment logs;
5. restore from a verified clean backup if integrity is uncertain;
6. validate provider permissions and application state before reopening access;
7. document impact and notify affected parties as required.

The initial architecture does not provide hardware-backed key storage, database encryption, high availability, remote tamper-resistant audit storage, automatic credential rotation, or disaster-recovery guarantees. Operators must supply these controls when their threat model requires them.

## Review gate

A feature does not pass security review until it has:

- a named trust boundary and minimum required privilege;
- positive and negative authorization tests;
- input, output, time, memory, and concurrency bounds;
- redaction and secret-canary tests where data can leave the service;
- failure behavior that is visible and closed by default;
- updated threat notes, deployment permissions, and operator guidance.
