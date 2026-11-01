# ADR-0019: Request-local compact citation handles

- Status: Accepted
- Implementation: Candidate; exact-source hosted verification pending
- Date: 2026-11-01
- Owners: Project maintainers

## Context

The real five-read workflow succeeds at `4eb5d7e`. Another hosted CPU run at
`1f52587` reaches the unchanged 40-second Persian generation limit with 1422
input tokens. Successful replies need about 124 output tokens for one fact,
including its long opaque citation. Failures remain failed. Increasing deadlines,
removing Persian or relaxing exact facts/citations would conceal the problem.

## Decision

The local transport assigns validated ordered references compact handles `e1`
through `e64` for one counted request. Only outer evidence `id` changes on the
private wire; evidence text, scope, quality and sealed proofs remain intact.
Exact formatted input includes these handles. The generation schema admits only
that request's handle set. Tables stay volatile and independent per request.

Complete canonical envelopes with exactly matching version, source, provider,
configuration, policy, restore generation and window may share one scope header.
Their remaining per-item fields are encoded as JSON data without double-quoting
the entire document. Retain every original record, embedded canonical citation ID,
quality/time, page/index/count and coverage value. Mixed scopes, missing/unknown
envelope shapes or a mismatched record ID fall back to unchanged opaque text.
Factoring grants no authority. Prove lossless reconstruction and preserve JSON
escaping so untrusted role delimiter text cannot become literal prompt framing.

Before returning complete output, strictly decode and validate closed JSON against
the handle set. Reject unknown/duplicate handles, raw canonical IDs, active text
and malformed output. Restore only the bounded bijection to original references.
Re-serialize and enforce the canonical 64-KiB byte cap after expansion. The shared
gateway independently validates the original canonical IDs before publication.
Public contracts, proof identity and citation locators never use compact handles.

Freeze gateway reference authority before backend counting; pass detached evidence
and coverage slices so a backend cannot rewrite caller/admission authority.
Usage remains actual counted/generated tokens, independent of expansion bytes.
Single-use dispatch, uncertainty, current authorization, privacy, cancellation,
lease, queue, byte/token and run/call budgets remain unchanged. No model/engine/
template pin, worker count, default-off policy or schema version changes.

## Verification and consequences

Prove handle order, per-request isolation, immutable caller/reference authority,
unknown/duplicate rejection, canonical expansion bounds, strict final validation
and actual usage. Run full hosted source/race/database/install/provider/browser
checks, the six prior direct events and eight real desktop journeys. Retain
failures and source/profile-bound timings; p95 and capacity remain separate.

The adapter gains bounded post-processing; consumers keep canonical references.
No execution runs on the user's workstation. This record does not accept source,
performance, quality or a release. Alternatives rejected: arbitrary bindings,
global mutable alias tables, persistent aliases, unmatched IDs, more deadlines
or weaker semantic assertions.
