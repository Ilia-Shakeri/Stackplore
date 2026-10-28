# Smart Assistant evidence and privacy policy

[فارسی](evidence-policy.fa.md)

Status: **stage 3 candidate source; not a completed or publicly enabled feature**,
2026-10-25. This guide describes the current preparation code. It does not prove
public admission, browser behavior, remote transmission, deployed privacy or
stage completion. The [stage ledger](implementation.md) owns delivery state.
The accepted rules remain [ADR-0014](../adr/0014-read-only-helper-scope-and-boundaries.md),
[ADR-0015](../adr/0015-helper-durable-runtime-and-volatile-context.md),
[ADR-0016](../adr/0016-helper-authority-and-egress-policy.md) and
[SPEC section 15](../../SPEC.md).

## Source boundaries

The candidate uses one Go service and PostgreSQL. No new HTTP operation or
production evidence registration is added by these packages. Current helper server
preparation composes only metadata maintenance; it does not compose the content
reader or explanation service. A future transport must ship its canonical
contract and access tests with its implementation.

| Component                                                                         | Responsibility                                                                              |
| --------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| [`logsafe.Policy`](../../apps/api/internal/provider/logsafe/policy.go)            | Immutable built-in and added-field masking policy; opaque policy fingerprint.               |
| [`logpolicy.Reader`](../../apps/api/internal/logpolicy/reader.go)                 | Effective settings, masking and bounded log capture outside HTTP.                           |
| [`configuredLogReader`](../../apps/api/internal/httpapi/configured_logs.go)       | Existing configured HTTP log reads delegate to that shared policy.                          |
| [`contextbuilder`](../../apps/api/internal/contextbuilder/builder.go)             | Closed projections, canonical citations, truthful quality and byte limits.                  |
| [`assistantevidence.Reader`](../../apps/api/internal/assistantevidence/doc.go)    | Typed reads, live scope/family checks, sealed volatile evidence and citation resolution.    |
| [`assistantservice`](../../apps/api/internal/assistantservice/service.go)         | Candidate single-resource local explanation; durable reservations and volatile content.     |
| [`PostgreSQL authority`](../../apps/api/internal/postgres/assistant_authority.go) | Separate admission/evidence purposes and current owner/session/control/source/claim checks. |

Trusted server composition registers a shared reader with operator-owned
providers. The browser and returned content cannot register tools, provide
endpoints or host paths, change metric names, follow arbitrary cursors, inject
SQL, read Secret values, or gain shell, exec or provider-write ports. Existing
administrator/viewer observation rights remain installation-wide; selecting a
scope does not create tenant or per-source access control.

## Fixed read registry

`ReadRequest` contains a trusted actor, admitted run, exact source/resource/time
scope, closed step kind, destination and local log consent. It has no question,
URL, path, arbitrary query or extension field. Each read uses one family and no
cursor-following loop. Results must pass their domain `ValidateFor` checks.

| Family    | Fixed data read                                                                                                                 | Current live permission proof                                                                                                                       |
| --------- | ------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| Inventory | Exact selected resource snapshots from resource-detail reads.                                                                   | Fresh inventory capture and matching resource partitions.                                                                                           |
| Health    | `GetHealth` for the selected resources.                                                                                         | Fresh inventory plus actual health read; each selected snapshot must be available and complete.                                                     |
| Metrics   | `GetMetricHistory`, at most 64 points, for CPU usage, memory usage and restart count only.                                      | Fresh inventory plus actual current-metric reads for those fixed metric names and every selected resource.                                          |
| Events    | One `ListEvents` page, limit 64, exact scope/window and no free-text search.                                                    | Fresh inventory plus a validated one-item event query.                                                                                              |
| Topology  | One relationship page, limit 64. Keep only edges whose two endpoints are selected; mark remaining data when edges are excluded. | Fresh inventory plus a validated one-item relationship query. A partial topology proof is rejected; a derived edge can depend on another partition. |
| Logs      | Shared `Capture`, selected targets/window, at most 64 lines before effective settings and 32 KiB of message bytes.              | Fresh inventory plus shared policy-applied log read, explicit local consent and a later same-query policy check.                                    |

`CaptureAuthority` receives the actual operator-bound provider, not only a
persisted observation reader. `Source.Read` may serve validated stored facts;
its support enum alone grants no live permission. Missing family ports, denied
partitions, mismatched identities and unavailable live proof fail closed. Live
authority must be available, not future-dated and at most five seconds old.
Demo uses fixed synthetic times and is explicitly exempt from that age check.
This does not promise detection within five seconds of an unobserved revocation.

