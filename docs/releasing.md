# Version and release procedure

Each roadmap stage has its own checked development version. Root/workspace
package versions, OpenAPI metadata, Helm chart/app versions, default image tags
and changelog agree; `npm.cmd run check:version` enforces this. A version number
does not establish a passing release or customer installation.

Current code is the 0.9.0 candidate. Published checkpoint receipts through 0.7.0
remain in the [evidence plan](implementation-plan.md); the candidate's final
changed-tree, browser, native, scan and release-artifact gates are
recorded there separately. Do not promote an older passing run to proof of the
candidate. Record commit/tag identities only after they exist.

The [artifact procedure](operations/release.md) ([فارسی](operations/release.fa.md))
defines the implemented tag/dispatch workflow, strict scans, SBOMs, checksums,
signed provenance and consumer identity verification. Its presence does not
establish that a remote packaging/signing run succeeded. Registry publication is
not implicit: the workflow packages a Linux amd64 Docker-save archive.

1. Read the [roadmap](roadmap.md), [audit](codebase-audit.md) and current
   [evidence](implementation-plan.md). Preserve unrelated work.
2. Finish the stage's tests, contract output, both locales and affected docs.
3. Set all release metadata and write the exact dated changelog with known gaps.
4. Run bootstrap, full gate with a disposable real PostgreSQL URL, relevant
   regressions, native restore proof and any named stage-specific target gates.
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

After milestone checks, exact-commit CI and review, create the matching
annotated tag. Never reuse an old checkpoint's tag command for a new version:

```sh
git tag -a vVERSION -m "Stackplore VERSION verified milestone"
git push origin vVERSION
```

Use the task's existing branch or a focused `main/` branch. Never force push,
rewrite historical release tags, or stage all local files without review.

## Final 1.0 gate

Every specification requirement needs code, tests, current accurate documents
and direct evidence. Include browser/a11y/RTL/theme/performance, real provider,
Compose/Helm/cluster/TLS, backup/restore/upgrade, security/dependency/image scans,
SBOM, signed artifact provenance, checksums, supported-version policy and final
independent review. Screenshots must come from the running final demo. Workflow
definitions and local scanner receipts do not replace exact-tag package, image
scan, signing and consumer-verification evidence. Do not label a 0.x development
checkpoint as a completed 1.0 release.

## Platform and acceptance scope

The native runtime validation targets are Windows amd64 and Linux amd64; the
container target is Linux amd64. Require the release's matching startup,
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
