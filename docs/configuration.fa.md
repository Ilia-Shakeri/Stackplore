<div dir="rtl">

# پیکربندی اجرا

کد در حال توسعهٔ alpha.4، [تنظیم دستیار](v2/operator-profile.md) و
[اتصال اصلی](v2/application-runtime.md) را دارد؛ نسخهٔ منتشرشدهٔ alpha.3 این
کلیدها را رد کند. پیش‌فرض بسته است. هنگام فعال‌سازی، همهٔ pinها لازم‌اند؛
هنگام خاموشی حذف شوند. حالت مهمان فعال نشود. تنظیم از محیط پردازش بومی بیاید؛
manifestهای فعلی Compose و Helm هنوز این کلیدها را عبور ندهند. پردازنده
جداگانه روی loopback مدیریت شود؛ برنامه آن را نصب یا شروع یا probe نکند.
شروع یکسان، توقف ثبت‌شده و سهمیه و مصرف را حفظ کند. جریان سمت سرور و صفحهٔ
دوزبانه هنوز مانده‌اند.

این راهنما نکته‌های عملی را فارسی می‌گوید. نام متغیرها، حدها و مرجع کامل در
[راهنمای انگلیسی پیکربندی](configuration.md) و
[loader برنامه](../apps/api/internal/config/config.go) است.

## قاعده‌های پایه

- برنامه فقط محیط فرایند را می‌خواند؛ خودش `.env` را بار نمی‌کند. launcher توسعه
  ریشه `.env` را می‌خواند. Compose فایل `.env` خودش را می‌خواند.
- نام ناشناخته، تکراری، خالی یا بیرون از قالب، شروع را با خطای ثابت می‌بندد.
  مقدار ردشده را در log چاپ نکن.
- `.env.example` فقط قالب توسعه است. راز واقعی را در مخزن، argument، ticket یا
  گزارش CI ننویس.
- هیچ مسیر فایل، آدرس provider یا نشانی endpoint از مرورگر گرفته نمی‌شود.

## origin، پراکسی و سرویس وب

در production، `STACKPLORE_PUBLIC_ORIGIN` باید origin دقیق HTTPS باشد؛ مسیر،
query و fragment نداشته باشد. در Compose محلی از `http://localhost:PORT` استفاده
می‌شود. برنامه UI embedشده، API، health، readiness و metrics را از یک Go server
می‌دهد. مسیر مرورگر به index می‌رسد؛ فایل گمشده با پسوند و خطای API به UI تبدیل
نمی‌شوند.

`STACKPLORE_TRUSTED_PROXY_PREFIXES` باید فقط peer واقعی ingress را بشناسد.
شبکهٔ خصوصی کامل را به خاطر خصوصی‌بودنش trusted نکن. Ingress باید scheme و host
دقیق origin را نگه دارد. در Compose مستقیم، پراکسی حذف است.

## PostgreSQL

`STACKPLORE_DATABASE_URL` برای production به `sslmode=verify-full` نیاز دارد.
برای CA عمومی سیستم، مقدار `sslrootcert` را تنظیم نکن. برای CA اختصاصی، مسیر
فایل گواهی CA را با `sslrootcert` بده. در Helm مقدار
`api.databaseTLS.enabled=true`، Secret گواهی عمومی CA و کلید `ca.crt` را تعیین
کن؛ chart آن را فقط‌خواندنی در
`/var/run/stackplore/database-ca/ca.crt` mount می‌کند. کلید خصوصی TLS پایگاه به
برنامه داده نمی‌شود.

pool، timeout اتصال/query/migration/health و تعداد connection حد ثابت دارند؛
حدهای هر مقدار در مرجع انگلیسی است. migration فقط جلو می‌رود، تاریخچهٔ تغییرکرده
رد می‌شود و یک API مالک اجراست. SQLite پشتیبانی نمی‌شود.

## فراهم‌کننده و رازها

Docker و Kubernetes پیش‌فرض خاموش‌اند. نمایه و فایل گواهی را اپراتور در زمان
اجرا می‌دهد. Docker socket فقط برای توسعهٔ loopback است؛ آن را به Compose عمومی
mount نکن. نمایهٔ restricted از پراکسی mTLS محدود استفاده می‌کند. Kubernetes به
۱ تا ۶۴ namespace مشخص نیاز دارد؛ ClusterRole جداگانه و اختیاری است. Secret value
و ConfigMap value خوانده نمی‌شوند.

