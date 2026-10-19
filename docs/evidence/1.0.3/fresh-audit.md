# Corrective audit — 2026-10-19

Reviewed baseline: 089727fdfac0fa0bacb15c520212c3fbd37bd8b1. Scope includes
all first-party Go/React/domain/adapters, repositories, jobs, migrations,
contracts, deployment, release scripts, tests, and operator/user documents.
No schema migration is needed: ledger **17** remains unchanged.

| Finding                     | Reproduced failure                                                                                                                                                                          | Correction and proof                                                                                                                                                                                                                                                                                                                                                                         |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Retention settings          | A real PostgreSQL row eight days old was deleted despite configured 8,760-hour retention; the default 168-hour control passed.                                                              | Each bounded batch reads validated persisted settings under a shared row lock through all deletes. Real PostgreSQL tests cover all seven settings, concurrent ordering and policy reload between batches.                                                                                                                                                                                    |
| Denied native stream replay | New reads denied access while an old shared replay buffer returned retained data with Last-Event-ID.                                                                                        | Current inventory/topic denial fences queued/history frames, closes the shared feed and returns the typed 403. Poll-time denial sends reset and closes clients; a later authorized feed is independent. Forty HTTP subcases and hub tests pass.                                                                                                                                              |
| Native stream frame budget  | A legal 100-event REST page was HTTP 200 and 1,669,440 bytes while SSE closed HTTP 200 without a frame.                                                                                     | Keep the 256 KiB cap and emit a bounded snapshot_overflow reset for REST refetch. Fingerprints detect changed large data, suppress duplicates and recover smaller snapshots. Real HTTP event/log and lifecycle/boundary tests pass; full HTTP/realtime packages pass 19.922s/0.721s.                                                                                                         |
| Fixed-window refresh        | A real mounted event page refetched relative ranges (1 to 2 reads) but kept absolute ranges at one read after reset; synthetic log polling/reset each stayed at two reads instead of three. | Pass stream/poll revisions directly to bounded pagination; preserve exact from/to and filters. Three failing-before component regressions reproduce this additional final-review finding. Three corrected regressions and the related seven-file/46-test suite pass; full web/browser type and scoped lint checks pass. Complete local E now passes all 530 web tests with unchanged source. |
| Matrix delivery test        | Browser composition put the adapter query inside a path rejected by the real transport; zero requests reached the endpoint.                                                                 | Pass the adapter as the transport query, with actual client composition tests.                                                                                                                                                                                                                                                                                                               |
| Unicode lengths             | Legal supplementary-code-point summaries and searches failed UTF-16 length checks.                                                                                                          | Shared bounded code-point helpers align parsing, drafts, URL/search and form inputs with the contract. Legal supplementary text passes; over-limit text remains rejected.                                                                                                                                                                                                                    |
| Browser response budget     | A legal 100-event Persian page was HTTP 200 and 1,669,340 bytes, exceeding the browser's inherited 1 MiB budget.                                                                            | Production browser factory uses the server's bounded 16 MiB ceiling. Generic client remains 1 MiB by default; Content-Length and streamed over-limit rejection stay enforced.                                                                                                                                                                                                                |
| Alert defaults              | Configured window 900 seconds and pending 23 seconds still opened a 300/60-second draft.                                                                                                    | New rules read effective settings; failure blocks creation with retry. Late settings do not overwrite edited fields or existing rule drafts.                                                                                                                                                                                                                                                 |
| Helm names                  | A legal long release rendered a 67-character Service name rejected by the pinned Kubernetes DNS validator.                                                                                  | Stable hashed API names stay within 63 characters, with matching Deployment/Ingress references. Previously valid names, ServiceAccount and runtime PVC identities remain stable. Nine actual render cases and strict lint pass; server validation is a separate CI gate.                                                                                                                     |
| Retry controls              | Data errors on history/exploration pages exposed no retry action.                                                                                                                           | Localized retry restarts bounded reads without reload and preserves source/filter context. Nine error/recovery page tests cover the missing action.                                                                                                                                                                                                                                          |
| Environment navigation      | Required shell environment selector was absent.                                                                                                                                             | Bounded lazy-loaded selector binds source/environment URLs on resources/search/events/topology. Other destinations remain source-wide; choosing there navigates to resources. Filters survive same-route choice and dependent targets/cursors clear.                                                                                                                                         |

