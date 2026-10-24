# Smart Assistant — durable runtime candidate

[فارسی](runtime.fa.md) · [Active ledger](implementation.md) ·
[Accepted lifecycle decision](../adr/0015-helper-durable-runtime-and-volatile-context.md)

Status: **stage-2 candidate/preparation, not accepted delivery**. This document
describes the current metadata domain, PostgreSQL repository and standalone
maintenance worker. Fresh full-source local, concurrency, migration, recovery
and hosted receipts remain necessary. No production admission, public helper
HTTP route, text dispatcher, evidence collector or enabled processor follows
from these packages. Shared privacy and product conversation remain later
stage gates. The separate `assistantservice` experiment is not this stage-2
worker and does not establish its production composition.

**Database boundary:** accepted version 1.0.3 and the gateway-only checkpoint use
schema 17. This candidate adds migration **18**. A checkout containing that
migration applies it during normal startup even when the optional capability
is disabled. Use fresh disposable databases for unaccepted source; never attach
it to accepted or customer data as a way to test a disabled feature.

## Ownership and storage

Keep one Go service and one PostgreSQL store. The `assistant` package owns typed
goals, run states, step kinds, actor references and repository transactions.
`assistantworker` imports that domain and standard library only; it has no
provider, transport, context-builder or generation dependency.

Migration 18 adds six bounded metadata concerns:

| Table                      | Stored information                                                                                   |
| -------------------------- | ---------------------------------------------------------------------------------------------------- |
| `assistant_control`        | Default-off enablement, profile/policy/configuration versions, quota period, paid-generation fence   |
| `assistant_runs`           | Owner/session digest reference, typed goal/scope, versions, lease/fence, state, counters, timestamps |
| `assistant_steps`          | Fixed kind, numbered state, per-call reservation, fence and timing                                   |
| `assistant_status_events`  | Ordered typed run status and version; no answer text                                                 |
| `assistant_usage_accounts` | Installation/user used and reserved units                                                            |
| `assistant_usage_ledger`   | Held, unknown or settled reservations and safe receipt digests                                       |

No raw question, prompt, answer, log body, provider payload or conversation text
belongs in these tables. A keyed session digest is an internal authority
reference; raw session tokens and credentials never belong in runtime metadata.
Time is UTC with microsecond precision. Existing migration history is unchanged;
normal advisory locking and exact migration-ledger validation still apply.

Every significant repository operation receives a context and uses the existing
bounded pool/transaction boundary. PostgreSQL time determines current authority,
lease validity and durable transition timing; an old caller timestamp cannot
extend authority. No transaction spans a provider or processor request.

## Admission, state and fairness

`AdmitAssistant` atomically validates the actor, current session/auth revision,
source version, restore generation, profile/policy/configuration and usage
allowance, then inserts a run, reservation and first status event. The selected
source narrows an installation-wide role; it does not create tenant isolation.
Provider/observation authority still requires its own later use-case boundary.

Admission permits at most **16 queued runs per installation**, **two queued per
user**. Claims permit **two running globally** and **one per user**. Candidates
are ordered by creation time and ID, excluding owners already running, with a
16-row `FOR UPDATE ... SKIP LOCKED` selection. Shared control-row locking also
serializes accounting and configuration. This is bounded oldest-eligible
scheduling, not a strict starvation-free or multi-replica throughput guarantee.

| State or transition     | Meaning                                                                                                    |
| ----------------------- | ---------------------------------------------------------------------------------------------------------- |
| `queued` → `running`    | An authorized claim obtains a bounded lease and new fence                                                  |
| `running` → `completed` | Valid current lease commits accepted completion                                                            |
| `failed`                | Work ended without an accepted successful result                                                           |
| `cancelled`             | Explicit durable cancellation wins against late publication                                                |
| `expired`               | The run deadline elapsed                                                                                   |
| `interrupted`           | Required authority or volatile input is lost                                                               |
| `uncertain`             | A pending generation may have executed; no automatic resend                                                |
| `running` → `queued`    | Only a recoverable typed recipe with current authority, remaining deadline, capacity and attempt allowance |

All states except `queued` and `running` are terminal. A terminal cancellation
can still retain **unknown usage** if a generation step was pending; terminal
state is not evidence that no processing occurred.

Leases last **one to 30 seconds**, never beyond the **120-second run deadline**.
Transitions bind run version, lease owner, lease expiry and fencing generation.
Renewal cannot overwrite a newer step version. Recovery invalidates old leases;
an old worker cannot complete after cancellation, replacement, source/config
change or restore. At most three claims are allowed.

