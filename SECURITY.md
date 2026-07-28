# Security Policy

## Project status

Stackplore has no released version yet. The repository is in its initial design phase, so there is currently no supported release range or production-security claim.

This policy will be updated with supported versions and release-specific guidance before the first release.

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

Maintainers will review private reports as availability permits, confirm a safe communication path, and coordinate validation and remediation when the report is in scope. No fixed acknowledgement, remediation, release, or disclosure deadline is promised at this prerelease stage.

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

The initial threat model and required controls are documented in [docs/security.md](docs/security.md). That document describes intended architecture; it does not certify that unreleased work is complete or audited.
