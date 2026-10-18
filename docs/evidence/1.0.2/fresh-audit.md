# Fresh full audit — accepted 1.0.2

Review date: 2026-10-18. Baseline: documentation delivery `657a600`; previous
immutable release source `73ece4754554cca548c242768b195d21664d37b8`. The review
covers backend transport/authentication/domain/services/providers/runtime and
commands, PostgreSQL migrations/repositories, collection/retention/alert and
notification jobs, frontend/client/interface/PWA, contracts, deployment,
workflows, release scripts and current documentation. Generated and upstream
legal bytes are checked against their authorities; local caches and unrelated
`artifacts/` are outside product source.

Quality reopened at **8/10** after concrete defects and an accepted navigation
gap were found. Final independent backend/interface/release-helper review now
assigns **9/10** with no open verified defect in that scope; whole-delivery
acceptance is **9/10**, with all nine stages complete after actual publication
and independent public/archive proof.
Scores are review judgments, not measured coverage or customer certificates.
Earlier local gate C and the scoped production-interface browser passed before
the retained-event response amendment. That amendment has focused passing proof;
full replacement D now passes. Current immutable `v1.0.2` source
`376ed0379900ab870dd8eb8af6b55ef3e5ec389b` passed all six exact main/tag jobs,
the strict signed trial and independent actual trial archive recovery. Public
publication and independent public consumption also pass in the
[exact public receipt](published-release.json).
[Gate ledger](README.md).

Candidate `18e8558e76d5c2fec4eb1d752bceaa43aaa1f260` was committed/pushed,
then review found that already-persisted native-event summaries bypassed provider
masking on REST and SSE output. Its main CI `39131357982` and trial `39131368817`
remain earlier-source results; the trial package passed but cannot certify this
amendment; no tag was made on that superseded source. The shared response mapper now applies the
existing deployment policy before the 8,192-rune summary limit, including later
SSE polls. Stored rows and old archives remain unchanged.

After replacement B and the browser UI run passed, one more independent probe
found a serialized URL whose escaped slashes survive an outer JSON decode.
Two generated-marker cases failed (nested string and malformed JSON; the
ordinary one-escape JSON case passed). Private
`.tmp/read-only-escaped-url-probe-b.log` retains the 1.148s failure. The bounded
escaped-slash correction now matches bounded multi-layer slash escapes in both
userinfo and clipped-authority guards while preserving the original scheme
prefix. Twenty-seven generated-marker cases cover backslash depths 1/2/4 in
text, object strings, arrays, malformed objects, member names, clipped text and
objects, with safe controls. Before correction, 21 cases failed and six controls
passed; afterward the entire package passed in 1.250s and the independent
three-case overlay passed in 1.159s. Private
`.tmp/fresh-audit-serialized-url-before.log`,
`.tmp/fresh-audit-serialized-url-after.log` and
`.tmp/read-only-escaped-url-probe-after.log` retain those outcomes. New complete
gate C subsequently passed through final web/API builds, exit 0. Earlier
local/browser passes retain their earlier-state scope; they did not replace that
corrected-source gate.

## Verified findings and corrections

