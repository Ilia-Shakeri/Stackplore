# Version and release procedure

Each roadmap stage has its own checked development version. Root/workspace
package versions, OpenAPI metadata, Helm chart/app versions, default image tags
and changelog agree; `npm.cmd run check:version` enforces this. `0.x` checkpoints
do not claim production support or publish images merely by tagging source.

Current state: `0.1.0` and `0.2.0` are verified/tagged checkpoints. The `0.3.0`
development checkpoint is locally verified: covered alert evidence, durable replay
and isolated outbound lanes. Full local/real PostgreSQL, disposable HTTPS/native
and Helm lint/render checks passed; exact results live in the evidence plan.
Commit/tag identities are recorded there only after they exist. The replacement
repository instructions on 2026-10-16 prohibit push; no new remote result is claimed.
See [observation boundaries](observations.md) and [alert/delivery boundaries](alerts-notifications.md). Preserve old proof as historical;
rerun applicable gates when code changes after the recorded verification.

1. Read the [roadmap](roadmap.md), [audit](codebase-audit.md) and current
   [evidence](implementation-plan.md). Preserve unrelated work.
2. Finish the stage's tests, contract output, both locales and affected docs.
3. Set all release metadata and write the exact dated changelog with known gaps.
4. Run bootstrap, full gate with a disposable real PostgreSQL URL, relevant
   regressions, native restore proof and any named stage-specific target gates.
5. Inspect the full diff, untracked files and `git diff --check`. Exclude runtime
   data, generated builds, unknown screenshots, local secrets and tool caches.
6. Commit one reviewed stage using Conventional Commits. Create an annotated
   `vVERSION` tag at that commit. Current work stays local under the no-push rule.
7. If later explicitly authorized to push, inspect CI for that exact revision.
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
