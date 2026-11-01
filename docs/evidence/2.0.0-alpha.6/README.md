# Stage-6 development evidence

This is internal preparation, not stage-6 or whole-version acceptance.
All execution is on GitHub Actions. Stage-5 immutable publication remains
independent. [Workflow and quality requirements](../../v2/incident-workflows.md).

At `7ca21db`, run `41302285591`, a new unsupported-family fixture omitted the
domain's mandatory reason code. [The recorded state](invalid-fixture-ci.json)
contains the focused failure while other jobs were still running; it is not
a completed workflow or cleanup receipt. The followup corrects the fixture,
without relaxing production validation. No local test or formatter runs.

At `75ad179`, run `41303099170`, focused correlation/context/service/authority
tests pass. [The recorded state](correlation-format-ci.json) retains a full-code
formatting failure and incomplete runtime/cluster jobs. The correction applies
an inspected source-only formatting patch produced by Actions. That partial
receipt cannot prove cleanup or accept later lifecycle/transport changes.

The internal service slice derives reports only from retained bounded proofs,
uses existing fresh publication/poll fences, clones report arrays and nested
references, and erases the owning report with the answer. Focused lifecycle and
revocation tests plus complete exact-source verification remain required.
The next source adds an unaccepted explicit report-route/contract candidate.
Generated client types come only from the inspected Actions source patch.
Source `b542955`, run `41309091245`, passes seven jobs but fails source formatting
([record](client-format-ci.json)). Its real engine/old desktop flow does not accept
the later report view. The next source applies inspected hosted formatting and
adds explicit bilingual report controls and stronger real report metadata gates.
Fresh complete client/desktop, alert/replica correlation and held-out quality
remain pending. There is no alpha.6 tag or stage-6 completion claim.
