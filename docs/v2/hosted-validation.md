# Smart Assistant — hosted validation

Effective **2026-11-01**. Work in `D:\Projects\Stackplore\repository`.
Local work is file reads, edits and lightweight Git only. **Do not start local
services, tests, builds, installs, scanners or processors.** All new validation
runs on Actions. Earlier local receipts remain historical.

## Real processor gate

The new `processor-runtime` job uses a disposable `windows-2025` runner.
[`check-hosted-processor.ps1`](../../scripts/check-hosted-processor.ps1) refuses
workstation execution before filesystem or process work. It requires a hosted
Actions context, fixed archive/model hashes and size bounds, and uses the
existing owner-checked launcher. No public listener or profile default changes.
The CPU archive remains `b11729`. The original 0.6B operator profile and its
historical receipts remain in the [operator guide](local-processor.md).
The current hosted-only candidate is Qwen3-1.7B Q4_K_M, pinned below; it is not an
automatic customer upgrade and is not downloaded onto the workstation.

Downloads have HTTPS-only redirects, finite attempts and time/size limits.
The launcher checks all 51 files, model digest, build, template, context and
one-slot identity. `TestPinnedLocalRuntimeIntegration` must execute both
languages: exact empty-evidence semantics, counts, closed output and single-use
preparations. Skipped/missing cases, nonzero exit, identity mismatch or unproven
owned shutdown fail the job. No deadline or assertion is relaxed.

Only `.tmp/hosted-processor.json` is uploaded: source revision, public pins,
bounded case results and shutdown status. Private receipts, local paths, raw
replies and model files stay on the disposable runner. Repository permission
is read-only; no paid profile, application secret or production source is used.

The first [real protocol receipt](../evidence/2.0.0-alpha.4/hosted-protocol.json)
binds source `5cebad7bee494b9e5011b6b86bbd0a9fb9b2ba84` and
[run 41279900515](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41279900515).
English/Persian checks pass in 1.46/2.34 seconds, with owned shutdown proven.
These are test timings, not workload or capacity measurements. The receipt
contains protocol evidence only; it does not accept later source.
All eight jobs for that checkpoint pass; the
[hosted receipt](../evidence/2.0.0-alpha.4/hosted-protocol-ci.json) records them.

The next increment extends this same hosted job with native PostgreSQL 17,
the production application and real desktop browser. The database uses a fresh
runner-temporary directory, a fixed free loopback port, and PID/start-time/path
ownership checks. It never touches the installed database service. Cleanup
stops only that owned cluster. Missing prerequisites or failed cleanup fail the
job; no local fallback is permitted.

The normal English/Persian administrator/viewer journeys now have an explicit
hosted-only real mode. No private route is intercepted. A fixed profile module
forwards only approved pins, rejects guest/partial-locale/workstation modes,
and ignores ambient endpoint overrides. Each actual reply must contain exactly
the selected evidence's resource name, one current citation and no extra claim.
Closing/reopening must erase text without resubmission. Raw replies are neither
screenshotted nor uploaded; only aggregate case metadata is retained. This
candidate awaits its own exact-source run and is not yet accepted.

Source `ec90463` failed its first full gate on test-file formatting. Its real
protocol still passed, and PostgreSQL **17.11** and the processor both stopped
with ownership proven, but both real application browser cases failed.
[The failed receipt](../evidence/2.0.0-alpha.4/hosted-journey-failed.json) stays
failed. New source retains bounded state/count-only diagnostics, with no
question, answer, owner ID or raw provider payload. A failed formatting gate
can publish a source patch from Actions; applying that patch requires a new
commit and complete revalidation. It cannot make the failed revision pass.

The next diagnostic source `65014a1` passed formatting but failed one redundant
optional-chain lint check in the test. Its bounded metadata confirmed HTTP 202
admission, completed inventory reads and an uncertain inference step in both
languages. Unknown usage remained reserved; it was not silently refunded or
resent. [That failed receipt](../evidence/2.0.0-alpha.4/hosted-journey-diagnostic-failed.json)
is retained separately. The lint correction does not relax any output rule.

