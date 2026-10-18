# بستهٔ انتشار و بررسی آن

[English](release.md) · [روند مرحله‌ها](../releasing.md) · [مدرک فعلی](../implementation-plan.md)

گردش‌کار `release` در اجرای دستی بستهٔ آزمایشی می‌سازد و با tag هماهنگِ
`vVERSION` بسته را منتشر می‌کند. وجود فایل گردش‌کار، مدرک موفقیت انتشار نیست.
شناسهٔ commit، اجرای دقیق و نتیجهٔ بررسی بسته‌ها را پس از اجرا در سند مدرک ثبت
کن. شکست scan، امضا یا بررسی، انتشار را می‌بندد.

## کار نگه‌دارنده

۱. گیت محلی با PostgreSQL واقعی و همهٔ بررسی‌های نام‌بردهٔ مرورگر، فراهم‌کننده،
نصب و بازیابی را تمام کن. diff و changelog را بازبین. ابزار محلی، دادهٔ خصوصی و
عکس بی‌منشأ را وارد commit نکن.

۲. commit نامزد را پوش کن و `verify` همان commit را ببین. اجرای دستی `release`
روی شاخه، ساخت بسته، scan و امضا را می‌آزماید. بستهٔ آزمایشی ۳۰ روز نگه داشته
می‌شود؛ اجرای دستی release عمومی نمی‌سازد.

۳. tag توضیح‌دارِ هماهنگ را روی commit سنجیده بساز و پوش کن. tag منتشرشده را
جابجا نکن. گردش‌کار تطبیق tag با commit خود را می‌سنجد؛ حداکثر ۳۰ دقیقه برای
`verify` همان tag منتظر می‌ماند و موفقیت هر شش کار `code`، `deploy`،
`compose-install`، `helm-install`، `docker-provider` و `kubernetes-provider` را
لازم می‌داند.

۴. هر دو اجرای tag و انتشار را ببین. کار ساخت، خواندن کد و هویت امضای کوتاه‌عمر
دارد. فقط کار جداگانهٔ انتشار می‌تواند release بسازد. این کار بار دیگر گیت tag
را می‌سنجد و checksum همهٔ فایل‌ها را پس از انتقال بررسی می‌کند.

۵. بستهٔ عمومی را جدا دانلود و طبق روش زیر بررسی کن. نتیجه و شناسهٔ commit/run
را ثبت کن. نسخه‌های زیر `1.0.0` و نسخه‌های دارای پسوند آزمایشی، prerelease هستند.

نشر ابتدا draft می‌سازد، همهٔ فایل‌ها را بارگذاری می‌کند و فقط با موفقیت کامل
عمومی می‌شود. release موجود جایگزین نمی‌شود. شکست بارگذاری، draft را برای بررسی
نگه می‌دارد؛ فایل یا release بی‌خبر حذف نمی‌شود. پیش از بازیابی دستی، draft و
خطا را بررسی کن. تغییر کد پس از انتشار tag، نسخه و tag تازه می‌خواهد.

## فایل‌ها و مرز مدرک

