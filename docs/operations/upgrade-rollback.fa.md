<div dir="rtl">

# ارتقا و برگشت

دادهٔ SQLite نسخه‌های قدیمی توسعه، ورودی پشتیبانی‌شدهٔ runtime نیست. PostgreSQL
تنها پایگاه پشتیبانی‌شده است. هر migration جلو می‌رود؛ اجرای binary قدیمی روی
schema جدید برگشت امن نیست.

## پیش از ارتقا

1. changelog، migration و تغییر قرارداد نسخهٔ مقصد را بخوان.
2. tag و checksum artifact را بررسی کن؛ checksum و منشأ release باید پیش از ۱٫۰
   کامل شود.
3. نسخهٔ فعلی، major PostgreSQL، تنظیمات استقرار و نگهداری runtime key را ثبت
   کن؛ مقدار secret را کپی نکن.
4. backup بومی PostgreSQL بگیر و آن را در DB خالی تازه restore و verify کن.
5. build مقصد را روی نسخهٔ بازیابی‌شده با migration، readiness، ورود، خواندن
   نمونه و لغو session پس از restore بیازما.
6. برای تنها API برنامهٔ rollout هماهنگ کن؛ دو runtime هم‌زمان روی یک DB اجرا
   نکن.

جزئیات migrationهای قدیمی `0.1.0` تا `0.5.0` و checksumها در
[مرجع انگلیسی](upgrade-rollback.md) است. ledger تغییرناپذیر است؛ آن را برای رد
کردن کنترل upgrade ویرایش نکن.

## اجرا و بررسی

API قبلی را drain و متوقف کن، نسخهٔ بررسی‌شده را نصب کن و probeها، ورود، کاربران،
قابلیت منبع، خواندن‌های اصلی و ممیزی/اعلان را بررسی کن. volumeهای PostgreSQL و
runtime key را نگه دار. تعریف استقرار قبلی و backup را تا پایان پنجرهٔ برگشت حفظ
کن. Helm rollback یا image قدیمی schema را عقب نمی‌برد.

در `0.7.0` chart یک Go app با UI embedشده نصب می‌کند؛ API یک replica می‌ماند و
Ingress به همان Service می‌رسد. Compose نیز یک application service دارد. مهاجرت
از نسخه‌ای با Nginx جدا باید در محیط آزمایشی و با backup تازه تمرین شود؛ تغییر
chart تنها proof اجرای target نیست.

## برگشت

اگر نسخهٔ تازه با دادهٔ جدید سازگار نیست، برنامه را offline نگه دار. backup پیش
از upgrade را به DB خالی تازه برگردان و با build قدیمی خودش verify کن؛ سپس secret
استقرار را کنترل‌شده عوض کن. نوشته‌های پس از زمان backup از دست می‌روند؛ اثرش را
پیش از cutover بسنج. DB تازه/ناموفق و آرشیو قبلی را در روند اپراتور نگه دار.

Restore همهٔ session و pre-auth را لغو می‌کند؛ کاربر باید دوباره وارد شود.
پاک‌کردن DB یا آرشیو قدیمی بخشی از فرمان برنامه نیست. مدرک هر upgrade/rollback
روی target جدا از تست محلی ثبت می‌شود.

</div>
