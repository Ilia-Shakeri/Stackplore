# Alerts, live changes and outbound delivery

[فارسی](alerts-notifications.fa.md). This describes the locally verified 0.3.0 development
checkpoint. Exact executed checks live in the [evidence plan](implementation-plan.md).
It is not a production capacity or live-provider claim.

## Evidence and windows

Rules require enabled operator-owned sources and provider-neutral evidence.
Collection outcomes are stored separately from cached observations and bound to
the current source configuration version. Re-enabling a source cannot reuse an
earlier configuration's successful collection. Missing profiles, disabled sources,
denied rights, partial or unsupported results, future timestamps and collection
gaps produce unknown evaluation, not a clear or firing condition.

- CPU cores and memory bytes compare the time-weighted mean of held gauge values
  over `window_seconds`. Each held interval requires a sample no more than 90
  seconds old and complete collection coverage.
- Restart thresholds compare observed positive counter increments over that
  window. A counter reset contributes the new counter value; the reset itself
  contributes no negative or inflated increment. This is an observed count per
  configured window, not a per-second rate and not proof of unseen restarts.
- Unavailable replicas compare the time-weighted mean of desired-minus-ready
  deficit over the window. The mean can be fractional; the configured count
  threshold remains an integer. Missing replica evidence breaks coverage.
- Unhealthy and unavailable resource rules require fresh complete evidence for
  the exact resource. Unknown health does not clear an active alert.
- Connection loss uses the raw collection result. Only known provider-outage or
  timeout reasons count as unavailable; cached stale data, disabled profiles and
  denied permissions do not establish connection failure.
- Event severity examines the whole bounded evaluation-interval window and
  applies exact source, optional environment and resource scope. Truncated or
  incomplete history is unknown. The scan stops at 20 pages of at most 200 events
  each (4,000 facts); insufficient coverage remains unknown. `window_seconds` is absent for this rule kind.
  Evaluation reads at most 20 pages of 200 events (4,000 total); a remaining next
  page makes the condition unknown even if an earlier page contained a match.

Metric, replica and raw collection window readers cap at 4,096 facts per query.
They fail to unknown when the cap or coverage cannot prove the requested window.
A 24-hour window at the nominal 30-second collection cadence fits this cap;
backoff, incomplete upstream history and fixed-clock synthetic fixtures can still
leave the window unproved. Short rule evaluation intervals do not increase the
collector's evidence cadence.

## Episodes, claims and demo data

States are pending, firing, acknowledged and resolved. Pending time counts only
continuous proven matches. A gap longer than the evaluation interval plus its
10% scheduling margin (capped at five seconds) starts pending time again. Unknown
evidence preserves an existing firing/acknowledged alert. A proven later match
after resolution creates a new episode identifier.

PostgreSQL evaluation claims use 60-second leases and generation/rule-version
fences. Completion also holds and verifies the current enabled source configuration
version and checks lease expiry against the database clock after acquiring locks.
Late results cannot overwrite a newer claim, acknowledgement, rule edit or source toggle.
Rule replacement or deletion closes the old active episode and invalidates its
claim in the same transaction. Scheduling claims one task immediately before
reading it, within a maximum 32-task run. Shutdown cancels stream contexts at its
start, allows ordinary requests to drain, then waits for jobs before closing storage.

Demo fixture version `demo-v1` supplies evidence for the same evaluator/lifecycle
and creates one synthetic rule, firing episode, transition and in-app notice.
Seeding waits for the first enabled administrator and captured resource. A durable
fixture marker makes repeated starts idempotent. Acknowledged episodes and deleted
rules are never resurrected by startup. Synthetic notices never enter outbound
delivery, including when an adapter is enabled.

## Durable alert SSE

`GET /api/v1/streams/alerts?source_id=...` reads committed PostgreSQL change
metadata, not provider fixture alerts. `alerts.snapshot.v1` contains the bounded
first alert page and its page metadata. `alerts.changed.v1` contains at most 100
ordered change facts: decimal-string ID, source/rule IDs, optional alert ID,
kind and time. Fetch authoritative paginated HTTP state after either event.

