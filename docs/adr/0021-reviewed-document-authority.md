# ADR-0021: Reviewed document authority and exact references

Status: accepted implementation design; hosted release acceptance pending.
Date: 2026-11-08.

## Context

Operating notes are useful for investigation, but prose cannot grant read or write
authority. A document identifier alone is insufficient when its text, audience or
installation changes. Search terms and draft bodies must not enter durable run
history or audit. The existing PostgreSQL and account boundary should own this
feature without another service, store or executable extension.

## Decision

Keep inert English/Persian documents in explicitly approved corpus revisions.
The canonical length-framed digest binds identity, revision, audience and exact
document bytes independent of input order. One-use two-minute metadata previews
bind actor/session, action, expected version, digest and restore generation.
Approval and revocation consume a separate `runbook.manage` recent grant in the
same transaction as content, catalog revision, confirmation and safe audit.

Live database-clock session/role checks surround reads. A shared catalog lock
fences replacement against search/read transactions. Search uses fixed simple
full-text configuration, exact language, audience filtering, deterministic keyset
order, a ten-row page and a current generation/catalog fence. Opaque cursors bind
the current actor/session, role/revision, query and page parameters for ten minutes.
The query binding is keyed; no query text or unkeyed query hash is serialized.

A document reference names corpus ID/version/digest, document ID/language and
restore generation. Opening it performs a new authorized read, and rejects old
versions, changed audiences, revocation and restores. Approval does not certify
semantic accuracy. Document prose remains outside inference payloads and tool
plans in this increment; there is no automatic retrieval-to-generation handoff.

There are at most 16 corpus identities, including revoked tombstones, each with
32 documents, 16-KiB bodies and 512-KiB aggregate material. Existing identity
tombstones preserve monotonic versions and prevent stale-reference resurrection.
Preview stores no draft prose. Replacing/revoking deletes old live document rows
and their derived index entries; PostgreSQL MVCC, WAL, replicas and backups retain
their own lifecycle. No secure-erasure or automatic approval expiry is claimed.

The private desktop page separates search/read and administrator review. Private
POST bodies keep queries/references out of URLs. Current session, CSRF, same-origin
and purpose checks remain mandatory. Text is displayed as plain text, with no
HTML/Markdown execution or navigable prose links. Unknown commit outcomes require
catalog inspection before a fresh preview. No automatic retry is allowed.

## Consequences and verification

The normal product can search approved notes while its optional processor is
disabled. Guest deployments omit the service. No external text store, crawler,
embedding system or host path is introduced. Exact-language simple tokenization
does not normalize Persian/Arabic letter variants or provide relevance ranking.

Real PostgreSQL tests cover atomic audit failure, changed material, replay,
revoked sessions, locale/audience boundaries, replacement, restore, confirmation
expiry/replacement and identity quota. Browser journeys cover both languages,
administrator approval/revocation and viewer reads. Hosted query-plan inspection
uses bounded synthetic fixtures and is not a production capacity claim. Source,
exact-tag, scanned artifacts and independent product acceptance remain distinct.
