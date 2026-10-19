> **UNACCEPTED development source — schema 18.** This branch is isolated preparation, identified by its own commit; it is not the released `v2.0.0-alpha.0` source at `eb86ab7554db555f023c1de72c3119acf101226a` (schema 17). Normal startup automatically applies migration 18 even while the new capability is disabled. Use only a fresh disposable test database. Do not point this source at an accepted or customer schema-17 database. Returning an older binary requires its verified backup/restore procedure. Existing receipts do not accept this branch. Real processor/profile and stage exit gates remain open. The configured HTTP log path now delegates to the shared policy; this branch also changes an existing path.

> **شاخهٔ توسعه، نپذیرفته — طرح پایگاه ۱۸.** این کد، آماده‌سازی جدا با کامیت خودش است؛ منبع منتشرشدهٔ `v2.0.0-alpha.0` همان `eb86ab7554db555f023c1de72c3119acf101226a` و طرح ۱۷ است. شروع عادی این شاخه، مهاجرت ۱۸ را خودکار اجرا کند؛ خاموشی قابلیت مانع آن نیست. فقط پایگاه تازه و دورریختنیِ تست به کار رود. به پایگاه پذیرفته یا مشتری با طرح ۱۷ وصل نشود. بازگشت باینری قدیمی، روش پشتیبان/بازیابی تأییدشده لازم دارد. رسید قدیمی، پذیرش این شاخه نیست. گیت پردازندهٔ واقعی و پایان گام‌ها باز است. مسیر HTTP گزارش نیز از سیاست مشترک استفاده کند؛ پس یک مسیر موجود هم تغییر کرده است.

# Stackplore delivery plan

## Active version 2 implementation — 2026-10-19

The maintainer authorized all stages, tests, documentation, checkpoint versions,
commits and pushes on 2026-10-19. This explicit authorization supersedes the
default no-push repository rule for this delivery. Preserve every existing tag
and all unrelated user work. Stage 0 starts at `2.0.0-alpha.0`; stable 1.0.3 and
its schema-17 acceptance remain immutable. No helper runtime is shipped yet.

Follow the [version 2 roadmap](docs/roadmap-v2.md) ([فارسی](docs/roadmap-v2.fa.md)),
the [active stage ledger](docs/v2/implementation.md), and accepted ADRs 0014–0016.
Only passing exact-source gates close a stage; scores, mocks and earlier release
receipts do not establish new runtime or installation proof.

Corrective **1.0.3** is published and independently accepted, schema **17**, immutable `v1.0.3` at `656581cf05d4774358d5644a6e1af3c69a210f81`. [Public acceptance](docs/evidence/1.0.3/published-release.json) binds the exact source and artifacts. Main CI [39235362032](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39235362032), tag CI [39238088634](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39238088634), signed trial [39235380836](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39235380836) and publication [39238088607](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39238088607) passed; independent checks verified 39 public files, 30 checksum entries, 31 provenance subjects and 7 SBOM attestations. Release 417182030 was published at 2026-10-19T10:51:50Z. The 9 stages are complete within the recorded scope; review 9/10. Customer installation, provider rights, receiver delivery and capacity require separate operator proof.
Historical released source and receipts below retain their original scope.

## Historical 1.0.2 acceptance

Accepted release: **1.0.2**, immutable `v1.0.2` at
`376ed0379900ab870dd8eb8af6b55ef3e5ec389b`, schema **17**. All **nine stages**
are complete; architecture/code and release acceptance score **9/10**. Full
local D, six-job main CI `39132989641`, six-job tag CI `39134154907` after
its unchanged-source code retry, signed trial `39133022021`, signed publication
`39134154891` and independent public/archive verification pass. Release
`416503671` was published at `2026-10-18T14:43:05Z`. First failed tag/browser
and blocked-release attempts remain failed. The later main-only harness/docs
follow-up is outside frozen release source; six-job CI on the commit containing
this review must pass before handoff. Final run/source binding belongs to commit
checks and the release body. [Published proof](docs/evidence/1.0.2/published-release.json).

## Historical accepted 1.0.1 delivery

Immutable `v1.0.1` points to `73ece4754554cca548c242768b195d21664d37b8`.
Main CI `39122125020` and tag CI `39123016027` passed all six jobs each.
Signed trial `39122125492` and release `39123016084` passed; stable
[release 1.0.1](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v1.0.1)
was published as `416416913` at `2026-10-18T11:43:54Z`.

