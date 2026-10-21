<img src="apps/web/public/brand/stackplore-mark-192.webp" width="96" height="96" alt="Stackplore" />

# Stackplore

**Explore every service in your stack.**

[فارسی](README.fa.md)

Stackplore is a self-hosted infrastructure visibility workspace. It normalizes source inventory, health, topology, logs, current metrics, and events into one versioned API and a responsive English/Persian interface.

**2.0.0-alpha.0 is published** as the stage-0 design and supplied-branding
checkpoint, schema **17**, immutable tag `v2.0.0-alpha.0` at
`eb86ab7554db555f023c1de72c3119acf101226a`. Exact main CI
[39258522963](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39258522963)
and tag CI [39262174884](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174884)
passed all six jobs. [Release run 39262174841](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174841)
published [prerelease 417357745](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v2.0.0-alpha.0).
[Public acceptance](docs/evidence/2.0.0-alpha.0/published-release.json) binds
all 39 files, 30 checksums, 31 provenance subjects and seven SBOM attestations,
the byte-identical recovery-tested Windows archive, application-image load and
identity only, and actual isolated PostgreSQL archive execution. Stage 0 is
accepted within that recorded scope. [Evidence](docs/evidence/2.0.0-alpha.0/README.md)
separates the local, hosted and runtime checks. This is not stable 2.0.0;
version-2 stages **1–10 remain incomplete** and new runtime capabilities remain unshipped.

