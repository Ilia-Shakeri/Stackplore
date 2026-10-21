# Stackplore delivery plan

## Active version 2 implementation — 2026-10-21

The maintainer authorized all stages, tests, documentation, checkpoint versions,
commits and pushes on 2026-10-19. This explicit authorization supersedes the
default no-push repository rule for this delivery. Preserve every existing tag
and all unrelated user work. Stage 0 starts at `2.0.0-alpha.0`; stable 1.0.3 and
its schema-17 acceptance remain immutable. No helper runtime is shipped yet.

Published stage-0 checkpoint **2.0.0-alpha.0**, schema **17**, is immutable at
`eb86ab7554db555f023c1de72c3119acf101226a`. Main CI
[39258522963](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39258522963)
and tag CI [39262174884](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174884)
pass all six jobs; release run
[39262174841](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174841)
published prerelease [417357745](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v2.0.0-alpha.0).
[Public acceptance](docs/evidence/2.0.0-alpha.0/published-release.json) binds
39 files, 30 checksums, 31 provenance subjects, seven SBOM attestations and eight
strict scans with zero HIGH/CRITICAL findings. Public Windows bytes match the
actual recovery-tested trial. Application-image load/identity passed without
starting a container; the different public PostgreSQL archive independently
passed engine/helper/root-drop, bounded isolated SQL and six-container cleanup
with absence verified. **Stage 0 is accepted within the recorded scope.**
[Alpha.0 evidence](docs/evidence/2.0.0-alpha.0/README.md) retains failed consumer
attempts and separates each runtime claim.

Stage 1 now has an actual native local protocol proof: startup and both English
and Persian typed requests pass. The bounded gateway candidate is
**2.0.0-alpha.1**, with public dispatch absent and schema **17** in its isolated
source. Its [candidate receipt](docs/evidence/2.0.0-alpha.1/README.md) separates
that result from hosted and publication gates. Full local gate C passes on
tree `695c705a379ae46f3eb570009efab10654ae6f85`: 544 web, 40 client, 48 release
and 11 corpus tests, cached Go checks and production build. The unchanged
PostgreSQL/native checks actually passed in attempt B; both earlier full-gate
failures remain recorded. Final receipt/document edits need exact-commit CI.
The earlier launch
rejection remains historical; the maintainer explicitly authorized the new
installation/startup on 2026-10-21. The current CI definition adds a seventh,
Windows pure-tool job; historical alpha.0 checks remain six jobs.


The previous alpha.1 source `f1ae80391aca646fca89fed7a5ecf79271999c97`
passed seven hosted jobs and fourteen actual browser cases, but signed trial
`39592142674` failed on CVE-2026-111943 in the Go 1.26.9 standard library
embedded in gosu. No alpha.1 tag was created. The Go 1.26.10 source correction
requires fresh local, hosted and artifact acceptance;
[retained failure and toolchain proof](docs/evidence/2.0.0-alpha.1/README.md#go-security-correction--2026-10-21). Earlier compiler results do
not accept the correction.

Stages **1–10 remain incomplete** until their own full gates pass. Draft
[PR 1](https://github.com/Ilia-Shakeri/Stackplore/pull/1),
`e660e16bd0b3c56f0096bb43b5688195135892b1`, contains unaccepted stage-1–3
preparation, not a delivered milestone. Its earlier code/SQL and draft CI do not
accept the newer gateway files or replace later stage gates.

**Checkout boundary:** cleanup was also review-rejected. Modified and untracked
preparation, including migration 18, remains. Normal startup
applies schema **18** even with the capability disabled. The branch name and
alpha.0 version label do not make this checkout clean schema-17 source. Use a
verified release or a clean exact-source export for alpha.0; retained preparation
uses a fresh disposable schema-18 database only. See the
[separate preparation status](docs/v2/preparation-status.md).

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
