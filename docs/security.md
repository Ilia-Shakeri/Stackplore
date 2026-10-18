# Security Model

Status: 1.0.2 corrective candidate after the fresh audit; review reopened at
8/10. The complete local and fresh production-browser gates pass; exact-source
hosted, signed/public-consumer and final review gates remain pending in the [active plan](implementation-plan.md) and
[current evidence](evidence/1.0.2/README.md). Historical 1.0.1 proof retains its
original scope.

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
  -> same-origin Go application (embedded web and API)
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

Docker socket access is effectively host-root access. The production Docker
profile uses the narrowly scoped restricted proxy and documents its remaining
daemon authority. Kubernetes credentials use a dedicated identity and the
smallest resource/verb/namespace set needed for enabled capabilities. Secret
value read permission is not needed for the default observation profiles.

Docker and Kubernetes adapters remain behind operator flags. The Docker
production profile requires a fixed HTTPS origin, server verification and a
client certificate. The private proxy and adapter both enforce a bounded GET
allowlist; the application has no daemon socket. The proxy still has root-level
daemon authority and is part of the trusted operator boundary. Kubernetes uses
explicit namespaces; cluster reads are opt-in and never include Secret or
ConfigMap values. Kubernetes profiles can use a restricted in-cluster identity or
an explicitly enabled, server-mounted kubeconfig profile. The latter accepts only
the selected bounded regular file, verified HTTPS CA/server identity and
supported static credentials; exec/auth plugins and browser-supplied paths are
rejected. Projected token refresh remains in memory. Safe reference files
must be regular bounded files; projected links cannot escape the selected mount
directory. PostgreSQL stores profile references and enablement, never credential
values. Browser APIs cannot create profiles or supply endpoints, paths or
credentials. Stage 6 uses separate operator-owned writer references; rotate them
through the protected deployment channel described in
[guarded actions](operations/guarded-actions.md). The browser cannot change them.

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

The single API image serves embedded UI files and runs as non-root with Linux capabilities dropped. Its container uses a read-only root filesystem and bounded temporary storage. Compose binds the application only to host loopback and preserves PostgreSQL and runtime-key named volumes.

The cluster chart uses restricted pod security contexts, read-only roots, probes,
resource bounds, an external PostgreSQL Secret, TLS ingress support and one API
replica with persistent runtime-key storage. Set `api.image.digest` to a reviewed
`sha256:` digest to render `repository@digest`; otherwise the chart uses its tag.
Use [artifact verification](operations/release.md) before loading/deploying an image.
The non-root init container prepares a private PVC child, preserves a validated
legacy key by no-overwrite rename, and restores directory/key modes `0700/0600`.
It does not weaken runtime-file validation or gain root authority.

Proxy CIDRs must name only the actual edge network. Broad defaults in example values must be narrowed for the target. An ingress or service mesh must preserve the exact scheme and host used by `STACKPLORE_PUBLIC_ORIGIN`.

Local/render and historical native PostgreSQL receipts remain separate from
current exact-tree Compose/cluster install, TLS, browser, release signing and
target checks. The active plan records each observed result independently.
An operator's customer deployment requires its own ingress, credentials,
database, backup and provider probes; project acceptance does not deploy it.

The production service worker caches exactly four public offline-shell files:
`/offline.html`, `/offline.css`, `/offline.js` and
`/brand/stackplore-favicon.svg`. It never caches app HTML, API/authentication,
provider/log data, health probes or metrics. Offline navigation uses the public
offline page. A waiting update activates/reloads only after the user's action.

## Fresh corrective security boundaries

Provider text masking removes complete credential URL userinfo before log
transport or Kubernetes event summary persistence, including usernames used as
tokens and nested JSON string values. Clipped text must fail closed when a URL
authority may be incomplete. Redaction is defense in depth; review existing
retained observations separately before sharing or exporting them. New masking
does not rewrite historical event rows or prove all arbitrary text harmless.

A later native event page's permission-denied result overrides
earlier success and drops accumulated native events. Canonical byte-collation
keysets prevent database-default ordering from disagreeing with cursor checks.
Native storage commands preserve literal database identity, so percent sequences
or equals characters cannot select a different target through double decoding
or libpq option interpretation. These corrections retain ownership, empty-target,
deadline, secret-handling and audit boundaries. [Proof](evidence/1.0.2/fresh-audit.md).

## Logs, metrics, and retained data

