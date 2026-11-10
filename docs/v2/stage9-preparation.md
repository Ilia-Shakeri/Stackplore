# Stage 9 — deployment rehearsal preparation

Status: **preparation only**. Stage8 quality acceptance and an opted-in pilot are
not available. No beta or pilot acceptance is claimed. All maintainer execution
uses GitHub Actions; the workstation performs file editing and lightweight Git.

## Hosted lifecycle precursor

The separate `stage9-lifecycle` workflow runs eight service-boundary regressions
one hundred times each with race detection, two parallel tests, shuffle seed214
and a ten-minute test deadline. Its report requires all800 planned observations;
missing, failed, skipped, duplicate or incomplete results reject. Only fixed
names, counts, durations and the source revision are retained.

These use deterministic service dependencies, not a real processor or database.
They exercise active producer shutdown, concurrent cancellation, late admission,
ownership loss, expiry and revoked-user reads. They do not satisfy real process
restart, browser reconnect, installation, sixty-minute soak or pilot requirements.
The main eight-job gate still tests actual PostgreSQL, processor, desktop and
deployment. [Candidate receipt scope](../evidence/2.0.0-alpha.8.2/README.md).

## Entry gate

The release owner must inspect the complete reviewed corpus and actual independent
sign-offs, three held-out passes per supported pinned profile, security-critical
results, language/provider quality denominators, measured hardware limits and open
risks. Public seed labels are not a blind holdout. The corpus publication floor
is necessary and cannot establish complete eligibility. [Stage8](stage8-readiness.md).

## Rehearsal matrix

| Rehearsal | Required observations | Owning evidence |
| --- | --- | --- |
| Native, feature off/on | Exact downloaded binary and digest, startup, login, permissions, bounded profile and shutdown | Platform, source/artifact digest and independent runtime receipt |
| Compose and Helm, off/on | Persistent PostgreSQL, restricted networking, unavailable profile, quotas and stop switch | Exact hosted install runs plus opted-in pilot deployment receipt |
| Upgrade from 1.0.3 | Pre-upgrade native backup, migration history, interrupted boundaries, data and ownership fences | Real database upgrade and recovery receipts |
| Restore and rollback | Restore verification, revoked old sessions/generations, no uncertain usage refund | Independent restore receipt; rollback through the verified old backup where schema compatibility is absent |
| Lifecycle | 60-minute soak and 100 cancel/reconnect/restart cycles, no late answer/orphan/queue growth | Case counts, failures, owned cleanup, RSS and duration metadata |
| Workload | Same-source feature-off/on baseline, unchanged v1 workload, latency/RAM and database waits | Named hardware and absolute limits; at most 10% p95 regression |
| Pilot | Real desktop English/Persian journeys, operator responsibilities and incident response | Named environment, operator opt-in and independent sign-off |

## Evidence rules

Keep source verification, signed artifact publication, synthetic hosted rehearsal
and real customer/pilot acceptance separate. Record failures and actual denominators.
Use only synthetic data on hosted runners. Do not expose prompts, answers, credentials,
private service targets, account IDs or raw database plans in public metadata.
An external environment needs its own authorization; current permission covers
repository work and hosted tests. No environment or reviewer is invented to close
the matrix. [Full roadmap](../roadmap-v2.md), [recovery](../operations/backup-restore.md).

## فارسی

این سند آماده‌سازی گام ۹ است؛ آغاز یا پذیرش پایلوت نیست. نخست شرط‌های کیفیت
و امنیت گام ۸ باید مدرک داشته باشند. نصب، ارتقا، برگشت، یک ساعت دوام و صد
چرخهٔ عمر هر کدام رسید جدا می‌خواهند. پایلوت و ارزیاب فعلاً در دسترس نیستند.
همهٔ اجرای فعلی روی GitHub Actions است؛ اجرای محلی مجاز نیست.
