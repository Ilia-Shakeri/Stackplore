# Repository Conventions

This document defines repository-wide contribution and Git conventions. The
rules apply to application code, deployment files, tests, and documentation.

## 1. Change scope

- One numbered delivery phase maps to one coherent commit.
- Do not mix refactors, dependency updates, and feature work unless the current
  phase requires all of them.
- Preserve unrelated work in the worktree.
- Add only files owned by the current change to the index.
- Update tests and documentation in the same change as behavior.
- Record a deliberate architecture change in `docs/adr`.

## 2. Branches

The default branch is `main`.

Suggested short-lived branch names:

```text
feat/<short-name>
fix/<short-name>
docs/<short-name>
chore/<short-name>
```

Use lowercase words separated by hyphens. Do not encode personal names, issue
tracker state, or dates unless they add project value.

## 3. Conventional Commits

Commit subjects use:

```text
<type>(<optional-scope>): <imperative summary>
```

Allowed common types:

- `feat`: user-visible capability;
- `fix`: defect correction;
- `docs`: documentation-only change;
- `refactor`: internal change with no intended behavior change;
- `test`: test-only change;
- `perf`: measured performance change;
- `build`: build or packaging change;
- `ci`: continuous integration change;
- `chore`: repository maintenance.

Useful scopes include `api`, `web`, `auth`, `storage`, `demo`, `docker`,
`kubernetes`, `topology`, `logs`, `metrics`, `events`, `alerts`, `actions`,
`settings`, `pwa`, and `i18n`.

Subject rules:

- use lowercase after the colon unless a proper name requires otherwise;
- use imperative mood;
- omit the trailing period;
- describe the outcome, not the activity;
- keep the first line concise;
- add a body when the reason or migration impact is not obvious;
- add a `BREAKING CHANGE:` footer only for a real incompatible contract change.

Examples:

```text
docs: define Stackplore product architecture and delivery plan
feat(auth): add secure sessions and role-based access control
fix(logs): bound reconnect buffers for slow clients
```

## 4. Phase commit rule

Before a phase commit:

1. Confirm the current phase in `PLAN.md`.
2. Format changed files.
3. Run phase checks and existing regression checks.
4. Run `git diff --check`.
5. Check every untracked text file for UTF-8, declared line endings, trailing
   whitespace, and a final newline.
6. Review every changed and untracked file.
7. Update the phase checklist and `CHANGELOG.md`.
8. Confirm no secret or private infrastructure data is present.
9. After explicit phase paths are staged, run `git diff --cached --check` and
   review `git diff --cached`.
10. Confirm the commit subject matches the actual diff.

The phase checklist is marked complete inside the phase commit. The next phase
does not repair the prior phase's tracking state.

## 5. File and symbol names

- Go package names are short, lowercase, and singular where idiomatic.
- Go exported names follow standard initialism casing such as `HTTP`, `API`,
  `URL`, and `ID`.
- TypeScript files use `kebab-case` except React component files, which use
  `PascalCase.tsx`.
- Test files stay close to their subject unless an integration boundary needs a
  dedicated test tree.
- Translation keys use feature namespaces and stable semantic names.
- Database migration files use a monotonic numeric prefix and descriptive name.
- ADR files use `NNNN-kebab-case-title.md`.

## 6. Formatting

- UTF-8 and final newlines are required.
- Line endings follow `.gitattributes`.
- Indentation follows `.editorconfig`.
- Go formatting uses `gofmt`.
- Frontend formatting and lint commands are defined in Phase 1.
- Markdown uses one top-level heading per file and relative internal links.
- YAML uses two-space indentation and no tab characters.

## 7. Backend boundaries

- Domain packages do not import HTTP, SQLite, Docker, or Kubernetes packages.
- Services coordinate use cases and own policy decisions.
- Repositories own persistence translation.
- Provider adapters own SDK translation and capability detection.
- HTTP handlers validate transport input and map service results.
- Cross-cutting middleware stays small and has focused tests.
- Time, identifiers, and external clients are injectable at test boundaries.

## 8. Frontend boundaries

- Route files compose feature screens.
- Features own their query hooks, forms, view models, and tests.
- Shared UI components contain no infrastructure business rules.
- The API client owns transport and problem parsing.
- Query keys are centralized by feature.
- URL state owns shareable filters and selections.
- Locale catalogs contain all production copy.
- English and Persian catalogs are added and changed together inside the owning
  feature.
- Catalog keys, message kinds, plural forms, and named placeholders stay in
  exact parity.
- Plural messages include an `other` form and visible copy contains no raw
  markup.
- Components use the shared locale formatter for dates, numbers, and plurals
  instead of scattered host-locale calls.
- Technical identifiers are rendered without locale mutation.
- Directional icons, keyboard order, overlays, and compact layouts are reviewed
  in both directions.

## 9. Data and migrations

- Migrations are forward-only after release.
- Schema changes and repository changes ship together.
- A migration is idempotent only when explicitly designed that way.
- Store timestamps in UTC.
- Use stable opaque IDs at HTTP boundaries.
- Enforce foreign keys.
- Add indexes from known access patterns, not speculation.
- Test new migrations from an empty database and from the prior schema.
- Retention deletes use bounded batches.

## 10. Security-sensitive changes

A security-sensitive change includes authentication, authorization, sessions,
CSRF, provider access, file paths, redaction, outbound delivery, mutation,
audit, or deployment permissions.

Such changes require:

- negative tests;
- safe failure messages;
- structured logs without sensitive values;
- explicit time and size bounds;
- threat-model review;
- documentation of any new trust boundary.

Security reports follow [SECURITY.md](../SECURITY.md).

## 11. Dependencies

- Use one dependency per responsibility.
- Pin exact versions.
- Commit lock and checksum files.
- Check license and active maintenance.
- Prefer standard library behavior when it stays clear and safe.
- Avoid a dependency for a trivial helper.
- Keep provider SDK use inside its adapter.
- Refresh and test one dependency group at a time.

## 12. Tests and fixtures

- Tests must be deterministic.
- Freeze or inject time.
- Avoid network access in unit tests.
- Use synthetic resource names and addresses.
- Keep fixture secrets clearly invalid and non-sensitive.
- Test permission denial, cancellation, timeout, and overflow paths.
- Do not weaken assertions to hide nondeterminism.

## 13. Documentation truth

- Use present tense only for behavior that exists.
- Use future or planned language for unfinished work.
- Do not publish placeholder screenshots as product evidence.
- Do not publish estimated coverage, benchmarks, or compatibility claims.
- Link claims to a verification command or release artifact when practical.

## 14. Review checklist

- Scope matches the active phase.
- Architecture boundaries remain intact.
- User-facing strings exist in English and Persian.
- Keyboard and direction behavior were considered.
- Sensitive values cannot enter responses, logs, events, or audit records.
- Work is bounded in time, memory, and storage.
- New behavior has useful tests.
- Documentation and change history are accurate.
- The full diff contains no unrelated file.