Logs are potentially sensitive even after provider fields are filtered. Bound line bytes, page bytes, time windows, stream buffers, and retention. Treat log search as exact user input, not an expression language. Escape display output and isolate technical text in RTL.

The log download is a read-only server stream bound to its original source,
resource, UTC window, container/previous, search and severity. Capture/export
policy and redaction apply on each continuation. Limits are 10 seconds, 1 MiB
including markers, 1,000 matching lines, 10,000 scanned lines and 100 pages;
global/per-user admission is bounded. A midstream failure or absent completion
marker cannot become a successful browser download. The browser bounds UTF-8
reads, validates markers and cancels stale selections. Raw log bodies are not
added to PostgreSQL history by this feature.

Metric values must be finite, non-negative, correctly unit-tagged, and bounded to safe integers where required. Events and topology evidence need stable IDs, source ownership, normalized reason codes, safe text, deterministic order, and explicit provenance.

A mixed event timeline follows the source's native `EventHistory` capability.
Denial closes the entire feed and returns no items, including safe operation/
alert facts. The current contract has no separately exposed system subfeed;
those facts cannot be used to bypass the native event permission boundary.

A scheduled retention worker uses effective configured ages, one aggregate deadline and bounded
transactions with fair quotas across eligible metrics, health, events and
terminal ordinary deliveries. Synthetic facts, current state, newest current
metric samples, pending/audit-linked deliveries and audit rows are excluded. Audit policy enforces a 90-day minimum and bounded quota. Scheduled expiry is capped at an operator-verified archive cutoff, preserves operation/delivery-linked facts, writes a policy/cleanup audit record, and prunes at most 1,000 rows per transaction. Without archive proof no audit row expires. Monitor growth; bounded cleanup is not a production capacity guarantee. Exact limits are in
[observations](observations.md).

Source registration inserts each new `source.create` audit fact in the same
transaction as its source row. Audit-capacity or insertion failure rolls both
back; restart of an already registered source does not manufacture another
creation event. Source profile updates retain their separate protected audit.

## Guarded operations

Read-only behavior remains the product default. Stage 6 implements only the
fixed operations and gates in [guarded actions](operations/guarded-actions.md):

- the provider reports the exact operation as supported;
- the current user is an administrator;
- recent authentication is valid for that purpose;
- the UI displays the exact target, effect, and risk and requires explicit confirmation;
- the request has an idempotency key and bounded deadline;
- the adapter verifies the result and maps partial/unknown outcomes honestly;
- an immutable audit entry records actor, target, request ID, reason, and outcome without secret values.

No generic shell, arbitrary manifest apply, SQL console, or free-form provider request belongs in version 1.

## Supply chain

Tool/package versions, action revisions and container base digests are pinned.
CI uses frozen pnpm/Go dependencies, contract drift checks, tests, measured
coverage, builds, real PostgreSQL and separate install/provider lanes. The
implemented release workflow builds native/container/source/chart archives,
scans source dependencies including build/dev packages, native binaries and
images, scans Git secrets, and creates SPDX SBOMs, checksums and signed
provenance/SBOM bundles. Its exact-tag verification and consumer identity checks
are described in [release operations](operations/release.md). A failed scan or
signature has no unsigned passing fallback. Runtime legal inventories copy
reviewed exact material and distinguish the one pinned supplemental notice.

CI configuration is not evidence until a trusted run passes. A clean source scan does not prove secret rotation or incident closure.

## Acceptance and residual boundaries

Historical tagged `v1.0.1` at `73ece4754554cca548c242768b195d21664d37b8` passed the full
local gate, exact main/tag CI, strict signed-trial scans, independent trial
verification and actual Windows/database archive recovery. Architecture/code
review is 9/10; [consumer receipt](evidence/1.0.1/consumer-trial.json).
Final stable publication and independent public verification also passed;
that historical delivery was accepted in the
[published receipt](evidence/1.0.1/published-release.json). Older checkpoints, a local
dependency scan or a workflow definition alone cannot establish those results.

Browser-managed profiles/credentials are deliberately unsupported. Provider
reference rotation uses deployment-owned files; real mutations remain disabled
by default and require every guarded-action gate. A live operator receiver,
customer network/CA, credential rights and production restore/rollout need that
installation's separate proof. Cross-compiled macOS/arm64 packages have no native
support claim without native probes. See [testing](testing.md),
[incident response](operations/incident-response.md), [backup/restore](operations/backup-restore.md)
and [the security policy](../SECURITY.md).