`STACKPLORE_KUBERNETES_ACCESS_PROFILE` پیش‌فرض `kubernetes_incluster` است.
برای kubeconfig فقط‌خواندنی، مقدار `kubernetes_kubeconfig` را بده. مسیر مطلق فایل
در `STACKPLORE_KUBERNETES_KUBECONFIG_FILE` و ۱ تا ۸ پوشهٔ mount مجاز در
`STACKPLORE_KUBERNETES_REFERENCE_ROOTS` لازم است؛ جداکننده در Linux `:` و در
Windows `;` است. ریشهٔ کل فایل‌سیستم مجاز نیست. نام دقیق context را با
`STACKPLORE_KUBERNETES_CONTEXT` بده؛ نبودنش از current-context فایل می‌خواند.
نام حداکثر ۲۵۶ بایت است. فایل، CA، گواهی و کلید هرکدام سقف ۱ MiB و توکن سقف
۱۶ KiB دارند. تمام symlinkها و ارجاع بیرون پوشه‌ها رد می‌شوند. فایل فقط ارجاع
CA و token-file یا جفت گواهی/کلید دارد؛ دادهٔ inline، افزونه، proxy، impersonation،
احراز هویت basic، extension و TLS ناامن در هر context رد می‌شود. writer با این
نمایه فعال نمی‌شود. پس از تغییر CA، گواهی، endpoint یا context سرویس را restart
کن. [نمونهٔ نمایه](kubernetes-provider.fa.md) مرز را شرح می‌دهد.

`STACKPLORE_LOG_REDACT_FIELDS` اختیاری، حداکثر ۳۲ نام فیلد اضافه برای پوشاندن
دادهٔ لاگ دارد؛ با ویرگول، بدون فاصله. هر نام ۱ تا ۶۴ بایت، با حرف لاتین آغاز
و فقط از حرف، رقم، `_` و `-` ساخته می‌شود. تطبیق و تشخیص تکرار، بزرگی حرف و
`_` و `-` را نادیده می‌گیرد. این قانون برای صفحه، دانلود و جریان زنده است؛
پوشاندن رازهای داخلی خاموش نمی‌شود. مقدار راز را اینجا نگذار؛ فقط نام فیلد است.

وب‌هوک و Matrix پیش‌فرض خاموش‌اند. مقصد باید HTTPS عمومی و دقیق باشد؛ هر تلاش،
DNS، IP، redirect، timeout و اندازهٔ پاسخ را بررسی می‌کند. راز مقصد و توکن را
در secret store نگه دار. شناسهٔ مقصد راز نیست و برای پایداری تحویل باید ثابت بماند.

## نصب تازه و secret

در نصب خالی غیر loopback، proof تصادفی ۳۲ بایتی و expiry حداکثر یک ساعت لازم است؛
۱۵ دقیقه پیشنهاد می‌شود. هر دو را موقت در محیط operator بگذار، setup را کامل کن
و بعد حذفشان کن. مقدار را در `.env`، shell history، argument یا گزارش نگذار.
راهنمای [راه‌اندازی و بازیابی](operations/account-recovery.fa.md) قدم‌ها را دارد.

Helm از Secret موجود برای `database-url` و secretهای اختیاری می‌خواند؛ Secret را
از secret manager بساز. `api.replicaCount` همیشه ۱ است. PVC کلید محلی digest را
نگه می‌دارد. Docker Compose دو volume نام‌دار دارد و `docker compose down` آنها
را پاک نمی‌کند؛ `down --volumes` پاک‌سازی دائمی است.

## مرجع‌های عملیاتی

- [نصب](operations/deployment.fa.md)
- [backup و restore](operations/backup-restore.fa.md)
- [ارتقا و برگشت](operations/upgrade-rollback.fa.md)
- [پایش و واکنش رخداد](operations/monitoring.fa.md)
- [عیب‌یابی](operations/troubleshooting.fa.md)

Compose متغیر اختیاری اپراتور `STACKPLORE_LOG_REDACT_FIELDS` را بدون ساخت مقدار
خالی عبور می‌دهد. در Helm، آرایهٔ `api.logRedactFields` همان نام فیلدها را دارد؛
آرایهٔ خالی تنظیم را حذف می‌کند. پس از تغییر سیاست، برنامه را دوباره اجرا یا
rollout کنید. نام‌ها مقدار راز نیستند؛ حذف پوشش پیش‌فرض هم مجاز نیست.

</div>
