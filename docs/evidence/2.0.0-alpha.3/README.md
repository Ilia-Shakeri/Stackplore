# Stage 3 — shared privacy and evidence checkpoint

Updated: **2026-10-28**. Candidate **2.0.0-alpha.3**, schema18.

The candidate extracts shared effective log policy, bounded closed projections,
current-authority evidence reads and an in-process volatile local explanation
service. No public content route or operator profile composition is enabled.
Later contract and configuration preparation is excluded from this checkpoint.

Actual PostgreSQL focused checks pass for admission outcomes, deferred commit
rollback, current authority and one grounded local dispatch. The latter runs
with both a synthetic backend and the production HTTP adapter against a bounded
synthetic protocol server. Every content endpoint observes the committed SQL
reservation. Wire inspection checks closed keys, exact tokens, disabled prompt
caching/streaming and absence of display names, native identities and extension
fields. All owned SQL tables are checked for absence of question, answer and
source display text. This is not a real model or deployed telemetry proof.

The shared HTTP log-reader tests pass separately, including unchanged observation
behavior. Earlier compile/fixture failures and incomplete commands remain failed.
The full local gate passed on 2026-10-26 against the isolated staged source tree:
format, documentation, version/runtime pins, notices, locales, contract drift,
corpus/brand validation, lint, types, tooling tests, all Go tests with actual
PostgreSQL, 40 client tests, 544 web tests and production builds. Initial Compose
version drift was fixed; a cold module-graph timeout passed after warming the
same pinned offline cache. Failed attempts remain retained. Exact-source hosted
concurrency/browser gates and signed packaging remain required.

The package-manager audit failed with a network fetch/timeout error. A separate
native-fetch query to the official bulk advisory endpoint on 2026-10-26 checked
all 393 locked versions, including development and optional platform packages,
and returned no matching advisories. This is not a successful package-manager
audit. Publication reports from alpha.2 do not pass a new scan. No exception or
suppression is added. A fresh Trivy database downloaded on 2026-10-28 and the
canonical source dependency scan passed for both the Go module graph and locked
packages, including development dependencies, with zero reported findings.
The source SPDX document was produced. Binary/container scans remain separate.

The staged-diff secret scan passed with zero findings. A directory-only scan
failed on 16 pre-existing synthetic redaction/provider test literals in two
files verified byte-identical to alpha.2; no ignore was added or widened. The
canonical whole-history scan on the committed source remains required.

[Local gate](local-gate.json), [independent locked advisory query](locked-advisory-query.json)
and [fresh source scan](source-scan.json) record exact scope and retained failures.

The accepted >=90% branch measurement remains open; Go statement coverage is
separate. Human grounded-quality review, the bilingual public conversation,
current local engine startup, soak/load and later roadmap gates remain open.

## فارسی

نامزد گام۳، طرح۱۸: سیاست مشترک لاگ، شاهد محدود و بسته، دسترسی تازه و متن موقت.
مسیر عمومی هنوز فعال نیست. تست واقعی پایگاه، بازخوانی، سهمیه و ارسال مصنوعی
از مسیر واقعی HTTP پاس؛ متن خام در جدول‌های تست پیدا نشد. موتور واقعی، حریم
استقرار و کیفیت پاسخ با این تست ثابت نشوند. تست HTTP هم جدا پاس شد.

گیت کامل محلی در ۲۰۲۶-۱۰-۲۶ پاس: پایگاه واقعی، ۴۰ تست client، ۵۴۴ تست وب و ساخت.
CI و بستهٔ امضاشده هنوز لازم. ابزار audit خطای شبکه داد؛ پرس‌وجوی مستقیم رسمی
برای همهٔ ۳۹۳ نسخهٔ قفل‌شده، شامل توسعه و بسته‌های اختیاری، یافته نداشت.
این نتیجه جای پاس ابزار audit حساب نشود. اسکن منبع در ۲۰۲۶-۱۰-۲۸ با پایگاه تازه
برای Go و بسته‌ها پاس؛ یافته صفر. اسکن رازِ تغییرات پاک؛ اسکن پوشه ۱۶ رشتهٔ
مصنوعی قدیمی در دو فایل دست‌نخورده دید و رد شد. اسکن تاریخچه، بستهٔ نهایی، شرط
پوشش شاخه و گام‌های بعد هنوز بازند.
