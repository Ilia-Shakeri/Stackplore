# Corrective 1.0.3 evidence

Baseline: 089727fdfac0fa0bacb15c520212c3fbd37bd8b1. Schema remains **17**.
The 2026-10-19 review verified nine bugs and two missing interface requirements.
The historical 1.0.2 acceptance did not catch these defects; its immutable source,
tag and receipts keep their original results. The active correction is **1.0.3**.

- [Fresh audit and correction loop](fresh-audit.md)
- [Final independent review and limits](final-review.md)
- [Exact hosted gates and retained failed candidates](release-gates.json)
- [Independent signed trial](consumer-trial.json)
- [Independent public acceptance](published-release.json)
- [Exact main production browser evidence](main-browser.json)
- [Replacement complete local F gate and source manifest](local-gate-f.json)
- [Earlier complete local E gate](local-gate.json)
- [Lifecycle amendment: Windows API and Linux detector proof](lifecycle-race.json)
- [Production browser snapshot and limits](browser-gate.json)
- [Active execution ledger](../../implementation-plan.md)
- [Historical 1.0.2 public proof](../1.0.2/published-release.json)

Immutable `v1.0.3` is `656581cf05d4774358d5644a6e1af3c69a210f81`.
Main `39235362032`, tag `39238088634`, signed trial `39235380836` and
public package/publication `39238088607` pass. Independent consumption verifies
all 39 public files, 30 manifest entries, 31 provenance subjects, seven SPDX
attestations and eight clean strict vulnerability reports. Release `417182030`
was published at `2026-10-19T10:51:50Z`.

The public Windows archive is byte-identical to the actual recovery-tested
trial. The different public database archive independently passes isolated SQL,
engine/helper/root-drop and owned cleanup checks. Exact main browser evidence
passes 14 cases, 64 route/profile journeys and the unchanged bounded workload.
The two earlier candidates and their successful-but-unaccepted signed trials
retain their failed main outcomes. Earlier local E/F/B receipts retain their
own source scope; none is promoted to proof of a later source.

All nine roadmap stages are complete in this recorded scope; final review is
**9/10**. The final documentation/evidence commit has its own required six-job
main gate before handoff; its final revision/run binding belongs to commit
checks and the reconciled release body. Customer rollout, provider rights,
receiver delivery and capacity need the installation's own proof. macOS and
Linux arm64 outputs are cross-built/scanned; their execution is not claimed.
