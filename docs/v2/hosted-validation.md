# Smart Assistant — hosted validation

Effective **2026-11-01**. Work in `D:\Projects\Stackplore\repository`.
Local work is file reads, edits and lightweight Git only. **Do not start local
services, tests, builds, installs, scanners or processors.** All new validation
runs on Actions. Earlier local receipts remain historical.

## Real processor gate

**Current budget amendment:** [ADR-0017](../adr/0017-profile-call-budgets.md)
keeps the default 25 seconds and permits explicit profiles up to 40 seconds.
The current Linux candidate explicitly uses 40s in both direct and application
paths. Its receipt records the budget. Every 25-second failure below retains
its original criterion; none is relabelled passed. Whole-stage acceptance is open.

The current `processor-runtime` job uses disposable `ubuntu-24.04` compute.
[`check-hosted-processor-linux.mjs`](../../scripts/check-hosted-processor-linux.mjs)
refuses workstation execution before filesystem, network or process operations.
It verifies the exact CPU archive and model digest/size, bounds archive paths
and link targets, checks the free loopback listener and pins runtime/model/
template/context identity. Its owned child handle is stopped and joined; the
existing labelled PostgreSQL fixture is removed only after ownership checks.
The full real desktop journey remains 1536 × 960 in both locales and roles.
The separate Windows native launcher job and historical Windows receipts remain;
Linux compute does not establish Windows processor capacity or public artifacts.

