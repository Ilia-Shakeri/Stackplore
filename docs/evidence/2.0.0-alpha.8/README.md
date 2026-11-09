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

## Attempt trail

- Source `62a04f0`, [42600815264](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42600815264), and source `cc1e8ed`, [42600963503](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42600963503): superseded and automatically cancelled by later source pushes; neither is a passing receipt.
- Source `75c639322142aa91ebc3e80c69aa3d99d9c355f8`, [42601045344](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42601045344): code job fails formatting in four script files. Before that failure, 47 release-policy tests, 16 corpus tests, 38 focused client tests and 175 focused web tests pass, along with domain and selected real PostgreSQL checks. Six deployment/provider/tool jobs pass; the processor job has its own result. This is not complete source acceptance. The hosted formatting artifact is applied without running a local formatter.
- Signed trial [42601216218](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42601216218) belongs to `75c6393`; its result cannot certify the later formatting correction. Fresh source and package receipts remain required before tagging.

Disposable structural fixtures exercise both passing and failing release paths.
They exist only inside test-owned temporary directories and are not curated
ground truth, independent review, a sealed acceptance set or new corpus counts.
