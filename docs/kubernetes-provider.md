# Kubernetes observation

[فارسی](kubernetes-provider.fa.md) · [Configuration](configuration.md) · [Provider boundary](adr/0002-provider-neutral-boundary.md)

The Stage 5 adapter uses official `k8s.io/client-go`, `k8s.io/api`,
`k8s.io/apimachinery`, and `k8s.io/metrics`, all pinned to `v0.37.1`.
Core, apps, batch, networking and discovery use their `v1` APIs. The optional
Resource Metrics API uses `metrics.k8s.io/v1beta1`, the exception accepted in
ADR 0002. SDK objects stay inside the adapter.

## Scope and setup

The source is off by default. Enable it only with an explicit namespace list:

```dotenv
STACKPLORE_KUBERNETES_ENABLED=true
STACKPLORE_KUBERNETES_NAMESPACES=shop,ops
STACKPLORE_KUBERNETES_CLUSTER_OBSERVATION=false
```

Names must be Kubernetes DNS labels: lowercase letters, digits and internal
hyphens; 1–63 characters. Supply 1–64 entries, separated by commas without
spaces. The deployment parser sorts and deduplicates entries. The adapter
constructor also checks its scope and rejects duplicate input.

The in-cluster endpoint is fixed to `https://kubernetes.default.svc`.
`STACKPLORE_KUBERNETES_TOKEN_FILE` and `STACKPLORE_KUBERNETES_CA_FILE` are
operator-owned absolute file references. Defaults are the mounted service
account `token` and `ca.crt` under
`/var/run/secrets/kubernetes.io/serviceaccount/`. The token read is capped at
16 KiB; the CA read at 1 MiB. TLS must validate the service host. Browser
requests cannot select an endpoint, file, credential or authentication plugin.

Projected service-account tokens refresh from the bounded operator file at
most once per 60 seconds during normal requests. A `401` invalidates that cache.
A read may refresh and retry once with a different token. Writes never retry
on that path. A failed refresh rejects the request and does not reuse the old
credential. Shutdown closes the owned idle transport and clears its cached
credential reference.

Namespace mode lists only configured namespaces. It does not list namespaces,
nodes or objects across all namespaces. Cluster identity, namespace objects and
node data are not observed. The source reports partial coverage with a fixed
profile reason; it does not invent a healthy cluster resource. Disabled kinds
are absent from authority coverage, so old cached cluster data cannot remain
visible after the profile narrows.

With `STACKPLORE_KUBERNETES_CLUSTER_OBSERVATION=true`, the adapter also lists
nodes and reads each configured namespace by name. Namespaced lists remain
restricted to the allowlist. Namespace resources use their UID, so deletion and
recreation changes identity. Cluster reachability alone does not prove cluster
health; the cluster resource has unknown health without such evidence.

## Mounted kubeconfig and context

Outside the cluster, the operator can select a mounted read-only kubeconfig:

```dotenv
STACKPLORE_KUBERNETES_ENABLED=true
STACKPLORE_KUBERNETES_NAMESPACES=shop,ops
STACKPLORE_KUBERNETES_ACCESS_PROFILE=kubernetes_kubeconfig
STACKPLORE_KUBERNETES_KUBECONFIG_FILE=/run/stackplore/kubernetes/config.yaml
STACKPLORE_KUBERNETES_REFERENCE_ROOTS=/run/stackplore/kubernetes
STACKPLORE_KUBERNETES_CONTEXT=observed-cluster
```

The kubeconfig uses `apiVersion: v1` and `kind: Config`. Its cluster has one
HTTPS `server` origin and a `certificate-authority` file reference. Its user
has either `tokenFile` or paired `client-certificate` and `client-key` references.
Relative references resolve beside the kubeconfig; absolute references must also
remain under an approved root. Context selection is exact, or uses
`current-context` when the environment key is absent. The configured namespace
list determines observation authority; context namespace does not expand it.

Every declared profile rejects inline secret/CA data, exec/auth-provider plugins,
basic credentials, proxy, impersonation, TLS name overrides, insecure TLS,
extensions, unknown keys and multi-document files. Files are bounded regular
files; all symlinks and references outside the roots are refused. No default
credential search or process environment credential fallback runs. TLS verifies
the selected endpoint against the selected CA. SDK requests remain inside that
origin and the fixed read endpoint and namespace allowlists.

The application stores only profile metadata and an authority digest, never the
kubeconfig, endpoint, file references or credentials. Switching the profile,
context or non-secret configuration fences cached observations before new
collection publishes. Restart after editing kubeconfig, CA or client certificate;
token-file refresh follows the existing bounded rule. This profile rejects
`KUBERNETES_WRITE_ENABLED=true`; use a separately reviewed in-cluster writer
profile for the supported guarded operations. The packaged Helm chart currently
installs the in-cluster profile; external mount layout is operator-managed.

