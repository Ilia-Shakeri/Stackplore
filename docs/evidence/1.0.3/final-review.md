# Final independent review — 1.0.3

Frozen release source: **656581cf05d4774358d5644a6e1af3c69a210f81**; tag **v1.0.3**; PostgreSQL ledger **17**.
Reviewed baseline: 089727fdfac0fa0bacb15c520212c3fbd37bd8b1. Final scoped code and
delivery review: **9/10**, **9/9** roadmap stages complete.
This record requires the actual successful [public receipt](published-release.json),
[signed trial](consumer-trial.json) and [exact-source browser receipt](main-browser.json).

## Nine defect areas and two required controls

| Area                    | Verified correction                                                                                                                                                                                              |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Retention               | Persisted validated policy is read and locked through each bounded deletion batch; real PostgreSQL tests cover policy changes and lock ordering.                                                                 |
| Denied stream replay    | Fresh inventory/topic denial rejects subscription before replay; observed poll denial revokes the feed, ring and queued frames. Recovery uses a separate feed. Already-sent network bytes cannot be recalled.    |
| Native stream budget    | Legal oversized snapshots emit bounded snapshot_overflow resets; changed data, duplicate suppression and small-snapshot recovery retain the 256 KiB cap. Full maximum-length event/log HTTP fixtures remain.     |
| Fixed-window refresh    | Stream and poll revisions refresh absolute event/log ranges without changing their bounds or filters.                                                                                                            |
| Matrix test delivery    | Adapter selection reaches the actual transport query rather than an invalid endpoint path.                                                                                                                       |
| Unicode limits          | Shared code-point bounds admit legal supplementary text and still reject over-limit values.                                                                                                                      |
| Browser response budget | The production transport explicitly uses the server's bounded 16 MiB ceiling; generic defaults and streamed over-limit rejection remain.                                                                         |
| Alert defaults          | New drafts read effective settings with retry; stale settings cannot replace edited or existing drafts.                                                                                                          |
| Helm names and types    | Stable API names obey Kubernetes limits; quoted YAML string values decode strictly. ServiceAccount/PVC identity and references remain stable; 23 typed profiles and real server validation cover the boundaries. |

Required controls: localized retry preserves filter/source context; the lazy,
bounded environment selector preserves supported URL filters and clears dependent
targets/cursors. EN/FA keyboard, focus, mobile drawer and accessibility journeys
run against the actual browser transport.

## Correction loops and retained failures

Initial quality was **8/10**. Verified fixes raised the scoped review to **9/10**;
independent follow-up reopened it for oversized snapshots, refresh boundaries,
close ordering, YAML typing and the lifecycle fixture. Each failing proof remains
retained; corrected regressions and matching gates return the final review to **9/10**.

- Candidate 1a3d79638faf86615b2ea2c540282edd32d421d4: main 39230365227 failed code and Helm installation; four other jobs passed. Signed trial 39230384305 packaged successfully but was unaccepted because main failed. No tag/public acceptance belongs to that candidate.
- The code failure contained three assertions without a detector data-race report. A deterministic negative barrier proved producer cancellation could precede revocation/queue commit. The fix commits state under the hub lock before cancellation; lock order and replacement ownership remain intact. Heavy HTTP fixtures use the real 15-second request timeout and are serialized.
- Candidate fdb96008160f692d37daac3133d4881c4100177c: main 39233787971 failed only the lifecycle/events first-overflow wait during the concurrency gate; five other jobs passed. Signed trial 39233804624 packaged successfully but remained unaccepted. No additional test or detector data-race failure appeared in the retained code-job log.
- Pinned Linux negatives and safe timing instrumentation showed 1,670,083-byte snapshot builds taking 2,090ms and 2,255ms; changing a generation during an old build made the next assertion wait for two builds. The lifecycle test now proves unchanged-read completion with barriers and an explicitly oversized 100 × 2,622-character fixture. Separate HTTP tests keep maximum Persian payloads. Production limits and 3-second/100-ms/350-ms test bounds are unchanged.
- The old-code queue barrier, Linux focused-a and timing-b failures remain in private ignored scratch. Earlier cold-cache, fixture, lint and local-gate failures keep their recorded failed outcomes in the linked audit and local receipts. They are not relabeled as successful final-source runs.

