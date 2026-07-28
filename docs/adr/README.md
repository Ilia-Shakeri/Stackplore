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

| Record | Status | Decision |
| --- | --- | --- |
| [ADR-0001](0001-monorepo-and-same-origin.md) | Accepted | Use one repository and one production origin |
| [ADR-0002](0002-provider-neutral-boundary.md) | Accepted | Put provider-neutral domain ports between use cases and infrastructure clients |
| [ADR-0003](0003-sqlite-single-instance-and-repositories.md) | Accepted | Use SQLite with one active API instance behind repository ports |
| [ADR-0004](0004-server-sent-events.md) | Accepted | Use bounded SSE streams for live server-to-browser data |
| [ADR-0005](0005-local-sessions-and-csrf.md) | Accepted | Use local server-side sessions with synchronizer CSRF protection |
| [ADR-0006](0006-read-only-and-guarded-mutations.md) | Accepted | Keep infrastructure read-only by default and gate narrow actions |
| [ADR-0007](0007-deterministic-isolated-demo.md) | Accepted | Run deterministic demonstration fixtures in an isolated mode |

## Adding a record

1. Copy [`0000-template.md`](0000-template.md).
2. Allocate the next four-digit number; never reuse a number.
3. Name the file `NNNN-short-kebab-title.md`.
4. Describe the current constraints and at least the serious alternatives.
5. State measurable consequences and verification work.
6. Add the record to this index.
7. Link any superseded record in both directions.

