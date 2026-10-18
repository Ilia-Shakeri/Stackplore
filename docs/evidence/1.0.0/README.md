# 1.0.0 final acceptance

Date: 2026-10-18. Status: final candidate, not yet a published accepted release.
The exact final commit, tag runs and public artifact verification will be
recorded after they exist. Failed attempts retain their original scope.

The [requirement matrix](../0.9.0/requirements.md) covers the accepted product
scope. Stage 9 rechecks the full architecture, security, persistence, providers,
transport, interface, deployment, release and documentation boundaries against
that matrix. Its corrections remove sensitive auth fixture failure output and
stale security, installation, project and contribution claims. See the
[final review](final-review.md).

| Gate | Current result |
| --- | --- |
| Stage 8 baseline | `v0.9.0` source `20f8c2e30cb2ccdfc16aa266f0b0d5647c429881`; exact main/tag CI, signed trial/publication and all 34 public assets passed independent verification. The published Windows archive matches the recovery-tested trial; [published receipt](../0.9.0/published-release.json). |
| Consumer trial | Downloaded archive API digest matched; all 26 manifest entries matched. Independently verified 12 provenance subjects and all six archive SBOM attestations with exact main workflow/source identity; wrong source digest was rejected. |
| Windows archive recovery | Executed the verified 0.9.0 trial tools against owned PostgreSQL databases: setup, acknowledgement, SSE/replay, restart, native backup/empty restore, revoked sessions and ownership-loss shutdown passed. |
| Final 1.0 local gate | [Receipt](local-gates.json): complete `npm.cmd run verify` exit 0 with real PostgreSQL, changed server/HTTP SQL cases uncached, 40 client tests, 453 web tests, ten release tests and both builds. Backend core 80.6%/82.6%; frontend statements 81.53%, branches 77.49%. Failed stopped-database attempt A remains separate. |
| Final 1.0 browser | [Receipt](browser-runtime.json), [measurements](browser-measurements.json), [16 public screenshots](screenshots/): all 14 actual production-browser cases passed on Chrome 155.0.8059.80; 64 loaded route states, eight profiles, public-only offline/cache/update and both authenticated language journeys. No skipped, unexpected or flaky result. |
| Final exact CI and release scans/signatures | Pending. |
| Public artifact verification and versioned recovery | Pending. |
| Final tag and publication | Pending. |

Runtime proof targets Windows amd64, Linux amd64 and the Linux amd64 image,
with PostgreSQL 17. Native Linux arm64 and macOS archives are cross-built only.
The browser matrix records actual Chromium, English/Persian, RTL/LTR, narrow/wide,
light/dark, keyboard, reduced motion, offline privacy and update behavior.
Automated accessibility checks do not certify every assistive technology.

Compose and Helm fixtures prove installation/restart in owned disposable
environments. Helm checks ingress route/certificate objects and uses a Service
port-forward; it does not exercise a customer's chosen ingress controller.
Customer credentials, TLS edge, provider rights, receiver reachability, backups
and capacity remain deployment-specific acceptance work, using the bilingual
[operator guides](../../README.md).
