# Smart Assistant — stage 4 development checkpoint

Date: **2026-10-28**. Development version **2.0.0-alpha.4**, schema18.

The foundation prepares a snapshot-only, default-off loopback profile parser and
closed conversation contracts/types. The private request constructor preserves
same-origin protections, exact idempotency keys, bounded valid UTF-8, explicit
cancellation and no automatic retry. It rejects route overrides and unknown
body fields. Closed reply validation is now part of this client boundary;
the owning application must still bind its retained scope and render inert text.

Focused profile checks produced 71 passing test/subtest events; the complete
configuration package tests and vet passed. Contract drift checks, 52 client
tests, strict types and targeted lint passed on preparation source. Initial
client fixture/abort checks and strict-type/lint failures remain retained.
The complete clean development-snapshot gate then passed: real PostgreSQL and
repository tests, 52 client tests, 544 web tests, notices/contracts/locales,
format/lint/types, release/corpus tooling and production builds. Fresh source
scans cover the Go module graph and all locked packages with no findings; the
standard package-manager audit also reports zero findings across 393 versions.
[Source-bound local receipt](local-gate.json). Exact-source hosted checks for
the new development commit remain separate. The original foundation commit
`cf3dbc52a771bd47c727a7548a39452a178b93dc` subsequently passed all seven
[hosted jobs](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/40657271098).
That run does not certify later response-boundary changes.

## Closed reply increment

The [reply boundary](../../v2/conversation-replies.md) rejects malformed state,
extra fields, executable object properties, ungrounded/forged citations,
cross-resource/time-range answers, unsafe text and independent text/JSON byte
overflows. Run/citation identities and admission scope cannot silently change.
Returned data is detached from the decoded source. Failures expose a fixed code
without private text and never cause an automatic resend. Full-source checks
for this increment pass on clean tree `66b580f57af397186203712bc276639bb4eeb1d9`:
**118 client and 544 web tests**, real PostgreSQL **151.689 s**, native database
operations **13.424 s**, strict types/lint, contracts/notices/locales, release
and corpus tooling, and production builds. Fresh dependency scans report no
HIGH/CRITICAL findings; the standard package audit reports zero findings at all
severities across **393 locked versions**. The staged secret scan reports zero
findings. The [separate increment receipt](reply-boundary.json) binds the six
tested code files and the actual gate/report hashes. Earlier failed tool/type
attempts and the different intermediate snapshot are retained; only the final
snapshot accepts these code bytes. Commit `da79cb5581457ca73e93f9b0513fbe86fdd45177`
then passed all seven hosted jobs; all 49 canonical commits passed the existing
whole-history secret policy. [Exact reply-increment hosted receipt](reply-hosted.json).
This accepts the reply increment only; later decoder code has different bytes.

No startup profile registration, enabled application dispatch, public admission,
live status stream or bilingual conversation is delivered here. Conditional
library read/cancel/citation handlers have their own later increment below.
Those owning paths, real database/browser/privacy proofs, accepted branch
measurement and current processor/human quality gates remain open. There is no
alpha.4 milestone tag or full-stage acceptance claim.

## Status decoder increment

The [pure status decoder](../../v2/status-decoder.md) checks bounded UTF-8 frames,
closed metadata, same-run identity, sequence/version order, reset fencing,
duplicate fields/escaped JSON aliases and deterministic cancellation cleanup.
It supports every two-chunk split of replay frames, bytewise multibyte comments,
LF/CRLF/CR, exact frame limits, all 64 statuses and late/truncated data rejection.
Focused client tests now pass **186/186**, with strict types and targeted lint.
Clean-snapshot format/docs/version/contracts, full frontend lint, all workspace
types and production web build also pass. Fresh source dependency scans and
the standard package audit report no findings. The [source-bound decoder
receipt](status-decoder.json) distinguishes these focused checks from a full
local gate and retains the corrected tool-path/lint attempts.
It opens no connection and adds no public endpoint or browser journey. The
earlier 118-test full gate binds its own immutable reply increment, not this
later source. The decoder commit `f2dd57ad5d94a87823d33bfac030d92725dcccc6`
subsequently passed all seven hosted jobs; its 50-commit canonical history scan
found no secrets under the existing policy. [Decoder hosted receipt](status-hosted.json).
This does not certify the later HTTP boundary.

## Private HTTP read increment

The [conditional transport](../../v2/private-run-http.md) delegates read, owner
cancellation and citation resolution to the existing authority-checked service.
It is not supplied by the server entry point and cannot be composed with guest
mode. It projects only safe metadata and complete validated content; ordinary
JSON/format/log paths cannot disclose its private answer. The shared synthetic
publication fixture checks both server output and client validation.

Focused HTTP/inference tests pass; **187 client tests** and strict client types
and lint pass. A fresh real PostgreSQL run passes the original synthetic and
production HTTP-adapter cases plus the new in-process HTTP projection case:
replay, citation, cancellation, account-revision revocation, one dispatch and
private-content absence from SQL/logs. Authentication and processor output in
that combined fixture remain synthetic. A new malformed-Unicode regression was
reproduced before fixing silent decoder repair. Exact-source full and hosted
gates for this increment are source-bound in the [HTTP increment receipt](http-boundary.json).
The complete local gate passes on tree `d2a7e2efe3179eb01964c2bcfd956e44a3b073c5`:
real PostgreSQL **125.477 s**, **187 client**, **544 web**, contracts/notices/locales,
strict format/lint/types, release/corpus tooling and production builds. Native
database operations are cached in the final gate; they actually passed in
**17.220 s** on this same code during the earlier failed outer gate.

