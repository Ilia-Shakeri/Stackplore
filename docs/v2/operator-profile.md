# Smart Assistant — operator profile preparation

Status: **stage-4 development source, 2026-10-28; not registered in application
startup and not a delivered public capability**. Baseline source is
`21475ec1135c0496c5929329738142f3c0d5a9d0`. The standalone parser and its tests
are separate preparation. The released startup loader still rejects these
unregistered variables. Do not use this document as an enablement runbook.

The parser takes one bounded environment snapshot, uses no second lookup, and
performs no network or file access. Zero configuration and an explicit `false`
produce a disabled capability with no address or limits. Disabled profiles reject
latent pins. Unknown, duplicate, mixed-case, blank, noncanonical, invalid UTF-8
and over-bound values fail with fixed errors that omit values.

Every proposed key starts with `STACKPLORE_ASSISTANT_`:

| Suffix | Proposed contract |
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

Remaining composition work must bind the exact profile to audited durable
settings and the private service, fence generation changes and restoration,
keep the maintenance worker independent, and close owned workers on ownership
loss/shutdown. Disabled and guest modes must create no content client. Public
admission, owner-only replay, cancellation, citation routes, bounded status
streams and bilingual browser behavior need their own contract and real tests.

The [conditional HTTP read boundary](private-run-http.md) is separate library
composition; it does not register or enable these startup variables.

## فارسی

این سند فقط آماده‌سازی گام۴ است؛ متغیرها هنوز در شروع برنامه ثبت نیستند و
برنامهٔ منتشرشده آن‌ها را رد کند. نبود تنظیم یا `false`، مقصد و محدودیت فعال
نسازد. ورودی پنهان هنگام خاموشی، کلید ناشناخته/تکراری، مقصد بیرونی، مقدار
مبهم یا بزرگ رد شود. parser فقط یک snapshot بخواند؛ فایل یا شبکه نخواند.

هویت مدل، build، template، ظرفیت و generation از اپراتور بیاید. مرورگر آن‌ها
را تعیین نکند. مقصد این تنظیم فقط loopback صریح باشد؛ DNS، شبکهٔ خصوصی و remote
مجاز نیستند. خطا، JSON و قالب‌بندی معمول، مقدار pin را آشکار نکنند.

۷۱ رخداد تست/زیرتست محدود، کل تست بستهٔ config و vet پاس شدند. پوشش ثبت‌شده
statement است؛ اثبات شاخه، موتور واقعی، شروع برنامه یا گفت‌وگوی عمومی نیست.
اتصال runtime، اختیار تازه، stream محدود، قرارداد و مرورگر دوزبانه هنوز لازم.
