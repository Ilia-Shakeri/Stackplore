# Smart Assistant — local processor profile

Status: implementation preparation. The gateway and opt-in integration test are
present in the working tree; the application has no enabled profile or public
dispatch route from this document. Stage 1 remains open until a real pinned
process passes its own test. The active [ledger](implementation.md) records the
execution restriction and earlier probes.

## Operator trust and identity

The operator supplies a single-model immutable process. Verify the actual model
file checksum out of band, use `sha256-<64 lowercase hexadecimal digits>` as its
only model alias, and bind its exact build string and UTF-8 chat-template digest.
An alias or metadata response is an operator assertion, not cryptographic proof
that a process loaded the corresponding bytes. Keep the approved deployment,
network and model files under operator control. Router mode, dynamic model
loading and mutable global properties are outside this profile.

The prepared adapter accepts an operator-owned `Profile` with an ASCII code ID,
an origin, 1–16 exact approved IP addresses, the model alias, build, template
digest and a positive profile generation. The origin has no credentials, path,
query or fragment. Only loopback destinations may use HTTP; private network addresses
require TLS with normal hostname and certificate verification. Public,
link-local, multicast and metadata service destinations are rejected.

The browser supplies no endpoint, model path, network allowance or profile
credentials. The current adapter supports no remote or paid profile. A future
configuration layer must preserve these operator boundaries and fail closed on
an unavailable or changed profile.

## Count, reserve and dispatch

The use case must admit the run and reserve its call before any processor I/O.
The fixed protocol is:

1. Read `/v1/models` and `/props`; require one exact alias, build and template.
   Ignore unrelated processor metadata, including its model path.
2. Apply the processor's template to the fixed system message and bounded user
   envelope through `/apply-template`.
3. Use `/tokenize` with special-token handling to obtain the complete numeric
   token sequence. Check both 64 KiB of rendered input and 16,000 input tokens.
4. After durable admission and per-call reservation, use the same counted
   sequence for `/completion`, with the closed JSON schema, explicit output
   limit, temperature zero, no prompt cache and no raw stream. Recheck identity
   before dispatch. Check at most 4,096 output tokens and 64 KiB decoded output.
5. Accept only complete bounded sections and server-issued citation IDs.
   An uncertain dispatch is not automatically repeated or refunded.

Every request resolves and validates its destination again, pins the approved
address for that attempt, rejects redirects, ignores ambient proxies and caps
wire input at 1 MiB. Count and generation each have a maximum 25-second context;
the complete run has its own 120-second cap and renewable lease. No network I/O
belongs inside a database transaction. One prepared request is usable once,
including when a caller copies its wrapper.

Approve the process context size and memory on the actual machine before
acceptance; the configured context must cover the complete accepted input and
output budget. Application limits are bounds, not a hardware capacity claim.

These endpoint and token-sequence semantics follow the
[upstream server protocol](https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md).
Pin and verify the chosen build rather than assuming all versions behave alike.

## Real integration gate

Use an already-running, operator-approved disposable loopback process. The test
does not launch programs, download models or inspect infrastructure. Prepare the
repository's pinned Go tools, then set these test-only environment variables in
the private local shell:

| Variable                                    | Required value                                  |
| ------------------------------------------- | ----------------------------------------------- |
| `STACKPLORE_TEST_LOCAL_INFERENCE`           | Literal `true`                                  |
| `STACKPLORE_TEST_INFERENCE_ORIGIN`          | Literal loopback origin of the approved process |
| `STACKPLORE_TEST_INFERENCE_ALIAS`           | Independently verified checksum alias           |
| `STACKPLORE_TEST_INFERENCE_BUILD`           | Exact approved build string                     |
| `STACKPLORE_TEST_INFERENCE_TEMPLATE_SHA256` | Exact template digest                           |

Run `go -C apps/api test ./internal/provider/inference -run '^TestPinnedLocalRuntimeIntegration$' -count=1 -v`.
The test must actually run both English and Persian cases. A skipped test,
schema-only greeting, estimated token count or successful HTTP request cannot
close this gate. Keep raw answers and private endpoint details out of public
receipts; record source, profile pins, synthetic case IDs, token counts, outcomes
and duration. This minimal probe establishes protocol compatibility only;
grounding quality and the held-out evaluation remain separate gates.

Unsetting the opt-in leaves ordinary tests independent of an external process.
Configuration and integration credentials never enter an untrusted review job.

## فارسی — پروفایل محلی

وضع: کدِ آماده‌سازی؛ پروفایل پذیرفته و مسیر اجرای عمومی هنوز نداریم. اپراتور،
پردازندهٔ تک‌مدلیِ ثابت و checksum واقعی فایل را بسنجد. alias، build، hash قالب
و نسل پروفایل بسته شوند؛ پاسخ metadata به‌تنهایی، محتوای فایل را ثابت نکند.
مرورگر، مسیر فایل و آدرس ندهد. HTTP فقط loopback؛ شبکهٔ خصوصی با TLS معتبر.

قالب کامل ساخته، همان tokenها پیش از ارسال شمرده و هنگام اجرا فرستاده شوند.
۶۴ KiB و ۱۶٬۰۰۰ token ورودی، ۴٬۰۹۶ token و ۶۴ KiB خروجی، هر فراخوان ۲۵ ثانیه
و کل اجرا ۱۲۰ ثانیه سقف‌اند؛ توان سخت‌افزار نیستند. بودجه پیش از ارسال رزرو؛
درخواست نامعلوم خودکار تکرار یا آزاد نشود. تغییر هویت، redirect، مقصد نامجاز
و پاسخ ناسازگار، رد شوند. متن خام در پایگاه یا رسید عمومی نرود.

فرمان بالا فقط به پردازندهٔ مجازِ ازپیش‌اجراشده وصل شود. دو زبان واقعاً اجرا
شوند؛ skip و جواب سلام، مدرک پایان نیستند. این تست سازگاری قرارداد را بسنجد؛
کیفیت، حریم، ظرفیت و corpus بسته، گیت‌های جدا دارند. مسیر مجاز برای اجرای
پردازنده هنوز باز است؛ رد خودکارِ راه‌اندازی در رسید فعال حفظ شده است.
