# Exploration checkpoint evidence — 2026-10-17

These files record the running disposable PostgreSQL/API/web fixture, not a
production deployment. No live provider or customer data was used. The browser
used the development proxy with the actual API, session cookies and CSRF checks.
The display name in captures belongs to an invented test account.

- [Loaded product matrix](loaded-matrix.json): 120 checks, six routes, English/Persian,
  light/dark and widths 360, 768, 1024, 1440 and 1920. Each check waited for its
  route's actual fixture data, checked document direction, one main region,
  document overflow and header control clipping. All passed.
- [Authentication form matrix](auth-matrix.json): 40 setup/login layout checks
  across the same locales, themes and widths. Real browser login, logout and
  login again passed. Account creation was exercised through the HTTP/native
  drill; setup browser proof covers its required-field rejection and layout.
- [Final topology matrix](final-topology-matrix.json): all 20 topology states
  repeated after the last edge-routing correction; all passed.
- [English dark topology](final-topology-en-dark-1440.png),
  [Persian light topology](final-topology-fa-light-1440.png) and
  [Persian mobile logs](loaded-logs-fa-dark-360.png) show loaded data.
- [SHA-256 checksums](SHA256SUMS) identify these immutable proof files.

Direct keyboard checks exercised the mobile menu focus trap, Escape and focus
return, visible focus, graph controls, evidence disclosures and chart tables.
Browser accessibility-tree checks covered names, headings, regions and live
states. This is semantic and keyboard proof; no Narrator or NVDA process was
run. Broader assistive-technology and cross-browser release review remains in
stage 8.

Additional real journeys exercised URL filter reload, detail/log links,
resource-scoped events, log pause/range/wrap and a downloaded six-line redacted
fixture, topology grouping/node selection/evidence and persisted alert
acknowledgement. The copy action showed success; the automation clipboard
bridge returned no text, so byte-level clipboard proof comes from the component
regression rather than a browser clipboard read.

Initial geometry-only screenshots sometimes captured loading. They were
replaced for this evidence with data-marker waits and a fresh complete matrix.
The initial mobile toolbar clipping and apparent graph links crossing cards
were fixed and rechecked. Raw temporary probes, private runtime keys and
authentication material remain outside the repository.

Native restart, replay, backup/restore, ownership-loss and SQL plan evidence is
recorded in the [active plan](../../implementation-plan.md). These files do not
prove target capacity, a live Engine/cluster, or final 1.0 acceptance.
