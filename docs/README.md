# Documentation

Active correction: **1.0.3**, schema **17**, baseline 089727f.
The 2026-10-19 review reopened nine verified bugs and two missing interface
requirements. Fixes and focused tests are implemented; final local/browser/hosted
gates remain pending. [Current audit](evidence/1.0.3/fresh-audit.md).
Historical released source and receipts below retain their original scope.

## Historical 1.0.2 acceptance

Start with the [English overview](../README.md) or [Persian overview](../README.fa.md).
The [roadmap](roadmap.md) ([فارسی](roadmap.fa.md)) gives delivery order;
the [evidence plan](implementation-plan.md) states what actually passed.
The accepted corrective release is **1.0.2**, immutable `v1.0.2` at
`376ed0379900ab870dd8eb8af6b55ef3e5ec389b`, schema **17**. All **nine stages**
are complete; architecture/code and release review score **9/10**. Full local D,
all six exact-source main/tag jobs, signed trial/publication and independent
public checksum/provenance/SBOM/archive checks pass. Failed first tag/browser
and blocked-release attempts retain their outcomes; unchanged-tag retry passed.
[Published proof and runtime limits](evidence/1.0.2/published-release.json).

The immutable 1.0.1 source and its [published receipt](evidence/1.0.1/published-release.json)
retain their original results and scope. Those older checks do not certify a different source. Customer production and air-gapped installation require separate
operator evidence.

The unsafe 0.9.0 database default is superseded; immutable 1.0.0 remains
withheld. Failed intermediate trials and transport attempts keep their original
outcomes alongside real passing subchecks. [Final review](evidence/1.0.1/final-review.md).

| Need                                                       | Document                                                                                                                                                                                                                                              |
| ---------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Product scope and completion rules                         | [Specification](../SPEC.md)                                                                                                                                                                                                                           |
| Current gaps and review coverage                           | [Codebase audit](codebase-audit.md)                                                                                                                                                                                                                   |
| Current user journeys                                      | [User guide](user-guide.md), [راهنمای کاربر](user-guide.fa.md)                                                                                                                                                                                        |
| Paginated exploration, time correlation, logs and topology | [Exploration](exploration.md), [بررسی مشاهده‌ها](exploration.fa.md)                                                                                                                                                                                   |
| Deterministic scenario and isolated guest mode             | [Demonstration](demo.md), [نمونه و مهمان](demo.fa.md)                                                                                                                                                                                                 |
| System boundaries                                          | [Architecture](architecture.md), [decisions](adr/README.md)                                                                                                                                                                                           |
| Threat model and private reporting                         | [Security model](security.md), [policy](../SECURITY.md)                                                                                                                                                                                               |
| Local tools and commands                                   | [Development](development.md)                                                                                                                                                                                                                         |
| Runtime names, defaults and bounds                         | [Configuration](configuration.md), [پیکربندی](configuration.fa.md)                                                                                                                                                                                    |
| Collection, source toggles and retention                   | [Observations](observations.md), [مشاهده‌ها](observations.fa.md)                                                                                                                                                                                      |
| Restricted Docker access and runtime facts                 | [Docker provider](docker-provider.md), [فراهم‌کنندهٔ Docker](docker-provider.fa.md)                                                                                                                                                                   |
| Namespace authority and Kubernetes runtime facts           | [Kubernetes provider](kubernetes-provider.md), [فراهم‌کنندهٔ Kubernetes](kubernetes-provider.fa.md)                                                                                                                                                   |
| Guarded operations and credential-reference rotation       | [Guarded actions](operations/guarded-actions.md), [عملیات زیرگیت](operations/guarded-actions.fa.md)                                                                                                                                                   |
| Application settings, data cleanup and browser preferences | [Settings](settings.md), [تنظیمات](settings.fa.md)                                                                                                                                                                                                    |
| Alert evidence, durable streams and outbound delivery      | [Alerts and notifications](alerts-notifications.md), [هشدار و اعلان](alerts-notifications.fa.md)                                                                                                                                                      |
| Proof levels and test gates                                | [Testing](testing.md)                                                                                                                                                                                                                                 |
| Accepted corrective release                                | [1.0.2 public proof](evidence/1.0.2/published-release.json), [evidence](evidence/1.0.2/README.md), [fresh audit](evidence/1.0.2/fresh-audit.md)                                                                                                       |
| Historical accepted correction                             | [1.0.1 release](evidence/1.0.1/README.md), [review](evidence/1.0.1/final-review.md), [rejected image receipt](evidence/1.0.1/failed-database-image.json)                                                                                              |
| Historical checkpoint receipts                             | [0.4 evidence](evidence/0.4.0/README.md), [0.5 evidence](evidence/0.5.0/README.md), [superseded 0.9 evidence](evidence/0.9.0/README.md), [withheld 1.0.0](evidence/1.0.0/README.md), [canceled publication](evidence/1.0.0/publication-withheld.json) |
| Approved PostgreSQL server/client baseline                 | [Database image policy](../deploy/postgres-image.json), [patch-upgrade guidance](operations/upgrade-rollback.md#postgresql-17-patch-baseline), [راهنمای ارتقای وصله](operations/upgrade-rollback.fa.md)                                               |
| Install and first administrator                            | [Deployment](operations/deployment.md), [نصب](operations/deployment.fa.md), [Setup/recovery](operations/account-recovery.md), [راه‌اندازی/بازیابی](operations/account-recovery.fa.md)                                                                 |
| Protect and recover data                                   | [Backup/restore](operations/backup-restore.md), [پشتیبان/بازیابی](operations/backup-restore.fa.md), [upgrade/rollback](operations/upgrade-rollback.md), [ارتقا/برگشت](operations/upgrade-rollback.fa.md)                                              |
| Diagnose and monitor                                       | [Troubleshooting](operations/troubleshooting.md), [عیب‌یابی](operations/troubleshooting.fa.md), [Monitoring](operations/monitoring.md), [پایش](operations/monitoring.fa.md)                                                                           |
| Preserve evidence and recover from an incident             | [Incident response](operations/incident-response.md), [پاسخ به رخداد](operations/incident-response.fa.md)                                                                                                                                             |
| Tag, package, sign and verify a version                    | [Release procedure](releasing.md), [artifact verification](operations/release.md), [بسته و بررسی انتشار](operations/release.fa.md), [changelog](../CHANGELOG.md)                                                                                      |
| HTTP models, routes and limits                             | [Canonical OpenAPI](../contracts/openapi.yaml)                                                                                                                                                                                                        |
| Contribute and review changes                              | [Contribution](../CONTRIBUTING.md), [project guide](../PROJECT_GUIDE.md), [conventions](conventions.md)                                                                                                                                               |
| License and bundled notices                                | [License](../LICENSE), [third-party notices](../THIRD_PARTY_NOTICES.md)                                                                                                                                                                               |

Historical ADRs and phase records explain earlier decisions. Superseded SQLite
content is not an installation option. Only current accepted ADRs and current
evidence determine supported behavior. Missing target checks stay explicit.

This inventory includes the specification, delivery/audit/evidence plans,
architecture/ADRs, threat model/security policy, developer/contribution guides,
canonical API contract, test/coverage/runtime matrix, configuration and provider
references, bilingual product tutorials, deployment/setup/recovery, native
backup/restore, upgrade/rollback, monitoring/troubleshooting/incident response,
release/signature verification, changelog and license/notices. English/Persian
operational entry points above cover the same implemented behavior; exact test
receipts retain their original language and immutable historical scope.
