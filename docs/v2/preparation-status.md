# Smart Assistant — separate development preparation

The unaccepted [draft change](https://github.com/Ilia-Shakeri/Stackplore/pull/1)
backs up prepared code at
`e660e16bd0b3c56f0096bb43b5688195135892b1`, branch `prep-v2-runtime`.
It is not integrated into main or the immutable `v2.0.0-alpha.0` source
`eb86ab7554db555f023c1de72c3119acf101226a`.

**The draft automatically applies schema 18 on normal startup.** Disabled
capability wiring does not make this migration inert. Use a fresh disposable
test database only; never use the accepted or customer schema-17 database.
Returning an older binary requires its verified backup/restore procedure.
Both draft READMEs, its plan and implementation ledger carry this warning.

The draft includes a local count/dispatch adapter, metadata-only run/usage
transactions, shared log privacy/context projection and a single disabled local
resource explanation use case. It has no enabled profile or public helper route.
The existing configured HTTP log reader also delegates to the shared policy;
the branch changes an existing path as well as adding preparation packages.

The final pre-commit concurrency run passed 132 test/subtest events across six
complete packages and focused real PostgreSQL/use-case/upgrade checks. Its
34-file source manifest matches the committed code byte for byte, SHA-256
`4d990879c1753acf93ae9b461b3fa7425bd972b705019e835fa5c1617edba3e1`.
These are scoped code and database results; the
[development evidence](preparation-evidence.json) keeps
their source separate from alpha.0 acceptance.

The [draft's six-job verification](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262991115)
passes, including the full gate, concurrency checks, disposable Docker and
Kubernetes providers, Compose, external-TLS Helm and browser checks. The tested
pull-request merge tree is `8756d6a8b8e7db57234b1f85b2eeee1f665c0d56`, exactly
equal to the preparation commit's tree. Its merge commit is
`a194ff7fdcb625d9cb75dc3ac223779d285b9567`, with base `eb86ab7` and head
`e660e16`. This is draft verification, not promotion or real-profile proof.

The maintainer's local checkout still contains the prepared files, including
`apps/api/postgresmigrations/000018_assistant_runtime.sql` and the two modified
existing files. After verifying a remote backup, automatic review rejected
removing that local preparation with only `blocked by policy`; no alternative
removal was attempted. The working tree is therefore **not a clean schema-17
alpha.0 source**. Its branch name alone does not exclude the untracked migration.
Use the verified release or a clean exact-source export for alpha.0 execution;
keep this retained preparation on disposable schema-18 test data only.

The real local processor test was explicitly skipped. The maintainer confirmed
that no compatible process is already running. Automatic review rejected the
candidate launch with only `blocked by policy`; no alternative launch was used.
The stage-1 profile, answer quality, all subsequent stage exits and final version
2 acceptance remain open. This draft has no milestone tag or release and must
not inherit alpha.0's receipts.

## فارسی

کد آماده‌سازی در پیش‌نویس جدا پوش شد؛ منبع و رسید خودش را دارد. در main یا
تگ ثابت `alpha.0` ادغام نشده. اجرای عادی شاخه، **طرح پایگاه ۱۸** را خودکار
اعمال کند؛ خاموشی قابلیت جلوی مهاجرت را نگیرد. فقط پایگاه تازهٔ تست به کار
رود؛ پایگاه پذیرفته یا مشتری با طرح ۱۷ به آن وصل نشود. برگشت باینری قدیمی،
پشتیبان/بازیابی تأییدشده لازم دارد.

درگاه محلی، مخزن متادیتا، سیاست گزارش و یک شرح محدود آماده‌اند؛ مسیر عمومی
و پروفایل فعال ندارند. مسیر HTTP گزارش نیز از سیاست مشترک استفاده کند.
تست رقابت و پایگاه **۱۳۲ رویداد قبولی** داشت؛ ۳۴ فایل کد با منبع کامیت‌شده
یکسان سنجیده شد؛ این‌ها پذیرش پردازنده یا پایان گیت بازیابی نیستند.

شش گیت پیش‌نویس هم پاس؛ درخت merge تست‌شده با کامیت آماده‌سازی یکسان بود.
فایل‌های محلی، از جمله مهاجرت ۱۸ و دو فایل تغییرکرده، باقی‌اند. حذف آن‌ها
بعد از تأیید پوشِ نسخهٔ پشتیبان در بررسی خودکار رد شد؛ دلیل فقط
`blocked by policy`. روش حذف دیگر اجرا نشد. نام شاخه، فایل untracked را
کنار نگذارد؛ درخت کار فعلی، منبع پاکِ `alpha.0` با طرح ۱۷ نیست. اجرای آن
نسخه از بستهٔ تأییدشده یا خروجی پاکِ منبع دقیق باشد؛ آماده‌سازی فقط روی
دادهٔ دورریختنیِ تست با طرح ۱۸ اجرا شود.

تست پردازندهٔ واقعی اجرا نشد. کاربر، نبود سرویس سازگار را گفت. راه‌اندازی
نامزد در بررسی خودکار رد شد؛ دلیل فقط `blocked by policy`. دور زدن اجرا نشد.
گام ۱، گام‌های بعد، کیفیت جواب و پایان نسخهٔ ۲ بازند. پیش‌نویس، تگ یا انتشار
مرحله‌ای ندارد؛ رسید `alpha.0` را به خود نسبت ندهد.
