# Smart Assistant — operator profile

Status: **stage-4 development source, 2026-10-28**. The development server now
registers the bounded profile and conditionally owns conversation admission,
reads, cancellation and citation routes. The public alpha.3 release at
`21475ec1135c0496c5929329738142f3c0d5a9d0` still rejects these variables.
This document describes newer source; it is not alpha.3 enablement guidance.
The status server has [separate bounds and evidence](status-server.md); the
owning [resource conversation](resource-conversation.md) has separate implementation and acceptance gates.

The parser takes one bounded environment snapshot, uses no second lookup, and
performs no network or file access. Zero configuration and an explicit `false`
produce a disabled capability with no address or limits. Disabled profiles reject
latent pins. Unknown, duplicate, mixed-case, blank, noncanonical, invalid UTF-8
and over-bound values fail with fixed errors that omit values.

Every key starts with `STACKPLORE_ASSISTANT_`:

| Suffix | Contract |
| --- | --- |
| `ENABLED` | Exact `true` or `false`; default disabled |
| `ORIGIN` | Exact HTTP/HTTPS origin with a canonical literal loopback address; no DNS name, credentials, path, query, fragment or IPv6 zone |
| `MODEL_SHA256` | Nonzero lowercase 64-digit hexadecimal identity pin |
| `BUILD` | Exact printable ASCII build pin, 1–256 bytes, no surrounding whitespace |
| `TEMPLATE_SHA256` | Nonzero lowercase 64-digit hexadecimal UTF-8 template digest |
| `CONTEXT_TOKENS` | Canonical decimal, 20,096–65,536; exact server context pin |
| `GENERATION` | Canonical decimal, 1–9,007,199,254,740,991 |
| `OUTPUT_TOKENS` | Optional downward bound, 1–4,096 |
| `CALL_TIMEOUT` | Optional canonical duration, 1 ms–40 s; default 25 s |

Only the last two values have enabled-profile defaults. All identity pins are
required when enabled. Limits retain the existing 64 KiB/16,000 input-token,
64 KiB output, three inference-call, twelve tool-call and 120-second run caps.
The returned trusted local profile is a copy. Formatting and ordinary JSON hide
its pins; trusted composition must still avoid logging individual fields.

This initial configuration surface intentionally permits loopback only. The
lower-level adapter's independently reviewed private-TLS capability does not
authorize a LAN or remote environment setting. No browser value chooses an
origin, model file, trust root or network allowance.

Focused parser checks produced 71 passing test/subtest events. The complete
configuration package tests and vet also passed on the isolated baseline plus
these two files. Focused statement measurement gives the parser 98.5%; the other
profile helpers have 100% measured statements. These are statement results,
not accepted branch coverage, real processor proof or startup integration.

The accompanying typed request constructor adds explicit admission, private
metadata reads, cancellation and citation requests through the existing protected
same-origin client. It accepts closed scope fields and canonical opaque IDs,
limits question UTF-8 bytes, rejects malformed Unicode/accessors/extra fields,
and ignores route overrides in read options. Already-aborted work is never sent.
Fresh random idempotency keys are generated only on explicit calls; retries keep
the exact key and input. It performs no automatic resend or browser persistence.
All 52 client tests, strict types and targeted lint pass after the fixture and
abort-boundary fixes. The owning flow must supply per-call deadlines and validate
the closed reply before rendering it. Response validation, status streaming and
the owning UI have separate development source and evidence; these earlier
parser/client measurements do not certify those later bytes.

The [application owner](application-runtime.md) binds exact pins and limits to
audited durable settings, preserves quotas and retained operator stops, reads
current restore/source epochs, and joins owned workers before closing storage.
Maintenance remains independent. Disabled profiles construct no content client;
guest mode rejects enablement. Configuration does not install, launch or contact
a processor. A compatible separately managed service and exact approved pins
are required for a real admitted run. Current engine and human-quality acceptance
remain open. The bounded client status reader opens one explicit connection;
the server subscription has separate checks and the bilingual browser journey
has separate source verification. Current processor, remaining profile behavior
and whole-stage acceptance remain open.

The [private HTTP boundary](private-run-http.md) and
[admission coordinator](conversation-admission.md) retain their own earlier
source receipts. New application composition requires its own complete gates.

## فارسی

