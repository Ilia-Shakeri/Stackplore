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

## Private status server increment

New source adds the [private status server](../../v2/status-server.md), with
metadata-only SQL access, bounded replay/reset, fresh session authority, owner
slots and deterministic transport cleanup. Actual TCP expiry exposed truncated
HTTP termination; its failing receipt is retained and the bounded completion
fix is tested. A separate regression catches valid maximum-length owners being
rejected by the internal namespace; the full identity limit remains unchanged.
SQL/public-admission checks cover snapshots, retained replay, future cursors,
an owned missing sequence and revocation without provider/processor work.
Authentication and processor are controlled fixtures. Complete source/hosted
gates remain separate; the bilingual owning view and full-stage acceptance
remain open. No alpha.4 milestone tag is claimed.

The 2026-10-31 [dependency refresh](../../v2/dependency-refresh-20261031.md) found
new high/moderate advisories in the earlier 393-version graph. Final pins are
serializer 1.6.11 and matcher 11.0.1, with 394 locked versions and regenerated
production legal metadata. The first status-server snapshot passed functional
checks but contains the superseded dependency graph; it does not accept these
new bytes. Complete final snapshot and fresh scan evidence are recorded separately.

The final [status source receipt](status-server.json) binds clean tree
`7a6ba67335850bc2c55357177ae6a0f4758f5fc8` and the refreshed server/browser graph.
Complete verification passes: **213 client and 544 web tests**, actual PostgreSQL
**288.040 s**, native database operations **13.425 s**, strict checks and builds.
The final 394-version all-severity package audit is clear. Canonical source
HIGH/CRITICAL scanning passes; the separate all-severity report retains
GO-2026-5932 without suppression. Fresh five-platform, three-command import
inventories contain none of its affected OpenPGP package family. The fresh
database came from the documented official public ECR repository.

Path-aware staged and 53-commit history secret checks pass. A failed pathless
diff probe remains recorded: an unchanged public oauth2 checksum in context
was classified as a key. No new ignore is added. The failed B notice gate and
earlier advisory reports remain failed and do not certify final bytes.
The preceding application commit has its [seven-job hosted receipt](application-hosted.json).
Later receipt/document edits receive their own document/format checks;
the exact final commit still requires hosted checks. No bilingual owning view,
current processor, full-stage acceptance or alpha.4 milestone tag is claimed.

گیت کاملِ منبع تازه، ۲۱۳ تست کارخواه و ۵۴۴ تست وب، پایگاه واقعی و ساخت را
پاس کرد. اسکن شدید و بحرانی صفر؛ هشدارِ بستهٔ واردنشده در گزارش کامل بماند.
روش اصلیِ اسکن محرمانه بی‌یافته است؛ هشدارِ checksum در متن diff حفظ شد.
صفحهٔ دوزبانه، موتور واقعی، تگ و پایان مرحله هنوز جدا لازم‌اند.

## Browser ownership increment

The [browser ownership boundary](../../v2/browser-ownership.md) adds an opaque
volatile session identity and explicitly owned private transport. Protected
mutations retain their proof inside the session runner; same-proof authority
changes fence late dispatch/results. REST calls have 15-second lifetimes and
status has a 120-second outer bound. Owner loss, caller cancellation and
timeout settle locally even when transport ignores abort. Cleanup and closed
reply/status validation remain mandatory; no view, storage, automatic read,
reconnect or resend is added.

Focused checks pass **47 tests** with **96.15% branch coverage** on the private
port; type/lint checks pass. Earlier fixture, generic-mock typing, lint,
synchronous-stop rejection and insufficient-coverage failures remain retained.
A complete source/hosted gate is separate. The owning bilingual journey, real
processor, full stage and alpha.4 tag remain open.

نشان مالک در حافظه ساخته شد؛ تغییر نشست و مجوز، پاسخِ دیررس را رد کند.
۴۷ تست و سنجش سخت شاخه پاس؛ صفحه هنوز وصل نیست. شاهد گیت کامل و CI جدا لازم.

The complete [ownership source receipt](browser-ownership.json) binds tree
`b5391a2aacadf8d622c0e74ecad55b2fc532285b`: **213 client and 576 web tests**, real SQL/native
results with cache markers preserved, strict checks and production builds pass.
Fresh canonical dependency scanning, the 394-version all-severity package audit
and staged/history secret checks pass within their recorded scopes. The known
package-specific no-fix finding remains visible in the separate all-severity
source report. The prior status commit has its [seven-job hosted receipt](status-server-hosted.json).
Later receipt/doc edits receive separate document/format checks; the exact
ownership commit still requires hosted checks. Controller and view source
are not included in this ownership checkpoint.

## Owning resource view source

Newer source mounts the [bilingual selected-resource conversation](../../v2/resource-conversation.md).
The volatile controller binds source/resource/window/language and session owner,
limits eight submitted turns and thirty minutes from first input, freezes
explicit retries and rejects stale/late replies. Plain-text rendering and fresh
citation authority are independent. Cancellation distinguishes acknowledged
cancel, already-ended work and unknown outcome. Closing/page exit, owner or
scope change and expiry clear the view. There is no automatic question resend.

Focused controller/view checks pass **51 cases**; earlier controller branch
measurement passes above 90% in its declared file scope. Types, lint and locale
parity have separate source checks. Initial strict-type/lint/coverage failures
remain retained. A production browser fixture uses actual normal authentication
and exploration but intercepts private replies synthetically; it starts no
processor. Complete clean-source, browser and hosted gates are still required.
The prior ownership checkpoint does not certify these later view/controller
bytes. No alpha.4 tag or full-stage acceptance is claimed.

