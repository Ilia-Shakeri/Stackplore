# ADR-0009: Use Feature-Owned Catalogs and One Localization Runtime

- Status: Accepted
- Date: 2026-07-31
- Owners: Project maintainers

## Context

Stackplore must provide English and Persian as equal product languages. A live
language change affects copy, document metadata, writing direction, typography,
keyboard behavior, overlay placement, numbers, dates, and plural forms. These
concerns must stay consistent without coupling product features to one large
global message module or scattering direct `Intl` calls through components.

The first language preference is local browser state. A later authenticated
preferences phase will add a server-backed user choice, but it must not replace
the rendering contract or create a second localization system.

## Decision

Use one small application localization runtime backed by paired, feature-owned
English and Persian JSON catalogs.

- Every production message has a stable, namespaced key owned by its feature.
- English defines the compile-time key shape. Persian supplies the same keys and
  message kinds.
- A repository check rejects missing, extra, duplicate, blank, malformed plural,
  and placeholder-mismatched messages.
- Plain messages and plural records are distinct typed calls. Every plural
  record contains an `other` form.
- The runtime uses browser `Intl` implementations with `en-US` for English and
  `fa-IR-u-ca-persian-nu-arabext` for Persian presentation.
- The selected `en` or `fa` value persists under `stackplore.locale`. A small
  pre-paint script applies `lang` and `dir` before the application mounts.
- The provider updates `document.lang`, `document.dir`, description metadata,
  and the shared primitive direction context whenever the locale changes.
- Technical identifiers retain their source text and render inside an explicit
  left-to-right bidirectional-isolation element.
- Missing runtime keys render a visible bracketed marker. The catalog gate is
  expected to prevent that state in a verified build.

Phase 24 may load an authenticated preference and then set the same provider.
Local storage remains the pre-authentication and first-paint source. Domain and
transport timestamps remain canonical UTC values; locale conversion stays in
presentation.

## Consequences

### Benefits

- Feature copy, tests, and ownership remain close without losing a single
  runtime contract.
- Live direction changes reach wrapped tabs, menus, drawers, tooltips, toasts,
  and document metadata without a reload.
- Built-in number, date, and plural behavior avoids another runtime dependency.
- Exact catalog parity and placeholder checks fail early and deterministically.

### Costs and risks

- Every visible behavior change must update two catalogs.
- Natural Persian still needs human review; key parity cannot judge writing
  quality.
- The pre-paint script and provider must retain the same storage and fallback
  rules.
- Layout and keyboard behavior still require real-browser review because a DOM
  test environment cannot prove geometry.

## Alternatives considered

### One repository-wide message file

This is initially simple but removes feature ownership and turns common edits
into a high-conflict file.

### Hardcoded component copy with selective translation

This makes language coverage incomplete and permits stale mixed-language
screens after a runtime switch.

### A larger message-formatting dependency

The current two-locale needs are covered by typed lookup, small placeholder
replacement, and browser `Intl`. Another runtime would add size and maintenance
without a required capability.

## Verification

The locale check validates all feature bundles. Component tests cover
persistence, document language and direction, both route catalogs, number,
date, plural, technical-text isolation, direction-sensitive keyboard behavior,
and localized error states. Browser review covers both directions at the
required responsive widths.

## References

- [Product specification](../../SPEC.md)
- [Project guide](../../PROJECT_GUIDE.md)
- [Architecture overview](../architecture.md)
- [ADR-0008: Project-owned interface system](0008-project-owned-interface-system.md)
