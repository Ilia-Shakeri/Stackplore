# Troubleshooting

Start with safe process codes and `/healthz` versus `/readyz`. Liveness means
the process answers; readiness additionally checks the current database/ledger
and runtime ownership. One provider being unavailable need not fail global
readiness. Do not send raw environment, credentials, logs or notification URLs
to support channels.

| Symptom | Check | Safe next action |
| --- | --- | --- |
| Process fails before listen | Startup fixed error code and configuration names | Compare [configuration](../configuration.md); correct names/canonical values without printing values. |
| Fresh Compose/chart API exits | Empty install, wildcard API bind, missing/expired paired setup proof | Generate proof using [setup](account-recovery.md), supply both for first boot, complete setup, then remove them. |
| Database unavailable | Private network, role/database, verified TLS and certificate path | Restore connectivity; verify readiness. Do not downgrade production TLS. |
| Ownership unavailable | Another API or offline maintenance job uses the installation | Stop the intended old owner through normal deployment control; do not bypass the advisory lock. |
| Ownership lost after database connection reset | Runtime stops and refuses to reacquire silently | Ensure only one process, then perform controlled restart. |
| Migration rejected | Changed/unknown ledger, foreign tables, wrong application version | Keep target offline; use matching build/backup. Never edit ledger rows. |
| First setup denied | Existing first account, wrong/expired/reused proof, wrong origin | Use login after successful setup. Recover through the offline operator command if ownership is lost. |
| Login fails repeatedly | Coarse login response, throttles, disabled account | Wait for bounded retry; ask an administrator. Do not infer account existence from the error. |
| Unsafe action rejected | Current role, exact origin, CSRF, current purpose-bound grant | Retry through the current authenticated UI; never reuse a consumed grant. |
| Browser origin/auth failure behind ingress | Exact public HTTPS host, edge trust and forwarded scheme/host | Use configured TLS ingress; narrow proxy trust to actual peers. |
| Kubernetes source unavailable | Collector explicitly enabled, service-account mounts and current cluster reader rights | Current namespace-only profile is unfinished; do not grant Secret access or cluster-admin. |
| Docker cannot enable in production | Direct socket development profile rejected | Keep off until the accepted restricted production proxy profile exists. |
| Webhook test queued but no receipt | Terminal delivery/attempt result, safe receiver/network policy | A queue is not receipt. Review the receiver privately; do not loosen SSRF/redirect policy. |
| Runtime key rejected | Directory ownership/permissions, replaced/symlink/hardlink file | Preserve the existing private key; follow recovery policy, not ad hoc replacement. |
| Restore rejected | Target not empty, wrong archive/build, incompatible native tools | Use a new disposable empty target and [restore procedure](backup-restore.md). |
| Large view looks partial | Page/capability state and current frontend paging limits | Do not read partial data as complete; see [remaining work](../roadmap.md). |
| Source cache stale after disable or outage | `source.disabled`, collection attempt and optional-port state | History remains; live logs stop on disable. Review the [collection policy](../observations.md). |
| Source preview fails | Exact current version, operator profile, two-minute expiry and current role | Reload state and review again. Missing profiles cannot enable; never automatically replay an uncertain write. |
| Source registry exceeds runtime bound | At most 64 registered profiles and startup's safe reason | Review operator-owned registry offline. Do not silently skip sources or delete their history. |
| History grows despite cleanup | Fixed fair batch quotas, newest/synthetic/protected exclusions | Monitor capacity and verified native backups. Audit retain-all is protected; audit quotas are still planned. |

Before asking for help, record the release/commit, deployment kind, time in UTC,
safe error code, request ID, and a minimal invented reproduction. Private
security reports follow [the security policy](../../SECURITY.md).
