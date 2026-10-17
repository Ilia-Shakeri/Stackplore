# Deployment

## Local Compose

The Compose profile binds the edge only to `127.0.0.1`, keeps PostgreSQL and the local digest key in named volumes, and waits for database and API readiness before starting the web edge.

1. Copy `deploy/compose.env.example` to `.env`.
2. Replace both password placeholders. URL-encode the password inside `STACKPLORE_COMPOSE_DATABASE_URL`.
3. For a fresh database, generate paired short-lived setup proof/expiry in the
   operator shell using [first setup](account-recovery.md). Compose passes the
   two values through without saving them in `.env`. The API binds a private
   wildcard address and therefore still requires this ownership proof.
4. Run `docker compose up --build -d`, check `docker compose ps`, then open
   `http://localhost:8080/setup` and complete first administrator setup.
   Remove both setup values from the shell and recreate only the API with
   `docker compose up -d --force-recreate api` after setup closes.
5. Stop without data loss using `docker compose down`. Do not add `--volumes` unless permanent data removal is intended.

This profile uses PostgreSQL without transport TLS only on its private local bridge. It is not proof of a production database or edge.

## Cluster chart

The chart does not install a database. Supply an external PostgreSQL URL using `sslmode=verify-full` in an existing Secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: stackplore-runtime
type: Opaque
stringData:
  database-url: postgresql://user:password@database.example:5432/stackplore?sslmode=verify-full
```

Use a secret manager in real environments; the manifest above only documents the required key shape. Set the real image repositories, immutable tags, public origin, ingress host, trusted proxy CIDRs, and storage class in a private values file. Then run:

For a fresh empty database, add the two short-lived setup keys named by
`api.setup.proofSecretKey` and `api.setup.expirySecretKey` to the existing
operator-managed Secret, and set `api.setup.enabled=true` for the first rollout.
Generate them using [first setup](account-recovery.md). Enter the proof at
`/setup`; after completion, disable the setup block and remove those keys from
the Secret. Existing initialized databases do not need setup values. Never
paste the proof into values files, command arguments or a public manifest.

```sh
helm upgrade --install stackplore deploy/helm/stackplore \
  --namespace stackplore --create-namespace \
  --values values.production.yaml \
  --atomic --wait
```

The API stays at one replica because its digest key uses a ReadWriteOnce persistent volume and the current process enforces single-writer ownership. The web edge defaults to two replicas and has a disruption budget.

The Kubernetes source defaults off. Enabling `api.kubernetes.enabled` requires
one to 64 explicit entries in `api.kubernetes.namespaces`. Each namespace gets
a read Role and RoleBinding for workloads, pods/logs, services, claims,
ingresses, endpoint slices, events and optional Metrics API pods. Cluster
observation is a separate flag; its ClusterRole adds only node lists and GETs
for the named namespaces. Logs/events remain sensitive read surfaces. No Secret
or ConfigMap value reads are granted. Optional-kind denial preserves other
allowed partitions. Use the [provider guide](../kubernetes-provider.md) for the
exact access and token-rotation boundaries. Do not grant cluster-admin.

Generic webhook delivery stays off by default. For Compose, set `STACKPLORE_WEBHOOK_ENABLED=true`, a stable safe `STACKPLORE_WEBHOOK_DESTINATION_REFERENCE_ID`, an HTTPS `STACKPLORE_WEBHOOK_URL`, and a canonical unpadded base64url 32-byte `STACKPLORE_WEBHOOK_SIGNING_KEY`. For the chart, set `api.webhook.enabled=true`; place the URL and signing key under `api.webhook.urlSecretKey` and `api.webhook.signingKeySecretKey` in `api.existingSecret`. Never put either secret value in a values file. Delivery uses stable idempotency keys, fixed signed payload bytes, bounded attempts, and PostgreSQL leases. Live receiver proof remains a separate target-environment check.

Matrix also defaults off. Compose forwards `STACKPLORE_MATRIX_ENABLED`,
`STACKPLORE_MATRIX_DESTINATION_REFERENCE_ID`, `STACKPLORE_MATRIX_ORIGIN`,
`STACKPLORE_MATRIX_ROOM_ID` and `STACKPLORE_MATRIX_ACCESS_TOKEN`. Use one exact
canonical HTTPS origin and an existing room; discovery is disabled. In Helm,
enable `api.matrix.enabled`, set its safe `destinationReferenceId`, and place
origin, room and token under `originSecretKey`, `roomIdSecretKey` and
`accessTokenSecretKey` in `api.existingSecret`. No credential belongs in values
or rendered reports. Both lanes reject private destinations and redirects;
preserve their logical references across restarts for stable delivery identity.
Use a new guarded test for each lane and inspect terminal audit; queued is not
receipt. See [notification semantics](../alerts-notifications.md).

The Compose profile keeps both live sources off. The current direct Docker
socket adapter is accepted only in an explicit loopback development profile;
production startup rejects it. A restricted production proxy is roadmap work.
Do not mount a host socket into this wildcard-bound Compose deployment.

## Release checks

- `corepack pnpm verify`
- PostgreSQL integration tests with `STACKPLORE_TEST_POSTGRES_URL`
- `docker compose --env-file deploy/compose.env.example config --quiet`
- `helm lint deploy/helm/stackplore --strict`
- Image build and vulnerability scan in the release registry
- Backup, empty-database restore, and rollback rehearsal against the target PostgreSQL service
- Browser smoke checks in English, Persian RTL, keyboard-only use, and narrow/mobile widths
