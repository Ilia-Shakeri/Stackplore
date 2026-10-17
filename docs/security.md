# Security Model

Status: verified checkpoints through 0.4.0 and the 0.5.0 candidate boundaries;
exact local, remote and target evidence remain in the active plan.

## Assets

Stackplore protects:

- local account credentials, sessions, CSRF and recent-auth proofs;
- provider connection credentials and infrastructure metadata;
- PostgreSQL state, history, alerts, notifications, and audit records;
- the local keyed-digest file used for browser proof digests;
- backup artifacts and release configuration;
- operator trust in whether data is real, stale, partial, denied, unsupported, or synthetic.

## Trust boundaries

```text
Untrusted browser/input
  -> public web edge
  -> same-origin Go API
  -> local policy and provider adapters
  -> PostgreSQL / external infrastructure APIs

Operator terminal
  -> offline recovery and storage commands
  -> PostgreSQL and local digest-key directory
```

The browser, proxy headers, provider payloads, resource names, log lines, event summaries, cursor values, and database contents after an uncontrolled restore are untrusted. A host or cluster administrator can replace binaries, read process memory, alter mounted files, reach provider sockets, and modify database traffic. Application controls cannot make a compromised host trustworthy.

## Authentication

- There is no default account or password.
- Passwords use Argon2id with versioned parameters and random salts.
- Session, pre-auth, CSRF, and recent-auth values are random opaque tokens. PostgreSQL stores keyed digests, not usable bearer values.
- Session cookies are host-only, `HttpOnly`, `SameSite=Lax`, scoped to `/api/v1`, and `Secure` on HTTPS.
- Every authenticated request resolves the current enabled user and current installation-wide role.
- Unsafe browser requests require the exact public origin and a session-bound CSRF proof.
- First setup is transactional and closes permanently. A non-loopback empty install needs a short-lived one-use operator bootstrap proof.
- High-risk account, source-toggle and notification-test changes require the current password and a one-use recent-auth proof bound to the exact operation.
- Administrator recovery is an offline operator command. It takes the shared maintenance lock, reads the new password from standard input, advances authorization state, and revokes sessions and grants.

Sign-in and setup errors stay deliberately coarse. Rate limits cover source and account scopes without revealing whether an account exists.

## HTTP boundary

The API enforces:

- canonical public origin and explicit proxy CIDR trust;
- replacement of invalid caller request IDs;
- bounded headers, request bodies, responses, connections, concurrent requests, and deadlines;
- strict methods, media types, query keys, path IDs, and closed JSON shapes;
- per-client token-bucket limits with bounded client state;
- fixed security headers and safe problem responses;
- structured access logs without cookies, authorization data, request bodies, provider payloads, database URLs, or proof values.

`/healthz` is public process liveness. `/readyz` reports coarse readiness. Detailed readiness is administrator-only. `/metrics` needs both the exact public origin and an allowed client network.

## Provider boundary

Provider credentials must remain server-side. Adapters must use least-privilege read-only access by default and drop secret-bearing fields before constructing domain values. Raw payloads, environment values, mounted secret values, Kubernetes Secret data, registry credentials, auth headers, and Docker configuration blobs must never enter public responses, logs, events, topology evidence, or search indexes.

Every capability is explicit. Unsupported, denied, unavailable, stale, and partial results stay different through provider, domain, API, and UI layers. Synthetic results carry a marker. The product must not fill missing real data with demo data.

Docker socket access is effectively host-root access. A production Docker adapter should prefer a narrowly scoped proxy or remote read-only API and must document remaining authority. Kubernetes credentials should use a dedicated ServiceAccount and the smallest resource/verb/namespace set needed for enabled capabilities. Secret read permission is not needed for the planned default feature set.

