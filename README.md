# Stackplore

**Explore every service in your stack.**

[فارسی](README.fa.md)

Stackplore is a self-hosted infrastructure visibility workspace. It normalizes source inventory, health, topology, logs, current metrics, and events into one versioned API and a responsive English/Persian interface.

> **Status:** Released and accepted `v1.0.1` at `73ece4754554cca548c242768b195d21664d37b8`. All nine stages are delivered; architecture/code review is **9/10**. Exact main/tag CI, signed publication, independent public verification and actual Windows/database archive checks passed. [Published receipt](docs/evidence/1.0.1/published-release.json).

All 39 public files, 30 checksum entries, all 31 provenance subjects and seven
SBOM attestations were verified. The public Windows archive matches the
recovery-tested trial; the different public database archive was independently
loaded and executed. Earlier 0.9.0 is superseded for its unsafe database default;
immutable 1.0.0 remains withheld and failed intermediate trials remain history.

## Implemented

- PostgreSQL-only server startup with bounded pooling, embedded forward migrations, advisory migration and ownership locks, readiness checks, and native `pg_dump`/`pg_restore` operations
- Local administrator/viewer accounts, Argon2id credentials, opaque sessions, CSRF and recent-auth proofs, first-run setup, user administration, and operator recovery
- Provider-neutral domain ports and a deterministic synthetic source with stable resource identity, cursor-bound inventory, health, topology, logs, current metrics, and event history
- Official Docker and Kubernetes clients behind bounded adapters: restricted mTLS Docker observation, namespace-scoped Kubernetes with independent inventory coverage, safe typed resource facts, exact Pod log identity checks and metric lifetime checks. See [Docker](docs/docker-provider.md) and [Kubernetes](docs/kubernetes-provider.md) for limits and disposable target receipts.
- Versioned nine-resource, two-environment demonstration with stable absolute-clock facts, isolated guest viewing without accounts or sessions, server-side write denial and disabled outbound delivery. Normal authenticated Demo restarts preserve same-version acknowledgements; isolated deployments reset derived facts while preserving protected history. See [demo](docs/demo.md).
- Persistent safe source references and normalized observations; product inventory, health, topology, metrics and events use PostgreSQL reads, while logs remain bounded live reads. Collection has concurrency limits, cancellation and retry backoff; fair bounded retention excludes protected audit history. See [observations and source settings](docs/observations.md).
- Versioned OpenAPI contract, generated browser types, strict runtime response parsing, safe problems, request bounds, rate limits, proxy trust, liveness, readiness, and guarded process metrics
- Paginated overview, resources, detail, topology, logs, events and alert journeys with URL filters, truthful partial states, keyboard alternatives and English/Persian RTL. See [exploration](docs/exploration.md).
- Administrator source enable/disable with exact target/version, impact preview, expiring one-use confirmation, purpose-bound recent authentication and atomic audit; profiles and credentials remain operator supplied
- Covered evidence for all eight declared alert predicates, administrator rule management, persistent synthetic demo episodes, durable alert change replay and linked in-app notifications. Opt-in guarded webhook and Matrix lanes have safe status, recent-auth test delivery, stable retry payloads, fenced leases and terminal audit. See [semantics and limits](docs/alerts-notifications.md).
- One non-root Go image serving the UI and API, persistent local Compose, and an external-PostgreSQL cluster chart. CI checks code, PostgreSQL integration, image build, fresh Compose readiness/UI/restart, and actual TLS-backed chart installation.
- Public-only PWA offline shell and explicit updates, bounded streamed redacted log downloads, URL event filters, health history, metric trends, coalesced live inventory/topology, and operator-mounted Kubernetes token-file or certificate profiles. See [exploration](docs/exploration.md) and [Kubernetes profiles](docs/kubernetes-provider.md).

## Release verification

All release gates passed: exact main/tag CI, strict scans, signed publication,
independent public verification and actual archive recovery/runtime checks.
The accepted public set contains 39 files, 30 checksums, all 31 provenance
subjects and seven SBOM attestations. [Evidence](docs/evidence/1.0.1/README.md).

Customer production rollout is an operator deployment decision. Its credentials,
provider permissions, receiver reachability, backup destination and capacity
must be verified in that environment using the supplied runbooks. Disposable
fixture evidence does not certify a customer's production installation.

## Development

Use Node.js `24.18.0`, pnpm `11.17.0`, and Go `1.26.9`:

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

The corrective tree defines `stackplore-postgres:1.0.1` from a digest-pinned
PostgreSQL 17.11 base in [`deploy/postgres-image.json`](deploy/postgres-image.json).
It rebuilds unchanged `gosu` 1.19 with Go 1.26.9 while retaining the upstream
database engine and entrypoint. Compose/CI must build the owned patched image;
the signed trial includes its Docker-save archive and signed SBOM. Actual
derivative build/runtime/scan, signed publication and independent public archive
consumption passed. Helm continues to use
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