این سند برای کد در حال توسعه گام۴ است؛ متغیرها اکنون ثبت شوند، ولی نسخهٔ
منتشرشدهٔ alpha.3 آن‌ها را رد کند. نبود تنظیم یا `false`، مقصد و محدودیت فعال
نسازد. ورودی پنهان هنگام خاموشی، کلید ناشناخته/تکراری، مقصد بیرونی، مقدار
مبهم یا بزرگ رد شود. parser فقط یک snapshot بخواند؛ فایل یا شبکه نخواند.

هویت مدل، build، template، ظرفیت و generation از اپراتور بیاید. مرورگر آن‌ها
را تعیین نکند. مقصد این تنظیم فقط loopback صریح باشد؛ DNS، شبکهٔ خصوصی و remote
مجاز نیستند. خطا، JSON و قالب‌بندی معمول، مقدار pin را آشکار نکنند.

۷۱ رخداد تست/زیرتست محدود، کل تست بستهٔ config و vet پاس شدند. پوشش ثبت‌شده
statement است؛ اثبات شاخه، موتور واقعی، شروع برنامه یا گفت‌وگوی عمومی نیست.
اتصال اصلی، قفل توقف و پایان کارگر ساخته شده؛ جریان سمت سرور شاهد جدا دارد؛ مرورگر
دوزبانه هنوز مانده. نصب یا شروع پردازنده از تنظیم برنامه انجام نشود.


The explicit 40-second ceiling follows [ADR-0017](../adr/0017-profile-call-budgets.md).
Defaults stay 25 seconds. A CPU operator selecting 40s must provision a server
HTTP timeout beyond that value; the hosted candidate uses 45 seconds. Token,
worker, queue, authority and 120-second run bounds stay unchanged. Old 25-second
failures are not accepted under the new profile. Actual capacity remains measured.

## Hosted-verified development profile

The [real desktop receipt](../evidence/2.0.0-alpha.4/hosted-desktop.json)
verifies this explicit development profile at source `1e21b36` on disposable
Linux compute. It is not a customer hardware or always-on service acceptance.
The model is not bundled, installed or started by the application. Provision a
separate owned process on the application host, bound only to loopback, using
the [pinned archive/model and launch policy](hosted-validation.md).
Use one slot, context 20480, four threads, no cache, disabled reasoning, no
web interface, no tools, no downloads after startup and a 45-second server
HTTP timeout. Do not expose the processor listener to the browser or network.
The hosted script is intentionally forbidden outside its disposable runner;
its checks and immutable pins document the development profile, not an installer.

```dotenv
STACKPLORE_ASSISTANT_ENABLED=true
STACKPLORE_ASSISTANT_ORIGIN=http://127.0.0.1:58102
STACKPLORE_ASSISTANT_MODEL_SHA256=061b54daade076b5d3362dac252678d17da8c68f07560be70818cace6590cb1a
STACKPLORE_ASSISTANT_BUILD=b11729-412a363c6
STACKPLORE_ASSISTANT_TEMPLATE_SHA256=57f1fd00f0013a2be96aa79b857391f27e23df5b5f847072b524c897e24d0361
STACKPLORE_ASSISTANT_CONTEXT_TOKENS=20480
STACKPLORE_ASSISTANT_GENERATION=1
STACKPLORE_ASSISTANT_OUTPUT_TOKENS=512
STACKPLORE_ASSISTANT_CALL_TIMEOUT=40s
```

Enable only in normal authenticated mode; guest composition is rejected.
Changing identity or limits changes the trusted configuration binding. Opening
or typing in the view does not dispatch. Pre-dispatch counting rejects unavailable or mismatched processors. Failures
after dispatch retain uncertain usage and are never automatically repeated.
A container deployment must deliberately share the application's loopback
namespace with its owned processor; a public or arbitrary container DNS origin
is not accepted by this configuration parser. Default deployment stays disabled.

این نمونه فقط در میزبان موقت لینوکس سنجیده شده است؛ نصب‌کننده یا تضمین ظرفیت
مشتری نیست. برنامه مدل را نصب یا شروع نمی‌کند. مقصد فقط loopback همان میزبانِ
برنامه است؛ مرورگر به سرویس مدل وصل نشود. فعال‌سازی فقط برای حالت عادیِ دارای
ورود است. در این گفت‌وگوی توسعه، هیچ سرویس یا تستی روی سیستم کاربر اجرا نشود.
