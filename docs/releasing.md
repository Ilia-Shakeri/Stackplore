# Version and release procedure

Latest recorded version-2 artifact acceptance is **2.0.0-alpha.3** at immutable `21475ec1135c0496c5929329738142f3c0d5a9d0`, schema18. [Public proof](evidence/2.0.0-alpha.3/published-release.json) binds fourteen main/tag jobs, signed publication,39 files, eight strict scans and actual Windows recovery. Development alpha.4 is an unfinished conversation foundation. The detailed earlier trial sequence below retains its historical scope; current engine, branch/quality, capacity and full version-2 gates remain separate.

Corrective **1.0.3** is published and independently accepted, schema **17**, immutable `v1.0.3` at `656581cf05d4774358d5644a6e1af3c69a210f81`. [Public acceptance](evidence/1.0.3/published-release.json) binds the exact source and artifacts. Main CI [39235362032](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39235362032), tag CI [39238088634](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39238088634), signed trial [39235380836](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39235380836) and publication [39238088607](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39238088607) passed; independent checks verified 39 public files, 30 checksum entries, 31 provenance subjects and 7 SBOM attestations. Release 417182030 was published at 2026-10-19T10:51:50Z. The 9 stages are complete within the recorded scope; review 9/10. Customer installation, provider rights, receiver delivery and capacity require separate operator proof.
Historical released source and receipts below retain their original scope.

Version-2 runtime checkpoint **2.0.0-alpha.2**, schema **18**, is published and
[accepted within its recorded artifact scope](evidence/2.0.0-alpha.2/published-release.json).
The next shared-policy/evidence source checkpoint **2.0.0-alpha.3** is tagged at
`21475ec1135c0496c5929329738142f3c0d5a9d0`: seven main jobs, signed trial scans,
independent trial signatures and actual Windows recovery pass. Public tag
consumption remains separate; see [current evidence](evidence/2.0.0-alpha.3/README.md)
and the [active ledger](v2/implementation.md). No stable version-2, overall branch
threshold, current engine, human quality, capacity or customer/pilot claim follows.
Future public-route/operator-profile working files remain outside that tag.

Historical [alpha.0](evidence/2.0.0-alpha.0/README.md) retains schema **17** and
its own immutable source. The [earlier draft record](v2/preparation-status.md)
retains its historical source and receipts; it is not the current main source.
Use clean exact-source exports and disposable databases for development. Schema
18 is applied at startup even with the helper disabled; returning a schema-17
binary requires the verified backup/restore procedure.

گام۲ در محدودهٔ رسید خودش منتشر و پذیرفته شد. منبع گام۳ تگ خورد؛ تست میزبان،
اسکن و بستهٔ آزمایشی پاس. پذیرش بستهٔ عمومی تگ و پایان کل نسخهٔ۲ هنوز جداست.
فایل آماده‌سازی گام بعد داخل تگ نیست. طرح۱۷ و ۱۸ با هم قاطی نشوند.

## Historical 1.0.2 acceptance

Each roadmap stage has its own checked development version. Root/workspace
package versions, OpenAPI metadata, Helm chart/app versions, default image tags
and changelog agree; `npm.cmd run check:version` enforces this. A version number
does not establish a passing release or customer installation.

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

Historical 1.0.1 evidence only: Main CI `39122125020` and tag CI `39123016027` passed all six jobs each.
Trial `39122125492` and release `39123016084` succeeded; stable public release
`416416913` was published at `2026-10-18T11:43:54Z`. Failed upstream-only and
report-format candidates, withheld 1.0.0 and superseded 0.9.0 remain history.

The [artifact procedure](operations/release.md) ([فارسی](operations/release.fa.md))
defines the implemented tag/dispatch workflow, strict scans, SBOMs, checksums,
signed provenance and consumer identity verification. Its presence does not
establish that a remote packaging/signing run succeeded. Registry publication is
not implicit: the workflow packages a Linux amd64 Docker-save archive.

1. Read the [roadmap](roadmap.md), [audit](codebase-audit.md) and current
   [evidence](implementation-plan.md). Preserve unrelated work.