Volatile runs are bound to the originating process. A later process cannot
reconstruct a lost question. A claim with the wrong memory owner interrupts
that work; maintenance expires an otherwise still-queued run at its deadline.
An uncertain pending generation is never requeued for automatic dispatch.

## Idempotency and usage

Idempotency is owner-bound and includes the live actor plus material goal,
conversation, source/resource/time scope, restore/profile/policy versions and
keyed input fingerprint. A matching retained retry returns the existing run;
changed material conflicts. Unresolved ledger tombstones continue to reject
re-admission after ordinary run metadata expires. This is not exactly-once
external processing, and ordinary settled-history expiry is not indefinite
idempotency storage.

Reserve installation and user capacity atomically before accepting work. A
future dispatcher must call `BeginAssistantStep` before any corresponding
processor I/O, including remote counting, and must never hold the transaction
open during that I/O. The repository limits a run to **12 read steps**, **three
generation steps**, one pending step at a time, 64 status versions and at most
**1,000,000 reserved units**. User/installation configured quotas are positive,
bounded by 1,000,000,000 units and installation quota cannot be below user quota.
Units are accounting values, not a currency conversion or measured capacity.
Byte/token/field limits remain independent gateway and privacy requirements.

Confirmed use settles the observed amount and releases unused reservation.
Uncertain use retains the reservation with a known minimum; timeout is not zero
usage. There is no automatic retry or refund. A separately authorized new
attempt needs a new run and reservation, not a replay of the old submission.

The internal trusted operator ports are:

| Port                     | Required behavior                                                                                |
| ------------------------ | ------------------------------------------------------------------------------------------------ |
| `ConfigureAssistant`     | Version-check settings; changes fence existing authority                                         |
| `ResetAssistantQuota`    | Start a new accounting period without releasing unknown reservations                             |
| `SettleAssistantUsage`   | Lock the ledger; accept supported observed use or an explicitly labelled conservative settlement |
| `SettleAssistantRestore` | Bind reviewed exposure/receipt to the current restore generation                                 |

These are **composition/repository ports**, not shipped commands or HTTP
endpoints. They accept trusted operator audit metadata; that is not browser
authentication. Do not expose them without the proper operator boundary or
administrator, CSRF and purpose-bound recent-auth transaction. No command to
enable or reconcile this candidate is supplied by this guide.

Concurrent matching usage settlement is idempotent. Changed amount, receipt or
settlement kind conflicts. Observed settlement cannot fall below confirmed use
or exceed reservation; conservative settlement consumes the full exposure and
stays distinguishable from observed use. Ledger/account/status/audit changes
commit together, and a status-write fault rolls the transaction back.

## Maintenance, replay and protected lifecycle

The standalone worker performs one recovery and one retention pass on startup.
Defaults: recovery every **one second**, at most **18 changed runs**; retention
every **one minute**, at most **128 deleted rows in aggregate**; each call at
most **three seconds**. Both intervals are bounded between one second and one
minute. Calls are sequential with no backlog-drain or missed-tick burst.
Recovery inspects the bounded active set so a healthy head cannot hide expired
work. The repository's lower-level retention API caps a caller at 1,000 rows;
the production maintenance wrapper intentionally uses the tighter 128-row cap.

Construct maintenance independently of admission enablement. It has no claim or
dispatch port. The caller must run it with service ownership context, cancel on
ownership loss/shutdown, join `Run`, then close the store. It does not return
while its own context-aware storage call is still active.

`LastResult` exposes recovery/retention attempt time, count and a fixed outcome:
`not_started`, `ok`, `unavailable`, `cancelled`, `invalid_clock` or
`invalid_result`. Only `ok` has a confirmed count. Zero accompanying failure
does not prove that zero rows changed. Raw database errors are not retained;
transient failure remains visible and retries at the next scheduled pass.

Ordinary run/step and settled-ledger metadata have a **seven-day** lifecycle;
status replay has **24 hours**. Replay uses authorized owner/run scope, ordered
sequence keysets and pages of at most 100. Invalid/future positions or retained
gaps conflict so a later transport can reset/refetch truthfully. There is no
shipped stream route here, and answer text is never persisted for replay.

Retention removes old events, then eligible steps, runs without children and
settled usage, sharing one total row budget per transaction. Unknown reservations
survive run removal, ordinary expiry and quota reset until explicitly settled.
Protected audit entries have a separate lifecycle and no cascade from run
deletion. Scheduled bounded pruning gives eventual cleanup, not a precise
wall-clock deletion promise during downtime or backlog.

