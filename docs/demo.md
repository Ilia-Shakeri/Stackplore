# Deterministic demonstration

[راهنمای فارسی](demo.fa.md) · [Exploration controls](exploration.md)

The packaged `demo-v2` scenario runs without Docker, Kubernetes, provider files,
or control-plane requests. All its sources, resources, observations and alerts
are synthetic. It demonstrates a checkout incident; it does not report the
health of the host running Stackplore.

## Two environments and nine resources

| Environment | Resources |
| --- | --- |
| Demo shop (`env_demo_shop`) | Frontend, Gateway, API, Checkout, Worker, PostgreSQL, Redis |
| Demo operations (`env_demo_ops`) | Monitoring, Monitoring dashboard |

The source remains `source_demo`. The original API, Checkout, Gateway,
PostgreSQL, Redis and Worker identities remain stable across the fixture upgrade.
Opaque resource identifiers derive from provider/source/environment/kind/scope/native
identity. They do not depend on inventory order or display names.

The initial scenario clock is `2026-08-16T08:00:00Z`. Checkout is unhealthy,
Gateway is degraded, and the other seven resources are healthy. The checkout
log reports a failed request with a full connection pool; a firing synthetic
alert points to Checkout. The topology contains thirteen explicit relationships,
including Gateway routes, declared application dependencies and monitoring
references. A monitoring reference does not assert an application dependency.

The initial fixture has nine log lines, nine infrastructure events and twelve
five-minute metric samples per resource and metric name. Nine supported metric
names cover CPU/memory use and requests/limits, restart count and network
counters. A single-resource history request remains within its 200-point bound;
larger requests can be partial. Current samples and matching history samples
have the same fact identity and value. Request order and selected metric names
do not change those facts.

## Clock and reset

Runtime starts at step zero. There is no browser control that advances or
resets the scenario. Test constructors explicitly select one of three steps:

| Step | Clock | Incident |
| --- | --- | --- |
| 0 | 08:00 UTC | Checkout unhealthy, Gateway degraded, alert firing |
| 1 | 08:05 UTC | Checkout and Gateway degraded, checkout restarted, alert resolved |
| 2 | 08:10 UTC | All nine healthy, recovery log present |

The constructor never reads wall time or generates random observations. Earlier
log/event/metric facts stay identical when a later step adds observations.
Restarting an isolated demonstration deployment restores its versioned initial
dataset. Fixture upgrades replace demo-derived runtime state in that deployment.
The operator must not use a normal installation's database or data directory
for this purpose.

In a normal authenticated installation, choosing the Demo source does not
enable guest access or reset the whole installation. Same-version durable
demonstration alert acknowledgements survive ordinary restarts. Real source
observations and protected history belong to the normal installation's separate
lifecycle.

The first `demo-v1` to `demo-v2` upgrade replaces only packaged synthetic facts;
authentication, administrator-created alert rule configuration, source enabled/disabled choices and protected audit history are retained. The guest alert seed is
system owned and needs no invented user account. Isolated demonstration resets
also preserve protected authentication/audit history.

## Guest demonstration deployment

An operator must explicitly set `STACKPLORE_DEMO_DEPLOYMENT=true` and
`STACKPLORE_GUEST_DEMO=true`, supply a dedicated absolute
`STACKPLORE_DEMO_DATA_DIRECTORY`, and use a separate PostgreSQL database.
The demonstration directory must differ from the normal data directory.
See [deployment](operations/deployment.md) and [configuration](configuration.md)
for the normal PostgreSQL, origin, listener and production startup requirements.

This profile rejects real provider configuration, provider credential-file
references, outbound notification configuration and infrastructure write flags.
The database profile marker and runtime-directory marker prevent the same
storage from serving normal and demonstration deployments. A mismatched or
populated normal database is rejected; it is not converted into a demo database.

### Windows development recipe

