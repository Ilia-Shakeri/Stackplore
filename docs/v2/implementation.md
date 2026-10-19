> **UNACCEPTED development source — schema 18.** This branch is isolated preparation, identified by its own commit; it is not the released `v2.0.0-alpha.0` source at `eb86ab7554db555f023c1de72c3119acf101226a` (schema 17). Normal startup automatically applies migration 18 even while the new capability is disabled. Use only a fresh disposable test database. Do not point this source at an accepted or customer schema-17 database. Returning an older binary requires its verified backup/restore procedure. Existing receipts do not accept this branch. Real processor/profile and stage exit gates remain open. The configured HTTP log path now delegates to the shared policy; this branch also changes an existing path.

> **شاخهٔ توسعه، نپذیرفته — طرح پایگاه ۱۸.** این کد، آماده‌سازی جدا با کامیت خودش است؛ منبع منتشرشدهٔ `v2.0.0-alpha.0` همان `eb86ab7554db555f023c1de72c3119acf101226a` و طرح ۱۷ است. شروع عادی این شاخه، مهاجرت ۱۸ را خودکار اجرا کند؛ خاموشی قابلیت مانع آن نیست. فقط پایگاه تازه و دورریختنیِ تست به کار رود. به پایگاه پذیرفته یا مشتری با طرح ۱۷ وصل نشود. بازگشت باینری قدیمی، روش پشتیبان/بازیابی تأییدشده لازم دارد. رسید قدیمی، پذیرش این شاخه نیست. گیت پردازندهٔ واقعی و پایان گام‌ها باز است. مسیر HTTP گزارش نیز از سیاست مشترک استفاده کند؛ پس یک مسیر موجود هم تغییر کرده است.

# Version 2 implementation ledger

Started: **2026-10-19**. Authorized scope: implement the complete
[roadmap](../roadmap-v2.md) ([فارسی](../roadmap-v2.fa.md)), test each stage,
update documents, version, commit and push. Stable 1.0.3 remains independently
accepted at `656581cf05d4774358d5644a6e1af3c69a210f81`, schema 17.
Baseline documentation source: `c018cf09c20456d27dfa6daa3f74e342fe46e27e`.

## Stage status

| Stage | Target                 | State                                | Acceptance still required                                              |
| ----- | ---------------------- | ------------------------------------ | ---------------------------------------------------------------------- |
| 0     | `2.0.0-alpha.0`        | Local candidate verified             | Exact-source hosted CI, signed packaging and prerelease consumer chain |
| 1     | `2.0.0-alpha.1`        | In progress; isolated implementation | Gateway tests and real pinned profile proof                            |
| 2     | `2.0.0-alpha.2`        | In progress; isolated implementation | Real SQL state/lease/quota/recovery proof                              |
| 3     | `2.0.0-alpha.3`        | Not delivered                        | Shared privacy and evidence tools                                      |
| 4     | `2.0.0-alpha.4`        | Not delivered                        | Bilingual grounded conversation                                        |
| 5     | `2.0.0-alpha.5`        | Not delivered                        | Bounded investigation loop                                             |
| 6     | `2.0.0-alpha.6`        | Not delivered                        | Incident analysis                                                      |
| 7     | `2.0.0-alpha.7`        | Not delivered                        | Approved corpus and safe history                                       |
| 8     | `2.0.0-alpha.8`        | Not delivered                        | Held-out quality, attack and failure gates                             |
| 9     | `2.0.0-beta.1`         | Not delivered                        | Actual pilot, upgrade and deployment                                   |
| 10    | `2.0.0-rc.N` → `2.0.0` | Not delivered                        | Independent candidate and stable acceptance                            |

Parallel code preparation does not bypass a predecessor's exit gate. No helper
capability is enabled or claimed shipped by this ledger. Stage 0 now has a
versioned `alpha.0` checkpoint because the maintainer requested a version at
every stage; this supersedes the proposal's unversioned planning checkpoint.
Existing tags remain immutable. New tags wait for their own passing gates.

## Stage 0 decisions and acceptance

- Accepted design: ADRs 0014–0016, additive SPEC section 15 and architecture.
- Evaluation: [protocol](evaluation.md), [threat model](threat-model.md), and
  [prerelease compatibility](prerelease-compatibility.md). Seed fixtures do not
  meet release corpus size, independent review or live quality gates.