## Helm permissions

```yaml
api:
  kubernetes:
    enabled: true
    namespaces: [shop, ops]
    clusterObservation: false
```

Create target namespaces before installing. The installer needs permission to
create the chart's Role and RoleBinding in each target namespace. Each binding
names the release's service account in the release namespace. Observation grants
only `get`/`list`, with `get` for `pods/log`. No Secret or ConfigMap object read,
write, `exec`, attach, proxy or wildcard permission is granted.

The cluster option adds a separate ClusterRole/ClusterRoleBinding for node
`get`/`list` and namespace `get` constrained by `resourceNames`. It adds no
mutation rights. An enabled chart with no namespace list fails validation.
The chart does not create target namespaces or grant rights to other namespaces.

## Inventory, health and relationships

Discovery includes Pods and regular/init containers; Services; PVCs;
Deployments, StatefulSets, DaemonSets, ReplicaSets, Jobs and CronJobs; Ingresses;
and EndpointSlices. ConfigMap and Secret references come from safe Pod and
workload specifications, including zero-replica templates, projected volumes,
init containers and image-pull references. Their object bodies are never read.

Observed object resources use source, environment, kind, namespace and native
UID. Container identity adds the container name to its Pod UID. ConfigMap and
Secret entries represent names referenced by permitted specifications; their
identity uses a deterministic reference key, and does not prove object existence,
UID, content or recreation. Safe detail contains bounded
container states, readiness, restart counts, CPU/memory requests and limits, and
known condition types. It contains no environment values, commands, raw
messages, arbitrary labels or Secret values. Unknown runtime state does not
prove a zero restart count or a failed readiness check. A rollout may have more
ready replicas than desired; this does not discard its replica evidence.

Topology includes observed ownership, Service selectors, Ingress routes,
EndpointSlice targets, mounts and configuration references. An unknown custom
controller kind does not discard unrelated inventory. Annotation dependencies
use this exact form:

```yaml
metadata:
  annotations:
    stackplore.io/depends-on: Service/database,Deployment/worker
```

The value is a comma-separated list of `kind/name` references in the **same
namespace**. It is capped at 2,048 bytes and 32 entries. Kind matching is case
insensitive. Accepted kinds: Pod, Service, Deployment, StatefulSet, DaemonSet,
ReplicaSet, Job, CronJob, Ingress, PersistentVolumeClaim, ConfigMap and Secret.
The snake-case aliases `persistent_volume_claim` and `config_map` also work.
Names must be valid Kubernetes DNS subdomains. Invalid entries are ignored;
an oversized list is ignored as a whole. An edge appears only when its target
is present in permitted inventory. Annotation edges are explicit evidence,
not a claim that a live network connection exists.

## Partial access and cache authority

Each namespace/kind list has a separate result. A denied Ingress list does not
discard a successful Pod list. `403`/`401` mark the affected partition denied;
an absent optional API marks it unsupported. Timeout, page, item and byte limits
cannot mark a list complete. A later failed page preserves earlier safe items
as partial, except denial or unsupported responses remove that partition's
current items. Repeated continuation tokens stop the walk.

Only an available, complete partition can retire unseen cached identities.
An empty complete Pod list can therefore retire a deleted Pod even while an
Ingress list is denied. Denied, unsupported and omitted authority cannot serve
old cached resources. An unavailable permitted partition may retain stale
evidence. If no partition is usable and a current denial is known, the source
returns permission denied rather than converting authorization failure into a
generic outage. Operator scope changes are fenced in persistence until a new
valid authority publication; a failed first collection does not expose the old
profile's data.

Reference names depend on the parent Pod/workload lists. If any such parent
kind is denied or unsupported in a namespace, both derived ConfigMap and Secret
reference partitions are hidden there. This conservative rule prevents old
reference names from surviving a loss of parent-list permission. Other allowed
Pod and workload facts remain usable.

## Logs, events and metrics

Historic logs support exact container selection and current/previous instances.
They use `sinceTime`, a byte limit, timestamps and no follow stream. They do not
change `tailLines` between pages. The read window is bounded to the first 1,000
lines and the requested byte limit across at most 64 expanded container targets.
An HTTP page remains capped at 100 lines. Truncation is partial; it is not an
empty complete history. Redaction runs before the final response byte check.
Cursors bind source, targets, previous/current choice, time range and observed
window. A changed window rejects an old cursor; restart that history read.
An exact namespaced Pod read before and after each log read must match the
trusted inventory UID. Deletion, recreation, permission loss or an unverifiable
identity discards that target's lines. These checks share the original deadline
and byte budget.

Events use the namespaced core Events API and observed UID/time/count facts.
Event messages pass through the same safe text boundary as logs. Collection is
bounded to 1,000 events. No watch or log-follow capability is claimed.