Create a fresh dedicated database using the PostgreSQL procedure in
[Development](development.md). Put its database URL in the private local file
`%LOCALAPPDATA%\Stackplore\demo.env`; keep the file out of the repository and
do not paste its contents into chat or logs. The file contains these settings,
with the database URL and absolute directory filled in by the operator:

```dotenv
STACKPLORE_ENVIRONMENT=development
STACKPLORE_HTTP_ADDRESS=127.0.0.1:8080
STACKPLORE_PUBLIC_ORIGIN=http://127.0.0.1:5173
STACKPLORE_DATABASE_URL=<fresh dedicated PostgreSQL database URL>
STACKPLORE_DEMO_DEPLOYMENT=true
STACKPLORE_GUEST_DEMO=true
STACKPLORE_DEMO_DATA_DIRECTORY=<new private absolute directory>
```

Use a new terminal with no other `STACKPLORE_*` variables. Do not include real
provider or outbound keys, even with empty values. The development launcher also
loads the repository's `.env`, so use a checkout without that file for this
recipe. After the pinned bootstrap has completed:

```powershell
$repoRoot = (Get-Location).Path
$env:PATH = "$repoRoot\.tools\node-v24.18.0-win-x64;$repoRoot\.tools\go\bin;$env:PATH"
$env:CI = 'true'
$env:GOCACHE = "$repoRoot\.tmp\go-build-v1"
$demoEnvFile = Join-Path $env:LOCALAPPDATA 'Stackplore\demo.env'
if (!(Test-Path -LiteralPath $demoEnvFile -PathType Leaf)) { throw 'Private demo configuration is missing.' }
if (Test-Path -LiteralPath (Join-Path $repoRoot '.env')) { throw 'Use a checkout without a root .env for this isolated recipe.' }
node --env-file="$demoEnvFile" scripts/dev.mjs
```

Open `http://127.0.0.1:5173`. The API stays on loopback port 8080 and the browser
uses the development proxy. The first product view is the guest overview, with
no setup account. To reset the scenario, stop and restart this same isolated
process against its demonstration database/directory. Do not erase storage or
point it at a normal installation. This is a development recipe; a packaged
same-origin demonstration deployment has a separate deployment gate.

Guest entry reads `/api/v1/setup/status`. A `guest_demo` response supplies only
fixture version, scenario step and observation time. It creates no account,
session, session cookie or CSRF proof. Setup and login redirect to the overview.
The header displays a Guest label and an English/Persian notice with the
synthetic/read-only status, fixture version, scenario step and clock.

Guests can read sources, environments, inventory, detail, health, topology,
logs, metrics, events, alerts, notification history and bounded observation
updates. Account administration, audit administration, source configuration,
alert rule changes/acknowledgement and outbound test delivery are unavailable.
The server rejects write requests even if a caller bypasses the interface.
Locale and theme preferences remain local browser choices.

## Explore the incident

1. Open Overview and inspect the unhealthy and degraded counts.
2. Open Resources, search for Checkout, and follow its stable detail link.
3. Follow Logs and Events from that detail. Default relative windows use the
   demonstration clock, so a fixture recorded in the past remains visible.
   An explicit shared absolute time range retains its exact bounds.
4. Open Topology, select Checkout, inspect its neighbours and evidence, and use
   the accessible node/relationship list when preferred.
5. Open Alerts and inspect the firing synthetic checkout alert. Guest mode
   offers no acknowledgement action.
6. Switch English/Persian, theme and screen width. Loaded log copy/download and
   other bounded controls behave as documented in [Exploration](exploration.md).

## Verification boundaries

Provider tests check the two environments, nine stable resources, controlled
steps, immutable prior facts, metric selection independence and a fixed fixture
snapshot. The packaged adapter is checked for filesystem/network imports. Web
tests check the guest response boundary, routes, read-only notices and rejected
local write paths. These checks do not establish a Docker, Kubernetes, external
notification, browser clipboard or production deployment result. Milestone
runtime proof is recorded separately in the [evidence plan](implementation-plan.md).
