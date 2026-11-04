# Selected history integration evidence

Candidate **2.0.0-alpha.6.2**, schema19. Tests/builds/services/scans run on hosted
GitHub Actions only. No stable-version or whole-stage acceptance is claimed.

Initial source `46afaabca5e083e9b086dd0641e7b9601e2d9153` in
[41807130071](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41807130071)
passes the focused Go suite and actual PostgreSQL history/migration checks.
The code job fails during client type checking: the historical citation union
assertion is stale, and the new negative fixture does not narrow array indexes.
The correction retains the expanded closed union, uses a fixed typed fixture
tuple and checks the coverage element before mutation. Hosted derivation and
formatting are applied from that run's source-only patch. The failed run remains
failed; complete fresh verification and signed packaging are still required.

[Behavior, privacy and bounds](../../v2/incident-history.md).

The next source `4930fab27a47bf1981c561e4941714b24faa328c` in
[41807685783](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41807685783)
passes focused Go, SQL, client type/response and desktop component checks. Its
full gate rejects nullable schema type arrays: the repository requires one
explicit type per branch. The correction uses closed string/integer-or-null
branches and keeps exact state/count checks. Review also updates chosen-profile
SQL accounting from 16 to 20 actual reservations, including both historical
families, and adds historical witness focus/clear checks in both languages.

Source `ac6a1910d701ca0ff291afd02ef474da3817178c` in
[41808594523](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41808594523)
passes 158 focused web checks, including historical witness focus/clear in both
languages, and the focused Go/SQL/transport gates. Full verification stops at
formatting; the hosted patch supplies formatting and the updated nullable derived
type. Further review binds even an empty proof to its exact requested family and
adds a negative substitution test. No failed receipt is reused as source acceptance.

Source `8b42489f88cabe1f368b816617cac36e88976f4f` in
[41809369390](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41809369390)
passes formatting, documentation, manifests, contract derivation, runtime-image
policy, locale parity and focused checks. Lint rejects redundant optional access
in the fixed tuple fixture and a test callback marked async without awaiting.
The correction uses definite tuple members and explicitly flushes the asynchronous
witness response. Lint remains enabled. The signed branch trial is a separate
receipt and does not accept this failed source gate.
