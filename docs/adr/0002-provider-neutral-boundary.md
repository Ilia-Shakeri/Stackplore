# ADR-0002: Use Provider-Neutral Domain Ports

- Status: Accepted
- Date: 2026-07-28
- Owners: Project maintainers

## Context

Demo, Docker, and Kubernetes expose different object models, health signals,
relationship evidence, log APIs, metric units, event semantics, permissions,
and watch behavior. If vendor objects cross the service or HTTP boundaries, the
frontend and application rules would accumulate provider branches. That would
make safe metadata filtering, tests, and future providers harder.

At the same time, forcing every provider into the smallest common shape would
discard evidence needed for troubleshooting. The boundary must normalize common
behavior while retaining reviewed, typed detail and honest capability gaps.

## Decision

Define provider-neutral domain types and focused provider ports inside the Go
domain boundary. Split the ports by behavior:

- inventory and resource detail;
- health;
- topology evidence;
- bounded log reads and live tails;
- metric reads and sampling;
- event history and watches;
- capability inspection.

Keep the infrastructure command port separate from all observation ports.

Each source has a stable identifier and provider kind. Resource identity
combines source, environment scope, kind, namespace or project, and the
provider-native identifier. Display names are not keys.

Provider adapters translate vendor objects into:

- common resource, health, metric, log, event, and relationship types;
- explicit unavailable, unsupported, stale, and permission-denied states;
- bounded typed metadata whose fields are approved for display;
- relationship provenance and confidence.

The Docker adapter uses `github.com/moby/moby/client`. The Kubernetes adapter
uses official `client-go` packages and GA APIs where available. The optional
read-only `metrics.k8s.io/v1beta1` Resource Metrics API is the one named
pre-GA exception until upstream provides a GA replacement. Those dependencies
stay inside their adapters. The Demo adapter uses versioned local fixtures.

Adapters may receive raw provider payloads with sensitive fields, but no
adapter returns or persists raw credentials, complete environment variables,
Kubernetes Secret values, unfiltered labels, or unbounded provider payloads.
Kubernetes Secret references are learned only from safe resource
specifications; Secret read permission is not requested.

HTTP handlers and the browser receive only domain DTOs and capability results.
Provider-specific safe detail appears in small tagged metadata structures, not
opaque maps.

## Consequences

### Benefits

- Application services and the browser share one coherent model.
- Provider fixtures can test domain behavior without live control planes.
- Missing permissions become visible capability or degraded states.
- Metadata review and redaction happen at one trust boundary.
- Another provider can be added without changing unrelated routes and views.

### Costs and risks

- Translation code must be maintained as provider APIs evolve.
- A poorly chosen common model could erase useful provider evidence.
- Tagged metadata needs schema review and size limits for every new field.
- Capability results can change while the service is running, so callers must
  still handle provider rejection.

## Alternatives considered

### Pass vendor objects through the API

This couples the browser contract to upstream APIs, expands the secret-review
surface, and makes provider comparison difficult.

### One large provider interface

Large interfaces force every adapter to implement unrelated behavior and make
tests broad. Focused ports allow explicit optional capabilities.

### Normalize to opaque key-value maps

Opaque maps are difficult to validate, document, localize, and filter safely.
Typed detail makes additions deliberate.

### Infer dependency from environment-variable values

This risks credential exposure and creates weak, surprising topology evidence.
Stackplore will use explicit annotations, Compose declarations, ownership,
selectors, routes, storage references, and clearly labeled network membership.

## Implementation and verification

Later phases must:

- define provider-neutral contracts and safe metadata rules before adapters;
- run the same provider contract suite against deterministic fakes;
- test stable identity, duplicate relationship evidence, confidence, and
  incomplete metadata;
- test that forbidden fields and Secret values cannot reach DTOs, logs, errors,
  or audit records;
- use mocked Docker and Kubernetes clients for denied and partial permissions;
- label live integration instructions separately from automated fixture proof.

## References

- [Architecture overview](../architecture.md)
- [ADR-0006: Read-only and guarded mutations](0006-read-only-and-guarded-mutations.md)
- [ADR-0007: Deterministic isolated demo](0007-deterministic-isolated-demo.md)
