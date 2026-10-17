# Durable observations and source settings

This reference includes the 0.5.0 candidate provider and partition boundaries;
the exact executed gates remain in the active plan. [فارسی](observations.fa.md),
[architecture](architecture.md), [configuration](configuration.md) and
[current evidence](implementation-plan.md) cover the surrounding boundaries.

## Collection and reads

The operator enables Demo, a loopback-development Docker socket, the restricted
Docker proxy, or explicitly namespace-scoped Kubernetes through deployment
configuration. Startup inserts missing registry rows and preserves enablement.
An operator profile/authority change is audited, advances the source version,
and fences cache until fresh coverage publishes.
The browser can toggle these registered profiles. It cannot create a profile,
change its name, supply a host/path/URL or upload credentials. See the exact
[Docker](docker-provider.md) and [Kubernetes](kubernetes-provider.md) profiles.

One worker polls the registry every second. Successful sources are collected
every 30 seconds; failures back off from 30 seconds to five minutes. A changed
configuration version resets the schedule. At most four sources collect at once,
and at most 64 registered sources are accepted by this runtime. Source registry
overflow fails startup and collection rather than silently ignoring sources.

Each source has a read deadline of the lesser of the configured HTTP request
timeout and 20 seconds, followed by a ten-second aggregate persistence deadline.
Runtime collection caps each inventory category at 1,000, metrics at 10,000,
events at 1,000 and paging at 100 pages. Provider capture itself has a hard 10,000
item ceiling. Truncation is partial; it never proves absence. Capture writes use
typed batches of at most 100 records. Published kind/scope partitions have their
own complete generation. A complete allowed kind can retire missing members
while another kind is denied. Partial partitions cannot prove absence; denied,
unsupported or omitted partitions hide cached identities. Retirement performs
at most eight batches per cycle. Failed whole-source capture preserves authority
fences and never proves deletion. See [ADR-0013](adr/0013-provider-coverage-and-demo-lifecycle.md).

Inventory, detail, topology, health, replicas, metrics, history and events use
PostgreSQL read ports. A browser read or SSE poll does not recollect the estate.
Logs remain bounded live provider reads and are never stored by this worker.
The demo alert is seeded into durable rule/episode/transition/notification state
after an enabled administrator and resource capture are present. All alert reads
and replay use PostgreSQL; fixture startup preserves acknowledgement and rule
deletion. See [alert evidence and delivery](alerts-notifications.md).
PostgreSQL contains normalized facts and safe profile references, not raw
provider payloads, Secret values, mounted credentials or log bodies.

An outage may expose previously observed inventory as stale. Disabled collection
retains prior observations and marks them stale with `source.disabled`; it blocks
live log reads. Provider permission denial and unsupported ports expose no data.
Each optional port retains its own status and clock. Collection in progress and
interrupted batched writes must not appear as a complete fresh estate. The demo
keeps its fixture observation timestamps; a new polling attempt does not invent
new synthetic evidence.

Raw collection outcomes are stored separately and bound to the current source
configuration version. Alert evidence does not equate cached stale inventory
with provider connection failure. Re-enabling a source requires a collection
under the new version; an evaluation claimed under the old source version cannot
commit after the change. Missing operator profiles produce explicit unknown
evidence rather than extending a previous success.

Current metric status is conservative across the source when resource support
or availability is mixed. Denied sets block that port; mixed support/outage can
mark it stale and partial. This is not exact per-resource current-status storage;
per-name status is not claimed. The default view requests three names, while
collection requests all nine. The nine-resource demonstration persists 972
fixed facts across twelve samples per name and resource. Current facts older
than five minutes are stale against the real read clock, or the controlled Demo
clock. Future facts beyond the source receipt or one minute of real clock skew
cannot shadow valid current samples. Partitioned event history exposes only
events whose resource identity retains current authority; unscoped history is
hidden because its former namespace cannot be proven.

Keyset cursors contain signed immutable sort values and IDs, bound to the query.
Infrastructure cursors are capped at 4,096 bytes; local account cursors retain
their separate 1,024-byte input limit. Mutable pages are not a transaction-wide
snapshot: a renamed or newly observed resource may move between pages. Clients
must not infer an exact total or a point-in-time export from a page sequence.
Repeated logical events keep their first stored observation time; changes in
event facts receive a different stable revision ID. Conflicting immutable
history under the same ID is rejected.

## Exact source toggle

An administrator opens `/settings/connections`, selects a registered source and
requests an enable/disable preview. The server checks the quoted current
`If-Match` version and CSRF proof. It returns the exact target, requested state,
collection impact and a confirmation that expires after two minutes. A new
preview replaces this session's previous preview for the same source.

The administrator explicitly confirms the named change and enters the current
password. A `source.update` recent-authentication grant is purpose-bound and
one-use. The final PATCH consumes the hashed confirmation and grant, rechecks
the live administrator/session, changes the exact source version and appends
the safe audit fact in one PostgreSQL transaction. Failure rolls all of these
back. A capture using the old configuration cannot commit after the toggle.
No provider write occurs in this workflow, so observation toggles do not enable
restart or scale permissions.

Confirmation rows store digests only. Capacity is capped at 4,096 rows, one per
source/session, with up to 1,000 expired rows pruned on each preview. A profile
absent from current operator configuration can be disabled but cannot be
enabled. Viewers cannot read or change the administrator registry. If a write
response is lost, reload actual state and obtain a new preview; do not replay
the old confirmation automatically.

## Retention and protected audit

The fixed worker runs every minute. Each run has one ten-second aggregate
deadline and at most ten cleanup transactions. Each transaction deletes at most
1,000 rows across fair quotas for metrics, health, events and terminal ordinary
notification deliveries. Metrics older than seven days are eligible, health,
events and eligible terminal deliveries older than 30 days are eligible. The
newest metric for each resource/name is kept for current reads. Unchanged health
refreshes update current state without writing another history fact.

Deterministic synthetic observations, current identities/settings, pending
deliveries and audit-linked deliveries are excluded. Audit rows use a separate
90-day-minimum policy and operator-verified archive cutoff. The daily bounded
audit worker prunes only rows older than both limits and never expires linked
operation or notification facts; without an archive cutoff it does no pruning.
The cutoff is monotonic: startup rejects removing or rolling back a previously
stored cutoff. There is no browser retention override. Monitor database growth
and preserve PostgreSQL-native verified backups. A backlog can
outgrow the bounded cleanup rate and needs operator attention; these caps are
not a production capacity claim.

A separate bounded alert-history job retains the newest replay position and raw
collection outcome per source and the newest replica fact per resource. Its
seven-day eligibility, prune watermark and limits are described in
[alert history retention](alerts-notifications.md). It does not expire audit,
alert episodes or transitions.

## Verification limits

Real local PostgreSQL tests cover migrations, restart reads, row/version
transactions, replay, expiry, failed audit rollback, source isolation, status
truth, keysets and retention. Local query-plan timings come from disposable
warm-cache fixtures and are not production latency proof. Live provider,
browser, container and target-cluster gates remain separately recorded in the
roadmap and evidence plan.