Docker and Kubernetes adapters remain behind operator flags. The Docker
production profile requires a fixed HTTPS origin, server verification and a
client certificate. The private proxy and adapter both enforce a bounded GET
allowlist; the application has no daemon socket. The proxy still has root-level
daemon authority and is part of the trusted operator boundary. Kubernetes uses
explicit namespaces; cluster reads are opt-in and never include Secret or
ConfigMap values. Projected token refresh remains in memory. Safe reference files
must be regular bounded files; projected links cannot escape the selected mount
directory. PostgreSQL stores profile references and enablement, never credential
values. Browser APIs cannot create profiles or supply endpoints, paths or
credentials. Broader operator-reference rotation remains stage 6 work.

Changing configured scope/profile raises a durable authority fence and source
version before serving cache. Failed collection cannot clear it. Fresh partition
coverage closes denied/unsupported/omitted resource reads, associated topology
and metric/event evidence. Unscoped events cannot prove namespace authority and
are hidden once partitioned coverage is active. Derived Secret/ConfigMap reference
coverage also fails closed when a contributing parent kind is denied.

An explicit guest deployment must use a separate empty database and private data
directory with a persistent mode marker. Only the synthetic provider is allowed;
outbound settings and real-provider inputs are rejected even when supplied empty.
Named public reads create no account/session and set no authentication cookie.
Every mutation and private administration/authentication read is denied before
its handler. Guest stream quotas use bounded verified-peer buckets, not an
invented account. Restart resets derived fixture state while preserving audit
and authentication history. This mode is deliberately separate from a normal
installation; see [the isolation guide](demo.md).

Source changes require administrator role, exact same origin, CSRF, quoted
version, exact target/state impact preview, an expiring one-use confirmation and
a `source.update` one-use recent-auth grant. The final transaction serializes
with account changes and rechecks the live administrator/session before
consuming confirmation and grant, changing configuration and inserting audit.
Only digests are stored; failure rolls the transaction back. A missing operator
profile may be disabled but cannot be enabled. This workflow changes observation
settings and sends no provider write. See [source settings](observations.md).

## PostgreSQL

Production requires an explicit PostgreSQL URL with `sslmode=verify-full`. The database role should own only the Stackplore database/schema and should not have cluster administration, replication, role creation, or unrelated database access.

Startup validates pool and deadline bounds, acquires ownership before migrations, applies forward migrations under a separate advisory lock and verifies the immutable ledger. Dedicated ownership-session checks seal the runtime on loss. Queries and whole transactions have deadlines. Auth mutations serialize through a common transaction lock before current actor/target/final-admin checks. Notification inserts and high-water scans share a commit-order lock. Queries are parameterized.

The local digest key is separate from PostgreSQL. A database-only attacker should not be able to turn stored browser digests into bearer tokens. The key directory must be private, persistent, backed up separately when recovery policy requires session continuity, and never placed in the database backup artifact.

The server wires PostgreSQL authentication, source settings, normalized
observation storage/read ports, bounded collection and retention, alert
evaluation/management, in-app notifications, audit reads and opt-in webhook/Matrix
workers with separate destination leases and cursors. Product reads do not initiate estate collection. Capture must
match the active source version; incomplete/failed collection cannot prove
absence. No raw provider payload, mounted credential or log body is stored by
the collection worker. Webhook URLs, signing material, Matrix origin/room/token
come only from operator configuration, stay redacted in formatting and structured
encoding and are never returned through browser APIs. Every attempt revalidates
DNS and public addresses, pins the checked destination, verifies TLS, rejects
redirects and bounds time and response bytes. IPv6 uses a conservative global
range policy. Retries retain canonical bytes and delivery identity; ambiguous
receipt stays uncertain. Synthetic notices are excluded from both lanes. See
[delivery semantics and limits](alerts-notifications.md). Disposable local HTTPS
tests do not prove an operator receiver or target network.

Alert evaluation requires covered evidence and a current raw collection outcome
bound to the enabled source version. Commit rechecks source configuration,
rule version, claim generation, actual database-clock lease expiry and current
episode state. Rule edits/deletion close the old episode atomically. Alert replay
publishes committed metadata under a commit-order lock, uses bounded client queues
and preserves a pruning watermark; stale cursors reset and refetch paginated state.

## Backup and restore

