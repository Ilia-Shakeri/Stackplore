# Smart Assistant — private metadata history preview

Version **2.0.0-alpha.6.3**, schema **19**, source
`48b3aec446303926f6f8e0ad993a257cbf6fdd11`.

## Source receipt

All eight [hosted source jobs](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42458989142)
pass. They cover the complete source/type/contract/locale/build gate, actual
PostgreSQL ownership/keyset/restore/session cases, concurrency, Docker/Kubernetes,
fresh Compose and external-TLS Helm installs, the selected real processor and
bilingual production-browser/offline journeys. New history-page checks exercise
keyboard paging/focus, replacement rows, status filters, refresh, owner loss,
late replies and page-exit cleanup. The actual processor/application journey opens
the real history page in both roles and languages with no extra admission.

The exact [tag verification](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42460315714)
and [signed publication workflow](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42460315807)
are separate gates; publication must not be inferred from this source receipt.
The [public release](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v2.0.0-alpha.6.3)
and [asset metadata](https://api.github.com/repos/Ilia-Shakeri/Stackplore/releases/tags/v2.0.0-alpha.6.3)
provide the publication identity, time, asset inventory and public digests once
the publication gate succeeds. A tag alone does not establish public availability.

## Delivered scope and bounds

- Owner-private safe metadata over the existing run tables, one exact status and
  seven-day visible retention. No migration is added.
- Ten rows per page; SQL uses one sentinel, an existing owner/state/time index,
  selected restore generation, deterministic keysets and current session checks.
- Ten-minute authenticated cursors bind owner, session, authentication revision,
  filter, page size, anchor and generation. Invalid scope fails before repository
  access. Reads have a three-second deadline; shutdown cancels work and clears keys.
- Closed `no-store` HTTP replies and detached typed-client validation. Unknown
  fields, changed state, invalid IDs/clocks/windows/counts, duplicate or misordered
  rows and cursor loops fail closed. Invalid client construction uses the existing
  synchronous safe-error contract and never reaches transport.
- Separate `/assistant/history` desktop page with English/Persian labels, URL
  status filters, ten-row replacement paging and at most 32 volatile page cursors.
  Owner/page loss erases rows and cursors; late prior-owner completion is ignored.
- Resource links require fresh current reads and explicit new-question submission.
  Old questions, answers, reports, evidence bodies and session material are neither
  persisted by this feature nor reconstructed from metadata.
- The operational build-info metric uses the canonical application version,
  replacing its fixed `0.0.0` placeholder without adding private metric labels.
- The corpus package is a pure structural preparation model. It grants no approval
  and supplies no durable corpus, import, search or document-read endpoint.

## Attempts and review boundary

The [implementation trail](../../v2/runbooks.md) retains failed SQL fixture,
cursor-parameter, type, formatting, lint and assertion attempts. Failed runs are
not reused as acceptance. Signed branch trials at earlier sources are separate
receipts: [d9251da](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42457760683)
and [a09244e](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42458551830)
pass packaging/security, while their source gates still fail in the recorded ways.
Only the complete green source above closes this implemented increment's source gate.

Code/architecture review: **9/10 within the implemented owner-history boundary**.
This is not a whole-stage or stable-product quality score. Stage-7 corpus storage,
explicit approval/revocation, authorized search and document views remain open.
The dataset still has 12 paired seed scenarios and 18 adversarial cases against
the 200-pair/1,000-case targets. Independent semantic review, pilot/soak/capacity
and final independent stable-artifact acceptance remain unaccepted. The maintainer
reports no evaluator or pilot and authorizes continued hosted preparation.

All tests, builds, services and scans execute on Actions. Workstation work is
limited to file reads/edits and lightweight Git/remote metadata operations.

## فارسی

هر هشت تست منبع بالا گذشت؛ رسید تگ و انتشار، گیت جداست. این پیش‌نمایش فقط
تاریخچهٔ فرادادهٔ حساب را می‌افزاید: ده ردیف، فیلتر وضعیت، صفحه‌بندی و پاک‌شدن
هنگام تعویض حساب یا خروج صفحه. پرسش، پاسخ یا گزارش منقضی‌شده بازسازی نمی‌شود.
مرجع تازه برای منبع، دسترسی جاری می‌خواهد و بررسی خودکار آغاز نمی‌کند. امتیاز
۹ از ۱۰ فقط برای کدِ همین مرز است؛ تأیید و جست‌وجوی سند، مجموعهٔ ارزیابی کامل،
بازبین مستقل، پایلوت و پذیرش نسخهٔ پایدار هنوز بازند. هیچ تست، ساخت یا سرویس
محلی اجرا نشد.
