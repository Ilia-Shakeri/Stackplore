# Exploring observations

[فارسی](exploration.fa.md). This guide describes the 0.4.0 exploration checkpoint.
The [evidence plan](implementation-plan.md) records executed checks; this guide
does not establish live Docker, Kubernetes, proxy, or deployment proof.

## Sources and trustworthy states

Choose an infrastructure source before following a resource, log, event, or
topology link. Resource detail URLs include the exact source, provider, resource
kind, and opaque resource identifier. The source in that path takes precedence
over a conflicting query parameter.

Health and availability are separate. Healthy, degraded, unhealthy, and unknown
describe observed resource health. Available, stale, unavailable, unsupported,
and permission denied describe access to observations. Stale observations may
remain visible with a stale notice. Partial observations never establish that
an unseen resource, relationship, event, or metric does not exist. Missing metrics
display an unavailable state; a measured zero remains zero. Source counts and
health totals are observations, with a partial notice where coverage is partial.

## Inventory and search

Inventory and search use the same server-filtered, cursor-paginated resource
reader. Search covers safe public name, opaque identifier, environment, kind,
and scope fields. The interface supports provider, environment identifier,
resource kind, health, observation availability, scope kind, and scope name filters, plus name, kind, or
observation-time sorting in either direction. Resource kind uses its complete
namespaced value, such as `docker.container`. Text filters apply when the field
loses focus or Enter is pressed; the main search applies with Search.

The URL stores `source`, `q`, `provider`, `environment_id`, `kind`, `health`,
`availability`, `scope_kind`, `scope_name`, `sort`, and `direction`. Reload and browser history
restore those filters. Unknown names and malformed values do not become
arbitrary backend options. The type/scope column visibility preference stays
in this browser. A resource link opens the exact resource detail page, whose
navigation opens resource logs, resource events, its topology selection, or
source alerts.

Views request 100 items per page and keep at most 1,000 unique loaded items.
Load next page preserves loaded rows and uses the server cursor. A failed next
page keeps the current rows and offers a retry. A source, query, or live refresh
cancels the old page request and discards late replies. Duplicate identifiers
merge, repeated cursors fail safely, and partial/stale notices survive page
merges. The 1,000-item notice also appears if a final page would cross that
browser limit. Narrow server filters to inspect a smaller set. This is a bounded
explorer, not an unbounded browser archive.

## Time correlation

Overview metrics and event/log views offer one-hour, six-hour, and one-day
windows. Absolute `from` and `to` values are UTC timestamps with at most six
fractional digits. Both bounds are required together; the interface offers
windows of at most one day. Metric history additionally supports the API's
bounded `minutes` query, independently of the absolute-range form. The current
interface requests at most 200 history samples per read.

Live sources use the current clock. Synthetic sources end one millisecond after
the selected source's observed fixture clock, covering its sub-millisecond facts
inside the contract's inclusive-start, exclusive-end interval. A deterministic
story therefore does not disappear as the calendar advances. Explicit absolute
bounds remain exact shareable snapshots. Charts retain
accessible table alternatives and show missing or partial observation states.
An observation clock, a collection clock, and a source-wide count do not prove
that all resources were sampled at the same instant.

## Logs

The log target accepts an observed opaque resource identifier and, where
supported, a container name and previous-container selection. A container or
previous-container request requires an exact resource. Observed container names
from loaded lines appear as suggestions; a suggestion list does not claim a
complete pod-container inventory. Unsupported targets and denied permissions
show their actual availability rather than an empty successful result.

Live tail uses the authenticated source stream for prompt invalidation and a
bounded refresh five seconds after the preceding read settles for the exact HTTP target. Stream invalidations wait behind an active read and coalesce into a subsequent refresh. The extra refresh covers
resources or container changes outside the stream's initial resource window.
Changing a target or refreshing cancels the older request; the view has one
active page request. Live windows roll forward. Pause closes the stream and
stops the refresh timer while keeping loaded lines visible; Resume opens a new
stream and returns to a rolling window. An explicit `from`/`to` deep link opens
paused. `tail=live` or `tail=paused` records the selected mode; `minutes` records
the rolling window size.

Search and severity apply to the **loaded lines**, and use URL parameters
`log_q` and `severity`. Wrapping and timestamp visibility use `wrap` and
`timestamps`. Select individual lines to copy their timestamp, severity, and
already-redacted text. Copy failures have a visible fallback message. The
download contains only the currently loaded lines that match the local filters,
up to 1,000 lines, as a text file. It is not a full-history export and does not
fetch hidden raw text. Redacted and shortened lines have explicit labels. Logs
are technical LTR text in both interface languages and are not stored as a
general PostgreSQL archive.

## Events and alerts

Events use the exact selected source, optional resource, and time window, with
cursor pagination. A resource event link searches the server inventory for the
opaque identifier, so it can find a resource beyond the first loaded page.

Alerts load their durable history in cursor pages. The state filter uses
`alert_state` and explicitly applies to **loaded alerts**; load more to expand
that set. An empty filtered loaded set does not claim that no matching alert
exists in later pages. Administrators may acknowledge supported real firing
alerts, including persisted synthetic demonstration metadata. This changes alert
state and does not authorize a provider write. A later stream refresh wins over a delayed acknowledgement or old-page
reply. Durable replay gaps trigger a fresh read. Rule settings also paginate;
their current server cursor is shareable as `rule_cursor`.

## Topology and keyboard use

Resource nodes and relationships have separate bounded page controls. A partial
notice remains while either server result has more pages. A relationship whose
endpoint has not loaded cannot establish that the endpoint does not exist.
The graph lazily loads, draws at most 200 matching nodes, and reports the graph
limit. All loaded matching nodes remain available in the accessible list,
including isolated nodes; that list paginates in groups of 50.

Filter by environment, scope, kind, health, or search. Group by scope,
environment, kind, or no grouping. Select a node for its details, or highlight
direct neighbors, upstream, or downstream nodes. Zoom, fit, pan buttons, minimap,
and full-screen controls complement pointer navigation. The node and relationship
list provides keyboard links to equivalent resource and evidence details.

Topology state uses `topology_q`, `topology_kind`, `topology_environment`,
`topology_scope`, `topology_health`, `topology_group`, `topology_selected`, and
`topology_neighbors`. Edges distinguish source-to-target, bidirectional, and
undirected relationships. Evidence records provider, provenance, certainty,
confidence, and observation time. Inferred relationships differ visibly from
explicit ones. Shared network membership expresses co-location; it does not
prove an application dependency.

## Limits and further reading

These controls preserve provider-neutral, read-only observation. They add no
shell, arbitrary proxy, credential input, or infrastructure mutation. A stream
connection alone does not establish fresh or complete provider evidence. Real
provider permission/fidelity checks and deployment/browser evidence remain
separate gates in the [roadmap](roadmap.md) and [testing guide](testing.md).
See [observations](observations.md), [alerts and delivery](alerts-notifications.md),
and the [HTTP contract](../contracts/openapi.yaml) for canonical limits.