The SSE ID is the committed change position. Pass `Last-Event-ID` on reconnect;
IDs survive process restart. Commit-order serialization prevents late lower
sequences from being skipped. IDs are scoped by installation and source; do not
reuse a cursor for another source. Without `Last-Event-ID`, the server sends an
initial snapshot without a reset. A nonzero future or expired replay position
causes `stream.reset.v1` with `replay_gap` and a new snapshot. A snapshot at zero
change position emits an empty SSE `id:` field, clearing the browser's old cursor;
an empty alert page at a nonzero position keeps that position. The browser refetches on reset and on committed
changes. An initial snapshot reads the cursor before the page, so a concurrent
commit may repeat but cannot be silently skipped.

Each connection has a 16-frame queue, 256-KiB frame cap, ten-second read deadline,
two-second poll, 15-second keepalive, 30-second authorization check, 30-minute
lifetime and five-second write deadline. The shared cap is 64 streams and four
per user. Backpressure ends that connection with a reset when writable; cancellation
stops its polling. Other observation streams retain their 128-frame process-local
replay ring. All streams disable cache and proxy buffering. See
[ADR-0012](adr/0012-durable-alert-change-replay.md).

A separate minute job deletes at most 1,000 old change/raw-collection/replica
facts across fair quotas under one ten-second deadline. Eligibility starts at
seven days, with the newest position/outcome retained for each scope. Audit,
alert episodes and transitions are not expired by that job. Cleanup is bounded,
so actual retained history may exceed seven days while a backlog drains.
Each source also retains the highest pruned change ID. A replay cursor before
that watermark resets even when an older change row remains, preventing a hole
from being mistaken for complete replay.

## Webhook and Matrix

Both lanes default off. Operators supply full destinations and credentials in
the environment or referenced deployment Secret. The administrator settings page
shows enabled state and safe logical references for each lane; it cannot replace
hidden credentials. See [configuration](configuration.md) and
[deployment](operations/deployment.md).

Every attempt enforces HTTPS scheme, strict host/IP checks, fresh DNS validation,
pinned validated destination addresses, TLS hostname verification, no redirects,
ten-second timeout and 64-KiB response cap. Private, loopback, link-local and
special-use destinations are rejected. IPv6 policy conservatively allows only
global `2000::/3` addresses after excluding special-use, transition,
protocol-assignment and documentation ranges from the
[special-purpose address registry](https://www.iana.org/assignments/iana-ipv6-special-registry/).
Matrix uses one canonical operator origin, one exact room and
`PUT /_matrix/client/v3/rooms/{room}/send/m.room.message/{id}`; there is no discovery.
The bearer token never follows a redirect or another origin.

The outbox persists an immutable delivery ID and canonical body per adapter and
destination reference. Webhook retries preserve their signed payload and stable
idempotency key. Matrix retries reuse the same transaction ID and `m.notice` body.
Each lane has independent cursor, lease and recovery state. Attempts are claimed
immediately before sending, fenced by current lease and bounded by its deadline.
Ordinary delivery allows at most three attempts with bounded retries. Timeout or
lost response can mean the receiver accepted a message; delivery is at-least-once
and receivers must deduplicate. An uncertain terminal outcome stays uncertain.
Matrix transaction deduplication also depends on the same device and endpoint
context, as defined by the [Matrix transaction identifier contract](https://spec.matrix.org/v1.15/client-server-api/#transaction-identifiers).
Changing credentials to a different device context during retry can cause a duplicate; drain/reconcile
pending deliveries before operator credential or destination changes.

Transient DNS lookup failures can retry without claiming that bytes were sent.
A timeout, lost response, oversized response or invalid successful Matrix reply
can leave acceptance unknown and uses the same stable delivery identity on retry.
Migration 10 cancels any legacy synthetic pending/in-flight delivery and closes
its unfinished attempt; fresh claims also exclude synthetic notifications.

`POST /api/v1/settings/notifications/test` defaults to `generic_webhook`; optional
`?adapter=matrix` selects Matrix. It requires administrator, same origin, CSRF and
a one-use `notification.test` recent-auth grant. It queues one bounded single-
attempt notice with durable intent and terminal audit. Grant consumption precedes
the intent transaction; a failed or lost request needs fresh authentication and
state inspection. Never automatically replay it. A queued response is not proof
of receipt. Credentials, room IDs, full URLs and response bodies never cross this
browser boundary or enter audit metadata.

Disposable local HTTPS tests and real PostgreSQL tests are distinct from live
operator receiver, container, reverse-proxy, browser and target-cluster proof.
The [roadmap](roadmap.md) keeps those remaining checks explicit.