| Boundary                                | Trigger and consequence                                                                                                                                                                                                                        | Correction and regression                                                                                                                                                                                                                                                                                                                                                                                            |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Provider text masking                   | A log or Kubernetes event containing a URL with credential userinfo exposed those bytes, including JSON strings/member names.                                                                                                                  | Shared masking removes complete userinfo, including usernames used as tokens. Independent review exposed apostrophe, clipped authority, embedded scheme and JSON member-name residuals after the initial fix. Whole-object masking prevents credential-name leaks/key collisions; final package regressions pass.                                                                                                    |
| Retained event output                   | A native event stored by an older version could contain credential text. REST and initial/polled SSE mapped that stored summary without output redaction.                                                                                      | One response mapper applies the existing deployment policy and bounds the resulting summary to 8,192 runes. Generated-marker tests cover both transports, later real SSE polling, safe text, metadata, Persian limit, mask expansion and reader immutability. No row or backup rewrite is claimed.                                                                                                                   |
| HTTP 405 response                       | Unsupported methods on log download and three source-setting path forms returned an empty Allow header.                                                                                                                                        | Exact GET/GET/PATCH/POST methods are now published; all four real-handler regressions pass. Successful route methods and contract shapes are unchanged.                                                                                                                                                                                                                                                              |
| Event collection authority              | Page one succeeded and a later page denied permission. The collector converted the denial to available/partial and retained one event.                                                                                                         | A later permission-denied status stays authoritative; accumulated unauthorized native events are discarded. The regression covers the page transition; no additional unsupported-page correction is claimed.                                                                                                                                                                                                         |
| Notification/rule keysets               | An installation with non-byte text collation ordered mixed-case identifiers differently from Go cursor validation, causing repository-unavailable failures.                                                                                    | Both ordering and seek predicates use canonical byte collation. Migration 17 adds matching indexes without rewriting prior SQL; real non-byte-collation and ledger-14/16 forward-upgrade checks pass.                                                                                                                                                                                                                |
| Native PostgreSQL name selection        | URI names containing a literal percent sequence were decoded twice; names containing equals could be parsed as libpq connection options by native tools.                                                                                       | Decode the URL path once and preserve the selected database as a literal escaped connection value. Real owned-name backup/restore tests cover percent, equals, quote and backslash and verify restored state/generation.                                                                                                                                                                                             |
| Log severity and language               | Trace/unknown deep links and history lost their severity; several views showed raw severity codes instead of English/Persian text.                                                                                                             | All seven log severities survive URL state and use one locale helper in logs/alerts/notifications. API/download values remain unchanged; native events retain five supported severities.                                                                                                                                                                                                                             |
| Rule editing                            | Deleting the rule currently being edited left a stale draft that could be submitted against the deleted rule. Final peer review also found active Cancel could begin another draft while deletion was pending, then the late reply cleared it. | Successful same-rule deletion clears the draft; failure retains it. Cancel stays disabled while saving/loading, and a deferred-response regression proves the lock through completion.                                                                                                                                                                                                                               |
| Topology node facts                     | Graph nodes omitted required provider icon, scope and observed state despite available resource facts.                                                                                                                                         | Graph/list present equivalent actual facts; technical scope stays LTR, long text is bounded and full accessible details remain available. No graph geometry expansion or invented health state is used.                                                                                                                                                                                                              |
| Keyboard navigation                     | The accepted quick-navigation journey had no implementation.                                                                                                                                                                                   | A bilingual fixed-route palette supports keyboard search/selection, role-aware destinations, source preservation, Escape and focus return. It adds no arbitrary route/endpoint execution.                                                                                                                                                                                                                            |
| Account rendering scale                 | Manual load-more appended every fetched account into the DOM, creating unbounded rendered growth across requested pages. No authorization bypass was found.                                                                                    | A 50-row render window preserves previous/next access to the full loaded cache and existing final-administrator checks. Six generated multi-page/bilingual regressions plus the existing tests pass. The cache itself remains manually grown until refresh/unmount.                                                                                                                                                  |
| Release document links                  | Packaged release notes pointed into mutable branch documentation.                                                                                                                                                                              | Generated document links bind to the immutable source revision. Release-policy tests check the resulting links; a successful helper does not prove publication.                                                                                                                                                                                                                                                      |
| Browser control-read/report diagnostics | The first exact-tag browser test treated an unchecked control response as worker source and failed before update assertions. Its intended hidden JSON reports were omitted from the uploaded artifact.                                         | Separate signed-trial reproduction observes default-bucket 429 on one extra worker GET and one reload logo; original hosted status remains unknown. A main-only follow-up bounds 429-only Retry-After handling, validates status/MIME/size/one marker and includes only the existing sanitized report whitelist. Every PWA assertion and product limit remains. It is separate from frozen release source `376ed03`. |

## Focused proof