Unavailable or unsupported data stays distinct from a denial. Missing metrics
are not zero. A partial page is not a complete estate scan. These reads are not
one atomic infrastructure snapshot. The metadata and individual records retain
their own observation times and quality.

## Fields that may enter a bundle

Every document has a version, source handle, provider kind, source-configuration
version, policy version, restore generation, exact UTC window, quality and
`more_available`. Quality contains availability, completeness, observation time
and a synthetic flag. Each record has its server citation ID and closed kind.

| Record       | Additional projected fields                                                                                           |
| ------------ | --------------------------------------------------------------------------------------------------------------------- |
| Resource     | Normalized resource handle, opaque version handle when present, safe health state.                                    |
| Health       | Resource handle and normalized inventory health state. Health prose and evidence messages are omitted.                |
| Metric       | Resource handle, closed metric name/unit, finite numeric value, sample time and whole-second interval.                |
| Event        | Resource handle, severity and occurrence time. Summary, arbitrary kind/reason text and other event prose are omitted. |
| Relationship | Endpoint handles, closed kind/direction and validated provenance/certainty/confidence values.                         |
| Local log    | Resource handle, timestamp, severity, stream, policy-applied message and redacted/truncated flags.                    |

Unreviewed display names, native identifiers, labels, annotations, arbitrary
resource text, raw versions, event summaries, health prose, runbook text and
notification targets are not added by this projection. Secrets and credentials
remain forbidden even for a local destination. Masking known patterns is not
proof that arbitrary text is safe; construction can reject a request.

Citation IDs are `ev_` plus 64 lowercase hexadecimal characters. They bind the
server run, source, versions, restore generation, exact window, object identity
and projected record content. Export does not reissue an ID or change its
record. Remote source/resource/version/endpoint handles are also scoped to one
run. Exact lookup identities stay in the local citation map, not the remote
payload. A handle grants no authority and is never an arbitrary link or action.

## Context and wire bounds

| Boundary                                 | Hard cap                                                                                                                             |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| Registered sources per reader            | 64; source IDs must be unique.                                                                                                       |
| Selected resources per read              | 32, unique and bound to the exact source.                                                                                            |
| Reader time window                       | 24 hours; log window at most 15 minutes.                                                                                             |
| Base builder                             | 256 records and 64 KiB serialized document; callers can lower both.                                                                  |
| Candidate reader/export                  | At most 64 records/items, with tighter byte caps still applied.                                                                      |
| One exported `Item.Text`                 | 8,192 bytes of valid JSON.                                                                                                           |
| Explicit `[{"id":...,"text":...}]` array | 65,536 bytes including IDs, string escaping, brackets and commas.                                                                    |
| Local log projection                     | 200-line design ceiling, 32 KiB of message bytes, 4,096 UTF-8 bytes per line; this reader requests at most 64 lines before settings. |
| Complete processor input                 | Separate 64 KiB formatted-input cap and 16,000 verified input-token cap.                                                             |

[`Bundle.Items()`](../../apps/api/internal/contextbuilder/items.go) repeats the
full source/version/window/quality envelope around one unchanged record. It adds
zero-based `item_index` and total `item_count`. It checks every canonical ID and
reference, each text and the escaped array. It returns all items or a fixed
budget/validation error with no prefix. It does not silently drop selected facts
or truncate already-cited content. An escaped 4 KiB log line can exceed the 8 KiB
item cap and must then be rejected. Existing pre-citation UTF-8 truncation is
explicitly flagged. Remaining page data stays marked.

A valid empty bundle exports an empty array and invents no citation. Do not
infer healthy or complete infrastructure from that array. Retain the owning
use case's unavailable/unsupported state. The initial explanation service
requires exactly one resource item and rejects an empty or mismatched result.

The item-array cap is not the full request budget. Locale, question, system
text, JSON/schema/template framing and special tokens still need the gateway's
complete input validation and exact pre-dispatch counter. A byte estimate or
post-generation usage count is not that proof.

## One shared log policy

The configured HTTP wrapper delegates to `logpolicy.Reader.ReadLogs`. It reloads
valid application settings before the provider call and applies the tighter
requested/effective line count. It preserves ordinary provider built-in masking
and adds configured masking fields. A helper log read uses `Capture`, which
always reapplies built-in and added masking, validates the returned page and
seals copies. A raw provider page cannot be supplied in place of a capture.

