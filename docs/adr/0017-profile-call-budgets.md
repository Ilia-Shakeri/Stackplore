# ADR-0017: Explicit bounded processor call budgets

- Status: Accepted
- Implementation: Chosen Linux profile verified at immutable alpha.4; broader capacity remains separate
- Date: 2026-11-01
- Owners: Project maintainers
- Partially supersedes: [ADR-0015](0015-helper-durable-runtime-and-volatile-context.md), inference-call time ceiling only

## Context

The original 25-second ceiling preceded real bilingual CPU evidence tests.
Hosted Windows and Linux receipts retain successful exact gap responses and
English evidence, but repeated Persian evidence deadline failures. The official
pinned 1.7B profile has also completed exact English/Persian evidence at
14.17/18.04 seconds, so one passing measurement does not establish capacity.
Changing threads, reducing framing and changing compute did not establish the
original bound. Failed receipts remain failed; they are not reclassified.

## Decision

- Preserve the **25-second default** for configured and direct local profiles.
- Permit an explicit canonical `STACKPLORE_ASSISTANT_CALL_TIMEOUT` up to a
  **40-second hard ceiling**. Browser input cannot choose it.
- Bound the shared gateway at 40 seconds. The adapter applies the configured
  smaller timeout; parent cancellation and the 120-second run deadline always
  take precedence. Call counts are upper bounds, not promised completed steps.
- The hosted CPU candidate explicitly requests `40s` in both its direct probe
  and production application composition. Its receipt records this budget.
  The server's HTTP timeout is 45 seconds, outside the client deadline.
- Preserve all byte/token, call-count, queue/worker, lease, authority and
  reservation limits. No automatic repeat or refund of uncertain work.
- Keep strict complete JSON, current citations, exact semantic assertions and
  both language/role browser journeys. Do not call a 40-second result a pass
  under the old 25-second criterion.

## Consequences

Explicit CPU profiles can spend more bounded time per call. A worker may stay
occupied up to 15 additional seconds, while global run, queue and reservation
bounds remain. Heartbeats and current-authority checks must stay independent
of provider work. A late or cancelled result still cannot publish.

The roadmap's measured warm-answer p95 gate remains separate. A new hard ceiling
does not prove that gate, model quality, capacity or whole-stage acceptance.
The historical Windows profile and its 25-second failures remain separate from
the Linux candidate. Default-off startup and customer pins do not change.

## Alternatives considered

Keeping 25 seconds for every profile rejected repeatedly observed CPU behavior.
Increasing every default would silently change existing operators' limits.
Removing language cases, accepting incorrect output, repeating timed-out work
or disabling the real gate would conceal failures. Unbounded calls would break
the resource and authority policy. Dedicated faster compute remains possible,
but its capacity and cost need independent acceptance.

## Verification required

Prove unchanged defaults, acceptance of explicit 40 seconds and rejection above
40 seconds. Run full source, real database, race and cancellation/fencing gates.
Execute both strict direct cases per language and all real desktop journeys at
the same source, with owned model/database cleanup. Re-measure timing against
the recorded 40-second profile; retain every failure and its original budget.
