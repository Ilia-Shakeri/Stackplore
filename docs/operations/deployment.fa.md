# نصب و راه‌اندازی

نسخهٔ اصلاحی **۱٫۰٫۳ منتشر و مستقل پذیرفته شد**؛ ledger برابر **۱۷**، تگ ثابت `v1.0.3` روی `656581cf05d4774358d5644a6e1af3c69a210f81`. [رسید عمومی](../evidence/1.0.3/published-release.json) منبع و بستهٔ دقیق را پیوند می‌دهد. نصب مشتری، مجوز منبع، رسیدن اعلان و ظرفیت، مدرک جداگانهٔ اپراتور می‌خواهند. پذیرش و رسید ۱٫۰٫۲ پایین فقط تاریخی‌اند.

## Historical release proof

هدف منبع تاریخی **۱٫۰٫۲**، تگ ثابت `v1.0.2` روی `376ed03` و ledger برابر ۱۷
است. هر شش بررسی اصلی و تگ همان منبع، شامل Compose،‏ Helm با TLS خارجی و هر
دو فراهم‌کننده گذشتند. بستهٔ آزمایشی امضاشده و اجرای واقعی آرشیو بومی و پایگاه
گذشت؛ انتشار امضاشده و بررسی مستقل ۳۹ فایل و اجرای آرشیو عمومی هم در
[رسید عمومی](../evidence/1.0.2/published-release.json) گذشت. رسید
[۱٫۰٫۱](../evidence/1.0.1/published-release.json) با منبع و
محدودهٔ تاریخی خود می‌ماند. فرمان‌ها مسیر ساخت از منبع و آرشیو بررسی‌شده را
می‌گویند؛ نصب بدون شبکه یا rollout مشتری را ثابت نمی‌کنند. volume پایگاه و
کلید runtime موجود را نگه دار.

## اجرای محلی با Compose

Compose **دو تصویر** می‌سازد: برنامهٔ Go غیر root با فایل‌های وب درون آن، و
تصویر مشتق PostgreSQL برای Linux amd64 با نام `stackplore-postgres:1.0.3` از
`deploy/Dockerfile.postgres`. درگاه برنامه فقط روی `127.0.0.1` باز است.
PostgreSQL و کلید runtime در volumeهای نام‌دار می‌مانند. readiness تنها پس از
آماده‌شدن API و پایگاه داده موفق می‌شود.

[سیاست پایگاه](../../deploy/postgres-image.json)، پایهٔ PostgreSQL 17.11 و منبع
دست‌نخوردهٔ gosu 1.19 را ثابت می‌کند؛ ابزار با Go 1.26.10 بازساخته می‌شود.
entrypoint بالادستی پایگاه حفظ می‌شود: آماده‌سازی می‌تواند با root شروع شود و
سپس gosu دسترسی را به کاربر پایگاه پایین می‌آورد. برنامهٔ Go غیر root می‌ماند.
ساخت از منبع، تصویر پایهٔ ثابت، منبع ابزار با checksum ثابت و وابستگی‌های قفل‌شده
را می‌گیرد؛ شبکه لازم است، مگر ورودی‌ها پیش‌تر در cache ساختِ بررسی‌شده باشند.
برای نصب آفلاین، هر دو آرشیو انتشار بررسی‌شدهٔ زیر را به کار ببر. تصویر
بالادستیِ تنها که رد شده است را جایگزین نکن.

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

### نصب از آرشیو انتشار بررسی‌شده

این مسیر فقط پس از گذر امضا، checksum، هویت منبع و هفت گواهی SBOM نسخهٔ دقیق
در [بررسی بسته](release.fa.md) به کار می‌رود. بسته باید ۳۹ فایل، ۳۰ ورودی
checksum و همهٔ ۳۱ موضوع provenance داشته باشد؛ این شمار، مدرک انتشار نیست.
منبع/فایل Compose و هر دو تصویر Linux amd64 باید از همان نسخهٔ پذیرفته‌شده باشند.
فرمان‌های آرشیو در نصب بدون شبکه اجرا نشده‌اند؛ رسید CI ساخت از منبع و restart
در Compose، این مسیر را ثابت نمی‌کند.

پس از انتشار و بررسی مستقل ۱٫۰٫۳، هر دو آرشیو فشردهٔ همان نسخه را بار کن؛
نام آرشیوهای مورد انتظار ۱٫۰٫۳ چنین است:

```sh
docker image load --input stackplore_1.0.3_image.tar.gz
docker image load --input stackplore_1.0.3_postgres_image.tar.gz
```

