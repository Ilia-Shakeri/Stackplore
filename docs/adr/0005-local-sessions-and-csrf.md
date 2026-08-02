# ADR-0005: Use Local Server-Side Sessions and CSRF Protection

- Status: Accepted
- Date: 2026-07-28
- Implementation: Phase 7
- Owners: Project maintainers

## Context

Stackplore is a self-hosted v1 application with administrator and viewer roles.
It does not need federation or third-party identity in v1. It does need safe
first-run setup, password storage, session expiry and revocation, cookie
protection, rate limiting, and audit history.

Cookie authentication is a good fit for the same-origin application and SSE,
but browsers attach cookies automatically. State-changing routes therefore
need CSRF defense independent of interface confirmation. Storing bearer session
tokens directly would also turn a database read into active session compromise.

## Decision

Use local accounts, Argon2id password hashes, and opaque server-side sessions.

Password records include the Argon2id algorithm version and parameters so
successful login can rehash after an approved parameter change. Password input
has explicit byte limits and is never logged or copied into audit metadata.

On login, create a cryptographically random opaque session token. Send the raw
token only in an HttpOnly cookie and store only a keyed one-way digest with
user identifier, creation time, last-use policy, absolute expiry, and revocation
state. Resolve the current user status and role on every authorized request;
session records do not freeze authorization state. Rotate sessions on login and
any privilege-sensitive transition. Logout and administrative revocation
invalidate the server record.

Use a host-only cookie where deployment permits, with:

- HttpOnly;
- SameSite=Lax;
- Secure whenever the public origin is HTTPS;
- path `/api/v1`;
- no broad Domain attribute;
- a bounded lifetime consistent with server expiry.

State-changing API requests also require a synchronizer CSRF token bound to the
session and sent in a custom request header. The server validates it in constant
time, then validates the `Origin` header against the configured public origin.
An unsafe browser request with no `Origin` is rejected. Fetch metadata may
provide an additional rejection signal but is not the sole defense. Safe HTTP
methods never change protected application state.

Login and first-run setup use a bounded pre-auth variant because no session
exists yet. An auth preflight creates a short-lived server nonce record, sets
an HttpOnly SameSite pre-auth cookie, and returns a distinct header token bound
to that record. Login or setup requires the cookie, header token, and valid
`Origin`, then consumes the record. The nonce grants no authorization.

Authorization is enforced in application use cases, not only middleware. Roles
are `administrator` and `viewer`; the default is no permission. Login, setup,
password change, user management, and action attempts receive endpoint-specific
rate limits and safe audit events.

High-risk account, source, notification, export, cleanup, and infrastructure
changes require recent authentication in addition to the active session.
Reauthentication verifies the current password through the bounded login hash
path and creates a short-lived, one-use server-side grant bound to user,
session, and exact purpose. The operation consumes it atomically. Session or
account-security changes revoke every outstanding grant.

Changing one's own password requires that grant, rotates the current session,
and revokes all other sessions and grants. An administrative password reset
revokes all sessions and grants for the target user.

First-run administrator creation runs in one transaction and succeeds only when
the user table is empty. Concurrent setup attempts cannot create extra initial
administrators. A deployment reachable beyond loopback also requires a
short-lived, one-use bootstrap proof supplied through an operator-controlled
out-of-band channel. The proof is compared safely and never stored, logged,
returned in an error, or copied into audit metadata. Successful setup
permanently closes the network path. The last enabled administrator cannot be
disabled, deleted, or demoted. A local operator recovery command may reset or
create an administrator without reopening network setup. Password, role,
enabled-state, and recovery changes rotate or revoke affected sessions under
the rules above. No default credentials exist.

Authentication is required in production. A guest view is permitted only when
an explicit demo-deployment setting is active. That mode cannot configure real
sources, manage users as a normal installation, or perform infrastructure
actions.

A non-loopback production profile requires an HTTPS configured public origin
and accepts original-scheme and host forwarding headers only from explicitly
trusted proxy addresses.

## Consequences

### Benefits

- Sessions can be expired and revoked centrally.
- Same-origin HTTP and SSE share one authentication mechanism.
- A database disclosure does not immediately reveal usable raw session tokens.
- The design has no external identity dependency.
- CSRF validation is independent of browser interface behavior.

### Costs and risks

- Operators own account recovery and password policy.
- Session cleanup and revocation require durable storage.
- TLS termination and the configured public origin must be correct.
- Argon2id cost settings need performance and denial-of-service review.
- Per-request user and role lookup adds a small database read.

## Alternatives considered

### Stateless bearer tokens

They complicate revocation, increase browser storage risk, and do not remove the
need for CSRF if placed in a cookie.

### Store raw session tokens

This would make a database read sufficient to impersonate active users.

### SameSite cookies without CSRF tokens

SameSite is useful defense in depth but browser behavior, redirects, and future
changes make it insufficient as the only control for high-impact routes.

### OAuth or SSO

Those features add provider setup, callback, account-linking, and recovery
complexity outside the v1 scope.

## Implementation and verification

Phase 7 writes Argon2id version 19 records with a 16-byte random salt, a
32-byte hash, 64 MiB of memory, three iterations, and one lane. Passwords are
bounded to 12 through 128 UTF-8 bytes. The single-lane setting keeps CPU demand
predictable on small self-hosted systems; a process-wide two-worker limit also
bounds concurrent password work. The memory and iteration settings follow the
low-memory profile described by RFC 9106, while the lane count is an explicit
deployment tradeoff rather than a claim of profile identity.

On 2026-08-02, this repository's benchmark ran on Go 1.26.5 for windows/amd64
and an Intel Core i5-1334U:

```text
go -C apps/api test ./internal/auth -run '^$' -bench '^BenchmarkPasswordHasher$' -benchtime=3x -count=3
```

The median of the three reported means was 103.47 ms per hash with
67,113,176 bytes allocated per operation. This is a host-specific development
measurement, not a production capacity promise. Operators should repeat it on
the target host before changing the cost or concurrency bounds.

Phase 7 implements and tests:

- atomic first-administrator setup, permanent setup closure, one-use remote
  bootstrap proof validation, and local recovery without reopening setup;
- setup races, last-administrator protection, login rotation, sliding and
  absolute expiry, logout, user revocation, and live role or status changes;
- keyed session-token digests, one-use recent-auth grants bound to purpose,
  user, and session, and password-change session rotation;
- missing, invalid, expired, replayed, cross-cookie, cross-session, and
  cross-origin pre-auth, CSRF, and recent-auth attempts;
- cookie flags for HTTP development and HTTPS production origins;
- source- and account-aware durable throttle bounds with non-enumerating login
  responses, bounded retention, and fail-closed overflow buckets;
- safe authentication and user-management audit metadata plus integration
  scans proving raw passwords, session tokens, CSRF values, recent-auth proofs,
  and bootstrap proofs do not enter logs or database files.

## References

- [RFC 9106: Argon2 Memory-Hard Function](https://www.rfc-editor.org/rfc/rfc9106)
- [Architecture overview](../architecture.md)
- [ADR-0001: Monorepo and same origin](0001-monorepo-and-same-origin.md)
- [ADR-0003: SQLite and one active API instance](0003-sqlite-single-instance-and-repositories.md)
- [ADR-0006: Read-only and guarded mutations](0006-read-only-and-guarded-mutations.md)
