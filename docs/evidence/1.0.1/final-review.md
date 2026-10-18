# Corrective architecture and release review — pending

Review date: 2026-10-18. Active target: 1.0.1. Exact source and execution receipts:
pending. Scope includes provider-neutral architecture, PostgreSQL, transport,
jobs/streams, interface, deployment images, release scans, signed artifacts and
English/Persian operator documents. All nine stages are functionally implemented;
final corrective acceptance is still open.

## Score, findings and corrections

Current architecture/code/release-chain score: **8/10**. Earlier scoped 9/10
reviews predated the database-image finding and cannot close this review.

- [Deployment database]: Compose and CI used vulnerable PostgreSQL 17.6. A passing application test did not make that server safe.
- [Release scans]: The separately deployed database image was absent from OS/package scan proof. Application/native/source scans left that boundary uncovered.
- [Acceptance evidence]: Existing 1.0.0 receipts passed their recorded checks. Their coverage cannot establish corrected 1.0.1 runtime, deployment or publication.

The corrective tree pins PostgreSQL 17.11 and its upstream digest in the
[canonical database policy](../../../deploy/postgres-image.json). Release gates
add engine/digest metadata, a database OS scan and an SPDX inventory. Native
server/client minimums and bilingual upgrade documents now require 17.11 within
major 17, a validated backup, maintenance and applicable upstream configuration,
cleanup and reindex review. The complete local gate and all14 browser cases passed. The exact database image and hosted/scanned artifacts remain to be executed and
independently verified; they do not earn a higher score by themselves.

| Correction-loop step | Score | Evidence state |
| --- | --- | --- |
| Prior scoped review | Historical 9/10 | Preserved in the [1.0.0 review](../1.0.0/final-review.md); superseded for whole-release acceptance by the late database-image finding. |
| Late database review | 8/10 | Vulnerable database default and missing database-image scan identified; 1.0.0 publication canceled. |
| Corrective source and document work | 8/10 | Pin and new scan/identity gates defined. Real corrective execution receipts remain pending. |
| Required reassessment | Pending | Return to 9/10 only after actual corrective real-PostgreSQL, browser, hosted deployment and strict scans pass. Record exact source and run identities first. |

## Open acceptance gates

The [acceptance checklist](README.md) requires real PostgreSQL 17.11 and native
backup/empty-target restore, the full local gate, actual production-browser
matrix, all six exact-source hosted CI jobs, strict database/native/application/
source/history scans, signed trial and actual Windows 1.0.1 recovery. Exact tag
checks, signed publication and independent public-asset acceptance are further
required before final delivery. Expected counts are 37 assets, 29 checksum
entries, 30 provenance subjects and six archive/image SBOM attestations; they
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
Trivy0.76 historical output with18 APKs passed the parser and six independent
identity/inventory mutations were rejected. This parser check does not prove
the current database image; exact hosted runtime and scans remain required.
