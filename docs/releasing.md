# Version and release procedure

Each roadmap stage has its own checked development version. Root/workspace
package versions, OpenAPI metadata, Helm chart/app versions, default image tags
and changelog agree; `npm.cmd run check:version` enforces this. A version number
does not establish a passing release or customer installation.

Current code is corrective 1.0.1; all nine stages are functionally implemented,
but the current whole-release score is 8/10. Exact corrective real-PostgreSQL,
browser, hosted-deployment and strict scan proof remains pending, followed by
tag/publication and independent public-asset acceptance. Track results in the
[active plan](implementation-plan.md) and [1.0.1 checklist](evidence/1.0.1/README.md).

The pushed upstream-only candidate `a252718efffbdbb03c8f4552c25e998f481c77bb`
is rejected and untagged. Local B (24 release/policy, 40 client, 453 web tests)
and all 14 browser cases passed. Trial `39119787671`, job `117652865830`, passed
the exact upstream pull/compiled PostgreSQL 17.11 probe but failed the strict
database-image scan. Fresh local Trivy 0.76.0 reproduced one CRITICAL and 24 HIGH
gosu Go-library findings; all remain blocking. The hosted job did not retain
its JSON; the [failed receipt](evidence/1.0.1/failed-database-image.json) records
the local reproduction and scope. No exception or suppression may bypass it.

Immutable `v1.0.0` remains at `a424d9f4c9f23d25960b93482b3096a6d05d0e1f`.
Its six-job main/tag CI, signed trial, consumer verification and actual Windows
recovery passed within their original scope. Publication run `39117205810` was
canceled after finding the PostgreSQL 17.6 default and absent database-image
scan; the release API confirmed no 1.0.0 GitHub release. The earlier published
0.9.0 prerelease has the unsafe database default and is superseded.
[Withheld receipt](evidence/1.0.0/publication-withheld.json). Preserve the tag and
historical receipts. Do not promote an old passing run to corrective proof or
record new commit/tag identities before they exist.

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
separate release boundary. The intended runtime `stackplore-postgres:1.0.1`
uses the same digest-pinned upstream PostgreSQL 17.11 base and rebuilds unchanged
official gosu 1.19 with Go 1.26.9. Engine and entrypoint are retained. Actual
build, helper/engine versions and root-drop behavior must be checked, followed
by a full image scan with all base findings still blocking. Compose/CI build
owned patched fixtures; Helm retains its external database policy.

Require actual base/derivative and engine/helper metadata in `database-image.json`,
the full scan in `database-image-scan.json` and inventory in `database.spdx.json`,
with identity binding between them. Publish the actual database Docker-save
archive and its seventh signed SBOM. Application-image scanning does not cover
the database image. Expected totals are **39 public assets, 30 checksum entries,
all 31 provenance subjects and seven SBOM attestations**: five native archives,
the application image and the database image. Counts and successful derivative
execution remain unverified until the exact new trial/publication is checked.

After milestone checks, exact-commit CI and review, create the matching
annotated tag. Never reuse an old checkpoint's tag command for a new version:

```sh
git tag -a vVERSION -m "Stackplore VERSION verified milestone"
git push origin vVERSION
```

Use the task's existing branch or a focused `main/` branch. Never force push,
rewrite historical release tags, or stage all local files without review.

## Final 1.0 scope and corrective 1.0.1 gate

Every specification requirement needs code, tests, current accurate documents
and direct evidence. Include browser/a11y/RTL/theme/performance, real provider,
Compose/Helm/cluster/TLS, PostgreSQL 17.11/native backup/restore, operator minor-update
guidance, derivative engine/helper/root-drop probes,
security/dependency/application-image/full-database-image scans,
SBOM, signed artifact provenance, checksums, supported-version policy and final
independent review. Screenshots must come from the running final demo. Workflow
definitions and local scanner receipts do not replace exact-tag package, image
scan, signing and consumer-verification evidence. Do not label a 0.x development
checkpoint or the withheld 1.0.0 tag as a completed corrective release. Return
the review score to 9/10 only after actual corrective PostgreSQL, browser,
hosted-deployment and strict scans pass; final publication acceptance is additional.

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
