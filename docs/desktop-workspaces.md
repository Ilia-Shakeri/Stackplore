# Desktop workspaces — 2.0.0-alpha.6.1

This development preview separates tasks into pages. It does not complete
version-2 incident-history integration, later roadmap stages or stable acceptance.
Schema 19 is unchanged from the preceding development source. Published alpha.5
remains immutable at schema 18; use the documented pre-upgrade backup procedure.

| Page | Purpose | Bounds and behavior |
| --- | --- | --- |
| `/overview` | Current-source totals, health and next workspace | One source card; no metric-history, event or inventory read |
| `/monitoring` | One trend at a time | `monitor=cpu`, `memory`, `restarts`, `receive` or `send`; validated URL values, existing time range and freshness states |
| `/monitoring?monitor=dashboard` | Existing personalized dashboard | Same five panel IDs and per-account browser storage; no server or cross-device synchronization |
| `/insights` | Environments, loaded consumers and recent events | Existing bounded observations, at most five consumers per metric and ten event previews; full lists remain on their own pages |
| `/assistant` | Smart Assistant | Authenticated source and resource choice, ten resources per page and at most ten cursor pages before refining search |
| `/help` | Task guides and support | One `help_topic` at a time: `start`, `monitoring`, `assistant` or `security`; guide body makes no backend request |

The sidebar, page navigation and command palette expose the new destinations.
Links retain the selected source. Metric and help-topic URLs support direct
entry and normal browser navigation. Monitoring keeps the current observation
clock, refresh coalescing, explicit partial/unavailable states and data tables.
The optional My dashboard view can be longer because it deliberately displays
several selected panels; it is not the default monitoring or home page.

## Smart Assistant

Choose a source, find a resource by name, then choose one resource. The page
fetches that resource again and verifies its source and identity before mounting
the existing conversation panel. The panel starts expanded; entering the page,
choosing a resource or typing a question never submits a question automatically.
The existing explicit submission, goal/window choice, log consent, report and
fresh citation controls remain in the shared resource panel. The resource-detail
entry point remains available.

Changing resource, source or leaving unmounts the old view and aborts outstanding
reads. The question and volatile conversation are not copied into URLs or saved
settings. A late read for an old selection cannot mount its conversation. A
foreign-source response is rejected. Resource choices replace a page instead of
appending an endless list; cursor history is bounded to ten pages. The guest demo
shows an explicit unavailable notice and makes no private assistant request.
An authenticated installation still needs an operator-enabled profile.

## Help and support

The built-in guide covers getting started, metric selection and browser-local
dashboards, resource-scoped investigation, and account/privacy basics. It links
to the relevant workspaces. Reporting a problem opens the public repository issue
tracker in a separate tab with opener/referrer isolation. The interface asks
users to remove credentials and private data before sharing; no support address
or response-time promise is invented.

## Validation

All execution is hosted. Required checks include direct URLs in both languages,
short overview height and absence of hidden history requests, one default metric,
retained dashboard editing, help-topic selection, bounded resource pages,
foreign/late resource rejection, guest restrictions, and no automatic admission.
The real browser fixture types a temporary question, changes selection, returns
and verifies a cleared input with zero submissions. Existing real processor,
report/citation, ownership, migration, offline, provider and packaging gates remain.
The established eight-route timing receipt keeps its existing scope; the new
workspace flows add functional checks, not unmeasured capacity claims.

Fresh source, tag and package receipts must identify the exact revision before a
preview release is accepted. A green interface preview is not stable-2.0 acceptance.
