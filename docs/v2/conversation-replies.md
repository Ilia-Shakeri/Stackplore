# Smart Assistant — closed conversation replies

Development version: **2.0.0-alpha.4**, **2026-10-28**. This is a client boundary,
not a delivered conversation or permission to enable content routes.

`createAssistantClient` validates successful JSON replies before returning typed
data. Receipt, run-status and citation parsers are also exported for consumers
that already hold bounded decoded JSON. These parsers perform no network calls,
storage, retries, navigation, rendering or authorization.

- Accept only exact own data fields on plain objects, with dense bounded arrays.
  Reject missing/extra fields, symbols, accessors and unexpected prototypes.
- Bind status/cancel replies to the requested run ID and citation replies to
  the requested citation ID. Admission must retain the selected source,
  resource, duration and any explicitly supplied conversation ID.
- Require HTTP 202 for admission and 200 for reads/cancellation. Recheck caller
  cancellation before releasing a resolved reply; discard late content with a
  fixed abort error, without exposing the caller's cancellation reason.
- Preserve all eight durable states. `pending` requires queued/running;
  `volatile_lost` and `ready` require completed. `unavailable` does not imply
  text loss, a safe retry, or a free reservation.
- Validate canonical IDs, integer versions, flags, actual UTC calendar dates
  and ordering. Scope endpoints have at most microsecond precision; compare
  instants without millisecond truncation. Scope duration must match its
  selected window. Each citation must match the ready reply's exact source,
  resource and pinned range.
- Require 1–12 closed answer sections and exactly one selected-resource locator.
  Facts need a citation. Other sections may have none. Reject duplicate and
  fabricated references. Text must be nonblank, well-formed Unicode and at
  most 4096 UTF-8 bytes; reject angle brackets, NUL, Markdown link syntax and
  lowercase HTTP/HTTPS links as specified by the canonical contract. The
  serialized answer has an independent 64 KiB limit, including JSON escaping.
- Return detached copies after full validation. Failures contain only the fixed
  `unexpected_response` transport error; rejected text is not attached to it.
  Request validation also rejects Unicode whitespace-only questions.

The shared transport still bounds raw response bytes and enforces same-origin,
no-store, redirect rejection and fixed problem handling. A standalone parser
does not replace those transport bounds. Its input is decoded JSON or ordinary
data objects, not a sandbox for executable in-process Proxy objects.

The owning application must still enforce per-call deadlines, clear volatile
state on logout/account changes, validate responses against its retained scope,
render text only through inert text nodes, and build internal citation routes
from validated locators. An individually resolved citation has no run metadata;
its source/range must be checked against the retained run before navigation.
Fresh server-side owner/session/source/profile/policy checks remain mandatory.
No status stream, public handler, startup composition or bilingual view is
added by this boundary. Stage-4 browser and later roadmap gates remain open.

## فارسی

این بخش فقط مرز پاسخ کارخواه است. پاسخ کامل پیش از برگشت نوع‌دار بررسی
می‌شود: فیلد بسته، شناسهٔ همان کار، بازهٔ دقیق، ارجاع همان منبع، متن بی‌خطر و
حد بایت. متنِ پاسخ در خطا نمی‌آید. پاسخِ گم‌شده، ناموجود و در صف یکی نیستند؛
هیچ‌کدام اجازهٔ ارسال دوبارهٔ خودکار نمی‌دهند. متن و فهرست برگشتی کپی جدا دارند.

صفحه باید متن ساده نمایش دهد، با خروج یا عوض‌شدن کاربر حافظه را پاک کند، مهلت
هر درخواست را ببندد و ارجاع را با بازه و منبع همان کار بسنجد. اجازهٔ تازهٔ سمت
سرور لازم است. مسیر عمومی، جریان وضعیت، اتصال شروع برنامه و صفحهٔ دوزبانه
هنوز تحویل نشده‌اند؛ گام۴ کامل نیست.
