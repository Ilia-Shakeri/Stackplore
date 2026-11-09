# Alpha8.1 — measured development evaluation and fault receipts

Version **2.0.0-alpha.8.1**, schema22. Fresh source, package, tag and publication
receipts are required; implementation alone does not imply success.

This increment adds three real pinned-profile passes across the twelve public
paired seed scenarios: **72 planned observations**, not 72 independent scenarios.
The safe summary retains case/locale/pass identity, missing/rejected/budget cases,
per-provider/language denominators, latency percentiles and validated completion
tokens. Rejected usage stays uncertain. Gap sections are observations, not correct
abstention judgments. Human grounding, adversarial profile evaluation and blind
holdout acceptance remain pending; `release_accepted` stays false.

Passes run on three hosted workers in parallel, each limited to one processor
and 24 observations. An aggregator rejects missing/duplicate passes, mixed
source/profile pins, missing cases, out-of-budget memory and unproven cleanup.
The separate main gate retains all actual browser assertions. Slow answers may
require the existing explicit status-follow control after its 30-second stream
ends; browser journeys now exercise this bounded control without resubmission.

## Attempt trail

Sources `e981cfb` and `4ab9cb6` were superseded/cancelled; the latter's code job
found script formatting. The hosted formatter artifact was applied without local
execution. Source `605b108`, [42612228372](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42612228372),
passes 47 release and 23 focused corpus/evaluation tests and all nineteen fault
regressions, but code validation finds a missing `URL` import in its test fixture.
The import is fixed and script lint moves ahead of expensive hosted work.

Source `619a4a0`, [42613156363](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42613156363),
and development [42613156209](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42613156209)
fail the real application/browser journey before seed evaluation. Both preserve
owned cleanup. All baseline English and the earlier Persian journeys pass; the
Persian viewer incident answer completes in SQL but the client status lifetime
ends before publication is observed. Actual generation approaches 39 seconds
against an unchanged 30-second web status lifetime. The test incorrectly waited
for automatic delivery instead of using the explicit status-follow control.
Failure stays failure; stream/generation budgets and one-admission assertions
are unchanged. The correction exercises bounded explicit follow in both owning
journeys. The before-failure protocol RSS sample is about 3.05 GiB; it does not
represent an end-of-failed-run resource measurement. Final sampling now runs
before cleanup on failures as well. Fresh source/profile receipts are required.

The existing real processor gate now records the owned process's kernel RSS
high-water mark and CPU ticks, pinned profile, runner hardware and an 8-GiB CI
RSS ceiling. This is neither customer capacity nor an approved production limit.
The separate `stage8-quality` workflow performs development measurements; a
successful collection job does not imply the quality thresholds passed.

The fault receipt requires nineteen named regressions, including real PostgreSQL
crash/reconciliation/query-plan paths, lease/restore/rollback boundaries, adapter
DNS/cancellation/malformed responses and one-use dispatch. A 72-case malformed
output matrix plus a ten-second bounded single-worker parser fuzz run extends
the machine suite. These permutations do not inflate the eighteen curated attacks.

Review loop: **8/10 pending actual hosted results**. Source, measurement parsing,
unit/fault checks, full regressions, security/package checks and real development
measurements must be inspected before a bounded score or tag is accepted.
No workstation tests, builds or services run.

[Evaluation protocol](../../v2/evaluation.md),
[stage8 roadmap](../../v2/stage8-readiness.md),
[stage9 preparation](../../v2/stage9-preparation.md).