| فایل | محتوا و مرز |
| --- | --- |
| `stackplore_VERSION_OS_ARCH.tar.gz` | سرور با وب داخلی، ابزار storage و operator، مجوز و notices؛ برای Linux amd64/arm64، Windows amd64 و macOS amd64/arm64. ساخت برای یک سیستم، مدرک اجرای واقعی روی همان سیستم نیست. |
| `stackplore_VERSION_image.tar.gz` | آرشیو Docker-save تصویر Linux amd64 با کاربر غیرریشه. registry خودکار منتشر نمی‌شود. |
| `stackplore-VERSION.tgz` | Helm chart؛ PostgreSQL بیرونی و Secret زمان اجرا را اپراتور فراهم می‌کند. |
| `stackplore_VERSION_source.tar.gz` | کد ردیابی‌شدهٔ همان commit، سندها، lockfile و تنظیم نصب. |
| `*.spdx.json` | موجودی SPDX برای هر بستهٔ بومی و تصویر؛ `source.spdx.json` اطلاعات وابستگی منبع و lockfile وب را هم دارد. بررسی باینری به‌تنهایی همهٔ وابستگی‌های وب داخلی را نمی‌بیند. |
| `dependency-scan.json`، `image-scan.json`، `*.scan.json`، `secret-scan.json` | رسید scan موفق وابستگی منبع، تصویر، باینری بومی و تاریخچهٔ Git. نتیجه با پایگاه آسیب‌پذیری تغییر می‌کند. |
| `release.json`، `image-id.txt` | commit، ref، run، هدف ساخت، نسخهٔ ابزار و digest پیکربندی تصویر محلی؛ این شناسه، digest manifest رجیستری نیست. |
| `native-smoke.json` | رسید بدون دادهٔ خصوصی از اجرای آرشیو بازشدهٔ Linux amd64 روی پایگاه‌های موقت جدا: وب داخلی، سلامت و readiness، خواندن مهمان و رد مسیر خصوصی، ساخت مدیر و نشست، نسخهٔ دقیق، خاموشی عملیات و توقف سالم. اجرای storage و operator فقط رد آرگومان خالی را می‌سنجد؛ backup و restore گیت جدا دارند. |
| `SHA256SUMS` | هش بسته‌ها، SBOM، اطلاعات ساخت و رسید scan موفق. |
| `provenance.sigstore.json`، `*-sbom.sigstore.json` | منشأ امضاشدهٔ همهٔ فایل‌های پیش از امضا، از جمله manifest checksum، و امضای جداگانهٔ SBOM هر آرشیو. bundle خودش را هش نمی‌کند؛ امضا، درستی آن را می‌سنجد. |

scan وابستگی منبع، وابستگی‌های توسعه و ساخت را هم می‌سنجد. عمل‌های گردش‌کار به
commit بازبینی‌شده بسته‌اند. scanner پیش از بازشدن با نسخهٔ
ثابت و SHA-256 منبع اصلی سنجیده می‌شود. Trivy همهٔ یافته‌های HIGH/CRITICAL
وابستگی و تصویر را رد می‌کند؛ حتی اگر اصلاح نشده باشند. استثنای خودکار و
`ignore-unfixed` وجود ندارد. Gitleaks تاریخچهٔ کامل checkout را با پوشاندن یافته
می‌سنجد. فهرست استثنا فقط fingerprint دقیقِ commit/مسیر/قاعده/خطِ آزمون‌های
قدیمی با اعتبارنامهٔ ساختگی را دارد؛ یافتهٔ تازه رد می‌شود. پوشهٔ آزمون را کلی
مستثنا نکن.

زمان و فرادادهٔ tar بومی به زمان commit یکسان می‌شود. دریافت ابزار، پایگاه
آسیب‌پذیری، مخزن بستهٔ تصویر و runner میزبان به شبکه نیاز دارند. تکرار بایت‌به‌بایت
ساخت یا نبود همهٔ آسیب‌پذیری‌ها ادعا نمی‌شود. قطع شبکه، scanner یا امضا، شکست
انتشار است؛ گیت سبز حساب نمی‌شود.

## پیش از بازکردن یا بارکردن تصویر

یک GitHub CLI معتبر با پشتیبانی بررسی attestation نصب کن. tag و commit مورد
انتظار را از کد بازبینی‌شده، جدا از `release.json` دانلودی بگیر. آرشیو انتخابی،
`SHA256SUMS` و `provenance.sigstore.json` همان release را دانلود کن. مقدارهای زیر
را با tag، commit کامل و نام بستهٔ انتخابی جایگزین کن.

