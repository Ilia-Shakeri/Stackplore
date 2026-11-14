# Alpha8.3 — actual application workload and soak candidate

Candidate version **2.0.0-alpha.8.3**, schema22. No acceptance result is recorded
until exact-source GitHub Actions receipts exist. No test, build or service runs
on the workstation. Runtime product defaults, limits and authority stay unchanged.

## Load

The dedicated `stage9-workload` workflow requires a hosted runner. Its load job
keeps the complete pinned real processor and both-language administrator/viewer
desktop journeys. On the English administrator incident it additionally executes
the existing release workload: ten rounds of eight parallel reads, 600-ms minimum
round spacing, 80 reads for each of idle and admitted-processing phases. Actual
source-scoped queries come from the UI; every envelope must remain valid and every
response200. No rate-limit changes or response interception are used.

The observer records safe wall-clock start and duration metadata for actual
generation. Acceptance requires exactly one completed generation overlapping the
busy batch, no generation overlapping idle, exact source, all160 reads, no missing
samples and at most10% p95 regression. The ratio is recomputed from observations.
This compares an enabled but idle profile with one admitted real generation on
the same application/worker. **It does not satisfy feature-off/on deployment
comparison**, all-profile load coverage, production capacity or repeated statistical
performance acceptance. A threshold failure remains failed.

## Real read and interface soak

The independent soak job preserves the actual application, PostgreSQL and pinned
processor for at least60 elapsed minutes. After normal English viewer journeys,
100 scheduled cycles reopen and clear the interface, assert no retained reply,
perform80 actual scoped reads, check application and processor health, then return
to the resource page. Expected totals:8000 successful workload reads,200 health
checks and100 cleared/reopened views. Sessions remain under their normal sliding
idle and absolute limits. Generated answers are exercised by the preceding normal
journeys, not repeated throughout the hour. This is **read/interface soak**, not
active-generation, cancel/reconnect/process-restart soak or a pilot.

Every30 seconds the owning browser supervisor samples only its application child
and the processor child supplied by the owning parent. It records RSS, high-water
RSS, open-descriptor counts, active/waiting database connection counts, queued/
active runs and uncertain usage counts. No PID, SQL text, database name, credential,
question, reply or resource identity enters these receipts. The hour requires at
least120 ordered samples spanning3600 seconds, application peak at most512 MiB,
processor peak at most8 GiB and at most256 descriptors per process. Settled-window
RSS growth above64 MiB or descriptor growth above32 fails. Final queue, active runs
and uncertain usage must be zero. These are CI limits, not customer sizing or a
proof of zero leaks. Cleanup still uses the existing owned-process/database fence.

The soak collector preserves the load ratio even if it fails; its soak result is
separate from load acceptance. The separate load job enforces the ratio. Failure
reports preserve partial cycle counts. Hosted source, eight-job verification,
security/package, exact-tag and publication gates remain separate.

## Self-correction

Initial boundary review:8/10. Required corrections: prove actual overlap; retain
all denominators and failures; guard execution before I/O; bound duration, reads,
memory and descriptors; keep source, load, soak, pilot and semantic acceptance
separate. These are implemented in the candidate. Fresh hosted tests and code
review are required before assigning a bounded9/10. This is not an infrastructure
or stable-release score.

## فارسی

این نسخه سنجشِ بار و یک ساعت دوامِ برنامهٔ واقعی را روی میزبان آماده می‌کند.
صد نوبت خواندن و بازوبسته‌کردن، هشت هزار خواندن و دویست بررسی سلامت لازم است.
آزمون بار باید هم‌زمان با پردازشِ واقعی باشد و افتِ صدک۹۵ بیشتر از۱۰٪ نباشد.
داومِ پردازشِ پیوسته، لغو، اتصال دوباره و راه‌اندازی دوباره هنوز جدا هستند.
داوریِ مستقل، پایلوت و پذیرشِ پایدار با این تست‌ها ساخته نمی‌شوند.
