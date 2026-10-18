# Corrective 1.0.2 evidence

Status: **candidate; acceptance pending**. Independent final backend/interface/
release-helper review is **9/10** with no open verified defect in that scope;
whole-delivery acceptance remains **8/10** pending exact-source hosted and
signed/public gates. The fresh audit starts from
documentation delivery `657a600`, following immutable `v1.0.1` at
`73ece4754554cca548c242768b195d21664d37b8`. Quality reopened at **8/10**.
There is no recorded 1.0.2 source commit, tag, signed artifact or public release
yet. The accepted 1.0 product scope remains unchanged.

The [fresh audit](fresh-audit.md) records verified triggers, scoped fixes,
negative-to-positive checks and proof limits. Current schema ledger is **17**;
earlier published migration bytes and all historical receipts remain unchanged.
The [local-gate receipt](local-gate.json) records the actual precommit run;
exact immutable source, browser and release receipts remain separate gates.

| Boundary | Current evidence |
| --- | --- |
| Provider text and HTTP methods | URL-userinfo leakage and missing Allow headers reproduced. Four real handler regressions pass. Independent review iteratively exposed apostrophe, clipping, JSON member names, embedded schemes and multilayer slash escapes. Final 27 escaped-slash cases failed 21 leak cases with six safe controls before correction; the complete masking package, independent three-case overlay and replacement full gate C pass. |
| Collection authority | A successful first event page followed by permission denial previously retained unauthorized partial evidence; the regression now rejects/discards it. |
| PostgreSQL ordering and upgrade | Real non-byte collation reproduced notification/rule keyset failure. Canonical byte ordering and matching migration-17 indexes pass real PostgreSQL, including forward upgrades from ledgers 14 and 16. |
| Native database selection | Double-decoding and libpq connection-option interpretation reproduced. Actual native backup/empty-target restore passes for owned names containing percent, equals, quote and backslash. |
| Interface | Severity URL/history and English/Persian labels, locked deleted-rule drafts, topology facts and the 50-row account render window have 65 unique focused passing tests across ten product files. Final rules/users run passes 29 tests in three files; scoped lint/type checks pass. Navigation palette: five additional focused tests pass. |
| Release-note policy | Immutable-source document links have 29 passing release/policy checks. This does not establish a signed release. |
| Full local | [Current replacement C](local-gate.json) passes exit 0 after final escaped-URL correction: all Go packages/native fixture/core coverage, 40 client tests, 29 release/policy tests, 477 web tests/49 files, 725 locale messages, 235 schemas, lint/types/contracts/notices/version/runtime policies and final web/API builds. Earlier B remains an actual earlier-state pass. |
| Browser and chart | [UI receipt](browser-gate.json): all 14 production-browser cases pass for the earlier UI build: 12 guest and two normal-language journeys, 64 measured routes, eight palette/accessibility/focus profiles and 24 screenshots. Four named screenshots were visually inspected. The later backend text correction requires new exact-source gates. Actual strict Helm lint passes one chart; lint is not an installation/TLS-runtime receipt. |
| Exact-source CI | Pending reviewed commit and exact hosted results; no older green run is reused. |
| Signed/public consumer | Pending scans, trial/signing, exact-tag publication, every checksum/provenance/SBOM and actual archive checks. |

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

Final C web coverage: 82.91% statements, 78.68% branches, 86.71% functions and
84.48% lines. Backend core: domain 80.6%, alerting 82.6%. The production web build
checks 63 exact legal packages/files. These are current local results, not
hosted or signed-release proof.

Final C transcript SHA-256 is
`2de5e8b1bb80b71bec05e25ccbbc62ad19341b40df9c2effbf501cb9870891e1`.
The receipt retains B's earlier hash/coverage separately. Current source has no
recorded immutable commit/tag or hosted result yet; no public release is claimed.

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
candidate render window now shows at most 50 rows and keeps every loaded account
accessible. The manually requested cache can still grow until refresh/unmount;
no fixed total-account cap or memory bound is claimed. A final peer review also
found an alert-rule Cancel/deletion race. Its deferred-response regression
failed before correction; saving/loading now disables Cancel, and the final
29-test rules/users run passed. Full candidate delivery gates remain pending.