Backup uses custom-format `pg_dump` to a temporary file, validates it with `pg_restore --list`, then publishes it through an atomic no-overwrite hard link. Password material uses the child environment, not command arguments.

Restore accepts an empty target only, uses one transaction, and verifies the exact migration ledger after import. Finalization advances restore generation and removes all sessions and pre-auth challenges. A successful command still needs target readiness, sign-in, data sampling, and rollback checks.

Backup files contain sensitive application state. Encrypt them at rest, restrict access, define retention, test restore, and record custody. See [backup/restore](operations/backup-restore.md).

## Deployment

The API and web images run as non-root and drop Linux capabilities. The local web container has a read-only root filesystem and bounded temporary filesystems. Compose binds only the edge to host loopback and preserves PostgreSQL and runtime-key named volumes.

The cluster chart uses restricted pod security contexts, read-only roots, probes, resource bounds, an external PostgreSQL Secret, TLS ingress support, and one API replica with persistent runtime-key storage. Use registry-enforced immutable tags in current chart values. Digest fields and release artifact verification remain stage8 work; the current chart appends a tag.

Proxy CIDRs must name only the actual edge network. Broad defaults in example values must be narrowed for the target. An ingress or service mesh must preserve the exact scheme and host used by `STACKPLORE_PUBLIC_ORIGIN`.

Local Helm lint and default/setup/cluster-observation/ingress/annotation renders have direct proof. Local real PostgreSQL integration and native empty-target restore have direct proof. Image/Compose runs, cluster installation/policy, browser and target TLS/database proof remain separate gates.

## Logs, metrics, and retained data

Logs are potentially sensitive even after provider fields are filtered. Bound line bytes, page bytes, time windows, stream buffers, and retention. Treat log search as exact user input, not an expression language. Escape display output and isolate technical text in RTL.

Metric values must be finite, non-negative, correctly unit-tagged, and bounded to safe integers where required. Events and topology evidence need stable IDs, source ownership, normalized reason codes, safe text, deterministic order, and explicit provenance.

A scheduled retention worker uses fixed ages, one aggregate deadline and bounded
transactions with fair quotas across eligible metrics, health, events and
terminal ordinary deliveries. Synthetic facts, current state, newest current
metric samples, pending/audit-linked deliveries and audit rows are excluded.
Audit has a protected retain-all lifecycle; operator quotas, minimum retention,
reviewed cleanup and archive/export custody remain stage 6 work. Monitor growth;
bounded cleanup is not a production capacity guarantee. Exact limits are in
[observations](observations.md).

## Guarded operations

Read-only behavior is the current product default. A future infrastructure mutation may ship only when all are true:

- the provider reports the exact operation as supported;
- the current user is an administrator;
- recent authentication is valid for that purpose;
- the UI displays the exact target, effect, and risk and requires explicit confirmation;
- the request has an idempotency key and bounded deadline;
- the adapter verifies the result and maps partial/unknown outcomes honestly;
- an immutable audit entry records actor, target, request ID, reason, and outcome without secret values.

No generic shell, arbitrary manifest apply, SQL console, or free-form provider request belongs in version 1.

## Supply chain

Tool and package versions are locked. CI uses a frozen pnpm lock, the Go checksum file, contract drift checks, tests, builds, PostgreSQL integration, image builds, Compose validation, and chart lint. Release work should also pin action revisions and image digests, generate an SBOM, scan dependencies and images, sign artifacts, and verify provenance before promotion.

CI configuration is not evidence until a trusted run passes. A clean source scan does not prove secret rotation or incident closure.

## Known gaps

- Production restricted Docker access and namespace-scoped Kubernetes/partial-permission fidelity
- broader safe operator-owned profile/credential-reference setup and rotation; raw credential values must not be persisted
- configurable retention quotas and protected audit cleanup/archive lifecycle
- live operator receiver, reverse-proxy/browser and provider-fidelity proof for the implemented alert windows and durable replay
- guarded infrastructure operations
- target production PostgreSQL/restore/TLS, Compose, cluster, browser, accessibility and performance proof

These gaps block a 1.0 production-ready claim.
