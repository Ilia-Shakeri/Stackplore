# Smart Analysis incident workflows

Status: stage-6 implementation candidate. A closed report read contract is under
hosted verification; no stage-6 release or quality acceptance exists yet.
Stage 5 must retain its own exact-source verification,
signed security trial and immutable publication. All execution takes place on
GitHub Actions; local work is limited to files and lightweight Git.

## Product behavior

The candidate uses explicit owner-only `GET /api/v1/assistant/runs/{run_id}/report`.
It reuses current run/evidence authorization without admitting another tool or
generation. Only an investigation can return report content. Pending/lost/
unavailable states carry metadata only. The ordinary answer contract stays
independent. The server recomputes typed report claims against exact current
references and checks report plus answer within one 64-KiB content budget.
Desktop controls and strict typed client parsing still need integration.

An operator selects one resource, one source and one fixed time window, then
explicitly requests an investigation. A desktop report must show:

1. The selected scope, observation times and coverage limits.
2. A bounded chronological view of actual observed symptoms.
3. A concise symptom summary with individually resolvable references.
4. Ranked possible explanations, explicitly qualified as hypotheses.
5. Supporting and contrary observations, missing evidence and safe next reads.
6. A concise incident report that never presents correlation as proof of cause.

The existing short explanation remains available. Logs stay unchecked by
default, local only, policy-filtered and limited to fifteen minutes. All reports
are volatile: permission loss, configuration or restore changes, logout,
cancellation and expiry erase them together with the owning conversation.
Historical metadata cannot recreate expired answers, reports or evidence.

## Architecture and authority

Extend the existing modular service. Do not introduce another engine, external
store, browser-selected query language or background autonomous execution.
Sealed evidence from the owning run is the only input to report projection.
Preserve separate proofs for each read family and reauthorize all retained
proofs before formatting, dispatch, publication, polling and citation resolution.

The first implementation slice should project immutable typed observations from
the existing context bundle. Do not reparse provider output in HTTP handlers or
the browser. Projection may expose only already approved scalar values, opaque
references, quality and actual observation/event/sample times. It must not expose
resource names, labels, raw event descriptions, credentials, arbitrary links or
log text. Sort by actual event/sample time with a stable tie break; inventory
observation time is not an inferred incident start time.

Incident correlation belongs in a pure package with a closed input vocabulary.
It must not import HTTP, SQL, a provider SDK or generation adapters. Its report
uses typed symptom, hypothesis, contrary-evidence and next-read codes plus exact
allowed citation IDs. Rank means deterministic presentation order, not a numeric
probability or proof of cause. Render known codes through English/Persian
catalogs. Generated text stays plain text in the existing bounded sections.
It cannot create an action, URL, command, query or additional read.

Existing metrics, health, events and topology cover only their measured domain.
They cannot stand in for alert history or desired/available replica history.
Add those correlations only through narrow typed repository ports, selected
resource/window predicates, current authority and separate durable reservations.
Source-wide alert pages must not be read and filtered in application memory.
Any added step kind needs a reviewed migration, strict domain/OpenAPI/client
updates, profile accounting, restore tests and query-plan evidence. Until those
ports exist, expose those families as missing; do not invent an alert or replica
observation from a generated sentence or a resource health flag.

## Failure taxonomy and interpretation

| Symptom family | Required observation | Disallowed inference |
| --- | --- | --- |
| Restart activity | Actual bounded restart samples or explicit typed event | A cumulative count alone proves a restart in the selected window |
| Memory pressure | Actual memory measurements with their unit and interval | Bytes used prove exhaustion without a measured limit |
| Termination/OOM | Explicit typed cause evidence | A warning event or restart alone proves OOM |
| Unhealthy resource | Actual health snapshot and its quality/time | A current state proves when the incident began |
| Failed workload | Actual desired/available replica history | A missing metrics response proves a workload failure |
| Connectivity | Actual bounded connectivity/provider authority result | Missing topology proves a network outage |
| Contradictory evidence | Distinct supporting and contrary references | Silently discard the later or inconvenient observation |
| Insufficient data | Unavailable/unsupported/partial family and coverage | Empty records prove absence of incidents |

Hypotheses must be grounded and explicitly provisional. No rule may overrule
permission denial or turn unavailable data into a healthy status. A positive
measurement of one resource does not authorize or explain an adjacent resource.
An observed recovery can be contrary evidence without disproving an earlier
failure. Bounds and limited sampling remain visible in every report.