- Source boundary: read-only fixed tools, one Go runtime, PostgreSQL, owner-bound
  runs, volatile text, default-off/no egress and unchanged v1 installation roles.
- Checkpoint labels: `alpha.0` for decisions, `alpha.1`–`alpha.8` for implementation,
  `beta.N`, `rc.N`, then actual stable `2.0.0`; each accepted artifact uses its
  own source, checksums, scans, signatures and independent consumer receipts.

## Commands and honest outcomes

Pinned Windows tools: Node 24.18.0, Go 1.26.9; use `npm.cmd`. Native PostgreSQL
17 fixture is a new owned loopback instance on port 55474 in a dedicated D-drive
scratch directory. Existing database files and unrelated `artifacts/` are
untouched. Heavy temporary output uses the D drive; initial C-drive free space
was 3.7 GiB, available memory about 2 GiB of 15.6 GiB.

| Attempt           | Command or procedure                                                                 | Outcome and scope                                                                                                                                           |
| ----------------- | ------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Baseline A        | Pinned `npm.cmd run verify`; `.tmp/v2-stage0-baseline-verify.log`                    | Failed at offline Go notice graph; inherited module cache was unavailable. No test weakened.                                                                |
| Cache preflight   | `node scripts/check-go-notices.mjs` with explicit existing user Go module cache      | Pass: 72 modules, 106 exact upstream files, Linux/amd64 and Windows/amd64.                                                                                  |
| Baseline B        | Pinned `npm.cmd run verify`; `.tmp/v2-stage0-baseline-verify-b.log`                  | Failed formatting while new corpus files were still being written. Not a clean baseline or runtime failure.                                                 |
| Prerelease policy | `node --test scripts/release.test.mjs scripts/runtime-images.test.mjs`               | 35/35 pass for this slice; actual Helm package/show/render and inert archive round trips cover four checkpoint forms. Real OCI/publication remains pending. |
| Database fixture  | Native `initdb`, `pg_ctl` and `psql` against a new owned loopback directory/database | Started; SQL/migration acceptance remains a separate gate. Existing fixture was not restarted.                                                              |
| Automatic review  | Attempt to start the previous PostgreSQL fixture                                     | Rejected: only reason returned was `blocked by policy`. Switched to a fresh, owned test database.                                                           |
| Automatic review  | Attempt to launch a newly downloaded local processor binary                          | Rejected: only reason returned was `blocked by policy`. No alternate launch path used; real profile acceptance remains pending.                             |

## Subsequent verification and supplied artwork

- Frozen baseline C used staged tree `5078b8f1255755a5c1b816920a978da2e0390842`.
  All Go/native SQL tests, 35 release checks, 11 corpus checks, 40 client tests
  and 530 web tests pass. Build correctly rejects dependency links outside the
  exported workspace; overall `npm.cmd run verify` fails. Preserve
  `.tmp/v2-stage0-frozen-verify-c.log`; do not call this attempt green.
- A new independent export installed all 342 packages offline from the existing
  C-drive store, with zero downloads. The initial default D-drive store attempt
  lacked its own cache and failed. Main workspace links were restored before
  testing the current interface. Removing old exported dependency junctions was
  rejected by automatic review with only `blocked by policy`; they were left
  untouched and no alternate deletion was attempted.
- `.tmp/v2-probe-installed-model.mjs` ran two synthetic schema/locale probes on
  the installed local service: English 7,818 ms and Persian 7,518 ms, both pass.
  No raw answer retained; pre-dispatch counting and grounding remain unproven.
- The maintainer supplied three logos and requested placement, compression and
  a matching theme before continuing. [Brand inventory](../brand.md) records
  twelve derivatives totaling 525,385 bytes, an 85.5% reduction. Original source
  files remain unchanged. `node scripts/check-brand-assets.mjs` checks exact
  digests, dimensions and total bytes; it passes.
- Current branding checks: 57 focused tests across three files, scoped lint
  and strict UI/web/browser types pass. Corrected an insufficient info tint
  instead of reducing the 4.5 text-contrast floor. Real browser proof remains a
  separate gate. The public cache stays limited to four paths; its logo is now
  a PNG. This change is not proof of helper runtime completion.
- Review score for the branding slice was 8/10: the server still gave the new
  offline icon a private cache policy, the worker test supplied its former MIME
  type, and the offline page ignored an explicit theme preference. Fixed the
  exact public icon allowlist, kept all other logo files outside worker storage,
  resolved the selected theme in the existing hashed offline script, and added
  opposite-device theme checks without expanding the four cached paths.
