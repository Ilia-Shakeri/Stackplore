# 1.0.0 withheld checkpoint

Date: 2026-10-18. Source: `a424d9f4c9f23d25960b93482b3096a6d05d0e1f`.
Annotated `v1.0.0` is pushed and immutable. Exact main/tag CI, the signed artifact
trial and actual Windows archive recovery passed. Publication run `39117205810`
was canceled after late review found the vulnerable PostgreSQL 17.6 deployment/CI
default and omitted database-image scan. The release API confirmed no `v1.0.0`
GitHub release. The [withheld-publication receipt](publication-withheld.json)
records cancellation, release absence and unchanged tag identity. This checkpoint
is withheld, not an accepted publication.
Corrective [1.0.1 acceptance](../1.0.1/README.md) is active and pending.
Successful tests and failed attempts retain their original scope.

The [requirement matrix](../0.9.0/requirements.md) covers the accepted product
scope. Stage 9 rechecks the full architecture, security, persistence, providers,
transport, interface, deployment, release and documentation boundaries against
that matrix. Its corrections remove sensitive auth fixture failure output and
stale security, installation, project and contribution claims. See the
[earlier final review](final-review.md). Its 9/10 judgment preceded the database
finding; the current whole-release score is 8/10 in the
[corrective review](../1.0.1/final-review.md).

| Gate | Current result |
| --- | --- |
| Stage 8 baseline | `v0.9.0` source `20f8c2e30cb2ccdfc16aa266f0b0d5647c429881`; exact main/tag CI, signed trial/publication and all 34 public assets passed independent verification. The published Windows archive matches the recovery-tested trial; [published receipt](../0.9.0/published-release.json). Its unsafe database default is now superseded; do not treat that historical publication as the corrective baseline. |
| Consumer trial | Downloaded archive API digest matched; all 26 manifest entries matched. Independently verified 12 provenance subjects and all six archive SBOM attestations with exact main workflow/source identity; wrong source digest was rejected. |
| Windows archive recovery | Executed the verified 0.9.0 trial tools against owned PostgreSQL databases: setup, acknowledgement, SSE/replay, restart, native backup/empty restore, revoked sessions and ownership-loss shutdown passed. |
| Final 1.0 local gate | [Receipt](local-gates.json): complete `npm.cmd run verify` exit 0 with real PostgreSQL, changed server/HTTP SQL cases uncached, 40 client tests, 453 web tests, ten release tests and both builds. Backend core 80.6%/82.6%; frontend statements 81.53%, branches 77.49%. Failed stopped-database attempt A remains separate. |
| Final 1.0 browser | [Receipt](browser-runtime.json), [measurements](browser-measurements.json), [16 public screenshots](screenshots/): all 14 actual production-browser cases passed on Chrome 155.0.8059.80; 64 loaded route states, eight profiles, public-only offline/cache/update and both authenticated language journeys. No skipped, unexpected or flaky result. |
| Final exact main CI | [39116327391](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39116327391), all six jobs passed: code/race/browser/real PostgreSQL, image/chart, Compose, Helm/TLS and both real providers. |
| Final signed trial | [39116335008](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39116335008), package passed; [consumer receipt](consumer-trial.json) records all 26 checksums, 12 provenance subjects, six SBOM attestations, all five three-executable scans, image OS/server and source gomod/pnpm analysis. |
| Versioned Windows archive recovery | Exact verified 1.0.0 trial archive passed setup/read, acknowledgement/SSE/replay, restart, native backup/empty restore, protected history, revocation, fresh login, version and ownership-loss shutdown. Binary metadata identifies the exact source and `vcs.modified=false`; [receipt](consumer-trial.json). |
| Exact tag CI | [39117205748](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39117205748), all six jobs passed on the unchanged release source. |
| Final signed publication | [39117205810](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39117205810), canceled; the release API confirmed no 1.0.0 GitHub release. The tag remains at the source above. |
| Public artifact verification | Not performed: 1.0.0 publication was withheld. Trial verification remains passing within its original coverage. |
| Late database-image finding | Compose/CI used vulnerable PostgreSQL 17.6, and strict scans omitted the database image. Application/native/source/history checks do not cover that separately deployed image. Corrective 1.0.1 must provide fresh engine/digest, OS-package and SPDX proof plus runtime/deployment checks. |

Runtime proof targets Windows amd64, Linux amd64 and the Linux amd64 image,
with PostgreSQL 17. Native Linux arm64 and macOS archives are cross-built only.
The browser matrix records actual Chromium, English/Persian, RTL/LTR, narrow/wide,
light/dark, keyboard, reduced motion, offline privacy and update behavior.
Automated accessibility checks do not certify every assistive technology.

Compose and Helm fixtures prove installation/restart in owned disposable
environments. Helm checks ingress route/certificate objects and uses a Service
port-forward; it does not exercise a customer's chosen ingress controller.
Customer credentials, TLS edge, provider rights, receiver reachability, backups
and capacity remain deployment-specific acceptance work, using the bilingual
[operator guides](../../README.md).
