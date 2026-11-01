# Smart Assistant — alpha.5 candidate evidence

Development target **2.0.0-alpha.5**, schema **18**. No alpha.5 acceptance or tag
is claimed here. All execution is on Actions; the workstation only reads/edits
files and performs lightweight Git. Desktop takes priority.

- Source `6775404e7cc32574aacdc5e12c750562b3395d80`, run `41293766740`:
  [failed full source formatting gate](internal-format-failed-ci.json). Seven
  other jobs pass, including the prior ordinary real processor/desktop journey.
  This failure does not accept the new investigation internals. A bounded source
  formatting patch was generated on Actions and reviewed/applied as file edits.
- Source `72a1ecc20da68bf87012c26d703fbd89ee7ba4bf`, run `41294520726`:
  focused Go candidate tests pass for the reader, conversation, HTTP projection,
  inference adapter and bounded planner. The service suite fails an old assertion
  that the newly supported investigation goal must be rejected. The correction
  retains rejection of the still unsupported alert goal and adds arbitrary shell
  goal rejection; actual investigation success/revocation/consent tests remain.
  Complete source and install jobs also require the generated transport update.
  An Actions-generated derivation/format patch was applied; this failed revision
  is not relabelled as passing.

The next source still needs complete formatting/contracts/types/lint/tests/build,
race, source/security scans, actual SQL/browser new-mode reservations, eight
bilingual/role desktop journeys, package trial and immutable exact-tag gates.
[Design, catalog, stop rules and current candidate](../../v2/investigation.md).
The accepted alpha.4 chosen-local checkpoint remains immutable and independently
scoped in its [own evidence](../2.0.0-alpha.4/README.md).
