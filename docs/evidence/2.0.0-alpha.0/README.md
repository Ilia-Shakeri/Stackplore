# 2.0.0-alpha.0 evidence

This checkpoint adds the supplied artwork, matching accessible palette and
accepted version-2 design. It retains schema **17** and the existing product
routes. New helper runtime preparation is excluded from the immutable release
source `eb86ab7554db555f023c1de72c3119acf101226a`.

Status reconciled **2026-10-21**: `v2.0.0-alpha.0` is published, not stable
2.0.0. Main/tag hosted gates, signatures and native proof have passed in their
recorded scopes. Independent public database archive execution also passes;
[public acceptance](published-release.json) closes stage 0 within its recorded
scope, including exact public-to-trial Windows archive identity.
Receipt UTC timestamps retain their actual dates, including 2026-10-19;
this later reconciliation does not rewrite them.

## Local staged-tree proof

[Local receipt](local-verification.json) binds the frozen staged tree
`ee35c24874c0b81e596dcb1c790c78f358329e1b`, exact local log digests and public
fixture measurements. Documentation/evidence and the consumer's diagnostic
wording were updated afterward; the receipt separates those changes. The
committed-source hosted receipts below provide their own source binding.

- Pinned `npm.cmd run verify`: exit 0; 544 web tests, 40 client tests, 35 release
  tooling checks, 11 corpus-checker tests, real PostgreSQL 17.11 repositories and
  native recovery tools, contract/locale/lint/types/notices, and production build.
- `node scripts/browser-gate.mjs`: exit 0; twelve guest cases plus normal English
  and Persian setup/authentication/history/logout journeys, all without retry.
  Eight locale/size/theme profiles cover 64 routes, keyboard/accessibility and
  public-only caching. Both explicit offline themes override an opposite device
  preference. [Measurements](browser-measurements.json).
- The unchanged bounded fixture completed 80 validated reads across eight clients
  and ten rounds: median 29 ms, p95 100 ms, minimum round spacing 600 ms. These are
  local observations, not a capacity promise or a 600 ms latency ceiling.
- Thirty-two PNGs include the new About view. Desktop light/dark and Persian
  mobile dark views were visually inspected. [Light](screenshots/about-en-1280-light.png),
  [dark](screenshots/about-en-1280-dark.png), [Persian mobile](screenshots/about-fa-360-dark.png).
- The [archive consumer](../../../scripts/verify-browser-evidence.ps1) passed its
  native 29-case regression and separately checked the actual local browser ZIP.
  Synthetic archive fixtures prove checker behavior only; local archive parsing
  does not prove hosted CI identity or execution.

The [active ledger](../../v2/implementation.md) preserves failed/partial attempts.
In particular, attempt D failed its obsolete icon-MIME test; it is not green.
The corrected attempt E passes. [Artwork inventory](../../brand.md) records all
twelve compressed derivatives and unchanged source hashes.

## Exact committed-source hosted and consumer proof

- [Hosted gates](hosted-gates.json): main CI
  [39258522963](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39258522963)
  passes six jobs on `eb86ab7554db555f023c1de72c3119acf101226a`; signed main
  trial [39258604051](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39258604051)
  passes packaging while publication is intentionally skipped for the trial.
- [Hosted browser consumption](main-browser.json) binds the actual main CI
  artifact, its ZIP digest, fourteen sole-attempt cases, 64 route measurements
  and 32 images to that exact commit. Local measurements above retain their
  own scope and are not substituted for hosted measurements.
- [Trial consumer](consumer-trial.json) independently verifies the signed trial
  and records all 39 files, 30 checksum entries, 31 provenance subjects,
  seven SBOM attestations and eight strict scans with zero HIGH/CRITICAL
  findings. The signed Linux amd64 archive startup receipt and actual Windows
  amd64 schema-17 backup/restore, recovery, replay, revocation and ownership-loss
  proof are recorded separately inside it. The earlier failed Windows fixture
  assertion is retained; correcting its expectation did not change the product.
- Tag CI [39262174884](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174884)
  passes all six jobs; release run
  [39262174841](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174841)
  published prerelease [417357745](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v2.0.0-alpha.0).
  Independent public trust checks pass for all **39 files, 30 checksum entries,
  31 provenance subjects and seven SBOM attestations**, with eight strict scans
  showing zero HIGH/CRITICAL findings at their recorded time. The
  [public receipt](published-release.json) binds the actual public Windows
  archive's byte identity to the recovery-tested trial. Public archive runtime
  checks retain the narrower outcomes below.

## Public archive execution boundaries

The actual public application archive passed load and image-identity checks:
five layers, five exact derived legacy documents and 18 archive entries.
Configuration digest is
`sha256:dd92449f88dfac9ef71ce5e4b50e14e8405052e5b654d81f50099ea18b344a40`;
validated OCI/runtime digest is
`sha256:326063a438831b1e40c088bdc239e5736c8fb74f3b17aef507fee314ec402ea8`.
**No application container was started.** Earlier consumer attempts failed on
an unreferenced blob before load and configuration-versus-manifest identity
after load. Both failures remain retained; corrections were confined to the
consumer helper, whose 18 pure regression tests pass.

