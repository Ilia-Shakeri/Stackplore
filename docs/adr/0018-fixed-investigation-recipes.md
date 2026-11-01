# ADR-0018: Fixed bounded investigation recipes

- Status: Accepted
- Implementation: Stage-5 internal candidate; public mode remains unshipped
- Date: 2026-11-01
- Owners: Project maintainers

## Context

The local resource conversation owns one typed inventory read and one closed
generation. Existing runtime goals and evidence ports already represent other
read families. Adding several reads must preserve independent family authority,
durable reservations, explicit log consent and honest coverage. Text returned by
a source cannot grant another capability or define a tool call.

## Decision

The server owns finite recipes. Ordinary explanation keeps inventory alone.
An explicitly selected investigation reads inventory, health, metrics, events
and topology in that order; a sixth log read requires explicit consent for the
same local run and a window of at most fifteen minutes. No recursive steps,
model-chosen names/arguments, arbitrary endpoints or cursor following exist.

Each logical read reserves a durable step before I/O. Additional families use
at most eight records and 8192 projected bytes per read. A read may include
bounded authority/metadata checks; one logical step is not a promise of one
network request. Existing aggregate time/call/queue/token/byte limits remain.
The mandatory selected-resource inventory record remains required.

Retain each sealed proof separately. Reauthorize every retained family before
counting, dispatch, publication and content/citation release. Any denial clears
the complete answer and all proofs. Empty/unavailable families contribute typed
coverage metadata and no invented evidence IDs. Closed coverage includes family,
availability, completeness, record count and whether additional data exists.
Its bytes and every actual record enter exact input counting. At most one final
generation occurs; uncertainty is never retried automatically.

Goal and explicit consent are bound to the keyed admission fingerprint. No
schema migration is required: schema 18 already owns these goals and step kinds.
Default-off operation, selected-resource scope, remote policy and existing
ordinary explanation semantics remain unchanged. Public contract, desktop
control and acceptance land separately; this record alone ships no new mode.

## Consequences and verification

Work stays finite and explainable, at the cost of not adapting its read list to
arbitrary question text. Missing capability remains a gap, never proof of health
or absence. One resource does not authorize adjacent topology endpoints.

Focused tests must prove fixed order, reservation before every read, unchanged
ordinary explanation, consent/window/fingerprint checks, separate family
reauthorization, truthful unsupported coverage, bounded queries and cancellation
without partial publication. Full hosted race/database/browser/install/security
checks and actual new-mode desktop journeys remain required before acceptance.
No tests, services, generators or builds run on the user's workstation.

## Alternatives considered

Executing tool names supplied by generated text would enlarge authority and
enable instruction poisoning. Reusing the inventory proof for all families
would hide later log/health revocation. Treating an empty result as a fabricated
record would create invalid citations. Increasing aggregate budgets would mask
capacity and does not follow from adding fixed read families.
