# Corrective 1.0.2 evidence

Status: **released and accepted; all nine stages complete; review 9/10**.
Independent final backend/interface/release-helper and public/archive checks
support that score within the recorded scope. The fresh audit starts from
documentation delivery `657a600`, following immutable `v1.0.1` at
`73ece4754554cca548c242768b195d21664d37b8`. Quality reopened at **8/10**.
Candidate source `18e8558e76d5c2fec4eb1d752bceaa43aaa1f260` was committed and
pushed, then superseded by the retained-event response correction below.
Its main CI `39131357982` and trial `39131368817` remain bound to that earlier
source; its main six-job CI and trial package passed but cannot certify the change.
Current immutable `v1.0.2` points to
`376ed0379900ab870dd8eb8af6b55ef3e5ec389b`, schema **17**. Full local D, all six
exact-source main/tag checks, strict signed trial and independent trial recovery
pass. The first tag browser and blocked release attempts remain failed; the
unchanged-tag code retry passed. Stable [release 1.0.2](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v1.0.2),
ID `416503671`, was published at `2026-10-18T14:43:05Z` and independently
verified. The accepted product scope is unchanged. [Published receipt](published-release.json).

The [fresh audit](fresh-audit.md) records verified triggers, scoped fixes,
negative-to-positive checks and proof limits. Current schema ledger is **17**;
earlier published migration bytes and all historical receipts remain unchanged.
The [local-gate receipt](local-gate.json) records the actual precommit run;
exact immutable source, browser and release receipts remain separate gates.

| Boundary                        | Current evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Provider text and HTTP methods  | URL-userinfo leakage and missing Allow headers reproduced. Four real handler regressions pass. Independent review iteratively exposed apostrophe, clipping, JSON member names, embedded schemes and multilayer slash escapes. Final 27 escaped-slash cases failed 21 leak cases with six safe controls before correction; the complete masking package, independent three-case overlay and replacement full gate C pass.                                                  |
| Retained event responses        | Older persisted native-event summaries bypassed ingestion masking in REST and SSE. Shared output mapping now applies the deployment redaction policy and an 8,192-rune limit to REST, initial SSE and subsequent polls. Ten leaking cases and a poll regression failed before correction while four safe controls passed; all 14 cases and the real poll now pass. Existing rows/backups are not rewritten.                                                               |
| Collection authority            | A successful first event page followed by permission denial previously retained unauthorized partial evidence; the regression now rejects/discards it.                                                                                                                                                                                                                                                                                                                    |
| PostgreSQL ordering and upgrade | Real non-byte collation reproduced notification/rule keyset failure. Canonical byte ordering and matching migration-17 indexes pass real PostgreSQL, including forward upgrades from ledgers 14 and 16.                                                                                                                                                                                                                                                                   |
| Native database selection       | Double-decoding and libpq connection-option interpretation reproduced. Actual native backup/empty-target restore passes for owned names containing percent, equals, quote and backslash.                                                                                                                                                                                                                                                                                  |
| Interface                       | Severity URL/history and English/Persian labels, locked deleted-rule drafts, topology facts and the 50-row account render window have 65 unique focused passing tests across ten product files. Final rules/users run passes 29 tests in three files; scoped lint/type checks pass. Navigation palette: five additional focused tests pass.                                                                                                                               |
| Release-note policy             | Immutable-source document links have 29 passing release/policy checks. This does not establish a signed release.                                                                                                                                                                                                                                                                                                                                                          |
| Full local                      | [Current replacement D](local-gate.json) passes exit 0 after the retained-event response amendment: all Go/native/core, client 40, release/policy 29 and web 477/49 files plus checks and final web/API builds. Earlier B/C keep their actual source scope.                                                                                                                                                                                                               |
| Browser and chart               | [Local UI receipt](browser-gate.json) retains its earlier snapshot: 14 cases, 64 measured routes, eight palette profiles and 24 screenshots; four were visually reviewed. Exact main/tag CI also passes all 14 production-browser cases on `376ed03`. Strict local Helm lint passed one chart; actual installation/TLS/provider checks are separate hosted jobs.                                                                                                          |
| Exact-source CI                 | [Main `39132989641`](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39132989641) passed all six jobs. [Tag `39134154907`](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39134154907) is green after one unchanged-source failed-code-job retry; the other five original jobs stayed green. Real PostgreSQL/race/browser, images, Compose, external-TLS Helm and both provider jobs passed.                                                             |
| Signed trial and consumer       | [Trial `39133022021`](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39133022021) package passed. Independent [trial consumer](consumer-trial.json) verified every payload checksum, all 31 provenance subjects and seven SBOM attestations against exact main source and hosted identity, rejected a wrong source, and ran the actual archives.                                                                                                                 |
| Public publication and consumer | [Release `39134154891`](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39134154891) first refused packaging after failed tag CI. Its retry succeeded: package `117702264862`, publication `117703943506`. [Independent public proof](published-release.json) verifies all 39 files, 30 manifest entries, all 31 provenance subjects and seven SBOM attestations against exact tag/source/hosted identity, with wrong-source rejection and actual archive checks. |

