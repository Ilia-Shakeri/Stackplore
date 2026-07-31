# ADR-0008: Keep Interface Primitives Behind One Project-Owned Package

- Status: Accepted
- Date: 2026-07-29
- Owners: Project maintainers

## Context

Stackplore needs a consistent, accessible interface across infrastructure
inventory, topology, logs, metrics, events, alerts, settings, and guarded
operations. Those screens need shared semantic colors, spacing, typography,
focus behavior, themes, overlays, navigation, and feedback. Letting each
feature compose third-party primitives or raw styles directly would make
accessibility fixes, visual changes, and future direction-aware behavior hard
to apply consistently.

The interface must support light, dark, and system themes without a runtime
font or stylesheet service. Phase 3 will add Persian and right-to-left behavior,
so Phase 2 foundations must use logical layout properties and keep font and
direction choices centralized.

## Decision

Use the private `@stackplore/ui` workspace package as the only shared interface
primitive boundary.

- CSS custom properties own semantic color, typography, spacing, radius,
  elevation, focus, motion, and layer tokens.
- Tailwind CSS consumes those semantic tokens for application composition.
- Radix primitives provide accessible behavior for overlays, menus, tabs,
  toasts, and tooltips, while the basic select wrapper keeps native HTML
  semantics. Application code imports only project wrappers.
- Lucide is the single application icon family and is exposed through a narrow
  project icon entry point.
- Inter and Vazirmatn variable fonts are bundled through exact Fontsource
  packages. Their separate license notices ship with the web assets.
- A document `data-theme` value owns the resolved light or dark palette. The
  stored preference may be `light`, `dark`, or `system`.
- The package remains source-only and private. Vite compiles the same source
  used by the web application, so no stale generated package copy exists.

Feature code may extend layout around a wrapper, but it must not import a
wrapped primitive directly or redefine global semantic tokens.

## Consequences

### Benefits

- Focus, keyboard, overlay, status, and theme behavior have one repair point.
- Product features use stable project vocabulary instead of third-party APIs.
- Light, dark, Persian, and direction-aware changes can evolve without
  replacing every feature component.
- Exact source and dependency pins keep the development build reproducible.

### Costs and risks

- Project wrappers require maintenance and focused behavior tests.
- A wrapper can hide useful upstream behavior if its public API is too narrow.
- Source-package changes affect the web build immediately and must pass the
  complete frontend gate.
- The pre-paint theme script and runtime provider must keep one storage and
  resolution contract.

## Alternatives considered

### Import third-party primitives in features

This reduces wrapper code initially but spreads dependency APIs and
accessibility policy through every feature.

### Adopt a complete themed component kit

This adds a second visual system, makes the original design direction harder to
enforce, and increases replacement cost.

### Keep all interface code inside the web application

This avoids a workspace package but weakens the architecture boundary intended
for a future desktop wrapper and makes feature imports easier to couple.

## Implementation and verification

Phase 2 owns the initial package, semantic tokens, themes, bundled fonts,
wrappers, responsive shell, component tests, and manual width review. Phase 3
adds the shared direction context and verifies the same boundary under English,
Persian, left-to-right, and right-to-left behavior. Later feature phases add
wrapper capability only when a real product flow requires it.

Dependency updates must refresh official metadata, preserve exact manifest and
lockfile pins, and run the full frontend verification task.

## References

- [Product specification](../../SPEC.md)
- [Project guide](../../PROJECT_GUIDE.md)
- [Architecture overview](../architecture.md)
- [ADR-0001: Monorepo and same origin](0001-monorepo-and-same-origin.md)
- [ADR-0009: Localization runtime](0009-feature-owned-localization-runtime.md)
