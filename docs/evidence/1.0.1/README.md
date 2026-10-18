# 1.0.1 corrective acceptance — pending

Date: 2026-10-18. Rejected corrective candidate:
`a252718efffbdbb03c8f4552c25e998f481c77bb`, pushed but not tagged or accepted.
Its local gate B and all 14 browser cases passed. Exact
[main CI 39119787227](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39119787227)
then passed all six jobs. The
[artifact trial 39119787671](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39119787671)
failed its strict database-image scan before signing/publication.

All nine stages remain functionally implemented. Current whole-release review
score is **8/10**. A further 1.0.1 derivative-image correction is in progress;
its exact source and fresh runtime/build/scan/recovery/artifact proof are pending.
No `v1.0.1` tag or accepted publication is established.

Immutable `v1.0.0` remains at `a424d9f4c9f23d25960b93482b3096a6d05d0e1f`.
Its six-job main CI `39116327391`, six-job tag CI `39117205748`, signed trial
`39116335008`, independent 26-checksum/12-provenance/six-SBOM verification and
actual Windows recovery passed within their recorded scope. Publication run
`39117205810` was canceled after discovery of the vulnerable PostgreSQL 17.6
default and missing database-image scan. The release API confirmed no 1.0.0
GitHub release; the [withheld-publication receipt](../1.0.0/publication-withheld.json)
records that check. Preserve the tag and [checkpoint receipts](../1.0.0/README.md).
The published 0.9.0 prerelease has the same unsafe database default and is
superseded.

## Corrective database boundary

The [failed database-image receipt](failed-database-image.json) retains the
rejected trial and local reproduction. Hosted package job `117652865830`
successfully pulled the exact upstream image and ran the compiled PostgreSQL
17.11 probe, then failed the strict image scan. That job did not retain its scan
JSON. A fresh-database Trivy 0.76.0 Windows reproduction found **25 blocking
findings: one CRITICAL and 24 HIGH**, all in `/usr/local/bin/gosu`'s Go 1.24.6
standard library. Its 45 Alpine 3.24.2 OS packages had no blocking findings;
that narrower result does not make the complete image pass. No finding is
suppressed, excluded or accepted as an exception.

The canonical [database policy](../../../deploy/postgres-image.json) now defines
the intended Linux amd64 runtime image `stackplore-postgres:1.0.1`, derived from
`postgres:17.11-alpine@sha256:b0f9560a2de083e2cc7382e75f808c7381a32852a7ec49117deedb300e552b24`
by rebuilding unchanged official `gosu` 1.19 source with Go 1.26.9. Source commit
is `6456aaa0f3c854d199d0f037f068eb97515b7513`; its source archive SHA-256 is
`33d7537d588ea49458b9509bcf4554bdf5ceacc66da71e5caa1058ea3b689c3b`.
The correction retains the upstream PostgreSQL engine and entrypoint; actual
derivative build, engine/helper versions, root-to-database-user transition and
strict scan success remain to be proved. Compose and CI must build owned patched
fixtures first. Helm continues to use operator-owned external PostgreSQL under
the same server/client policy.

An independent Windows-to-Linux amd64 build of the pinned unchanged gosu source
actually executed under Linux and reported gosu 1.19 with Go 1.26.9. Linux Trivy
rootfs analysis covered **one real Go executable**, with zero HIGH/CRITICAL
findings using the existing 2026-10-16 vulnerability database. An earlier
Windows scan exited zero with zero analyzed executables and was rejected for
empty coverage. This standalone helper proof does not establish a fresh full
derivative-image scan, image startup/root-drop behavior or signed artifact proof.

