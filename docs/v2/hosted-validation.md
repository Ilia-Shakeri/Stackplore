# Smart Assistant — hosted validation

Effective **2026-11-01**. Work in `D:\Projects\Stackplore\repository`.
Local work is file reads, edits and lightweight Git only. **Do not start local
services, tests, builds, installs, scanners or processors.** All new validation
runs on Actions. Earlier local receipts remain historical.

## Real processor gate

The new `processor-runtime` job uses a disposable `windows-2025` runner.
[`check-hosted-processor.ps1`](../../scripts/check-hosted-processor.ps1) refuses
workstation execution before filesystem or process work. It requires a hosted
Actions context, fixed archive/model hashes and size bounds, and uses the
existing owner-checked launcher. No public listener or profile default changes.
Inputs remain the exact `b11729` CPU archive and Qwen3-0.6B Q8 revision recorded
in the [operator guide](local-processor.md).

Downloads have HTTPS-only redirects, finite attempts and time/size limits.
The launcher checks all 51 files, model digest, build, template, context and
one-slot identity. `TestPinnedLocalRuntimeIntegration` must execute both
languages: exact empty-evidence semantics, counts, closed output and single-use
preparations. Skipped/missing cases, nonzero exit, identity mismatch or unproven
owned shutdown fail the job. No deadline or assertion is relaxed.

Only `.tmp/hosted-processor.json` is uploaded: source revision, public pins,
bounded case results and shutdown status. Private receipts, local paths, raw
replies and model files stay on the disposable runner. Repository permission
is read-only; no paid profile, application secret or production source is used.

The first [real protocol receipt](../evidence/2.0.0-alpha.4/hosted-protocol.json)
binds source `5cebad7bee494b9e5011b6b86bbd0a9fb9b2ba84` and
[run 41279900515](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41279900515).
English/Persian checks pass in 1.46/2.34 seconds, with owned shutdown proven.
These are test timings, not workload or capacity measurements. The receipt
contains protocol evidence only; it does not accept later source.
All eight jobs for that checkpoint pass; the
[hosted receipt](../evidence/2.0.0-alpha.4/hosted-protocol-ci.json) records them.

The next increment extends this same hosted job with native PostgreSQL 17,
the production application and real desktop browser. The database uses a fresh
runner-temporary directory, a fixed free loopback port, and PID/start-time/path
ownership checks. It never touches the installed database service. Cleanup
stops only that owned cluster. Missing prerequisites or failed cleanup fail the
job; no local fallback is permitted.

The normal English/Persian administrator/viewer journeys now have an explicit
hosted-only real mode. No private route is intercepted. A fixed profile module
forwards only approved pins, rejects guest/partial-locale/workstation modes,
and ignores ambient endpoint overrides. Each actual reply must contain exactly
the selected evidence's resource name, one current citation and no extra claim.
Closing/reopening must erase text without resubmission. Raw replies are neither
screenshotted nor uploaded; only aggregate case metadata is retained. This
candidate awaits its own exact-source run and is not yet accepted.

Source `ec90463` failed its first full gate on test-file formatting. Its real
protocol still passed, and PostgreSQL **17.11** and the processor both stopped
with ownership proven, but both real application browser cases failed.
[The failed receipt](../evidence/2.0.0-alpha.4/hosted-journey-failed.json) stays
failed. New source retains bounded state/count-only diagnostics, with no
question, answer, owner ID or raw provider payload. A failed formatting gate
can publish a source patch from Actions; applying that patch requires a new
commit and complete revalidation. It cannot make the failed revision pass.

Even passing these bounded Demo cases is not human quality, a held-out incident
evaluation, soak/capacity or whole-version acceptance. Failed attempts remain
recorded. Desktop themes/accessibility remain covered by the separate complete
synthetic-protocol journey.

## Desktop and delivery

The complete resource journey uses **1536 × 960**: both languages, roles and
themes, keyboard, evidence focus, accessibility and history cleanup. One brief
**390 × 844** input/overflow smoke check keeps basic mobile usability. New
screenshots are labelled `desktop`; older mobile receipts remain immutable.

Source `80c7ebe424ef2c80f23458a3f03be5b619f8eb81` passed seven jobs in
[run 41192895511](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41192895511).
New source requires eight jobs. The [ledger](implementation.md) and
[roadmap](../roadmap-v2.md) own remaining profile, grounded journey, quality and
release gates. Alpha.4 waits for its exit criteria. Stages 5–10 remain open.

## فارسی

از ۲۰۲۶-۱۱-۰۱، روی سیستم کاربر فقط خواندن، ویرایش و Git سبک مجاز است.
سرویس، تست، ساخت، نصب و اسکن محلی اجرا نشود؛ چک‌ها در Actions انجام شوند.
رسیدهای محلی پیشین فقط تاریخچه‌اند.

میزبان موقت، فایل و هش دقیق را بسنجد و همان راه‌انداز موجود را به کار برد.
هر دو زبان واقعاً تست شوند؛ مورد ردشده یا اجرا‌نشده پاس حساب نشود. فقط رسید
امنِ نسخهٔ منبع، هش عمومی، نتیجه و توقف مالکیت‌دار بارگذاری شود؛ مسیر خصوصی،
پاسخ خام و مدل بارگذاری نشوند. دسکتاپ اول باشد؛ موبایل چک کوتاه داشته باشد.
این تست، کیفیت پاسخ یا پایان گام ۴ را ثابت نمی‌کند.
