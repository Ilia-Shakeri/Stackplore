# 2.0.0-alpha.0 local candidate evidence

This checkpoint adds the supplied artwork, matching accessible palette and
accepted version-2 design. It retains schema **17** and the existing product
routes. New helper runtime preparation is excluded from this candidate.

[Local receipt](local-verification.json) binds the frozen staged tree
`ee35c24874c0b81e596dcb1c790c78f358329e1b`, exact local log digests and public
fixture measurements. Documentation/evidence and the consumer's diagnostic
wording were updated afterward; the receipt separates those changes. Hosted
checks must bind the actual committed source before acceptance.

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

Exact-source hosted CI, signed packaging, public prerelease consumption and
stage-1 real-processor acceptance remain separate open gates. No stable 2.0.0,
customer deployment, helper answer quality or production capacity is accepted.

## فارسی

این نامزد، لوگو و تم و طرح پذیرفتهٔ نسخهٔ ۲ را دارد؛ schema همان **۱۷**.
کدِ آماده‌سازی اجرای یار، داخل این منبع نیست. گیت محلی، **۵۴۴ تست رابط، ۴۰
تست کلاینت، پایگاه واقعی، بازیابی و ساخت** را گذراند. **۱۴ تست مرورگر** بدون
تکرار پاس؛ ۸ نمای زبان/اندازه/تم و ۳۲ تصویر ثبت شد. cache فقط چهار فایل عمومی.

پوشش صفحه‌کلید، دسترسی، راست‌به‌چپ و تمِ آفلاین سنجیده شد. بار ۸۰ خواندن،
p95 برابر ۱۰۰ms در همین دستگاه داد؛ قول ظرفیت نیست. رسید شکست قبلی حفظ شد.
پذیرش منبع کامیت‌شده، بستهٔ امضاشده و پردازندهٔ واقعی هنوز گیت جدا دارند.
