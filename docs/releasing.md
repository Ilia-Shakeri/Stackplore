# Version and release procedure

Each roadmap stage has its own checked development version. Root/workspace
package versions, OpenAPI metadata, Helm chart/app versions, default image tags
and changelog agree; `npm.cmd run check:version` enforces this. `0.x` checkpoints
do not claim production support or publish images merely by tagging source.

Current state: stages 1–3 are verified/tagged checkpoints, including exact remote
results for the unchanged 0.3.0 tag pushed on 2026-10-17. The 0.4.0 exploration
checkpoint passed its final local, PostgreSQL, native and browser gates; exact
publication outcomes are recorded separately. The maintainer's explicit
test/version/commit/push request on 2026-10-17 supersedes the earlier no-push
instruction for this task. Record commit/tag identities only after they exist.
See [exploration](exploration.md), [observations](observations.md),
[alerts](alerts-notifications.md) and the [evidence plan](implementation-plan.md).

1. Read the [roadmap](roadmap.md), [audit](codebase-audit.md) and current
   [evidence](implementation-plan.md). Preserve unrelated work.
2. Finish the stage's tests, contract output, both locales and affected docs.
3. Set all release metadata and write the exact dated changelog with known gaps.
4. Run bootstrap, full gate with a disposable real PostgreSQL URL, relevant
   regressions, native restore proof and any named stage-specific target gates.
5. Inspect the full diff, untracked files and `git diff --check`. Exclude runtime
   data, generated builds, unknown screenshots, local secrets and tool caches.
6. Commit one reviewed stage using Conventional Commits. Create an annotated
   `vVERSION` tag at that commit, then push the reviewed commit and matching tag
   under the maintainer authorization. Never force push or move a published tag.
7. Inspect CI for that exact pushed revision.
   Report failed, pending or unobserved
   remote gates honestly. CI definitions are not CI proof.

Example for the 0.3 checkpoint only after all milestone checks and release review:

```sh
git commit -m "feat: complete covered alert evidence and durable delivery for 0.3.0"
git tag -a v0.3.0 -m "Stackplore 0.3.0 alert and delivery checkpoint"
```

Use the task's existing branch or a focused `main/` branch. Never force push,
rewrite historical release tags, or stage all local files without review.

## Final 1.0 gate

Every specification requirement needs code, tests, current accurate documents
and direct evidence. Include browser/a11y/RTL/theme/performance, real provider,
Compose/Helm/cluster/TLS, backup/restore/upgrade, security/dependency/image scans,
SBOM, signed artifact provenance, checksums, supported-version policy and final
independent review. Screenshots must come from the running final demo. The
current workflow validates/builds; registry publication, SBOM/signing and final
release packaging remain stage 8 work. Do not label a 0.x development checkpoint
as a completed 1.0 release.
