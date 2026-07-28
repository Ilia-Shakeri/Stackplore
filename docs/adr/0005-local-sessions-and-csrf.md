# ADR-0005: Use Local Server-Side Sessions and CSRF Protection

- Status: Accepted
- Date: 2026-07-28
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

Later phases must:

- benchmark and document selected Argon2id parameters;
- test setup races, login rotation, expiry, logout, revocation, and role change;
- test missing, invalid, expired, and concurrent bootstrap proof use;
- test last-administrator protection and local recovery without network setup;
- prove that raw session and CSRF values never enter storage, logs, errors, or
  audit metadata;
- test missing, invalid, cross-session, and cross-origin CSRF attempts;
- test missing, expired, replayed, cross-cookie, and cross-origin pre-auth CSRF
  attempts;
- test cookie flags under HTTP development and HTTPS production settings;
- test endpoint and account-aware rate limits without leaking account presence;
- test recent-auth expiry, replay, wrong-purpose, wrong-user, wrong-session, and
  revocation behavior;
- audit authentication and user-management outcomes with safe metadata.

## References

- [Architecture overview](../architecture.md)
- [ADR-0001: Monorepo and same origin](0001-monorepo-and-same-origin.md)
- [ADR-0003: SQLite and one active API instance](0003-sqlite-single-instance-and-repositories.md)
- [ADR-0006: Read-only and guarded mutations](0006-read-only-and-guarded-mutations.md)
