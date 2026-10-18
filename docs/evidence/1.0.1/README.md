# 1.0.1 released and accepted

Date: 2026-10-18. Source: `73ece4754554cca548c242768b195d21664d37b8`,
immutable `v1.0.1`. **All nine stages are delivered and release acceptance is
complete. Architecture/code review is 9/10.**

Stable [release 1.0.1](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v1.0.1),
ID `416416913`, was published at `2026-10-18T11:43:54Z`.
Main CI `39122125020` and tag CI `39123016027` passed all six jobs each.
Signed trial `39122125492` and release `39123016084` passed; final package
`117662309169` and publication `117666236715` succeeded.

Independent anonymous public downloads checked every API size/digest and all
**39 files, 30 checksum entries, all 31 provenance subjects and seven SBOM
attestations** against the exact tagged workflow/source and hosted-runner
policy. All eight public vulnerability reports have zero HIGH/CRITICAL
findings; secret scan has zero findings. Signed Linux amd64 guest/normal smoke
passed. [Published receipt](published-release.json).

The public Windows archive is byte-identical to the actual recovery-tested
trial. The public database archive differs from trial and was independently
loaded and executed successfully; no equality or unexecuted runtime claim is
borrowed from trial. [Trial consumer receipt](consumer-trial.json).

## Passed gates

| Gate | Observed result |
| --- | --- |
| Full local D | Complete `npm.cmd run verify` passed with PostgreSQL 17.11, 28 release/policy tests, 40 client tests, 453 web tests, 715 locale messages, 235 bounded schemas, notices/contracts/lint/types and both builds; [receipt](local-gates.json). Web coverage: statements 81.61%, branches 77.8%, functions 85.48%, lines 83.11%. |
| Fresh browser B | All 14 actual production-browser cases passed: 64 loaded states, eight English/Persian profiles, RTL/LTR, both themes, keyboard/reduced motion, public-only PWA/cache/update and authenticated journeys; [receipt](browser-runtime.json), [measurements](browser-measurements.json), [screenshots](screenshots/). |
| Exact main CI | All six jobs in `39122125020` passed, including real PostgreSQL/race/browser, application image/chart, Compose, Helm/external TLS PostgreSQL and both real providers. |
| Strict signed trial | `39122125492` passed database/native/application/source/history scans, exact engine/helper/root-drop probes, bound SPDX and signing. No inherited database finding was suppressed or ignored. |
| Independent trial consumer | Artifact ZIP SHA-256 `2d4fc51024f5e419963ad8a1d3ee97ff6e4c743269b0b19025233a4f5f624450`; all 39 files, 30 checksums, all 31 provenance subjects and seven SBOM attestations verified. Wrong identity rejected. |
| Actual Windows archive recovery | Archive SHA-256 `9637166b56564fc940c3c75bf3a35e71768283b7cf36db296c4e2a494a0a5964` passed setup/read, source state/version, acknowledgement/SSE/replay, restart, PostgreSQL-native backup/empty restore, 16 migrations, protected history, revoked sessions/preauth, fresh login and ownership-loss shutdown. Go 1.26.9, CGO disabled, exact source and `vcs.modified=false`. |
| Actual trial database archive | Archive SHA-256 `90f7c434a392b6c7dbaab54305ed0f0aa33835ec3a84a6ca07f444cd956cfeba` was independently loaded on Linux amd64. Archive configuration SHA, loaded rootfs and execution configuration matched; PostgreSQL 17.11, gosu 1.19/Go 1.26.9 and root-drop UID/GID 70 passed. A read-only, network-none, capability-dropped server ran as UID 70 and answered SQL with `server_version_num=170011`. COPYRIGHT hash matched. Owned tmpfs/container cleanup passed. |
| Legal inventory | The production web notice check covered 63 exact production packages and 63 exact upstream legal files. This count is separate from the full source SPDX inventory. |

## Database boundary and retained corrections