The current archive is `llama-b11729-bin-ubuntu-x64.tar.gz`, 17,973,345 bytes,
SHA-256 `ef0db60597825a63534a536bd60d3c290cf82631b2a48461278dc2c9eb436c25`,
from the [pinned upstream release](https://github.com/ggml-org/llama.cpp/releases/tag/b11729).
The model is the official Qwen3-1.7B Q8 revision/hash/size recorded below.
Product defaults and original customer/operator profile pins remain unchanged.
No model is downloaded onto the workstation.

Downloads have HTTPS-only redirects, finite attempts and time/size limits.
The original Windows launcher checks 51 files; the Linux path checks the pinned archive, bounded extraction, model digest, build, template, context and
one-slot identity. `TestPinnedLocalRuntimeIntegration` must execute both
languages: exact empty-evidence semantics, counts, closed output and single-use
preparations. Skipped/missing cases, nonzero exit, identity mismatch or unproven
owned shutdown fail the job. Strict JSON, citation and semantic assertions remain. The explicit time-budget amendment above is recorded separately.

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

The original Windows increment extended this hosted job with native PostgreSQL 17,
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

The Q4 candidate at `b039068` [remains unaccepted](../evidence/2.0.0-alpha.4/hosted-quantized-failed.json):
English exact-copy cases pass; Persian gaps include the requested literal with
extra text, and the Persian resource identity differs. Its actual template hash
is `87a2728cb8dc9fe424d624542f6060ec05a1d285ebbec578bb078900e33396b5`;
this does not match the application profile and must not be silently substituted.
The current candidate returns to the official Q8 revision/hash/size recorded
above, whose exact bilingual evidence cases already passed at `06499cf`.
Generic prefix/suffix/punctuation examples clarify the task contract without
embedding any expected probe answer or resource value. The unchanged assertions,
budgets and complete real-application gate still require fresh acceptance.

Source `8b49f29` [passes both exact empty-evidence cases](../evidence/2.0.0-alpha.4/hosted-literal-deadline-failed.json),
but both evidence calls hit the unchanged 25-second bound. The protocol source
at the pinned revision shows `/apply-template` uses the same initialized chat
parameters, including the launcher's disabled thinking mode
([upstream handler](https://github.com/ggml-org/llama.cpp/blob/412a363c655b479279d3050d0d1c67a5733697b4/tools/server/server-context.cpp#L5195)).
Missing inheritance is therefore not an established defect and no speculative
protocol flag is added. The next candidate removes redundant contract prose
while preserving trust, locale, citation, abstention and exact-copy rules. It
compares two allowed hosted threads and records runner logical-processor count.
The model pins, schema, budgets, no-retry policy and semantic assertions stay
unchanged. Successful execution and bounded latency still require real receipts.

The short contract at `73601cd` [passes both literal gap cases and exact English evidence](../evidence/2.0.0-alpha.4/hosted-thread-comparison-failed.json).
The two-thread candidate still times out on Persian evidence. The receipt
confirms four logical processors; the next candidate uses all four allowed
threads with the shorter contract. This is a profile comparison, not proof of
capacity. The 25-second ceiling and every exact assertion remain unchanged.

## Compute boundary correction

The last Windows candidate `9d8c4b6`
([run 41286894288](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41286894288))
[passes both exact gap literals but times out on both evidence calls](../evidence/2.0.0-alpha.4/hosted-windows-capacity-failed.json).
Four logical processors and four threads do not establish bounded capacity.
The new Linux compute path is unaccepted until its own full result. No deadline,
expected text, citation rule, language or role is removed. Windows capacity stays
explicitly failed; a Linux receipt must not be labelled a Windows success.
Pure guard/archive-path tests run in the source gate; imports do no I/O. The
script publishes only metadata. Neither generated answers nor browser DOM,
private paths, credential URLs, model files or raw logs are uploaded. Fixture
cleanup also has an independent always-run workflow step. Existing Windows
scripts are preserved for their recorded scope; none runs on the workstation.

## Explicit CPU budget correction

The first Linux source `e29e1c2`
([run 41287638018](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41287638018))
[still fails the original 25-second Persian evidence deadline](../evidence/2.0.0-alpha.4/hosted-linux-budget-failed.json).
Exact gaps and English evidence pass; both model and database are stopped.
Its source formatting failure is corrected using the Actions-generated patch.
The new script also rejects unsafe link targets before extraction and keeps
fixture credentials out of the processor environment.

The current candidate explicitly requests the new bounded 40-second profile,
not the unchanged 25-second default. Configuration and adapter tests verify
both defaults and the hard ceiling. The gateway and parent run remain bounded;
120-second overall timeout, strict output, usage reservations, cancellation,
heartbeats and authority fencing remain mandatory. This is an explicit policy
change, not acceptance of the old failed timing gate. New real results are pending.

The 40-second candidate `9bd9042`
([run 41288259353](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41288259353))
[passes all six strict direct events](../evidence/2.0.0-alpha.4/hosted-desktop-focus-failed.json).
Exact grounded English/Persian cases take 23.14/28.99 seconds. The actual
administrator journeys publish correct ready answers in both languages, with
completed durable runs and reconciled known usage. The browser then reads focus
immediately after the asynchronous citation click and fails before viewers run.
The correction waits at most the ordinary five-second assertion budget for the
same exact focus, emitting only a fixed error and citation HTTP status. It does
not replace the authority request, mutate focus, or relax the expected target.
Three Node global lint names are made explicit. New source requires all gates.
Model and database cleanup remain proven in the failed receipt.

## First complete real desktop result

Source `1e21b3695970f54185244925d2cd5096dcb861e8`
([run 41288750885](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41288750885))
[passes the real processor/application job](../evidence/2.0.0-alpha.4/hosted-desktop.json):
all six direct events, exact English/Persian evidence in 14.48/17.34 seconds,
and both actual administrator/viewer journeys per locale. Each admits once,
completes with ready evidence-supported content, resolves its citation with
HTTP 200 and correct focus, clears private text and does not resend. Database
metadata confirms known reconciled usage, one tool and one inference per run.
The owned model and patched database fixture stop. This is the explicit
40-second Linux profile; these timings do not establish p95, Windows capacity,
held-out quality, remote templates or customer deployment. Complete source CI
and whole-stage/release acceptance remain separate.

All eight jobs for `1e21b36` now pass in the same run; the
[complete CI receipt](../evidence/2.0.0-alpha.4/hosted-desktop-ci.json) includes
the full source/build, race, synthetic browser/offline, installation/provider
and real processor gates. The pre-tag review separately found the release guard
still required exactly seven jobs. New source requires all eight, explicitly
including `processor-runtime`, with missing/skipped/failed/incomplete regression
cases. Extra and duplicate jobs remain denied. This correction requires fresh
checks; a green parent revision is not acceptance of newer release code.
