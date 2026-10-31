# Smart Assistant — private run read boundary

Development version: **2.0.0-alpha.4**, **2026-10-28**, schema **18**.

The HTTP library accepts an optional trusted `Options.AssistantRuns` capability.
When supplied, it registers these existing contract paths:

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/v1/assistant/runs/{run_id}` | Owner metadata or one complete freshly authorized answer |
| POST | `/api/v1/assistant/runs/{run_id}/cancel` | Owner cancellation; metadata receipt only |
| GET | `/api/v1/assistant/runs/{run_id}/citations/{citation_id}` | One retained selected-resource locator |

The server supplies this capability only through an enabled, validated
[application runtime](application-runtime.md). The default remains off; guest
composition rejects it. The [admission capability](conversation-admission.md)
adds conditional public request handling. The [status subscription](status-server.md) is supplied only with its additional
trusted metadata capability. The bilingual view remains unfinished. Earlier library receipts cover their exact
source; later startup composition has separate acceptance.

## Authority and request limits

Read routes use the existing session authentication boundary. Cancellation also
requires the existing exact origin and CSRF checks. The transport constructs an
actor only from consistent authenticated principal/session metadata and a copy
of its keyed digest. It clears the temporary digest bytes; raw session IDs or
browser-supplied actor/profile fields never become run authority.

All three routes reject queries, request bodies, transfer-encoded bodies and
malformed IDs. They delegate to the existing service rather than reading saved
text or bypassing its current SQL/session/source/profile/policy/evidence checks.
The returned owner and run ID must still match. Missing/denied private resources
share a fixed 404. Storage faults and malformed upstream results expose fixed
problems without diagnostic strings. Existing request, concurrency, body and
response bounds continue to apply.

## Explicit publication

The durable run projection whitelists only the contract metadata. It omits actor
digests, owners, lease/fence values, profile/configuration details, reservation
internals and native identity. It preserves all eight states and both usage
flags. Active retained input is `pending`; completed known-absent content is
`volatile_lost`; unavailable publication does not invent a loss or permit a rerun.

Ready content requires completed retained output, exactly one matching
selected-resource citation, matching source/resource/range and authority epochs,
closed safe sections and an independent 64 KiB serialized-answer cap. Source
time ranges retain SQL microsecond precision; provider observation timestamps
may retain valid UTC nanoseconds. Answer/citation slices are detached.

Ordinary formatting, structured logging and JSON serialization of the private
reply are protected. Only the explicit response writer converts the validated
projection to its wire representation. Every private response is `no-store`.
Citation reads re-enter the same fresh service boundary; they return no answer,
native object ID, URL or browser-selected path. Cancellation returns no content
and can erase retained text without rewriting an already-completed durable state.

The processor and transport share decoded-answer validation. Strict JSON output
also rejects unpaired UTF-16 escapes before the standard decoder can silently
repair them. Valid paired escapes and literal escaped technical text retain
their meaning. This is structural validation, not a factual-quality claim.

## Evidence boundaries

The synthetic [shared publication fixture](../../contracts/fixtures/assistant-ready.json)
is checked by both Go publication tests and the TypeScript reply parser.
Transport checks cover authentication, origin/CSRF, wrong owners/IDs, malformed
or oversized output, absent retention, generic logging/serialization and default
off/guest rejection. The SQL integration extends the existing production HTTP
processor adapter fixture with real run authority, replay, citation, cancellation,
account-revision revocation, one dispatch and absence of private content from
owned SQL tables and captured logs.

Authentication is **synthetic in that combined SQL/transport fixture**, and the
processor is a bounded synthetic loopback server. These checks do not prove a
real user login/browser conversation, deployed telemetry, a running processor,
new-package branch acceptance or human-reviewed grounding. Existing real
authentication integration has separate coverage. Application startup has
separate lifecycle evidence; the full browser journey and later roadmap
acceptance remain required.

## فارسی

کتابخانهٔ HTTP سه مسیر خواندن، لغو و ارجاع دارد؛ فقط با وابستگیِ مورد اعتماد
وصل شوند. ورودی اصلی فقط با تنظیم درست آن را وصل کند؛ پیش‌فرض بسته است. حالت
مهمان این وابستگی را رد کند. ساخت اجرا و اتصال اصلی افزوده شود؛ جریان سمت
سرور شاهد جدا دارد؛ صفحهٔ دوزبانه مانده است.

کاربر از نشست موجود شناخته شود؛ لغو، مبدأ درست و CSRF هم بخواهد. مالک و
شناسهٔ همان کار دوباره سنجیده شوند. سؤال، بدنه، پارامتر اضافی، تنظیم پردازنده
و digest مرورگر پذیرفته نشوند. خطا متن خصوصی ندهد. پاسخ کامل فقط بعد از
اختیار تازه و بررسی متن و ارجاع بیرون رود؛ ارجاع URL یا نام بومی ندهد.

چاپ، لاگ و JSON عمومی، متن پاسخ را پنهان کنند؛ فقط نویسندهٔ پاسخ اجازهٔ
تبدیل صریح به قالب شبکه را دارد. لغو می‌تواند متن را پاک کند و حالت تکمیل‌شده
را درست نگه دارد. یونیکد شکسته پیش از ترمیم خودکار رد شود.

شاهد مشترک، قالب دو سمت را بسنجد. تست ترکیبی، پایگاه واقعی و پردازنده و
احراز هویت آزمایشی دارد؛ اثبات ورود واقعی، مرورگر، موتور واقعی یا کیفیت
معنایی نیست. اتصال برنامه شاهد جدا دارد؛ گیت‌های باقی‌مانده هنوز لازم‌اند.
