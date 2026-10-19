# Architecture Decision Records

This directory records durable architecture decisions for Stackplore. A record
captures why a choice was made, the trade-offs accepted, and what later work
must verify. Accepted records describe target architecture until the relevant
implementation phase provides evidence.

## Status meanings

- **Proposed:** under review and not yet binding.
- **Accepted:** the intended design for future work.
- **Superseded:** replaced by a later record, with links in both records.
- **Deprecated:** retained for history but no longer recommended.
- **Rejected:** considered and deliberately not selected.

Accepted records are immutable in their original decision and consequence
sections. Material changes require a new record that supersedes the old one.
Small corrections, links, and implementation notes may be appended without
changing the decision.

## Index

| Record                                                          | Status                 | Decision                                                                                  |
| --------------------------------------------------------------- | ---------------------- | ----------------------------------------------------------------------------------------- |
| [ADR-0001](0001-monorepo-and-same-origin.md)                    | Accepted               | Use one repository and one production origin                                              |
| [ADR-0002](0002-provider-neutral-boundary.md)                   | Accepted               | Put provider-neutral domain ports between use cases and infrastructure clients            |
| [ADR-0003](0003-sqlite-single-instance-and-repositories.md)     | Superseded by ADR-0011 | Historical SQLite single-instance decision                                                |
| [ADR-0004](0004-server-sent-events.md)                          | Accepted               | Use bounded SSE streams for live server-to-browser data                                   |
| [ADR-0005](0005-local-sessions-and-csrf.md)                     | Accepted               | Use local server-side sessions with synchronizer CSRF protection                          |
| [ADR-0006](0006-read-only-and-guarded-mutations.md)             | Accepted               | Keep infrastructure read-only by default and gate narrow actions                          |
| [ADR-0007](0007-deterministic-isolated-demo.md)                 | Accepted               | Run deterministic demonstration fixtures in an isolated mode                              |
| [ADR-0008](0008-project-owned-interface-system.md)              | Accepted               | Keep interface primitives behind one project-owned package                                |
| [ADR-0009](0009-feature-owned-localization-runtime.md)          | Accepted               | Use feature-owned catalogs and one localization runtime                                   |
| [ADR-0010](0010-versioned-contract-and-derived-client.md)       | Accepted               | Version the HTTP contract and derive the browser client                                   |
| [ADR-0011](0011-postgresql-primary-persistence.md)              | Accepted               | Use PostgreSQL as the primary persistence system                                          |
| [ADR-0012](0012-durable-alert-change-replay.md)                 | Accepted               | Replay committed source-scoped alert changes across restarts                              |
| [ADR-0013](0013-provider-coverage-and-demo-lifecycle.md)        | Accepted               | Publish bounded inventory authority by kind and scope; isolate demonstration lifecycle    |
| [ADR-0014](0014-read-only-helper-scope-and-boundaries.md)       | Accepted               | Keep the version 2 helper read-only, evidence-bound and within existing domain ports      |
| [ADR-0015](0015-helper-durable-runtime-and-volatile-context.md) | Accepted               | Persist typed runs and protected usage metadata; keep working text volatile               |
| [ADR-0016](0016-helper-authority-and-egress-policy.md)          | Accepted               | Share evidence privacy policy and bind local/remote inference egress to operator profiles |

Records 0014–0016 are accepted version 2 target design from roadmap stage 0.
Their acceptance does not ship a helper, HTTP route, migration, runtime profile
or release. Implementation and source-bound proof remain in the active plan.

## Adding a record

1. Copy [`0000-template.md`](0000-template.md).
2. Allocate the next four-digit number; never reuse a number.
3. Name the file `NNNN-short-kebab-title.md`.
4. Describe the current constraints and at least the serious alternatives.
5. State measurable consequences and verification work.
6. Add the record to this index.
7. Link any superseded record in both directions.
