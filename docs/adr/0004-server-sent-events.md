# ADR-0004: Use Server-Sent Events for Live Browser Streams

- Status: Accepted
- Date: 2026-07-28
- Owners: Project maintainers

## Context

Resource status, infrastructure events, metric samples, log tails, and alert
changes flow primarily from the server to the browser. The browser sends
queries, filters, and actions through ordinary HTTP requests. A full duplex
protocol would add lifecycle and security complexity without a v1 need for
arbitrary client messages.

Live streams can consume unbounded memory or leave provider watches running
after a client leaves. Networks and proxies also disconnect long-lived
requests. The protocol must make reconnect, replay gaps, slow clients, auth
changes, and shutdown explicit.

## Decision

Use server-sent events over authenticated same-origin HTTP for live
server-to-browser updates.

Expose purpose-specific, versioned stream routes instead of one unscoped global
feed. Each route validates the session, role, source and resource scope, filter
limits, and concurrent-stream quota before subscription. Stream payloads use
typed event names and schema-versioned JSON that matches the OpenAPI-described
domain models.

Every stream implementation provides:

- ordered event IDs within a documented replay scope;
- keepalive comments that do not contain product data;
- a bounded in-memory replay ring;
- a bounded queue per connected client;
- server-wide and per-user stream caps;
- context cancellation down to provider reads and watches;
- safe write deadlines where supported;
- explicit shutdown and authorization-revocation behavior.

Clients send `Last-Event-ID` when reconnecting. If the ID remains in the replay
ring, the server replays subsequent events in order. If the ID is too old,
unknown after restart, or invalid for the requested scope, the server signals a
reset and the browser refetches a bounded snapshot before resubscribing.

A slow client's queue must not grow. When it reaches the defined limit, close
that stream with a recoverable overflow reason and record a safe metric. Do not
block provider watches on one client and do not silently drop events while
leaving the view apparently current.

The live hub is transient fan-out, not durable storage. Events, alert changes,
and metric samples use repositories when product retention requires history.
Log-tail buffers remain bounded and are not persisted by default.

SSE responses disable intermediary and browser storage, compression where it
would delay flushing, and buffering headers where the deployment supports
them. PWA caching excludes all stream routes.

## Consequences

### Benefits

- The protocol matches the one-way data flow and standard HTTP infrastructure.
- Same-origin session cookies and existing authorization middleware apply.
- Browsers have a clear reconnect model.
- Ordinary HTTP remains available for commands, snapshot reads, and downloads.

### Costs and risks

- EventSource has limited request customization; route design and cookie auth
  must account for that.
- Replay memory is local to one process and disappears on restart.
- Each stream holds an HTTP connection and needs enforced quotas.
- Some reverse proxies need explicit buffering and idle-timeout configuration.
- Reset handling adds client state logic but is required for honest freshness.

## Alternatives considered

### WebSockets

WebSockets provide full duplex messaging, but v1 client input already fits
bounded HTTP requests. They would add another message authorization and
backpressure model without a required use case.

### Fixed-interval polling

Polling is simple but inefficient for log tails and events, increases stale
windows, and repeats unchanged payloads. It remains a fallback for snapshot
refresh after stream reset.

### One global event stream

A global feed creates overbroad authorization, payload, and fan-out scopes.
Purpose-specific streams make limits and cancellation easier to reason about.

### Persist every stream frame

Many frames are projections or log lines that do not need durable storage.
Durability belongs to explicit repositories with retention policies.

## Implementation and verification

Later phases must:

- test ordered IDs, replay, keepalive, reset, overflow, and malformed
  `Last-Event-ID`;
- test session expiry, logout, source removal, permission loss, and server
  shutdown while connected;
- prove provider watches stop after the last subscriber;
- run slow-reader and concurrent-client tests under the race detector;
- verify proxy configuration flushes events and does not cache streams;
- expose safe metrics for active streams, resets, overflows, and reconnects;
- show a visible stale or disconnected state in the browser.

## References

- [Architecture overview](../architecture.md)
- [ADR-0001: Monorepo and same origin](0001-monorepo-and-same-origin.md)
- [ADR-0003: SQLite and one active API instance](0003-sqlite-single-instance-and-repositories.md)

