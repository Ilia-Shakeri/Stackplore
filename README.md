# Stackplore

**Explore every service in your stack.**

[فارسی](README.fa.md)

Stackplore is a self-hosted infrastructure visibility workspace. It normalizes source inventory, health, topology, logs, current metrics, and events into one versioned API and a responsive English/Persian interface.

> **Status:** 1.0 work is active. Stages 1–4 are verified development checkpoints through `0.4.0`, with exact local, PostgreSQL, disposable native/receiver, browser and remote CI outcomes recorded separately in the plan. Stage 4 has passed its final local and browser gates; its exact remote outcome is recorded after publication. Stages 5–9 retain provider fidelity, guarded actions/settings, installation, performance and final release gates. A development checkpoint does not complete 1.0.

## Implemented

- PostgreSQL-only server startup with bounded pooling, embedded forward migrations, advisory migration and ownership locks, readiness checks, and native `pg_dump`/`pg_restore` operations
- Local administrator/viewer accounts, Argon2id credentials, opaque sessions, CSRF and recent-auth proofs, first-run setup, user administration, and operator recovery
- Provider-neutral domain ports and a deterministic synthetic source with stable resource identity, cursor-bound inventory, health, topology, logs, current metrics, and event history
- Persistent safe source references and normalized observations; product inventory, health, topology, metrics and events use PostgreSQL reads, while logs remain bounded live reads. Collection has concurrency limits, cancellation and retry backoff; fair bounded retention excludes protected audit history. See [observations and source settings](docs/observations.md).
- Versioned OpenAPI contract, generated browser types, strict runtime response parsing, safe problems, request bounds, rate limits, proxy trust, liveness, readiness, and guarded process metrics
- Paginated overview, resources, detail, topology, logs, events and alert journeys with URL filters, truthful partial states, keyboard alternatives and English/Persian RTL. See [exploration](docs/exploration.md).
- Administrator source enable/disable with exact target/version, impact preview, expiring one-use confirmation, purpose-bound recent authentication and atomic audit; profiles and credentials remain operator supplied
- Covered evidence for all eight declared alert predicates, administrator rule management, persistent synthetic demo episodes, durable alert change replay and linked in-app notifications. Opt-in guarded webhook and Matrix lanes have safe status, recent-auth test delivery, stable retry payloads, fenced leases and terminal audit. See [semantics and limits](docs/alerts-notifications.md).
- Non-root API and web images, a persistent local Compose profile, an external-PostgreSQL cluster chart, and CI definitions for code, PostgreSQL integration, image, Compose, and chart gates

## Still required for 1.0

- Restricted production Docker access, namespace-scoped Kubernetes and partial-permission fidelity, plus live-provider proof
- Broader safe profile setup and operator-owned credential-reference rotation; configurable quotas, protected audit cleanup, export and preferences
- Live operator receiver and proxy/browser proof, plus immutable audit coverage for unfinished mutation paths
- Explicitly guarded supported operations with authorization, confirmation, idempotency, timeouts, and audit records
- Live target proof for PostgreSQL backup/restore, Compose, cluster install/upgrade/rollback, browser flows, accessibility, and performance budgets

## Development

Use Node.js `24.18.0`, pnpm `11.17.0`, and Go `1.26.5`:

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
