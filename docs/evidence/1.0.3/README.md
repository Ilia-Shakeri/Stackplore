# Corrective 1.0.3 evidence

Baseline: 089727fdfac0fa0bacb15c520212c3fbd37bd8b1. Schema remains **17**.
The 2026-10-19 review verified nine bugs and two missing interface requirements.
The historical 1.0.2 acceptance did not catch these defects; its immutable source,
tag and receipts keep their original results. The active correction is **1.0.3**.

- [Fresh audit and correction loop](fresh-audit.md)
- [Replacement complete local F gate and source manifest](local-gate-f.json)
- [Earlier complete local E gate](local-gate.json)
- [Lifecycle amendment: Windows API and Linux detector proof](lifecycle-race.json)
- [Production browser snapshot and limits](browser-gate.json)
- [Active execution ledger](../../implementation-plan.md)
- [Historical 1.0.2 public proof](../1.0.2/published-release.json)

Candidate changes are implemented. Complete local F passes after first-candidate
CI corrections: 530 web tests, 40 client tests and all Go/native checks, with an
unchanged implementation manifest throughout the gate. The later test-only
lifecycle amendment passes all Windows API checks plus uncached complete Linux
HTTP/realtime detector checks; full local F keeps its earlier source scope.
Local browser B passes
14 cases before the final Helm/hub amendment. First-candidate main CI failed;
its successful signed trial is unaccepted. Exact replacement-source hosted,
signed and public acceptance remain pending.
No candidate version number alone establishes release or customer acceptance.