PostgreSQL server and native clients require 17.11 or a later 17.x patch.
Configuration alone does not establish runtime or scan success.
Follow the [upgrade procedure](../../operations/upgrade-rollback.md#postgresql-17-patch-baseline)
([فارسی](../../operations/upgrade-rollback.fa.md)) for validated backup,
maintenance and applicable upstream configuration/cleanup/reindex review.

The release gate must retain `database-image.json` for base/derivative identity
and engine/helper probes, `database-image-scan.json` for the complete image
scan and `database.spdx.json` for the bound package inventory. Publish the actual
database Docker-save archive and its signed SBOM as well. The **expected,
unverified** totals are now **39 public assets, 30 checksum entries, all 31
provenance subjects and seven SBOM attestations**: five native archives, the
application image and the database image. The earlier upstream-only package
shape is superseded.

## Required receipts

| Gate | Status and required proof |
| --- | --- |
| Exact corrective source | Rejected source `a252718efffbdbb03c8f4552c25e998f481c77bb` is pushed. New derivative-image source identity and complete verification are pending. |
| Real database and native restore | Pending corrective-source execution on PostgreSQL 17.11 with readiness, ledger checks and native backup/empty-target restore. Existing 17.x minor-update guidance is upstream-documented and requires operator validation; this release does not claim a specific 17.6-to-17.11 runtime experiment. |
| Final derivative-tree local gate | Passed complete `npm.cmd run verify` C: 27 release/policy, 40 client and 453 web tests, real PostgreSQL 17.11, format/lint/types/contracts/builds; [receipt](local-gates.json). Earlier A/B passes retain their original scopes. |
| Final derivative-tree browser | Fresh B passed all 14 actual production-browser cases, 64 states and eight profiles; [receipt](browser-runtime.json), [measurements](browser-measurements.json), [public screenshots](screenshots/). Exact hosted source and packaged binary checks remain separate. |
| Exact main CI | Rejected-source run `39119787227` passed all six jobs. A complete new derivative-source six-job result is pending. |
| Standalone rebuilt helper | Pinned gosu 1.19 built with Go 1.26.9 actually executed on Linux amd64; Linux Trivy analyzed one Go executable with zero HIGH/CRITICAL findings using the fresh 2026-10-18 database; [receipt](helper-rebuild.json). Zero-coverage Windows output was rejected. This is not a fresh full-image scan or root-drop proof. |
| Database engine and scans | Upstream PostgreSQL 17.11 engine probe passed, but trial `39119787671` failed the image scan. Local reproduction: one CRITICAL and 24 HIGH findings in gosu. New derivative engine/helper/root-drop probes, matching full scan/SPDX and zero blocking findings are pending. |
| Other strict scans | Pending native executables, application image, source dependencies, licenses and full-history secret checks on the corrective source. |
| Signed trial | Rejected upstream-only trial failed before signing. Pending a new matching derivative-source trial, all payload digests, 30 checksums, all 31 provenance subjects and seven SBOM attestations; reject wrong identity. |
| Windows corrective archive recovery | Pending execution of the exact verified 1.0.1 archive: setup/read, acknowledgement/SSE/replay, restart, native backup/empty restore, protected history, revoked sessions/preauth, fresh login, version/source and ownership-loss shutdown. |
| Exact immutable tag | Pending `v1.0.1` on the checked source and all matching tag CI jobs. Never move `v1.0.0`. |
| Signed publication and public consumer check | Pending 39 public assets, independent signatures/digests/identity, 30 checksums, all 31 provenance subjects and seven SBOM attestations, plus comparison with the recovery-tested Windows archive. |
| Final reconciliation | Pending exact commands, exit results, run identities and sanitized receipts; update this file, the active plan and [corrective review](final-review.md) only from observed evidence. |

Return the review score to 9/10 only after actual corrective real-PostgreSQL,
browser, hosted deployment and strict scan gates pass. Passing those gates alone
does not establish signed publication or independent public-asset acceptance.
Record source, tool/engine versions, image digests, command, exit status and
evidence scope; never record credentials, session proofs or private runtime data.

Runtime acceptance targets Windows amd64 and Linux amd64 native tools, plus
Linux amd64 application and database images.
Linux arm64 and macOS archives remain cross-built until separately executed.
Compose/Helm fixtures are owned disposable environments; customer credentials,
provider permissions, ingress/TLS, receiver reachability, backup destination and
capacity still require that deployment's own checks. Historical receipts remain
historical and must not be relabeled as corrective or customer proof.

Earlier local image attempts timed out before analysis/runtime and remain
separate failed receipts: [record](local-image-attempts.json). The later actual
strict-scan failure is recorded above. Neither is proof of a safe derivative image.

The [independent helper receipt](helper-rebuild.json) now also records actual
Linux execution and one-executable analysis with the fresh2026-10-18 database.
It has zero HIGH/CRITICAL findings; full derivative-image acceptance remains
separate. The earlier Windows rootfs exit0 with zero executable coverage was
rejected and never counted as successful analysis.
