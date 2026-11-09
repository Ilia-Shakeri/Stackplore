# ADR-0023: Explicit selected-document guidance

Status: accepted implementation design; hosted acceptance pending.
Date: 2026-11-09.

## Smart Assistant decision

An operator's reference-use review does not permit assisted processing. New
approvals explicitly include `assistance_reviewed: true` or `false`; the canonical
v3 digest binds that choice. Migration22 defaults existing revisions to false,
preserving existing text, digests and manual reads. Enable processing only through
a separately reviewed new version. No rights are inferred from legacy metadata.

The desktop user explicitly searches in the current language, opens an exact
current result, sees a bounded excerpt and consents before attaching one document
to a question. The private request contains consent and the six-field exact
reference, never caller-supplied document prose or an address to fetch. Selection
is bound to the admission fingerprint. No automatic search, crawler, training,
unbounded retrieval or remote processing is introduced.

Only the server-composed local profile enables this path. The owning reader
requires a durable `reviewed_document` reservation, then independently checks
live account/session, database-derived audience, exact corpus/version/digest,
restore generation and separate processing permission. Metadata authorization
is repeated before disclosure. Stale/revoked authority fails closed. Source
configuration and normal evidence ownership fences remain necessary.

The context includes at most 256 UTF-8 body bytes with its exact title and explicit
guidance marker. The counted processor envelope also carries trusted usage
classification, with localized system rules that never incorporate document
prose. Exact corpus/version/digest/document/locale metadata stays in the sealed
server witness instead of processor text; its private revision binding remains
part of the canonical citation fingerprint. The local adapter places the bounded
title/excerpt/truncation and request-local handle in a separate `guidance` array,
not in observation records. Unrecognized internal guidance remains opaque user
data. No document prose enters system instructions. Observation handles and
the complete canonical citation mapping stay unchanged; exact formatted token
counting includes both arrays before the single dispatch.
Within that counted request, observation handles use `eN` and guidance handles
use `gN`. The gateway derives and copies the classification from its trusted
original evidence before calling a backend. Counting and generation use the same
closed mapping; unknown or repeated guide IDs are rejected. This extends the
internal ADR-0019 encoding only: public canonical citations and authority do not
change, and post-generation fact/citation validation remains mandatory.
It cannot enter the observed-facts section or deterministic
incident timeline. Typed document citations can support hypotheses/next checks;
the witness is reopened through a fresh authorized exact-reference read. Normal
observation citations retain a null document reference.

Explain uses two reads with a document; incident uses eight. Documents and log
consent cannot coexist in one plan. Existing eight-read, one-dispatch, 40-second
inference, context/output and retention caps remain unchanged. Questions, selected
references, excerpts and generated prose remain volatile; history metadata export
still contains only its existing closed eleven fields. Restart cannot reconstruct
an answer. Ownership, language, resource and page-exit loss clear private views.

## Verification and rollout

Unit and integration cases cover explicit consent, exact-reference parsing,
reservation order, permission withdrawal, live session, digest changes, fact
rejection, UTF-8 clipping and old migration prefixes. Hosted bilingual
administrator/viewer journeys exercise actual approval, search, local processing,
typed citation and witness clearing. These are functional synthetic checks,
not independent semantic quality or production capacity evidence.

The desktop witness body has a bounded keyboard-scrollable area. Closing it
returns focus to the question; selection close/include returns focus to the
persistent guide heading. Hosted journeys reopen the fresh witness before
page-exit clearing so keyboard checks do not replace the privacy assertion.

Ship matching server and web assets together. Clients now require the nullable
`document_reference` citation field and the boolean provenance permission.
Back up before the forward-only schema22 upgrade. Existing approvals stay
reference-only; no implicit permission expansion. Stable2 remains gated on the
separate quality corpus, independent evaluation, pilot and artifact acceptance.
