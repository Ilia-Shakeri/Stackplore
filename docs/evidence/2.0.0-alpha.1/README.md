# Stage 1 — local gateway candidate

Status: candidate, **2026-10-21**. This checkpoint adds a provider-neutral bounded
gateway and a single operator-bound native local adapter. It exposes no public
dispatch route and adds no database migration. Stable 1.0.3 and published
alpha.0 retain their immutable sources and original proof.

The [real profile receipt](gateway-profile.json) binds all ten gateway/tool
source files, the exact engine/model/template pins, native hardware, safe case
IDs, exact token counts and durations. Raw questions, responses, local paths,
credentials and the private process ownership receipt are excluded.

| Actual check                  | Result                                                                          | Limit                                         |
| ----------------------------- | ------------------------------------------------------------------------------- | --------------------------------------------- |
| Native installed files        | Official pinned ZIP digest and all 51 installed files match                     | No general supply-chain certification         |
| Model bytes                   | 639,446,688 bytes, full SHA-256 and GGUF header verified                        | Protocol fixture, no quality approval         |
| Explicit startup              | Ready; exact build, alias, template, context 20,480 and one slot; loopback only | Trusted host; no local hostile-user isolation |
| English controlled request    | 104 input / 27 output tokens, 2,358 ms, pass                                    | Literal empty-evidence protocol probe         |
| Persian controlled request    | 132 input / 40 output tokens, 5,229 ms, pass                                    | Literal empty-evidence protocol probe         |
| One-use preparation           | Reusing the actual preparation rejected                                         | No durable admission claim                    |
| Gateway/adapter focused tests | Pass; statement coverage 95.4% / 90.2%                                          | Statement figures are not branch coverage     |
| Operator tool pure tests      | 125 pass                                                                        | No execution/network in the pure suite        |

The runtime was ready for the actual probes, then ownership-checked stop
temporarily released its memory for the isolated full gates. Restart with a
fresh private ownership receipt is required before final local handoff. Its model
and executable files live outside the repository. Operator startup and exact
ownership-bound stop procedures are documented in the
[English guide](../../v2/local-processor.md) and
[Persian guide](../../v2/local-processor.fa.md). A startup failure or rejected
attempt must retain its own outcome; no earlier failure is relabelled.

Review loop: **8 → fix → 9/10**, scoped to these gateway/tool boundaries. Fixes
closed duplicate/case-fold JSON keys, UTF-8/depth bounds, pinned context/slot
identity, opened-file size races, exact browser-origin policy and ownership-bound
receipt handling. Independent review and focused checks pass. The score does
not close the full exact-source, hosted or release gates.

## Retained failed gates

The first exact-source full gate failed. Its unchanged PostgreSQL performance
fixture timed out while seeding the 20,000-row workload; the package later hit
its ten-minute deadline during upgrade-test cleanup. Client/web tests and the
production build were not reached. The
[failed-attempt receipt](local-gate-failure-a.json) retains both deadlines and
the raw log digest. After stopping only the owned processor and pausing other
team compilation/database checks, the same focused test passed in **6.47 s**
with its original 90-second bound, workload, samples and assertions. Resource
contention is plausible; these changed host conditions do not establish a
single cause. No product or fixture code was changed. A new full gate must pass;
the earlier attempt remains failed. This does not establish simultaneous
processor/database capacity.

The [second failed gate](local-gate-failure-b.json) passed actual PostgreSQL
tests in 479.238 s and all 40 client tests. Web execution failed: 492/495 cases
passed, three cases exceeded existing lookup/test bounds, and one further file's
worker failed to start. The build was not reached. The same three unchanged
files then passed **80/80** focused cases in **59.09 s**. That probe did not enable
coverage and cannot replace the full gate. Existing deadlines and assertions
remain unchanged; a separate complete gate is required.

The publication check now requires exactly seven complete successful, uniquely
named jobs, including `processor-tool`; missing, extra, duplicate, failed or
skipped jobs cannot publish. A separate [focused run](release-gate.json) passed **48/48** release and
runtime-image checks with no skips, plus format and lint. Its receipt was
transcribed from tool output; raw stdout was not retained. The historical alpha.0
browser consumer still requires its original six-job set.

## Full local pass and remaining checkpoint gates

The [full C receipt](local-gate.json) records exit **0** on tree
`695c705a379ae46f3eb570009efab10654ae6f85`. All 45 changed staged files matched
the isolated export; migration 18 was absent. **544 web / 40 client / 48 release
/ 11 corpus tests pass**, with coverage floors, notices, locale/contract drift,
lint, types and production build. Web branch coverage is **80.33%**, duration
**227.98 s**. Go/PostgreSQL/native results include cache reuse; actual unchanged
SQL/native checks passed in gate B (479.238 s). C is not a new SQL execution.
Earlier full failures stay failed. The added receipt and document-only changes
are outside the tested tree and require the exact-commit hosted gate.

Retained stage-2/3 preparation and migration 18 stay outside this candidate.
Inspect the exact commit's seven hosted
jobs, including the Windows pure-tool job. A version tag and any published
artifacts require their own passing source-bound gates. No current artifact or
stable-release acceptance is implied here.

