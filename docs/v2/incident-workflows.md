# Smart Analysis incident workflows

History integration candidate **2.0.0-alpha.6.2** extends the earlier report core.
[Selected history behavior](incident-history.md) specifies independently reserved
reads, typed history fields, readable-empty coverage and bilingual witness focus.
Exact-source hosted checks and stage-6 quality acceptance remain pending.

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
The desktop candidate exposes an explicit report control after an investigation.
Its scrollable typed table shows occurrence/sample time separately from observed
time, family, scalar facts, freshness/partial/synthetic/redaction flags and fresh
citation controls. Qualified possibilities show supporting and contrary references;
gaps and next reads remain inert text. Both languages use the shared theme.
Closing the report aborts its read and drops local content. Owner/lifetime/scope
loss clears the owning conversation; late noncooperating replies are fenced.
Strict typed client parsing and these controls need fresh complete hosted proof.

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

### Selected alert-history port preparation

The candidate `IncidentAlertHistoryReader` selects actual transitions by source,
provider, exact current resource identity and a half-open window of at most 24
hours. It returns at most eight records with explicit partial/more coverage and
ordinal time/ID ordering. The adapter uses a bounded repeatable-read read-only
snapshot and SQL predicates, not source-wide pages or application filtering.
An additional-record indication is valid only for a full bounded page marked
partial; a shorter page cannot claim more records.
Current source policy denial produces empty metadata. Callers still need current
actor/source/resource authority and a separately reserved typed read proof.
Stale resource status marks the returned history stale, including a limited page.

Transition state and occurrence time are historical facts. Current alert severity
and rule kind can change after a transition and are deliberately absent. Actor,
acknowledging identity, private reason codes and prose are also absent. The last
provider collection time is not substituted for a later local transition time.
The new port is not yet connected to the fixed plan or public timeline; no new
read authority, schema migration or alert-correlation acceptance is implied.

The candidate workload-history port applies the same resource, time and record
bounds. It checks the exact current resource reference in the same read-only
snapshot and reports partial coverage when an extra row exists. Historical
sample time and desired/ready counts remain separate from current replica state.
Unavailable current collection or policy loss returns no history. The port
grants no authority; the owning candidate service now reserves and seals its read.
Stale current replica data marks the result stale even if source metadata is fresh.

The pure report candidate can correlate typed history when an owning caller
supplies readable-family coverage. Empty readable history stays distinct from
an unread family; unavailable individual samples contain no replica counts.
The shortfall hypothesis is qualified and cites an actual fresh sample; a
later recovery is contrary evidence and conflicting same-time values suppress
the hypothesis. This pure function is not a read proof. The alpha.6.2 report
candidate consumes these records only with independently supplied coverage from
the owning, freshly authorized sealed proofs. Legacy validation still rejects
nonempty history coverage. Empty proofs retain their requested read family so a
different empty family cannot substitute for a reserved history read.

### Existing execution limits

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

وضعیت: گام ۵ با نسخهٔ `alpha.5` منتشر شده است. گام ۶ در دست ساخت است؛
هستهٔ گزارش و تبدیل تاریخچه تست شده‌اند، ولی اتصال خواندن مجاز و رابط
هنوز کامل نشده و `alpha.6` منتشر نشده است. تست، سرویس و بیلد فقط روی
GitHub Actions؛ دسکتاپ اولویت دارد.

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