تگ‌های بارشده باید `stackplore-api:1.0.3` و `stackplore-postgres:1.0.3` باشند؛
هویت تصویرها را با `image-id.txt` و `database-image.json` بررسی‌شده بسنج.
سرویس API در Compose پایه، build دارد و تگ تصویر انتشار ندارد. فایل
`compose.release.yaml` را کنار منبع همان نسخه بساز تا تصویر بارشده صریح انتخاب شود:

```yaml
services:
  api:
    image: stackplore-api:1.0.3
  postgres:
    image: stackplore-postgres:1.0.3
```

مراحل `.env` خصوصی و proof راه‌اندازی پایگاه خالی را طبق بالا انجام بده؛ سپس
به‌جای فرمان شروع با ساخت از منبع، اجرا کن:

```sh
docker compose -f compose.yaml -f compose.release.yaml config --quiet
docker compose -f compose.yaml -f compose.release.yaml up --no-build --pull never -d
docker compose -f compose.yaml -f compose.release.yaml ps
```

نبود تصویر باید شروع را متوقف کند؛ در این مسیر تصویر جایگزین نساز یا نگیر.
پس از ساخت مدیر نخست، دو مقدار proof را از shell پاک کن و فقط API را دوباره بساز:

```sh
docker compose -f compose.yaml -f compose.release.yaml up --no-build --pull never -d --force-recreate api
```

برای شروع‌های بعدی، همین override و گزینه‌های `--no-build --pull never` را
نگه دار. با `docker compose -f compose.yaml -f compose.release.yaml down` و بدون
`--volumes` متوقف کن. این فرمان‌ها همان volumeها را حفظ می‌کنند؛ restore پایگاه
یا مجوز جابه‌جایی production مشتری نیستند. مرجع رسمی Docker،
[گزینه‌های Compose](https://docs.docker.com/reference/cli/docker/compose/up/) و
[بارکردن تصویر فشرده](https://docs.docker.com/reference/cli/docker/image/load/) را شرح می‌دهد.

## چارت Kubernetes

چارت پایگاه داده نصب یا مدیریت نمی‌کند. URL PostgreSQL بیرونیِ متعلق به اپراتور
با نسخهٔ 17.11 یا وصلهٔ جدیدتر از شاخهٔ 17.x را با `sslmode=verify-full` در
Secret موجود بگذار؛ ابزار نگهداری بومی هم همین حداقل را دارد. ساخت پایگاه
آزمایشی اصلاح‌شده در CI، آن را به پایگاه تحت مدیریت چارت بدل نمی‌کند و خدمت
اپراتور را ثابت نمی‌کند. برای CA خصوصی،
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

برای پایگاه خالی، دو کلید کوتاه‌عمر تعیین‌شده در `api.setup.proofSecretKey` و
`api.setup.expirySecretKey` را به Secret موجودِ اپراتور بیفزا و برای نخستین
rollout،‏ `api.setup.enabled=true` بگذار. طبق [راه‌اندازی نخست](account-recovery.md)
آن‌ها را بساز، proof را در `/setup` وارد کن و پس از پایان، بلوک setup را خاموش
و دو کلید را از Secret پاک کن. پایگاه آماده‌شده به این مقدارها نیاز ندارد.
proof را در values، آرگومان فرمان یا manifest عمومی نگذار.

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

تست handler مسیر فایل و SPA را می‌سنجد، نه image یا نصب را. CI هر دو تصویر Compose را
می‌سازد، روی PostgreSQL تازه اجرا می‌کند، readiness و UI و مسیرهای SPA را
می‌سنجد، API را restart می‌کند و ماندن migration ledger را بررسی می‌کند. CI
چارت در CI گیت نصب واقعی روی kind با PostgreSQL خارجی و TLS هم دارد؛ نتیجهٔ
هر اجرای دقیق باید در رسید باشد. lint و render به‌تنهایی نصب را ثابت نمی‌کنند؛
مسیر و گواهی ingress مشتری نیز به بررسی همان محیط نیاز دارند.

```sh
docker compose --env-file deploy/compose.env.example config --quiet
helm lint deploy/helm/stackplore --strict
```

ساخت واقعی هر دو تصویر برنامه و پایگاه، اسکن کامل سخت‌گیرانه، SPDX هم‌بسته،
probe موتور/ابزار/افت دسترسی و آرشیوهای Docker-save امضاشده لازم است. اسکن
ابزار تنها، شاهد کل تصویر نیست. این گردش‌کار registry منتشر نمی‌کند.

بازیابی backup، upgrade و rollback روی پایگاه هدف باید جداگانه تمرین و ثبت شوند.
جزئیات [backup و restore](backup-restore.md) و
[محدودیت upgrade و rollback](upgrade-rollback.md) را ببین.
