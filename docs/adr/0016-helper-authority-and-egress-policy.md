# ADR-0016: Share helper evidence privacy and bind inference egress

- Status: Accepted
- Date: 2026-10-19
- Owners: Project maintainers
- Scope: Version 2.0 target design; accepted during roadmap stage 0
- Clarifies: ADR-0002, ADR-0005, ADR-0006, ADR-0013
- Implementation status: Decision accepted; shared policy, adapters and transmission proof are not shipped by this record

## Context

Normalized infrastructure facts and free-form text have different disclosure
risks. Resource names, annotations, events, logs, runbooks and pasted questions
can contain credentials, private business data or hostile instructions.
Inference output is also untrusted. Masking known patterns does not establish
that arbitrary text is safe to transmit.

V1 additional log-field redaction and effective line limits are applied by an
HTTP wrapper. A helper that reads the provider directly would bypass those
operator choices. Notification networking has separate public-address guards;
supporting an internal inference endpoint must not weaken them.

## Decision

### Shared privacy policy before tool use

Extract effective log limits and added masking fields from
`httpapi/configured_logs.go` into a shared use-case policy before any production
helper log tool is registered. The HTTP and helper paths consume that same
policy. Default built-in masking remains protected. No provider-only shortcut
may bypass effective settings or current authority.

Context construction uses closed structural allowlists, minimization,
bounded fields and explicit quality states before pattern masking. Passwords,
raw tokens, session identifiers, provider credentials, private keys,
kubeconfig content, Secret values and full notification targets never enter
helper context, helper persistence, telemetry, output or test fixtures.
Errors use fixed safe codes and bounded metadata, not upstream payloads.

Local free-form input and locally opted-in log bodies remain volatile and
bounded under ADR-0015. User consent to a log read does not waive field
protection or authorize external transmission. Stop safely when a permitted
representation cannot be built; never claim universal free-text secret detection.

### Remote defaults and approved content

External transmission is off by default. An enabled remote profile accepts
only approved server-owned question templates and safe typed evidence fields.
It does not accept free-form questions, raw log bodies, resource annotations,
arbitrary event prose, unreviewed display names or retrieved runbook text.
Template parameters cannot smuggle arbitrary text through a typed identifier.

Define the initial remote field inventory explicitly: typed goal and resource
kind codes, approved numeric measurements and units, bounded timestamps and
quality/severity codes, and per-run opaque evidence handles. Resolve handles
to local resources only inside Stackplore. A server-issued handle does not
make an otherwise forbidden payload safe to send.

Do not broaden that inventory by toggling a generic permissive flag. Broader
text transmission requires a new accepted privacy decision, field review,
explicit operator enablement, updated notices and real wire-level proof.
Provider retention, training use, regional processing, subprocessors and
deletion terms must be reviewed for each enabled remote profile. Application
configuration does not prove control over provider storage or future terms.

### Operator-bound profiles and network access

The operator binds each profile to an exact approved scheme, hostname, port,
credential reference, trust roots, capability set and usage terms. The browser
may select only a safe profile ID; it never supplies an endpoint, host path,
credential, resolver, proxy or request header. The prompt, tool data and
inference response cannot change these bindings.

Remote profiles use verified TLS and approved public addresses. Local profiles
may use exact operator-approved private destinations; outside loopback they
require verified TLS. An explicit loopback-only local profile may use HTTP.
Private-profile support is not unrestricted access to the local network.

Every connection attempt revalidates scheme, exact host/port, all resolved
addresses and the profile allowlist, pins the selected address, verifies TLS
identity, and rejects redirects. Bound DNS results, connect/header/total
timeouts and decoded response bytes. Do not inherit an ambient proxy or use
an unrestricted client. Refreshing DNS or retrying a safe connection setup
must not bypass policy or resend an uncertain inference call.

Deny metadata endpoints, link-local and multicast addresses in every profile,
including IPv4/IPv6 and alternate textual forms. Disallow userinfo, fragments,
unreviewed request paths and resolver redirection. Public remote policy also
denies loopback/private/reserved addresses; local exceptions require their
own exact operator bindings.

