# Stage 3 — shared privacy and evidence checkpoint

Updated: **2026-10-28**. Published source checkpoint **2.0.0-alpha.3**, schema18,
immutable source `21475ec1135c0496c5929329738142f3c0d5a9d0`.

All seven [main jobs](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/40648487660)
and the [signed branch trial](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/40648550256)
pass. Independent checks verify 39 trial files, 30 checksums, 31 provenance
subjects, seven signed SBOMs, eight strict scan reports with no high/critical
findings, and the extracted Windows archive's actual restart/replay/native
backup/restore/session-revocation/ownership-loss behavior. Trial signatures bind
`refs/heads/main`; they do not accept public tag assets. Tag verification
`40650376731` and publication `40650376645` subsequently passed. Independent
public tag checks and actual Windows execution also passed; see
[public acceptance](published-release.json).
[Exact-source trial and audit receipt](release-gates.json).

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
concurrency/browser/deployment checks and signed trial packaging subsequently
passed. Independent public tag consumption subsequently passed within the recorded scope.

The initial package-manager audit failed with a network fetch/timeout error. A separate
native-fetch query to the official bulk advisory endpoint on 2026-10-26 checked
all 393 locked versions, including development and optional platform packages,
and returned no matching advisories. This is not a successful package-manager
audit. On 2026-10-28 the standard package-manager audit itself passed for all 393
locked versions after explicitly binding the outgoing client to IPv4, with zero
findings at every severity. TLS verification stayed enabled; no ignore list or
project/global network setting was changed. Publication reports from alpha.2 do not pass a new scan. No exception or
suppression is added. A fresh Trivy database downloaded on 2026-10-28 and the
canonical source dependency scan passed for both the Go module graph and locked
packages, including development dependencies, with zero reported findings.
The source SPDX document was produced. Binary/container scans remain separate.

The staged-diff secret scan passed with zero findings. A directory-only scan
failed on 16 pre-existing synthetic redaction/provider test literals in two
files verified byte-identical to alpha.2; no ignore was added or widened. The
canonical whole-history scan subsequently passed for all 47 committed revisions,
with zero findings and the existing exact historical fixture ignore unchanged.

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
هر هفت کار CI و بستهٔ آزمایشی امضاشده پاس شدند؛ بستهٔ عمومی تگ هم جدا بررسی و اجرا شد؛ محدودهٔ دقیق در رسید عمومی است. ابزار audit ابتدا خطای شبکه داد؛ پرس‌وجوی مستقیم رسمی
برای همهٔ ۳۹۳ نسخهٔ قفل‌شده، شامل توسعه و بسته‌های اختیاری، یافته نداشت.
بعد خود ابزار audit هم با اتصال خروجی IPv4 پاس شد؛ TLS و شرط اسکن همان ماند.
اسکن منبع در ۲۰۲۶-۱۰-۲۸ با پایگاه تازه
برای Go و بسته‌ها پاس؛ یافته صفر. اسکن رازِ تغییرات پاک؛ اسکن پوشه ۱۶ رشتهٔ
مصنوعی قدیمی در دو فایل دست‌نخورده دید و رد شد. اسکن درستِ تاریخچهٔ هر ۴۷
کامیت پاس و پاک بود. بستهٔ عمومی در محدودهٔ رسید پاس شد؛ شرط پوشش شاخه و گام‌های بعد هنوز بازند.
