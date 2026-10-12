# Current user guide

This describes the authenticated 0.1 baseline. It is not a promise that every
1.0 feature exists. See [remaining work](roadmap.md) and
[راهنمای فارسی](user-guide.fa.md).

1. Ask the operator to start the installation with PostgreSQL. A fresh
   installation exposed beyond loopback needs a short-lived setup proof from
   the operator. There is no default account.
2. Open `/setup`, choose the language, enter the required operator proof, and
   create the first administrator. Later users sign in at `/login`.
3. Choose a source in the product header. Demo data is synthetic. Real Docker
   and Kubernetes reads are operator enabled; the connection page reports
   status, and does not create a persistent connection through the browser.
4. Inspect `/overview`, `/resources`, and a resource's stable detail link. Health
   and capability status distinguish available, stale, unavailable, unsupported,
   and denied data. A displayed partial page is not the whole estate.
5. Use `/topology` for the graph and its list alternative, `/logs` for bounded
   current log reads, and `/events` for infrastructure events. Do not treat an
   inferred edge as a guaranteed dependency. Rich paging, correlated history
   and full log controls are still being completed.
6. Read `/alerts` and `/notifications`. Administrators can acknowledge an alert,
   manage alert rules, inspect safe notification status, and request a guarded
   test delivery when operator webhook configuration allows it. A queued test
   is not proof that a receiver accepted it.
7. Administrators use `/settings/users` for account management and `/audit` for
   protected history. High-risk account/test operations request the current
   password to obtain a purpose-bound one-use grant.
8. Switch English/Persian and the theme in the header. Preferences persist
   locally; the broader preference settings page is planned. Sign out through
   the account menu.

Viewers cannot administer users/rules/global settings or mutate infrastructure.
The current baseline exposes no live restart/scale action. The installable PWA,
guest demonstration deployment, persistent browser source setup, full detail
tabs, and complete history/export/cleanup flows remain roadmap work.

Loading, denied, unavailable and disconnected states deserve attention. Retry a
failed read; if it persists, give the operator only the safe error code and
request ID. Do not share raw logs, credentials, session data or private targets.
See [troubleshooting](operations/troubleshooting.md).
