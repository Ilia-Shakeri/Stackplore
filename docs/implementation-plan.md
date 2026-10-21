# Stackplore active implementation evidence

## Version 2 status — 2026-10-21

Stage-by-stage implementation, versioning, commits and pushes are authorized.
The [version 2 ledger](v2/implementation.md) owns new commands, failures,
source bindings and pending gates. Accepted design is separate from delivered
runtime. Existing 1.0 evidence below remains historical and immutable.

Stage-0 **2.0.0-alpha.0** is published, schema **17**, immutable
`eb86ab7554db555f023c1de72c3119acf101226a`. Its exact main verification
[39258522963](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39258522963)
and tag verification [39262174884](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174884)
pass all six jobs. Signed trial `39258604051` and publication
[39262174841](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174841)
pass; prerelease [417357745](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v2.0.0-alpha.0)
exists. [Public acceptance](evidence/2.0.0-alpha.0/published-release.json) verifies
39 files, 30 checksums, 31 provenance subjects, seven SBOM attestations and eight
strict scans with zero HIGH/CRITICAL findings. Public Windows bytes match the
actual recovery-tested trial. Public application archive load/identity passed
without starting the container. The separate public PostgreSQL archive passed
engine/helper/root-drop, isolated read-only bounded SQL and owner-checked
six-container cleanup/absence verification. **Stage 0 is accepted within this
recorded scope.** The [evidence index](evidence/2.0.0-alpha.0/README.md) keeps
local/hosted/browser/consumer proof and retained failures separate.

The [separate preparation status](v2/preparation-status.md) identifies unaccepted
draft PR 1 at `e660e16bd0b3c56f0096bb43b5688195135892b1`, covering preparation
for stages 1–3. A new explicitly authorized native startup and bilingual typed
protocol probes now pass for the alpha.1 candidate; see its
[source-bound receipt](evidence/2.0.0-alpha.1/README.md). The old rejected launch
and draft checks retain their historical scope. Full local gate C passes on
tree `695c705a379ae46f3eb570009efab10654ae6f85`, including 544 web, 40 client,
48 release and 11 corpus tests, cached Go checks and production build. Actual
unchanged PostgreSQL/native execution passed in attempt B. The
[local receipt](evidence/2.0.0-alpha.1/local-gate.json) retains these cache/source
boundaries and both earlier failed complete attempts. Final receipt/document
edits, seven exact-commit hosted jobs and publication remain separate.
Version-2 stages **1–10 are
not complete**; this protocol proof does not establish answer quality.


