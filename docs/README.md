# Documentation

Start with the [English overview](../README.md) or [Persian overview](../README.fa.md).
The [roadmap](roadmap.md) ([فارسی](roadmap.fa.md)) gives delivery order;
the [evidence plan](implementation-plan.md) states what actually passed.
The working documentation describes corrective 1.0.1, with all nine stages
functionally implemented and a current whole-release score of 8/10. Real
corrective runtime, deployment, scan and final artifact acceptance remain
pending in the [current receipt](evidence/1.0.1/README.md) and
[review](evidence/1.0.1/final-review.md). Immutable 1.0.0 passed its recorded
checks but publication was canceled; the release API confirmed no release.
The earlier published 0.9.0 prerelease has the unsafe database default and is
superseded. Historical successes retain their scope. Project acceptance does
not deploy or certify a customer's production installation.

| Need | Document |
| --- | --- |
| Product scope and completion rules | [Specification](../SPEC.md) |
| Current gaps and review coverage | [Codebase audit](codebase-audit.md) |
| Current user journeys | [User guide](user-guide.md), [راهنمای کاربر](user-guide.fa.md) |
| Paginated exploration, time correlation, logs and topology | [Exploration](exploration.md), [بررسی مشاهده‌ها](exploration.fa.md) |
| Deterministic scenario and isolated guest mode | [Demonstration](demo.md), [نمونه و مهمان](demo.fa.md) |
| System boundaries | [Architecture](architecture.md), [decisions](adr/README.md) |
| Threat model and private reporting | [Security model](security.md), [policy](../SECURITY.md) |
| Local tools and commands | [Development](development.md) |
| Runtime names, defaults and bounds | [Configuration](configuration.md), [پیکربندی](configuration.fa.md) |
| Collection, source toggles and retention | [Observations](observations.md), [مشاهده‌ها](observations.fa.md) |
| Restricted Docker access and runtime facts | [Docker provider](docker-provider.md), [فراهم‌کنندهٔ Docker](docker-provider.fa.md) |
| Namespace authority and Kubernetes runtime facts | [Kubernetes provider](kubernetes-provider.md), [فراهم‌کنندهٔ Kubernetes](kubernetes-provider.fa.md) |
| Guarded operations and credential-reference rotation | [Guarded actions](operations/guarded-actions.md), [عملیات زیرگیت](operations/guarded-actions.fa.md) |
| Application settings, data cleanup and browser preferences | [Settings](settings.md), [تنظیمات](settings.fa.md) |
| Alert evidence, durable streams and outbound delivery | [Alerts and notifications](alerts-notifications.md), [هشدار و اعلان](alerts-notifications.fa.md) |
| Proof levels and test gates | [Testing](testing.md) |
| Current corrective acceptance | [1.0.1 pending gates](evidence/1.0.1/README.md), [corrective review](evidence/1.0.1/final-review.md) |
| Historical checkpoint receipts | [0.4 evidence](evidence/0.4.0/README.md), [0.5 evidence](evidence/0.5.0/README.md), [superseded 0.9 evidence](evidence/0.9.0/README.md), [withheld 1.0.0](evidence/1.0.0/README.md), [canceled publication](evidence/1.0.0/publication-withheld.json) |
| Approved PostgreSQL server/client baseline | [Database image policy](../deploy/postgres-image.json), [patch-upgrade guidance](operations/upgrade-rollback.md#postgresql-17-patch-baseline), [راهنمای ارتقای وصله](operations/upgrade-rollback.fa.md) |
| Install and first administrator | [Deployment](operations/deployment.md), [نصب](operations/deployment.fa.md), [Setup/recovery](operations/account-recovery.md), [راه‌اندازی/بازیابی](operations/account-recovery.fa.md) |
| Protect and recover data | [Backup/restore](operations/backup-restore.md), [پشتیبان/بازیابی](operations/backup-restore.fa.md), [upgrade/rollback](operations/upgrade-rollback.md), [ارتقا/برگشت](operations/upgrade-rollback.fa.md) |
| Diagnose and monitor | [Troubleshooting](operations/troubleshooting.md), [عیب‌یابی](operations/troubleshooting.fa.md), [Monitoring](operations/monitoring.md), [پایش](operations/monitoring.fa.md) |
| Preserve evidence and recover from an incident | [Incident response](operations/incident-response.md), [پاسخ به رخداد](operations/incident-response.fa.md) |
| Tag, package, sign and verify a version | [Release procedure](releasing.md), [artifact verification](operations/release.md), [بسته و بررسی انتشار](operations/release.fa.md), [changelog](../CHANGELOG.md) |
| HTTP models, routes and limits | [Canonical OpenAPI](../contracts/openapi.yaml) |
| Contribute and review changes | [Contribution](../CONTRIBUTING.md), [project guide](../PROJECT_GUIDE.md), [conventions](conventions.md) |
| License and bundled notices | [License](../LICENSE), [third-party notices](../THIRD_PARTY_NOTICES.md) |

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
