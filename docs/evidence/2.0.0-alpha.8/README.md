# Alpha8 — corpus review and publication floor

Version **2.0.0-alpha.8**, schema22. Immutable tag `v2.0.0-alpha.8` identifies
source `ec866cad060a711a21a1f706ca5ef25cec92443d`.

- [Source verification 42601603143](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42601603143): all eight jobs pass, including complete source/build/tests, real PostgreSQL, concurrency, desktop/offline, pinned processor, providers and deployment.
- [Signed package/security trial 42601658556](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42601658556): package, strict scans and signing pass; branch publication correctly skips.
- [Exact-tag verification 42605047683](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42605047683): owning live receipt for the immutable tag; source success alone does not substitute for this result.
- [Signed publication 42605047651](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42605047651): owning live receipt for package/publication. [Public release location](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v2.0.0-alpha.8); publication is not independent public installation or runtime acceptance.

The tag and publication links above expose their own current workflow results;
their outcomes must be read independently of source/trial success. Follow-up
documentation commits also require their own complete hosted gate. Tags are
immutable and do not move to follow-up documentation sources.

[Source processor receipt](source-processor.json) preserves the passed bilingual
application/protocol checks and owned cleanup; synthetic functional evidence
does not establish independent semantic quality, holdout custody or capacity.

Entry documentation source `7a39e7d6eea4dc384c010f71ff2c87261fa8d936` passes
all eight [jobs](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42595387984).
Published alpha7.3 has its own [immutable receipts](../2.0.0-alpha.7.3/README.md).

Current changes validate actual review files, exact revision/digest/counts and
reviewer sets, reject final-newline identities and linked/non-regular files,
bound file reads, and require release-ready corpus evidence before v2 non-alpha
publication. Negative tests cover missing/mismatched/oversized/invalid-encoding
receipts, reviewer substitution, opened path boundaries and early publication.
Alpha packaging remains possible without claiming corpus acceptance.

Review loop: **8 → fix → 9/10 within this corpus-checker/publication-floor code
boundary**, after the complete source gate and signed security/package trial.
The missing receipt read and non-alpha publication connection are corrected;
47 release-policy tests and 16 corpus tests pass, alongside full existing gates.
This does not score whole-version semantic quality. No local execution occurs.

The seed remains 12 paired scenarios / 24 questions / 18 attacks with pending
review and profile evaluation. No review receipt, reviewer identity, pilot or
holdout result is fabricated. This increment does not complete stage8 or stable2.
[Remaining roadmap and receipt schema](../../v2/stage8-readiness.md).

## Attempt trail

- Source `62a04f0`, [42600815264](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42600815264), and source `cc1e8ed`, [42600963503](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42600963503): superseded and automatically cancelled by later source pushes; neither is a passing receipt.
- Source `75c639322142aa91ebc3e80c69aa3d99d9c355f8`, [42601045344](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42601045344): superseded/cancelled after its code job fails formatting in four script files. Before that failure, 47 release-policy tests, 16 corpus tests, 38 focused client tests and 175 focused web tests pass, along with domain and selected real PostgreSQL checks. Six deployment/provider/tool jobs pass. This is not complete source acceptance. The hosted formatting artifact is applied without running a local formatter.
- Signed trial [42601216218](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42601216218) passes for `75c6393`; it does not certify the later formatting correction. Corrected source `ec866ca` subsequently passes its own complete eight-job source gate and signed trial, linked above, before tagging.

Disposable structural fixtures exercise both passing and failing release paths.
They exist only inside test-owned temporary directories and are not curated
ground truth, independent review, a sealed acceptance set or new corpus counts.
