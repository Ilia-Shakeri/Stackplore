# Security Policy

## Current dependency correction

The unpublished alpha.1 candidate applies exact patched build/test dependency
versions after a failed signed dependency gate. The separate current package
audit reports no findings; [evidence and limits](docs/evidence/2.0.0-alpha.1/dependency-correction.json).
A fresh strict signed scan and complete source-bound gates must pass before a
tag or publication. Historical releases and scanner outcomes remain unchanged.

## Current compiler security patch

Current source pins Go 1.26.10 for the application and rebuilt gosu 1.19.
[GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809), alias CVE-2026-111943,
adds a crypto/x509 denylist for distrusted intermediate certificates; the
[official advisory](https://raw.githubusercontent.com/golang/vulndb/master/data/osv/GO-2026-6809.json)
fixes Go 1.26 at 1.26.10 and Go 1.27 at 1.27.3. The scanner found the older
standard library in the candidate gosu binary. This is component-presence proof,
not a claim of demonstrated reachability through the helper.

The alpha.1 signed trial `39592142674` failed its strict database image scan.
No alpha.1 tag was created for that attempt. Preserve its failure. The patch
requires verified tool archives, regenerated legal inventory, a new complete
local gate, exact-source CI, fresh complete scans, signed trial and independent
archive checks before acceptance. Historical scans below retain their original
source, compiler and vulnerability-database dates. No finding is suppressed.

## Project status

The maintained stable release line is **1.0.x**. Latest published stable **1.0.3**
is immutable at `656581cf05d4774358d5644a6e1af3c69a210f81`, schema 17. Its
[public receipt](docs/evidence/1.0.3/published-release.json) and
[review](docs/evidence/1.0.3/final-review.md) record the accepted corrective scope.
Those source-bound checks are historical snapshots and do not establish absence
of vulnerabilities disclosed later. The compiler correction described above is
still an unpublished alpha.1 candidate; it does not change downloaded stable
archives or their compiler.

## Historical 1.0.2 correction

Corrective **1.0.2** is immutable
`v1.0.2` at `376ed0379900ab870dd8eb8af6b55ef3e5ec389b`, schema 17. Code
and release review are **9/10**; all nine stages are complete after actual public
publication and independent archive verification.
Provider URL-userinfo masking, retained-event REST/SSE output, later-page event authority, canonical feed
ordering and literal native database selection have corrections. Focused
regressions and complete local D pass. All six exact main/tag jobs, the strict
signed trial/publication and actual public archive verification pass. The first tag/browser and
blocked-release attempts remain failed; unchanged-tag retry passed.
[Current evidence](docs/evidence/1.0.2/README.md) separates the later main-only
harness/report-retention fix from frozen release `376ed03`. Earlier C and
`18e8558` trial/CI retain their earlier-source scope.

An upgrade does not rewrite historical event rows, backups or prior downloads.
Restrict an exposed older runtime, deploy the verified correction and rotate
credentials actually affected by prior exposure. Protect retained state and
archives; bounded expired-history cleanup does not sanitize WAL or backups.
Preserve protected audit history. [Upgrade response](docs/operations/upgrade-rollback.md#retained-event-text-and-exposure-response)
([فارسی](docs/operations/upgrade-rollback.fa.md#متن-رویداد-قدیمی-و-پاسخ-به-افشا)).

Immutable `v1.0.1` remains at
`73ece4754554cca548c242768b195d21664d37b8` with its original
[published receipt](docs/evidence/1.0.1/published-release.json). Its historical
passing gates do not certify a different source or erase these fresh
findings. Report the exact version and full source commit.

The historical 1.0.1 derivative retains PostgreSQL 17.11 and the upstream entrypoint, rebuilding
unchanged official gosu 1.19 using Go 1.26.9. Exact full-image scans, bound SPDX,
engine/helper/root-drop probes and actual loaded archive SQL startup passed
without suppressions; [consumer receipt](docs/evidence/1.0.1/consumer-trial.json).
The earlier upstream-only candidate failed on one CRITICAL and 24 HIGH gosu
Go-library findings and remains rejected; [failed receipt](docs/evidence/1.0.1/failed-database-image.json).
The later source-main package-URL validation failure also remains failed
history; neither is relabeled as a passing release.

Runtime proof targets Windows amd64 and Linux amd64 native tools and Linux
amd64 application/database images. Linux arm64 and macOS archives remain
cross-built-only until separately executed. PostgreSQL server and native
`psql`, `pg_dump` and `pg_restore` clients require 17.11 or a later 17.x patch.
The [upstream security list](https://www.postgresql.org/support/security/17/)
and [minor-update procedure](docs/operations/upgrade-rollback.md#postgresql-17-patch-baseline)
([فارسی](docs/operations/upgrade-rollback.fa.md)) govern operator review.
No particular customer's server update is established by project tests.

Immutable 1.0.0 was withheld and has no published release; 0.9.0 is superseded
for its unsafe database default. Browser evidence covers the recorded
English/Persian profiles, not every assistive technology. Customer credentials,
ingress/TLS, provider rights, receivers, backups and capacity require separate
deployment-specific acceptance.

## Report a vulnerability privately

Do not open a public issue, discussion, or pull request for a suspected vulnerability.

Use one of these private paths:

1. Open a private vulnerability report through the repository's **Security** tab and GitHub private vulnerability reporting, when that option is available.
2. If private reporting is not available, use a private contact method
   published on the [maintainer profile](https://github.com/Ilia-Shakeri).

If neither private path is available, use the
[no-detail contact request form](https://github.com/Ilia-Shakeri/stackplore/issues/new?template=private_contact_request.yml).
That request is public. Disclose only that a private channel is needed. Do not
include vulnerability details, credentials, exploit steps, sensitive logs, or
affected data.

## What to include

Provide enough information to reproduce and assess the issue safely:

- the affected commit, branch, deployment mode, or prerelease artifact;
- the component and configuration involved;
- clear reproduction steps or a minimal proof of concept;
- expected and observed behavior;
- likely impact and required access;
- relevant sanitized logs or screenshots;
- any known workaround;
- whether the issue has been disclosed elsewhere.

Remove real credentials, personal data, provider tokens, session cookies, cluster configuration, database contents, and unrelated secrets.

## Response process

Maintainers review private reports as availability permits, confirm a safe
communication path, and coordinate validation and remediation when the report
is in scope. No fixed acknowledgement, remediation, release or disclosure
deadline is promised.

Please allow maintainers to investigate and prepare a fix before public disclosure. Disclosure timing will be discussed with the reporter and may depend on severity, release status, downstream impact, and the availability of a safe update.

## Scope

Reports are especially useful when they concern:

- authentication, session handling, CSRF, or authorization;
- first-run administrator ownership;
- credential, metadata, log, or Secret value exposure;
- Docker or Kubernetes privilege expansion;
- arbitrary file access or path traversal;
- unsafe infrastructure-changing actions;
- webhook SSRF or unsafe outbound requests;
- injection, cross-site scripting, or cache disclosure;
- audit, backup, update, or supply-chain integrity.

General feature requests, configuration questions, dependency update suggestions without a demonstrated security impact, and denial of service that requires host administrator control may be handled through normal project channels.

Only test systems and data you own or have explicit permission to assess. Avoid privacy violations, destructive actions, persistence, service disruption, and access beyond what is needed to demonstrate the issue.

## Security design

The threat model and implemented controls are documented in
[docs/security.md](docs/security.md). The [requirement matrix](docs/evidence/0.9.0/requirements.md)
maps controls to implementation and checks. Design documentation and automated
tests do not certify the absence of every vulnerability or a customer's live
installation.