Failed attempts retain their outcomes: initial owned-cluster startup after the
host restart, private negative probes, blocked build attempts, one interface
worker startup timeout and the first topology fixture mismatch. An intermediate
full API attempt failed while the tree was changing, with a noncanonical local
temporary-directory spelling and a native fixture cleanup deadline. Independent
reproduction tied cleanup to a slow paced checkpoint and a closed reused admin
connection. A test-only bounded cleanup correction passed separately; it does
not change the earlier failure into a full passing gate. Passing focused reruns do not relabel
those failures or prove a complete milestone. The first full verify's 18 lint
findings remain failed in `.tmp/reaudit102-full-verify.log`. Complete replacement
B passes exit 0 in `.tmp/reaudit102-full-verify-b.log` and its exit marker.
The normal thread pool/coverage completed without a startup failure. Go checks
retain accepted cache reuse; the separately recorded focused SQL/native runs
were uncached. Normal module checksum verification and the native-tool fixture
flag remain enabled.

Current D web coverage: 82.91% statements, 78.68% branches, 86.71% functions and
84.48% lines. Backend core: domain 80.6%, alerting 82.6%. The production web build
checks 63 exact legal packages/files. The standard-pool 477-test run completed
in 206.67s. These are current precommit local results, not
hosted or signed-release proof.

Current D transcript SHA-256 is
`8595a3ded232a88fb920555c0484da10f9bbc85a867f2e8c478c342fae4cdb31`.
The receipt retains B/C's earlier hashes/coverage separately. Exact hosted proof
now covers immutable source `376ed03`; local D remains its precommit proof.
The later main-only browser-harness/report-retention change is separate from
the frozen release. Public acceptance binds the actual tagged assets below.

## Accepted public assets

All 39 assets were independently fetched with their API sizes/digests and
verified signed manifest, all 30 payload checksums, 31 provenance subjects and
seven SBOM attestations. Wrong all-zero source identity was explicitly rejected.
All eight strict vulnerability reports have zero HIGH/CRITICAL findings; the
secret scan has zero findings. Public manifest SHA-256 is
`c3409cc8300212fcbf27e83a1524ba0c64291e9f0503b8d9c7ebaf9789e45bf1`.

Public Windows SHA-256
`d7c1e7807b5b71946c2df274da07945e4c284421ae425f9b29febe2d36ddde1b`
matches the actual 17-migration recovery-tested trial. The public database archive
differs from trial: SHA-256
`9c5e686b1350c4ac21c68ad9234199efb04b5c9f95bf695ed8311060844ac79f`.
It was independently loaded/executed on Linux amd64, verifying configuration/
rootfs/execution identity, PostgreSQL 17.11 (`170011`), gosu 1.19/Go 1.26.9,
UID/GID 70 root-drop, restricted read-only/network-none/non-root SQL and owned
cleanup. Its configuration digest is
`sha256:ef0b232c67945f66c51c2c9291c81b2130e29b96fe9f88d160507ee03975291f`.
Signed Linux amd64 native guest/normal smoke passed. [Exact public receipt](published-release.json).

## Independent trial archive proof

Artifact `12138922020` ZIP SHA-256 is
`92bde1ce3544271c82e3bf26a204aebae89da0658cc2c078feb43b8f8bfb7efb`;
manifest SHA-256 is
`a4358fbba48fafe6232acc6a0e4e130a5a5fe7876b3971de168f738380b15e63`.
The consumer checked 30 payload checksums, all 31 provenance subjects and seven
SBOM attestations using exact main source/ref and hosted-runner policy; a
wrong all-zero source was rejected. All eight strict scan reports have zero
HIGH/CRITICAL findings, and the secret report has zero findings. Source SPDX
contains 470 packages, 393 npm package URLs and 74 Go package URLs; the 63-package
production web legal inventory is a separate check.

