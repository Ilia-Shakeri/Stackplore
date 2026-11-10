# ADR-0024: Compact observation reference metadata on the private wire

- Status: Accepted
- Implementation: Source `01b8cfc` passes all eight hosted jobs, real bilingual protocol/application, three development passes and signed trial. Independent quality and release acceptance remain separate. [Receipts](../evidence/2.0.0-alpha.8.2/README.md).
- Date: 2026-11-10
- Owners: Project maintainers
- Amends: [ADR-0019](0019-request-local-citation-handles.md), embedded observation citation metadata only

## Context

Source `af93c95` passes all three public development passes and 800 service
boundary observations. Its real desktop Persian incident reaches the unchanged
40-second dispatch limit with 1,226 counted input tokens. The observer records
client closure and incomplete generation; the owning run correctly stays uncertain.
The failure remains in the [attempt ledger](../evidence/2.0.0-alpha.8.2/README.md).
Each of seven observation records repeats a long opaque citation already bound
to its request-local outer handle. This metadata consumes input without adding facts.

## Decision

For a complete matching canonical envelope group only, replace the literal
observation record's `citation_id` with its already-bound outer handle. Require
exactly one record and an exact original-ID match. Case-variant metadata keys
are ambiguous and make the entire group fall back to untouched opaque input.
Mixed scopes or unfamiliar envelope shapes retain the same full-group fallback.

Change no question, observation fact, resource identity, nested field, quality,
coverage, time, pagination value or original caller evidence. Guidance records
retain their canonical metadata for the existing separate owning projection.
The private request-local bijection still restores canonical answer citations;
the gateway independently validates them against original admitted references.
No compact handle reaches public reports, witness locators, proofs or persistence.

Counting and generation use the identical resulting input. Actual token accounting,
uncertain usage, no-retry dispatch, schema/byte caps, engine/profile pins, parallelism,
40-second call and run deadlines remain unchanged. This is input formatting and
grants no authority or additional read capability.

## Consequences and alternatives

Expected benefit is less redundant input; actual token and latency changes require
fresh owning processor/application receipts. Added cost is one bounded record
projection and reconstruction through the private binding for lossless checks.
This supersedes only the earlier requirement to retain the embedded canonical
observation ID on the processor wire. It does not change canonical evidence.

Longer deadlines, fewer facts, changed expected answers, dropped Persian cases,
inference retries and relaxed output checks are rejected. Blind whole-text
substitution is rejected because it could corrupt facts or untrusted content.

## Verification

Test lossless reconstruction through the binding, unchanged caller input, exact
facts/nested IDs, guidance separation, role-delimiter escaping, full-group fallback
and rejection of unissued output handles. Run fresh hosted source/race/database,
both-locale real protocol and all owning desktop journeys, three development
passes and signed package/security gates. Keep the failed baseline. No claim of
same-hardware speedup, production capacity, human quality or stable acceptance.