Independent public verification checked all 39 files, 30 checksums, all 31
provenance subjects and seven SBOM attestations. Public Windows bytes match the
actual recovery-tested trial. The different public database archive was
independently loaded and executed with identity, engine/helper/root-drop,
read-only non-root SQL, license and owned cleanup checks passing.
[Published receipt](docs/evidence/1.0.1/published-release.json).

Immutable 1.0.0 remains withheld; 0.9.0 is superseded for its unsafe database
default. Intermediate failed trials and interrupted consumer transfers retain
their honest outcomes. Customer/offline/cross-built limits remain separate.

| Stage | Version | Delivered outcome                                                                                                                                                                                                                                 |
| ----- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | 0.1.0   | Complete: safe ownership/migrations, final administrator, bounded calls, invalid-input handling, masking, source selection and reproducible baseline.                                                                                             |
| 2     | 0.2.0   | Complete: durable PostgreSQL observations and safe source profiles, bounded collection/retention, truthful authority and audited source settings.                                                                                                 |
| 3     | 0.3.0   | Complete: covered alert evidence/lifecycle, durable replay and fenced opt-in webhook/Matrix delivery.                                                                                                                                             |
| 4     | 0.4.0   | Complete: paginated exploration, URL filters, log controls, resource details, metric truth, accessible topology and bilingual browser journeys.                                                                                                   |
| 5     | 0.5.0   | Complete: official scoped Docker/Kubernetes adapters, restricted mTLS, independent coverage, safe typed facts, stable log/metric identity and isolated deterministic guest demo.                                                                  |
| 6     | 0.6.0   | Complete: narrowly guarded restart/scale, durable intent/result audit, settings, preferences, quota/archive-bound audit retention and bounded cleanup.                                                                                            |
| 7     | 0.7.0   | Complete: one non-root embedded application, actual Compose setup/restart, Helm installation against external TLS PostgreSQL and bilingual operator runbooks.                                                                                     |
| 8     | 0.9.0   | Functionally complete: PWA, redacted logs, health history, filters, measured interface/performance and signed publication passed their recorded checks. Superseded because its database default is unsafe and database-image scanning was absent. |
| 9     | 1.0.2   | Complete: corrective implementation, full local D, six-job exact main/tag CI, signed trial/publication, independent 39-file public verification and actual archive acceptance. Failed attempts and historical proof retain their scope.           |

The historical **1.0.1** correction loop returned to **9/10** after its full local D (28 release/policy,
40 client and 453 web tests), fresh browser B (14 cases), exact six-job main CI,
strict signed-trial scans, independent verification and actual archive recovery.
The derivative retains PostgreSQL 17.11 and the upstream entrypoint, rebuilding
unchanged gosu 1.19 with Go 1.26.9. Exact full-image/SPDX and engine/helper/root-drop
proof passed without suppressions. The [requirement matrix](docs/evidence/0.9.0/requirements.md)
maps the accepted 1.0 scope to implementation and proof limits.

For the historical 1.0.1 source, all release gates closed: exact-tag CI, signed publication, independent
public identity/checksum/provenance/SBOM checks and actual public archive
acceptance passed. That earlier review did not detect the newly verified defects and navigation gap;
[1.0.2](docs/evidence/1.0.2/README.md) tracks their corrective acceptance. Interrupted public transfers remain
transport failures; bounded range resume retained normal certificate checks,
API sizes/digests and signed-manifest verification.

Customer production acceptance remains an operator task. Credentials, provider
rights, ingress/TLS, receiver reachability, backups and capacity require the
customer environment's own checks. A published release does not certify that
installation. Runtime evidence covers Windows amd64, Linux amd64 and the Linux
amd64 application and database images with PostgreSQL 17; other native outputs
are cross-built only.

- [Completion roadmap](docs/roadmap.md) ([فارسی](docs/roadmap.fa.md)) records scope and exit gates.
- [Active evidence](docs/implementation-plan.md) retains exact commands, outcomes, failures and corrections.
- [Repository audit](docs/codebase-audit.md) and [corrective review](docs/evidence/1.0.1/final-review.md) record reviewed boundaries and the open score.
- [Documentation inventory](docs/README.md) links current user, development and operator guides.
- [Historical phase ledger](docs/history/phase-plan.md) preserves superseded foundation planning.

The maintainer authorized stage-by-stage testing, versioning, commits and pushes
on 2026-10-17. Existing tags and failed receipts remain intact. Unrelated user
artifacts stay outside release commits.
