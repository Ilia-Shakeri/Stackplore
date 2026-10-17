# Current user guide

This describes the authenticated exploration checkpoint and the implemented
0.5 provider/guest candidate. Current PostgreSQL, actual browser and disposable
provider proof is recorded separately in the evidence plan; full installation
and production acceptance remain later gates. Every 1.0 feature is not yet
available. See [remaining work](roadmap.md) and
[راهنمای فارسی](user-guide.fa.md).

1. Ask the operator to start the installation with PostgreSQL. A fresh
   installation exposed beyond loopback needs a short-lived setup proof from
   the operator. There is no default account.
2. Open `/setup`, choose the language, enter the required operator proof, and
   create the first administrator. Later users sign in at `/login`.
3. Choose a source in the product header. Demo data is synthetic. Real Docker
   and Kubernetes profiles are operator supplied. Administrators can preview and
   toggle registered profiles at `/settings/connections`; browsers cannot create
   profiles, change endpoints/paths or upload credentials. Profile availability
   means configured, not a successful live connection check.
4. Inspect `/overview`, `/resources`, and a resource's stable detail link. Health
   and capability status distinguish available, stale, unavailable, unsupported,
   and denied data. A displayed partial page is not the whole estate.
5. Use `/topology` for the graph and its list alternative, `/logs` for bounded
   current log reads, and `/events` for infrastructure events. Do not treat an
   inferred edge as a guaranteed dependency. Inventory, events, topology and
   alert history have bounded paging; shared filters, correlated time windows,
   and log tail/pause/wrap/copy/download controls are described in the
   [exploration guide](exploration.md). Log downloads contain loaded redacted
   lines, and local log/alert filters apply only to their loaded set.
6. Read `/alerts` and `/notifications`. Administrators can acknowledge a persisted firing alert, including synthetic demonstration metadata,
   manage alert rules, inspect safe notification status, and request a guarded
   test delivery when operator webhook or Matrix configuration allows it. Select
   the configured test lane and supply the current password. A queued test
   is not proof that a receiver accepted it.
7. Administrators use `/settings/users` for account management and `/audit` for
   protected history. High-risk account/test operations request the current
   password to obtain a purpose-bound one-use grant.
8. Switch English/Persian and the theme in the header. Preferences persist
   locally; the broader preference settings page is planned. Sign out through
   the account menu.

## Administrator source changes

1. Open `/settings/connections` and find the exact registered source. Enabled
   state persists across restart. An unavailable operator profile can be
   disabled, but cannot be enabled until the operator supplies it.
2. Choose enable/disable and review the server's exact source ID, current version,
   collection impact and two-minute expiry. No infrastructure resource changes.
3. Confirm the named change and enter the current password. A one-use
   `source.update` grant, one-use confirmation, version change and audit commit
   together. Viewers cannot read or mutate this administrator registry.
4. After stale, expired, used or uncertain results, reload actual profiles and
   obtain a fresh preview. The interface does not replay a lost write response.

Inventory, details, health, topology, metrics and events read stored normalized
observations. Collection runs independently with bounded polling and backoff;
loading a page does not recollect the estate. Outages can show old data as stale.
Disabling a source keeps stored observations, marks them stale and blocks live
log reads. Missing metrics are unavailable, rather than fabricated zero totals.
See [collection, source settings and retention](observations.md).

Metric status can be partial when resource support or connection results differ
within a source. Stored samples keep their own timestamps; a source-level metric
status does not prove each resource is currently supported or freshly observed.
Mixed metric support keeps a conservative source-wide status. Per-name resource
status is not persisted; current samples older than five minutes are stale and
retain their actual timestamps. Restricted provider facts and guest isolation
are described in the [Docker](docker-provider.md),
[Kubernetes](kubernetes-provider.md) and [demonstration](demo.md) guides.

Viewers cannot administer users/rules/global settings or mutate infrastructure.
The current baseline exposes no live restart/scale action. The installable PWA,
broader safe profile setup, full detail
tabs, and complete history/export/cleanup flows remain roadmap work.

Loading, denied, unavailable and disconnected states deserve attention. Retry a
failed read; if it persists, give the operator only the safe error code and
request ID. Do not share raw logs, credentials, session data or private targets.
See [troubleshooting](operations/troubleshooting.md).

Alert windows require covered fresh history; incomplete evidence remains unknown.
Restart thresholds count observed increases per configured window. Alert changes
and resets refetch durable state, including acknowledgement after restart. Rule
source is immutable during editing; create a new rule to move to another source.
See [exact alert and delivery semantics](alerts-notifications.md).
Ordinary history has fixed bounded retention; audit currently retains all rows.
Configurable quotas, protected audit cleanup, exports and archive custody are
later-stage work.