Two earlier commands reached the unchanged **10-minute** package timeout while
the SQL suite continued to advance. Both remain failed. A three-case comparison
on old/current sources was diagnostic only; it is not a p95 acceptance benchmark.
The unchanged source subsequently passed the complete gate with the same timeout,
assertions and thresholds. No skip, timeout increase or scanner exception was added.
Fresh source/package and staged-secret scans pass. Final documentation-only edits
and exact final-commit hosted verification remain separate.

[Operator profile preparation](../../v2/operator-profile.md) describes the exact
proposed variables and why current startup still rejects them. The alpha.3
[evidence](../2.0.0-alpha.3/README.md) keeps its own immutable source and artifacts.

## فارسی

پایهٔ توسعهٔ گام۴: parser خاموش، مقصد loopback بسته، قرارداد و نوع‌ها، درخواست
محافظت‌شده با کلید ثابت در retry، حد UTF-8 و لغو پیش از ارسال. تست محدود، تست
کامل config، vet، ۵۲ تست client، نوع و lint پاس. گیت کامل snapshot پاک نیز با
پایگاه واقعی، ۵۴۴ تست وب و ساخت پاس شد. اسکن منبع و audit همهٔ ۳۹۳ نسخه پاک.
CI هفت‌کارِ کامیت پایهٔ `cf3dbc5` پاس شد؛ تغییر بعدی شاهد جدا لازم دارد.

مرز تازهٔ پاسخ، متن بد، ارجاع جعلی، منبع یا بازهٔ ناهماهنگ، زمان غلط و فیلد
اضافی را رد کند. پاسخ کپی جدا دارد؛ خطا متن خصوصی ندهد و ارسال دوباره نسازد.
شاهد این بخش با گیت قدیمی یکی نیست. گیت کامل کد آخر با پایگاه واقعی، ۱۱۸ تست
کارخواه، ۵۴۴ تست وب و ساخت پاس شد. اسکن منبع و audit همهٔ ۳۹۳ نسخه بی‌یافته؛
اسکن فیلدهای محرمانهٔ تغییرات ثبت‌شده نیز بی‌یافته. CI کامیت آخر هنوز جداست.

خوانندهٔ جداگانهٔ وضعیت نیز با ۱۸۶ تست کارخواه، نوع و lint پاس شد: حد بایت،
UTF-8، ترتیب شماره، reset و پاک‌کردن قاب در لغو. اتصال شبکه یا مسیر سرور
نسازد. شاهد گیت کامل بخش قبلی، اثبات این تغییر تازه نیست؛ CI منبع تازه جداست.

مرز کتابخانه‌ای خواندن، لغو و ارجاع ساخته شد؛ در ورودی اصلی وصل نیست. نمونهٔ
قالب مشترک و ۱۸۷ تست کارخواه پاس؛ مسیر ترکیبی با پایگاه واقعی نیز پاس شد.
احراز هویت و پردازندهٔ آن تست آزمایشی‌اند. گیت منبع تازه جدا لازم است.
اتصال شروع برنامه، ساخت عمومی اجرا، stream و صفحهٔ دوزبانه هنوز مانده‌اند.
تگ گام۴ و پذیرش کامل ادعا نشود.

## Explicit admission increment

The [conversation admission boundary](../../v2/conversation-admission.md) adds
conditional POST handling, closed bounded requests, trusted server identities,
frozen retries, missing-row fencing and atomic volatile continuation guards.
Actual SQL/HTTP tests exercise one reserved dispatch and settlement, replay,
changed-request conflicts, cancellation and session-revision revocation.
Authentication, resource selection and the loopback processor remain synthetic.
The settings read port changes neither control version nor audit/usage state.
Application startup, live status and the bilingual journey remain open. No
alpha.4 milestone tag or whole-stage acceptance is claimed. Source-bound checks
for this increment are recorded separately from earlier receipts.

The [admission source receipt](conversation-admission.json) binds 15 code files
and clean tree `36fbfb7e857f3c33afc70b1d1fe8f22357fcb9fd`. Full verification passes: 187 client tests,
544 web tests, actual PostgreSQL 146.706 s, native database operations 13.186 s,
contracts/notices/locales, format/lint/types, release/corpus tooling and builds.
Fresh source scans and the standard 393-version package audit report zero
findings within their severity scopes. The initial fixture compile failure is
retained. The preceding HTTP increment has its [exact hosted receipt](http-hosted.json):
seven successful jobs and 51 canonical commits with no secret findings.

## Application ownership and explicit connection increment

New development source conditionally wires the [application runtime](../../v2/application-runtime.md)
and adds a [single status connection](../../v2/status-connection.md). It preserves
retained stops, quota/restore boundaries and evidence-reader seals while joining
workers before storage close. Actual SQL startup/restart and listener-failure
checks, controlled transport checks and complete source gates are separate from
the earlier admission receipt. Server subscription, bilingual UI and whole-stage
acceptance remain open. No alpha.4 milestone tag is claimed.

The [application source receipt](application-runtime.json) binds 16 code files
and clean tree `348ddd79896ca2ab0bfbefcd45df2d4671cd8a72`. Full verification passes: 212 client tests,
544 web tests, actual PostgreSQL 150.548 s, native operations 13.085 s,
actual server package 6.876 s, strict checks/tooling and production builds.
Fresh dependency scans, the standard 393-version audit and staged secret scan
report zero findings within their scopes. Failed fixture assumptions remain
retained, corrected against unchanged migration constraints. The preceding
admission commit has its [hosted receipt](admission-hosted.json): all seven jobs
and 52 canonical commits with zero secret findings. These receipts do not
certify the later server-status implementation.
