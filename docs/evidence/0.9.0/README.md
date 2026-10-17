# 0.9.0 candidate verification

Date: 2026-10-17. This directory records scoped evidence for the working-tree
release candidate. It does not yet identify an accepted final tag or a customer
production deployment. Exact remote run and artifact identities will be added
after they complete. Failed attempts remain separate from replacement passes.

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
| Whole candidate | Active [implementation evidence](../../implementation-plan.md) | Complete local Go 1.26.9 gate passed: real PostgreSQL 225.357s, 40 client tests, 453 web tests, nine release tests, core coverage 80.6%/82.6%, frontend statements 81.61% and branches 77.8%, both builds. Exact remote CI, release trial, signatures and publication remain pending. |

The initial browser run used a stale embedded bundle after a type-check build
failure. Its eight guest profiles passed, but worker readiness failed because
the stale binary lacked the new public worker files. That run is not PWA proof.
After correcting the browser TypeScript project and rebuilding the embedded
bundle, all nine tests passed in 1.2 minutes. The private failed receipt remains
ignored and distinct from the passing replacement.

Architecture and code review scores are scoped judgments, not test coverage or
deployment certification. The independent corrected-code review reached 9/10;
the release is still a candidate until its remaining gates pass.
