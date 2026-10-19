> **UNACCEPTED development source — schema 18.** This branch is isolated preparation, identified by its own commit; it is not the released `v2.0.0-alpha.0` source at `eb86ab7554db555f023c1de72c3119acf101226a` (schema 17). Normal startup automatically applies migration 18 even while the new capability is disabled. Use only a fresh disposable test database. Do not point this source at an accepted or customer schema-17 database. Returning an older binary requires its verified backup/restore procedure. Existing receipts do not accept this branch. Real processor/profile and stage exit gates remain open. The configured HTTP log path now delegates to the shared policy; this branch also changes an existing path.

> **شاخهٔ توسعه، نپذیرفته — طرح پایگاه ۱۸.** این کد، آماده‌سازی جدا با کامیت خودش است؛ منبع منتشرشدهٔ `v2.0.0-alpha.0` همان `eb86ab7554db555f023c1de72c3119acf101226a` و طرح ۱۷ است. شروع عادی این شاخه، مهاجرت ۱۸ را خودکار اجرا کند؛ خاموشی قابلیت مانع آن نیست. فقط پایگاه تازه و دورریختنیِ تست به کار رود. به پایگاه پذیرفته یا مشتری با طرح ۱۷ وصل نشود. بازگشت باینری قدیمی، روش پشتیبان/بازیابی تأییدشده لازم دارد. رسید قدیمی، پذیرش این شاخه نیست. گیت پردازندهٔ واقعی و پایان گام‌ها باز است. مسیر HTTP گزارش نیز از سیاست مشترک استفاده کند؛ پس یک مسیر موجود هم تغییر کرده است.

# Smart Assistant — isolated implementation preparation

Status: working-tree preparation, not a completed stage or released capability.
The `2.0.0-alpha.0` candidate retains schema 17 and the existing product routes.
The separate files described here include proposed schema 18. Do not infer their
acceptance from the alpha.0 tests, version number or historical stable release.

## Prepared boundaries

- A neutral count/dispatch gateway and operator-bound local adapter check the
  complete rendered input, numeric tokens, one-use preparation, pinned identity,
  bounded network reads and closed cited output. [Profile contract](local-profile.md).
- Metadata-only runs, steps and status replay use real PostgreSQL transactions.
  Admission, idempotency, quotas, lease fencing, cancellation, recovery, unknown
  usage and audited reconciliation have typed repository methods. Proposed
  migration 18 leaves protected audit history independent of run retention.
- Shared effective log policy and typed evidence projection bound content,
  preserve source/time/version/quality and retain local citation references.
  Remote structured aliases are run-scoped; projection alone grants no access.
- The prepared use case supports one local, unpaid, single-resource explanation:
  one inventory read and one reserved count/dispatch. It has a fixed worker pool,
  bounded volatile memory, heartbeat, explicit cancellation and repeated authority
  checks. Conversation, incident analysis, retrieval and an investigation loop
  have not been implemented through this narrow path.

There is no enabled application profile, public helper endpoint, generated
transport change or user-facing conversation in this preparation. Normal product
roles and the existing guarded-operation gates are unchanged.

## Corrections and actual proof

Review fixed stale caller-time checks, missing per-call usage bounds, lost known
usage during reconciliation, configuration resurrection, caller-controlled paid
mode, short leases, unbounded retention fan-out and small-batch recovery
starvation. Conservative settlement remains distinguishable from observed use.
The use case also fixes an unbounded admission wait, replay overwriting a worker
version, a degenerate fingerprint key and authority denial suppressed during
completion. Read-only closure review scored the bounded preparation 9/10 after
the specific fixes; this is not a product acceptance score.

The final Linux concurrency run used Go 1.26.9, GCC 13.3 and an owned PostgreSQL
17.11 container. Six complete packages and the focused real SQL/use-case/upgrade
suite passed: **132 test/subtest pass events**, zero failures, exit 0. The sole
skip was the explicitly configured real-processor integration. The 34-file source
manifest stayed unchanged, SHA-256
`4d990879c1753acf93ae9b461b3fa7425bd972b705019e835fa5c1617edba3e1`.
Owned container and volume removal was verified. The earlier concurrency attempt
D was deliberately interrupted for the completion-race fix and is not passing
proof. Earlier receipts retain their own pre-fix scopes.

Native schema-18 backup/restore passed in **52.790 seconds**, including the
restore-generation paid-admission fence and preserved unknown-usage ledger.
Real SQL plus a synthetic gateway also proved reservation visibility from a
second transaction before count/generation, one cited result, observed use of
13 units, release of unused reservation, matching replay without redispatch and
a changed-request conflict. These probes establish repository/use-case behavior;
the synthetic gateway does not establish processor compatibility or answer quality.

The final preparation receipt is in the owned scratch directory
`runtime-race-20261019-e/receipt.json`; the before/after source manifests and both
package/SQL logs remain there. It is separate from alpha.0's frozen source proof.
Future edits require new affected tests and source binding.

## Open gate

The maintainer confirmed that no compatible local process is already running.
Automatic review rejected starting the downloaded candidate binary with only
`blocked by policy`; no alternative launch path was used. Hashing the existing
local model file was a read-only check, not runtime acceptance. Existing greeting
probes do not establish exact pre-dispatch counting, grounding or hardware limits.

Stage 1 needs an approved real profile and an actually executed integration test.
Subsequent stages still need their own complete local, SQL, browser, deployment,
quality and release receipts. Keep this preparation disabled and separate from a
milestone claim until those gates pass.

## فارسی

این کد، آماده‌سازی جدا در درخت کار است؛ پایان گام یا قابلیت منتشرشده نیست.
نامزد `alpha.0` همان schema ۱۷ و مسیرهای فعلی را دارد؛ این بخش schema ۱۸ را
پیشنهاد کند. برای سنجش نسخهٔ پذیرفته، منبع دقیق همان نسخه به کار رود.

درگاه محدود، مخزن متادیتا، سیاست مشترک گزارش، شاهد نوع‌دار و یک شرح محلی با
یک خواندن آماده‌اند. گفت‌وگو، بررسی چندگامی، تحلیل رخداد و جست‌وجوی راهنما
هنوز از این مسیر ساخته نشده‌اند. متن خام فقط حافظهٔ محدود؛ مسیر عمومی خاموش.

شکاف‌های زمان، سهمیه، بازیابی و لغو هنگام پایان رفع شدند. تست رقابت با پایگاه
واقعی **۱۳۲ رویداد قبولی** و صفر شکست داشت؛ تنها تست پردازندهٔ واقعی skip شد.
بازیابی schema ۱۸ و تست مخزن همراه پردازندهٔ ساختگی هم پاس شدند؛ این‌ها مدرک
کیفیت جواب یا پذیرش پروفایل نیستند. امتیاز ۹/۱۰ فقط به همین کد محدود تعلق دارد.

کاربر، نبود سرویس سازگار را گفت. راه‌اندازی باینری تازه در بررسی خودکار رد شد؛
دلیل فقط `blocked by policy`. مسیر دور زدن اجرا نشد. گام ۱ و گام‌های پس از آن
تا مدرک واقعی خود باز بمانند؛ آماده‌سازی با پایان پروژه یکی نشود.