## فارسی

نامزد گام ۱: درگاه محدود و پردازندهٔ محلیِ اپراتور؛ مسیر عمومی ندارد و طرح
پایگاه همان ۱۷ بماند. سرویس واقعاً شروع شد؛ هویت، مدل، قالب، context برابر
۲۰۴۸۰ و یک slot سنجیده شد. دو درخواست کنترل‌شده پاس: انگلیسی ۱۰۴/۲۷ token
در ۲۳۵۸ms؛ فارسی ۱۳۲/۴۰ در ۵۲۲۹ms. استفادهٔ دوباره از درخواست رد شد.
۱۲۵ تست خالص ابزار و تست‌های کد پاس شدند. متن خام و مسیر خصوصی در رسید نیست.

نمرهٔ همین محدوده ۹/۱۰؛ کیفیت پاسخ، ظرفیت نهایی و کل نسخهٔ ۲ ثابت نشدند.
گیت کاملِ منبع پاک، PostgreSQL واقعی، هفت کار CI و پذیرش نسخه هنوز جدا لازم.
فایل‌های آماده‌سازی با طرح ۱۸ وارد این نامزد نشوند. سرویس برای تست‌های بعد
پیش از تحویل دوباره روشن شود؛ توقف فقط با رسید مالکیتِ همان فرایند باشد.

گیت کامل نخست شکست؛ کاشت ۲۰هزار ردیف به مهلت ۹۰s خورد و سپس سقف کل بسته
تمام شد. شکست حفظ شد. سرویس خودمان موقت متوقف و ساخت‌های موازی جدا شدند؛
همان تست بی‌تغییر در ۶٫۴۷s پاس شد. فشار میزبان محتمل؛ علت یگانه ثابت نیست.
گیت کامل تازه لازم؛ ظرفیت همزمانِ پردازنده و پایگاه هنوز ثابت نشد.

گیت دوم: پایگاه و ۴۰ تست کلاینت پاس؛ وب ۴۹۲ از ۴۹۵ پاس، سه شکست و یک خطای
شروع کارگر داشت. همان سه فایل بی‌تغییر، جدا ۸۰ از ۸۰ پاس شدند؛ جای گیت کامل
و پوشش را نگیرند. شکست دوم حفظ شد. قفل نشر هفت کار دقیق خواهد؛ تست جداگانهٔ
نشر و تصویر، ۴۸ از ۴۸ پاس، بی‌skip. این‌ها هنوز نشر واقعی نیستند.

گیت کامل C پاس: ۵۴۴ وب، ۴۰ کلاینت، ۴۸ نشر، ۱۱ corpus و ساخت محصول. منبع پاک
با طرح ۱۷ سنجیده شد. Go و پایگاه از کش پاس شدند؛ اجرای واقعیِ بی‌تغییر در
گیت B بود، نه اجرای تازه در C. رسید و سند پس از گیت، CI منبع نهایی خواهند.
پوشش شاخهٔ وب ۸۰٫۳۳٪؛ کیفیت پاسخ یا کل نسخهٔ ۲ ثابت نشد.

## Go security correction — 2026-10-21

Previous source `f1ae80391aca646fca89fed7a5ecf79271999c97` passed all seven
hosted jobs and **14/14** browser cases, with no case retry;
[previous-source receipt](previous-source-hosted.json). Its signed
[trial 39592142674](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39592142674)
then failed the database-image vulnerability gate. The
[failed scan receipt](release-trial-failure.json) records **CVE-2026-111943**,
HIGH, in the Go 1.26.9 standard library embedded in `gosu`. Component presence
is proven; exploit reachability through this helper was not demonstrated.
Publication was skipped and no alpha.1 tag was created. No finding was suppressed.

The correction pins Go **1.26.10** in the module, bootstrap, CI, application and
database-helper builds, release metadata and policy fixtures. Both attempted
official Windows binary downloads returned **404**. A Windows SDK was instead
built from the verified official tagged source in **334.359 s**, exit **0**;
[toolchain receipt](toolchain-source.json). Full source integrity and actual
`go version` passed; official binary-archive acceptance remains false. Historical
Go 1.26.9 receipts retain their original compiler and scope.

The new source still needs its own full local gate, exact-commit seven-job CI,
strict signed trial, independent artifact verification and checkpoint publication.
Earlier passes do not transfer to the correction. Stage 1 remains a candidate.

نشر نامزد در اسکن شکست؛ کتابخانهٔ Go 1.26.9 در `gosu` ایراد HIGH داشت.
تگ ساخته نشد؛ خطا پنهان نشد. منبع پیشین هفت کار CI و ۱۴ تست مرورگر پاس داشت؛
این پاس برای اصلاح تازه نیست. Go 1.26.10 از منبع رسمیِ سنجیده روی ویندوز
ساخته شد؛ دانلود ZIP رسمی ۴۰۴ داد. گیت کامل، اسکن و نشرِ منبع تازه هنوز لازم.