Actual Windows amd64 trial archive SHA-256 is
`d7c1e7807b5b71946c2df274da07945e4c284421ae425f9b29febe2d36ddde1b`.
Its current nested `windows_runtime.proof` records 17 migrations, source state,
acknowledgement/SSE/replay, restart, native backup/empty restore, generation 1,
protected history, revoked sessions/preauth, fresh login and ownership-loss
shutdown with owned cleanup. The trial database archive SHA-256 is
`c73d5d409fd344f8058dcc673b2ecde1a08456b60eff28eeb7a1e7baae88c082`.
Independent Linux amd64 loading/execution verified configuration/rootfs identity,
PostgreSQL 17.11 (`170011`), gosu 1.19/Go 1.26.9, root-drop 70:70, read-only,
network-none non-root SQL and exact-owner cleanup. Signed Linux amd64 native
guest/normal smoke passed. The public section above records Windows byte identity
and separate actual database execution; it does not assume database equality.

## Retained tag failure and maintainer harness follow-up

The first tag code/browser attempt failed before the worker-update acceptance
assertions: a fetched control body did not match the expected worker declaration.
Its original HTTP status was not recorded and cannot be recovered. Eleven guest
and two normal cases passed, but the tag gate failed and release publication
stayed closed. A single unchanged-tag code retry then passed all 14 browser cases.

The [separate signed-trial diagnostic](browser-fixture-diagnostic.json) used four
cold contexts at unchanged default 20 requests/s and burst 40. One rapid reload
logo request received 429; another context's extra `/sw.js` control GET received
429 and recovered to a valid single-marker worker after refill. No cold-navigation
or API failures were observed in those four cases. The first private diagnostic
failed on its shutdown classifier; the successful confirmation verified server
exit, exact-owned database absence and unbound ports. Neither result changes the
first hosted failure into a pass or proves customer capacity.

A later main-only harness follow-up checks worker status/MIME, a 64-KiB body
assertion and exactly one marker, with at most four attempts/15 seconds and
bounded `Retry-After` handling only for 429. It retains every worker update/cache
assertion and emits safe response metadata. Hidden result reports are included
only through the existing explicit sanitized upload whitelist. This follow-up
does not change the frozen release source or product rate limits. Fourteen
in-memory branch cases pass. Separate live helper-coverage attempts C/D remain
exit 1: each got a valid first 200 and did not exercise the intended 429 retry
branch; owned cleanup passed. Those failed coverage guards are not live retry
proof. Six-job CI on the commit containing this maintainer follow-up must pass
before handoff; commit checks and the release body bind the final run/source.

Maintainer commit `7545339d82f0ab54fd483b77048dd79ab3cc3fa3` then failed
main CI `39138645362` during test 11's unmeasured `/events` setup, before the
80-read workload. Thirteen of 14 browser cases passed, including the full worker
update case with one valid 200 control response. The retained hidden reports
show a setup-component API 429; the document status was not captured and is not
inferred. Other five jobs and the code job's earlier full/race checks passed;
the browser failure remains failed.

The subsequent main-only setup fix adds four 2.5-second refills between validated
setup routes and search, with safe fixed-route document-status diagnostics.
An AST-extracted execution of the exact revised callback against the signed
`376ed03` runtime passed: 86 setup responses were 200, then all unchanged
80 reads/8 parallel/10 rounds/minimum 600-ms spacing were 200 with valid envelopes.
The four actual refills were 2504/2507/2504/2508 ms, outside the measured 6049-ms
read window; median/p95 86/129 ms are local synthetic observations, not capacity
claims. Ten in-memory diagnostic checks also pass. Owned server/database/port
cleanup passed. [Failure and focused proof](browser-fixture-diagnostic.json).
This focused callback is not a full 14-case run or final six-job main CI; the
review-commit handoff requirement above remains.

The [active plan](../../implementation-plan.md) records commands and gate
updates; [browser measurements](browser-measurements.json) retain all eight
profile results. Historical [1.0.1 public proof](../1.0.1/published-release.json) remains
bound to its original source, 39 files, 30 checksums, 31 provenance subjects,
seven SBOM attestations and migration-16 runtime. Those counts/results are not
current 1.0.2 execution proof.

Customer ingress/TLS, provider permissions, notification receivers, credentials,
backup destination, capacity and a full air-gapped installation require their
own operator checks. Linux arm64/macOS outputs require separately named runtime
proof; cross-building and scanning alone do not establish execution. The manual
account-table load-more path accumulated DOM rows in the audit baseline. Its
corrected render window now shows at most 50 rows and keeps every loaded account
accessible. The manually requested cache can still grow until refresh/unmount;
no fixed total-account cap or memory bound is claimed. A final peer review also
found an alert-rule Cancel/deletion race. Its deferred-response regression
failed before correction; saving/loading now disables Cancel, and the final
29-test rules/users run passed. All nine stages and current release acceptance
are complete at 9/10; customer deployment and the stated platform limits remain
separate operational boundaries.
