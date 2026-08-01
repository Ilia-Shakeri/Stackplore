# Stackplore

**Explore every service in your stack.**

[فارسی](README.fa.md)

> **Project status:** Phase 4 adds provider-neutral Go domain contracts,
> focused provider ports, a versioned OpenAPI 3.1 schema foundation, and a
> contract-derived TypeScript client package with reproducible policy and drift
> checks. The versioned contract intentionally contains no paths yet, so no
> `/api/v1` product operation exists. Product workflows, providers, storage,
> authentication, deployment assets, a demo, and a published release do not
> exist yet.

Stackplore is a planned self-hosted infrastructure explorer for understanding Docker and Kubernetes environments from one web interface. Its v1 scope is intended to bring topology, health, logs, metrics, events, alerts, and carefully guarded operations into a coherent troubleshooting workflow.

## Planned v1 direction

- Docker, Docker Compose, and Kubernetes discovery behind a shared provider model
- Live topology with relationship provenance and confidence
- Searchable resources, health details, logs, metrics, and event history
- Local alert rules, in-app notifications, and optional outbound integrations
- Read-only defaults with explicit authorization and audit records for supported changes
- Local accounts with administrator and viewer roles
- English and Persian interfaces with full LTR and RTL support
- A responsive, installable web application
- A deterministic demo mode that is clearly separated from real infrastructure
- Self-hosted deployment through containers and Kubernetes

These are design goals, not claims about the current repository state. Delivery is split into reviewed phases so documentation, implementation, tests, and deployment assets can evolve together.

## Current repository

Phase 0 established the product definition, architecture boundaries, security
model, delivery plan, and community standards. Phase 1 adds the pnpm workspace,
strict TypeScript web scaffold, Go API scaffold, tests, builds, and portable
task entry points. Phase 2 adds the shared interface system, theme behavior,
responsive navigation shell, route foundations, and local brand assets. Phase
3 adds paired feature catalogs, live English and Persian switching, full shell
direction changes, locale formatters, and catalog parity checks. Phase 4 adds
provider-neutral domain and provider-port foundations, the canonical OpenAPI
document, contract-derived browser types, a small same-origin client
boundary, and contract validation tasks.

The main project documents are:

- [Product specification](SPEC.md)
- [Project guide](PROJECT_GUIDE.md)
- [Delivery plan](PLAN.md)
- [Architecture](docs/architecture.md)
- [Security model](docs/security.md)
- [Architecture decisions](docs/adr/)
- [Development guide](docs/development.md)
- [Change history](CHANGELOG.md)

## Development

Use the exact Node.js, Corepack, pnpm, and Go versions in the
[development guide](docs/development.md), then run:

```sh
node scripts/bootstrap.mjs
corepack pnpm run check:contracts
corepack pnpm run verify
corepack pnpm run dev
```

The development page runs at `http://127.0.0.1:5173`. The API binds only to a
loopback address in this phase.

## Security posture

Stackplore is designed for access to sensitive infrastructure. The planned
application therefore keeps provider credentials on the server, does not seek
secret values for inference, drops sensitive provider fields at the adapter
edge, and does not retain, log, or expose those values. It uses least-privilege
access, defaults to read-only behavior, and requires explicit safeguards for
every supported operational change.

Do not post credentials, access tokens, private keys, kubeconfig content, session data, private infrastructure addresses, or unredacted logs in issues or pull requests. See [SECURITY.md](SECURITY.md) for private vulnerability reporting guidance.

## Contributing

The project is being built one numbered phase at a time. Before proposing a change, read [CONTRIBUTING.md](CONTRIBUTING.md), check the active phase in [PLAN.md](PLAN.md), and keep the change within that phase.

Community participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Stackplore is licensed under the [Apache License 2.0](LICENSE).
