# Versioned browser evidence checks

`scripts/verify-browser-evidence.ps1` requires PowerShell 7.4 or newer. It reads ZIP entries in memory, with bounded input sizes, without extracting or executing archive content. The historical 1.0.3 consumer remains unchanged.

Run the checker regression first:

```powershell
pwsh -NoProfile -File scripts/verify-browser-evidence.test.ps1
```

These synthetic fixtures test acceptance and rejection rules. They do not run a browser or prove release readiness. Each run writes a separate receipt beneath `.tmp/v2-browser-consumer-proof/`.

Check an existing archive with its exact version:

```powershell
pwsh -NoProfile -File scripts/verify-browser-evidence.ps1 `
  -Version 2.0.0-alpha.0 -ValidateArchive C:\scratch\browser-evidence.zip
```

Local validation performs no credential lookup or network request. Its output states that archive structure alone proves no source identity, CI execution or browser execution.

For authenticated retrieval, provide `-RunId`, the exact lowercase 40-character `-Revision`, `-Version`, and a new `-Destination` directory. The directory must be a child of `-DestinationRoot`, which defaults to this repository's `.tmp/browser-evidence/`. An optional unique `-ReceiptName v2-example.json` selects a new receipt in `.tmp/`; existing directories and receipts are refused. Retrieval uses the existing Git credential helper, keeps credentials in memory, and uses normal TLS. Credentials accompany only fixed GitHub API requests; the signed HTTPS artifact transfer receives no authorization header.

Acceptance requires the exact main-branch revision, CI workflow, six successful jobs, unique unexpired `browser-evidence` artifact, and matching ZIP size and SHA-256. Selected content must contain:

- Fourteen sole-attempt passing cases: twelve guest, one normal English, one normal Persian; zero retries, skipped cases, flaky cases or report errors.
- Eight locale, width and theme profiles; sixty-four route measurements; eight palette journeys; four environment-selector journeys, including two mobile drawers.
- Exactly thirty-two current-version PNG names and valid signatures: overview, topology, command palette and about for each profile. Signature checks do not decode images or assess their visual quality.
- The exact public-only offline/update cache paths, both selected-theme overrides against opposite device themes, installability checks and bounded public worker control read.
- Eighty successful validated reads across eight parallel clients and ten rounds, minimum 600 ms round spacing, exact setup/refill gates and separately recorded timing. These fixture observations are not a production capacity claim.

`-ExpectedConclusion failure` is a diagnostic mode: the code job must have failed and the other five jobs succeeded. It parses bounded reports but skips screenshot and workload acceptance. A diagnostic receipt cannot satisfy acceptance.

## فارسی

این چک به PowerShell نسخهٔ ۷٫۴ یا بالاتر نیاز دارد. فایل فشرده را محدود و بدون استخراج یا اجرا می‌خواند. ابزار تاریخی نسخهٔ ۱٫۰٫۳ عوض نشده است.

تست نخست با نمونه‌های ساختگی، قانون پذیرش و رد فایل را می‌سنجد. مرورگر اجرا نمی‌کند و آمادگی انتشار را ثابت نمی‌کند. بررسی محلی نیز به شبکه و اطلاعات ورود دست نمی‌زند؛ فقط ساختار فایل را می‌سنجد.

دریافت واقعی به شناسهٔ اجرا، نسخه، شناسهٔ دقیق تغییر و پوشهٔ تازه نیاز دارد. هویت منبع، هر شش کار موفق، اندازه و هش فایل باید یکی باشند. اطلاعات ورود فقط در حافظه و فقط برای درخواست ثابت مخزن به کار می‌رود؛ به مقصد انتقال فایل فرستاده نمی‌شود. فایل خروجی و پوشهٔ موجود بازنویسی نمی‌شود.

پذیرش به هر ۱۴ تست موفق بدون تکرار، ۸ نمای زبان/اندازه/تم، ۳۲ تصویر با نام دقیق، ۶۴ سنجش مسیر، بررسی دسترسی و صفحه‌کلید، کش عمومی محدود و هر ۸۰ خواندن موفق نیاز دارد. فاصلهٔ ۶۰۰ میلی‌ثانیه حداقل فاصلهٔ دورهاست؛ سقف تأخیر درخواست نیست. امضای تصویر، کیفیت دیداری آن را ثابت نمی‌کند. خروجی تشخیصی اجرای شکست‌خورده نیز مدرک پذیرش نیست.
