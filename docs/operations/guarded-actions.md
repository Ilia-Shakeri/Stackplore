# Guarded infrastructure actions

[فارسی](guarded-actions.fa.md). Stage 6 adds a small fixed action set. The
[active plan](../implementation-plan.md) separates local tests from disposable
provider proof and does not imply permission to mutate an operator's target.

## Actions and gates

The API accepts only Docker container restart, Kubernetes Deployment,
StatefulSet or DaemonSet rollout restart, and Deployment or StatefulSet replica
scaling. DaemonSet scaling, Demo resources, arbitrary patches, manifests, shell
commands and browser-selected hosts are rejected. Provider SDK types and raw
provider errors stay inside their adapters.

Every request needs a live administrator session, CSRF and same-origin checks,
the exact configured source/resource/version, explicit deployment mutation
enablement, separate provider write enablement, a server preview, a two-minute
one-use confirmation, a purpose-bound recent-password grant and an idempotency
key. PostgreSQL atomically rechecks the live actor, enabled non-synthetic source,
resource identity/version, confirmation, grant, capacity and audit intent before
dispatch. A target permits one active action. Provider calls and result
reconciliation have fixed deadlines; no request is automatically retried.

The result records a safe state and fixed reason code. A verified result is
success; a definite provider rejection is failure; a lost or unverified reply is
uncertain until a fixed provider receipt proves the operation. Durable history
shows the same receipt after reconnect. Recovery marks expired in-progress work
uncertain and writes its result audit; it does not replay the provider call.
Intent and result audits contain actor, resource, request ID and bounded reason,
never the confirmation, recent-auth proof or raw credential.

## Operator-owned write identities

All four permissions below default off: `MUTATIONS_ENABLED`,
`DOCKER_WRITE_ENABLED`, `KUBERNETES_WRITE_ENABLED`, and the source's persisted
enabled state. Turning on one deployment flag does not change another. The UI
reports the effective deployment flags but cannot change them. Demo mode refuses
all real provider and writer references.

Docker writes need a separate HTTPS writer origin, CA, client certificate and
private-key file reference (`DOCKER_WRITER_ORIGIN`, `DOCKER_WRITER_CA_FILE`,
`DOCKER_WRITER_CERT_FILE`, `DOCKER_WRITER_KEY_FILE`). The writer origin must
differ from the observation proxy. The adapter pins the origin and TLS identity,
allows only a fixed-version container inspect and one exact restart request, and
rejects redirects, request bodies, arbitrary query strings, identifiers and
retry-enabling headers. Keep its client identity separate from the read-only
observation proxy identity.

Kubernetes writes need separate operator-owned token and CA file references and
explicit writer namespaces (`KUBERNETES_WRITER_TOKEN_FILE`,
`KUBERNETES_WRITER_CA_FILE`, `KUBERNETES_WRITER_NAMESPACES`). Writer namespaces
must be a subset of observed namespaces. Give that service identity only the
required `get` and `patch` rights on deployments, statefulsets and daemonsets
inside those namespaces; do not grant cluster-admin or Secret reads. The
adapter enforces the same namespace and fixed API path/method allowlist even if
the token has broader rights. The write identity and observation identity must
use different token files.

## Rotate references without storing credentials

1. Prepare a new credential in the operator's secret store and mount it as a
   regular, bounded, private file. Do not paste it into a browser or command
   argument. Do not reuse the observation identity.
2. Update only the deployment-owned writer reference (and writer CA/origin when
   needed) through the deployment's protected secret/configuration channel.
   Browser settings accept none of these values. PostgreSQL stores no writer
   credential or file path.
3. Restart through the normal controlled deployment path. Docker TLS material
   and Kubernetes trust roots load at service startup; Kubernetes projected
   bearer-token files are reread for bounded requests. A bad reference must
   fail startup or the request closed, without falling back to observation
   credentials.
4. Verify readiness, exact source capability and the new identity's narrow
   permissions against an operator-approved disposable target. Review the
   operation receipt and audit result before revoking the old identity.
5. If verification fails, keep writes disabled, restore the last known-good
   operator reference through the same deployment channel, restart and verify
   readiness. Never repair a failed operation by resending it; inspect its
   durable receipt first.

This is an operator deployment procedure, not a browser credential editor or a
credential manager. Live target operation remains an explicit operator action.

Audit policy is reconciled at startup. Once `AUDIT_ARCHIVE_VERIFIED_THROUGH` is
stored, it is monotonic: startup rejects omitting it or setting an earlier time.
Keep the exact UTC cutoff in deployment configuration through every upgrade and
rollback. To extend expiry, independently verify a newer archive and move the
cutoff forward. The service does not verify archive contents for the operator.
