# Final architecture and implementation review

Review date: 2026-10-18. Source:
`73ece4754554cca548c242768b195d21664d37b8`, immutable `v1.0.1`.
Architecture/code/release-chain review: **9/10** after actual checks.
**All nine stages are delivered and public release acceptance is complete**;
[publication receipt](published-release.json).

The reviewed boundaries include provider-neutral domain ports, transport and
jobs, PostgreSQL transactions/migrations/history, authentication/guarded writes,
bounded collection/streams, the bilingual interface, deployment images, release
identity/scans, legal materials and operator documents. No unresolved severe
implementation defect or missing accepted 1.0 feature was found in this final
scope. This is a review judgment, not measured coverage, an external penetration
test, universal accessibility certification or proof of a customer's deployment.

## Correction loop and evidence

| Pass | Score | Finding, correction and real result |
| --- | --- | --- |
| Earlier scoped review | Historical 9/10 | Preserved in the [withheld 1.0.0 review](../1.0.0/final-review.md); the later database-image finding reopened whole-release review. |
| Database-default review | 8/10 | PostgreSQL 17.6 default and missing database-image scan blocked publication. Immutable 1.0.0 was withheld; its passing tests stayed passing within their scope. |
| Upstream-only 1.0.1 | 8/10 | Candidate `a252718efffbdbb03c8f4552c25e998f481c77bb` passed local/browser and six-job main CI, but trial `39119787671` failed on one CRITICAL and 24 HIGH gosu Go-library findings. [Failed receipt](failed-database-image.json). |
| Derivative and identity fixes | 8/10 | Rebuilt unchanged official gosu 1.19 with Go 1.26.9, retained PostgreSQL 17.11 engine/entrypoint and added complete scans, bound SPDX, database archive and seventh SBOM. Empty-coverage Windows helper output was rejected; fresh Linux helper analysis covered one real executable. [Helper receipt](helper-rebuild.json). |
| Real report format correction | 8/10 | Trial `39121495195` on `f046d523` built/probed the real image and found zero blocking findings across 45 Alpine/four Go packages, then rejected a valid versionless pinned-source main-module package URL. The narrow correction accepts only that source form and preserves dependency versions and full inventory/graph identity. The trial remains failed; five other CI jobs passed while code was canceled. [Checkpoint](database-runtime-checkpoint.json). |
| Final actual verification | 9/10 | Full local D, fresh browser B, all six exact main CI jobs `39122125020`, signed trial `39122125492`, independent 39-file/30-checksum/all-31-provenance/seven-SBOM verification, actual Windows recovery and actual loaded database archive runtime passed. [Consumer receipt](consumer-trial.json). |

Full local D passed 28 release/policy tests, 40 client tests, 453 web tests and
all format/notices/contracts/lint/type/build gates with PostgreSQL 17.11.
Fresh browser B passed all 14 cases across 64 loaded states and eight profiles.
Web coverage is 81.61% statements, 77.8% branches, 85.48% functions and 83.11%
lines. The production web legal check covers 63 exact packages and 63 exact
upstream legal files. [Local receipt](local-gates.json),
[browser receipt](browser-runtime.json).

The derivative database image retained the pinned PostgreSQL 17.11 engine and
entrypoint and rebuilt gosu 1.19 from unchanged source
`6456aaa0f3c854d199d0f037f068eb97515b7513`, archive SHA-256
`33d7537d588ea49458b9509bcf4554bdf5ceacc66da71e5caa1058ea3b689c3b`.
Strict scans, helper/engine/root-drop probes, signed inventory and actual
Docker-save consumption passed without suppressing inherited findings. The
loaded archive passed configuration/rootfs/execution identity, UID/GID 70
root-drop, network-none/read-only/non-root SQL startup, version and COPYRIGHT
checks, with owned tmpfs/container cleanup. The Windows trial archive passed
the full native recovery drill with exact source, Go 1.26.9, CGO disabled and
`vcs.modified=false`.

An initial private database consumer check incorrectly treated Docker 29's
manifest identity as a configuration digest and was rejected before runtime.
The corrected check validates archive configuration SHA, loaded rootfs diff IDs
and execution configuration directly. It changed no release artifact; the
passing result and limitation are retained in [consumer-trial.json](consumer-trial.json).

## Accepted publication and limits

All six exact-tag CI jobs `39123016027` and release `39123016084` package/
publish jobs succeeded. Stable release `416416913` was published at
`2026-10-18T11:43:54Z`. Independent public downloads and exact tagged
workflow/source/hosted-runner checks verified all 39 files, 30 checksum entries,
all 31 provenance subjects and seven SBOM attestations. All eight public
vulnerability reports have zero HIGH/CRITICAL findings; full-history secret scan
has zero findings. Signed Linux amd64 guest/normal native smoke passed.

The public Windows archive equals the actual recovery-tested trial. The
different public database archive was independently loaded and executed:
configuration/rootfs/execution identity, PostgreSQL 17.11, gosu 1.19/Go 1.26.9,
UID/GID 70 root-drop, read-only/network-none/non-root SQL, COPYRIGHT identity
and owned tmpfs cleanup passed. [Published receipt](published-release.json).
Source SPDX has 470 packages, 393 npm URLs and 74 Go URLs, distinct from the 63
production web packages and exact legal files.

Interrupted public transfers remain consumer transport failures with partial
bytes/nonzero exits retained. Successful bounded range resume preserved normal
certificate checks, API sizes/digests and signed-manifest verification; it did
not change an artifact or trust policy.

The earlier 0.9.0 prerelease is superseded for its unsafe database default.
Immutable 1.0.0 remains withheld; its [cancellation receipt](../1.0.0/publication-withheld.json)
and all earlier successful/failed receipts retain their original scope.
Single-runtime ownership, fixed operator profiles, bounded loaded subsets,
truthful unsupported/denied states and cross-built-only native targets remain
explicit limits. Customer ingress, receivers, backups, capacity, air-gapped
installation and production rollout require separate operational evidence.