The [canonical policy](../../../deploy/postgres-image.json) defines
`stackplore-postgres:1.0.1` from
`postgres:17.11-alpine@sha256:b0f9560a2de083e2cc7382e75f808c7381a32852a7ec49117deedb300e552b24`.
It retains the upstream PostgreSQL engine/entrypoint and rebuilds unchanged
official gosu 1.19 with Go 1.26.9, from source
`6456aaa0f3c854d199d0f037f068eb97515b7513`, archive SHA-256
`33d7537d588ea49458b9509bcf4554bdf5ceacc66da71e5caa1058ea3b689c3b`.
Compose/CI use the patched image; Helm keeps operator-owned external PostgreSQL.
Server and native clients require 17.11 or a later 17.x patch. Existing-installation
minor updates follow [upstream-based guidance](../../operations/upgrade-rollback.md#postgresql-17-patch-baseline)
([فارسی](../../operations/upgrade-rollback.fa.md)); no specific 17.6-to-17.11
customer upgrade is claimed.

Retain these failures at their original scope:

- Immutable `v1.0.0` at `a424d9f4c9f23d25960b93482b3096a6d05d0e1f` passed its recorded main/tag/trial/recovery checks, but publication `39117205810` was canceled for the unsafe database default and missing image scan. The API confirmed no release; [withheld receipt](../1.0.0/publication-withheld.json). The published 0.9.0 prerelease has that unsafe default and is superseded.
- Upstream-only candidate `a252718efffbdbb03c8f4552c25e998f481c77bb` passed local B, browser and all six main CI jobs `39119787227`, but trial `39119787671`, job `117652865830`, failed with 25 gosu Go 1.24.6 findings: one CRITICAL and 24 HIGH. Its clean 45-package Alpine OS result did not make the full image safe. No findings were suppressed; [failed receipt](failed-database-image.json).
- Derivative candidate `f046d523` built and probed the actual image and had zero HIGH/CRITICAL findings across 45 Alpine and four Go packages. Trial `39121495195` still failed on a valid versionless pinned-source main-module package URL; signing and archive recovery were not reached. Its main CI code job was canceled while five other jobs passed. The narrow format correction retained versioned dependencies and full identity/graph checks; [checkpoint](database-runtime-checkpoint.json).
- Earlier local image timeouts remain failed [attempts](local-image-attempts.json). A Windows helper scan with exit zero but zero analyzed executables was rejected. The later [helper receipt](helper-rebuild.json) records actual Linux execution and one analyzed executable with zero blocking findings using the fresh 2026-10-18 database.
- The first database consumer attempt rejected a Docker 29 image-ID assumption before runtime. The replacement checks archive configuration SHA, loaded rootfs diff IDs and execution configuration directly. No release artifact changed; the successful archive checks are in [consumer-trial.json](consumer-trial.json).

## Accepted publication and scope

All publication gates passed, including independent public downloads, exact
tag/workflow/source identity, 30 checksums, all 31 provenance subjects and seven
SBOM attestations. Public manifest SHA-256 is
`439ec1f5bd5a543b0b903cd4eccc3be3691614abd299c42ef99604e6abdd7854`.
The public Windows SHA-256
`9637166b56564fc940c3c75bf3a35e71768283b7cf36db296c4e2a494a0a5964`
matches the actual recovery-tested trial.

The different public database archive SHA-256 is
`8b9caa13e4a31b510fb7e2cc438d545ab295a0c0511837f1ddb67565f11af1db`.
Its configuration digest is
`sha256:372b3d466213f14583bf26c74345d22988fc312550f9248cbdaa555f30070843`.
Independent loading verified configuration/rootfs/execution identity, PostgreSQL
17.11, gosu 1.19/Go 1.26.9, UID/GID 70 root-drop, read-only/network-none/non-root
SQL with `server_version_num=170011`, COPYRIGHT identity and owned tmpfs cleanup.
The signed Linux native archive passed guest/normal smoke and graceful cleanup.
Source SPDX inventories 470 packages, 393 npm package URLs and 74 Go package
URLs; the production web notice check separately covers 63 exact packages/files.

Interrupted managed/curl transfers retain their nonzero outcomes and partial
bytes as consumer transport history. Final bounded public HTTPS/range resume
preserved normal certificate checks, every API size/digest and the signed
manifest; no artifact or trust policy changed. [Published receipt](published-release.json).

Runtime proof covers Windows amd64 and Linux amd64 native tools and Linux
amd64 application/database images. Linux arm64 and macOS outputs remain
cross-built-only. Automated accessibility checks do not certify every assistive
technology. Owned Compose/Helm/provider fixtures and tmpfs archive probes do not
certify customer credentials, ingress/TLS, receiver reachability, backups,
capacity, an air-gapped installation or an unrequested production rollout.