`logsafe.Policy` normalizes and sorts immutable added-field names and includes a
versioned built-in policy identity in its SHA-256 fingerprint. Equivalent field
order/case/separators have the same identity. Changing built-in masking semantics
requires a new built-in identity. Added fields cannot remove built-in protection.

The capture fingerprint combines that policy identity with the actual
`application_settings.Version` snapshot used before the read, effective line and
byte limits, built-in byte/rune line caps, query-window duration and maximum
lookback. It contains no target, log text, source identity or absolute timestamp.
Settings-free test captures use version zero; production evidence registration
requires a settings reader. An unsealed capture has an all-zero fingerprint and
is invalid. A masking identity change during capture rejects the result.

Sealed log evidence retains consent and this fingerprint. Before content use or
citation resolution, the reader repeats the same trusted bounded log query and
compares the new policy fingerprint. A one-line permission probe alone is not
that comparison. New body text is discarded, not substituted into the old cited
record. Changed settings versions, added masking fields or effective limits
invalidate the old content even if the old citation still exists. Replacing
added-field configuration must replace the shared immutable policy for both
paths and invalidate dependent volatile content.

## Authority, source pin and sealed citations

`AuthorizeAdmission` checks only a zero-version candidate and current metadata;
it does not perform a provider read or grant stored-run access. `Authorize`
checks persisted-run metadata only. Status polling uses the owner's repository
read and does not need provider reachability. Neither method releases content.
`AuthorizeEvidence` is required before every use of collected content.

The metadata gate binds the current session/auth revision, owner, installation
role, control/profile/policy, source enablement/configuration and restore
generation. Admission and stored evidence use separate repository methods.
Queued/completed snapshots require exact metadata. A running read may observe
forward version/time/counter/lease progress only within the same attempt, fence,
lease owner and fixed run identity. A future or changed snapshot is rejected.
This read rule does not relax any repository mutation's compare-and-swap check.

Each registered `Source.ConfigurationVersion` is positive and must equal
`Scope.ConfigurationVersion` and `Run.SourceVersion`. It is the source version,
not the runtime control's separate `Run.ConfigurationVersion`. The same source
ID with a new version cannot reuse an old registered adapter.

`Evidence` has private fields. It is sealed to the originating reader instance,
actor/session, admitted run, selected scope, read family and local log policy.
Passing a caller-made reference or zero value does not create a valid seal.
The volatile lifetime ends 30 minutes after admission. Waiting for a provider
slot is bounded by that lifetime and, while running, by the remaining lease and
run deadline too.

Content checks repeat current metadata and the same live read family. Log checks
also repeat the effective policy check. Citation resolution requires the same
owner/session, the same admitted attempt and a successfully completed run with
the expected next fence and newer version. It returns only a closed kind and
normalized locator; transport must map this to its own fixed authorized view.
Unknown IDs, another reader, changed scope/configuration/policy/restore state,
expired content and uncertain usage are denied.

Source rotation is a composition requirement, not a shipped hot-reload command:

1. Commit the source configuration/version change through its owning use case.
   Current database authority must reject the old version.
2. Fence and clear old source-bound volatile state before cancelling its work;
   use the owning service's source invalidation path.
3. Build new operator-bound providers and a new shared reader with the exact new
   source pin and the same effective log policy. Discard the old binding.
4. Require fresh authorized work. Do not transplant old evidence into the new
   reader or automatically resend an uncertain call.

## Independent bounded lanes

| Lane             | Operations                                            | Bound                                                                                              |
| ---------------- | ----------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| Metadata         | Admission checks and persisted-run metadata checks.   | Two slots; at most three seconds including slot wait, or the tighter configured/parent deadline.   |
| Provider/content | Reads, content authorization and citation resolution. | Two slots; at most 25 seconds including slot wait, further bounded by lease/run/volatile lifetime. |

Two blocked provider reads cannot consume the metadata slots used for ownership
and lease progress. The reader owns no background goroutine, cursor loop or
unbounded queue. Composition must share its instance; constructing one per
request would not enforce an installation-wide bound. Owning worker, database,
admission and transport limits remain separate requirements.

One logical read reservation can perform multiple bounded authority probes:
at most four inventory captures, three family probes, three passes over the
selected resources (32 each), six metadata checks, one capability read and one
family data read. Logs add a same-query policy capture and final metadata check.
All share the read context. These are validation work, not extra processor-chosen
actions. These counts are code bounds, not capacity or latency measurements.

