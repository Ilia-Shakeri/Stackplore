# Stage 8 — evidence and release readiness

Current work is **2.0.0-alpha.8**, schema22. This is a preparation checkpoint,
not completion of stage8. Workstation activity is limited to file edits and
lightweight Git; execution belongs to GitHub Actions.

Immutable tag `v2.0.0-alpha.8` identifies `ec866ca`, which passes all eight source
jobs and signed package/security trial. Exact-tag and publication have separate
[owning live receipts](../evidence/2.0.0-alpha.8/README.md); these results do not
close the independent quality or pilot gates below.

## Verified entry point

Published alpha7.3 source `20daeeb` passes all eight source jobs, all eight
exact-tag jobs and signed public packaging. Its documentation follow-up
`7a39e7d6eea4dc384c010f71ff2c87261fa8d936` passes all eight
[hosted jobs](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/42595387984).
[Immutable feature receipts](../evidence/2.0.0-alpha.7.3/README.md).

## Smart Assistant — corpus review boundary

An accepted manifest previously supplied only a review receipt path. The checker
now reads the actual bounded UTF-8 JSON file, rejects links and non-regular
files in the repository-relative input path, and matches the receipt to the
exact corpus ID, revision and SHA-256. Authors must match; at least two distinct
reviewers must match the manifest and accept the review. Actual scenario/attack
counts must agree. A rubric revision and sealed, tuning-excluded holdout
attestation are mandatory. Unknown fields fail closed.

Receipt schema1 has exactly these fields:

| Field | Required value |
| --- | --- |
| `schema_version` | `1` |
| `corpus_id`, `revision`, `sha256` | Exact manifest identity and corpus-byte digest |
| `status` | `accepted` |
| `authors` | Exact manifest author set |
| `reviews` | Exact manifest reviewer set, each `{ reviewer, decision: "accepted" }` |
| `paired_scenarios`, `adversarial_cases` | Actual validated counts |
| `rubric_revision` | Bounded canonical identifier |
| `holdout` | `{ sealed: true, tuning_access: false }` |

The path stays `docs/evidence/v2/<name>.json`. The receipt limit is 64 KiB;
manifest limit 16 KiB; corpus limit 4 MiB. Reads use a bounded buffer and an
opened regular-file identity check. These checks reject static link escapes;
they do not make a concurrently attacker-modified checkout a trusted workspace.
Canonical identifiers, digests and review paths reject final-newline aliases.
Failures print fixed codes rather than corpus or review contents.

This schema records a review assertion. It cannot authenticate reviewer identity,
prove the review occurred, enforce custody outside the repository, or establish
semantic accuracy. Release owners must inspect the actual independent sign-off.
An accepted small seed still cannot pass the release target floor.

## Publication floor

Both `release.mjs check-ci` and `release.mjs publish` require the corpus release
check before network publication for major-version2 releases other than numeric
alpha checkpoints. This includes beta, release candidate, stable and other v2
prerelease labels. Missing, seed, incomplete or unreviewed corpus evidence blocks
publication. Alpha previews retain all exact-tag, security and artifact checks;
they do not claim reviewed corpus acceptance. Version1 publication is unchanged.
Branch packaging trials can still prepare artifacts without claiming release.

This is a necessary corpus floor, **not a complete stable eligibility system**.
Profile scoring, security outcomes, named-hardware measurements, pilot and actual
stable-asset acceptance require their own receipts and release-owner decision.

## Remaining roadmap and exit gates

| Stage | Remaining work | Acceptance evidence |
| --- | --- | --- |
| 8 — corpus | Expand distinct mechanisms from 12 paired scenarios / 18 attacks to 200 / 1,000; independent review and controlled holdout | Exact snapshot, rubric, two reviewers, no tuning leakage |
| 8 — evaluation | Three held-out passes per supported pinned profile; language/provider denominators and human review | Citation authorization 100%, grounding/abstention at least 95%, critical security 100%; preserve failures |
| 8 — resilience | Fault injection, usage reconciliation, safe telemetry inventory, hot-query review | Fresh hosted database, concurrency and lifecycle receipts; named-hardware RAM and latency bounds |
| 9 — pilot | Native/Compose/Helm, upgrade/restore, 60-minute soak, 100 lifecycle cycles | Opted-in environment, owned cleanup, recovery and pilot sign-off |
| 10 — stable | Frozen support scope, final documents, signed stable packages and independent downloaded-asset acceptance | Exact-source and artifact receipts for the actual stable release |

No independent evaluator or pilot is currently available. Preparation and hosted
regressions continue; neither missing gate is marked complete. Desktop English
and Persian journeys remain primary. See [roadmap](../roadmap-v2.md),
[evaluation](evaluation.md), [threat model](threat-model.md) and
[attempt ledger](../evidence/2.0.0-alpha.8/README.md).

## فارسی

گام فعلی ۸ است. مسیر سند تا گزارش و شاهد در آلفای ۷٫۳ منتشر شده است.
این گام، خود فایل رسید بررسی را با هش و شمار واقعی نمونه‌ها می‌سنجد و
انتشار زودرس بتا و پایدار را بدون پیش‌نیاز مجموعهٔ بررسی می‌بندد.
رسید JSON مدرک مستقل هویت یا انجام بررسی نیست. ارزیاب و پایلوت هنوز
در دسترس نیستند؛ سنجش انسانی، آزمون دوام و پذیرش بستهٔ پایدار باز هستند.
همهٔ تست‌ها و ساخت‌ها فقط روی GitHub Actions انجام می‌شوند.
