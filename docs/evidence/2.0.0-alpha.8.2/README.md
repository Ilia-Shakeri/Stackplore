# Alpha8.2 — repeated service lifecycle preparation

Candidate **2.0.0-alpha.8.2**, schema22. Hosted receipts are required before a tag
or publication claim. No workstation test, build, scanner or service is run.

First source `dae2adf` passes [800/800 lifecycle observations](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42779994761)
with race detection. [Safe receipt](first-lifecycle.json). Its
[source gate](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42779994666)
finds formatting of the two new scripts before heavy code tests. The exact hosted
formatting patch is applied; a fresh complete source gate remains mandatory.

The separate `stage9-lifecycle` workflow repeats eight existing service-boundary
regressions one hundred times each, with the Go race detector, two parallel tests,
a recorded deterministic shuffle seed and a ten-minute test deadline. Each named
case must have exactly one hundred starts and successful terminal events. Missing,
duplicate, failed, skipped or incomplete observations reject. Package-level race
failure rejects even if individual cases claim success. Reports contain fixed
names and counters; test output, questions, answers and identifiers are omitted.

The eight cases cover shutdown joining an active producer, concurrent close and
cancel, a late admission commit, explicit cancellation, pending-admission cleanup,
ownership loss, memory expiry and revoked-user reads. Dependencies are deterministic
service fixtures. **800 planned observations are not 800 real installations or
processor calls.** This does not establish browser reconnect, real process restart,
database recovery, a sixty-minute soak or customer/pilot acceptance.

The complete existing eight-job source gate and signed package/security checks
remain separate and unchanged. The new workflow supplements rather than substitutes
for real processor, desktop, provider, deployment and PostgreSQL checks. No product
runtime, permission, deadline, schema or processing profile changes in this increment.

## Correction loop

Initial review: **8/10 within lifecycle receipt collection**. Repeated runs need
exact denominators, nested-failure handling and output privacy before the checker
can serve as evidence. These requirements are implemented and tested remotely;
a fresh hosted result is required before the bounded score can reach 9/10.
This score cannot certify the whole infrastructure, semantic quality or stage8 exit.

## Roadmap position

[Published alpha8.1](../2.0.0-alpha.8.1/README.md) has 72/72 structural development
observations and nineteen named fault regressions. Independent reviewed corpus,
sealed holdout, human grounding/correct-abstention judgment, same-hardware helper-load
comparison and risk acceptance remain open. [Stage9 preparation](../../v2/stage9-preparation.md)
can proceed without claiming its entry gate, a pilot or stable2 acceptance.

## فارسی

این نسخه تستِ تکراری مرزهای چرخهٔ عمر را آماده می‌کند: هشت تست، هر یک صد بار،
با کشف تداخل و شمارش کامل. اجرای واقعی فقط روی GitHub Actions است. این تست‌ها
جای تست پردازشگر واقعی، اتصال دوبارهٔ مرورگر، راه‌اندازی دوبارهٔ فرایند، یک ساعت
دوام یا پایلوت را نمی‌گیرند. داوری مستقل و شرط‌های پذیرش نسخهٔ پایدار هنوز بازند.