CPU and memory usage come from the Resource Metrics API. A missing API (`404`)
reports unsupported with no invented zero samples. A denied namespace reports
denied independently. Any current usage permission denial conservatively blocks
the entire current/history metric port, including cached fallback and configured
metric samples. Inventory and safe configured detail metadata remain usable.
This avoids presenting older usage after permissions change; per-metric-name
authority is not published separately. Configured per-container requests/limits and observed
restart counts remain usable without that API. Container requests are not
summed and reported as effective Pod scheduling requests; init/sidecar rules
make that sum insufficient. Missing values stay unknown.

Usage timestamps more than one minute ahead of collection are rejected. The
SDK's fractional sampling window is rounded to the nearest whole second for
the HTTP contract's integer interval; sample timestamps retain microseconds.
Usage also requires a stable authoritative Pod UID across observation and a
known creation time covering the entire sample window. A cached sample from an
earlier Pod with the same name cannot become usage of its replacement. Unknown
identity or age suppresses usage while configured resource facts remain usable.

## Bounds and proof

The constructor accepts at most 10,000 inventory items and 32 MiB response data;
production composition uses 10,000 and 8 MiB. Raw object pages and expanded
normalized resources have separate checks. A collection shares its byte budget,
has a 30-second ceiling, and each list page has a 5-second ceiling; an earlier
caller deadline wins. A list uses at most 100 pages, with up to 500 items per
page. HTTP transport rejects redirects, unexpected compression and implicit
retry delays. Rate limiting is shared across clients. No browser-facing path
can reach the SDK's exec, arbitrary proxy or Secret read operations.

Local focused tests cover wire requests through the official clients, scope,
partial rights, pagination, cancellation, limits, token rotation, metadata,
log cursors and replica evidence. Helm lint/render and missing-scope rejection
were checked locally. These checks do not prove a live cluster.

The required `kubernetes-provider` CI job creates its own disposable kind target:
kind `v0.33.0`, Kubernetes `v1.37.0`, with pinned binary and image digests. It
uses a new service account with rights in one namespace, denial in another,
a denied optional kind, current/previous logs, events, absent Metrics API and
deletion/recollection checks. The observer uses a CA-only TLS transport, so the
fixture administrator's certificate cannot bypass its service-account rights.
Missing configuration fails the required job instead of skipping it.

The next named proof installs metrics-server `v0.9.0` from a checksum-pinned
official manifest and digest-pinned image, then checks fresh CPU/memory samples
and namespace denial. `scripts/install-test-metrics-server.sh` requires the
explicit disposable flag and the exact owned `kind-stackplore-proof-*` context.
Only this disposable fixture enables `--kubelet-insecure-tls` for kind's
self-signed kubelet certificate; observation still verifies the API server CA.
The mixed-namespace observer proves that denial blocks the whole metric port.
An allowed-namespace-only observer with the same service-account credential
proves configured facts without the API and positive usage when it is present.

**Current target proof, 2026-10-17:** both final tests passed on disposable
Kubernetes `1.37.0` under kind `0.33.0`, Docker Engine `29.1.3` and Go `1.26.5`
in local WSL. The missing-API test passed in `7.06s`; the positive Metrics API
test passed in `18.84s`. Both ran the same final Linux test binary, SHA-256
`b9614bc6a63e6eeb6a5d9c9932d2f49f5fad1f9ab0b3037e709377fef9f6753c`.
The named cluster and its ephemeral kubeconfig were removed; zero owned kind
containers remained. Other Kubernetes versions and the remote CI run have no
runtime result in this local proof.

The initial metrics-server pull returned registry `403` inside the local node.
The final run used the exact official image index, native manifest, configuration
and 14 layers fetched through the host, with every descriptor hash checked,
then imported only into that disposable node. It kept the pinned image digest;
no daemon configuration changed. Failed pull/import receipts remain separate
from the passing final receipt. Local receipts are
`.tmp/stage5-kube-final-real.log`, `.tmp/stage5-kube-final-build.log`,
`.tmp/stage5-kube-final-uid-tests.log`, `.tmp/stage5-kube-final-uid-vet.log`,
`.tmp/stage5-kube-helm-final.log` and `.tmp/stage5-kind-cleanup.log`.
See [the active evidence plan](implementation-plan.md) for milestone gates.

Primary dependency and target references:
[client-go v0.37.1](https://github.com/kubernetes/client-go/tree/v0.37.1),
[metrics v0.37.1](https://github.com/kubernetes/metrics/tree/v0.37.1),
[kind v0.33.0 release and image digests](https://github.com/kubernetes-sigs/kind/releases/tag/v0.33.0),
[metrics-server v0.9.0](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.9.0).