The separate [unaccepted preparation](docs/v2/preparation-status.md), draft PR
[1](https://github.com/Ilia-Shakeri/Stackplore/pull/1) at
`e660e16bd0b3c56f0096bb43b5688195135892b1`, contains stage-1–3 preparation.
No compatible local processor is running; its candidate launch was rejected by
automatic review. Passing draft checks do not close stage 1 or accept answer quality.

**Local checkout warning, 2026-10-21:** two modified existing files and the
untracked preparation, including migration 18, remain after automatic review
rejected their removal. This checkout is not clean alpha.0 source. Normal startup
automatically applies schema **18**, even when the new capability is disabled.
Use the verified release or a clean exact-source export of `eb86ab7` for alpha.0;
keep retained preparation on a fresh disposable schema-18 test database only.
Follow the [active stage ledger](docs/v2/implementation.md). The stable release
below retains its original scope.

> **Stable 1.0.3:** Corrective **1.0.3** is published and independently accepted, schema **17**, immutable `v1.0.3` at `656581cf05d4774358d5644a6e1af3c69a210f81`. [Public acceptance](docs/evidence/1.0.3/published-release.json) binds the exact source and artifacts. Main CI [39235362032](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39235362032), tag CI [39238088634](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39238088634), signed trial [39235380836](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39235380836) and publication [39238088607](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39238088607) passed; independent checks verified 39 public files, 30 checksum entries, 31 provenance subjects and 7 SBOM attestations. Release 417182030 was published at 2026-10-19T10:51:50Z. The 9 stages are complete within the recorded scope; review 9/10. Customer installation, provider rights, receiver delivery and capacity require separate operator proof. Historical immutable [1.0.2 proof](docs/evidence/1.0.2/published-release.json) keeps its original source and limits.

Immutable [1.0.1](docs/evidence/1.0.1/published-release.json) keeps its historical
39-file public verification and actual archive results. Those receipts do not
certify a different source. The 0.9.0 database default remains superseded,
immutable 1.0.0 remains withheld and failed attempts retain their outcomes.

## Implemented

- PostgreSQL-only server startup with bounded pooling, embedded forward migrations, advisory migration and ownership locks, readiness checks, and native `pg_dump`/`pg_restore` operations
- Local administrator/viewer accounts, Argon2id credentials, opaque sessions, CSRF and recent-auth proofs, first-run setup, user administration, and operator recovery
- Provider-neutral domain ports and a deterministic synthetic source with stable resource identity, cursor-bound inventory, health, topology, logs, current metrics, and event history
- Official Docker and Kubernetes clients behind bounded adapters: restricted mTLS Docker observation, namespace-scoped Kubernetes with independent inventory coverage, safe typed resource facts, exact Pod log identity checks and metric lifetime checks. See [Docker](docs/docker-provider.md) and [Kubernetes](docs/kubernetes-provider.md) for limits and disposable target receipts.
- Versioned nine-resource, two-environment demonstration with stable absolute-clock facts, isolated guest viewing without accounts or sessions, server-side write denial and disabled outbound delivery. Normal authenticated Demo restarts preserve same-version acknowledgements; isolated deployments reset derived facts while preserving protected history. See [demo](docs/demo.md).
- Persistent safe source references and normalized observations; product inventory, health, topology, metrics and events use PostgreSQL reads, while logs remain bounded live reads. Collection has concurrency limits, cancellation and retry backoff; fair bounded retention excludes protected audit history. See [observations and source settings](docs/observations.md).
- Versioned OpenAPI contract, generated browser types, strict runtime response parsing, safe problems, request bounds, rate limits, proxy trust, liveness, readiness, and guarded process metrics
- Paginated overview, resources, detail, topology, logs, events and alert journeys with URL filters, truthful partial states, keyboard alternatives and English/Persian RTL. See [exploration](docs/exploration.md).
- Fixed-route quick navigation from the header or Ctrl+K/Command+K, with translated search, role-aware destinations, source preservation and focus return. Fresh corrections preserve all log severity links, translate severity labels, reset a deleted rule's draft and show actual node icon/scope/state.
- Administrator source enable/disable with exact target/version, impact preview, expiring one-use confirmation, purpose-bound recent authentication and atomic audit; profiles and credentials remain operator supplied
- Covered evidence for all eight declared alert predicates, administrator rule management, persistent synthetic demo episodes, durable alert change replay and linked in-app notifications. Opt-in guarded webhook and Matrix lanes have safe status, recent-auth test delivery, stable retry payloads, fenced leases and terminal audit. See [semantics and limits](docs/alerts-notifications.md).
- One non-root Go image serving the UI and API, persistent local Compose, and an external-PostgreSQL cluster chart. CI checks code, PostgreSQL integration, image build, fresh Compose readiness/UI/restart, and actual TLS-backed chart installation.
- Public-only PWA offline shell and explicit updates, bounded streamed redacted log downloads, URL event filters, health history, metric trends, coalesced live inventory/topology, and operator-mounted Kubernetes token-file or certificate profiles. See [exploration](docs/exploration.md) and [Kubernetes profiles](docs/kubernetes-provider.md).

## Historical 1.0.2 release verification

Corrective source `376ed03` passes full local D and all six exact main/tag jobs,
including real SQL/race/browser, Compose, external-TLS Helm and both providers.
Its signed trial and independent consumer pass 30 payload checksums, all
31 provenance subjects, seven SBOM attestations, strict scans and actual native/
database archive recovery. Signed public publication and independent public-byte
verification also pass in [published proof](docs/evidence/1.0.2/published-release.json).
Packaging remains 39 files; the later main-only browser-harness/report-retention
follow-up has a separate source/CI boundary from the immutable release tag.

Customer production rollout is an operator deployment decision. Its credentials,
provider permissions, receiver reachability, backup destination and capacity
must be verified in that environment using the supplied runbooks. Disposable
fixture evidence does not certify a customer's production installation.

## Development

Use Node.js `24.18.0`, pnpm `11.17.0`, and Go `1.26.9`:

For accepted alpha.0 behavior, use a clean exact-source export or verified
release. The retained local preparation warning above applies before any start,
test or development command using this checkout's migrations.

```sh
node scripts/bootstrap.mjs
corepack pnpm run check:contracts
corepack pnpm run verify
corepack pnpm run dev
```

The web development server uses `http://127.0.0.1:5173`; the API uses `http://127.0.0.1:8080`. PostgreSQL integration tests run only when `STACKPLORE_TEST_POSTGRES_URL` names a disposable test database.

The development launcher loads the root `.env`; direct Go commands read only
process environment. Use `.env.example` for development. A Compose `.env`
contains deployment-only names rejected by the strict Go loader and cannot be
reused unchanged for development. See [configuration](docs/configuration.md).

## Deployment

The alpha.0 source defines `stackplore-postgres:2.0.0-alpha.0` from a digest-pinned
PostgreSQL 17.11 base in [`deploy/postgres-image.json`](deploy/postgres-image.json).
It rebuilds unchanged `gosu` 1.19 with Go 1.26.9 while retaining the upstream
database engine and entrypoint. Compose/CI must build the owned patched image;
the signed trial includes its Docker-save archive and signed SBOM. Historical
1.0.2 trial/public archives passed independent Linux amd64 loading, engine/helper
and root-drop checks, restricted non-root SQL and full-image scans; that proof
does not accept the different alpha.0 database archive. That archive now has its
own passing [public execution receipt](docs/evidence/2.0.0-alpha.0/published-release.json):
PostgreSQL 17.11, root drop/server UID 70, isolated read-only containers, bounded
temporary data, SQL transaction and six-container cleanup/absence verification.
Helm continues to use
operator-owned external PostgreSQL. Do not use the superseded 0.9.0 database
default. Existing installations need the reviewed
[patch-upgrade procedure](docs/operations/upgrade-rollback.md#postgresql-17-patch-baseline),
a validated backup and operator maintenance before changing the database image.

For local Compose, copy `deploy/compose.env.example` to `.env`, replace the placeholders, then run:

For a fresh database, first generate paired short-lived setup proof/expiry in
the operator shell using [first setup](docs/operations/account-recovery.md).
Compose passes them through; do not save them in `.env`. Open `/setup` and
complete the first administrator before removing those values and recreating
the API. See the complete [deployment steps](docs/operations/deployment.md).

```sh
docker compose up --build -d
```

Named volumes preserve PostgreSQL and runtime key data. `docker compose down` keeps them. The cluster chart requires an existing Secret containing an external PostgreSQL URL with `sslmode=verify-full`. See [deployment](docs/operations/deployment.md) and [backup/restore](docs/operations/backup-restore.md).

## Project documents

- [Document index](docs/README.md)
- [Completion roadmap](docs/roadmap.md)
- [Repository audit](docs/codebase-audit.md)
- [Product specification](SPEC.md)
- [Current implementation plan](docs/implementation-plan.md)
- [Architecture](docs/architecture.md)
- [Security model](docs/security.md)
- [Architecture decisions](docs/adr/)
- [Development guide](docs/development.md)
- [Observations and source settings](docs/observations.md)
- [Account recovery](docs/operations/account-recovery.md)
- [Delivery entry point](PLAN.md)
- [Change history](CHANGELOG.md)

## Security

Provider access is sensitive. Keep credentials server-side, grant read-only access by default, and never place passwords, tokens, private keys, kubeconfig content, session data, private addresses, or unredacted logs in issues or pull requests. See [SECURITY.md](SECURITY.md) for private reporting.

## License

Stackplore uses the [Apache License 2.0](LICENSE).
