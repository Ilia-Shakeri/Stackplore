# Stackplore active implementation evidence

Review date: 2026-10-12. Target: 1.0.0. Current checkpoint: 0.1.0, stage 1 complete after local exit gates.
[Roadmap](roadmap.md) ([فارسی](roadmap.fa.md)) defines delivery order and exit gates.
[Audit](codebase-audit.md) records defects/coverage. [Historical evidence](history/implementation-plan-2026-08.md) preserves earlier timing and is not current proof.
The maintainer authorized implementation, versions, commits, tags and pushes in this request.

## Review entry and boundaries

Entry commit `08028f4` had substantial uncommitted PostgreSQL/provider/product/notification/deployment work. It was preserved, reviewed and integrated. An existing unreferenced dashboard image stays outside this release because capture provenance was not established. No contributor/runtime data was reset or deleted.

Review covered domain/auth/HTTP/providers/realtime/alerts/notifications/operation scaffold; both persistence generations, composition roots and tests; web/interface/client/contracts; scripts/deployment/workflows and docs. Legacy storage was removed only after real PostgreSQL parity and native restore proof. The system PostgreSQL service was not changed; all SQL proof used a separate repo-local cluster and disposable databases.

Before the first tag, untracked SQL was normalized once to LF with one final
newline, so local embedded bytes match the Git index/fresh checkout. The final
SQL and native gates were rerun on new disposable data after this step. Existing
development database history was not changed; see [upgrade constraints](operations/upgrade-rollback.md).

## Correction loop

| Pass | Scoped score | Remaining baseline problems | Correction |
| --- | --- | --- | --- |
| Entry | 5/10 | Auth/ownership/outbox races, log leaks, invalid success replies, source races, first boot and contradictory docs | Whole-code review and ordered roadmap |
| 1 | 7/10 | Key lifetime race, restore ownership/scope/durability, chart YAML/mount/forwarded scheme and false metric states | Fixes plus real SQL/HTTP/provider/frontend regressions |
| 2 | 8/10 | New version check import/lint, web test formatting, incomplete setup quick start and unchecked final gates | Fix scripts/docs/format and rerun relevant gates |
| Independent scope review | 9/10 | No known severe defect in delivered stage-1 fixes | Backend/frontend/PostgreSQL review; final gate and release diff required before completion |

Scores judge stage-1 infrastructure and corrected code, not measured coverage or whole-product completion. Stage 2–9 features and target proof remain open. The final 1.0 architecture still needs accepted target alignment.

## Stage 1 changes

- Common auth transaction serialization preserves setup/current actor/final-admin invariants. Queries and whole transactions use deadlines.
- Ownership precedes migrations; dedicated same-session probes detect loss, seal storage and stop service/jobs. Pool minimum capacity, shutdown and ledger reads are bounded.
- Notification allocation/enqueue share a transaction lock; a late earlier sequence cannot be stranded. Leases/retries/terminal audits remain.
- 18 real auth/product repository parity contracts replace legacy storage behavior. SQLite code/migrations/dependencies/licenses and busy-timeout setting are removed. Key/path/single-link checks remain in `runtimefiles`.
- Bounded non-audit PostgreSQL prune exists. Scheduled quotas and separately protected audit expiry remain future work.
- Bad input returns a problem, denied/outage codes stay truthful, metric replies cap at 200, source/health summaries mark partial/unknown, exact resource reads work beyond the first page.
- Shared plain/structured log scrub masks bearer/basic/cookies/keys; repeated pod container names remain distinct; webhook attempts close idle sockets.
- Explicit web source requests, stable details/history/search, absent metric states, strict runtime guards, real predicate translations and Persian log direction have regressions.
- Setup environment/Secret inputs, pod mount ownership, chart annotation YAML, HTTPS forwarding, edge probe separation and Compose filesystem/resource bounds are corrected. Real source profiles default off.
- Native backup/restore covers public schema, has a 15-minute bound and maintenance ownership monitor; publication syncs and cannot overwrite.
- Current guides/roadmap/audit, consistent 0.1.0 metadata, document/link/version gates, changelog and pinned workflow actions are included.

