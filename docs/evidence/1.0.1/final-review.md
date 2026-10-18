# Corrective architecture and release review — pending

Review date: 2026-10-18. Active target: 1.0.1. Pushed candidate
`a252718efffbdbb03c8f4552c25e998f481c77bb` is rejected after actual trial
`39119787671` failed the strict database-image scan. No 1.0.1 tag or accepted
publication exists. The derivative-image replacement and its exact source/proof
are pending. Scope includes provider-neutral architecture, PostgreSQL, transport,
jobs/streams, interface, deployment images, release scans, signed artifacts and
English/Persian operator documents. All nine stages are functionally implemented;
final corrective acceptance is still open.

## Score, findings and corrections

Current architecture/code/release-chain score: **8/10**. Earlier scoped 9/10
reviews predated the database-image finding and cannot close this review.

- [Deployment database]: Compose and CI used vulnerable PostgreSQL 17.6. A passing application test did not make that server safe.
- [Release scans]: The separately deployed database image was absent from OS/package scan proof. Application/native/source scans left that boundary uncovered.
- [Acceptance evidence]: Existing 1.0.0 receipts passed their recorded checks. Their coverage cannot establish corrected 1.0.1 runtime, deployment or publication.
- [PostgreSQL 17.11 image]: The upstream engine probe passed but its full image scan failed. Fresh local analysis found one CRITICAL and 24 HIGH gosu standard-library findings. All remain blocking.

The corrective [database policy](../../../deploy/postgres-image.json) defines
`stackplore-postgres:1.0.1` from the same pinned PostgreSQL 17.11 base, retaining
its engine and entrypoint. It rebuilds unchanged official gosu 1.19 source
`6456aaa0f3c854d199d0f037f068eb97515b7513` with Go 1.26.9; source archive SHA-256
is `33d7537d588ea49458b9509bcf4554bdf5ceacc66da71e5caa1058ea3b689c3b`.
Actual build, gosu/Go versions, root-drop behavior, engine and full-image scan
success remain pending. No base finding is suppressed or ignored. Release gates
require engine/helper metadata, a complete database image scan, bound SPDX,
the database Docker-save archive and its signed SBOM. Native
server/client minimums and bilingual upgrade documents now require 17.11 within
major 17, a validated backup, maintenance and applicable upstream configuration,
cleanup and reindex review. The rejected source passed full local gate B
(24 release/policy, 40 client, 453 web tests) and all 14 browser cases. Those
successes and all six jobs in exact main CI `39119787227` remain passing within
their scope. The new derivative tree needs a
fresh full gate and exact hosted/runtime/scan/recovery/artifact proof.

| Correction-loop step | Score | Evidence state |
| --- | --- | --- |
| Prior scoped review | Historical 9/10 | Preserved in the [1.0.0 review](../1.0.0/final-review.md); superseded for whole-release acceptance by the late database-image finding. |
| Late database review | 8/10 | Vulnerable database default and missing database-image scan identified; 1.0.0 publication canceled. |
| Upstream-only corrective candidate | 8/10 | Local B and 14 browser cases passed; exact upstream pull/engine probe passed. Trial `39119787671`, job `117652865830`, failed the strict image scan. [Failed receipt](failed-database-image.json). |
| Derivative-image correction | 8/10 | Rebuild gosu 1.19 with Go 1.26.9 from pinned unchanged source, retain PostgreSQL engine/entrypoint and require all findings to clear. Actual replacement proof is pending. |
| Independent standalone helper | 8/10 | Pinned source cross-built on Windows for Linux amd64, actually executed as gosu 1.19/Go 1.26.9. Linux Trivy covered one real Go executable with zero HIGH/CRITICAL findings using the existing 2026-10-16 database. Empty-coverage Windows output was rejected; complete derivative proof remains pending. |
| Required reassessment | Pending | Return to 9/10 only after actual corrective real-PostgreSQL, browser, hosted deployment and strict scans pass. Record exact source and run identities first. |

## Open acceptance gates

The [acceptance checklist](README.md) requires real PostgreSQL 17.11 and native
backup/empty-target restore, the full local gate, actual production-browser
matrix, all six exact-source hosted CI jobs, strict database/native/application/
source/history scans, signed trial and actual Windows 1.0.1 recovery. Exact tag
checks, signed publication and independent public-asset acceptance are further
required before final delivery. Expected counts are 39 assets, 30 checksum
entries, all 31 provenance subjects and seven SBOM attestations; they
are not observed counts yet.

Immutable `v1.0.0` remains at `a424d9f4c9f23d25960b93482b3096a6d05d0e1f`.
Its main/tag CI, trial consumer verification and Windows recovery remain passing
within their original scope. Publication run `39117205810` was canceled; the
release API confirmed no GitHub release for that tag; see the
[withheld-publication receipt](../1.0.0/publication-withheld.json). The earlier published
0.9.0 prerelease has the unsafe database default and is superseded. Keep tags,
successful receipts and failed attempts intact.

Scores are review judgments, not measured coverage, external penetration-test
results, universal accessibility certification or customer deployment proof.
Single-runtime ownership, fixed operator credential profiles, bounded loaded
subsets, truthful unsupported/denied states, cross-built-only native targets and
customer-owned ingress/receiver/backup/capacity checks remain explicit limits.

The independent scan review also found an unbound database SPDX document.
The correction binds image name/configuration, document/container/OS relations
and the complete reachable APK inventory to the actual scan, including bounded
dependency cycles. Eighteen release tests and six policy tests passed; actual
Trivy 0.76 historical output with 18 APKs passed the parser and six independent
identity/inventory mutations were rejected. This parser check does not prove
the current database image. The later real scan rejected the upstream-only
image despite 45 Alpine 3.24.2 OS packages having no blocking findings: all 25
blocking findings were in `/usr/local/bin/gosu`'s Go 1.24.6 standard library.
The hosted job did not retain scan JSON; the fresh Windows Trivy 0.76.0
reproduction is in the [failed receipt](failed-database-image.json). Exact
derivative engine/helper/root-drop runtime, scans and artifacts remain required.

The final derivative-tree full local C passed all 27 release/policy tests, 40
client tests and 453 web tests with the complete format/lint/types/contracts/
notices/build gates. Fresh production-browser B passed all 14 cases. Independent
helper analysis also passed with the fresh 2026-10-18 database and one real Go
executable. Whole-release acceptance remains 8/10 until the complete exact
hosted derivative image and signed artifact/recovery gates pass.
