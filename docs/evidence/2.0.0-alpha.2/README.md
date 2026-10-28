# Stage 2 — durable runtime checkpoint

## Published artifact acceptance — 2026-10-25

Immutable `v2.0.0-alpha.2` points to `2086b3c1ca22fbd484eb28e29471e61567efc10e`,
schema18. [Public acceptance](published-release.json) binds all seven main jobs,
all seven tag jobs, publication run40102848842, 39 public assets, 30 checksums,
31 provenance subjects and seven signed SBOMs. Eight signed publication scan
reports have no high/critical findings; the secret report is empty. This is a
review of publication reports, not a new vulnerability-database scan.

The independently executed **public Windows archive** passes startup, persisted
source/alert state, SSE replay after restart, actual native backup/restore,
revoked restored sessions, fresh login and nonzero shutdown on ownership loss.
Its SHA-256 is `1bb6f81e6ab81e78c91810290fa146b571b07ada3f031af279b023e9acc1a703`.
The signed Linux producer smoke is separate. Public OCI execution, current local
engine startup, new-package branch measurement, quality and capacity remain open.

Interrupted downloads are retained. The consumer now reuses only complete files
whose API length/hash match and resumes a pinned retained prefix into a **fresh**
destination, with exact HTTPS ranges and the final complete API/checksum digest.
No incomplete archive is extracted or executed. Failed consumer attempts remain
failed. Canonical working files now live under `D:\Projects\Stackplore`; old
receipt paths remain historical. Native checks use the canonical fixture identity.
The first native attempt failed during restart and its path-recording error
masked the startup failure; fresh attempts pass. No single failure cause is claimed.

گام۲ با منبع ثابت و طرح۱۸ منتشر شد. هفت کار شاخه و هفت کار تگ، امضاها و هشت
گزارش اسکن پاس؛ بستهٔ عمومی ویندوز هم واقعی اجرا و بازیابی شد. شکست‌ها حفظ
شدند. این شاهد، اسکن تازه با پایگاه آسیب‌پذیری تازه یا پذیرش کامل نسخه۲ نیست.

## Historical local candidate evidence

Date: **2026-10-24**. Candidate **2.0.0-alpha.2**, schema **18**.
[Published alpha.1](../2.0.0-alpha.1/published-release.json) remains immutable
at `1f4d46d5178b032fd5c3e7a2540cd7d2718d2698`, schema17. This checkpoint
adds durable metadata admission, quota/usage ledger, fencing, recovery, status
replay and maintenance inside the existing Go service. It exposes no public
question/dispatch endpoint and stores no raw question, answer or tool output.

[Focused evidence](focused-runtime.json) binds all twenty changed Go/SQL files
to tested tree `4b80e52b11db03a65a794c054c6289d9e6559e87`. Unit, real PostgreSQL and native-tool checks pass.
Three actual child-process crash checkpoints pass; dispatch is synthetic, while
process kill, blocked-commit rollback, reopening and state assertions are real.
The crash child entry is intentionally skipped in the parent test; that does
not skip its three launched cases. Windows excludes one platform-specific
credential-file test. Neither skip becomes a provider/runtime acceptance claim.

The schema17→18 upgrade preserves existing data and immutable migration history.
Actual native backup/restore preserves unknown usage and fences paid generation;
owned source/restore databases are removed and their absence checked. Seven
`EXPLAIN (ANALYZE, BUFFERS)` paths retain real batch/protected-state assertions
on 4,096 history runs and 7,168 steps. They are diagnostics, not a production
capacity guarantee; bounded deletion count does not bound scan work.

[Full local gate](local-gate.json) passes: **544 web, 40 client, 48 release and
11 corpus** cases, real PostgreSQL/native recovery and production build. Web
branch coverage remains **80.33%**, duration188.59s; it is not Go branch coverage.
The earlier preparatory server-unit failure remains failed. The new invocation
has a built embedded frontend, Go10 and an isolated fixture; no production or
unit assertion was changed, and no single earlier failure cause is established.

Scoped runtime review: **8 → fix → 9/10**. Reviewed changes include database-clock
authority, skipped locked candidates, heartbeat/version fencing, reservation
accounting, conservative/unknown usage, bounded retention, crash rollback and
shutdown composition. Source score and local passes do not replace exact-source
hosted concurrency, signed scans, public artifacts or the separate ≥90% new
runtime/policy branch requirement. Those gates remain open. Later evidence,
conversation and contract preparation stays outside this candidate.

## فارسی

نامزد گام۲، طرح۱۸: پذیرش و صف، سهمیه، اجاره، لغو، بازیابی و نگه‌داری متادیتا.
مسیر عمومی و ذخیرهٔ متن خام ندارد. منبع۲۰فایل جدا سنجیده شد. صف و رقابت و
سه قطع واقعی فرایند پاس؛ ارسال در تست مصنوعی بود. ارتقای۱۷به۱۸ و بازیابی
بومی واقعی پاس؛ پایگاه‌های خود تست پاک و نبودشان سنجیده شد. هفت طرح پرس‌وجو
بر۴۰۹۶کار و۷۱۶۸گام سنجیده شد؛ ادعای ظرفیت نیست.

گیت کامل پاس:۵۴۴وب،۴۰کلاینت،۴۸نشر،۱۱corpus و ساخت. پوشش شاخهٔ وب۸۰٫۳۳٪؛
پوشش شاخهٔ Go نیست. شکست قدیم حفظ؛ علت یگانه ثابت نشد. نمرهٔ همین برش۹/۱۰؛
CI، رقابت، اسکن و پذیرش نسخه و شرط شاخهٔ تازه هنوز جدا لازم.
