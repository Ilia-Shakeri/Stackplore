# Corrective 1.0.3 evidence

Baseline: 089727fdfac0fa0bacb15c520212c3fbd37bd8b1. Schema remains **17**.
The 2026-10-19 review verified nine bugs and two missing interface requirements.
The historical 1.0.2 acceptance did not catch these defects; its immutable source,
tag and receipts keep their original results. The active correction is **1.0.3**.

- [Fresh audit and correction loop](fresh-audit.md)
- [Complete local E gate and source manifest](local-gate.json)
- [Production browser snapshot and limits](browser-gate.json)
- [Active execution ledger](../../implementation-plan.md)
- [Historical 1.0.2 public proof](../1.0.2/published-release.json)

Candidate changes are implemented. Focused regression tests are recorded in the
fresh audit; complete local E passes (530 web, 40 client, all Go/native checks). Browser A passes 14 pre-amendment cases. Exact final-source hosted, signed and public acceptance remain pending.
No candidate version number alone establishes release or customer acceptance.
