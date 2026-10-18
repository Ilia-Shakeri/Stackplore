# 1.0.1 corrective acceptance — pending

Date: 2026-10-18. Exact corrective source, tag and run identities: **pending**.
All nine stages are functionally implemented. Current whole-release review
score is **8/10** while corrective runtime, deployment and scan proof is open.
The final local gate and all14 browser cases passed; exact hosted deployment and signed artifact gates remain open.

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

The canonical [database policy](../../../deploy/postgres-image.json) selects
`postgres:17.11-alpine@sha256:b0f9560a2de083e2cc7382e75f808c7381a32852a7ec49117deedb300e552b24`
for Linux amd64. PostgreSQL server and native clients must be 17.11 or a later
17.x patch. Configuration alone does not establish runtime or scan success.
Follow the [upgrade procedure](../../operations/upgrade-rollback.md#postgresql-17-patch-baseline)
([فارسی](../../operations/upgrade-rollback.fa.md)) for validated backup,
maintenance and applicable upstream configuration/cleanup/reindex review.

The release gate must retain `database-image.json` for exact engine/digest
identity, `database-image-scan.json` for OS vulnerability analysis and
`database.spdx.json` for the package inventory. These are three additional
publication assets, bringing the **expected, unverified** totals to 37 public
assets, 29 checksum entries and 30 provenance subjects. The six native
archive/application-image SBOM attestations remain separately required.

## Required receipts

| Gate | Status and required proof |
| --- | --- |
| Exact corrective source | Pending reviewed commit identity, consistent 1.0.1 metadata and clean release inputs. |
| Real database and native restore | Pending corrective-source execution on PostgreSQL 17.11 with readiness, ledger checks and native backup/empty-target restore. Existing 17.x minor-update guidance is upstream-documented and requires operator validation; this release does not claim a specific 17.6-to-17.11 runtime experiment. |
| Full local gate | Passed final complete `npm.cmd run verify`: 24 release/policy, 40 client, 453 web tests, real PostgreSQL17.11 server composition, format/lint/types/contracts/builds; [receipt](local-gates.json). |
| Browser | Passed all14 actual production-browser cases, 64 loaded states, eight profiles, English/Persian authenticated journeys, public-only cache/install/update and 80 paced reads; [receipt](browser-runtime.json), [measurements](browser-measurements.json), [public screenshots](screenshots/). |
| Exact main CI | Pending all six jobs on the recorded corrective source, including race/real PostgreSQL/browser, application image/chart, Compose, Helm/TLS and both real providers. |
| Database engine and scans | Pending exact approved image index/platform/configuration identity and actual engine version, matching OS scan, SPDX package inventory and zero blocking HIGH/CRITICAL findings. |
| Other strict scans | Pending native executables, application image, source dependencies, licenses and full-history secret checks on the corrective source. |
| Signed trial | Pending matching source/workflow/ref identity, all payload digests, 29 checksum entries, 30 provenance subjects and six archive/image SBOM attestations; reject wrong identity. |
| Windows corrective archive recovery | Pending execution of the exact verified 1.0.1 archive: setup/read, acknowledgement/SSE/replay, restart, native backup/empty restore, protected history, revoked sessions/preauth, fresh login, version/source and ownership-loss shutdown. |
| Exact immutable tag | Pending `v1.0.1` on the checked source and all matching tag CI jobs. Never move `v1.0.0`. |
| Signed publication and public consumer check | Pending 37 public assets, independent signatures/digests/identity, 29 checksums, 30 provenance subjects and six archive/image SBOM attestations, including comparison with the recovery-tested Windows archive. |
| Final reconciliation | Pending exact commands, exit results, run identities and sanitized receipts; update this file, the active plan and [corrective review](final-review.md) only from observed evidence. |

Return the review score to 9/10 only after actual corrective real-PostgreSQL,
browser, hosted deployment and strict scan gates pass. Passing those gates alone
does not establish signed publication or independent public-asset acceptance.
Record source, tool/engine versions, image digests, command, exit status and
evidence scope; never record credentials, session proofs or private runtime data.

Runtime acceptance targets Windows amd64, Linux amd64 and the Linux amd64 image.
Linux arm64 and macOS archives remain cross-built until separately executed.
Compose/Helm fixtures are owned disposable environments; customer credentials,
provider permissions, ingress/TLS, receiver reachability, backup destination and
capacity still require that deployment's own checks. Historical receipts remain
historical and must not be relabeled as corrective or customer proof.

Local image attempts timed out before analysis/runtime. They are failed receipts,
not proof of a safe image; [record](local-image-attempts.json). Hosted exact-image
engine/OS/SPDX verification is required before acceptance.
