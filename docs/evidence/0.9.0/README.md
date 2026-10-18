# 0.9.0 checkpoint verification

Dates: 2026-10-17–18. Tagged source:
`20f8c2e30cb2ccdfc16aa266f0b0d5647c429881`, `v0.9.0`.
[Exact main CI](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39038430422)
and [signed artifact trial](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39038439418)
passed. [Exact tag CI](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39114483036)
and [signed publication](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39114483033)
also passed; [published verification](published-release.json) records all 34
downloaded assets, signatures and the exact runtime-tested Windows archive match.
Stage 8 is complete. This is checkpoint evidence,
not customer production proof. Failed attempts remain separate from replacements.

[Consumer trial](consumer-trial.json) binds downloaded hashes, exact source,
provenance/SBOM verification and actual Windows archive restart/recovery.

[Requirement reconciliation](requirements.md) maps the accepted 1.0 scope to
code, tests, documentation and the limits of its actual evidence.

| Boundary | Actual evidence | Scope |
| --- | --- | --- |
| Installation baseline | Stage 7 commit `64fd4391113cebc53e99bdd09a3d81bbff6054ce`, [main CI 39031584134](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39031584134), [tag CI 39032396854](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39032396854) | Both exact runs passed all six jobs, including fresh Compose setup/restart and owned kind Helm install against separate TLS PostgreSQL. This is the preceding checkpoint. |
| PostgreSQL query plans | [Measured before/after](performance.md) | Real PostgreSQL 17.11, 20,000-row inventory/event/alert fixtures, first/middle/deep keysets and five measured warm samples. Regression fixes add an index and inclusive cursor bounds. |
| Dependency and historical-secret scans | [Sanitized receipt](release-security-receipt.json) | Zero HIGH/CRITICAL source dependency findings after pinned upgrades; no unreviewed historical-secret finding. Exact native/image/tag scans remain separate. |
| Native persistence/recovery | [Sanitized receipt](native-recovery.json) | Actual Windows amd64 executables, isolated PostgreSQL, acknowledgement, replay, restart, backup, empty-target restore, revoked sessions and ownership-loss stop. Built before the final source-creation audit correction; the final gate must include that correction. |
| Browser/accessibility/PWA | [Final local receipt](browser-runtime.json), [measurements](browser-measurements.json), public [screenshots](screenshots/) | Go 1.26.9 and the actual production bundle on disposable PostgreSQL: all 14 tests passed, including 64 loaded route states, offline/privacy, real installability, update acceptance/reload, 80 bounded reads and English/Persian account/role journeys. All 16 screenshots were replaced after data loaded. No private captures or authentication material are retained. |
| Executable vulnerability coverage | [Before/after receipt](native-security.json) | Rootfs analysis detected all three real Linux executables. Go 1.26.5 had 33 HIGH findings across 11 unique IDs; rebuilt Go 1.26.9 tools have zero HIGH/CRITICAL findings and a 97-package SPDX inventory. Empty language scans were rejected. Exact tag/image/Windows archive gates remain separate. |
| Whole checkpoint | Active [implementation evidence](../../implementation-plan.md), [published receipt](published-release.json) | Final legal-classifier replacement: real PostgreSQL 177.158s, 40 client tests, 453 web tests in 47 files, ten release tests, core coverage 80.6%/82.6%, frontend statements 81.53% and branches 77.49%, both builds. Exact main/tag CI and signed publication passed, as did independent verification of all 34 public assets. |

The initial browser run used a stale embedded bundle after a type-check build
failure. Its eight guest profiles passed, but worker readiness failed because
the stale binary lacked the new public worker files. That run is not PWA proof.
After correcting the browser TypeScript project and rebuilding the embedded
bundle, all nine tests passed in 1.2 minutes. The private failed receipt remains
ignored and distinct from the passing replacement.

Architecture and code review scores are scoped judgments, not test coverage or
deployment certification. The independent corrected-code review reached 9/10;
the completed checkpoint remains a 0.x prerelease. Final 1.0 acceptance has its
own [receipt](../1.0.0/README.md).