The previous alpha.1 source `f1ae80391aca646fca89fed7a5ecf79271999c97`
passed seven hosted jobs and fourteen actual browser cases, but signed trial
`39592142674` failed on CVE-2026-111943 in the Go 1.26.9 standard library
embedded in gosu. No alpha.1 tag was created. The Go 1.26.10 source correction
requires fresh local, hosted and artifact acceptance;
[retained failure and toolchain proof](evidence/2.0.0-alpha.1/README.md#go-security-correction--2026-10-21). Earlier compiler results do
not accept the correction.

The local checkout retains modified existing files and untracked preparation
after cleanup was review-rejected. Its migration 18 applies automatically on
normal startup even when the capability is disabled. For accepted alpha.0
execution use a verified archive or clean exact-source export; retained
preparation must use a fresh disposable schema-18 test database. Alpha.0's
schema-17 receipts do not accept this mixed working tree.

## Accepted corrective audit — 1.0.3 — 2026-10-19

Immutable v1.0.3 is **656581cf05d4774358d5644a6e1af3c69a210f81**, schema **17**.
The [fresh audit](evidence/1.0.3/fresh-audit.md) records nine defect areas,
two required controls and every reopened correction loop. The
[final review](evidence/1.0.3/final-review.md) is **9/10**; all **nine stages**
are complete within the recorded scope. Earlier releases and failures retain
their own sources and results.

| Gate                           | Exact command, outcome and scope                                                                                                                                                                                                                                                                                                                                                                              |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Retention/settings             | Real PostgreSQL 17.11 focused A/C and full SQL gates pass; all seven settings, shared-lock ordering and per-batch reload are covered. The wrong-column fixture B remains failed.                                                                                                                                                                                                                              |
| Authority and bounded streams  | Initial/queued/replay denial and replacement ownership pass. Revocation commits before cancellation. Native oversized pages produce bounded resets/refetches. Complete Linux HTTP/realtime detector checks pass uncached in 29.022s/2.039s; the fixture amendment preserves all production limits and maximum-payload HTTP tests.                                                                             |
| Interface                      | Matrix composition, code points, 16 MiB production response budget, settings-derived alert drafts, nine retry views, source-bound environment navigation and three fixed-window refresh regressions pass. Locale parity is 738 messages; OpenAPI checks cover 235 bounded schemas.                                                                                                                            |
| Complete local                 | Full local F (npm.cmd run verify) passes 530 web/53 files, 40 client/2 files, 29 release checks, all Go/native tests and builds with unchanged implementation manifest. F precedes the final test-only lifecycle amendment; later complete Windows API/core checks and uncached Linux detector proof pass separately. [F](evidence/1.0.3/local-gate-f.json), [amendment](evidence/1.0.3/lifecycle-race.json). |
| Exact hosted main/tag          | Main 39235362032 and tag 39238088634 each pass all six jobs on their first attempts: full verification, real PostgreSQL/detector, actual Docker/Kubernetes providers, Compose and external-TLS Helm. The chart also passes 23 typed local render profiles and strict lint. [Run metadata](evidence/1.0.3/release-gates.json).                                                                                 |
| Exact production browser       | Actual main artifact independently validates 14 sole-attempt cases, 64 journeys/eight profiles, four environment journeys and 24 PNG signatures; four final images visually reviewed. All 80 measured reads pass with unchanged 8/10/600-ms bounds and fixture p95 45ms. [Proof](evidence/1.0.3/main-browser.json). Local browser B remains earlier-scope evidence.                                           |
| Signed trial                   | 39235380836 passes. All 39 files/30 checksums/31 provenance subjects/seven SPDX attestations/eight strict scans verify; actual Windows backup/restore/replay/ownership and independent trial database archive execution pass. [Trial](evidence/1.0.3/consumer-trial.json).                                                                                                                                    |
| Public publication/consumption | 39238088607 passes package and publication. Stable release 417182030 was published 2026-10-19T10:51:50Z. All 39 public files and signatures verify; the explicit wrong-source digest is rejected. Public Windows bytes match the actual tested trial; the different public database archive independently passes isolated SQL/runtime and cleanup. [Public receipt](evidence/1.0.3/published-release.json).   |
| Retained failures              | Main 39230365227 failed code/Helm; main 39233787971 failed only the heavy lifecycle fixture. Their signed trials passed but remain unaccepted. Old-code negatives, fixture/tool-preflight/extraction failures and the first public TLS-connection failure keep their outcomes; the normal-TLS retry succeeded without a bypass.                                                                               |

The final documentation/evidence follow-up is separate from frozen release
source. All six main jobs on the commit containing this review must pass before
handoff; its exact final revision/run binding belongs to commit checks and the
reconciled release body. No recursive receipt commit is required.

Unrelated untracked user artifacts remain untouched and outside commits and
release payloads. The owned local PostgreSQL fixture was stopped after public
runtime acceptance; its PID/listener absence was verified. No checksum, scan,
ownership, assertion, coverage or production timeout was weakened.

Customer rollout, provider rights, ingress/TLS, receiver delivery, capacity and
backup destinations require the installation's own proof. Windows amd64 and
Linux amd64 have the recorded runtime evidence; macOS/Linux arm64 outputs are
cross-built and scanned only. Public acceptance does not certify a customer
deployment.

## Fresh corrective audit — 1.0.2 — 2026-10-18

The reviewed baseline is documentation commit `657a600`; immutable 1.0.1
remains at `73ece4754554cca548c242768b195d21664d37b8`. The changed tree is an
accepted 1.0.2 release with schema **17**. The review score
reopened at **8/10**. Final independent backend/interface/release-helper review
is **9/10** with no open verified defect in that scope; whole-delivery acceptance
is **9/10** after publication and independent public/archive proof; all nine
roadmap stages are complete.
[Fresh audit](evidence/1.0.2/fresh-audit.md) records verified triggers and fixes.
Candidate `18e8558e76d5c2fec4eb1d752bceaa43aaa1f260` was pushed, then
superseded by the verified retained-event REST/SSE response correction. Its
main CI `39131357982` and trial `39131368817` retain their earlier-source scope;
the trial package and all six main jobs passed only for that earlier source.
Current immutable `v1.0.2` is
`376ed0379900ab870dd8eb8af6b55ef3e5ec389b`, schema 17. Full local D, exact
main/tag six-job CI, strict trial and actual trial archive recovery pass.
Signed publication and independent public consumption pass in the
[published receipt](evidence/1.0.2/published-release.json).

| Gate                                   | Current result                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| -------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Backend URL/HTTP regressions           | Initial and independent apostrophe/clipping/JSON-member/embedded-scheme probes failed before fixes. Later multi-layer escaped-slash cases failed 21 leak cases with six safe controls; corrected whole package passes 1.250s, independent three-case overlay passes 1.159s and full corrected gate C passes. All four real 405 route regressions pass.                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| Retained event REST/SSE                | Ten leaking summaries and the later-poll test failed before correction; four safe controls passed. All 14 transport cases plus real SSE polling pass in 6.991s, full HTTP/masking packages in 5.821s/0.810s. Shared mapping applies deployment redaction then an 8,192-rune limit without changing retained rows/backups.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Observation and PostgreSQL regressions | Later-page event denial, non-byte-collation notification/rule keysets, ledger-14/16-to-17 upgrade and actual native special-name backup/restore pass. All five related packages pass uncached with native PostgreSQL 17.11 tools; the current complete local receipt also passes.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| Interface regressions                  | Severity URL/localization, locked deleted-rule drafts, topology facts and account rendering have 65 unique focused passes across ten files. Final rule/users run passes 29 tests in three files (22.06s); scoped lint/type checks pass. Five palette checks and the scoped production-interface browser pass separately.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| Release-note policy                    | Immutable document-link helper and 29 release/policy checks pass. Signed trial and exact-tag public publication also pass.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| Full local gate                        | [Current precommit replacement D](evidence/1.0.2/local-gate.json) passes exit 0 after the retained-event amendment through final web/API builds: all Go/native/core, client 40/2 files, release/policy 29, web 477/49 files, locale 725 and schemas 235. Normal checksum/native-tool/coverage/path/ownership policies remain enabled; prior failures keep their outcomes.                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Local chart check                      | Actual strict Helm lint passes one chart (`.tmp/reaudit102-helm-lint.log`); no install/TLS-runtime result follows from lint.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| Production browser                     | [Local UI snapshot](evidence/1.0.2/browser-gate.json) retains its 14 cases/64 routes/eight profiles/24 images (four visually reviewed). All 14 production-browser cases also passed exact main CI and unchanged-tag code retry on `376ed03`; original first tag failure is retained.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| Exact-source main/tag CI               | Main `39132989641` passed all six jobs. Tag `39134154907` passed after one unchanged-source code-job retry, with the other five original jobs green. Real SQL/race/browser, images, Compose, external-TLS Helm and both providers passed.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Strict trial and independent consumer  | Trial `39133022021` package passed. [Current consumer](evidence/1.0.2/consumer-trial.json) verified 30 payload checksums, all 31 provenance subjects and seven SBOM attestations, rejected wrong source, checked eight clean strict scan reports and executed actual Windows recovery plus Linux native/database archives.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| Tag release/public consumer            | Release `39134154891` first refused packaging after failed tag CI. Retry passed package `117702264862` and publication `117703943506`. Release `416503671`, published `2026-10-18T14:43:05Z`, has independent [39-file public proof](evidence/1.0.2/published-release.json): 30 manifest entries, all 31 provenance subjects/seven SBOM attestations, wrong-source rejection, eight clean scans and actual archive identity/execution.                                                                                                                                                                                                                                                                                                                                                                    |
| Maintainer harness follow-up           | [Actual diagnostic](evidence/1.0.2/browser-fixture-diagnostic.json) retains original tag/rate failures and main `7545339`/CI `39138645362` failure in unmeasured setup before load (13/14 browser pass, setup API 429, document status unknown, PWA pass). Four 2500-ms setup refills and safe document diagnostics preserve exact 80/8/10/600-ms workload/assertions. Focused exact callback on signed `376ed03` passes 86 setup responses and all 80 timed reads/valid envelopes; ten diagnostic checks and cleanup pass. Fourteen worker in-memory cases pass; live C/D retry-coverage guards remain failed. This main-only work changes no frozen public source/product limit. Six-job CI of the containing review commit must pass before handoff; commit checks/release body bind final run/source. |

Backend focused command passed with exit 0:

`go -C apps/api test ./internal/provider/logsafe ./internal/httpapi -run 'TestMessage|Test.*AllowedMethod|TestAuthRouteMethodAllowSets|TestWriteProviderProblem|TestLogDownload' -count=1`.

Private proof: `.tmp/audit-backend-20261018/affected-tests.log` and its exit
marker (0). Earlier private overlay probes and blocked build attempts remain
separate. No checksum, assertion, ownership rule or test threshold was weakened.
The audit baseline's manual account-table load-more path accumulated DOM rows.
The corrected 50-row render window passes bilingual three-page/103-account and
final-administrator checks while retaining access to all loaded records. The
manually requested cache may still grow until refresh/unmount.
Historical proof below
retains its exact version, counts, commits and outcomes.

Current D web coverage is 82.91% statements, 78.68% branches, 86.71% functions
and 84.48% lines; the standard thread pool completed 477 tests in 206.67s.
B/C's earlier coverage/timing remain in the receipt's historical property.
The web build checks 63 exact legal packages/files. Native tools were
explicitly enabled and module checksum verification stayed normal. Failed
lint/API/cleanup attempts are retained; local success does not establish
public release acceptance. Exact hosted main/tag results are recorded separately;
the local browser receipt retains its precommit UI snapshot scope. Current trial
artifact ZIP SHA-256 is
`92bde1ce3544271c82e3bf26a204aebae89da0658cc2c078feb43b8f8bfb7efb`.
Windows recovery used archive
`d7c1e7807b5b71946c2df274da07945e4c284421ae425f9b29febe2d36ddde1b`
and 17 migrations; actual database execution used archive
`c73d5d409fd344f8058dcc673b2ecde1a08456b60eff28eeb7a1e7baae88c082`.
The public Windows archive matches those recovery-tested bytes. The different
public database archive
`9c5e686b1350c4ac21c68ad9234199efb04b5c9f95bf695ed8311060844ac79f`
was independently loaded and passed identity, PostgreSQL 17.11, gosu 1.19/Go
1.26.9, root-drop, restricted SQL and owned cleanup. Public manifest SHA-256 is
`c3409cc8300212fcbf27e83a1524ba0c64291e9f0503b8d9c7ebaf9789e45bf1`.
The public receipt keeps runtime/platform/customer limits explicit.

## Historical accepted 1.0.1 release — 2026-10-18

**All nine stages are delivered; release 1.0.1 is accepted.** Architecture/code/
release-chain review is **9/10** at immutable `v1.0.1`, source
`73ece4754554cca548c242768b195d21664d37b8`. Stable release `416416913`
was published at `2026-10-18T11:43:54Z`.
[Published receipt](evidence/1.0.1/published-release.json).

| Gate                                | Observed final result                                                                                                                                                                                                                                                                                                                                                |
| ----------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Full local D                        | Complete `npm.cmd run verify` passed with real PostgreSQL 17.11, 28 release/policy tests, 40 client tests, 453 web tests, 715 locale messages, 235 bounded schemas, notices/contracts/lint/types and both builds; [receipt](evidence/1.0.1/local-gates.json).                                                                                                        |
| Fresh browser B                     | All 14 production-browser cases passed across 64 loaded states and eight English/Persian profiles, RTL/LTR, keyboard/reduced motion and public-only PWA/privacy/update; [receipt](evidence/1.0.1/browser-runtime.json).                                                                                                                                              |
| Exact main/tag CI                   | Main `39122125020` and tag `39123016027` passed all six jobs each, including real PostgreSQL/race/browser, image/chart, Compose, Helm/external TLS PostgreSQL and both real providers.                                                                                                                                                                               |
| Trial/publication                   | Signed trial `39122125492` passed. Release `39123016084`, package `117662309169` and publish `117666236715` succeeded.                                                                                                                                                                                                                                               |
| Independent public consumer         | All 39 public files, 30 checksums, all 31 provenance subjects and seven SBOM attestations verified against exact tag/source/workflow and hosted-runner policy. Every API size/digest matched.                                                                                                                                                                        |
| Vulnerability/secret/license checks | All eight public vulnerability reports have zero HIGH/CRITICAL findings; secret scan has zero findings. Source SPDX: 470 packages, 393 npm URLs, 74 Go URLs. Production web notices: 63 exact packages and 63 exact upstream legal files.                                                                                                                            |
| Actual Windows archive              | Public SHA-256 `9637166b56564fc940c3c75bf3a35e71768283b7cf36db296c4e2a494a0a5964` equals the full recovery-tested trial: setup/read, state/version, acknowledgement/SSE/replay, restart, native backup/empty restore, protected history, revocation, fresh login and ownership-loss shutdown passed. Go 1.26.9, CGO disabled, exact source and `vcs.modified=false`. |
| Actual public database archive      | Different from trial; SHA-256 `8b9caa13e4a31b510fb7e2cc438d545ab295a0c0511837f1ddb67565f11af1db` independently loaded/executed. Configuration/rootfs/execution identity, PostgreSQL 17.11, gosu 1.19/Go 1.26.9, UID/GID 70 root-drop, read-only/network-none/non-root SQL, license hash and owned tmpfs cleanup passed.                                              |
| Linux native smoke                  | Signed Linux amd64 archive passed guest/normal startup and graceful cleanup; owned databases removed.                                                                                                                                                                                                                                                                |

Web coverage is 81.61% statements, 77.8% branches, 85.48% functions and 83.11%
lines; backend core is 80.6%/82.6%. The accepted derivative retains the pinned
PostgreSQL 17.11 engine/entrypoint and rebuilds unchanged official gosu 1.19
with Go 1.26.9. Full image/SPDX and runtime proof passed without suppressions.
Compose/CI use the derivative; Helm keeps external PostgreSQL.
[Trial receipt](evidence/1.0.1/consumer-trial.json),
[final review](evidence/1.0.1/final-review.md).

Correction history remains honest: immutable 1.0.0 passed its recorded tests
but publication `39117205810` was canceled for the unsafe database default and
missing scan; 0.9.0 is superseded. Upstream-only `a252718` passed local/browser/
main CI but trial `39119787671` failed on 25 gosu findings. Derivative `f046d523`
passed actual build/probes/scans but trial `39121495195` rejected a valid
versionless pinned-source main-module package URL; main code CI was canceled
while five other jobs passed. The narrow format correction preserved dependency
versions and full inventory/graph checks before local D and successful exact
trials. Empty-coverage helper output and local image timeouts remain failed.

Initial consumer Docker 29 identity assumptions were corrected in the private
harness without changing release artifacts. Interrupted public transfers retain
partial bytes and nonzero outcomes as transport history; final bounded resume
kept normal certificate checks, API sizes/digests and signed-manifest policy.
[Failed image](evidence/1.0.1/failed-database-image.json),
[report-format checkpoint](evidence/1.0.1/database-runtime-checkpoint.json),
[withheld publication](evidence/1.0.0/publication-withheld.json).

No unresolved severe implementation defect or missing accepted 1.0 feature
remains in final review. Customer production, credentials, ingress/TLS, receivers,
backup destinations, capacity and full air-gapped installation require separate
operator proof. Linux arm64 and macOS outputs remain cross-built/scanned-only.
Dated history below retains its original checkpoint scope.

## Withheld 1.0.0 verification history — 2026-10-18

The following records preserve the chronology before the late database-image
finding. Current corrective status above takes precedence over their earlier
scores and then-pending gates. The 1.0.0 whole-repository requirement review
initially retained architecture/code at 9/10 after independent corrections.
The documentation/failure-path review initially scored 8/10: stale
security/project/development/contribution text and authentication test failure
messages could misstate support or print proof-bearing objects. Corrections
remove those values without changing assertions or runtime controls, reconcile
all current stage/installation claims and define the actual supported matrix.
The corrected code/doc review then scored 9/10 within that earlier scope;
the later database-image finding reduced the current whole-release score to
8/10. See [withheld checkpoint evidence](evidence/1.0.0/README.md).

Stage 8 source `20f8c2e30cb2ccdfc16aa266f0b0d5647c429881` passed all six jobs in
[exact main CI 39038430422](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39038430422).
[Artifact trial 39038439418](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39038439418)
passed all package/native/image/source/history scans, extracted Linux runtime
smoke and provenance/SPDX signing. Its archive digest matched the artifact API;
all 26 checksum entries matched. Independent consumer verification passed for
12 provenance subjects and all six archive/image SBOM attestations, requiring
the exact main workflow/ref/source and hosted runner. A wrong source digest
was rejected. Official trust metadata initially returned 403 to the CLI; using
the existing system network proxy allowed normal official TUF chain verification.
The verified root was then supplied for offline consumer checks. No signature
or identity check was disabled. [Sanitized receipt](evidence/0.9.0/consumer-trial.json).

The actual verified Windows trial archive passed setup, nine-resource reads,
source version/state, alert acknowledgement, SSE change/replay, process restart,
native PostgreSQL backup/empty-target restore, 16 migration entries, protected
history, session/preauth revocation, fresh login, exact runtime version and
nonzero ownership-loss shutdown. These are fresh owned databases, not customer
data. Linux archive smoke is separately in the signed `native-smoke.json`.

The legal-classifier replacement full local gate reached all passing stages:
real PostgreSQL 177.158s, backend core 80.6%/82.6%, 40 client tests, 453 web tests
in 47 files, ten release tests, frontend statements 81.53%, branches 77.49%,
functions 85.17%, lines 83.05%, and the built web/embedded assets. The resumed
session no longer exposed its process handle; the final API build was explicitly
rechecked and returned exit 0. The preceding timeout attempt remains failed:
setup returned a safe 503, its fixture cleanup timed out, the SQL package hit its
deadline and one mutual-TLS fixture timed out. Both focused cases passed unchanged
on rerun, followed by the complete replacement above; no timeout/assertion was
weakened.

Annotated `v0.9.0` was pushed on that unchanged checked source. Exact
[tag CI 39114483036](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39114483036)
and [release 39114483033](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39114483033)
passed: all six exact-tag jobs and package/publication succeeded. Independently
downloaded all 34 public assets, checked all 26 signed-manifest entries and
verified 12 provenance subjects plus all six SBOM attestations against the exact
tag workflow/source with hosted-runner enforcement. The public Windows archive
is byte-identical to the archive used in the actual recovery drill. The first
public transfer was interrupted; its partial receipt remains separate and every
resumed file required its API digest before signed-manifest verification.
[Published receipt](evidence/0.9.0/published-release.json) closes Stage 8.
At that earlier point the 1.0.0 local/browser/exact-CI/artifact/recovery/tag/
publication gates were pending; later 1.0.0 results are recorded below. The 0.9.0
receipts were never promoted to proof of 1.0. Its database default is now superseded.

Final local attempt A failed two real-PostgreSQL connection gates after the host
restart: the owned loopback test cluster was stopped. The existing workspace
data directory and port 55473 were verified before starting that cluster; the
separate PostgreSQL service on port 55432 was not changed. Native crash recovery
completed and readiness then passed. The failed receipt remains
`.tmp/stage9-final-verify-a.log` with exit 1. Replacement B reran the complete
gate with the same assertions, limits and disposable test URL.

Final replacement B **passed exit 0**, with the exit marker retained. All Go
packages passed against the real owned PostgreSQL 17.11 cluster; changed
full-server and HTTP SQL tests ran uncached (8.953s and 6.816s), while unchanged
repository tests reused valid cache. Core coverage is 80.6%/82.6%. The gate also
passed 40 client tests, 453 web tests in 47 files, ten release tests, 715 locale
messages, 235 bounded schemas, format/lint/types/contracts/notices and both builds.
Frontend coverage is 81.53% statements, 77.49% branches, 85.17% functions and
83.05% lines. Current Helm lint/render and document/link checks passed.

The freshly embedded 1.0.0 server then passed **all 14 browser cases**, exit 0:
64 loaded route states across eight language/width/theme profiles, WCAG 2.2 AA
rules, layout/keyboard/reduced motion, public-only offline/cache/installability,
explicit update acceptance/reload, all 80 bounded concurrent read envelopes and
normal English/Persian setup/recent-auth/role/history/download/logout journeys.
All 16 public screenshots were freshly captured. Chrome was 155.0.8059.80.
[Sanitized final local/browser receipts](evidence/1.0.0/README.md) bind the
observed outputs and screenshots; at that point exact CI/artifact/recovery/tag
gates remained, with their subsequent outcomes recorded below.

Final source was committed and pushed as
`a424d9f4c9f23d25960b93482b3096a6d05d0e1f`. [Exact main CI
39116327391](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39116327391)
passed all six jobs. [Signed trial 39116335008](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39116335008)
passed packaging, Linux archive execution, all five native three-executable
scans, image OS/server scan, source gomod/pnpm scan, full-history scan and signing.
Source SPDX inventories 470 packages, including 393 npm and 74 Go package URLs;
there are zero blocking findings. Independent trial verification passed all 26
manifest entries, 12 provenance subjects and six SBOM attestations under the
exact main workflow/ref/source and hosted-runner policy.

The verified 1.0.0 Windows archive passed the full owned-database native recovery
drill, including exact version/source, 16 migrations, nine resources, 972 metric
samples, source version/state, durable acknowledgement/SSE/replay, restart,
backup/empty-target restore, protected audit, revoked sessions/preauth, fresh
login and nonzero shutdown after ownership loss. Its build metadata reports
Go 1.26.9, Windows amd64, CGO disabled, exact source revision and
`vcs.modified=false`. [Consumer receipt](evidence/1.0.0/consumer-trial.json).
After all local runtime tests finished, the owned test PostgreSQL cluster was
stopped cleanly, retaining its data and all diagnostic receipts.

Annotated `v1.0.0` was pushed on that unchanged checked candidate. Exact
[tag CI 39117205748](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39117205748)
passed all six jobs. [Release 39117205810](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39117205810)
was subsequently canceled after the database-image finding; the release API
confirmed no published 1.0.0 release. No tag was moved. This checkpoint is
withheld; corrective 1.0.1 acceptance is governed by the current section above.

The following sections retain the dated Stage 8 and preceding correction
history. The current final-gate status above takes precedence.

## Stage 8 correction history — 2026-10-17

Stage 7 replacement [CI 39031584134](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39031584134)
passed **all six jobs** on exact commit
`64fd4391113cebc53e99bdd09a3d81bbff6054ce`. Actual Compose setup, UI/readiness,
restart/session preservation, actual Helm installation with separate TLS
PostgreSQL, image/chart validation, real SQL/race and both provider jobs passed.
Annotated `v0.7.0` was created at that unchanged commit and pushed. Exact tag
[CI 39032396854](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39032396854)
also passed all six jobs. The three failed attempts below remain failed receipts.

Stage 8 `0.9.0` is implemented in the working tree. The final changed-tree full
gate with Go 1.26.9 and the expanded production-browser matrix have passed.
Exact remote verification, artifact trial, consumer signature verification and
publication remain open.
[Candidate receipts](evidence/0.9.0/README.md) keep SQL performance, dependency
scans, native restore and browser proof separate. Scope includes mounted safe
Kubernetes profiles, additive log redaction, streamed log download, system event
facts, source-creation audit, event filters, health history, metric trends and
coalesced live reads, public-only offline/update behavior and signed packaging.

Correction loop: initial candidate 8/10; fixes addressed unindexed deep pages,
stale source-wide event clocks, missing atomic source-creation audit, missing
upstream legal text and vulnerable pinned transitive packages. Scoped independent
architecture/code review returned 9/10 after corrections. Whole-candidate and
release acceptance remain separate. A full SQL test found an old guest-reset
assertion assuming the pre-existing audit count; corrected exact audit checks
pass while preserving all guest authentication/outbound guarantees.

Further actual browser review lowered the candidate score to 8/10: waiting only
for a page title had hidden loading/error states, the overview started duplicate
read batches on initial clock/stream messages and hit the default rate limit,
and Chromium rejected the SVG install icon. The stronger loaded-data/browser
gate is retained. Corrections coalesce overview refreshes and fence canceled
responses, replace install icons with raster exports of the owned vectors, and
test update delivery through a fixed local test proxy. The final Go 1.26.9 browser
run passed all 14 tests: 64 loaded route states, four-path offline/privacy,
regular-profile installability, explicit update acceptance/reload, 80 bounded
reads and English/Persian account/role journeys. A browser-installed-to-waiting
worker race was reproduced and fixed with a focused regression. Test-only
separate clients refill the unchanged shared loopback rate bucket between cases;
neither request limits nor assertions were weakened. The mounted profile exposed a source
settings constructor allowlist omission; its negative-first regression, all
five reviewed profiles and actual full-server startup against real PostgreSQL
and a scoped HTTPS Kubernetes fixture now pass.

The complete Go suite passed with the disposable SQL URL (PostgreSQL package
276.112s), before the later mounted-startup test. A separate measured web run
passed 449 tests in 46 files: statements 81.23%, branches 77.17%, functions
85.05%, lines 82.76%. These are not the final changed-tree gate. Full gate B
passed format/docs/version/notices/locales/contracts, then failed 19 strict lint
errors in new components/tests and the coverage script; all were corrected
without disabling rules. The Windows formatter also outgrew the
process argument limit; it now uses bounded batches of 32 files. Failed and
interrupted receipts are retained separately from the final replacement run.

Full gate D passed on Go 1.26.5, but executable-aware rootfs scanning then found
33 HIGH findings across 11 standard-library CVE IDs in the three native tools.
Zero-language filesystem scans were rejected as incomplete. Release scanning
now requires every shipped native executable, the server and OS in the image,
and Go-module SPDX package URLs. Go was upgraded to 1.26.9, including CI,
container digest, bootstrap, legal inventory and release metadata. Its official
download endpoints returned 404 here; the official `actions/go-versions` mirror
archive was downloaded and SHA256-verified instead. The local pinned tool was
updated while preserving its predecessor. Native cache maintenance freed space;
no user artifact or failed diagnostic receipt was removed.

The **complete `npm.cmd run verify` with Go 1.26.9 passed exit 0** using the real
disposable PostgreSQL URL, followed by current chart render/schema and document
checks. It includes core coverage thresholds, 40 client tests, 453 web tests,
nine release-safety tests, locale/contract drift, lint/types and both builds.
The final browser runner also exited 0 after all 14 cases and owned database
cleanup. Current rootfs scan analyzes all three rebuilt Linux binaries, reports
zero HIGH/CRITICAL findings and inventories 97 SPDX packages. Fresh official-DB
dependency analysis executed both gomod and pnpm analyzers, including development
dependencies, with zero HIGH/CRITICAL findings. Its exact copied input hashes,
scope and raw report hash are retained in the sanitized receipts. The slower
interrupted Windows-mount filesystem traversal is not counted as a pass.

The corrected architecture and code score **9/10**. Release/completion proof
remains separate until exact CI, signed artifacts, consumer verification and
the final 1.0 review/tag/publication close. See [browser](evidence/0.9.0/browser-runtime.json),
[native security](evidence/0.9.0/native-security.json) and
[requirements](evidence/0.9.0/requirements.md).

The first exact 0.9.0 candidate `f36e405288a1fbcc0576365520240db265779856`
failed [verification 39037869138](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39037869138)
and [artifact trial 39037871184](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39037871184).
The legal scanner had classified a Lucide icon source file named `copyright.mjs`
as a notice. Its generated `dist` copy was ignored by Git, so the Linux checkout
correctly rejected the incomplete manifest. The scanner now distinguishes source
extensions from legal material, with a regression for both classes; the container
copies the shared classifier. The regenerated inventory preserves all 63 actual
notices for 63 packages. The failed runs remain failed; replacement verification
and artifact trial must pass before tagging.

The following Stage 7 and earlier paragraphs are chronological historical
snapshots. The latest status above takes precedence over their pending wording.

Stage 7 correction loop: candidate `1a39e780abed587104092eab33ec3da0256a02d9`
failed [CI run 39025933317](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39025933317).
The code, image/chart, Docker-provider and Kubernetes-provider jobs passed;
Compose could not reach readiness and the TLS PostgreSQL rollout timed out.
Do not count this run as install evidence. The TLS fixture now uses Alpine's
PostgreSQL group 70; bounded service diagnostics are retained on future failure.
An independent review also found the complete HTTP boundary overwrote embedded
HTML/JavaScript MIME and CSP. The boundary now consumes an internal asset marker,
keeps the trusted asset MIME, allows same-origin shell resources, and preserves
immutable caching only for hashed assets. A complete-router regression test
checks HTML, SPA, JavaScript, private API isolation and marker removal. Scoped
HTTP and web asset tests pass. The release gate remains open.

Second Stage 7 run [39030046196](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39030046196)
passed code, image/chart and both provider jobs. Its diagnostics proved Compose
injected an unset Matrix reference as an empty value, rejected by strict config.
Optional notification fields now stay absent unless the operator supplies them.
PostgreSQL TLS became ready after correcting the Alpine group. Helm then reached
the app, which rejected the group-accessible PVC root as `storage_start`.
The chart now uses a non-root init to prepare a private child and retain the
existing key identity, then mounts that child at the original application path.
No runtime permission validation was relaxed. The rendered init passed a real
Linux fixture: fresh/restart modes, stable legacy key, duplicate/no-overwrite,
symlink, hardlink and malformed-key rejection. Exact replacement CI is pending.

Run [39030871663](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39030871663)
passed code, Compose installation/restart, image/chart and both provider jobs.
Helm progressed past storage and correctly refused remote first-run auth with
no operator proof (`authentication_start`). The disposable install fixture now
provides a randomly generated expiring bootstrap proof through its runtime
Secret and enables the chart's setup profile. No auth guard was weakened.

Published through Stage 6: `v0.6.0` is at commit
`365dc214161bf74e0f77aaedc9d32529c1ec1cf2`. [Main CI run
39022543132](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39022543132)
and [tag CI run
39023109917](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39023109917)
passed on that exact commit. Both ran the full local gate, Go race tests, image
and chart checks; the tag run also passed disposable Docker and Kubernetes
guarded-writer jobs.

Stage 7 `0.7.0` is in progress. Current changes embed the production web bundle
in the Go server, make Compose use one application service, remove the separate
Nginx chart workload, add a Secret-mounted PostgreSQL CA option, and define CI
for fresh Compose setup/restart plus a disposable Helm install against TLS
PostgreSQL. The latest completed local Go tests, 402 web tests, API build and
strict Helm render/lint are recorded below after the final changed-tree gate.

Stage 6 local full gate: `npm.cmd run verify` passed with Node 24.18.0, Go 1.26.5, CI mode, and disposable PostgreSQL 17 at localhost port 55473. It includes real migration and repository tests, audit lifecycle verified-cutoff/bounded-delete proof, 36 API-client tests, 402 web tests, type checks and both production builds. `git diff --check` passed. Exact remote provider-write proof passed in the linked tag run.

Stage 5 publication follow-up: candidate `e04bc622ddb22e08a9c493000edd4c18cc0dc717`
was committed and pushed on 2026-10-17. The first
[remote run 39018000098](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39018000098)
failed workflow admission before any job ran: job-level environment expressions
cannot use `runner.temp`. No remote code, race, image or provider success is
claimed for that run. The temporary kubeconfig is now assigned from the step's
`RUNNER_TEMP` and exported for subsequent steps. Actionlint 1.7.7 reproduced the
old rejection and passed the corrected workflow, with optional shell/Python
linters explicitly disabled for this context-only check. Receipts remain
`.tmp/stage5-ci-context-before.log` and `.tmp/stage5-ci-context-after.log`.
The application source and final full local gate are unchanged by this workflow
correction. Replacement remote execution and the final annotated tag remain
pending; Stage 6 implementation has begun in the working tree.

Review dates: 2026-10-12–17. Target: 1.0.0. Stages 1–6 are published
checkpoints through `0.6.0`; local, CI and target proof remain separately named.
Stage 5 provider fidelity passed its local gate and disposable provider/browser
receipts. Stage 6 passed exact main and tag CI with disposable provider writers.
Current work is Stage 7; no 1.0 deployment-completeness claim follows from these
checkpoints. Stages 8–9 remain open.
[Roadmap](roadmap.md) ([فارسی](roadmap.fa.md)) defines ordered exit gates.
[Audit](codebase-audit.md) records coverage and defects.
[Historical evidence](history/implementation-plan-2026-08.md) retains prior results.

On 2026-10-17 the maintainer explicitly renewed authorization to test,
version, commit and push each remaining stage. This supersedes the earlier
no-push instruction for this task. The unchanged `v0.3.0` implementation
`7815e52a2e71b8452e1863f798beec5b7b0c8fb9` was pushed with its existing tag.
[Main run 39007896683](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39007896683)
and [tag run 39007898666](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39007898666)
completed successfully. Code/full real-PostgreSQL/race and image-build/chart
jobs passed; neither run proves a Compose start or a cluster install.

## Review entry and boundaries

Entry commit `08028f4` had substantial uncommitted PostgreSQL/provider/product/notification/deployment work. It was preserved, reviewed and integrated. An existing unreferenced dashboard image stays outside this release because capture provenance was not established. No contributor/runtime data was reset or deleted.

Review covered domain/auth/HTTP/providers/realtime/alerts/notifications/operation scaffold; both persistence generations, composition roots and tests; web/interface/client/contracts; scripts/deployment/workflows and docs. Legacy storage was removed only after real PostgreSQL parity and native restore proof. The system PostgreSQL service was not changed; all SQL proof used a separate repo-local cluster and disposable databases.

Before the first tag, untracked SQL was normalized once to LF with one final
newline, so local embedded bytes match the Git index/fresh checkout. The final
SQL and native gates were rerun on new disposable data after this step. Existing
development database history was not changed; see [upgrade constraints](operations/upgrade-rollback.md).

## Correction loop

| Pass                     | Scoped score | Remaining baseline problems                                                                                      | Correction                                                                                 |
| ------------------------ | ------------ | ---------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| Entry                    | 5/10         | Auth/ownership/outbox races, log leaks, invalid success replies, source races, first boot and contradictory docs | Whole-code review and ordered roadmap                                                      |
| 1                        | 7/10         | Key lifetime race, restore ownership/scope/durability, chart YAML/mount/forwarded scheme and false metric states | Fixes plus real SQL/HTTP/provider/frontend regressions                                     |
| 2                        | 8/10         | New version check import/lint, web test formatting, incomplete setup quick start and unchecked final gates       | Fix scripts/docs/format and rerun relevant gates                                           |
| Independent scope review | 9/10         | No known severe defect in delivered stage-1 fixes                                                                | Backend/frontend/PostgreSQL review; final gate and release diff required before completion |

These historical scores judge stage-1 infrastructure and corrected code, not
measured coverage or whole-product completion. At that baseline, stages 2–9 and
target proof were open. The stage 2 candidate and remaining 1.0 scope are
recorded separately below. The final architecture still needs accepted target
alignment.

## Stage 1 changes

- Common auth transaction serialization preserves setup/current actor/final-admin invariants. Queries and whole transactions use deadlines.
- Ownership precedes migrations; dedicated same-session probes detect loss, seal storage and stop service/jobs. Pool minimum capacity, shutdown and ledger reads are bounded.
- Notification allocation/enqueue share a transaction lock; a late earlier sequence cannot be stranded. Leases/retries/terminal audits remain.
- 18 real auth/product repository parity contracts replace legacy storage behavior. SQLite code/migrations/dependencies/licenses and busy-timeout setting are removed. Key/path/single-link checks remain in `runtimefiles`.
- Bounded non-audit PostgreSQL prune exists. Scheduled quotas and separately protected audit expiry remain future work.
- Bad input returns a problem, denied/outage codes stay truthful, metric replies cap at 200, source/health summaries mark partial/unknown, exact resource reads work beyond the first page.
- Shared plain/structured log scrub masks bearer/basic/cookies/keys; repeated pod container names remain distinct; webhook attempts close idle sockets.
- Explicit web source requests, stable details/history/search, absent metric states, strict runtime guards, real predicate translations and Persian log direction have regressions.
- Setup environment/Secret inputs, pod mount ownership, chart annotation YAML, HTTPS forwarding, edge probe separation and Compose filesystem/resource bounds are corrected. Real source profiles default off.
- Native backup/restore covers public schema, has a 15-minute bound and maintenance ownership monitor; publication syncs and cannot overwrite.
- Current guides/roadmap/audit, consistent 0.1.0 metadata, document/link/version gates, changelog and pinned workflow actions are included.

## Exact local context

Windows PowerShell; Node.js 24.18.0, Corepack 0.35.0, pnpm 11.17.0, Go 1.26.5. PostgreSQL server/native clients 17.11 from the existing installation ran separately with repo-local data on loopback55473. Password-free trust is limited to this disposable test cluster and is not production guidance. Helm 3.17.3 was downloaded into ignored tools and verified against its published SHA-256. No global tool install occurred.

Docker/Compose, live providers/receiver, a cluster and a local C race compiler are absent from this proof. Linux CI has a race command; its outcome is separate from plain local Go tests.

```powershell
$repoRoot = (Get-Location).Path
$env:PATH = "$repoRoot\.tools\node-v24.18.0-win-x64;$repoRoot\.tools\go\bin;$env:PATH"
$env:CI = 'true'
$env:GOCACHE = "$repoRoot\.tmp\go-build-v1"
$env:STACKPLORE_TEST_POSTGRES_URL = 'postgresql://stackplore_test@127.0.0.1:55473/stackplore_test?sslmode=disable'
npm.cmd run bootstrap
npm.cmd run verify
```

The test URL contains no password and names only a disposable test database. Remove its test-only variable before runtime commands. Fixtures use unique schemas and a shared test advisory gate. Native restore uses fresh whole databases and private runtime-key directories.

## Historical 0.1 checks

| Command/check                                              | Result                                                                                                                                                    |
| ---------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `npm.cmd run bootstrap`                                    | Passed pinned tools/frozen lock                                                                                                                           |
| Focused HTTP/provider/notification Go suite                | Passed; real PostgreSQL HTTP setup/login/session/grant/user/logout and protected-data scan ran                                                            |
| `go -C apps/api test ./...` with real URL                  | Passed after legacy removal; latest ledger/maintenance regressions also passed                                                                            |
| PostgreSQL parity/concurrency                              | 18 auth/product contracts plus ownership, query deadlines, migration rollback/unknown/changed history, outbox commit order and protected retention passed |
| Web tests/types/lint                                       | 160 tests passed, types/lint passed, locale parity 345                                                                                                    |
| `check:contracts`                                          | 181 bounded schemas and generated output parity passed                                                                                                    |
| `check:docs`, `check:version`                              | Inventory/217 local links and consistent 0.1.0 metadata passed after final document edits                                                                 |
| Helm lint/default/setup+cluster+ingress/annotation renders | Actual Helm 3.17.3 passed; no pod/cluster claim                                                                                                           |
| Native backup/archive-list/empty-target restore            | Passed17.11; ledger6, users1, sessions0, preauth0, generation1 checked                                                                                    |
| Restored runtime                                           | Ready200, old session401, fresh admin login/source/audit reads passed                                                                                     |
| Own ownership-session termination                          | Runtime stopped with failure within monitor bound; latest marker `NATIVE_BACKUP_RESTORE_SESSION_REVOCATION_AND_OWNERSHIP_STOP_PASS`                       |
| Final `npm.cmd run verify`                                 | Passed exit0 with real PostgreSQL URL: all Go packages,36 client tests,160 web tests,345 locale messages,181 schemas, lint/types/builds                   |
| Remote workflow/race/images                                | Passed for tagged implementation `72f8e466cbf81d5e00c541ce09e60d5e886a9e81`; links below                                                                  |

Remote evidence for the immutable 0.1.0 implementation revision:

- [Tag run 38262601532](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/38262601532): completed successfully; code and deploy jobs passed.
- [Main run 38262599330](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/38262599330): completed successfully; code and deploy jobs passed.
- The code job ran the full gate against real PostgreSQL and `go -C apps/api test -race ./...` on Linux.
- The deploy job validated the Compose model, linted/rendered the reviewed chart profiles, and built both images. It did not start Compose or install a cluster.

The follow-up evidence documentation commit does not change runtime code or
retag the tested implementation. It records these results after the runs ended.

Failures were repaired: entry Go operation formatting; later web test formatting; version-check URL import/regex lint; empty annotation-map Helm YAML. Native helper field-name errors were corrected using fresh disposable databases. An interrupted validation session stopped tool jobs; the disposable PostgreSQL cluster recovered its own WAL, then final checks/native drill were rerun. This does not prove a production recovery.

## Remaining 1.0 scope

Stages 1–6 are published checkpoints. Stage 5 provider/guest behavior passed its
scoped real-target and browser gates; Stage 6 passed exact main/tag CI, including
disposable guarded writers. Stage 7 now implements built-web serving, a Compose
first-install/restart workflow, and a Helm CA mount with a disposable TLS
PostgreSQL install job. Exact Stage 7 local/remote outcomes are recorded in the
current entry above. Its native backup/restore, upgrade and rollback evidence
still needs exact-tree verification. Stage 8 covers PWA, accessibility,
performance, security scans, SBOM/provenance and full bilingual operations;
Stage 9 is the independent 1.0 review and final release.

## Stage 2 checkpoint — 2026-10-13

The durable observation/source checkpoint adds domain capture/read/retention
ports, PostgreSQL adapters and migrations 7–8, bounded collection and fair
retention jobs, cached provider reads, fixed operator-owned profile registration,
and exact administrator source toggles with one-use purpose-bound recent auth.
All six released 0.1 migrations remain byte-for-byte unchanged. Profile and
observation writes have source/version/generation checks and bounded batches;
partial capture cannot retire unseen records. Logs remain live and are not
persisted. See the [English](observations.md) and [Persian](observations.fa.md)
behavior documents for bounds and supported workflows.

The review covers every new backend/domain/SQL/job/transport/source UI module
and affected contracts, manifests, tests and documentation. Retention preserves
the current metric per resource/name, synthetic fixture history, pending and
audit-linked delivery rows. Audit policy has a 90-day minimum, bounded quota, operator-verified archive cutoff, immutable policy-change audit and bounded scheduled expiry that refuses to run without archive proof. Namespace
profiles and restricted production Docker remain stage 5, with their live proof
separate. The static demo alarm is preserved through a narrowly validated
compatibility wrapper until the durable alert stage.

| Correction pass        | Scoped score | Remaining issue                                                                                                   | Applied correction                                                                                                                              |
| ---------------------- | ------------ | ----------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Initial implementation | 7/10         | Missing runtime job wiring, incomplete source UI/contract and safe mutation proof                                 | Composition lifecycle, closed routes/types, bilingual UI, exact transaction tests                                                               |
| First self review      | 8/10         | Mutable cursor anchors, denied/stale cache truth, retention starvation/current deletion, unfinished capture truth | Immutable signed sort values, source/port status overlays, fair quotas/latest protection, partial in-progress status                            |
| Independent review     | 8/10         | Old authorization timestamp, multi-read snapshot race, cross-source event IDs, demo metadata/alarm regression     | Decode then prepare, database clock after auth lock, coherent bounded SQL read snapshot, source/revision IDs and explicit fixture compatibility |
| Final scope review     | 9/10         | No known severe defect within the delivered checkpoint; broader 1.0 and target gates remain open                  | Independent backend/SQL/frontend/docs review, complete local gate and exact-tree native drill                                                   |

These scores apply only to the delivered checkpoint. Whole 1.0 completion,
accepted target architecture alignment and live deployment evidence are still
open and cannot inherit this score.

Historical candidate checks before the final gate:

- Source SQL transaction/replay/expiry/wrong target/state/purpose/revoked actor
  and failed-audit rollback regressions passed against real PostgreSQL 17.11.
- PostgreSQL capture/restart/6-to-current upgrade/keyset/long Unicode/status/
  retention tests passed; final coherent-read regressions awaited the gate then.
- Web source UI tests, strict types, lint and build passed; both catalogs have
  391 keys. Long infrastructure cursors accept 4,096 and reject 4,097 bytes;
  account input remains 1,024.
- OpenAPI policy and regenerated transport output passed for 186 bounded schemas.
- A disposable API proved first capture, exact source disable, replay rejection,
  stale cached reads and persisted state after process restart. Native backup
  and restore ran, but the helper expected 18 metric samples while collection
  correctly records all nine supported names across six resources (54 samples).
  The fixture assertion was corrected and the fresh native drill then awaited rerun.

Local warm-cache `EXPLAIN (ANALYZE, BUFFERS)` used 2,100 synthetic/native
resources, 21,000 metric points and 10,000 events in isolated fixtures. Reviewed
inventory/name, event keyset, current metric, bounded prune and audit paths use
their relevant indexes. Example timings reported by the test were approximately
0.160ms inventory/51 rows, 0.044ms events/51 rows, 0.366ms current metrics/100 rows
and 0.867ms prune/250 rows. The initial full-table latest-metric check was
replaced with an indexed newest-row probe. These are fixture measurements,
not production capacity, cold-cache or latency/SLO proof. Exact reproducible
commands and final outcomes are recorded below after the completed gate.

## Final 0.2 local evidence

| Exact command/check                                                                                                    | Observed result                                                                                                                                                                                                                |
| ---------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `npm.cmd run bootstrap`                                                                                                | Passed pinned tools and frozen lock; four workspace manifests agree at 0.2.0                                                                                                                                                   |
| `npm.cmd run verify` with the same disposable real PostgreSQL URL above                                                | Exit 0: format/docs/version/locales/contracts/lint/types/all Go tests/client/web tests/API and web builds                                                                                                                      |
| PostgreSQL within the full gate                                                                                        | Passed 46.013s; no environment-gated SQL skip in this run                                                                                                                                                                      |
| Client/web                                                                                                             | 36 client tests and 206 web tests passed; 391 locale keys matched                                                                                                                                                              |
| Contract output                                                                                                        | 186 bounded schemas and regenerated TypeScript output matched                                                                                                                                                                  |
| SQL focused upgrade/capture/read/retention suite                                                                       | 21 new test functions; full package 41.759s, final focused snapshot/history/progress checks 4.066s                                                                                                                             |
| `go -C apps/api test ./internal/postgres -run TestObservationHotPathQueryPlansOnRealPostgreSQL -count=1 -v`            | Passed; five plans reviewed, final log 3.270s. Exact seed: 2,103 resources, 2,003 snapshots, 21,003 metric samples, 10,003 events and 10,000 audit rows                                                                        |
| Helm 3.17.3 `lint` and `template` with `api.setup.enabled=true`, `api.kubernetes.enabled=true`, `ingress.enabled=true` | Passed; actual cluster binding appeared. No cluster-install claim                                                                                                                                                              |
| Latest compiled API/storage native drill on new whole disposable databases                                             | Passed `STAGE2_CAPTURE_TOGGLE_REPLAY_RESTART_NATIVE_RESTORE_OWNERSHIP_PASS`                                                                                                                                                    |
| Native invariants                                                                                                      | Ledger 8, users 1, resources 6, metrics 54, disabled source/version 2 retained; sessions/preauth 0 and restore generation 1                                                                                                    |
| Actual HTTP/lifecycle drill                                                                                            | First persisted capture/fixture metadata, exact authenticated toggle, replay 409, stale reads, process restart, native backup/list/restore, old-session 401, fresh login/source/audit reads and terminal ownership loss passed |

The final native drill used PostgreSQL/native tools 17.11 and fresh private
test databases `stackplore_stage2_native_d` and `stackplore_stage2_restore_d`
on the same isolated loopback cluster. It did not touch the system service or
user data. All raw authentication values were generated in process memory and
not printed or committed. Runtime keys, archives, helper executables and logs
remain ignored private test artifacts.

Two helper mistakes were corrected without relaxing product checks: the sample
assertion was changed to the nine-name fixture contract (54), and removing
already absent setup environment names became idempotent. A fixture for old
authorization time also needed an earlier installation creation time so its
invented historical setup respected database constraints. The complete gate
and final fresh native drill passed after these corrections.

The 0.2 scope score is 9/10 after independent SQL/backend/frontend/document
review. The first six migration byte diffs remain empty. Real live provider,
browser geometry/accessibility, local race tooling, Compose startup, cluster
install/TLS and target production evidence remain open. Remote CI must be
checked for the exact tagged implementation before claiming that boundary.

## Final 0.2 remote evidence

Implementation commit `b8448bf2016a466e522263ae32983e1f2c6af232` and annotated
tag `v0.2.0` were pushed without force. The remote tag peels to that exact
implementation revision. Both workflows completed successfully on 2026-10-13:

- [Tag run 38381333991](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/38381333991): code and deploy jobs passed.
- [Main run 38381330095](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/38381330095): code and deploy jobs passed.
- Code ran the complete gate with real PostgreSQL and the Linux Go concurrency
  gate (`go -C apps/api test -race ./...`). Local Windows race tooling remains
  absent; the Linux result is actual separate proof.
- Deploy validated the Compose model, linted/rendered reviewed Helm profiles
  and built both images. It did not start the application through Compose,
  install a cluster or prove live infrastructure/receiver/browser flows.

This follow-up documentation records completed remote results. It changes no
runtime behavior and does not rewrite the implementation tag. The unknown
dashboard image remains unstaged; private test cluster/data/archive artifacts
remain ignored. The disposable PostgreSQL process was stopped after proof;
the system PostgreSQL service was not changed.

## Stage 3 — 0.3.0 alert evidence and delivery

Date: 2026-10-16. Scope includes covered alert evaluation, fresh claim fencing,
durable source-scoped replay, idempotent demo seed, webhook/Matrix lanes,
secret-free settings/test transport, bilingual interface and operator documents.
The first eight released migrations remain byte-identical; new migrations 9–10
are canonical LF before final native/SQL verification. Existing unknown
`artifacts/stackplore-dashboard-example.png` remains outside the change.

### Correction loop

| Pass                              | Scoped score | Remaining issue                                                                                                          | Correction                                                                                                                                      |
| --------------------------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Initial implementation            | 7/10         | Latest-only windows, incomplete event scope, cache/outage confusion, transient replay and missing Matrix lane            | Covered raw evidence ports, durable changes, shared safe outbound and separate lanes                                                            |
| Review                            | 8/10         | Claims age in serial queues, configuration changes reuse evidence, backdated middle-prune gap, stale lease/result writes | Fresh one-at-a-time claims, configuration/version fences, durable prune watermark and atomic episode closure                                    |
| Integration review                | 8/10         | Unchecked stream metadata/query, reset freshness, late interface writes and open-stream shutdown                         | Closed typed metadata/query, reset/change refetch, recovered snapshot publication, interface generation fences and stream shutdown cancellation |
| Final independent review and gate | 9/10         | No known severe defect in the delivered stage; target and later-stage limits stay open                                   | All local gates, real PostgreSQL, disposable HTTPS, final native C and reviewed migration bytes passed                                          |

Scores judge this delivered stage, not whole-project completion, measured test
coverage or production capacity. Stage 3 is a completed local development
checkpoint. At that checkpoint Stages 4–9 were open; current later-stage results
are recorded below. No whole-product or target-readiness claim is made.

### Executed focused proof

| Command/check                                                                              | Result                                                                                                                                                                                                                                                                                                          |
| ------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `npm.cmd run bootstrap`                                                                    | Passed pinned tools and frozen lock at 0.3.0                                                                                                                                                                                                                                                                    |
| `go -C apps/api test -count=1 ./internal/alerting ./internal/observation` and scoped `vet` | Passed 1.227s / 1.031s; raw freshness, coverage, resets, replicas/events, pending continuity and fresh claims                                                                                                                                                                                                   |
| `go -C apps/api test -count=1 ./internal/notification ./internal/config` and scoped `vet`  | Passed 5.308s / 0.706s; DNS/IP/redirect/TLS/size/timeout policy and closed Matrix settings                                                                                                                                                                                                                      |
| Real PostgreSQL focused alert/collection suite                                             | Passed 24.625s: eight-ledger upgrade, configuration fences, seed/ack/restart/delete, ordered replay, middle-prune reset, edit/delete lease invalidation, audit rollback and concurrent acknowledgement                                                                                                          |
| Real PostgreSQL cursor follow-up                                                           | Passed 5.613s: a nonzero cursor must be a committed row in the same source; foreign-source and rolled-back sequence gaps reset without hiding a later valid change                                                                                                                                              |
| Real PostgreSQL delivery suite                                                             | Seven focused tests passed 11.551s: upgrade from 8 and 9, legacy synthetic cancellation, adapter/reference isolation, stale ownership, lease expiry/recovery, retry exhaustion and Matrix test intent/result audits                                                                                             |
| Persisted disposable HTTPS receiver                                                        | Passed actual TLS server with injected checked-address/dial seam: webhook signature/idempotency and canonical retry bytes; Matrix PUT transaction ID and plain `m.notice` retry after repository reopen. No public endpoint or complete homeserver claim                                                        |
| `npm.cmd run check:contracts`                                                              | Final gate passed 192 bounded schemas, contract validation and generated parity                                                                                                                                                                                                                                 |
| Web/interface focused run                                                                  | 208 tests passed before final response-race regressions; final gate reruns the final tree                                                                                                                                                                                                                       |
| Native API/storage drill B                                                                 | Passed seed -> firing -> acknowledgement -> SSE change -> process restart -> replay/ack preservation -> native backup/list/empty-target restore -> old session401/new login -> ownership-loss failure stop. Initial helper A had a mistyped fixture filter; corrected on fresh B, no application failure hidden |
| Restored native invariants                                                                 | Ledger10, users1, resources6, metrics54, demo seed1, acknowledged alert1, outbound deliveries0, disabled source version2, sessions0/preauth0 and restore generation1                                                                                                                                            |
| Helm 3.17.3 lint and configured render                                                     | Passed actual local strict lint/render with setup, Kubernetes read profile, ingress, webhook and Matrix. Matrix token references the existing Secret; no cluster install claim                                                                                                                                  |
| Final `npm.cmd run verify` with real PostgreSQL                                            | Passed exit0 on frozen runtime tree: PostgreSQL package68.899s, all Go packages,36 client tests,211 web tests,394 locale messages,192 bounded schemas, format/lint/types and both builds. Log `.tmp/stage3-verify.log`                                                                                          |
| Final native drill after last runtime correction                                           | Fresh C passed on final compiled API/storage: `STAGE3_SEED_ACK_SSE_REPLAY_RESTART_NATIVE_RESTORE_OWNERSHIP_PASS`; source `stackplore_stage3_native_c`, restore `stackplore_stage3_restore_c`, private key directory `.tools/stage3-native-key-c`, ignored archive `.tools/stage3-review/native-backup-c.dump`   |
| Remote CI/race/images/Compose/cluster/browser/live operator receiver                       | Initially local under then-current instructions; on 2026-10-17 exact unchanged main/tag CI passed code/race and image/chart builds. Compose start, cluster, browser and live operator receiver were not part of those runs                                                                                      |

Native helpers and private data remain under ignored `.tools/stage3-review` and
repo-local test directories. Local proof logs stay in ignored `.tmp`; no credential,
cookie, full destination, private key or notification response body is copied
into committed evidence. Windows has no local C race toolchain or Docker/Compose
runtime; no new race/image/startup claim is made.

After all proof finished, the separate repo-local PostgreSQL process shut down
cleanly; the system service was untouched. Version metadata, documents and
reviewed release paths were rechecked after the final evidence update. The
checkpoint was committed and tagged locally as `v0.3.0`. The same immutable
implementation/tag was pushed on 2026-10-17 under renewed authorization, with
the exact successful CI results linked at the top of this plan.

### Disposable query plans

`go -C apps/api test -count=1 ./internal/postgres -run TestAlertHistoryHotPathQueryPlansOnRealPostgreSQL -v`
captured `EXPLAIN (ANALYZE, BUFFERS)` in the scoped test (13.928s). The corpus
has one source, 10,002 changes, 10,001 collection outcomes, 10,001 replica facts
and 10,003 metrics. Warm SQL-only rows/execution milliseconds:

| Query                  | Rows  | ms    |
| ---------------------- | ----- | ----- |
| Ordered replay         | 100   | 0.122 |
| Collection window      | 1,361 | 1.553 |
| Current collection     | 1     | 0.040 |
| Replica window         | 1,362 | 1.597 |
| Metric window          | 1,362 | 2.615 |
| Replay prune quota     | 33    | 0.295 |
| Collection prune quota | 33    | 0.452 |

This one-source corpus used primary-key/retention indexes; it did not demonstrate
selection of the added range indexes. These numbers are not end-to-end latency,
target scale, multi-source planner evidence or a service-level objective. The
source, caps, predicates and aggregate deadlines are part of the reviewed query
boundary. Broader measured capacity remains later-stage work.

## Stage 4 — 0.4.0 verified exploration checkpoint — 2026-10-17

The frozen runtime passed the full local gate, real PostgreSQL regressions, a
fresh native restart/restore drill and actual browser journeys. This completes
the scoped development stage; it does not complete 1.0 or target acceptance.

### Scope and correction loop

- Inventory filters run against the complete PostgreSQL set before keyset
  pagination. Provider/environment/scope/health/availability, sorting and source
  selection have closed query validation and cursor bindings. Counts aggregate
  the active snapshot rather than the first 200 resources/100 health targets.
- Source-wide topology/events no longer silently filter to an inventory prefix.
  Exact-resource requests still validate identity. Relationship transport carries
  bounded evidence; graph/list views preserve directions and isolated nodes.
- Logs use exact bounded ranges and resource/container/previous selection.
  Loaded-line search/severity and export remain bounded to loaded redacted data.
  Empty source reads preserve availability without calling target-only adapters.
- Product transport and pages are split by feature. Lists cap at 1,000 loaded
  items; the graph caps at 200 nodes with 50-node/50-edge list pages. Late replies,
  repeated cursors, cancellation and cap truncation have regressions.
- Pointer/key activity requests a server session context at most once per 60
  seconds, with a 10-second deadline, single-flight and session-generation fences.
  There is no invented expiry or idle background extension.
- Metric series use actual sample timestamps and keep separate resource curves.
  Tables expose identity/time/value; charts do not invent synchronized fleet sums.

| Pass                                | Scoped score | Findings and correction                                                                                                                                                                                                                                                                                                           |
| ----------------------------------- | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Initial feature review              | 8/10         | First-page source counts and topology/event scoping; live default range cursor drift; partial/denied health; empty target reads; incomplete pagination and stale session deadlines. Fixed in domain, PostgreSQL, HTTP and web boundaries with regressions.                                                                        |
| Independent and real-browser review | 8/10         | Missing partial-summary reason, source capability stale filter order, fractional rule threshold validation, irregular metric spacing and resource aggregation, last-page cap truncation, fixed demonstration-clock boundary, slow tail read starvation, query patch races and mobile toolbar layout. Fixed and directly verified. |

These are scoped architecture/implementation scores. Product completeness and
target evidence remain separate; stages 5–9 retain their exit requirements.

### Executed checks

| Check                                               | Result                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| --------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Real PostgreSQL inventory regression                | Three tests passed: 223-resource aggregate, 200+20 filtered continuation, all new filter cursor bindings, denied health, disabled-source stale and capability stale/denied classification. Latest focused run 9.073s.                                                                                                                                                                                                                                    |
| All Go packages with `STACKPLORE_TEST_POSTGRES_URL` | Passed after test-process recovery; PostgreSQL package 74.422s, HTTP 4.755s. Log `.tmp/stage4-api-all-recovered.log`.                                                                                                                                                                                                                                                                                                                                    |
| HTTP exploration query regressions                  | Eleven tests with subcases passed: closed filters, exact microsecond UTC ranges and bounds, previous/container targets, evidence mapping, all-source versus exact-resource pages, and 15 empty-source/status combinations.                                                                                                                                                                                                                               |
| Runtime client/form checks                          | Focused API/rule, paging, chart, session and routing tests, strict types and lint passed; the final full gate reran the frozen runtime.                                                                                                                                                                                                                                                                                                                  |
| Development proxy                                   | Real Vite/upstream probe passed Host and Origin preservation for API, health, ready and metrics paths. Marker `DEVELOPMENT_PROXY_HOST_ORIGIN_FOUR_ROUTES_PASS`.                                                                                                                                                                                                                                                                                          |
| Browser                                             | Actual disposable PostgreSQL/API/Vite: 120 data-loaded product states, 40 authentication form states and 20 final topology rechecks passed, with real login/logout, inventory filter/reload/detail/log links, redacted log download, events, grouping/evidence/keyboard and persisted alert acknowledgement. [Retained receipts](evidence/0.4.0/README.md) state semantic accessibility and clipboard limits.                                            |
| Documents/version                                   | Final document inventory/links, formatting, version and diff checks passed after the evidence update; all metadata matches 0.4.0.                                                                                                                                                                                                                                                                                                                        |
| Full `npm.cmd run verify`                           | Passed exit 0 on the frozen runtime: all Go packages with the real disposable PostgreSQL URL (cached after the uncached 74.422s package proof), 36 client tests, 284 web tests in 26 files, 509 locale messages, 192 bounded schemas, format/lint/types and both builds (web 10.64s). Log `.tmp/stage4-final-verify.log`.                                                                                                                                |
| Exact 0.4.0 commit/tag/push/CI                      | Implementation `4d63cb3b8e8ce853985d1d3e983ea3f77b94a9ef`, annotated `v0.4.0`, main and tag pushed without rewriting history. Both exact CI runs passed: [39011892562](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39011892562), [39011892582](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39011892582). Code/full real PostgreSQL/race and image-build/chart jobs passed; these runs do not start Compose or install a cluster. |

An initial full web test run reported 267 passing and one cold lazy-route timing
failure; the route regression was corrected without weakening assertions.
The first new PostgreSQL probe used the wrong test variable and skipped; it is
not SQL proof. Corrected real-URL runs above executed the tests. Denied-health
fixture setup initially violated the database's enforced unknown-state invariant;
the fixture was corrected, and the actual regression then passed.

A tool/session interruption stopped the disposable processes during a full Go
run. Its incomplete result was discarded. The separate repo-local cluster
performed normal WAL recovery, reached readiness, and the complete Go suite was
rerun successfully. The system PostgreSQL service was not changed. Local browser
fixtures, private runtime keys and raw temporary logs remain ignored; committed
evidence must contain no authentication material.

### Final correction and native proof

Final real-image review lowered the graph score from 9 to 8: a long edge crossed
an unrelated card and implied a false link. Routing now uses bounded gutters and
card-boundary ports; a halo separates crossings and geometry is memoized across
pan/zoom. Thirteen focused tests passed, including 1,000 distinct paths among
200 cards with every segment outside every card interior and inside world
bounds. All 20 locale/theme/width topology states were then repeated with actual
loaded data. Scoped architecture and implementation scores returned to 9/10.

Fresh compiled 0.4 API/storage native drill D passed:
`STAGE4_SEED_ACK_SSE_REPLAY_RESTART_NATIVE_RESTORE_OWNERSHIP_PASS`

The source `stackplore_stage4_native_d` and restored
`stackplore_stage4_restore_d` are disposable databases. Seed firing, one-use
source version changes, durable SSE snapshot/change/replay, acknowledgement,
API restart and preserved state passed. Native backup/list/empty-target restore
verified ledger 10, one user, six resources, 54 metric facts, one seed and one
acknowledged alert, zero outbound deliveries, disabled source version 2, zero
sessions/preauth and restore generation 1. The old session returned 401 and a
new login worked. Ownership loss terminated only the fixture process within the
bounded failure window. Private keys, dump and helper logs remain ignored.

The actual inventory query-plan drill also passed (5.735s package, 2.19s test)
with 2,003 demonstration resources, 100 native resources, 21,003 metric facts,
10,003 events and 10,000 audit records. Warm SQL-only observations were about
0.19ms for 51 name-sorted resources, 17.80ms for the full aggregate, 5.98ms for
101 health-filtered resources, 0.05ms for 51 events, 0.41ms for 100 current
metrics, 1.42ms for a 250-row retention quota and 0.10ms for 51 audit rows.
These are fixture SQL plans, not end-to-end latency, target scale or an SLO.

## Stage 5 — 0.5.0 locally verified checkpoint — 2026-10-17

Implementation and scoped evidence exist; **final changed-tree gate D passed;
publication/CI are pending**. No Stage 5 commit, tag, push or remote CI result is
claimed here. Stage 6–9 work remains open.

### Implemented scope

- Official Docker Engine and Kubernetes clients remain behind provider-neutral
  ports. Docker observation has a reviewed mTLS proxy profile; direct socket
  access remains loopback development only. Kubernetes uses explicit namespaces
  and optional separate cluster observation, with no Secret value/object reads.
- Disjoint kind/scope coverage publishes authority atomically. Complete empty
  partitions prove deletion; failed/denied/omitted partitions do not. Current
  reads and historical evidence hide revoked authority independently of bounded
  retirement. Operator policy changes fence cache until fresh publication.
- Closed Docker/Kubernetes metadata exposes only reviewed state, limits,
  conditions, ports, container mount targets and public labels. It excludes host
  mount paths, raw SDK maps, environment values and credentials. Domain, SQL,
  HTTP contract, strict browser guards and bilingual detail views share the bound.
- Pod log reads check expected immutable UID before and after the name-based
  read. Usage checks stable UID, known creation time and the full raw sample
  window before transport rounding. Known usage denial fences the whole metric
  port; missing usage with configured facts is stale. Separately observed
  configuration remains visible in typed detail metadata.
- `demo-v2` preserves the source and earlier resource identities while adding
  nine resources across two environments, 13 evidence-backed relationships,
  stable absolute-tick history and three deterministic test steps. The initial
  bounded metric history contains 972 facts. Synthetic collection keeps the
  fixed observation clock; operational receipts/backoff use wall time.
- Ordinary fixture upgrades replace packaged derived facts while preserving
  administrator rules, disabled-source choices, accounts and audit. Isolated
  startup/reset uses separate marked storage and resumable cleanup. Explicit
  guest viewing creates no account/session/grant, denies writes and private
  routes, and cannot enable real providers or outbound delivery.

See [Docker](docker-provider.md), [Kubernetes](kubernetes-provider.md),
[Demo](demo.md) and [ADR-0013](adr/0013-provider-coverage-and-demo-lifecycle.md).

### Correction loop and remaining gate

| Pass                      | Stage score | Finding                                                                                                                                           | Correction and state                                                                                                                                                                                                                      |
| ------------------------- | ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Provider/guest review     | 8/10        | Incomplete partition authority, derived reference leakage, fixture reset erasing administrator choices, and unbound Pod log/usage identity        | Narrow domain/SQL/adapter/HTTP/browser fixes plus real SQL and provider regressions passed.                                                                                                                                               |
| Final independent review  | 8.5/10      | Environment names retained omitted legacy authority; the complete gate exposed optional metric-port clock and retained-current-sample regressions | Environment leakage reproduced against real PostgreSQL, then corrected in both environment paging and initial-port inference. Environment and final clock fixes passed real SQL regressions; the final changed-tree gate remains pending. |
| Startup-profile follow-up | 8.5/10      | Source-settings startup allowlist omitted the valid restricted Docker profile                                                                     | A failing regression reproduced the rejection; the minimal closed-allowlist fix passed source-settings/server packages in 1.581s/6.208s. The final gate must include this later fix.                                                      |
| Final acceptance          | 9/10        | No known severe defect in the delivered Stage 5 scope after the final corrections                                                                 | Final gate D and independent index/scope review passed; exact publication/CI remain pending. This is not a 1.0 or deployment-completeness score.                                                                                          |

The environment regression first returned legacy environment A and current B
even though A's resources were already hidden. The passing replacement checks
legacy fallback, current/omitted/denied/unsupported partitions, authorized empty
environments, one-row keyset pages/counts and the pending operator-policy fence.
The before/after receipts are retained separately.

### Executed scoped proof

All PostgreSQL commands below used the explicit disposable
`STACKPLORE_TEST_POSTGRES_URL`; a run without that variable is not SQL proof.
Ignored local logs are diagnostic receipts, while the retained guest artifacts
contain only safe fixture data. These results do not substitute for the final
changed-tree gate.

| Boundary                                 | Executed result and receipt                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Docker Engine                            | `TestRestrictedProxyOnRealDockerEngine` passed in 128.01s against disposable Linux Engine 29.1.3, negotiated API 1.52, Go 1.26.5. Inventory/stats/logs/events, proxy method/path denials, mTLS and ownership cleanup are covered. Marker `REAL_DOCKER_RESTRICTED_MTLS_SDK_INVENTORY_STATS_LOGS_EVENTS_AND_DENIALS_PASS`; `.tmp/stage5-real-docker-wsl-d.log`.                                                                                                                                                                                                                                                |
| Kubernetes                               | Final same Linux test binary passed missing Metrics API in 7.06s and positive Metrics API in 18.84s on disposable Kubernetes 1.37.0, kind 0.33.0, Engine 29.1.3. Scoped inventory, denial, current/previous logs, usage and configured facts crossed the adapter boundary; `.tmp/stage5-kube-final-real.log`. Hash and exact profiles are in the provider guide. The owned cluster and kubeconfig were removed, with zero owned kind containers remaining.                                                                                                                                                   |
| Pod identity/metric authority            | Focused UID/lifetime, fractional raw window, independent item budgets, mixed 403 and stale/known-configuration tests passed in 4.847s; `.tmp/stage5-metric-identity-final.log`. Final Kubernetes/domain package tests passed in 5.136s/0.888s; `.tmp/stage5-kube-final-uid-tests.log`. Scoped vet receipt is empty on success.                                                                                                                                                                                                                                                                               |
| PostgreSQL authority and normal upgrades | Six real SQL regressions passed in 25.201s, including partial partitions, policy outage fences, profile rotation preserving disabled sources, partial replica evidence and revoked alert history; `.tmp/stage5-review-final-pg4.log`.                                                                                                                                                                                                                                                                                                                                                                        |
| Synthetic and optional-port clocks       | Domain/observation packages passed in 2.050s/2.013s; `.tmp/stage5-demo-clock-unit2.log`. Full synthetic capture/save/in-progress/failure plus ordinary fixture/profile preservation passed real SQL in 14.206s; `.tmp/stage5-demo-clock-pg.log`. Three final optional-port clock/retention/future-sample SQL regressions then passed in 6.295s; `.tmp/stage5-final-metric-clock-fix.log`. The metric capability clock bounds sample selection, including ports observed later than inventory.                                                                                                                |
| Restricted profile startup               | Negative regression reproduced source-settings rejecting the restricted Docker profile. The minimal reviewed allowlist correction passed source-settings/server tests in 1.581s/6.208s; `.tmp/stage5-source-profile-regression-green.log`.                                                                                                                                                                                                                                                                                                                                                                   |
| Environment-name authority               | New privacy/pagination test failed before the fix in `.tmp/stage5-environment-authority-before.log`. The new test and four adjacent real PostgreSQL regressions passed in 8.537s after the fix; `.tmp/stage5-environment-authority-after.log`.                                                                                                                                                                                                                                                                                                                                                               |
| Guest browser                            | [Retained matrix](evidence/0.5.0/browser-matrix.json): six loaded routes across English/Persian and 360px light/1280px dark profiles, 24 states with document direction/language and horizontal-overflow checks. [Runtime receipt](evidence/0.5.0/guest-runtime.json): restart advanced reset epoch 1→2, preserved nine resources/972 metrics, and kept users/sessions/grants/deliveries at zero; protected audit count increased. Named private/write requests returned 403, with no authentication cookie. This is guest exploration proof, not a complete browser or assistive-technology release matrix. |
| Native PostgreSQL                        | Fresh API/storage drill D recorded backup and restore completion and marker `STAGE5_SEED_ACK_SSE_REPLAY_RESTART_NATIVE_RESTORE_OWNERSHIP_PASS`; `.tmp/stage5-native-drill-d.log`. It is a disposable checkpoint receipt; the final changed-tree gate remains separate.                                                                                                                                                                                                                                                                                                                                       |
| Frontend guards and interface            | Guest/auth focused suites passed 44 tests; metadata/detail suites passed 67 tests; final metadata panel passed seven tests. Receipts `.tmp/stage5-guest-focused-final.log`, `.tmp/stage5-metadata-tests2.log`, `.tmp/stage5-ui-lasttest.log`. They are overlapping focused runs and must not be summed as distinct coverage.                                                                                                                                                                                                                                                                                 |
| Preliminary static gates                 | Verify B passed formatting, 399 local documentation links, aligned 0.5.0 metadata, notices for 72 linked modules/106 exact upstream files, 582 locale messages, 204 bounded schemas/derived drift, lint/vet and TypeScript before its PostgreSQL failures; `.tmp/stage5-final-verify-b.log`. Documentation and final code changes require another complete gate. Helm lint/render and long-release role-name isolation passed separately; `.tmp/stage5-kube-helm-final.log`.                                                                                                                                 |

### Failures kept separate

The early full gate A failed the synthetic capture expectation after the clock
correction; its receipt remains `.tmp/stage5-full-verify-a.log`. Full gate B
failed `TestObservationCapturePreservesPayloadAndPortClocks` and
`TestObservationRetentionFairQuotaKeepsNewestMetricAndCurrentHealth`; its receipt
is `.tmp/stage5-final-verify-b.log`. Neither is a passing milestone gate, and B
did not reach client/web tests or builds through that command chain.

The first metrics-server image pull failed with registry 403 inside the owned
kind node. Final provider proof kept the exact pinned official digest and used
descriptor-hash-checked host import into that disposable node. Failed pull/import
receipts remain distinct from the successful final run. No shared daemon or
operator service configuration was changed for that workaround.

Full gate C passed, including the fresh real PostgreSQL package run in 140.316s,
but compiled before the later startup-profile fix. Final gate D reran the
complete `npm.cmd run verify` after that fix and exited 0: server 5.960s,
source-settings 0.754s, real PostgreSQL cached from C, 36 client tests, 350 web
tests and both builds. Receipts `.tmp/stage5-final-verify-c.log` and
`.tmp/stage5-final-verify-d.log` remain separate from failed A/B. The reviewed
scope now scores 9/10; exact commit/tag publication/CI remain pending. Broader Stackplore
Compose/Helm installation, provider-version compatibility, guarded actions,
protected audit lifecycle, performance/accessibility/PWA and final release
evidence remain in Stages 6–9.

### Rejected upstream-only 1.0.1 local checkpoint — 2026-10-18

Both full local gates passed on real owned PostgreSQL17.11. Final B includes
24 release/policy regressions,40 client and453 web tests; format, lint, types,
715 locale messages,235 bounded schemas, legal notices and both builds passed.
All14 actual production-browser cases passed, with16 new public screenshots.
Helm lint/render passed. Database image pull/remote scan timed out locally
before analysis; these failures remain separate. At that point exact hosted
main/tag CI, database runtime/scans and signed recovery were pending. The later
strict hosted scan failure and derivative correction are recorded in the current
section above; this passing local checkpoint does not establish that replacement.
[Local receipt](evidence/1.0.1/local-gates.json), [browser receipt](evidence/1.0.1/browser-runtime.json).

### Final derivative-image local checkpoint — 2026-10-18

Full local C passed after all helper, image-policy, release-identity and owned
fixture code changes: 27 release/policy tests,40 client tests,453 web tests,
format, lint, types, locale/contracts, notices and both builds. A fresh actual
production browser B also passed all14 cases and replaced16 public screenshots.
The exact complete database image still requires hosted build/probe/scan proof.
The independently rebuilt helper passed real Linux execution and one-executable
analysis with the fresh2026-10-18 vulnerability database;
[receipt](evidence/1.0.1/helper-rebuild.json). Empty Windows executable analysis
was rejected. Build contexts now exclude unrelated artifacts and env files.

The derivative-source trial39121495195 on f046d523 built the complete image,
passed actual PostgreSQL17.11/gosu1.19/Go1.26.9 probes and analyzed45 Alpine
and4 Go packages with zero HIGH/CRITICAL findings. Its unsigned retained
reports exposed a validator error: the pinned-source main module has a valid
versionless package URL. The narrow correction accepts only that exact main
module form, preserves versioned dependencies and full graph/set identity, and
passes all three untouched reports. The trial remains failed; signed packages
and native recovery were not reached. [Actual checkpoint](evidence/1.0.1/database-runtime-checkpoint.json).

Final full local D passed on2026-10-18 after the narrow source-main package URL
correction: 28 release/policy tests,40 client tests,453 web tests and all
format/lint/type/contract/locale/notice/build gates. Backend core coverage is
80.6% domain and82.6% alerting. This local run measured frontend statements
81.61%, branches77.8%, functions85.48% and lines83.11%; historical runs retain
their own coverage values. No vulnerability threshold or dependency-version
requirement was relaxed.