2. Finish the stage's tests, contract output, both locales and affected docs.
3. Set all release metadata and write the exact dated changelog with known gaps.
4. Run bootstrap, full gate with a disposable real PostgreSQL 17.11 URL, relevant
   regressions, native backup/empty-target restore and the named target gates.
   The server and native clients require 17.11 or a later 17.x patch. Follow
   [upstream-based minor-update guidance](operations/upgrade-rollback.md#postgresql-17-patch-baseline)
   for existing installations; their operator must validate backup/restore and
   maintenance. No specific 17.6-to-17.11 runtime experiment is claimed.
5. Inspect the full diff, untracked files and `git diff --check`. Exclude runtime
   data, generated builds, unknown screenshots, local secrets and tool caches.
6. Commit one reviewed stage using Conventional Commits and push that candidate
   under the maintainer authorization. Inspect CI for that exact revision before
   creating the annotated `vVERSION` tag; tag the exact verified commit, then push
   the matching tag. Never force push or move a published tag.
7. Inspect tag CI for that exact pushed revision.
   Report failed, pending or unobserved
   remote gates honestly. CI definitions are not CI proof.
8. Inspect the separate release workflow and independently verify the downloaded
   checksum manifest, artifact signatures, expected workflow/tag/source identity
   and SBOMs. Record the run, digest and result. Publication waits for all required
   exact-tag verification jobs and all scans/signing; failed uploads remain drafts.

The canonical [PostgreSQL image policy](../deploy/postgres-image.json) is a
separate release boundary. The current runtime named by `deploy/postgres-image.json`
uses the same digest-pinned upstream PostgreSQL 17.11 base and rebuilds unchanged
official gosu 1.19 with Go 1.26.10. Engine and entrypoint are retained. Actual
build, helper/engine versions and root-drop behavior must be checked, followed
by a full image scan with all base findings still blocking. Compose/CI build
owned patched fixtures; Helm retains its external database policy.

Require actual base/derivative and engine/helper metadata in `database-image.json`,
the full scan in `database-image-scan.json` and inventory in `database.spdx.json`,
with identity binding between them. Publish the actual database Docker-save
archive and its seventh signed SBOM. Application-image scanning does not cover
the database image. Expected totals are **39 public assets, 30 checksum entries,
all 31 provenance subjects and seven SBOM attestations**: five native archives,
the application image and the database image. These counts, derivative execution and exact public publication/download checks
passed independent verification for 1.0.1; see the
[published receipt](evidence/1.0.1/published-release.json).

After milestone checks, exact-commit CI and review, create the matching
annotated tag. Never reuse an old checkpoint's tag command for a new version:

```sh
git tag -a vVERSION -m "Stackplore VERSION verified milestone"
git push origin vVERSION
```

Use the task's existing branch or a focused `main/` branch. Never force push,
rewrite historical release tags, or stage all local files without review.

## Final 1.0 scope and corrective 1.0.3 gate

Every specification requirement needs code, tests, current accurate documents
and direct evidence. Include browser/a11y/RTL/theme/performance, real provider,
Compose/Helm/cluster/TLS, PostgreSQL 17.11/native backup/restore, operator minor-update
guidance, derivative engine/helper/root-drop probes,
security/dependency/application-image/full-database-image scans,
SBOM, signed artifact provenance, checksums, supported-version policy and final
independent review. Screenshots must come from the running final demo. Workflow
definitions and local scanner receipts do not replace exact-tag package, image
scan, signing and consumer-verification evidence. Do not label a 0.x development
checkpoint or the withheld 1.0.0 tag as a completed corrective release. The
1.0.3 review reopened at 8/10, then returned to 9/10 after corrections, full
PostgreSQL/browser and exact main/tag gates, signed trial/publication and actual
independent public archive acceptance. [Final review](evidence/1.0.3/final-review.md).
The documentation follow-up still requires all six checks on its own commit;
final binding belongs to commit checks and the release body.

Historical corrective 1.0.2: immutable source `376ed03` passed six-job exact main/tag
CI, the strict signed trial and independent actual trial archive recovery.
The first tag/browser failure and blocked release are retained; the unchanged-tag
code retry passed. Signed publication and independent public/archive checks also
pass. A later main-only worker control-read/report-retention fix must have six-job
CI on its review commit before handoff; never
describe it as part of the unchanged release tag. [Proof](evidence/1.0.2/README.md).

## Platform and acceptance scope

The native runtime validation targets are Windows amd64 and Linux amd64; the
application and database image targets are Linux amd64. Require the release's matching startup,
readiness, authentication/read and storage probes before asserting support for
that artifact. The macOS amd64/arm64 and Linux arm64 archives are additional
cross-compiled outputs with no native-runtime compatibility claim until tested.

Project acceptance means the specified code, documents and named release gates
have evidence. A customer's production installation is a separate operational
task: its credential rights, ingress/TLS, receiver, backups and rollout require
its own checks. No customer production deployment is implied by source tagging
or project acceptance. Use [deployment](operations/deployment.md),
[upgrade/rollback](operations/upgrade-rollback.md) and
[incident response](operations/incident-response.md) for that operator work.