## Exact local context

Windows PowerShell; Node.js 24.18.0, Corepack 0.35.0, pnpm 11.17.0, Go 1.26.5. PostgreSQL server/native clients 17.11 from the existing installation ran separately with repo-local data on loopback55473. Password-free trust is limited to this disposable test cluster and is not production guidance. Helm 3.17.3 was downloaded into ignored tools and verified against its published SHA-256. No global tool install occurred.

Docker/Compose, live providers/receiver, a cluster and a local C race compiler are absent from this proof. Linux CI has a race command; its outcome is separate from plain local Go tests.

```powershell
$repoRoot = (Get-Location).Path
$env:PATH = "$repoRoot\.tools\node-v24.18.0-win-x64;$repoRoot\.tools\go\bin;$env:PATH"
$env:CI = 'true'
$env:GOCACHE = "$repoRoot\.tmp\go-build-v1"
$env:STACKPLORE_TEST_POSTGRES_URL = 'postgresql://stackplore_test@127.0.0.1:55473/stackplore_test?sslmode=disable'
npm.cmd run bootstrap
npm.cmd run verify
```

The test URL contains no password and names only a disposable test database. Remove its test-only variable before runtime commands. Fixtures use unique schemas and a shared test advisory gate. Native restore uses fresh whole databases and private runtime-key directories.

## Observed checks

| Command/check | Result |
| --- | --- |
| `npm.cmd run bootstrap` | Passed pinned tools/frozen lock |
| Focused HTTP/provider/notification Go suite | Passed; real PostgreSQL HTTP setup/login/session/grant/user/logout and protected-data scan ran |
| `go -C apps/api test ./...` with real URL | Passed after legacy removal; latest ledger/maintenance regressions also passed |
| PostgreSQL parity/concurrency |18 auth/product contracts plus ownership, query deadlines, migration rollback/unknown/changed history, outbox commit order and protected retention passed |
| Web tests/types/lint |160 tests passed, types/lint passed, locale parity 345 |
| `check:contracts` |181 bounded schemas and generated output parity passed |
| `check:docs`, `check:version` | Inventory/217 local links and consistent 0.1.0 metadata passed after final document edits |
| Helm lint/default/setup+cluster+ingress/annotation renders | Actual Helm 3.17.3 passed; no pod/cluster claim |
| Native backup/archive-list/empty-target restore | Passed17.11; ledger6, users1, sessions0, preauth0, generation1 checked |
| Restored runtime | Ready200, old session401, fresh admin login/source/audit reads passed |
| Own ownership-session termination | Runtime stopped with failure within monitor bound; latest marker `NATIVE_BACKUP_RESTORE_SESSION_REVOCATION_AND_OWNERSHIP_STOP_PASS` |
| Final `npm.cmd run verify` | Passed exit0 with real PostgreSQL URL: all Go packages,36 client tests,160 web tests,345 locale messages,181 schemas, lint/types/builds |
| Remote workflow/race/images | Not yet observed for the release revision |

Failures were repaired: entry Go operation formatting; later web test formatting; version-check URL import/regex lint; empty annotation-map Helm YAML. Native helper field-name errors were corrected using fresh disposable databases. An interrupted validation session stopped tool jobs; the disposable PostgreSQL cluster recovered its own WAL, then final checks/native drill were rerun. This does not prove a production recovery.

## Remaining 1.0 scope

Stage 2: persistent source/observations, collection/reconnect, retention/audit lifecycle and reviewed SQL hot paths. Stage 3: alert window/rate/outage/event semantics, durable live alert/replay, Matrix and receiver proof. Stage 4: full paging/detail/log/topology, idle session refresh and browser journeys. Stage 5: vendor-client alignment, namespace/partial rights, provider fidelity and full multi-environment demo/guest isolation. Stage 6: narrow operation gates and settings/export/cleanup. Stage 7: built-web serving alignment, real containers/cluster/TLS/upgrade/restore. Stage 8/9: safe PWA, accessibility/performance/security, artifacts/SBOM/provenance, complete bilingual docs and independent final 1.0 review.
