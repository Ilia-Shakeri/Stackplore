# ADR-0007: Use a Deterministic and Isolated Demo Mode

- Status: Accepted
- Date: 2026-07-28
- Owners: Project maintainers

Lifecycle clarification: [ADR-0013](0013-provider-coverage-and-demo-lifecycle.md)
distinguishes ordinary authenticated fixture persistence from isolated deployment
resets, protected history and explicit guest observation.

## Context

Stackplore needs a polished demonstration experience that exercises resources,
topology, health, logs, metrics, events, and alerts without requiring Docker or
Kubernetes. The same data must support reliable screenshots, browser journeys,
and provider contract tests.

Random values, wall-clock-dependent states, or accidental access to local
infrastructure would make results flaky and could mislead users. A public demo
also needs a tightly constrained trust surface that is much smaller than a
normal installation.

## Decision

Implement the Demo provider from versioned, deterministic fixture scenarios.
Fixture resources use stable identifiers. Health transitions, event order, log
lines, alert states, and metric samples are functions of a controlled scenario
clock and explicit step, never ambient wall time or randomness.

The v1 fixture set will include at least two environments and a realistic mix
of frontend, API, worker, PostgreSQL, Redis, reverse proxy, and monitoring
resources. It will include healthy and degraded resources, one unhealthy
resource, meaningful evidence-backed topology, realistic logs and metrics,
infrastructure events, and evidence that the Phase 22 rule engine turns into at
least one firing alert. Phase 8 owns provider evidence; it does not add alerts
to the provider contract. Phase 22 adds a versioned demo rule seed paired with
the fixture version.

Every demo source, resource, event, log, and metric is marked synthetic in its
domain representation and visible interface. Alerts later derived from that
evidence retain synthetic provenance. Stable scenario and fixture-version
identifiers support test assertions and future migrations.

Demo deployment mode is an explicit runtime profile:

- compose only the Demo provider;
- reject Docker and Kubernetes source configuration;
- load no provider credential references;
- expose no infrastructure command port;
- make the global mutation flag ineffective;
- prevent outbound notification delivery;
- permit guest viewing only under a separate explicit demo-deployment setting;
- require a separate demo data root and fixture-version-marked database;
- reject startup if that database holds any real source or non-demo history;
- retain server-enforced request, stream, and storage limits.

Demo provider code performs no control-plane network calls and reads only its
packaged fixtures. Its data cannot be merged implicitly with real provider data
under the same source identity.

Tests can advance the scenario clock directly. Runtime presentation can advance
through a fixed, documented cadence while producing the same sequence from the
same starting state. The scenario resets to its versioned initial step on
process start and explicit reset. A fixture upgrade clears demo-derived runtime
history before serving new data; state from different fixture versions is
never merged.

## Consequences

### Benefits

- Demonstrations and screenshots remain repeatable.
- Core product journeys work without external infrastructure.
- Provider-neutral use cases receive a complete contract fixture.
- Public demo deployments cannot accidentally inspect or mutate local control
  planes.
- Regressions can assert stable identifiers, order, values, and transitions.

### Costs and risks

- Fixtures require maintenance when domain contracts change.
- Realistic scenarios take more design effort than random sample rows.
- A fixed sequence can become too tailored to tests unless contract coverage
  includes varied explicit scenarios.
- A reset discards demo-derived runtime history by design.

## Alternatives considered

### Random demo values

Randomness makes screenshots, alerts, and tests unstable and can create
impossible relationships between health, events, and metrics.

### Use a live local Docker environment

That would require privileged access, vary between installations, and fail the
zero-prerequisite demo goal.

### Mix demo and real sources in one deployment mode

This increases confusion and risks presenting synthetic and real states
together. An explicit isolated mode gives users and operators a clear boundary.

### Static interface-only mock data

This would bypass domain services, API contracts, streams, alert evaluation,
and storage behavior. The Demo provider must use the same server boundaries as
other providers.

## Implementation and verification

Later phases must:

- validate fixtures against provider-neutral contracts;
- prove byte-stable output for the same fixture version and scenario step;
- test clock advance, reset, event order, log order, metric values, topology
  provenance, and alert transitions;
- test fixture upgrade cleanup and rejection of a normal installation database;
- test that demo mode rejects real source setup and every action path;
- run network-denial tests around the Demo provider;
- show a visible Demo Mode marker and synthetic labels in English and Persian;
- prove the paired Phase 22 rule seed yields the required firing alert and
  overview count;
- use the same fixtures in component and browser journey tests.

## References

- [Architecture overview](../architecture.md)
- [ADR-0002: Provider-neutral boundary](0002-provider-neutral-boundary.md)
- [ADR-0006: Read-only and guarded mutations](0006-read-only-and-guarded-mutations.md)