Keep notification public-address, redirect, timeout and response-size guards
unchanged. A new inference transport has its own reviewed policy and tests;
do not export a generic network proxy as a helper tool.

### Untrusted content, authority and output

Resource metadata, logs, events, runbooks, questions and returned text are data.
They cannot redefine the system goal, enable tools, expand scope, change
quotas, reveal protected fields or authorize writes. Parse closed tool and
response schemas, reject unknown fields and tool names, validate size/depth,
and enforce policy again before each dispatched read.

Resolve the actor's current session/auth revision and installation-wide role,
the run owner, source enablement/configuration, provider capability and
published observation authority before each step and before publication or
replay. Scope filtering does not add per-source or tenant access control.

When authority is observed revoked, fence queued/replay output before
cancellation and reject a late worker completion. Restore-generation changes
invalidate prior run and stream authority. Do not infer access from an old
evidence reference, cached response or prior successful read.

Render only validated answer sections and current server-owned citations.
Never stream raw unvalidated tokens into the page. Reject arbitrary links,
images, active markup, generated action buttons and fabricated references.
Private API, stream and helper content must not enter offline/service-worker
caches. Clear owner-bound volatile view state on session or account changes.

## Consequences

### Benefits

- Both HTTP and helper log reads honor the same operator privacy policy.
- Remote mode has a small reviewable disclosure surface by default.
- An internal inference profile does not weaken existing notification safety.
- Tool and rendering controls remain independent of returned instructions.

### Costs and risks

- Structured remote templates provide less flexibility than local free-form input.
- An allowed numeric or categorical fact can still be sensitive in context.
- Operator profile and provider-term reviews require ongoing maintenance.
- A local runtime and its host remain part of the trusted deployment boundary.
- Once a permitted field leaves the installation, application deletion cannot recall it.

## Alternatives considered

### Send all observations after regular-expression masking

Patterns cannot prove that arbitrary free text contains no secret or private
data. Structural allowlists and explicit transmission decisions are required.

### Let the browser choose any compatible endpoint

This creates credential-exfiltration and network-pivot paths. Profiles are
operator-bound and browser inputs select only safe identifiers.

### Relax notification guards for internal inference

That would expand an unrelated outbound surface. Dedicated profile-aware
inference policy preserves notification invariants.

### Trust retrieved instructions or generated links

Evidence and output have lower trust than server policy. Schema validation,
authority checks and safe rendering remain mandatory regardless of wording.

## Implementation and verification

Before dispatch becomes reachable, prove shared added-field masking, effective
limits, safe context construction and absent raw persistence. Capture actual
synthetic requests for each supported profile and assert the approved field
inventory; profile configuration alone is not transmission proof.

Test DNS rebinding, mixed allowed/forbidden answers, redirects, TLS mismatch,
private/metadata/link-local/multicast destinations, oversized/slow responses,
unknown profiles, path/header injection, proxy inheritance, changed source
authority and cancellation. Use both local and remote policy cases. No live
integration credential is available to untrusted pull requests; paid tests
use protected jobs, synthetic data and a hard budget cap.

Test returned and retrieved instruction attacks, hostile names and prose,
fabricated citations, cached private responses, cross-owner replay, account
changes, source denial during a read, restore invalidation and safe rendering
in both languages. Run existing notification security tests unchanged.

Until real profile and wire receipts exist, operator configuration, test
adapters and an accepted decision are not claims of deployed privacy or
connectivity. Subsequent stages record exact source, profile revision and limits.

## References

- [Provider-neutral boundary](0002-provider-neutral-boundary.md)
- [Local sessions and CSRF](0005-local-sessions-and-csrf.md)
- [Scoped inventory authority](0013-provider-coverage-and-demo-lifecycle.md)
- [Helper scope](0014-read-only-helper-scope-and-boundaries.md)
- [Durable runtime and volatile context](0015-helper-durable-runtime-and-volatile-context.md)
- [Current configured log wrapper](../../apps/api/internal/httpapi/configured_logs.go)
- [Existing notification outbound policy](../../apps/api/internal/notification/outbound.go)
- [Version 2 roadmap](../roadmap-v2.md)