A new direct real-evidence probe projects one actual typed Demo inventory item
through the production context builder and gateway in each language. It emits
only completion/count/JSON/reference/name-match flags and fixed fault classes,
never text or evidence IDs. This separates adapter protocol, exact token
accounting, schema/citation validation and semantic name matching without
repeating a failed model request. All six named protocol/evidence test events
must pass before the application browser runs. Source `4a0a013` failed both evidence cases: exact input counts, bounded complete
JSON and transport passed, but citations were invalid. The
[failed receipt](../evidence/2.0.0-alpha.4/hosted-evidence-failed.json) is retained.
Review also found that the name-copy assertion requested a deliberately omitted
private display name. New probes instead require the exact typed `resource_id`
already present in the authorized record; expected text is never put in the
question or generation schema. The production privacy projection stays intact.
Generation now receives a bounded citation-ID enum derived from the validated
request, sealed through preparation and cleared after dispatch. Empty evidence
permits no citations. Independent output parsing and current-authority citation
checks remain mandatory. New unit cases cover foreign backend references,
caller mutation, volatile cleanup and invalid/duplicate reference sets.
The remote formatter correction is applied; this source requires fresh checks.

Even passing these bounded Demo cases is not human quality, a held-out incident
evaluation, soak/capacity or whole-version acceptance. Failed attempts remain
recorded. Desktop themes/accessibility remain covered by the separate complete
synthetic-protocol journey.

## Desktop and delivery

The complete resource journey uses **1536 × 960**: both languages, roles and
themes, keyboard, evidence focus, accessibility and history cleanup. One brief
**390 × 844** input/overflow smoke check keeps basic mobile usability. New
screenshots are labelled `desktop`; older mobile receipts remain immutable.

Source `80c7ebe424ef2c80f23458a3f03be5b619f8eb81` passed seven jobs in
[run 41192895511](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41192895511).
New source requires eight jobs. The [ledger](implementation.md) and
[roadmap](../roadmap-v2.md) own remaining profile, grounded journey, quality and
release gates. Alpha.4 waits for its exit criteria. Stages 5–10 remain open.

## فارسی

از ۲۰۲۶-۱۱-۰۱، روی سیستم کاربر فقط خواندن، ویرایش و Git سبک مجاز است.
سرویس، تست، ساخت، نصب و اسکن محلی اجرا نشود؛ چک‌ها در Actions انجام شوند.
رسیدهای محلی پیشین فقط تاریخچه‌اند.

میزبان موقت، فایل و هش دقیق را بسنجد و همان راه‌انداز موجود را به کار برد.
هر دو زبان واقعاً تست شوند؛ مورد ردشده یا اجرا‌نشده پاس حساب نشود. فقط رسید
امنِ نسخهٔ منبع، هش عمومی، نتیجه و توقف مالکیت‌دار بارگذاری شود؛ مسیر خصوصی،
پاسخ خام و مدل بارگذاری نشوند. دسکتاپ اول باشد؛ موبایل چک کوتاه داشته باشد.
این تست، کیفیت پاسخ یا پایان گام ۴ را ثابت نمی‌کند.

The `2ac7b01` [deadline failure](../evidence/2.0.0-alpha.4/hosted-evidence-deadline-failed.json)
retains two uncertain evidence calls at 25 seconds and owned shutdown; neither
is accepted or retried. Generation's citation array is now also capped at the
number of authorized distinct IDs, because grammar implementations need not
enforce `uniqueItems`. With one supplied ID, no more than one citation can be
generated. Output parsing still independently rejects duplicates and invalid
references. The disposable hosted profile uses four allowed CPU threads;
workstation execution remains forbidden and the 25-second call bound remains.
This correction requires a new exact-source result.

