# Application settings and browser preferences

[فارسی](settings.fa.md). This guide describes the 1.0.3 candidate behavior. The
[active plan](implementation-plan.md) records verification and publication;
the presence of this guide is not a passing runtime or release gate.

## Who can use each page

Authenticated administrators and viewers can read **Settings → General**.
Viewers see effective values, application version, deployment write flags and
the protected audit policy. Only administrators can review and commit changes,
generate configuration exports or clean up eligible history. The server repeats
role, session, origin and CSRF checks; hiding a control is not authorization.

Guest demonstrations cannot call application settings or write routes. They can
use **Settings → Preferences**, which changes only the local browser display.
No account, server mutation or infrastructure call is needed for preferences.

## Account table windows

Only administrators can open **Settings → Users**. The table renders at most
50 accounts in one window. Previous/next controls retain access to every loaded
account; a new API page is fetched only at the end of the loaded cache. Refresh
or a successful account action resets the visible window. Loaded counts do not
claim a total installation size. Final-administrator safeguards consider the
full loaded cache and whether the final server page is known, and the server
still enforces them atomically. Manually requested cache entries remain until
refresh/unmount, so the render window is not a total-cache memory bound.

## Local display preferences

Language is English or Persian; theme is light, dark or system. Calendar,
12/24-hour clock and Latin/Persian digits can follow the language default or be
selected explicitly. Date, number, translated numeric counts and plural counts
use the same central formatters. Technical identifiers and canonical timestamps
keep their original values and LTR isolation; changing calendar does not change
an observation's instant. The preview uses a fixed UTC example.

Language and theme keep their existing browser keys, `stackplore.locale` and
`stackplore.theme`. Additional formats use `stackplore.format-preferences`, a
closed version-1 JSON object with `version`, `calendar`, `digits` and `hourCycle`.
Reads are capped at 256 characters and reject unknown keys, versions or values.
Invalid storage falls back to language defaults; valid storage events update
other open tabs. If a write is blocked, the selected format still works for the
visit and the page reports the actual save failure. Reset restores only date
and number format defaults. These values are not part of server configuration
export and do not travel between devices or accounts.

## Effective application policy

| Setting | Default | Accepted whole-number range |
| --- | --- | --- |
| Metric retention, hours | 168 | 1–8760 |
| Health history retention, hours | 720 | 1–8760 |
| Event history retention, hours | 720 | 1–8760 |
| Terminal notification delivery retention, hours | 720 | 1–8760 |
| Log lines per request | 200 | 10–200 |
| Default alert window, seconds | 300 | 60–86400 |
| Default alert pending period, seconds | 0 | 0–3600 |

Deployment mutation flags are read-only on this page. Enabling an application
flag alone never grants provider write authority or bypasses an action preview.
Audit retention/quota are separately operator controlled, with a 90-day minimum;
general history cleanup cannot edit or prune audit history.

The scheduled history worker resolves all four retention values in each bounded
PostgreSQL transaction and holds the settings row's shared lock until deletion
ends. A concurrent policy update takes effect before or after that whole batch;
the next batch reloads policy. A longer period preserves rows still present but
cannot restore rows already deleted. Defaults are not fixed worker constants.

New alert-rule drafts load the effective default window and pending period from
the application settings API. Creation waits for that read; a failed read shows
a retry state. Late settings responses preserve fields already edited by the
administrator. Existing rules retain their stored values; changing the defaults
does not rewrite them. A new draft after reset uses the loaded effective values.

## Review and change a policy

1. Enter the intended values and choose **Review changes**. No setting changes
   during preview. The request binds the current policy version with `If-Match`.
2. Read the exact before/after values. When a retention period decreases, review
   every newly affected history class and its server-frozen UTC cutoff/count.
   A capped count of 10,001 means **at least 10,001**, not a complete total.
3. Check the effect confirmation. A shorter retention policy also requires an
   explicit backup/data-loss acknowledgement. The limit of 1,000 rows applies
   to each cleanup batch; an approved shorter policy continues expiring eligible
   rows over later bounded runs and can remove more than 1,000 rows in total.
4. Enter the current password. A fresh one-use `settings.update` grant and the
   two-minute actor/session/version/material-bound confirmation authorize the
   exact update. The password is cleared before dispatch and is never part of
   the settings payload or an export.

A version conflict, expired preview or denial requires a new review. A submitted
confirmation stays disabled. A lost acknowledgement or server write failure
can have an uncertain result: reload effective settings and inspect audit
history before deciding on another action. The page does not retry that write.

## Explicit bounded history cleanup

Select only metric samples, health history, infrastructure events and terminal
notification deliveries. The server applies the current retention policy,
protects ineligible/current/pinned facts, and returns a frozen cutoff and a
bounded count for each selected class. It never accepts a table name, arbitrary
SQL, host path or caller-selected provider endpoint.

Review the scope, backup warning and hard confirmation, then supply fresh
`data.cleanup` authentication. One commit removes at most 1,000 eligible rows
in total and audits the result atomically. A remaining-work flag means another
batch needs a new preview and confirmation. The browser does not loop deletion.
Users, sessions, sources, current settings and audit records are excluded.

## Audit lifecycle

`AUDIT_RETENTION_DAYS` defaults to 365 and cannot be lower than 90. The global
`AUDIT_QUOTA_ROWS` defaults to 1,000,000. Audit expiry stays off until an
operator independently verifies a protected archive and configures
`AUDIT_ARCHIVE_VERIFIED_THROUGH` as its UTC cutoff. The service never advances
or rewinds that cutoff automatically. Policy changes create immutable audit
entries. A daily worker prunes at most eight batches of 1,000 rows, older than
both the retention age and verified cutoff, and preserves entries linked to
operations or notification deliveries. No archive cutoff means no audit expiry.
Synthetic fixture reset has its own [separate lifecycle](demo.md).

## Masked configuration export

An administrator confirms generation and supplies fresh `configuration.export`
authentication. The fixed POST route accepts no destination or output path.
It returns an allowlisted JSON artifact with `Cache-Control: no-store`, capped
at 65,536 encoded bytes, and records generation in audit. The browser repeats
the closed-field/masking/size checks before offering
`stackplore-configuration.json` for download.

The artifact includes approved policy values and safe source/integration
summaries. Provider display names, kinds, profile names and enabled flags remain;
host, target and logical credential-reference fields are exactly `[masked]`.
Passwords, tokens, runtime keys, raw environment, kubeconfig content and provider
payloads are excluded. This file cannot restore credentials. Audit records
generation, not whether a browser successfully saved a file to disk.

## HTTP boundaries and verification

The canonical [contract](../contracts/openapi.yaml) defines
`GET /api/v1/settings/application`, application previews and exact-version PUT,
`POST /api/v1/settings/export`, cleanup previews and bounded cleanup POST.
Browser transport parses unknown responses, checks preview/result scope and
cutoffs, and keeps confirmation/recent-auth material only in memory.

Focused tests cover numeric/schema bounds, unknown fields, masked exports,
count sentinels, version/scope/cutoff binding, preview expiry, password clearing,
single submission, uncertain outcomes, viewer controls and English/Persian
formatting. Fresh real PostgreSQL 17.11 tests also cover concurrent one-use
settings commits across all seven fields; version, material, confirmation,
backup, expiry and purpose rejection; audit-failure rollback; 1,000-row cleanup
with frozen cutoffs and remaining-work reporting; and masked export with one-use
authentication. Rejected or rolled-back transactions leave settings unchanged
and do not spend confirmations or grants. These local tests are distinct from candidate full-gate, browser
download and publication evidence in [testing](testing.md) and the active plan.
