# Smart Assistant — reviewed document approval/search preview

Version **2.0.0-alpha.7**, schema **20**, immutable source
`94425f8f6a690c3bf4628485ae92832ef6a2574a`.

## Exact-source receipts

| Boundary | Receipt | Result |
| --- | --- | --- |
| Full source verification | [42470870399](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42470870399) | All eight jobs pass |
| Signed package/security trial | [42470901375](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42470901375) | Package passes; branch publication correctly skipped |
| Exact-tag verification | [42473994865](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42473994865) | Attempt1 fails only the processor; bounded failed-job repeat running |
| Signed publication | [42473994721](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42473994721) | Attempt1 blocked by failed exact-tag verification; publication skipped |

The annotated tag has been pushed. No public release or stable acceptance is
claimed while the last two receipts remain pending. Every test, build, scan,
processor and database fixture ran on GitHub-hosted infrastructure. The maintainer
host was used only for reading/editing files, lightweight Git and receipt access.

## Implemented boundary

Migration20 adds bounded reviewed corpus revisions, document full-text indexes,
a catalog revision and metadata-only one-use confirmations. Approval/revocation
requires current administrator/session, exact material/version/restore generation,
explicit confirmation, a two-minute preview and a dedicated recent-auth grant.
Content, confirmation, grant and safe audit commit atomically.

Private HTTP contracts and generated client types expose administrator metadata,
preview/commit, body-only search, and exact reference opening. Search/read enforce
current language, audience and restore generation. Canonical identifier ordering
is independent of database collation. Opaque cursors bind actor/session and query.
The separate English/Persian desktop page renders plain text, clears private state
on owner/language/page changes, discards changed/expired previews, moves keyboard
focus into opened content, and preserves unknown commit outcomes.

Real PostgreSQL tests cover material substitution, approval replay, audience and
locale, replacement, revocation, restores, revoked sessions, atomic audit failure,
expired/replaced confirmations, corpus quotas, keyset ordering and query-plan
inspection. Full browser journeys approve, revoke, replace and reopen documents
as administrator and viewer in both languages. Existing provider, installation,
concurrency, offline, processor and packaging gates retain their own scope.

Code/architecture review: **9/10 within this approval/search/read boundary** after
the correction loop. It does not grade all remaining version-2 product work.

## Retained correction trail

| Source / run | Failure or limit | Correction |
| --- | --- | --- |
| `6ce64ad` / [42466426878](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42466426878) | Reader fixture bootstrapped an existing installation; verified tool download HTTP504 | Insert only the independent reader/session; preserve download failure |
| `bbba0b1` / [42467186114](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42467186114) | Focused checks pass; formatting, cluster download and processor archive fail | Apply hosted formatting; use official immutable fallback assets with unchanged hashes |
| `46aa568` / [42468005066](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42468005066) | Focused checks pass; full formatting fails | Apply hosted formatting and generated transport types |
| `9ae27a3` / [42468642643](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42468642643) | Cancellation check incorrectly narrowed across await; images fail to build | Re-read cancellation through a guard; preserve both pre/post checks |
| `24f1770` / [42468921345](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42468921345) | Focused checks and six install/provider/tool jobs pass; formatting fails | Apply hosted correction; improve focus and interrupted confirmation state |
| `8f12dcd` / [42469270826](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42469270826) | Package trial stops before scans on scanner download HTTP504 | Official API fallback, exact size, HTTPS and unchanged SHA256 |
| `e23376c` / [42469458194](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42469458194) | Strict lint and Metrics API download fail | Fix callbacks/event type/runtime guard/global; add verified fixture fallback |
| `e23376c` / [42469491808](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42469491808) | Signed trial passes, but source verification failed | Retain diagnostic proof; do not reuse it for acceptance |
| `4bd0dcf` / [42470063649](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42470063649) | Focused checks and six install/provider/tool jobs pass; formatting fails | Apply hosted correction and label restored approvals truthfully |
| `4847cdd` / [42470480150](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42470480150) | Focused checks pass; final page formatting fails | Apply the exact hosted correction at `94425f8` |

Some superseded in-flight jobs were cancelled by the branch concurrency policy.
A cancelled or failed run is not relabelled successful. Later green receipts
accept only their exact source and bounded evidence.

## Still open

This is a **bounded preview**, not completion of stage7 or stable2. Source/licence
and usage-permission metadata, selected document-to-answer/report integration,
and selected private metadata export remain separate stage7 work. Corpus prose
is not sent to inference in this preview. The seed still has 12 paired scenarios
and 18 adversarial cases rather than the 200/1000 targets. Independent semantic
review, held-out profile evaluations, pilot/soak/capacity and final stable artifact
acceptance remain open; the maintainer has no independent evaluator or pilot yet.

## Exact-tag attempt1 processor failure

The [sanitized receipt](tag-attempt-1-processor.json) records a failed
`application_browser` phase. Direct bilingual protocol/evidence cases and English
application calls pass. Persian resource explanation completes at 24114ms; the
Persian administrator investigation reaches the unchanged 40-second deadline
(1206 input tokens, 39998ms, client_closed, incomplete, no sections/citations).
Its inference step remains uncertain, and both owned processor and PostgreSQL
cleanup flags are true. No CPU cause, latency/capacity acceptance or successful
Persian report is inferred. A bounded repeat of the same failed job is allowed
only after the entire run finishes, with source, assertions and budget unchanged.
The original failed receipt remains part of the evidence.