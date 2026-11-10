# Alpha8.2 — repeated service lifecycle preparation

Tagged checkpoint **2.0.0-alpha.8.2**, schema22. Immutable tag
`v2.0.0-alpha.8.2` identifies source `01b8cfc70ff4b181866c2c391ae8fa931834d452`.
No workstation test, build, scanner or service is run.

- [Complete source verification 42784537380](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42784537380): all eight jobs pass, including source/build/tests, race detection, PostgreSQL, desktop/offline, real processor, providers and deployment.
- [Three development passes and aggregation 42784649757](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42784649757): [safe summary](source-evaluation.json) records 72/72 structurally complete, zero rejected/missing/not-run cases, attempted p95 28,592 ms, 24,705 validated input and 11,606 validated output tokens. Human grounding and correct abstention remain pending; observed gaps are not a correctness score.
- [Fault receipt](source-hardening.json): all nineteen named cases, 72 malformed-output subcases, three native crash boundaries, seven query-plan paths and bounded fuzz pass.
- [Processor and desktop receipt](source-processor.json): all twelve application generations complete, with full English/Persian role, report, current witness, guidance, usage and cleanup assertions. The sampled Persian incident inputs are 1,116 tokens, compared with 1,226 in the failed baseline. Call limits remain 40 seconds. Different workers do not establish a same-hardware latency improvement.
- [Lifecycle verification 42784654914](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42784654914): [safe receipt](source-lifecycle.json) records 800/800 successful service-boundary observations, zero failed/skipped; race detection, two parallel tests, 16,620 ms including compilation. This is deterministic dependency testing, not a real processor soak or pilot.
- [Signed package/security trial 42784659255](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42784659255): package, strict scans and signing pass; branch publication correctly skips.
- [Exact-tag verification 42787205012](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42787205012) and [exact-tag lifecycle 42787204894](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42787204894): independent owning live receipts; source success cannot replace their results.
- [Signed publication 42787205097](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42787205097): owning live package/publication receipt; [public release location](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v2.0.0-alpha.8.2). Independent downloaded-asset and customer acceptance remain separate.

Owned processor high-water RSS is 3,211,612,160 bytes and process CPU is 1,109.49
seconds for the complete protocol/application receipt on a four-logical-CPU
AMD EPYC 9V74 worker. These measurements exclude other processes and are not a
production minimum or same-hardware idle/busy workload comparison. Seed-worker
measurements remain separate in the development summary.

Tag/publication links expose their own current results. Documentation follow-ups
have their own hosted gate and never move the immutable tag. A public prerelease
does not close stage8 quality, stage9 pilot or stable2 acceptance.

First source `dae2adf` passes [800/800 lifecycle observations](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42779994761)
with race detection. [Safe receipt](first-lifecycle.json). Its
[source gate](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42779994666)
finds formatting of the two new scripts before heavy code tests. The exact hosted
formatting patch is applied; a fresh complete source gate remains mandatory.

The separate `stage9-lifecycle` workflow repeats eight existing service-boundary
regressions one hundred times each, with the Go race detector, two parallel tests,
a recorded deterministic shuffle seed and a ten-minute test deadline. Each named
case must have exactly one hundred starts and successful terminal events. Missing,
duplicate, failed, skipped or incomplete observations reject. Package-level race
failure rejects even if individual cases claim success. Reports contain fixed
names and counters; test output, questions, answers and identifiers are omitted.

The eight cases cover shutdown joining an active producer, concurrent close and
cancel, a late admission commit, explicit cancellation, pending-admission cleanup,
ownership loss, memory expiry and revoked-user reads. Dependencies are deterministic
service fixtures. **800 planned observations are not 800 real installations or
processor calls.** This does not establish browser reconnect, real process restart,
database recovery, a sixty-minute soak or customer/pilot acceptance.

The complete existing eight-job source gate and signed package/security checks
remain separate and unchanged. The new workflow supplements rather than substitutes
for real processor, desktop, provider, deployment and PostgreSQL checks. No product
permission, deadline, schema or processing profile changes in this increment.

## Real runtime failure and correction

Source `af93c95` passes [three development passes and aggregation](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42780229616)
and the [signed package/security trial](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42780235770),
but its [real application gate](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42780177276)
fails. [Failed processor receipt](attempt-af93-processor.json),
[development receipt](attempt-af93-evaluation.json). The Persian administrator
incident dispatch counts 1,226 input tokens and closes at 39,998 ms without complete
generation; the owning run correctly remains uncertain. It is not a successful
answer or a status-follow defect. Other successful checks cannot override it.

[ADR-0024](../../adr/0024-compact-observation-reference-metadata.md) removes only
redundant owning citation metadata from complete known observation envelopes on
the private counted wire, using the already-bound outer handle. All facts and
original caller evidence remain intact; guidance keeps its existing projection.
Mixed/unfamiliar/ambiguous groups fall back untouched. Final canonical expansion,
strict reference validation, no retry and all byte/token/time budgets remain.
New tests cover reversible private binding, unchanged facts/nested IDs, guidance,
ambiguous full-group fallback and unissued-handle rejection. Fresh complete source,
real application and development measurements are required after the correction.

Source `43f9e54`, [42782527566](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42782527566),
keeps its [failed receipt](attempt-43f9-processor.json). Both locales and all
owning desktop actions complete; actual incident generations stay within 40 seconds.
The English administrator incident's DevTools body read fails even though the UI,
typed report, fresh witness and SQL run all complete. The receipt correctly fails.
Replace that fragile passive body read with a separately bounded, current-owner
snapshot GET after UI completion, parsed by the production client parser. Keep
exact identity, coverage, one admission, report, witness and cleanup assertions.
The real SQL synthetic-wire fixture also still expected the old embedded canonical
ID; update only that private-wire expectation to the bound handle, retaining final
canonical answer, resource ownership, reservations and exact usage assertions.
Fresh complete hosted receipts remain required; neither failure is hidden.

## Correction loop

Initial review: **8/10 within lifecycle receipt collection**. Repeated runs need
exact denominators, nested-failure handling and output privacy before the checker
can serve as evidence. Those fixes are implemented. Formatted source
`af93c952c8ede8ee22e1ede602006dab5c07edab` passes the four receipt-contract tests
and [800/800 repeated race-enabled observations](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42780177199)
in 20,409 ms including test compilation. [Safe receipt](development-af93-lifecycle.json).
Follow-up review: **9/10 within lifecycle receipt collection**. No skipped or
failed observations are hidden; raw test output is never projected into receipts.
This score cannot certify the whole infrastructure, semantic quality or stage8 exit.

## Roadmap position

[Published alpha8.1](../2.0.0-alpha.8.1/README.md) has 72/72 structural development
observations and nineteen named fault regressions. Independent reviewed corpus,
sealed holdout, human grounding/correct-abstention judgment, same-hardware helper-load
comparison and risk acceptance remain open. [Stage9 preparation](../../v2/stage9-preparation.md)
can proceed without claiming its entry gate, a pilot or stable2 acceptance.

## فارسی

این نسخه تستِ تکراری مرزهای چرخهٔ عمر را آماده می‌کند: هشت تست، هر یک صد بار،
با کشف تداخل و شمارش کامل. اجرای واقعی فقط روی GitHub Actions است. این تست‌ها
جای تست پردازشگر واقعی، اتصال دوبارهٔ مرورگر، راه‌اندازی دوبارهٔ فرایند، یک ساعت
دوام یا پایلوت را نمی‌گیرند. داوری مستقل و شرط‌های پذیرش نسخهٔ پایدار هنوز بازند.
