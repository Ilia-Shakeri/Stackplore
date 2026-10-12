# Version and release procedure

Each roadmap stage has its own checked development version. Root/workspace
package versions, OpenAPI metadata, Helm chart/app versions, default image tags
and changelog agree; `npm.cmd run check:version` enforces this. `0.x` checkpoints
do not claim production support or publish images merely by tagging source.

1. Read the [roadmap](roadmap.md), [audit](codebase-audit.md) and current
   [evidence](implementation-plan.md). Preserve unrelated work.
2. Finish the stage's tests, contract output, both locales and affected docs.
3. Set all release metadata and write the exact dated changelog with known gaps.
4. Run bootstrap, full gate with a disposable real PostgreSQL URL, relevant
   regressions, native restore proof and any named stage-specific target gates.
5. Inspect the full diff, untracked files and `git diff --check`. Exclude runtime
   data, generated builds, unknown screenshots, local secrets and tool caches.
6. Commit one reviewed stage using Conventional Commits. Create an annotated
   `vVERSION` tag at that commit. Push the branch and exact tag without force.
   The maintainer authorized these actions for the current delivery request.
7. Inspect CI for the pushed revision. Report failed, pending or unobserved
   remote gates honestly. CI definitions are not CI proof.

Example after all checks, with an actual reviewed release version:

```sh
git commit -m "feat: establish verified PostgreSQL baseline"
git tag -a v0.1.0 -m "Stackplore 0.1.0 development baseline"
git push origin HEAD
git push origin v0.1.0
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
release packaging remain stage 8 work. Do not label the current 0.1 tag as 1.0.