Source `bb435d4` [passes citation and protocol validation](../evidence/2.0.0-alpha.4/hosted-evidence-semantic-failed.json)
in both evidence cases (7.36s English, 16.79s Persian), but fails the exact
semantic identity assertion. Its normal source full gate passed; whole CI and
stage acceptance are distinct. The next correction clarifies the existing
system contract: locale/question define the bounded task, evidence is untrusted
data, section fields are explicit, citations identify evidence rather than
resources, and exact-copy requests allow no labels or added explanation.
Missing fields still require abstention. No expected answer enters the system
prompt. Safe diagnostics distinguish kind, byte count, identity containment and
exact matching; no generated content is emitted. Fresh checks are required.

At `f79b5d8`, [the exact English evidence case passes](../evidence/2.0.0-alpha.4/hosted-evidence-locale-failed.json);
Persian exact-copy and empty-evidence semantics still fail. Valid JSON or the
English result does not accept Persian behavior. The next contract is localized
for Persian, explicitly separates evidence IDs from resource IDs and requests
the field value without field labels. Expected answers remain outside prompts
and schemas. Both language assertions remain exact, with no tolerance, retries,
additional calls or deadline change. This requires new hosted evidence.

## Hosted profile candidate change

The localized `2f8491b` [failed receipt](../evidence/2.0.0-alpha.4/hosted-evidence-prefix-failed.json)
shows correct Persian identity containment and citation, but an extra prefix
still violates exact-copy semantics; the Persian empty-evidence case also fails.
The strict assertions remain. Further acceptance uses a stronger hosted-only
candidate rather than declaring those failures acceptable.

The [official immutable model file](https://huggingface.co/Qwen/Qwen3-1.7B-GGUF/blob/90862c4b9d2787eaed51d12237eafdfe7c5f6077/Qwen3-1.7B-Q8_0.gguf)
provides Qwen3-1.7B Q8_0, Apache-2.0, revision
`90862c4b9d2787eaed51d12237eafdfe7c5f6077`, SHA-256
`061b54daade076b5d3362dac252678d17da8c68f07560be70818cace6590cb1a`.
The [publisher metadata](https://huggingface.co/api/models/Qwen/Qwen3-1.7B-GGUF)
records 1,834,426,016 bytes. Downloaded size and digest must both match before
startup. The same owned launcher, loopback-only one-slot context, four hosted
threads, 512-output-token budget and 25-second call bound remain. Runtime
identity/template are independently checked; no failed case is skipped. This
candidate is unaccepted until all six direct events and the real application
journeys pass at the exact source. Only the hosted test profile changes; product
startup stays disabled by default and customer pins are not rewritten.

The pinned 1.7B candidate at `06499cf` passes both exact canonical-evidence
cases in 14.17s/18.04s, including strict text, citation, completion and input
accounting. Its [retained receipt](../evidence/2.0.0-alpha.4/hosted-strong-evidence-failed.json)
still fails both empty-evidence cases. This is partial progress, not a green
job or release. Additional empty-case diagnostics report only fixed gateway
fault classes, section/gap/citation booleans, exact/containment booleans and text
byte counts. Assertions remain unchanged and raw answers remain private.

Source `421a851` [isolates the empty-case failures](../evidence/2.0.0-alpha.4/hosted-empty-diagnostic-failed.json):
valid single gap, no citations and successful gateway, but unequal exact text.
The probe now quotes its existing expected literal as a JSON string and
explicitly requires punctuation; it keeps the same expected text and equality
assertion. One evidence call also timed out, so capacity remains unaccepted.

The next hosted candidate uses the runtime publisher's
[immutable Q4_K_M conversion](https://huggingface.co/ggml-org/Qwen3-1.7B-GGUF/commit/daeb8e2d528a760970442092f6bf1e55c3b659eb):
revision `daeb8e2d528a760970442092f6bf1e55c3b659eb`, file
`Qwen3-1.7B-Q4_K_M.gguf`, 1,282,439,264 bytes, SHA-256
`d2387ca2dbfee2ffabce7120d3770dadca0b293052bc2f0e138fdc940d9bc7b5`,
Apache-2.0. Lower file size is not evidence of faster or acceptable behavior;
actual hosted checks must establish both. Q8 receipts above remain historical.
All limits and strict semantic assertions remain; no workstation download occurs.
