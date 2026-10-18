# Current user guide

This describes the implemented **1.0.2 candidate**. Its focused corrections
are recorded in the [fresh audit](evidence/1.0.2/fresh-audit.md); earlier full local C and the scoped production-interface browser passed. A later
retained-event REST/SSE correction has focused proof; full replacement D passes; new
exact-source hosted/signed/public-consumer acceptance remain pending. Historical [1.0.1 proof](evidence/1.0.1/published-release.json) retains
its original scope. Customer production and offline installation need separate
operator checks. [راهنمای فارسی](user-guide.fa.md).

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
   and denied data. A displayed partial page is not the whole estate. Overview
   ranks CPU/memory consumers only within loaded observations and labels that
   scope; resource details show metric and health history with exact source and
   resource selection. Network counters and restart counts are not invented rates.
5. Use `/topology` for the graph and its list alternative, `/logs` for bounded
   current log reads, and `/events` for infrastructure events. Do not treat an
   inferred edge as a guaranteed dependency. Inventory, events, topology and
   alert history have bounded paging; shared filters, correlated time windows,
   and log tail/pause/wrap/copy/download controls are described in the
   [exploration guide](exploration.md). Log display filters apply to loaded lines;
   a download separately streams matching redacted server pages under the current
   source/resource/time/container/search/severity selection and export policy.
   Event text/severity/environment filters apply on the server before paging.
   The alert-state filter still applies only to loaded alerts.
   When native event-history capability is denied, the entire event timeline is
   denied/empty; safe operation/alert facts are not a separately exposed subfeed.
6. Read `/alerts` and `/notifications`. Administrators can acknowledge a persisted firing alert, including synthetic demonstration metadata,
   manage alert rules, inspect safe notification status, and request a guarded
   test delivery when operator webhook or Matrix configuration allows it. Select
   the configured test lane and supply the current password. A queued test
   is not proof that a receiver accepted it.
7. Administrators use `/settings/users` for account management and `/audit` for
   protected history. High-risk account/test operations request the current
   password to obtain a purpose-bound one-use grant.
8. Switch English/Persian and the theme in the header. **Settings → Preferences**
   stores local calendar, digits and 12/24-hour choices, with truthful save-error
   feedback. **Settings → General** shows effective application policy; only
   administrators may preview/confirm its changes, masked configuration export
   or bounded eligible-history cleanup. See [settings](settings.md). Sign out
   through the account menu.

## Quick navigation

Use the header's navigation button or **Ctrl+K** (**Command+K** on macOS) to
open quick navigation. Search by the translated page name or its fixed path;
use Up/Down and Enter to choose, or Escape to close. Viewer/guest choices omit
administrator destinations. The selected infrastructure source follows the
navigation. Closing returns focus to the prior control; choosing a page moves
focus to its main content. An existing modal keeps its own keyboard interaction.

Log severity labels are translated, while their shareable URL/API values stay
stable, including trace and unknown. Graph nodes show their provider/type icon,
scope and actual observed state; the accessible list offers equivalent facts.
Deleting the rule being edited clears that draft only after successful deletion.
Cancel remains unavailable while the editor is loading or saving, so a late
deletion reply cannot erase a new draft opened during that request.

The account table renders at most 50 rows at once. Previous/next controls visit
every loaded account; a fresh server page is requested only at the end of that
cache. Refresh or a successful account action returns to the first window.
The displayed count means loaded accounts, not the installation total. The
manually requested cache can grow until refresh/unmount; windowing is not a
fixed limit on how many accounts may exist or be inspected.

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
Administrators can use only supported fixed [guarded actions](operations/guarded-actions.md)
when deployment enablement, separate provider write authority and all preview,
version, recent-auth, confirmation and audit gates allow them. Read-only remains
the default. The browser cannot create a provider profile or edit raw credentials.

## Offline shell and updates

On HTTPS or another secure browser context, the production build can register an
installable public offline shell. It caches only `/offline.html`, `/offline.css`,
`/offline.js` and `/brand/stackplore-favicon.svg`. It does not cache application
HTML, API responses, logs, account data, health probes or metrics. Offline
navigation shows the public offline page; it does not offer old private data.
A waiting worker shows an update action and reloads after the user chooses it.
Without service-worker support or installation permission, online use continues.

Loading, denied, unavailable and disconnected states deserve attention. Retry a
failed read; if it persists, give the operator only the safe error code and
request ID. Do not share raw logs, credentials, session data or private targets.
See [troubleshooting](operations/troubleshooting.md).

Alert windows require covered fresh history; incomplete evidence remains unknown.
Restart thresholds count observed increases per configured window. Alert changes
and resets refetch durable state, including acknowledgement after restart. Rule
source is immutable during editing; create a new rule to move to another source.
See [exact alert and delivery semantics](alerts-notifications.md).
Ordinary history follows the effective configured retention policy. Cleanup
previews freeze scope/cutoffs/counts; each confirmed manual batch removes at most
1,000 eligible rows. Protected audit policy has a 90-day minimum and a quota;
audit expiry remains disabled without an independently verified archive cutoff.
Audit rows linked to operations/deliveries stay protected. Archive custody and a
customer's backup/restore verification remain operator responsibilities. See
[settings](settings.md), [backup/restore](operations/backup-restore.md) and
[incident response](operations/incident-response.md).