Complete corrected Linux HTTP/realtime detector packages passed uncached in
29.022s / 2.039s; focused detector checks passed in 18.246s / 2.043s. Windows
lifecycle and API/native checks passed. [Local E](local-gate.json),
[local F](local-gate-f.json), [browser B](browser-gate.json) and
[lifecycle proof](lifecycle-race.json) retain their distinct precommit source scope.
See the [detailed audit](fresh-audit.md) for the failing-before and passing-after boundaries.

## Exact source, public artifacts and runtime proof

- Exact main **39235362032** and tag **39238088634** each completed all six required jobs successfully. Code/full verification, the concurrency detector, real PostgreSQL, live Docker/Kubernetes providers, Compose installation and external-TLS Helm installation have their actual hosted scopes; no local-only gate stands in for those integrations.
- Release **39238088607** succeeded in package **118011975469** and publish **118019458409**. Public release **417182030** was published at **2026-10-19T10:51:50Z**.
- All **39** public assets were fetched; **30** manifest entries, **31** provenance subjects and **7** SPDX attestations were verified against the exact tag/source and hosted workflow identity. Manifest SHA-256: `5f67f8cfc7236b2e808c3b87864fb38ebc169466d6e5c93a2d890999238f8bdd`. The explicit all-zero wrong-source probe was rejected with nonzero exit **1** and matching actual-source diagnostics.
- All **8** strict published scans have zero HIGH/CRITICAL findings; the secret scan has **0** findings. Source SPDX contains **470** packages, **393** npm and **74** Go package references. These are results at the recorded scan time.
- Signed trial **39235380836**, artifact **12178772825**, was independently consumed; ZIP SHA-256: `4e7f76183f0f40a57ea85e70ffc69a7cda76b0b0de2c1be761f7267dd2638851`. Trial success is distinct from the later public acceptance.
- Windows archive SHA-256: `66787ccdb48bd19900da24b6036eedd95087cca5e6a0a4899e77cecff7086e1e`. Its actual archive bytes bind migration/recovery, durable acknowledgement/replay across restart, native backup/restore, session/preauth invalidation, fresh login and owner-checked cleanup. Runtime source: **actual trial and public archive SHA256 match**.
- The public database archive was independently loaded and queried under isolated Linux amd64, PostgreSQL 17.11, gosu 1.19, Go 1.26.9; root dropped to UID/GID 70, networking was disabled and cleanup was owner checked. Archive SHA-256: `a9fbec264f7e01440a7130d46dd0a3c4bcd0fb0d8a94cdbea8f859a258bbf869`. Its bytes differ from the separately tested trial database archive; the public runtime proof is independent either way. Linux amd64 guest/normal native smoke also passed with disposable database cleanup.
- Main browser artifact **12178634390**, ZIP **23730383** bytes, SHA-256 `3a4031077f8cdbbe1e4be250d23981cd31f857225f8689294ede086b5638e892`: three actual hidden reports passed **12 + 1 + 1** cases on sole attempts. Eight profiles measured 64 routes and eight palette journeys; four EN/FA light-profile environment journeys each used two reads, with two mobile drawer checks. Twenty-four PNG headers were verified; four exact final-artifact screenshots were visually reviewed. Fixed load: 80 reads, eight parallel, ten rounds, 600ms spacing; all 80 responses/envelopes passed. Setup refill waits remain outside measured load time. Observed duration 6005ms, median 30ms and p95 45ms describe this disposable fixture, not customer capacity.

Receipt SHA-256 values at review generation: public `b49aff5eaf19cf25b07723ba7ddf0b82279731a7f6228855a329bfe95eca76e4`;
trial `d036bb2f6703dbe1463cc2be230dfbd4684300506aaf09b77907f9b5720c74f3`; browser `16978f24a612b1ce01fba5f365922c9f96860f95718eb163e08e1b03165f1d21`.

## Handoff boundary

Linux arm64 and macOS outputs are cross-built, scanned and signed; no native
execution on those systems is claimed. Customer installation, source credentials,
receiver delivery, ingress/TLS, capacity and external backup destinations still
need operator evidence. Observation stays read-only by default; guarded writes
retain their independent authorization gates.

The final documentation/evidence commit is separate from frozen release source
656581cf05d4774358d5644a6e1af3c69a210f81. The six-job verification on the commit containing this
review must pass before handoff. Checks and the public release body bind that
final commit/run externally; no recursive receipt commit is required. None of
these later review records changes the immutable released binaries or schema.
