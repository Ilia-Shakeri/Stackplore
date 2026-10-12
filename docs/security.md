# Security Model

Status: implemented foundation plus explicit 1.0 gaps

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
- High-risk account changes require the current password and a one-use recent-auth proof bound to the exact operation.
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

Docker and Kubernetes read adapters are wired behind operator flags. The Docker direct socket profile is restricted to loopback development; production support waits for the narrow proxy design. Kubernetes currently requires explicit cluster observation rights, and its chart defaults off. Persistent source/credential workflows and namespace-scoped fidelity remain open.

## PostgreSQL

Production requires an explicit PostgreSQL URL with `sslmode=verify-full`. The database role should own only the Stackplore database/schema and should not have cluster administration, replication, role creation, or unrelated database access.

Startup validates pool and deadline bounds, acquires ownership before migrations, applies forward migrations under a separate advisory lock and verifies the immutable ledger. Dedicated ownership-session checks seal the runtime on loss. Queries and whole transactions have deadlines. Auth mutations serialize through a common transaction lock before current actor/target/final-admin checks. Notification inserts and high-water scans share a commit-order lock. Queries are parameterized.

The local digest key is separate from PostgreSQL. A database-only attacker should not be able to turn stored browser digests into bearer tokens. The key directory must be private, persistent, backed up separately when recovery policy requires session continuity, and never placed in the database backup artifact.

The current live server wires PostgreSQL authentication, product history, alert evaluation and management, in-app notifications, audit reads, and an opt-in generic webhook worker. Webhook URL and signing material come only from operator configuration, stay redacted in formatted settings, and are never returned through browser APIs. Local tests do not prove a real receiver or target network.

## Backup and restore

Backup uses custom-format `pg_dump` to a temporary file, validates it with `pg_restore --list`, then publishes it by atomic rename. Password material uses the child environment, not command arguments.

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

A bounded PostgreSQL prune method covers named non-audit history classes. A scheduled retention worker, quota settings and the separately audited audit lifecycle remain unimplemented. Operators must not assume a helper or table schema performs automatic cleanup.

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

- Production restricted Docker access, namespace-scoped Kubernetes/partial-permission fidelity and persistent least-privilege connection flows
- encrypted provider credential persistence and rotation
- persistent discovery/observation/retention scheduling and quotas
- correct historical alert windows, connection-loss/event evidence, durable alert streams and real receiver proof
- guarded infrastructure operations
- target production PostgreSQL/restore/TLS, Compose, cluster, browser, accessibility and performance proof

These gaps block a 1.0 production-ready claim.
