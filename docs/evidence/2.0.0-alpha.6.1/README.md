# Desktop workspace preview evidence

Version: **2.0.0-alpha.6.1**, schema **19**. No stable or whole-stage acceptance.
All execution is on GitHub Actions; no workstation tests, builds or services.

Initial source `3dd2e754adc4446d8b35b5f3165248527a893530` failed hosted
[41630369873](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41630369873).
The database image policy rejected the numeric preview-patch suffix before the
database fixture could start. Container compilation also found a missing typed
guest-fixture field set and an un-narrowed final-page cursor access. Three other
jobs passed; that partial result is not source acceptance.

The correction applies the source-only formatting artifact, narrows the cursor
with `has_more`, supplies complete guest fixture metadata, and permits exactly
one additional canonical numeric prerelease segment in the image tag. Tests keep
rejecting leading zeros, empty segments, unbounded extra segments, unsafe tags
and non-project images. Package/version agreement and immutable engine digest
checks remain intact. Fresh source and signed packaging receipts are required.

The next run [41631254944](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/41631254944)
at `9ee0ef449625bbafadde477ace015ee46eb7ef1f` passed 154 focused web checks and
failed one fixture assertion: it expected an incomplete error sentence. The DOM
showed the correct error and no conversation for the foreign source. The corrected
assertion uses the actual catalog message and retains the no-input/no-admission
checks. An additional regression makes an explicit repeated search restart page
one even when its text is unchanged, including after a failed read.

[Feature guide](../../desktop-workspaces.md) and
[implementation ledger](../../v2/implementation.md) describe scope and remaining work.
