<div dir="rtl">

# پشتیبان‌گیری و بازیابی PostgreSQL

مسیر HTTP برای backup/restore وجود ندارد. فرمان storage از `pg_dump` و
`pg_restore` سازگار با major پایگاه استفاده می‌کند. URL فقط از
`STACKPLORE_DATABASE_URL` می‌آید و گذرواژه به child process می‌رود؛ URL و آرشیو
راز هستند.

پس از [بررسی بستهٔ بومی انتشار](release.fa.md)، از `./stackplore-storage`
یا در Windows از `.\stackplore-storage.exe` همان بسته استفاده کنید. در مثال‌های
زیر، این مسیر جای پیشوند `go -C apps/api run ./cmd/storage` می‌آید؛ زیرفرمان،
آرگومان و محیط محافظت‌شده یکسان است. شکل Go برای checkout کد و ابزار سنجاق‌شده
است. تصویر برنامه فقط سرور را دارد؛ نگهداری را از میزبان اپراتور با ابزار
بومی PostgreSQL سازگار اجرا کنید.

عملیات فقط schema برنامه در `public` را با مهلت کلی ۱۵ دقیقه می‌گیرد. backup
قالب custom را می‌سازد، با `pg_restore --list` می‌سنجد، در صورت پشتیبانی mode
خصوصی می‌گذارد و بدون overwrite منتشر می‌کند. restore فقط روی DB کاملاً خالی
مجاز است؛ API یا کار maintenance دیگر نباید مالک target باشد. پس از restore،
ledger دقیق بررسی و sessionها و pre-auth لغو می‌شوند. Windows تضمین sync پوشهٔ
والد را نمی‌دهد.

## backup زنده

پوشهٔ خصوصی بساز. URL را از secret store یا محیط امن بگیر؛ در history ننویس.

```sh
install -d -m 0700 /var/backups/stackplore
export STACKPLORE_DATABASE_URL='postgresql://role@db.example:5432/stackplore?sslmode=verify-full&sslrootcert=/run/secrets/postgres-ca.crt'
go -C apps/api run ./cmd/storage backup \
  --destination /var/backups/stackplore/stackplore.dump
sha256sum /var/backups/stackplore/stackplore.dump
```

PowerShell:

```powershell
$env:STACKPLORE_DATABASE_URL = 'postgresql://role@db.example:5432/stackplore?sslmode=verify-full&sslrootcert=C%3A%5CStackplore%5Cpostgres-ca.crt'
go -C apps/api run ./cmd/storage backup --destination 'C:\Stackplore-backups\stackplore.dump'
Get-FileHash -Algorithm SHA256 'C:\Stackplore-backups\stackplore.dump'
```

checksum را جدا از آرشیو نگه دار. exit غیرصفر یعنی backup قابل اعتماد نیست.

## restore آفلاین

1. checksum را بررسی کن.
2. DB تازه و خالی با نقش کم‌مجوز بساز؛ جدول برنامه نساز.
3. همهٔ Stackploreهایی را که به DB target دسترسی دارند متوقف کن.
4. `STACKPLORE_DATABASE_URL` را فقط به target تازه بده.
5. ابزار storage همان release آرشیو را اجرا کن.

```sh
export STACKPLORE_DATABASE_URL='postgresql://restore_role@db.example:5432/stackplore_restore?sslmode=verify-full&sslrootcert=/run/secrets/postgres-ca.crt'
go -C apps/api run ./cmd/storage restore \
  --source /var/backups/stackplore/stackplore.dump
```

target نباید هیچ table پایه داشته باشد. از `--clean`، ویرایش ledger، کپی کلید
auth بین نصب‌ها یا restore روی DB production استفاده نکن.

## پیش از جابه‌جایی

یک API روی آدرس خصوصی و با DB بازیابی‌شده اجرا کن. `/healthz`، `/readyz`، ورود
با credential جاری، خواندن کاربر و ممیزی و چند query نماینده را بررسی کن. session
قدیمی نباید کار کند. سپس فقط پس از موفقیت، secret استقرار را کنترل‌شده تغییر بده.
backup رمزگذاری، نگهداری خارج از میزبان و backup خود server کار اپراتور است.

`ownership_unavailable` یعنی runtime دیگری را متوقف کن. `restore_rejected` یعنی
target، archive یا ابزار را بررسی کن. `migration_rejected` یعنی build هم‌نسخهٔ
آرشیو را اجرا کن و بعد طبق [روند upgrade](upgrade-rollback.fa.md) جلو برو.
جزئیات ledgerهای قدیمی در [مرجع انگلیسی](backup-restore.md) است.

</div>
