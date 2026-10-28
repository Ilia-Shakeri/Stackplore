# Smart Assistant — operator profile

Status: **stage-4 development source, 2026-10-28**. The development server now
registers the bounded profile and conditionally owns conversation admission,
reads, cancellation and citation routes. The public alpha.3 release at
`21475ec1135c0496c5929329738142f3c0d5a9d0` still rejects these variables.
This document describes newer source; it is not alpha.3 enablement guidance.
Status-server connections and the bilingual conversation remain unfinished.

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
| `CALL_TIMEOUT` | Optional canonical duration, 1 ms–25 s |

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
the owning UI remain separate implementation work.

The [application owner](application-runtime.md) binds exact pins and limits to
audited durable settings, preserves quotas and retained operator stops, reads
current restore/source epochs, and joins owned workers before closing storage.
Maintenance remains independent. Disabled profiles construct no content client;
guest mode rejects enablement. Configuration does not install, launch or contact
a processor. A compatible separately managed service and exact approved pins
are required for a real admitted run. Current engine and human-quality acceptance
remain open. The bounded client status reader opens one explicit connection;
the server subscription and bilingual browser journey still require delivery.

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
اتصال اصلی، قفل توقف و پایان کارگر ساخته شده؛ جریان سمت سرور و مرورگر
دوزبانه هنوز مانده. نصب یا شروع پردازنده از تنظیم برنامه انجام نشود.
