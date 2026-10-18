# Stackplore delivery plan

Target: **1.0.2 — corrective candidate**. Stage 9 delivery acceptance remains
**8/10** pending hosted/signed/public proof; final scoped code review is **9/10**.
Confirmed fixes and focused checks are recorded in the
[fresh review](docs/evidence/1.0.2/fresh-audit.md). Earlier local C and the scoped production-interface browser passed. The later
retained-event REST/SSE correction has focused proof; full replacement D passes;
new exact-source hosted/signing/public gates remain pending. Source `18e8558`
was pushed but is superseded by that correction; no 1.0.2 tag was made. Current
schema ledger is **17**.

## Historical accepted 1.0.1 delivery

Immutable `v1.0.1` points to `73ece4754554cca548c242768b195d21664d37b8`.
Main CI `39122125020` and tag CI `39123016027` passed all six jobs each.
Signed trial `39122125492` and release `39123016084` passed; stable
[release 1.0.1](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v1.0.1)
was published as `416416913` at `2026-10-18T11:43:54Z`.

Independent public verification checked all 39 files, 30 checksums, all 31
provenance subjects and seven SBOM attestations. Public Windows bytes match the
actual recovery-tested trial. The different public database archive was
independently loaded and executed with identity, engine/helper/root-drop,
read-only non-root SQL, license and owned cleanup checks passing.
[Published receipt](docs/evidence/1.0.1/published-release.json).

Immutable 1.0.0 remains withheld; 0.9.0 is superseded for its unsafe database
default. Intermediate failed trials and interrupted consumer transfers retain
their honest outcomes. Customer/offline/cross-built limits remain separate.

| Stage | Version | Delivered outcome |
| --- | --- | --- |
| 1 | 0.1.0 | Complete: safe ownership/migrations, final administrator, bounded calls, invalid-input handling, masking, source selection and reproducible baseline. |
| 2 | 0.2.0 | Complete: durable PostgreSQL observations and safe source profiles, bounded collection/retention, truthful authority and audited source settings. |
| 3 | 0.3.0 | Complete: covered alert evidence/lifecycle, durable replay and fenced opt-in webhook/Matrix delivery. |
| 4 | 0.4.0 | Complete: paginated exploration, URL filters, log controls, resource details, metric truth, accessible topology and bilingual browser journeys. |
| 5 | 0.5.0 | Complete: official scoped Docker/Kubernetes adapters, restricted mTLS, independent coverage, safe typed facts, stable log/metric identity and isolated deterministic guest demo. |
| 6 | 0.6.0 | Complete: narrowly guarded restart/scale, durable intent/result audit, settings, preferences, quota/archive-bound audit retention and bounded cleanup. |
| 7 | 0.7.0 | Complete: one non-root embedded application, actual Compose setup/restart, Helm installation against external TLS PostgreSQL and bilingual operator runbooks. |
| 8 | 0.9.0 | Functionally complete: PWA, redacted logs, health history, filters, measured interface/performance and signed publication passed their recorded checks. Superseded because its database default is unsafe and database-image scanning was absent. |
| 9 | 1.0.2 | Reopened corrective candidate; 1.0.1 historical delivery remains recorded, while the fresh audit and new exact-source gates must close before current acceptance. |

The correction loop returned to **9/10** after full local D (28 release/policy,
40 client and 453 web tests), fresh browser B (14 cases), exact six-job main CI,
strict signed-trial scans, independent verification and actual archive recovery.
The derivative retains PostgreSQL 17.11 and the upstream entrypoint, rebuilding
unchanged gosu 1.19 with Go 1.26.9. Exact full-image/SPDX and engine/helper/root-drop
proof passed without suppressions. The [requirement matrix](docs/evidence/0.9.0/requirements.md)
maps the accepted 1.0 scope to implementation and proof limits.

For the historical 1.0.1 source, all release gates closed: exact-tag CI, signed publication, independent
public identity/checksum/provenance/SBOM checks and actual public archive
acceptance passed. That earlier review did not detect the newly verified defects and navigation gap;
[1.0.2](docs/evidence/1.0.2/README.md) tracks their corrective acceptance. Interrupted public transfers remain
transport failures; bounded range resume retained normal certificate checks,
API sizes/digests and signed-manifest verification.

Customer production acceptance remains an operator task. Credentials, provider
rights, ingress/TLS, receiver reachability, backups and capacity require the
customer environment's own checks. A published release does not certify that
installation. Runtime evidence covers Windows amd64, Linux amd64 and the Linux
amd64 application and database images with PostgreSQL 17; other native outputs
are cross-built only.

- [Completion roadmap](docs/roadmap.md) ([فارسی](docs/roadmap.fa.md)) records scope and exit gates.
- [Active evidence](docs/implementation-plan.md) retains exact commands, outcomes, failures and corrections.
- [Repository audit](docs/codebase-audit.md) and [corrective review](docs/evidence/1.0.1/final-review.md) record reviewed boundaries and the open score.
- [Documentation inventory](docs/README.md) links current user, development and operator guides.
- [Historical phase ledger](docs/history/phase-plan.md) preserves superseded foundation planning.

The maintainer authorized stage-by-stage testing, versioning, commits and pushes
on 2026-10-17. Existing tags and failed receipts remain intact. Unrelated user
artifacts stay outside release commits.