## Correction loop and proof limits

Initial review: **8/10**, due to the verified defects and missing required controls.
The first implementation review found same-route environment selection discarded
search filters; this was reproduced by three failing route tests, then corrected.
The remaining fixture matcher errors retain their failed outcomes separately.
Independent follow-up found a second Helm boundary: valid numeric release or override names produced a Service starting with a digit. Two real renders failed the official Kubernetes DNS1035 validator. An API-only alphabetic prefix fixes that input while preserving ServiceAccount and PVC identities. Nine independent real renders now pass that validator; nine contract renders and strict lint pass. The scoped independent code/architecture review is **9/10**, with no remaining verified defect in the reviewed source. A later root boundary review reopened the score at **8/10** after the oversized-native-frame failure. That bounded-reset fix passed real HTTP and complete HTTP/realtime packages. Independent client review then reproduced fixed-window event/log refresh failures; all three regressions pass after explicit refresh revisions, with 46 related tests and type/lint checks. The final reviewed code returns to **9/10**, with no remaining verified source defect in that scope. Whole-delivery acceptance still requires the final local, hosted and artifact gates.

Focused receipts remain in private ignored scratch, with no credentials or
customer payloads committed. Retention negative and positive probes used real
disposable PostgreSQL 17.11 databases with owner-checked cleanup. SSE probes used
real HTTP clients and provider authority transitions. Unicode/body probes used
the real browser transport factory, not a mocked replacement transport.

The pre-fix full gate B passed, demonstrating that the old suite missed these
boundaries. Its SHA-256 is
b9c226f612b41f2c5dd38bd6458ed0132ad54ef977ac43731689c5a131e25abe;
the earlier cold-cache failure remains failed. A pre-fix passing suite does not
certify these new changes. The [earlier local E receipt](local-gate.json),
[replacement local F receipt](local-gate-f.json) and
[browser B snapshot](browser-gate.json) record their own source scope, commands,
failures and results. Browser B precedes the final Helm/hub corrections; exact
replacement-source hosted browser acceptance remains a separate required gate.

Hosted Compose, external-TLS Helm, live provider, race, packaging/signing and
public-archive execution need fresh exact-source proof. Customer installation,
credentials, receiver delivery, ingress and capacity remain operator-specific.

## First candidate failures and correction

Candidate `1a3d79638faf86615b2ea2c540282edd32d421d4` was pushed after full local E.
Main CI `39230365227` failed its code and Helm-install jobs; its other four jobs
passed. The separate signed trial `39230384305` passed packaging and correctly
skipped publication. That trial is not accepted for release: the required main
gate failed. No tag or public 1.0.3 release was created for that candidate.

The code job reported three assertion failures during the concurrency gate,
without a detector data-race report. A deterministic cancellation barrier then
reproduced a semantic ordering defect: cancellation could close a subscriber
before the hub committed revocation and drained its queue. The score reopened
at **8/10**. The correction commits revocation, replay removal and all queued
subscription closures under the hub lock, releases that lock, then cancels the
producer. Manager-to-hub lock order and replacement-feed ownership stay intact.
The old-code barrier test failed; the corrected test and complete Windows HTTP
and realtime packages passed uncached in **22.687s / 1.659s**. The large-payload
fixtures now use the actual production 15-second request timeout and serialize
their heavy setup. Production deadlines and the existing 3-second, 100-ms and
350-ms test bounds remain unchanged. Replacement full local F passes with
unchanged source: 530 web tests across 53 files, 40 client tests, all Go/native
checks and unchanged coverage floors. Linux detector proof and the replacement
hosted/artifact gates are still required.

The Helm-install job caught numeric YAML labels that a DNS-only local check had
missed. All dynamic string names, labels, selectors and object references are
quoted, including the complete PVC identity and ingress class. A bounded strict
Kubernetes typed decoder now checks every rendered object and cross-object
reference. **23** real chart profiles and strict Helm lint pass locally. Separate
peer probes include **16** typed positive/negative fixtures and **11** namespace
argument/reference fixtures. Those probes caught and corrected the initially
missed numeric ingress class. Source review found no additional defect in these
corrections. Actual server validation and external-TLS installation remain
required hosted gates; the earlier failed run keeps its failure.
