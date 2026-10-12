# Documentation

Start with the [English overview](../README.md) or [Persian overview](../README.fa.md).
The [roadmap](roadmap.md) ([فارسی](roadmap.fa.md)) gives delivery order;
the [evidence plan](implementation-plan.md) states what actually passed.

| Need | Document |
| --- | --- |
| Product scope and completion rules | [Specification](../SPEC.md) |
| Current gaps and review coverage | [Codebase audit](codebase-audit.md) |
| Current user journeys | [User guide](user-guide.md), [راهنمای کاربر](user-guide.fa.md) |
| System boundaries | [Architecture](architecture.md), [decisions](adr/README.md) |
| Threat model and private reporting | [Security model](security.md), [policy](../SECURITY.md) |
| Local tools and commands | [Development](development.md) |
| Runtime names, defaults and bounds | [Configuration](configuration.md) |
| Proof levels and test gates | [Testing](testing.md) |
| Install and first administrator | [Deployment](operations/deployment.md), [setup/recovery](operations/account-recovery.md) |
| Protect and recover data | [Backup/restore](operations/backup-restore.md), [upgrade/rollback](operations/upgrade-rollback.md) |
| Diagnose failures | [Troubleshooting](operations/troubleshooting.md) |
| Tag and publish a checked version | [Release procedure](releasing.md), [changelog](../CHANGELOG.md) |
| HTTP models, routes and limits | [Canonical OpenAPI](../contracts/openapi.yaml) |
| Contribute and review changes | [Contribution](../CONTRIBUTING.md), [project guide](../PROJECT_GUIDE.md), [conventions](conventions.md) |
| License and bundled notices | [License](../LICENSE), [third-party notices](../THIRD_PARTY_NOTICES.md) |

Historical ADRs and phase records explain earlier decisions. Superseded SQLite
content is not an installation option. Only current accepted ADRs and current
evidence determine supported behavior. Missing target checks stay explicit.
