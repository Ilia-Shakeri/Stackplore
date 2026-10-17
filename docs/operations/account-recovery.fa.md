<div dir="rtl">

# راه‌اندازی نخست و بازیابی حساب

حساب یا گذرواژهٔ پیش‌فرض وجود ندارد. برای recovery مدیر، API باید خاموش باشد و
فرمان operator مالکیت انحصاری همان PostgreSQL را بگیرد.

در [بستهٔ بومی تأییدشده](release.fa.md)، مسیر `./stackplore-operator` یا در
Windows مسیر `.\stackplore-operator.exe` جای پیشوند
`go -C apps/api run ./cmd/operator` مثال‌ها می‌آید. آرگومان و محیط محافظت‌شده
همان است. تصویر برنامه فقط سرور دارد؛ بازیابی آفلاین از میزبان اپراتور انجام شود.

## راه‌اندازی روی loopback

origin پیش‌فرض فقط loopback است. برنامه را اجرا کن، `/setup` را باز کن و مدیر
نخست را بساز. تراکنش فقط وقتی جدول کاربر خالی است موفق می‌شود و setup را برای
همیشه می‌بندد؛ دو درخواست هم‌زمان، دو مدیر نخست نمی‌سازند.

## نصب بیرون loopback

proof تصادفی ۳۲ بایتی با base64url بدون padding و expiry UTC حداکثر یک ساعت لازم
است؛ ۱۵ دقیقه پیشنهاد می‌شود. مقدارها را در محیط shell بگذار، نه command، فایل
`.env` یا history. PowerShell:

```powershell
$proofBytes = New-Object byte[] 32
$proofRng = [Security.Cryptography.RandomNumberGenerator]::Create()
try { $proofRng.GetBytes($proofBytes) } finally { $proofRng.Dispose() }
$env:STACKPLORE_SETUP_BOOTSTRAP_PROOF = [Convert]::ToBase64String($proofBytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
$env:STACKPLORE_SETUP_BOOTSTRAP_EXPIRES_AT = [DateTime]::UtcNow.AddMinutes(15).ToString("yyyy-MM-dd'T'HH:mm:ss'Z'", [Globalization.CultureInfo]::InvariantCulture)
[Array]::Clear($proofBytes, 0, $proofBytes.Length)
Remove-Variable proofBytes, proofRng
```

POSIX با Node نسخهٔ پین‌شده:

```sh
export STACKPLORE_SETUP_BOOTSTRAP_PROOF="$(node -e 'process.stdout.write(require("node:crypto").randomBytes(32).toString("base64url"))')"
export STACKPLORE_SETUP_BOOTSTRAP_EXPIRES_AT="$(node -e 'const d=new Date(Date.now()+15*60*1000); d.setUTCMilliseconds(0); process.stdout.write(d.toISOString().replace(".000Z","Z"))')"
```

سرویس را در همان environment اجرا کن و proof را فقط در فیلد مربوط به operator در
`/setup` وارد کن. مقدار فقط digest می‌شود؛ در DB، log، خطا و audit ذخیره نمی‌شود.
proof نادرست، مصرف‌شده، منقضی یا غایب رد می‌شود. پس از setup، هر دو متغیر را از
محیط پاک کن و سرویس را دوباره بساز.

PowerShell:

```powershell
Remove-Item Env:STACKPLORE_SETUP_BOOTSTRAP_PROOF -ErrorAction SilentlyContinue
Remove-Item Env:STACKPLORE_SETUP_BOOTSTRAP_EXPIRES_AT -ErrorAction SilentlyContinue
```

POSIX:

```sh
unset STACKPLORE_SETUP_BOOTSTRAP_PROOF STACKPLORE_SETUP_BOOTSTRAP_EXPIRES_AT
```

## بازیابی آفلاین مدیر

1. API را متوقف کن؛ فرمان کنار runtime دیگر یا restore اجرا نمی‌شود.
2. همان DB URL، timeout، schema و data directory برنامه را در محیط operator بگذار.
3. login دقیق را بررسی کن؛ اشتباه ممکن است مدیر دیگری بسازد.
4. گذرواژهٔ تازه را فقط از stdin بده؛ در argument، environment، فایل یا history
   نگذار.

نمونهٔ PowerShell با prompt مخفی:

```powershell
$recoverySecret = Read-Host 'New administrator password' -AsSecureString
$recoveryCredential = [PSCredential]::new('local-recovery', $recoverySecret)
$previousOutputEncoding = $OutputEncoding
try {
  $OutputEncoding = New-Object System.Text.UTF8Encoding($false)
  $recoveryCredential.GetNetworkCredential().Password |
    go -C apps/api run ./cmd/operator recover-admin --login-name operator --display-name "Primary Operator"
  $operatorExitCode = $LASTEXITCODE
} finally {
  $OutputEncoding = $previousOutputEncoding
  Remove-Variable recoveryCredential, recoverySecret, previousOutputEncoding
}
if ($operatorExitCode -ne 0) { throw "Administrator recovery failed with exit code $operatorExitCode" }
```

گذرواژه ۱۲ تا ۱۲۸ بایت UTF-8 باشد. موفقیت باید دقیقاً
`administrator recovery complete` دهد. برای login موجود، حساب فعال و مدیر می‌شود،
گذرواژه و revision عوض می‌شود و sessionها و grantها لغو می‌شوند. سپس سرویس را
راه‌اندازی کن، `/readyz` را بررسی و با حساب بازیابی‌شده وارد شو؛ گذرواژهٔ موقت
را عوض کن. [مرجع کامل انگلیسی](account-recovery.md) محدودیت‌های POSIX و جزئیات را دارد.

</div>
