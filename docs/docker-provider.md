# Docker observation profile

[فارسی](docker-provider.fa.md)

The stage 5 candidate uses the official `github.com/moby/moby/client` `v0.6.2`
and `github.com/moby/moby/api` `v1.56.1`. The latter is a module version; the
highest negotiated HTTP API version is `1.56`, with client minimum `1.40`.
SDK objects stay inside the Docker adapter. Ambient `DOCKER_HOST`,
`DOCKER_API_VERSION` and Docker TLS environment settings are not read.
[SDK reference](https://pkg.go.dev/github.com/moby/moby/client@v0.6.2).

Local fixture, configuration and composition tests pass. On 2026-10-17 the
restricted mTLS proof passed on Linux Engine `29.1.3`, negotiated API `1.52`,
with Go `1.26.5` (128.01 seconds). Discovery, live stats, historic logs/events,
client authentication and prohibited-route denial all passed; the owned fixture
containers, network, volume and image were absent after cleanup. The dedicated
CI job's remote result remains pending until recorded in the
[evidence plan](implementation-plan.md). Another platform or operator daemon
needs its own check.

## Choose an operator profile

Docker defaults off. An authenticated administrator can enable an existing safe
source reference; the browser cannot supply endpoints, host paths, certificates
or environment names. Keep credentials outside the database and browser.

| Profile | Use | Required configuration |
| --- | --- | --- |
| `docker_socket_dev` | Explicit development with loopback listener and public origin | `STACKPLORE_DOCKER_ENABLED=true`, optional absolute `STACKPLORE_DOCKER_SOCKET_PATH` |
| `docker_restricted_proxy` | Observation through a fixed HTTPS authorization proxy | Enable Docker, select `STACKPLORE_DOCKER_ACCESS_PROFILE=docker_restricted_proxy`, and supply all four proxy references below |

The proxy references are `STACKPLORE_DOCKER_PROXY_ORIGIN`,
`STACKPLORE_DOCKER_PROXY_CA_FILE`, `STACKPLORE_DOCKER_PROXY_CERT_FILE` and
`STACKPLORE_DOCKER_PROXY_KEY_FILE`: one HTTPS origin and three absolute mounted
file references. Set `STACKPLORE_DOCKER_ENABLED=true` separately. The origin has no
userinfo, path, query or fragment. CA, client certificate and private key reads
are each limited to 1 MiB. Files must be regular; projected links are permitted
only inside the selected parent directory. File escapes, directories, oversized
files, redirects, ambient HTTP proxies and unverified TLS are rejected.

Direct daemon socket access grants extensive host authority. It is permitted
only in the loopback development profile. Mounting a socket read-only does not
make daemon requests read-only. Do not mount it into the API image in production.
The observation proxy does not grant restart or other writes; its write flag is
rejected. Supported guarded actions remain a separate delivery stage.

## Restricted Compose bootstrap

Use the normal [Compose deployment and first-administrator steps](operations/deployment.md).
The optional [Compose overlay](../compose.docker.yaml) adds only the reviewed
[HAProxy policy](../deploy/docker-proxy/haproxy.cfg) and private observation
network. The proxy image is pinned by digest. Its listener has no published
host port. Only the proxy receives the operator-selected daemon socket.

1. Have the operator PKI issue a server identity whose DNS SAN includes
   `docker-proxy`, plus a separate client identity allowed to observe that daemon.
   Enable server-auth and client-auth usages as appropriate. Do not reuse the
   server private key as the client key.
2. Prepare a private server directory containing `server.pem` (server leaf,
   chain and private key) and `ca.pem` (trusted client issuers). The proxy runs
   as UID/GID 65532; make its private directory and files readable only by that
   identity. It has no added capabilities, a read-only
   filesystem and `no-new-privileges`.
3. Prepare a separate client directory with `ca.pem` (trusted server issuers),
   `client.pem` (client certificate chain) and `client-key.pem`. Grant only the
   API's UID 10001 the required read access; mount this directory read-only.
   The API receives neither the server key nor the daemon socket.
4. Set these Compose-only names in the private operator environment or Compose
   environment file: `DOCKER_PROXY_SERVER_DIRECTORY`,
   `DOCKER_PROXY_CLIENT_DIRECTORY`, and
   `DOCKER_SOCKET_GID` (the numeric owning group of the selected
   daemon socket, already approved for group read/write access). This
   supplementary group permits socket access without root or changing socket
   permissions. On Linux, inspect the selected socket with
   `stat -c '%g' /var/run/docker.sock`; use the actual configured path. Set
   `DOCKER_SOCKET_PATH` when needed (default `/var/run/docker.sock`). These
   names are interpolation inputs, not accepted direct server variables.
5. Validate the merged deployment, then start it:

```sh
docker compose --env-file .env -f compose.yaml -f compose.docker.yaml config --quiet
docker compose --env-file .env -f compose.yaml -f compose.docker.yaml up --build -d
```

Check API readiness, the Docker source's actual capability states, one exact
container's inventory, logs and metrics, then inspect that the API has no socket
mount and the proxy has no published port. A reachable proxy alone does not
prove collection. No private key, certificate bundle, runtime directory or
unredacted log belongs in Git or a support message.

## Allowed requests and denial

The adapter and proxy permit reviewed `GET` paths only: Engine info/version,
container/image/network/volume lists, exact container inspect, one bounded stats
sample, finite historic logs and finite historic events. `HEAD /_ping` permits
SDK version negotiation. Versions are limited to `1.40`–`1.56`. Lists, stats,
logs and events accept only their fixed reviewed queries; log tail is at most
200. Streaming/follow queries, duplicate parameters, path encoding, traversal,
upgrade and request bodies are rejected.

`POST`, `PUT`, `PATCH`, `DELETE`, archive/export, attach, exec, build, login,
Secrets, configs and arbitrary forwarding are outside the policy. The real
proof probes the proxy directly so adapter checks cannot hide an overly broad
proxy. A proxy still has daemon authority through its socket; protect its
configuration, private network and client issuer policy as host-sensitive assets.

## Safe observations and partial permissions

Engine, Compose project/service, container, image, network and volume identities
remain stable across collection. Complete empty partitions can retire vanished
resources. A denied optional list preserves permitted partitions and marks its
coverage explicitly; incomplete or denied partitions cannot retire unseen
identities. A vanished or denied container inspect leaves its listed identity
with unknown health and partial details.

Engine version, container state, health, restart count, real start time, stopped
exit code, image reference, exposed/published port numbers, container mount
targets and available CPU/memory limits have validated bounded metadata. No host
IP, host mount source, environment value, raw inspect response or arbitrary label
is persisted or returned. Ports and mounts each cap at 64; labels cap at 16.
Overflow produces partial details.

Only `com.docker.compose.project`, `com.docker.compose.service` and
`stackplore.io/depends-on` labels are retained. Project/service tokens begin with
an ASCII letter or digit and continue with letters, digits, `.`, `_` or `-`, at
most 128 characters. Dependencies are a comma-separated, deduplicated, sorted
list of at most 32 such service tokens, with whitespace trimmed and total input
at most 4096 bytes. A dependency links existing services within the same Compose
project; it never reads environment values or invents an absent service.
The public dependency label value is retained only when at most 256 bytes.
Inert Compose-file import remains optional and is not implemented by this profile.

## Stats, logs and event history bounds

CPU uses two daemon samples, requested with `IncludePreviousSample=true`.
Current sample time comes from the daemon. Missing previous CPU data or reset
counters make CPU unavailable; an empty response cannot become zero usage.
Other valid measurements survive a missing CPU sample. Stats and inspect workers
have bounded concurrency; requests, connection pools, bodies and shutdown are
bounded. Production construction uses eight workers and an 8 MiB per-response
limit, plus a shared 32 MiB wire budget per observation; inventory has at most
10,000 normalized items. Exhausting that budget preserves partial coverage.

Logs use an exact finite UTC window, fixed tail of at most 200 lines, and one
shared wire-byte budget across selected containers. Non-TTY streams preserve
stdout/stderr; TTY output reports unknown stream. Redaction and per-line bounds
still apply. Byte/line truncation remains partial even if the first incomplete
frame yields no displayable line. Repeated pages use the same fixed tail;
provider log rotation can still remove earlier data. Previous-instance Docker
logs are unsupported. A supplied container name must match the inspected target.

The daemon retains only its latest 256 events. The collector reads the unfiltered
retained ring with `since=0` and a finite past `until`, then selects permitted
event types and the exact requested window locally. Coverage is complete only
when the earliest retained event reaches the requested start and the negotiated
API is at least `1.46`; empty history, an earlier requested start, invalid times
or a read bound remains partial. Polling does not recover events already lost
from the ring. Partial coverage cannot establish a complete alert window.
[Engine history source](https://github.com/moby/moby/blob/v28.5.2/daemon/events/events.go),
[event API source](https://github.com/moby/moby/blob/v28.5.2/api/server/router/system/system_routes.go).

## Certificate rotation

The process reads file references at startup. Replace files atomically; restart
or recreate the relevant API/proxy process to load new material and close old
connections. Do not assume a source toggle reloads certificates.

For issuer rotation, first install old and new issuer roots together on the
relevant verifying side and recreate it. Rotate the server or client leaf/chain
and its separate key, recreate that side, and verify actual authenticated
collection. Remove the retired issuer only after every active identity has
migrated; recreate and verify again. Treat client-issuer trust and server-issuer
trust as separate policies. Keep rollback material private until the new lane
passes, then remove retired credentials under the operator's key policy.

## Reproduce the proof

On a disposable Linux host with Docker Engine and the pinned Go tools:

```sh
GOMAXPROCS=2 STACKPLORE_TEST_DOCKER_ENGINE=1 go -C apps/api test -p=2 ./internal/provider/docker -run TestRestrictedProxyOnRealDockerEngine -count=1 -timeout=9m -v
```

The test builds a fixed shell-free fixture image, creates only uniquely named
and labeled containers/network/volume, generates short-lived private test TLS
material and removes only its own resources. It verifies discovery, Compose
dependency evidence, health, two-sample CPU/memory/network, TTY/non-TTY logs,
events, mTLS rejection and direct prohibited-route denials without changing the
fixture version. The passing marker is
`REAL_DOCKER_RESTRICTED_MTLS_SDK_INVENTORY_STATS_LOGS_EVENTS_AND_DENIALS_PASS`.
The fixture proxy uses the fixture owner's UID and existing socket groups;
generated key files remain mode `0600`. The Compose profile uses UID/GID 65532
with the operator-selected socket group and separately owned private files.
Never point this opt-in fixture setup at a customer daemon. Remove test-only
environment names before starting the application. Record the exact commit,
Engine version and CI result in the evidence plan after the run succeeds.
