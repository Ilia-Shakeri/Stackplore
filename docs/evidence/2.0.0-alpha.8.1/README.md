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