## Bounds, security and retention

Retain the existing 120-second run limit, one final generation, current token and
byte caps, queue/concurrency limits and unknown-use no-retry rule. Report data
shares the same 64-KiB canonical output budget; it cannot create a second hidden
payload allowance. Use bounded arrays, closed fields, unique citation sets and
strict cross-scope/reference validation on both sides of the public contract.
Do not follow cursors, page recursively or adapt tool names to generated text.

Safe next reads are inert recommendations from a fixed vocabulary, with honest
capability/consent status. They never execute automatically. No report control
may bypass the ordinary fixed plan, quotas, source selection or consent fence.
Captured event/log instructions remain data. Default remote policy is unchanged.
No new diagnostic text, evidence body or generated report is stored in SQL,
browser storage, telemetry, traces, downloadable diagnostics or backups.

## Required hosted proof and release gates

Pure tests must cover contradictory chronology, identical timestamps, partial
or stale evidence, missing metrics, two possible causes, unsupported families,
run-local reference identity, stable sorting, exact caps, immutable detached
inputs and no executable next-read output. Test unsupported OOM and connectivity
claims explicitly. Every rule needs both a positive and a contrary/insufficient
case; translated text must not influence decisions.

Integration tests must use real PostgreSQL for new reservations, narrow alert
and replica queries, ownership, leases, upgrade/restore invalidation, retention
and query plans. Existing Docker/Kubernetes/Demo authorities must remain intact.
The real processor and bilingual desktop journeys must exercise the report,
same-scope citations, explicit consent, revocation and no late publication.
Keep desktop keyboard/RTL/accessibility checks and the existing small mobile
smoke check. Repeat all eight source jobs and signed package/security gates.

The release quality gate remains the independent held-out protocol in
[evaluation](evaluation.md) and the [roadmap](../roadmap-v2.md): 200 paired
scenarios, 1000 adversarial cases, three passes per accepted profile, exact
denominators, at least 95% factual grounding and correct abstention, 100% valid
authorized references and all critical cases passing. Independent human review
must cover the defined stratified sample and critical/disputed cases. Public
seed fixtures and deterministic report tests do not establish this acceptance.

## فارسی

وضعیت: طرحِ گام ۶؛ قابلیت و نسخه هنوز تحویل نشده. نخست گیت و اسکن و انتشارِ
گام ۵ لازم. تست، سرویس و بیلد فقط روی GitHub Actions؛ دسکتاپ اولویت دارد.

گزارش باید محدوده و زمان، نشانهٔ واقعی، فرضیهٔ موقت و رتبه‌دار، شاهدِ موافق
و مخالف، دادهٔ کم و خواندنِ امنِ بعدی را جدا نشان دهد. هم‌زمانی، علت را ثابت
نکند. شمارِ کلِ ری‌استارت، زمانِ ری‌استارت را ثابت نکند؛ مصرفِ حافظه بدون سقف،
کمبود را ثابت نکند؛ اخطار یا ری‌استارت به‌تنهایی، OOM را ثابت نکند.

فقط شاهدِ بسته‌شده به همان اجرای مجاز وارد گزارش شود. همهٔ proofها پیش از
نمایش و استناد دوباره سنجیده شوند. تاریخِ رخداد/نمونه از زمانِ مشاهده جدا
بماند. دادهٔ هشدار و تاریخچهٔ replica، مسیرِ خواندنِ نوع‌دار و محدودِ مستقل
لازم دارند؛ نبودشان صریح گفته شود. صفحهٔ هشدارِ کلِ منبع، راهِ میان‌بر نیست.

گزارش موقت است؛ از دست‌رفتن مجوز، تغییر تنظیم/بازیابی، خروج، لغو یا انقضا
همه‌اش را پاک کند. متن در SQL، storage مرورگر، telemetry یا backup نرود.
فرضیه و پیشنهاد، دکمهٔ اجرا یا فرمان نسازند. رضایتِ لاگ پیش‌فرض خاموش و فقط
محلی و حداکثر پانزده دقیقه است. سقفِ فعلیِ زمان، بایت، توکن و رزرو حفظ شود.

تستِ ساختگی و seed عمومی، کیفیتِ واقعی را ثابت نکنند. گیتِ holdout و بازبینیِ
انسانِ مستقل، جدا و باز بماند؛ رسیدِ یک منبع، منبعِ تازه را قبول نکند.
