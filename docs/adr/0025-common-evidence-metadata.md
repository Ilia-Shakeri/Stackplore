# ADR-0025: Lossless common evidence metadata

- Status: Accepted design; exact-source hosted verification pending
- Date: 2026-11-15
- Owners: Project maintainers
- Amends: [ADR-0019](0019-request-local-citation-handles.md) and [ADR-0024](0024-compact-observation-reference-metadata.md), private wire encoding only

## Context

Real hosted workload source `6d322de` records a1.5 busy/idle p95 ratio; `48266ca`
records2.24 under lower processor priority. A three-thread attempt `fcbc20d`
reaches the unchanged40-second Persian incident deadline on one worker. All
failures remain in the [stage9 ledger](../evidence/2.0.0-alpha.8.3/README.md).
Repeated envelope metadata increases prefill cost without adding new observations.

## Decision

After the existing complete-envelope, record-binding and identical mandatory
scope checks succeed for the entire group, optionally move `quality`,
`more_available`, `item_index` and `item_count` into the common scope only when
that complete field's original JSON bytes are identical in every item. A differing
value remains in its original individual item. Records are never moved or merged.
The seven mandatory scope identity fields still require exact equality; unfamiliar
shapes, mismatched references or mixed identities retain full opaque fallback.

Reconstruct each original twelve-field envelope by merging its residual fields
with the common header and restoring the request-local citation binding. No field
may occur in both locations. No quality, pagination, fact, uncertainty, technical
value, question, proof or original caller evidence is discarded or rewritten.
JSON escaping and inert-data treatment remain mandatory. Guidance retains its
existing owning projection from original evidence. Unknown output handles still
reject before canonical expansion and independent gateway validation.

## Consequences and verification

The private header can contain7–11 fields; the residual envelope contains the
complement, and reconstructed keys must be exactly the original twelve. Count
and generation continue to use identical formatted bytes. This grants no read
authority and changes no public contract, schema, profile pins, output/input caps,
single-use dispatch, uncertain usage accounting or40-second call budget.

Test lossless reconstruction, identical versus differing quality/pagination,
immutable caller text, mixed-scope fallback, guidance separation and canonical
citation restoration. The actual SQL/adapter fixture validates the complete
reconstructed shape rather than assuming an invariant seven-field private header.
Require fresh hosted source, real bilingual desktop, protocol, development quality,
load and signed security/package results. Expected savings are not a measured
latency improvement until their owning receipts exist.

Alternatives rejected: longer deadlines, lost quality/facts, changed questions,
retries, looser output checks and whole-text replacement. Independent review and
pilot acceptance remain separate from this encoding decision.
