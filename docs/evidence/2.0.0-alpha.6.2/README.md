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
