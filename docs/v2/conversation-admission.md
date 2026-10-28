# Smart Assistant — explicit conversation admission

Development **2.0.0-alpha.4**, schema **18**, 2026-10-28.

The HTTP library registers `POST /api/v1/assistant/runs` only when its trusted
run dependency also implements `AssistantRunAdmitter`. A reader alone grants no
admission. The server entry point still supplies neither capability; this
increment does not register operator settings or enable application dispatch.

## Closed request and server authority

Admission requires the existing session, exact origin and CSRF checks. Queries,
duplicate nonce headers, noncanonical 64-character lowercase hexadecimal
idempotency keys, unsupported content types and extra request fields fail before
the use case. The JSON object contains exactly source ID, opaque resource ID,
selected window (5, 15, 60, 360 or 1440 minutes), locale (`en`/`fa`), question and
an optional nonempty conversation ID. Exact names, unique keys, scalar Unicode,
8 KiB question bytes and 64 KiB encoded body bounds apply. Escaped valid text
does not acquire a smaller arbitrary byte limit. Generic formatting and JSON
serialization protect the typed request.

The coordinator accepts only trusted repository/runtime/resource-resolver ports,
a fixed profile/policy/budget binding, clock and nonzero 32-byte key. Distinct
purpose-separated keyed hashes bind opaque run/conversation identities and the
idempotency digest to the authenticated owner and request nonce. The nonce,
question and answer never become durable fields or diagnostics. The browser
cannot supply a native identity, endpoint, configuration epoch or budget.

The server freezes a UTC microsecond time range. A retained retry reuses its
original range and conversation, checks selected resource/window consistency,
then enters the existing atomic admission/fingerprint boundary. A trusted
resolver must return precisely that source, resource, range and policy with
current configuration/restore epochs. Both lookup and resolution share a
five-second cancellation bound; late cancellation cannot reach admission.

## Retention, retry and lifecycle

Explicit continuation requires a live owned volatile conversation. Unknown,
foreign and expired conversation IDs share denial. Full owned history remains
the existing eight-turn quota. Presence of explicit continuation is included in
the private fingerprint, so adding it cannot silently change an original
request. Retained durable retries consume no new turn and cannot restore a lost
question. A row observed by the coordinator must still exist at atomic admission;
its disappearance cannot create a replacement run.

Acceptance returns only the validated metadata receipt with status 202 and
`Cache-Control: no-store`. It starts no worker itself. The existing worker owns
reservation, dispatch, fresh authority checks and settlement. Cancellation and
reads delegate through the same runtime. Coordinator close clears its own key;
the future application owner must cancel, close and join the runtime before
closing storage. This component does not claim that lifecycle integration.

`Store.GetAssistantSettings` reads the existing locked control metadata without
changing its version, audit rows, enablement or usage. It prepares idempotent
startup reconciliation; it does not itself configure or activate anything.

## Verification limits

Unit checks cover closed input, generated authority, frozen retries, resolver
swaps, cancellation, disappeared rows and private formatting. Core tests cover
atomic continuation ownership/expiry, retained retries and missing-row fencing.
The combined SQL/HTTP fixture exercises public admission, exact receipt replay,
changed-request conflict, one committed reservation/dispatch/settlement, answer
and citation reads, cancellation, SQL session-revision revocation, joined worker
shutdown and content absence from all owned fixture tables and captured logs.

That combined test uses synthetic authentication, resource selection and a
bounded loopback processor protocol. The database, coordinator, core worker,
HTTP transport and processor adapter are real code. This is not evidence of a
running engine, real cookie/browser journey, factual quality or whole-stage
acceptance. Startup, status connections, bilingual conversation, accepted branch
measurement and later roadmap gates remain open. No alpha.4 release tag exists.

## فارسی

ساخت اجرا فقط با وابستگی مورد اعتماد وصل شود؛ ورودی اصلی برنامه هنوز بسته
است. نشست، مبدأ، CSRF، کلید تکرار درست و بدنه محدود لازم است. کاربر فقط
منبع، شناسه پوشیده، بازه، زبان، سؤال و در صورت نیاز گفت‌وگوی موجود بدهد.
اختیار، تنظیم پردازنده، شناسه بومی و بودجه از مرورگر نیاید.

تکرار همان کلید، همان بازه و اجرا را نگه دارد. سؤال عوض شود، تعارض بدهد.
گفت‌وگوی ناشناس، بیگانه یا منقضی رد شود؛ تکرار متن گم‌شده را زنده نکند.
ردیف دیده‌شده اگر حذف شد، اجرای تازه نسازد. رسید فقط فراداده باشد. پرسش و
پاسخ و کلید خام در پایگاه و لاگ نرود. مالک برنامه باید کارگر را پیش از بستن
پایگاه متوقف کند و منتظر پایان آن بماند؛ این اتصال هنوز ساخته نشده است.

تست ترکیبی، پایگاه واقعی و احراز هویت و پردازنده آزمایشی دارد. راه‌اندازی
اصلی، جریان وضعیت، صفحه دوزبانه، موتور واقعی و پذیرش کل مرحله هنوز مانده‌اند.