نمای منبع به دو زبان وصل شد؛ متن در حافظه و محدود است. تغییر نشست و محدوده،
انقضا و بستن صفحه نما را پاک کنند. تست نمونه، شاهد موتور واقعی یا پایان
نسخه نیست؛ گیت کامل، مرورگر و CI منبع تازه جدا لازم‌اند.

The first browser probe failed because its synthetic request guard used a
noncanonical CSRF header name. The second passed English but failed Persian
on immediate history return after its role/theme journeys. Neither complete
probe is accepted. Keep both failures and retain immediate as well as settled
back-navigation in the next gate. Diagnostic output contains only fixed path
classes, document counts and bounded HTTP statuses, never private text.

Three red tests exposed retained text after explicit status transport denial;
401/403/404 now erase through the same owner fence as structured denial.
The corrected focused gate passes **51 cases**, **186/197 branches (94.41%)**
and **30/30 functions** in the controller file. This does not measure every
new runtime package or replace real browser/engine/whole-stage acceptance.
Resource citations also use centered nonanimated focus to avoid the sticky
header; UTC range endpoints have separate labels for RTL readability.

The third browser probe retained both history modes and recorded only safe
metadata on failure. It identified HTTP **429**, not a private reply or render
schema error: interception had disabled Chromium HTTP caching, so repeated
full-document loads exhausted the unchanged peer bucket. The pinned installed
browser implementation confirms cache disabling follows interception. The
corrected fixture restores normal asset caching after the four closed private
requests, keeps observing all private requests, and uses the existing 2.5-second
minimum refill cadence between history rounds. Immediate back within its round
remains tested; there is no question resend or limiter configuration change.
The third complete probe remains failed.

The locale gate now also covers the previously omitted assistant, authentication,
user and operation catalogs: **946 messages**, matching keys, duplicate checks
and interpolation placeholders. This expands the checker; old 738-message
receipts retain their original scope. Current focused view/controller checks
pass **51 cases**, **186/197 controller branches (94.41%)** and all 30 functions.

The cadence-corrected D snapshot passes both complete normal browser cases
without retries or skips: four language/role journeys, both themes, keyboard,
focus occlusion, accessibility, immediate and settled history return, eight
viewport images, and exactly four synthetic private requests per journey.
Its complete local gate passes **213 client and 627 web tests**, actual
PostgreSQL **154.322 s**, native operations **11.876 s**, strict checks and builds.
That source predates the final visual correction and does not certify its bytes.

Viewport review also found the existing retained-health table lacked the
spacing applied only to chart tables. The shared table rule now covers its
existing focusable scroll wrapper as well; data and behavior are unchanged.
A browser check requires readable inline cell spacing in both languages.
The E clean snapshot receives fresh browser and complete source gates.

Further review reproduced one view-level citation race: after changing the
evidence window, a late citation result correctly lost content authority in the
controller but still added an unavailable warning to the new view. The retained
red test fails exactly that assertion. The view now binds completion to its
lifetime, snapshot and latest check, and binds warnings to their originating
snapshot. Three added cases cover the late-window reply, warning cleanup on a
locale change, and superseded checks. All eleven panel cases pass. This newer
source needs its own clean snapshot; E evidence cannot accept the correction.

The final [resource conversation receipt](resource-conversation.json) binds
tree `e748464a632dd9a49c7913cbc481848ba3ca1933` and all seventeen runtime/test/style/checker
files. Complete local verification passes **213 client and 630 web tests**,
strict checks, actual SQL/native operations and production builds. Raw Go result
lines retain their cache markers. Both complete normal browser cases pass
without retries/skips: four role/language journeys, eight mobile viewport
images, both themes, keyboard, accessibility, current focus, spacing and both
history-return modes. Conversation replies are intercepted synthetic data;
normal authentication and exploration use the production service and actual
disposable PostgreSQL. No current processor is started by these checks.

Final focused checks pass **54 cases**, **187/197 controller branches (94.92%)**
and all thirty controller functions. The citation warning retains an opaque
marker, never a copy of a private snapshot. Fresh source policy scanning and
SBOM generation pass; the all-severity report still contains the documented
no-fix deprecated-package finding. The unchanged lock graph has a fresh passing
394-version package audit after its retained network failure. The source receipt
also retains earlier browser failures and the owned-fixture recovery boundary.
The fixture launcher now waits for its own process only; final verification
starts and cleanly stops only the owned PostgreSQL instance. The later H probe
failed two TypeScript checks. Its old wrapper also shadowed the native exit
status; its zero lifecycle marker is explicitly invalid and is not a passing
gate. Both failed gate receipts remain recorded. The final source corrects
the types and the wrapper checks both child status and actual gate receipts.

The prior browser-ownership commit has its [seven-job hosted receipt](browser-ownership-hosted.json).
Final document/receipt edits receive separate link, format and index-secret
checks. Exact new-commit hosted verification remains a separate gate. Current
processor, remaining profile behavior, human grounding, accepted global branch
measurement, soak/capacity and pilot remain open. No alpha.4 milestone tag or
whole-stage acceptance is asserted.

The first complete check of this final source failed: one unchanged Persian
103-account flow exceeded its 15-second limit, and another unchanged test file
could not start a worker. That run remains failed. Both affected files then
passed **62 cases** unchanged; the subsequent complete `verify` run passed
all **213 client and 630 web tests**, checks and builds on the same source.
No test limit, isolation, concurrency setting or assertion was changed. Free
physical memory was observed at 1.37 GiB near the failure and 3.17 GiB before
the retry; that correlation does not establish a root cause. The receipt keeps
the original failure, focused diagnostic, actual earlier Go/SQL/native results
and the passing retry's own cache markers separately.