```sh
tag=vVERSION
commit=FULL_REVIEWED_COMMIT_SHA
artifact=stackplore_VERSION_linux_amd64.tar.gz
identity="https://github.com/Ilia-Shakeri/Stackplore/.github/workflows/release.yml@refs/tags/$tag"
gh attestation verify SHA256SUMS \
  --repo Ilia-Shakeri/Stackplore --bundle provenance.sigstore.json \
  --cert-identity "$identity" --source-ref "refs/tags/$tag" \
  --source-digest "$commit" --deny-self-hosted-runners
gh attestation verify "$artifact" \
  --repo Ilia-Shakeri/Stackplore --bundle provenance.sigstore.json \
  --cert-identity "$identity" --source-ref "refs/tags/$tag" \
  --source-digest "$commit" --deny-self-hosted-runners
sha256sum --check --ignore-missing SHA256SUMS
```

هر دو بررسی امضا و checksum فایل انتخابی باید موفق شوند. برای دانلود همهٔ
فایل‌ها، `sha256sum --check --strict SHA256SUMS` را اجرا کن. در Windows، هش کامل
آرشیو را با `Get-FileHash -Algorithm SHA256` بگیر و با manifest امضاشده مقایسه
کن؛ دستورهای `gh` همان‌اند. هویت اجرای دستی شاخه را برای نصب release تگ‌شده نپذیر.

برای امضای SBOM آرشیو Linux بالا، `linux-amd64-sbom.sigstore.json` را هم بگیر.
همان بررسی‌های هویت و منبع را با `--bundle linux-amd64-sbom.sigstore.json` و
`--predicate-type https://spdx.dev/Document/v2.3` اجرا کن. برای هدف‌های دیگر یا
تصویر، bundle هم‌نام آن را بردار. خود فایل `.spdx.json` را با bundle منشأ یا
ورودی manifest امضاشده بررسی کن.

فقط پس از بررسی، ابزار را باز کن یا تصویر را با
`gzip -dc stackplore_VERSION_image.tar.gz | docker load` بار کن. نصب و پیکربندی را
با [راهنمای نصب](deployment.fa.md) ادامه بده و آمادگی و کارکرد محصول را دوباره
بسنج. بررسی بسته، منشأ و درستی آن را نشان می‌دهد؛ سلامت سرور مقصد را ثابت نمی‌کند.

## پیش‌نیاز امضا و منابع

امضا به attestation گیت‌هاب و هویت کوتاه‌عمر OIDC نیاز دارد. مخزن عمومی در طرح‌های
فعلی پشتیبانی می‌شود؛ مخزن خصوصی/داخلی طرح سازگار Enterprise Cloud می‌خواهد.
جایگزین بی‌امضا وجود ندارد. بررسی آفلاین ریشهٔ اعتماد جدا و معتبر هم می‌خواهد؛
ذخیرهٔ bundle به‌تنهایی آن را فراهم نمی‌کند.

اگر دریافت فرادادهٔ اعتماد خطا داد، دسترسی به نشانی رسمی TUF را از مسیر عادی
و مورد اعتماد اپراتور درست کن؛ بررسی امضا یا هویت را کنار نگذار. روی دستگاه
متصل و مورد اعتماد، `gh attestation trusted-root > trusted_root.jsonl` را اجرا
کن؛ ریشهٔ بررسی‌شده را جدا منتقل کن و به همان دستورهای بررسی،
`--custom-trusted-root trusted_root.jsonl` بیفزا. ریشه‌ای که فقط همراه دانلود
تأییدنشده آمده، منبع اعتماد مستقل نیست؛ [دستور ریشهٔ اعتماد](https://cli.github.com/manual/gh_attestation_trusted-root).

مرجع اصلی: [راهنمای attestation](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)،
[دستور بررسی](https://cli.github.com/manual/gh_attestation_verify)،
[SBOM در Trivy](https://trivy.dev/docs/latest/supply-chain/sbom/) و
[دستورهای Gitleaks](https://github.com/gitleaks/gitleaks#usage).
