<div dir="rtl">

# پیکربندی اجرا

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

</div>