The retained-event regression uses a fake persisted reader that bypasses provider
sanitization, exercising the real HTTP/SSE mapping rather than claiming a new
SQL migration. Before correction, ten summary cases and the later-poll test
failed while four safe controls passed. After correction, all fourteen REST/SSE
cases and the real subsequent poll passed in 6.991s:
`go -C apps/api test ./internal/httpapi -run 'TestRetainedEventSummariesAreRedactedAtHTTPBoundaries|TestRetainedEventStreamPollKeepsDeploymentRedactionPolicy' -count=1 -v`.
Full HTTP and masking packages then passed in 5.821s/0.810s. Private
`.tmp/fresh-audit-retained-event-before.log`,
`.tmp/fresh-audit-retained-event-after.log` and
`.tmp/fresh-audit-retained-event-package.log` retain those outcomes. Independent
review returned 9/10 within the amended scope; complete replacement D now passes.
For retained credentials, follow the
[upgrade response](../../operations/upgrade-rollback.md#retained-event-text-and-exposure-response)
([فارسی](../../operations/upgrade-rollback.fa.md#متن-رویداد-قدیمی-و-پاسخ-به-افشا)).

Backend private URL overlay `TestAuditURLCredentials` failed all three initial
text/JSON cases with marker-free messages. The exact original Allow function
failed all four missing path forms. The unchanged captured originals remain
separate from the corrected source. The corrected real handlers and shared
masking passed:

```text
go -C apps/api test ./internal/provider/logsafe ./internal/httpapi -run 'TestMessage|Test.*AllowedMethod|TestAuthRouteMethodAllowSets|TestWriteProviderProblem|TestLogDownload' -count=1
```

Exit 0; private `.tmp/audit-backend-20261018/affected-tests.log` and exit marker
remain available. A later independent generated-marker overlay found the
apostrophe and truncation residuals; those failures are retained rather than
hidden by this earlier pass. The final correction carries truncation through
text and recursive structured masking, removes possibly incomplete URL
authorities conservatively and masks broken truncated JSON suffixes. Three
apostrophe/all-subdelimiter and five clipping tests failed before correction;
the complete shared masking package then passed in 1.539s with normal module
verification. Additional peer probes then found a credential URL in a JSON
member name and a scheme embedded after an underscore; both failed before the
next correction. Removing the word-boundary restriction and masking the whole
object when a member name contains credential userinfo prevents both exposure
and rewritten-key collisions. Final package verification passed in 1.055s;
`.tmp/reaudit102-url-final-b.log` and exit marker 0 preserve that replacement.
The four independent peer overlays then passed in 0.932s, retained in
`.tmp/reaudit102-peer-url-final.log`.
The earlier `.tmp/reaudit102-url-key-before.log` remains failed. Cases include local clipping before the at-sign,
upstream-truncated text, structured values, escaped JSON slashes and broken
JSON. Processing and output bounds remain unchanged.

The event negative overlay `TestAuditDeniedEventPageProbe` failed with
available/partial status and one retained event. Real SQL
`TestNotificationKeysetUsesByteOrderUnderNonByteCollation` and
`TestAlertRuleKeysetUsesByteOrderUnderNonByteCollation` both failed under an
owned `en-US-x-icu` column collation. The native name parser regression failed
before correction; a safe nonexistent-name probe independently confirmed libpq
equals interpretation. No customer data or raw secret entered that proof.

After correction, focused authority/keyset/ledger-14/16-to-17 checks passed.
All five related packages then passed uncached against disposable PostgreSQL
17.11, with native tools explicitly enabled:

```text
go -C apps/api test ./internal/observation ./internal/postgres ./internal/postgresops ./internal/alerting ./internal/notification -count=1
```

Observed package times: observation 1.209s, PostgreSQL 116.660s, native operations
10.797s, alerting 1.153s and notification 5.107s. `pg_dump`/`pg_restore` were
17.11. The special-name test creates only unique owned databases, verifies
native backup/empty-target restore, retained marker and restore generation 1,
then checks ownership before cleanup. These focused runs are transcript proof;
the full gate supplies a durable aggregate receipt.

Interface proof uses the pinned runner with `--pool=forks`:

```text
corepack pnpm --filter @stackplore/web exec vitest run src/features/product/product-view-search.test.ts src/features/product/product-source.test.tsx src/features/product/severity-labels.test.tsx src/features/product/ExplorationLogsPage.download.test.tsx src/features/product/event-filters-api.test.ts --pool=forks --reporter=dot
corepack pnpm --filter @stackplore/web exec vitest run src/features/product/AlertRulesSettingsPage.test.tsx src/features/product/TopologyGraph.test.tsx src/features/product/topology-model.test.ts --pool=forks --reporter=dot
```

The severity set passed 20 tests in five files (24.95s); rule/topology passed
27 in three files (13.99s). Twelve tests are new. Type checking and diff checks
also passed. Private logs retain initial severity/parser/history/locale failures,
one worker startup timeout, three target rule/node failures and the first
topology fixture's trailing-space mismatch (25 pass/2 fail). Correcting that
fixture and running the real fork pool yielded the recorded 27 passes without
weakening product assertions. Palette focused proof is five passing tests;
release/policy focused proof is 29 passing tests. The account render-window
negative proved 100 rendered rows where 50 were required in both locales. After
correction, 17 tests in `user-admin-flow.test.tsx` and `users-api.test.ts` passed
(24.19s), including 103 accounts across three pages, cached forward/back access,
a second administrator outside the visible window, refresh/action reset,
failure/retry and stale aborted/401 responses. Type checking passed. Product
focused totals became 64 passes in ten files, with 18 new regressions; palette
proof is separate. Private `.tmp/fresh-audit-users-before-b.log`,
`.tmp/fresh-audit-users-after.log` and `.tmp/fresh-audit-users-typecheck.log`
retain their respective outcomes. The final deferred Cancel/deletion regression
then failed once alongside 11 existing passes; an earlier wrong-label fixture
attempt is retained separately. Disabling Cancel during saving/loading yielded
29 passing rule/users tests in three files (22.06s), with scoped lint passing.
Private `.tmp/fresh-audit-rule-lock-before-b.log` and
`.tmp/fresh-audit-final-rule-users-after.log` retain that transition. The product
focused total is now 65 unique tests across ten files, with 19 new regressions;
this is a sum of named focused runs, not a replacement full-suite receipt.
Independent frontend review is 9/10 within that domain. The later complete local,
exact main/tag, signed publication and independent public/archive results below
support current release acceptance separately from those focused checks.

## Remaining scope and acceptance

The account list requests 50-row API pages and now renders at most 50 rows per
window. All manually fetched accounts remain accessible through previous/next
controls; the cache can still grow with requested pages until refresh/unmount.
There is no new fixed total-account cap or virtualized-list library. Existing
infrastructure/provider bounds remain unchanged.

An intermediate full API run failed and is retained at
`.tmp/reaudit102-full-api.log`. Its noncanonical local `GOTMPDIR` spelling
caused strict path tests to fail; correcting the host path must not relax the
production path policy. That run also compiled the changing redaction tree
before its final fixes and hit a native fixture cleanup deadline under
concurrent SQL. Independent reproduction found a slow paced checkpoint exceeded
the ten-second drop deadline; reusing the closed administrator connection then
misreported ownership drift. The test-only correction uses a fresh owner-checked
connection for each target, one bounded six-minute cleanup budget, final absence
checks and teardown after the fixture gate closes. Focused native proof passed
in 2.81s and its package passed in 4.745s. No production timeout or ownership
check changed. The failed run remains failed; a complete replacement gate was
started with a canonical native temporary path. Its first complete verify
attempt then failed on 18 lint findings in the new palette/users code;
`.tmp/reaudit102-full-verify.log` retains that outcome. Corrections and a new
complete replacement were required. Replacement B then passed exit 0 in
`.tmp/reaudit102-full-verify-b.log` and its exit marker: all Go packages with the
native flag enabled, domain/alerting coverage 80.6%/82.6%, 40 client tests,
29 release/policy tests, 477 web tests in 49 files, 725 locale messages,
235 bounded schemas, lint/type/contracts/notices/version/runtime policies and
web/API builds. The standard thread-pool web run completed in 138.65s; coverage
is 82.99% statements, 78.99% branches, 87.02% functions and 84.54% lines. Go
retains accepted cache reuse; changed native operations ran in 7.573s,
masking 0.940s and HTTP 4.899s. Separate focused SQL/native proof above was
uncached. Normal module checksums and production path restrictions stayed
enabled. Strict Helm lint passed one chart; the fresh production browser
then passed all 14 cases in the
[scoped UI receipt](browser-gate.json): 12 guest in 1.7 minutes and normal
English/Persian journeys in 7.2s/6.8s. Eight profiles measured 64 routes and
eight palette/focus/accessibility/viewport checks. Twenty-four screenshots were
captured; four were visually reviewed: English 360px light palette/topology,
Persian 360px dark palette and English 1280px light topology. These are the
recorded UI build's results. The later backend escaped-slash finding required
corrected-source gate C, whose result follows. Exact hosted checks were still
pending at that earlier checkpoint; current main/tag results follow below.

After the final multi-layer escaped-slash correction, complete replacement C
passed exit 0, including final web/API builds. The
[current local receipt](local-gate.json) binds its transcript hash and retains
B's earlier-state hash/coverage separately. C passed all Go/native/core checks,
40 client tests, 29 release/policy tests, 477 web tests in 49 files, 725 locale
messages, 235 bounded schemas and all format/docs/lint/type/contract/notices/
version/runtime policies. Current web coverage is 82.91% statements, 78.68%
branches, 86.71% functions and 84.48% lines; the standard pool completed in
291.80s. This timing differs from the earlier run and is not a throughput claim.
Module checksums, native-tool enablement, ownership/path checks and coverage
thresholds remain unchanged. Browser proof remains scoped to its recorded
precommit UI snapshot; exact committed backend/browser checks are still required
in hosted jobs.

The later retained-event response amendment passed complete replacement D,
exit 0 through final web/API builds: all Go/native/core, client 40, release/policy
29 and web 477/49 files. The web run completed in 206.67s with coverage
82.91% statements, 78.68% branches, 86.71% functions and 84.48% lines. The
[current local receipt](local-gate.json) records transcript SHA-256
`8595a3ded232a88fb920555c0484da10f9bbc85a867f2e8c478c342fae4cdb31`
and preserves the earlier B/C results. C and the `18e8558` hosted/trial outcomes
remain true only for their earlier source. Current immutable `v1.0.2` points to
`376ed0379900ab870dd8eb8af6b55ef3e5ec389b`. Main `39132989641` passed all six
required jobs; signed trial `39133022021` package passed. The independent
[current trial consumer](consumer-trial.json) verified 30 payload checksums,
all 31 provenance subjects and seven SBOM attestations under exact-source and
hosted-runner policy, rejected a wrong source, found zero blocking findings in
eight strict scan reports/zero secret findings, and executed Windows recovery
with 17 migrations plus Linux native/database archives. The later public proof
binds Windows equality and separate database archive execution below.

## Exact tag failure, reproduction and unchanged-source retry

The first tag CI `39134154907` code attempt failed at the worker control-body
replacement assertion before the update/acceptance assertions ran. Eleven
guest and both normal-language journeys passed. The original control status
was not recorded; the uploaded artifact omitted hidden `.tmp` JSON reports,
so that hosted response cannot be retrospectively classified as 429. Release
`39134154891` correctly refused packaging and skipped publication.

The [separate actual signed-trial diagnostic](browser-fixture-diagnostic.json)
ran four cold contexts with the product's unchanged 20 requests/s, burst 40.
One context's reload logo received 429; another context's extra worker control
GET received 429 at 583ms, then returned the valid one-marker worker after a
2.5-second refill. No cold-navigation or API error was observed in those four
cases. This is a bounded rate-path reproduction, not proof of the original
hosted status or customer capacity. First private diagnostic A remains failed
on a shutdown classifier; confirmation B verified real server exit, exact-owned
database absence and unbound ports.

One failed-code-job retry on the unchanged original tag/source then passed all
14 browser cases; all six tag jobs are green with the other five original
successes. The tag was not moved. Its original failure and the first blocked
release remain failed attempts. Release `39134154891` was retried only after
that green exact tag; package `117702264862` and publication `117703943506`
passed on retry. Original failures remain failed.

A separate later main-only harness follow-up validates worker status, JavaScript
MIME, a 64-KiB body assertion and exactly one cache marker, with four attempts,
a 15-second deadline and bounded `Retry-After` handling only for 429. Safe
metadata attachments retain the response classification; the uploader includes
only the existing explicit sanitized report whitelist, never all `.tmp`.
Every PWA acceptance/cache assertion and production limit stays unchanged.
That follow-up is outside immutable release source `376ed03`. Fourteen
in-memory branch cases passed; private live helper-coverage attempts C/D remain
exit 1 because each first response was valid 200 and the intended 429 retry
branch was not reached. Owned cleanup passed, but those guards do not prove
live retry coverage. Six-job CI on the commit containing this review must pass
before handoff; commit checks and the release body bind the final run/source.
Preserve all prior tags, failed receipts and transfers.

### Later main setup failure and focused correction

Main-only review commit `7545339d82f0ab54fd483b77048dd79ab3cc3fa3` failed
CI `39138645362` during the concurrent-read case's unmeasured `/events` setup,
before the 80-read loop started. The browser result was 13/14: the complete
worker-update test passed, with one 200 JavaScript control response, 2576 bytes
and one marker. The actual artifact retained all three hidden reports; its
failed-case attachment recorded only setup-component API 429. The document's
HTTP status was not recorded and must not be inferred. Other five jobs and the
earlier full/race checks passed; the code job still failed on this browser step.

The subsequent main-only change spaces four setup navigations by 2500 ms after
their validated page/query checks, before the next route/search. It retains the
initial/preload refill and every measured assertion: 80 reads, eight parallel,
ten rounds and minimum 600-ms round spacing. Fixed public-document status
diagnostics record no raw response body; measured reads do not gain retries.

An AST-extracted execution of the exact callback/helper against the signed
`376ed03` runtime passed with the unchanged default 20 requests/s, burst 40.
All 86 setup responses were 200 with zero setup API 429, and all 80 timed reads
were 200 with valid envelopes. Actual added refills were 2504/2507/2504/2508 ms;
setup was excluded from the 6049-ms measured window. Median 86 ms and p95 129 ms
describe this local synthetic sample only. Ten in-memory diagnostic checks,
format/type/lint and exact-owned server/database/port cleanup passed. Test source
SHA-256 was `3a9d84c360b6e300fed4594a29a43658d2c6bbf6fb4b3ce019083f88ae04f838`.
[Root proof](browser-fixture-diagnostic.json) retains the failed CI and focused
pass separately. This is focused callback proof, not a complete new browser/CI
pass; six-job CI of the containing review commit must pass before handoff.
Frozen public release `376ed03` and its accepted proof remain unchanged.

## Accepted public release and final scope

Stable release `416503671` was published at `2026-10-18T14:43:05Z` for immutable
`v1.0.2`, source `376ed0379900ab870dd8eb8af6b55ef3e5ec389b`. Independent
anonymous downloads verified all 39 public assets, every API size/digest,
30 manifest entries, all 31 provenance subjects and seven SBOM attestations
against exact tagged source/workflow and hosted-runner identity. Wrong-source
rejection passed. Eight strict vulnerability reports have zero HIGH/CRITICAL
findings; secret findings are zero. Source SPDX records 470 packages, 393 npm
package URLs and 74 Go package URLs, separate from the 63-package web notice check.

Public manifest SHA-256 is
`c3409cc8300212fcbf27e83a1524ba0c64291e9f0503b8d9c7ebaf9789e45bf1`.
Public Windows archive SHA-256
`d7c1e7807b5b71946c2df274da07945e4c284421ae425f9b29febe2d36ddde1b`
matches the actual 17-migration recovery-tested trial. The different public
database archive SHA-256
`9c5e686b1350c4ac21c68ad9234199efb04b5c9f95bf695ed8311060844ac79f`
was independently loaded and executed with PostgreSQL 17.11, gosu 1.19/Go 1.26.9,
UID/GID 70 root-drop, restricted non-root/read-only/network-none SQL, matching
configuration/rootfs identity and owned cleanup. Signed Linux amd64 native
guest/normal smoke passed. [Machine-readable public proof](published-release.json).

All nine stages and release acceptance are complete at **9/10**. No customer
production rollout follows from this project proof; customer ingress, provider
rights, receivers, backup destination, capacity and full air-gap installation
still need operator evidence. Linux arm64/macOS remain cross-built and scanned,
without a native execution claim. Historical scores/counts and every failed
attempt retain their original version and scope.
