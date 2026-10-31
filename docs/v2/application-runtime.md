# Smart Assistant — application runtime ownership

Development **2.0.0-alpha.4**, schema **18**, 2026-10-28. The published alpha.3
does not contain this composition. This is not whole-stage acceptance.

## Enablement and authority

The main configuration loader registers the [closed operator profile](operator-profile.md)
from the same single environment inventory used for key validation. Profile
parsing performs no network/file access. A disabled profile builds no content
adapter, resolver or worker and injects no private routes. Existing metadata
maintenance remains independent. Guest mode rejects enabled profiles.

An enabled profile requires all current repository, approved provider and cached
read ports. Construction checks source identities and configured operator
profiles before publishing a handler. It does not install, launch, probe or
contact a processor. The existing adapter verifies exact processor identity
before content dispatch on an explicitly admitted run. Offline or mismatched
profiles never permit an unverified content request.

The durable profile ID hashes all identity pins, context size, generation, output
limit and call timeout. Profile/policy epochs bind both current admission and
retained reads. Policy/generation downgrades fail startup. Local composition
keeps paid execution off and preserves existing user/installation quotas;
it never resets usage or settles restored exposure. On a new pinned selection,
configuration changes once with an operator audit and compare-and-swap version.
Identical restarts do not rewrite the control row or repeat that audit. A
persisted stop on the same pinned profile remains stopped after restart. A
changed pinned selection is an explicit new operator configuration.

Purpose-separated keyed derivations from the existing installation key provide
stable conversation identities and a separate private fingerprint key. Copies
of the installation and derived key bytes are cleared after construction.
Each process receives a fresh opaque memory owner. No question or answer is
recovered after restart; old metadata cannot reconstitute private text.

## Current source and restore fences

The trusted resolver reads the current source configuration, matches its
approved provider/profile and resolves only a validated opaque resource through
the existing cached read port. It freezes the selected range and current
restore generation. This lookup has a five-second bound. Admission rechecks
all epochs atomically; the metadata read does not authorize restored usage.

One evidence reader retains its original seal and global concurrency limits.
Its trusted version advance is monotonic and limited to existing approved
sources. It cannot add/swap a provider boundary, lower an epoch, or renew old
evidence. New admissions can use current source versions without restarting;
old runs and citations remain tied to their original epochs. SQL authority and
live provider coverage still fence dispatch and replay.

## Lifecycle and verification

The application owns a fixed two-worker pool. Shutdown, ownership loss and
startup failure close admission, cancel content work, erase volatile retention,
clear keys and join producers before database close. Idle enablement alone
creates no run, step or reservation and makes no processor request. HTTP
readiness represents the application/database boundary, not processor quality
or a successful generated answer.

Unit checks cover default-off port isolation, incomplete/guest composition,
profile binding, idempotent reconciliation, retained stops, current resource
epochs and close waiting for cancelled repository work. Real PostgreSQL checks
run the actual application twice through readiness and joined shutdown, then
exercise a listener failure after helper creation. Control version/profile,
quotas and the single configuration audit survive restart. Run/step/usage
ledgers remain empty; the migration's one installation account retains zero
used/reserved units. A synthetic loopback endpoint receives zero requests.
The restore-generation getter independently observes a changed SQL fence and
does not write audit, usage or restore state.

These are application lifecycle and idle privacy checks. Separate admission
SQL/HTTP evidence uses a controlled processor and authentication facade. No
current real engine, complete authenticated browser conversation, accepted
branch measurement, human quality, soak/capacity or release artifact acceptance
is claimed. The [status subscription](status-server.md) has separate later checks; the
bilingual UI remains unfinished.

## فارسی

پیش‌فرض بسته است؛ حالت مهمان اجازهٔ فعال‌سازی ندهد. تنظیم کامل، اتصال اصلی
و مسیرهای خصوصی را بسازد، ولی پردازنده را نصب، شروع یا فراخوانی نکند. فقط
اجرای صریح، پس از بررسی هویت پردازنده، اجازهٔ خواندن و پاسخ داشته باشد.

شروع دوباره، سهمیه و مصرف را پاک نکند؛ تنظیم یکسان رسید تازه نسازد. توقف
ثبت‌شده با همان تنظیم حفظ شود. نسخهٔ قدیمی سیاست یا نسل پایین‌تر رد شود.
کلیدهای جدا از کلید نصب مشتق شوند و کپی‌ها پاک شوند. نسخهٔ فعلی منبع و نسل
بازیابی دوباره سنجیده شود؛ ارجاع کهنه با نسخهٔ تازه زنده نشود.

بستن برنامه، گم‌شدن مالکیت و خطای شروع، کارگر را لغو کند و تا پایان آن
صبر کند؛ بعد پایگاه بسته شود. تست با پایگاه واقعی، دو شروع و توقف و خطای
شنونده را بسنجد؛ موتور و گفت‌وگوی مرورگر و پذیرش کل مرحله هنوز اثبات نشده.
