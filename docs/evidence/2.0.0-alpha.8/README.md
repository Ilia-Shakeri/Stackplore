# Alpha8 — corpus review and publication floor

Version **2.0.0-alpha.8**, schema22. Candidate implementation; source, signed
trial, immutable tag and publication await their own hosted receipts.

Entry documentation source `7a39e7d6eea4dc384c010f71ff2c87261fa8d936` passes
all eight [jobs](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42595387984).
Published alpha7.3 has its own [immutable receipts](../2.0.0-alpha.7.3/README.md).

Current changes validate actual review files, exact revision/digest/counts and
reviewer sets, reject final-newline identities and linked/non-regular files,
bound file reads, and require release-ready corpus evidence before v2 non-alpha
publication. Negative tests cover missing/mismatched/oversized/invalid-encoding
receipts, reviewer substitution, opened path boundaries and early publication.
Alpha packaging remains possible without claiming corpus acceptance.

Review loop: **8/10 pending hosted verification**. The missing receipt read and
non-alpha publication connection are corrected in source; complete tests and
security/package checks are not inferred from edits. No local execution occurs.

The seed remains 12 paired scenarios / 24 questions / 18 attacks with pending
review and profile evaluation. No review receipt, reviewer identity, pilot or
holdout result is fabricated. This increment does not complete stage8 or stable2.
[Remaining roadmap and receipt schema](../../v2/stage8-readiness.md).