## Local input, remote defaults and first content I/O

Local free-form questions remain bounded and volatile. Local log consent is
explicit, does not waive field protection and never authorizes remote log
transmission. The current service candidate permits only the single-resource,
unpaid local inventory explanation; the six-family reader is broader preparation.

Remote projection accepts only structural fields and per-run opaque handles.
It rejects log capture. That projection is not permission to use a remote
processor. The generic gateway still has a separate `Question` field; future
remote composition must supply only reviewed server-owned templates and typed
parameters. It must not copy a free-form question, log body, annotations, event
prose or runbook text into that field. No remote public workflow or adapter is
enabled by this candidate. Broader text needs the separate accepted decision,
operator enablement, notices and wire proof required by ADR-0016.

Preparation itself sends content: the local counter calls the bound service's
template/tokenization endpoints before generation. The candidate service checks
sealed content authority after the inference reservation and **before
`Gateway.Prepare`**, again after preparation and before dispatch, and before
publication/replay. Checking only before generation would be too late for the
first content-bearing request. Profile network rules and the full token counter
remain required; see the [local profile guide](local-profile.md). Already-sent
bytes cannot be recalled after a later revocation.

## Volatile, database and telemetry boundaries

Questions, projected bodies, log text, counted tokens and returned text stay in
bounded private memory. The owning service limits working history to 30 minutes
or eight turns and fences/clears its state on expiry, cancellation, session/source
invalidation, shutdown and ownership loss. Lost text is not reconstructed after
restart. Typed metadata and unresolved usage keep the separate lifecycle in the
[runtime guide](runtime.md); ordinary cleanup must not release unknown usage.

Do not put raw content into SQL, audit, telemetry, error payloads, crash reports,
service-worker/offline caches or durable replay. Safe errors are the fixed
`assistant.evidence_invalid`, `assistant.evidence_denied`,
`assistant.evidence_unavailable`, `assistant.evidence_budget` and
`assistant.evidence_cancelled` codes. Upstream error text is not forwarded.

Ordinary formatting/JSON of request, bundle, item, capture, policy and sealed
result wrappers hides private content. Explicit bundle JSON, `Item.Text`, inner
transport fields and answer sections are intentional plaintext access and must
never be logged. These guards do not promise physical erasure of every runtime
copy or removal from prior backups. Provider retention and host privacy need
their own reviewed deployment proof.

## Verification and remaining gates

Focused local checks pass for masking/capture/projection, Reader/service and
shared HTTP log behavior. On 2026-10-25, actual PostgreSQL checks also passed for
committed admission outcomes, rollback, current authority and one grounded
dispatch with both a synthetic backend and the production HTTP adapter against
a bounded synthetic protocol server. Every content endpoint checked the committed
SQL reservation; closed wire fields and all owned SQL tables were checked for
absent private text. These checks do not prove a real engine, deployed provider
permissions, telemetry privacy or human answer quality. See the
[candidate evidence](../evidence/2.0.0-alpha.3/README.md). The full candidate gate,
exact-source hosted checks and accepted branch measurement remain separate gates.

After preparing the [pinned tools](../development.md), the focused commands are:

```powershell
go -C apps/api test -p 1 ./internal/provider/logsafe ./internal/logpolicy ./internal/contextbuilder -count=1 -timeout=2m -cover
go -C apps/api test -p 1 ./internal/assistantevidence ./internal/assistantservice -count=1 -timeout=2m -cover
npm.cmd run verify
```

Record the exact candidate source, tool versions, full logs, exit codes and
coverage kind. Statement coverage does not prove the accepted branch-coverage
target. Use an isolated real PostgreSQL fixture for migration/authority/claim
tests; mocks cannot prove SQL. Required negatives include source-version
rotation, owner/session/role/profile/policy/restore changes, future/stale claims,
same-family denial, changed log policy, blocked read lanes with working metadata
and cancellation, revocation before counting, malformed or over-budget evidence,
UTF-8/escaping, invented citations and hostile data treated only as data.

Before any production content route is enabled, capture synthetic requests for
every supported profile and prove the exact allowed inventory, complete input
counting and absence of raw remote questions/logs. Prove current authority at
counting, dispatch, publication and replay; inspect database/telemetry/cache
outputs for absent raw content. Add the owning bilingual browser and contract
proof when those public surfaces are implemented. Keep all failed evidence and
limitations in the stage ledger.