The public database archive **independently passed actual execution** on local
Linux amd64: 13 layers, 13 exact derived legacy documents and 34 entries;
PostgreSQL 17.11 and `gosu` 1.19 built with Go 1.26.9; actual root drop to
UID/GID 70 and server UID 70; network-none/read-only containers and bounded
tmpfs; create/insert/select/rollback SQL. All six owned containers were removed
and absence verified. Its final consumer passes 28 pure tests with review 9/10.
The [public receipt](published-release.json) binds the exact archive, consumer,
tests and runtime result. No customer database or deployment was used.

The original attempt failed `created_container_isolation`; its precise subphase
remains unclassified, and owned cleanup ran. Two separate stopped-container
diagnostics did not start a program. The root-drop diagnostic reproduced the
daemon's prefixed capability names. The consumer now accepts only either
complete exact known pair, rejecting mixed forms, duplicates and additional
permissions. This helper correction left product code unchanged. The failed
attempt and diagnostics retain their original result; only the later complete
runtime attempt establishes passing execution.

## Unaccepted preparation and remaining scope

[Draft PR 1](https://github.com/Ilia-Shakeri/Stackplore/pull/1),
`e660e16bd0b3c56f0096bb43b5688195135892b1`, is separate unaccepted stage-1–3
preparation. The [preparation status](../../v2/preparation-status.md) records
its source, scoped code/SQL checks and passing draft CI; it is not in alpha.0.
The maintainer has no compatible running processor, its real-profile test was
explicitly skipped, and automatic review rejected candidate launch with only
`blocked by policy`. No alternate launch was attempted. Stages **1–10**,
answer quality and stable 2.0.0 remain incomplete.

The local checkout retains two modified existing files plus untracked
preparation after cleanup was review-rejected. Normal startup applies migration
**18**, even with the capability disabled. Execute alpha.0 from verified release
bytes or a clean exact-source export; retained preparation uses a fresh
disposable schema-18 test database only. Alpha.0 schema-17 evidence does not
accept that mixed checkout. Customer deployment and production capacity also
require their own installation-specific proof.

## فارسی

این پیش‌انتشار، لوگو و تم و طرح پذیرفتهٔ نسخهٔ ۲ را دارد؛ schema همان **۱۷**
و منبع ثابت `eb86ab7`. کد آماده‌سازی در آن نیست. متن در ۲۰۲۶-۱۰-۲۱ تطبیق
شد؛ تاریخ واقعی UTC رسیدهای قبلی دست نخورد. گیت محلی، **۵۴۴ تست رابط، ۴۰
تست کلاینت، پایگاه واقعی، بازیابی و ساخت** را گذراند. **۱۴ تست مرورگر** بدون
تکرار پاس؛ ۸ نمای زبان/اندازه/تم و ۳۲ تصویر ثبت شد. cache فقط چهار فایل عمومی.

پوشش صفحه‌کلید، دسترسی، راست‌به‌چپ و تمِ آفلاین سنجیده شد. بار ۸۰ خواندن،
p95 برابر ۱۰۰ms در همین دستگاه داد؛ قول ظرفیت نیست. رسید شکست قبلی حفظ شد.

شش کار main و شش کار تگ، بستهٔ امضاشده، ۱۴ تست مرورگرِ منبع دقیق و بررسی
مستقل **۳۹ فایل، ۳۰ checksum، ۳۱ موضوع منشأ و ۷ گواهی SBOM** پاس شدند.
آرشیو واقعی Windows بازیابی و بازپخش و مالکیت را گذراند. آرشیو عمومی برنامه
بار شد و هویتش سنجیده شد؛ کانتینر برنامه اجرا نشد. دو شکست پیشین مصرف‌کننده
حفظ شدند. آرشیو عمومی پایگاه نیز مستقل اجرا شد: ۱۳ لایه و ۱۳ سند مشتق دقیق،
۳۴ عضو، PostgreSQL 17.11، افت دسترسی و UID سرور ۷۰، شبکهٔ خاموش، ریشهٔ
فقط‌خواندنی، tmpfs محدود، تراکنش SQL و پاک‌سازی و نبود شش کانتینر پاس.
مصرف‌کنندهٔ نهایی ۲۸ تست خالص دارد. شکست `created_container_isolation` حفظ
شد؛ زیرگام دقیقش طبقه‌بندی نشده. دو بررسیِ فقط‌ساخت برنامه اجرا نکردند؛
فقط تلاش کامل بعدی، اجرای پایگاه را ثابت کند. گام صفر در حد رسید پذیرفته شد.

PR 1 فقط آماده‌سازی نپذیرفتهٔ گام‌های ۱ تا ۳ است؛ گام‌های **۱ تا ۱۰**، کیفیت
پاسخ و پایدار ۲ کامل نیستند. پردازندهٔ سازگار در حال اجرا نیست و راه‌اندازی
نامزد در بررسی خودکار رد شد. درخت محلی دو فایل تغییرکرده و فایل‌های ثبت‌نشده
را نگه دارد؛ مهاجرت **۱۸** حتی با قابلیت خاموش اجرا شود. alpha.0 از بستهٔ
تأییدشده یا خروجی پاکِ منبع دقیق اجرا شود؛ آماده‌سازی فقط پایگاه تازهٔ
دورریختنی با طرح ۱۸ داشته باشد.