- Frozen attempt D uses tree `2cd2bafe646ea5a41be056ea82825ff74c12dd4b`; it
  predates those review corrections. Its database variables were named
  incorrectly, so skipped SQL integrations cannot count as database proof.
  Keep `.tmp/v2-stage0-frozen-verify-d.log`; the corrected full gate uses the
  repository's actual `STACKPLORE_TEST_POSTGRES_URL` and native-tool opt-in.
  Attempt D ends with 536/537 web tests passing and the expected obsolete
  icon-MIME fixture failure; the complete command fails. The corrected slice
  passes 33 offline/branding tests plus the complete HTTP asset-header boundary
  test. These focused results do not replace the next full gate or browser run.
- The versioned [browser artifact consumer](browser-evidence.md) preserves the
  exact fourteen-case journey, 80-read workload and eight locale/size/theme
  profiles while checking all 32 screenshots, including the new About views.
  Archive fixtures test the consumer only; actual browser and hosted source
  identity still require their own receipts.

## Profile selection and unresolved gates

Corrected frozen attempt E, tree
`ee35c24874c0b81e596dcb1c790c78f358329e1b`, passes the full local gate:
544 web tests, 40 client tests, 35 release checks, 11 corpus tests, real native
PostgreSQL 17.11 and recovery tools, and production build. Fourteen actual
browser cases also pass without retry; 32 screenshots and 64 route measurements
cover both languages, themes and viewport sizes. Both explicit offline themes
override opposite device preferences. The original logos retain their hashes.
[Local candidate evidence](../evidence/2.0.0-alpha.0/README.md) records exact scope,
log digests, unchanged workload, visual review and the post-gate documentation/
consumer-diagnostic wording changes. Branding review is 9/10 for this slice;
new runtime functionality is not part of this candidate.

The maintainer confirmed that no compatible local process is already running.
Stage 1's real-profile test therefore remains unavailable. Independent code
preparation and its synthetic/real-SQL checks do not close that predecessor gate.

Read the official local processor documentation before choosing the adapter.
The installed Ollama 0.12.9 exposes Qwen3 4B/8B, structured output and reported
post-generation token counts. It does not establish an exact pre-dispatch
counter for the complete chat envelope, so it is a probe candidate, not an
accepted production profile. No remote profile or paid fallback is enabled.

A pinned direct llama.cpp server can apply its template, tokenize it and accept
the exact numeric token sequence for completion. Candidate build **b11729**, source
`412a363c6`, official Windows CPU archive SHA-256
`aef737d26bffd08ceeb7be75f045bdef6d1ecc3187e0f4a1a3b0d878a03d2550` was verified.
Its launch was rejected by automatic review; downloaded bytes are not runtime
proof. Gateway construction and real TLS/network tests can proceed separately.
The operator must bind the immutable single-model process, model checksum,
template hash, build and endpoint; unsupported or changed identity fails closed.
No router/model-loading endpoint or arbitrary path is available to the browser.

The initial 53–88 person-day estimate remains provisional. Current machine
memory and the real-profile execution gate prevent a justified capacity or
quality claim. Do not close stage 1 using a mock, byte estimate or one successful
network response; do not claim stages 0–10 complete before their own receipts.

## فارسی — وضعیت اجرا

کار همهٔ گام‌ها مجاز؛ پایان هنوز ثابت نشده. گام صفر نسخهٔ `alpha.0` دارد؛
این خواست تازهٔ کاربر، طرح قدیمِ بدون نسخه را جایگزین کند. گام‌های بعد با
گیت خود بسته شوند؛ کد موازی، مدرک پایان نیست.

پایگاه تست تازه و جدا بالا آمد. کش Go درست پیدا شد؛ شکست نخست و شکست قالب
حین نوشتن fixture حفظ شدند. سیاست پیش‌انتشار با ۳۵ تست و Helm واقعی بررسی شد.
مدل محلی موجود، شمارندهٔ دقیقِ پیش از اجرا را ثابت نکند. اجرای باینری تازه
در بررسی خودکار رد شد؛ دلیل فقط `blocked by policy`. دور زدن انجام نشد.
تست درگاه و مخزن ادامه یابد؛ پذیرش پردازنده، کیفیت پاسخ و انتشار هنوز باز.
