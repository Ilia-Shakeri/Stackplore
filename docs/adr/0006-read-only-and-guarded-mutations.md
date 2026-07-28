# ADR-0006: Keep Infrastructure Read-Only by Default

- Status: Accepted
- Date: 2026-07-28
- Owners: Project maintainers

## Context

Stackplore connects to infrastructure control planes. Docker socket access can
carry host-level power, and Kubernetes write permissions can affect production
availability. Most exploration and troubleshooting needs observation only.

V1 still permits a small set of recovery operations, but a generic provider
proxy, shell, or command interface would create an unacceptable authority and
audit surface. Hiding a button is not authorization, and a global flag alone
does not prove that a particular source, target, or user may perform an action.

## Decision

The application is read-only by default in code, configuration, deployment
permissions, and interface state.

Infrastructure actions use a separate command port with action-specific request
types. There is no generic Docker request, Kubernetes request, shell, exec, or
arbitrary command route.

V1 allowlists only:

- restart one Docker container;
- restart a Kubernetes Deployment, StatefulSet, or DaemonSet through the
  supported rollout annotation update;
- scale a Kubernetes Deployment or StatefulSet.

Every action must pass all gates:

1. `STACKPLORE_MUTATIONS_ENABLED=true` was present at service startup.
2. The deployment profile granted the exact provider write permissions.
3. The session is valid, the actor is currently enabled and an administrator,
   and recent authentication is proven.
4. CSRF and same-origin checks pass.
5. Demo deployment mode is off and the target source is not synthetic.
6. The provider reports the exact action capability for the source and target.
7. The action and resource kind match the server allowlist, and the expected
   provider version or resource version still matches.
8. A short-lived, single-use confirmation is bound to actor, source, target
   identity, action, and material parameters.
9. A request-scoped idempotency record protects any retryable effect.
10. Audit intent commits durably before the provider call; write failure stops
    the action.
11. A server-enforced timeout and bounded provider call apply.
12. The final outcome is audited with request ID and safe metadata.

Confirmation creation returns a human-readable preview of the exact target and
effect. The execute request cannot alter those bound fields. Confirmation
reduces mistakes but does not replace any other gate.

Return the observed provider outcome honestly; a timeout does not imply that
the control plane made no change.

Docker production deployment uses a restricted socket proxy with separate
read-only and optional mutation allowlists. Kubernetes uses a read-only role
and a separate optional mutation role. Enabling the application flag does not
expand deployment permissions.

Delete, force delete, arbitrary API proxying, exec, shell, arbitrary Docker or
Kubernetes commands, Secret editing, image deployment, and cluster
administration are outside v1 and have no route or port.

## Consequences

### Benefits

- Default installation authority matches its observation purpose.
- Enabling one gate cannot bypass role, capability, confirmation, or deployment
  permission checks.
- Narrow request types and audit records make actions reviewable.
- Demo and viewer sessions cannot reach provider mutation code.

### Costs and risks

- Operators must deliberately enable both runtime policy and provider
  permissions.
- Capability checks can become stale before execution; the provider may still
  reject the action.
- Confirmation and idempotency add storage and interface work.
- Some actions have ambiguous outcomes after timeout and require follow-up
  discovery.
- A restricted Docker proxy reduces but does not remove the risk of Docker
  control-plane access.

## Alternatives considered

### Read-only product with no actions

This has the smallest risk but omits the explicitly scoped recovery journey.
The narrow allowlist preserves that journey without becoming a general control
surface.

### Global flag and administrator check only

This ignores source permissions, target support, CSRF, mistaken selection,
demo isolation, and auditable intent.

### Generic provider proxy

It would bypass domain validation and rapidly expand authority beyond reviewed
use cases.

### Interface-only confirmation

Browser behavior can be bypassed. The server must bind and verify confirmation
state.

## Implementation and verification

Later phases must:

- prove every gate with allow and deny tests;
- test confirmation expiry, reuse, actor mismatch, target mismatch, and
  parameter changes;
- test viewer, demo, disabled-flag, missing-permission, and unsupported states;
- verify idempotency and ambiguous timeout responses;
- prove that audit-intent write failure prevents the provider call;
- verify audit rows for denied, failed, timed-out, and successful attempts;
- inspect deployment manifests for separate read and write permissions;
- search routes and provider ports for unreviewed generic command paths;
- run optional live action checks only in disposable infrastructure.

## References

- [Architecture overview](../architecture.md)
- [ADR-0002: Provider-neutral boundary](0002-provider-neutral-boundary.md)
- [ADR-0005: Local sessions and CSRF](0005-local-sessions-and-csrf.md)
- [ADR-0007: Deterministic isolated demo](0007-deterministic-isolated-demo.md)
