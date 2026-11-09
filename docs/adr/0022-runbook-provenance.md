# ADR-0022: Recorded document provenance and legacy revisions

Status: accepted implementation design; alpha7.2 source/tag hosted verification and publication pass. See [exact receipts](../evidence/2.0.0-alpha.7.2/README.md). This does not accept later processing guidance or stable2.
Date: 2026-11-08.

Amendment: [ADR-0023](0023-selected-document-guidance.md) extends new approvals
with explicit separate processing permission and a v3 digest. The v2 digest and
absence of processing handoff described here apply to immutable alpha7.2;
existing revisions retain those exact digests and default to reference use only.

## Decision

New collection approvals require an explicit operator record: kind
`operator_authored`, `licensed_copy` or `public_domain`; source attribution up to
512 UTF-8 bytes; usage basis up to 128 UTF-8 bytes; and `use_reviewed: true`.
The record applies to every document in that collection. Split documents with
different sources or usage bases into separately reviewed collections. This is
an operator assertion, not verification of legal rights or factual accuracy.

Attribution is inert visible text, never an executable plan, fetched address or
automatic licence grant. The canonical v2 digest binds these exact fields along
with identity, version, audience and document bytes. The existing two-minute
one-use confirmation and recent grant therefore cannot authorize a substituted
source record. The existing aggregate limit includes all persisted text fields.

Append immutable migration21 to migration20. Nullable source columns preserve
legacy revisions without inventing permissions or changing their old digests.
The database rejects partial provenance records. Existing inert reads may return
legacy text under current audience/session/version/generation checks, explicitly
marked as lacking a recorded review. New approval requires complete provenance;
revocation uses an empty stub. No migration silently reapproves old content.

Catalog metadata and exact document reads expose the recorded attribution to
authorized readers. Reads return separate `document` and nullable `provenance`
objects; imports still contain plain documents and one collection provenance.
The bilingual interface displays source and usage basis as plain text during
review and reading. Legacy absence is visible and cannot be treated as permission
for a document-to-generation handoff. Alpha7.2 implements no such handoff;
the later alpha7.3 candidate requires the separate ADR-0023 permission path.

## Verification and limits

Hosted domain/client tests reject missing, partial, oversized, malformed and
unacknowledged source records. Real PostgreSQL tests bind changed attribution to
the exact confirmation, preserve the original confirmation after failed changes,
retain legacy text and reject partial database updates. Upgrade tests include
the exact shipped migration20 prefix with existing documents and unchanged
ledger bytes. Browser journeys inspect attribution in both languages/roles.

This does not fetch or validate source URLs, prove ownership, settle licence
interpretation, authorize remote disclosure, certify semantic quality or accept
stable2. An operator must establish appropriate rights outside the application.
Existing archive, revocation and restore retention rules remain in force.
