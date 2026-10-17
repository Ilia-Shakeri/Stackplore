# نصب و راه‌اندازی

## اجرای محلی با Compose

Compose یک سرویس Go غیر root می‌سازد و فایل‌های وب را داخل همان برنامه می‌گذارد.
درگاه فقط روی `127.0.0.1` باز است. PostgreSQL و کلید runtime در volumeهای نام‌دار
می‌مانند. readiness تنها پس از آماده‌شدن API و پایگاه داده موفق می‌شود.

1. `deploy/compose.env.example` را به `.env` کپی کن.
2. هر دو جای‌نگهدار گذرواژه را عوض کن. گذرواژه در
   `STACKPLORE_COMPOSE_DATABASE_URL` باید URL-encode شود.
3. برای پایگاه خالی، طبق [راه‌اندازی نخست](account-recovery.md) proof و زمان
   انقضای کوتاه بساز. این دو مقدار را در نشست shell بگذار؛ در `.env` ننویس.
4. `docker compose up --build -d` را اجرا کن، وضعیت `docker compose ps` را
   ببین و `http://localhost:8080/setup` را باز کن. پس از ساخت مدیر نخست، دو
   مقدار راه‌اندازی را از shell پاک کن و فقط API را با
   `docker compose up -d --force-recreate api` دوباره بساز.
5. برای توقف بی‌حذف داده `docker compose down` بزن. `--volumes` را فقط برای
   حذف دائمی داده‌ها به کار ببر. اجرای دوباره volumeها را نگه می‌دارد.

UI و API از یک origin می‌آیند. مسیرهای مرورگر `index.html` را می‌گیرند؛ فایل
گمشده با پسوند، مسیر API و مسیر health به UI نمی‌افتد. PostgreSQL بی‌TLS فقط در
شبکه خصوصی محلی است؛ این آزمایش جای نصب production یا ingress عمومی نیست.

## چارت Kubernetes

چارت پایگاه داده نصب نمی‌کند. URL PostgreSQL بیرونی را با
`sslmode=verify-full` در Secret موجود بگذار. برای CA خصوصی،
`api.databaseTLS.enabled` را روشن کن و Secret جداگانهٔ CA بساز. چارت فقط گواهی
عمومی CA را در `/var/run/stackplore/database-ca/ca.crt` mount می‌کند؛ همین مسیر
را با `sslrootcert` در URL بگذار. کلید خصوصی TLS پایگاه را به برنامه mount نکن.

ریشهٔ PVC از Kubernetes دسترسی گروهی می‌گیرد. init container محدود با همان
تصویر برنامه و UID غیر root برابر `10001`، زیرپوشهٔ `private/` را با دسترسی
`0700` آماده می‌کند. API فقط همان زیرپوشه را در `/var/lib/stackplore` می‌بیند.
کلید digest دسترسی `0600` دارد؛ برنامه پوشه و کلید خواندنی برای گروه را رد
می‌کند. `fsGroupChangePolicy: OnRootMismatch` در mount عادی از بازشدن بازگشتی
دسترسی جلوگیری می‌کند؛ init در هر شروع هم دسترسی خصوصی را دوباره برقرار می‌کند.

اگر volume چارت قدیمی، کلید را در ریشهٔ PVC دارد، init همان فایل معمولی ۳۲بایتی
با مالک درست و یک پیوند را می‌سنجد و اتمی به زیرپوشه می‌برد. کلید چاپ یا
جایگزین نمی‌شود. symlink، مالک یا اندازهٔ نادرست و وجود کلید در هر دو جایگاه،
شروع را می‌بندد. volume را نگه دار و با مدیریت معتبر storage آن را اصلاح کن؛
برای دورزدن گیت، کلید را حذف نکن چون هویت نشست را عوض می‌کند.

رازها را در سامانهٔ مدیریت راز نگه
دار و image repo، تگ تغییرناپذیر، origin عمومی، دامنه ingress و storage class را
در فایل خصوصی values تنظیم کن.

```sh
helm upgrade --install stackplore deploy/helm/stackplore \
  --namespace stackplore --create-namespace \
  --values values.production.yaml \
  --atomic --wait
```

Ingress به Service همان برنامه وصل است؛ وب جداگانه نصب نمی‌شود. API یک replica
می‌ماند چون کلید digest و قفل مالکیت تک‌نویسنده به volume از نوع ReadWriteOnce
وابسته‌اند. health فقط زنده‌بودن پردازه را می‌سنجد؛ readiness پایگاه داده را هم
می‌سنجد. دریافت secretهای فراهم‌کننده مجاز نیست.

منبع Kubernetes خاموش است. برای روشن‌کردن آن، ۱ تا ۶۴ namespace دقیق در
`api.kubernetes.namespaces` تعیین کن. Roleها فقط خواندن منابع مجاز را می‌دهند؛
خواندن مقدار Secret یا ConfigMap داده نمی‌شود. از `cluster-admin` استفاده نکن.
حدود کامل در [راهنمای فراهم‌کننده](../kubernetes-provider.md) است.

Compose هر دو منبع زنده را خاموش نگه می‌دارد. اتصال مستقیم Docker socket فقط
در پروفایل صریح توسعه با درگاه loopback پذیرفته می‌شود؛ شروع production آن را
رد می‌کند. خواندن Docker در production از پروکسی محدود mTLS و پروفایل mountشدهٔ
تحت مدیریت اپراتور می‌گذرد. ورودی اعتماد و مسیرهای مجاز در
[راهنمای Docker](../docker-provider.md) است. socket میزبان را به نصب عمومی mount نکن.

## مدرک و بررسی

تست handler مسیر فایل و SPA را می‌سنجد، نه image یا نصب را. CI سرویس Compose را
می‌سازد، روی PostgreSQL تازه اجرا می‌کند، readiness و UI و مسیرهای SPA را
می‌سنجد، API را restart می‌کند و ماندن migration ledger را بررسی می‌کند. CI
چارت را lint و render می‌کند؛ این کار نصب cluster را ثابت نمی‌کند.

```sh
docker compose --env-file deploy/compose.env.example config --quiet
helm lint deploy/helm/stackplore --strict
```

ساخت تصویر و scan آسیب‌پذیری پیش از تولید آرشیو Docker-save انتشار لازم است؛
این گردش‌کار registry منتشر نمی‌کند.

بازیابی backup، upgrade و rollback روی پایگاه هدف باید جداگانه تمرین و ثبت شوند.
جزئیات [backup و restore](backup-restore.md) و
[محدودیت upgrade و rollback](upgrade-rollback.md) را ببین.
