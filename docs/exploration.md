# Exploring observations

[فارسی](exploration.fa.md). This guide describes the implemented **1.0.2
candidate**. [Fresh proof](evidence/1.0.2/README.md) keeps focused regressions
separate from passing local/browser gates and pending exact-source hosted
and signed/public acceptance. Historical 1.0.1 receipts retain their original scope; a guide does
not prove a customer's installation.

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
page keeps the current rows and offers a retry. A source or query change cancels
the old page request and discards late replies. Inventory, resource health and
topology consume stream invalidations through a five-second coalescing window.
A live refresh refetches the loaded page depth, up to ten pages, instead of
silently collapsing a multi-page view to its first page. Duplicate identifiers
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

Resource detail additionally reads exact-resource health history from
`/api/v1/health/history`, with at most 200 samples per read and a maximum 30-day
API lookback. The selected source must own the resource; denied current coverage
does not authorize disclosure of old health facts. Each sample retains its own
health, availability, completeness and time. Unsupported history remains
unsupported, and incomplete/denied samples cannot establish a healthy state.

Overview shows the five largest loaded CPU and memory consumers, environment
links and recent loaded events. These are loaded-subset rankings, not whole-estate
top consumers. Metric charts include CPU, memory, restart and receive/send byte
counter histories where reported. Counters are displayed as observed counts;
they do not become invented traffic rates. Every chart retains its table
alternative and the source's partial/stale/unavailable notices.

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

All seven log severities, including trace and unknown, survive reload and
browser history. Their visible labels use the selected English/Persian locale;
URL/API/download values remain stable. Built-in masking removes credential
assignments, bearer/basic values, private-key blocks and complete URL userinfo,
including usernames that carry tokens. JSON string values use the same policy.
Treat masked logs as potentially sensitive and share only reviewed text.

Search and severity apply to the **loaded lines**, and use URL parameters
`log_q` and `severity`. Wrapping and timestamp visibility use `wrap` and
`timestamps`. Select individual lines to copy their timestamp, severity, and
already-redacted text. Copy failures have a visible fallback message. The
download instead reads `/api/v1/logs/download` on the server using the current
source, exact resource, fixed UTC range, container/previous selection, search and
severity. The range is frozen at download start. The server applies configured
redaction and export permission to each continued page. It can include matching
lines beyond the loaded browser page; it does not fetch hidden raw text.

The text stream is limited to 10 seconds, 1 MiB including protocol markers,
1,000 matching output lines, 10,000 scanned lines and 100 provider pages.
A complete footer distinguishes completion from a safety-limited result. Missing
or incomplete footers, malformed lines and oversized responses produce an error
and no file. Policy denial remains an explicit denial. Changing selection or
leaving the page cancels the download and suppresses late results. This is a
bounded read, not an unlimited historical archive. Redacted and shortened lines have explicit labels. Logs
are technical LTR text in both interface languages and are not stored as a
general PostgreSQL archive.

## Observed resource configuration

Resource detail shows optional, reviewed provider facts from its stored inventory
observation. Missing metadata is explicitly not reported. It is never a raw
provider response. A Docker engine reports its engine version. Docker containers
show lifecycle/health state, restart count, image, engine version, start/exit facts, resource limits, container/published port
numbers, mount targets/types/access mode and the three allowlisted public label
keys. Host mount source paths, environment values and commands have no field.
Lists are bounded to 64 ports, 64 mounts and 16 labels.

Kubernetes shows up to 64 container entries and 16 typed conditions. Container
facts include readiness, state, restart count, safe reason codes and configured
CPU/memory requests/limits. Unknown runtime state keeps readiness and restarts
unknown, even when a workload template has zero placeholders. An explicitly
reported configured zero is retained. Pod and container resource links select
their exact source, resource and observed container name for logs; a previous
container link requires an observed nonzero restart count. Unsupported or denied
log requests keep their own explicit states.

Technical names, image references, ports, mount targets and public label keys/
values remain isolated LTR in both languages. Optional absent quantities are
not displayed as zero. Metadata envelopes must match the resource provider;
unknown fields, unsafe strings, oversized collections and duplicate port/label/container/condition identities are rejected
at the browser boundary. Stored metadata has a separate 32 KiB bound.

## Events and alerts

Events use the exact selected source, optional resource, and time window, with
cursor pagination. Text (`event_q` in the view, `q` in the API), severity and
`environment_id` are URL-backed filters applied on the server before pagination.
Cursor binding includes these filters; changing one starts a new result set.
A source's timeline can combine native events with safe operation/alert facts.
If that source's native `EventHistory` capability is denied, the complete event
feed is denied and empty. The current contract does not expose the safe system
subfeed separately and does not relax provider permission to show a mixed feed.
A resource event link searches the server inventory for the
opaque identifier, so it can find a resource beyond the first loaded page.

Alerts load their durable history in cursor pages. The state filter uses
`alert_state` and explicitly applies to **loaded alerts**; load more to expand
that set. An empty filtered loaded set does not claim that no matching alert
exists in later pages. Administrators may acknowledge supported real firing
alerts, including persisted synthetic demonstration metadata. This changes alert
state and does not authorize a provider write. A later stream refresh wins over a delayed acknowledgement or old-page
reply. Durable replay gaps trigger a fresh read. Rule settings also paginate;
their current server cursor is shareable as `rule_cursor`. A successful deletion
of the rule currently being edited clears its draft; a failed deletion does not.
Cancel is disabled while loading/saving so the pending result cannot reset a
new draft opened during that operation.

An explicitly isolated [guest demonstration](demo.md) has no account session,
administration or alert acknowledgement action. Its observation stream is
read only; synthetic provenance stays visible.

## Topology and keyboard use

Every graph node presents its provider/type icon, scope and actual observed
state; unavailable or mixed observations are not converted to healthy values.
Long names/scopes are visually bounded while full accessible details remain
available. The list provides the equivalent facts. Quick navigation opens from
the header or Ctrl+K/Command+K, supports Up/Down, Enter and Escape, preserves
the source and respects the current role; [user guide](user-guide.md#quick-navigation).


Resource nodes and relationships have separate bounded page controls. A partial
notice remains while either server result has more pages. A relationship whose
endpoint has not loaded cannot establish that the endpoint does not exist.
The graph lazily loads, draws at most 200 matching nodes, and reports the graph
limit. All loaded matching nodes remain available in the accessible list,
including isolated nodes; that list paginates in groups of 50.

Live inventory, health and relationship invalidations coalesce over five
seconds. Refresh preserves bounded loaded page depth; a changed source/filter
still cancels the old read. A connected graph stream is not independent proof
that every node's provider observation is fresh or complete.

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