## Upgrade and native recovery

Follow the existing [backup/restore](../operations/backup-restore.md) and
[upgrade/rollback](../operations/upgrade-rollback.md) procedures with matching
verified release tools. Those documents retain their accepted release scope;
this candidate adds the schema-18 restrictions below.

Before a controlled **17 → 18** upgrade, make and independently restore-verify a
schema-17 backup. Upgrade uses the immutable migration prefix and preserves
existing data. There is no down migration. An older schema-17 binary must not
run against schema 18; rollback uses the verified pre-upgrade backup restored to
a separate empty database with its matching older storage tool.

A schema-18 backup/restore requires its matching schema-18 storage build and
PostgreSQL-native tools. Restore is offline into an empty target, verifies the
exact ledger, revokes restored sessions/pre-authentication grants and advances
the restore generation. Unknown usage and its reserved balance remain in the
archive; a new generation invalidates old run authority and the prior paid
generation even if enablement flags were restored as true.

An older backup can omit later dispatches. Paid admission stays fenced until a
trusted operator reconciles the missing interval or conservatively reserves its
maximum exposure under the new generation. An old account balance is not proof
of free allowance. Paid/remote execution is not implemented by these accounting
tests. Application retention does not erase old backups or PostgreSQL WAL;
archive custody, encryption, deletion and a real restore drill are operator duties.

## Verification and remaining acceptance

Prepare pinned tools and a fresh owned PostgreSQL fixture as described in
[testing](../testing.md). Set `STACKPLORE_TEST_POSTGRES_URL` privately to that
fixture; do not print its value. The native test also needs matching `pg_dump`
and `pg_restore` and `STACKPLORE_TEST_NATIVE_POSTGRES_TOOLS=true`.

```powershell
go -C apps/api test ./internal/assistant ./internal/assistantworker -count=1
go -C apps/api test ./internal/postgres -run '^TestAssistant' -count=1 -v
go -C apps/api test ./internal/postgres -run '^TestReleaseUpgradeFromShippedPrefixesPreservesDataAndImmutableLedger$' -count=1 -v
go -C apps/api test ./internal/postgresops -run '^TestAssistantNativeRestorePreservesUnknownUsageAndFencesPaidGeneration$' -count=1 -v
```

Use a supported concurrency toolchain with a C compiler for
`go -C apps/api test -race ./internal/assistant ./internal/assistantworker` and the
corresponding real SQL suite. A skipped integration is not SQL or recovery proof.
The native restore test creates owned source/restore databases; cleanup and
absence must be checked. Never point these fixtures at customer data.

The preparatory 2026-10-21 focused SQL run records **10 pass events**, **zero
skips**, seven query plans, stable seven-file source manifest and verified schema
cleanup. Its manifest digest is
`ec9ce466491f767f09b2b16c9ac779bd5fbea518956007ed5ab7ac46b1179de0`;
plan-summary digest is
`6651d4f85539977723655bc82d4ced7aff9d70f438ae438eca379b2b94df330c`.
The locked-candidate test actually held the first queued row and claimed the
next without altering quota. Transaction-fault tests verified rollback of
admission and step/run transitions.

The plan fixture used **4,096 synthetic history runs**. Claim execution was
0.133 ms in that one run. Run pruning took **220.297 ms** and included a
sequential scan of **7,168 synthetic `assistant_steps` rows**. Bounded deletion
count does not establish bounded query work. These are inspected local plans,
not a customer capacity, throughput or latency guarantee; broader history and
the complete frozen candidate require fresh profiling and acceptance.

Maintenance unit tests additionally exercise startup while dispatch is absent,
full-batch pacing, transient failure, deadlines, cancellation/join and invalid
counts/clocks. Their statement coverage is not branch-coverage proof. Fresh
stage-2 full-source verification, real migration/repository/native recovery,
concurrency, shutdown composition and hosted receipts remain open in the
[ledger](implementation.md). No prior profile or stage-0 receipt accepts them.

## Fresh local gate — 2026-10-24

[Candidate evidence](../evidence/2.0.0-alpha.2/README.md) binds twenty source
files to actual unit/server, SQL, three native crash phases, schema17 upgrade,
native restore and complete local verification passes. The earlier unit failure
stays failed; built frontend, compiler and isolated host conditions differ, so
no unique failure cause is claimed. Web544/client40/release48/corpus11 and
production build pass. The exact final-source hosted/race, signed checkpoint,
independent artifacts and separate runtime branch requirement remain open.
