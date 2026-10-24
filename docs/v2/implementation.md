# Version 2 implementation ledger

Current correction, **2026-10-24**: the previous source passed seven hosted
jobs but its signed trial failed the dependency gate. Exact package patches
now pass an independent audit with zero findings; fresh full-source and signed
release gates remain open. [Current correction evidence](../evidence/2.0.0-alpha.1/README.md#dependency-correction--2026-10-24).


Started: **2026-10-19**. Authorized scope: implement the complete
[roadmap](../roadmap-v2.md) ([فارسی](../roadmap-v2.fa.md)), test each stage,
update documents, version, commit and push. Stable 1.0.3 remains independently
accepted at `656581cf05d4774358d5644a6e1af3c69a210f81`, schema 17.
Baseline documentation source: `c018cf09c20456d27dfa6daa3f74e342fe46e27e`.

Current reconciliation: **2026-10-21**. The published design/brand checkpoint is
immutable `v2.0.0-alpha.0` at `eb86ab7554db555f023c1de72c3119acf101226a`,
schema **17**. Main CI
[39258522963](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39258522963)
and tag CI [39262174884](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174884)
each pass six jobs. Signed trial `39258604051` and release run
[39262174841](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174841)
pass; prerelease [417357745](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v2.0.0-alpha.0)
is public. [Public acceptance](../evidence/2.0.0-alpha.0/published-release.json)
closes stage 0 within its recorded design/brand and artifact-consumption scope.
The [evidence index](../evidence/2.0.0-alpha.0/README.md) separates
local, hosted, native and public archive results. Its immutable receipt dates
remain their actual UTC dates; this reconciliation does not redate them.


The previous alpha.1 source `f1ae80391aca646fca89fed7a5ecf79271999c97`
passed seven hosted jobs and fourteen actual browser cases, but signed trial
`39592142674` failed on CVE-2026-111943 in the Go 1.26.9 standard library
embedded in gosu. No alpha.1 tag was created. The Go 1.26.10 source correction
requires fresh local, hosted and artifact acceptance;
[retained failure and toolchain proof](../evidence/2.0.0-alpha.1/README.md#go-security-correction--2026-10-21). Earlier compiler results do
not accept the correction.

## Stage status

| Stage | Target                 | State                        | Acceptance still required                                  |
| ----- | ---------------------- | ---------------------------- | ---------------------------------------------------------- |
| 0     | `2.0.0-alpha.0`        | Accepted in recorded scope   | No helper runtime, customer installation or capacity claim |
| 1     | `2.0.0-alpha.1`        | Security correction pending  | Fresh full gate, seven hosted jobs, strict scans and publication  |
| 2     | `2.0.0-alpha.2`        | Unaccepted draft preparation | Complete lifecycle, quota, lease and recovery acceptance   |
| 3     | `2.0.0-alpha.3`        | Unaccepted draft preparation | Complete shared privacy and evidence-tool acceptance       |
| 4     | `2.0.0-alpha.4`        | Not delivered                | Bilingual grounded conversation                            |
| 5     | `2.0.0-alpha.5`        | Not delivered                | Bounded investigation loop                                 |
| 6     | `2.0.0-alpha.6`        | Not delivered                | Incident analysis                                          |
| 7     | `2.0.0-alpha.7`        | Not delivered                | Approved corpus and safe history                           |
| 8     | `2.0.0-alpha.8`        | Not delivered                | Held-out quality, attack and failure gates                 |
| 9     | `2.0.0-beta.1`         | Not delivered                | Actual pilot, upgrade and deployment                       |
| 10    | `2.0.0-rc.N` → `2.0.0` | Not delivered                | Independent candidate and stable acceptance                |

Parallel code preparation does not bypass a predecessor's exit gate. No helper
capability is enabled or claimed shipped by this ledger. Stage 0 now has a
versioned `alpha.0` checkpoint because the maintainer requested a version at
every stage; this supersedes the proposal's unversioned planning checkpoint.
Existing tags remain immutable. New tags wait for their own passing gates.

Stages **1–10 are not complete**. The
[separate preparation status](preparation-status.md) binds unaccepted draft
[PR 1](https://github.com/Ilia-Shakeri/Stackplore/pull/1) to
`e660e16bd0b3c56f0096bb43b5688195135892b1`; it is not part of alpha.0.
Its scoped real SQL/use-case checks and six-job draft verification are evidence
for that preparation, not acceptance of newer source or later stages. The
new local profile proof below is separately bound to its gateway/tool files.

**Local checkout boundary:** automatic review rejected cleanup with only
`blocked by policy`. Modified existing files and untracked preparation,
including migration 18, remain. Normal startup applies schema **18** even with
the capability disabled. This checkout is not clean schema-17 alpha.0 source.
Use the verified release or a clean exact-source export for alpha.0; retained
preparation uses a fresh disposable schema-18 database only. Returning an older
binary requires the verified backup/restore procedure.

## Stage 0 decisions and acceptance

- Accepted design: ADRs 0014–0016, additive SPEC section 15 and architecture.
- Evaluation: [protocol](evaluation.md), [threat model](threat-model.md), and
  [prerelease compatibility](prerelease-compatibility.md). Seed fixtures do not
  meet release corpus size, independent review or live quality gates.
- Source boundary: read-only fixed tools, one Go runtime, PostgreSQL, owner-bound
  runs, volatile text, default-off/no egress and unchanged v1 installation roles.
- Checkpoint labels: `alpha.0` for decisions, `alpha.1`–`alpha.8` for implementation,
  `beta.N`, `rc.N`, then actual stable `2.0.0`; each accepted artifact uses its
  own source, checksums, scans, signatures and independent consumer receipts.

## Historical local commands and honest outcomes

Pinned Windows tools for the historical runs below: Node 24.18.0, Go 1.26.9; use `npm.cmd`. Native PostgreSQL
17 fixture is a new owned loopback instance on port 55474 in a dedicated D-drive
scratch directory. Existing database files and unrelated `artifacts/` are
untouched. Heavy temporary output uses the D drive; initial C-drive free space
was 3.7 GiB, available memory about 2 GiB of 15.6 GiB.

| Attempt           | Command or procedure                                                                 | Outcome and scope                                                                                                                                           |
| ----------------- | ------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Baseline A        | Pinned `npm.cmd run verify`; `.tmp/v2-stage0-baseline-verify.log`                    | Failed at offline Go notice graph; inherited module cache was unavailable. No test weakened.                                                                |
| Cache preflight   | `node scripts/check-go-notices.mjs` with explicit existing user Go module cache      | Pass: 72 modules, 106 exact upstream files, Linux/amd64 and Windows/amd64.                                                                                  |
| Baseline B        | Pinned `npm.cmd run verify`; `.tmp/v2-stage0-baseline-verify-b.log`                  | Failed formatting while new corpus files were still being written. Not a clean baseline or runtime failure.                                                 |
| Prerelease policy | `node --test scripts/release.test.mjs scripts/runtime-images.test.mjs`               | 35/35 pass for this slice; actual Helm package/show/render and inert archive round trips cover four checkpoint forms. Real OCI/publication remains pending. |
| Database fixture  | Native `initdb`, `pg_ctl` and `psql` against a new owned loopback directory/database | Started; SQL/migration acceptance remains a separate gate. Existing fixture was not restarted.                                                              |
| Automatic review  | Attempt to start the previous PostgreSQL fixture                                     | Rejected: only reason returned was `blocked by policy`. Switched to a fresh, owned test database.                                                           |
| Automatic review  | Attempt to launch a newly downloaded local processor binary                          | Rejected: only reason returned was `blocked by policy`. No alternate launch path used; real profile acceptance remains pending.                             |

## Subsequent verification and supplied artwork

- Frozen baseline C used staged tree `5078b8f1255755a5c1b816920a978da2e0390842`.
  All Go/native SQL tests, 35 release checks, 11 corpus checks, 40 client tests
  and 530 web tests pass. Build correctly rejects dependency links outside the
  exported workspace; overall `npm.cmd run verify` fails. Preserve
  `.tmp/v2-stage0-frozen-verify-c.log`; do not call this attempt green.
- A new independent export installed all 342 packages offline from the existing
  C-drive store, with zero downloads. The initial default D-drive store attempt
  lacked its own cache and failed. Main workspace links were restored before
  testing the current interface. Removing old exported dependency junctions was
  rejected by automatic review with only `blocked by policy`; they were left
  untouched and no alternate deletion was attempted.
- The earlier `.tmp/v2-probe-installed-model.mjs` attempt recorded two synthetic
  schema/locale probes: English 7,818 ms and Persian 7,518 ms, both pass.
  No raw answer was retained; pre-dispatch counting and grounding remain
  unproven. This historical probe does not establish a currently running
  compatible processor or satisfy stage 1.
- The maintainer supplied three logos and requested placement, compression and
  a matching theme before continuing. [Brand inventory](../brand.md) records
  twelve derivatives totaling 525,385 bytes, an 85.5% reduction. Original source
  files remain unchanged. `node scripts/check-brand-assets.mjs` checks exact
  digests, dimensions and total bytes; it passes.
- Current branding checks: 57 focused tests across three files, scoped lint
  and strict UI/web/browser types pass. Corrected an insufficient info tint
  instead of reducing the 4.5 text-contrast floor. Real browser proof remains a
  separate gate. The public cache stays limited to four paths; its logo is now
  a PNG. This change is not proof of helper runtime completion.
- Review score for the branding slice was 8/10: the server still gave the new
  offline icon a private cache policy, the worker test supplied its former MIME
  type, and the offline page ignored an explicit theme preference. Fixed the
  exact public icon allowlist, kept all other logo files outside worker storage,
  resolved the selected theme in the existing hashed offline script, and added
  opposite-device theme checks without expanding the four cached paths.
- Frozen attempt D uses tree `2cd2bafe646ea5a41be056ea82825ff74c12dd4b`; it
  predates those review corrections. Its database variables were named
  incorrectly, so skipped SQL integrations cannot count as database proof.
  Keep `.tmp/v2-stage0-frozen-verify-d.log`; the corrected full gate uses the
  repository's actual `STACKPLORE_TEST_POSTGRES_URL` and native-tool opt-in.
  Attempt D ends with 536/537 web tests passing and the expected obsolete
  icon-MIME fixture failure; the complete command fails. The corrected slice
  passes 33 offline/branding tests plus the complete HTTP asset-header boundary
  test. These focused results do not replace the next full gate or browser run.
- The versioned [browser artifact consumer](browser-evidence.md) preserves the
  exact fourteen-case journey, 80-read workload and eight locale/size/theme
  profiles while checking all 32 screenshots, including the new About views.
  Archive fixtures test the consumer only; actual browser and hosted source
  identity still require their own receipts.

## Follow-up browser fixture failure

The docs-only follow-up source `3b80e3840d13d75e89f62c2d0ba2b81f2d540383`
failed [CI 39562252755](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39562252755)
only in the production browser step. Full verification, Go concurrency and the
five other jobs passed; **13/14** browser cases passed. The
[diagnostic receipt](../evidence/2.0.0-alpha.0/follow-up-browser-failure.json)
binds the exact failed source and artifact without returning private payloads.
Guest case 9 failed at the final online reload with setup **429**. Repeated
navigation bursts shared the unchanged default 40-token / 20-request-per-second
peer bucket. The fixture correction applies its existing 2,500 ms refill before
the controlled reload and both online resumes, preserves every assertion and
adds an explicit no-HTTP-failure check. No test retry, rate increase, timeout
increase or skip was added. Fresh exact-source CI and browser acceptance must
prove this correction; the failed attempt remains failed.

پیگیری `3b80e38` در مرورگر شکست؛ **۱۳ از ۱۴** تست پاس. گیت کامل، رقابت و
پنج کار دیگر پاس بودند. setup در بازگشت آنلاینِ تست آفلاین **۴۲۹** داد.
سه مکثِ موجود ۲۵۰۰ms و شرط نبود خطای HTTP افزوده شد؛ نرخ و همهٔ شرط‌ها حفظ.
رسید شکست باقی؛ منبع اصلاح‌شده، پذیرش تازهٔ CI و مرورگر لازم دارد.

## Current acceptance and unresolved gates

Corrected frozen attempt E, tree
`ee35c24874c0b81e596dcb1c790c78f358329e1b`, passes the full local gate:
544 web tests, 40 client tests, 35 release checks, 11 corpus tests, real native
PostgreSQL 17.11 and recovery tools, and production build. Fourteen actual
browser cases also pass without retry; 32 screenshots and 64 route measurements
cover both languages, themes and viewport sizes. Both explicit offline themes
override opposite device preferences. The original logos retain their hashes.
[Local candidate evidence](../evidence/2.0.0-alpha.0/README.md) records exact scope,
log digests, unchanged workload, visual review and the post-gate documentation/
consumer-diagnostic wording changes. Branding review is 9/10 for this slice;
new runtime functionality is not part of this candidate.

The actual committed source subsequently passed main and tag CI and signed
packaging. Independent verification checked all **39 public files**, **30
checksum entries**, **31 provenance subjects** and **7 SBOM attestations**.
The extracted Windows amd64 trial archive passed schema-17 backup/restore,
alert replay, recovery, revocation and ownership-loss checks; the actual public
archive is byte-identical. The public application archive passed bounded
load and image-identity verification, with **no container started**. Its two
earlier consumer failures remain failed: unreferenced blob before load, then
configuration-versus-manifest identity after load. Only the consumer helper
was corrected; these results do not prove application container startup.

Independent public database archive execution now passes: 13 layers and 13
exact derived legacy documents, 34 entries, PostgreSQL 17.11, `gosu` 1.19 built
with Go 1.26.9, actual root drop to UID/GID 70 and server UID 70, network-none/read-only
containers with bounded tmpfs, create/insert/select/rollback SQL, and six owned
containers removed with absence verified. Its final helper passes 28 pure tests.
The original `created_container_isolation` failure remains failed, with its
exact subphase unclassified. Two separate create-only diagnostics identified
the daemon's `CAP_SETUID`/`CAP_SETGID` representation; neither started a program.
The consumer correction accepts only either complete exact known pair and
rejects mixed, duplicate or additional permissions. Product code was unchanged.
The [public receipt](../evidence/2.0.0-alpha.0/published-release.json) binds all
these results; historical failure dates and statuses remain intact.

### Final workstation archive consumer commands

These are the exact final local command forms, with shared paths factored for
brevity. Both returned **exit 0**. The audited consumers are ignored workstation
harnesses, recorded by SHA-256 in the public receipt; they are not bundled
application utilities or files available to installed users. The committed
[operator verification runbook](../operations/release.md)
([فارسی](../operations/release.fa.md)) is a separate procedure.

```powershell
$consumerRoot = '/mnt/c/Users/Dear-OTCamp-User/Desktop/Stackplore/.tools/stage9-review'
$proofRoot = '/mnt/d/Stackplore-v2-20261019'
$publicAssets = "$proofRoot/alpha0-public-39262174841"
$signatureReceipt = "$proofRoot/alpha0-public-signatures-39262174841.json"
$releaseRevision = 'eb86ab7554db555f023c1de72c3119acf101226a'

wsl.exe -d Ubuntu-24.04 --exec /usr/bin/python3 `
  "$consumerRoot/consume-app-image-v2.py" `
  --assets $publicAssets --verification-receipt $signatureReceipt `
  --destination "$proofRoot/alpha0-public-app-image-proof-c" `
  --source-ref refs/tags/v2.0.0-alpha.0 --revision $releaseRevision `
  --version 2.0.0-alpha.0 --root $proofRoot

wsl.exe -d Ubuntu-24.04 --exec /usr/bin/python3 `
  "$consumerRoot/database_archive_v2.py" `
  --assets $publicAssets --verification-receipt $signatureReceipt `
  --receipt "$proofRoot/alpha0-public-database-39262174841-b3fdf2.json" `
  --kind published --revision $releaseRevision --version 2.0.0-alpha.0 `
  --root $proofRoot --docker /usr/bin/docker
```

The application command proves archive load and exact identity only:
`container_started=false`. The database command proves the isolated execution
and cleanup described above. Neither command accepts the mixed local product
checkout or a real processor profile. The failed application/database attempts
remain in the public receipt; the successful commands do not relabel them.

At the earlier checkpoint, no compatible local processor was running.
Stage 1's real-profile test was explicitly skipped. That candidate's downloaded
bytes and checksum were verified, but automatic review rejected launch with
only `blocked by policy`. No alternate launch was attempted. Independent
preparation and its synthetic/real-SQL checks do not close this predecessor
gate. The required pinned identity, exact pre-dispatch counting, bounded
execution and held-out quality still need their own real proof.

The initial 53–88 person-day estimate remains provisional. Those earlier checks
did not establish a capacity or quality claim. The new controlled protocol proof
below still does not prove grounding quality or maximum-load capacity. Stages
1–10 and stable 2.0.0 remain incomplete until their own gates pass.

## Stage 1 — authorized local installation and protocol proof

On 2026-10-21 the maintainer explicitly requested installation and startup of
the required local service. A new invocation of the reviewed native operator
tool succeeded. This is a distinct authorized attempt; the earlier automatic
review rejection remains recorded with its original outcome.

The official CPU archive pin and all 51 installed files match. The official
Qwen3-0.6B Q8_0 model at revision `23749fefcc72300e3a2ad315e1317431b06b590a`
was downloaded and its exact 639,446,688 bytes, full SHA-256 and GGUF header
verified. The native tool then started one loopback-only process with fixed
20,480 context, one slot, two CPU threads, quantized KV, no offload, raw logging,
prompt caching, writable properties, execution tools or dynamic model loading.
Actual build, model alias, template, context and slot pins passed before ready.
The operator-owned service remains running for later local checks; its private
ownership receipt and model/runtime binaries stay outside the repository.

| Exact procedure                                                                                                                           | Actual result                                                                                      |
| ----------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `pwsh -NoProfile -File scripts/local-processor.test.ps1`                                                                                  | 125 pure checks pass; these checks launch nothing                                                  |
| `scripts/local-processor.ps1 -Action Check` with the private verified inputs                                                              | Archive, all 51 members and model pass                                                             |
| `scripts/local-processor.ps1 -Action Start` with a fresh private receipt                                                                  | Ready; exact ownership and literal loopback listener pass                                          |
| `go -C apps/api test ./internal/provider/inference -run '^TestPinnedLocalRuntimeIntegration$' -count=1 -v` with explicit pinned variables | Both cases actually run, zero skips; English 104/27 tokens in 2,358 ms, Persian 132/40 in 5,229 ms |
| `go -C apps/api test ./internal/inference ./internal/provider/inference -count=1 -cover`                                                  | Pass; 95.4% / 90.2% statement coverage, no branch-coverage claim                                   |

The [minimal receipt](../evidence/2.0.0-alpha.1/gateway-profile.json) binds the
ten tested source files and exact pins without exposing raw prompts, answers,
paths or credentials. The [candidate index](../evidence/2.0.0-alpha.1/README.md)
records limitations. These literal no-evidence probes establish protocol and
locale obedience only, not incident reasoning or held-out quality.

Candidate version **2.0.0-alpha.1** adds the neutral gateway, local adapter,
operator tool and bilingual profile documentation. Public dispatch is absent;
schema 18 and all unaccepted runtime/privacy preparation stay outside this
candidate. A clean staged-source export is required for its full local gate.
The CI definition adds a separate Windows pure-tool job, bringing the candidate
to seven jobs; historical six-job receipts remain unchanged. Full verification,
exact-source hosted acceptance and publication are still separate gates.

Scoped review loop: **8 → fix → 9/10**. Independent review closed duplicate and
case-fold JSON fields, UTF-8/depth limits, context/slot identity, opened-file
size races, exact browser-origin policy and process/receipt ownership. It found
no remaining verified defect in this slice. That score does not accept later
runtime, privacy, quality, deployment or stable-release stages.

### Full local gate A — failed, retained

The isolated candidate tree `0ef4c3b42e00be5609b68cb3c06c825ca19b2233`
failed `npm.cmd run verify` in PostgreSQL tests. The unchanged representative
query-plan fixture exhausted its 90-second context while seeding measured
events for 20,000 rows. The package subsequently reached its ten-minute deadline
while an upgrade subtest's cleanup was active. This did not reach client/web
tests or the production build. The
[failed receipt](../evidence/2.0.0-alpha.1/local-gate-failure-a.json) keeps the
exact log digest, source and original deadlines.

For diagnosis, the ownership-checked stop command stopped only the new local
processor; its listener absence was verified. Other team compilation and SQL
tests were paused. The exact unchanged focused performance test then passed in
**6.47 seconds**, including both 1,000/20,000-row workloads and every original
sample, index and page assertion. Resource contention is plausible but these
combined host changes do not establish a unique cause. No product code,
fixture, workload or timeout was changed. Full gate B is a separate attempt on
the same frozen tree; the first failure remains failed. The processor is to be
restarted with a new ownership receipt after the isolated heavy gate, before
final local handoff. Concurrent capacity remains a later measured gate.

### Full local gate B — failed, retained

The same frozen tree passed real PostgreSQL/native recovery checks (PostgreSQL
package 479.238 s; some unchanged Go packages cached) and 40 client tests. The
web suite failed with 492/495 passing cases, three existing lookup/test-bound
failures and a threads-worker startup timeout before a further file ran. The
production build was not reached. The
[separate receipt](../evidence/2.0.0-alpha.1/local-gate-failure-b.json) binds
the failed log and the subsequent unchanged three-file focused probe: **80/80
pass, 59.09 s**, without coverage. No test, timeout or product implementation was
changed. That focused pass does not replace the failed complete command.

Source review also found the release checker still listed six required jobs.
It now requires the exact seven completed successful jobs with no duplicates or
extras. Focused release/runtime-image checks pass **48/48**, no skips; format and
lint pass. The recorded result was transcribed from tool output, with no raw
stdout file retained. A new full gate includes these release-check source edits;
hosted and publication acceptance remain open.

### Full local gate C — passed

`npm.cmd run verify` passes, exit **0**, on isolated staged tree
`695c705a379ae46f3eb570009efab10654ae6f85`. All **45 changed staged files**
matched exported bytes, and migration 18 was absent. It includes the corrected
seven-job publication gate. Results: **48 release/runtime-image, 11 corpus,
40 client and 544 web tests pass**, plus notices, locale parity, contract/type
drift, lint, strict types, coverage floors and production web/embed/Go build.
Web duration **227.98 s**, branch coverage **80.33%**; build **14.87 s**.

Go/PostgreSQL/native results include cached success. The actual unchanged
PostgreSQL package passed in gate B in **479.238 s**; C does not claim a second
SQL execution. Both earlier complete commands remain failed. The
[receipt](../evidence/2.0.0-alpha.1/local-gate.json) records log digest,
source/cache/isolation boundaries and limitations. Subsequent receipt/document
edits need the exact commit's hosted full gate. The processor stays stopped for
heavy isolated tests and must be restarted with a fresh owned receipt before
final local handoff. Stage 1 still awaits hosted/tag/publication acceptance;
answer quality and later stages remain open.

## فارسی — وضعیت اجرا

طرح و لوگوی گام صفر در `alpha.0` منتشر شد؛ منبع ثابت `eb86ab7` و طرح پایگاه
**۱۷**. شش کار main و شش کار تگ، بستهٔ امضاشده و بررسی مستقل **۳۹ فایل، ۳۰
checksum، ۳۱ موضوع منشأ و ۷ گواهی SBOM** پاس شدند. آرشیو برنامه بار شد و
هویت تصویر سنجیده شد؛ کانتینر برنامه اجرا نشد. آرشیو عمومی پایگاه هم مستقل
پاس شد: PostgreSQL 17.11، افت دسترسی و UID سرور ۷۰، SQL محدود با rollback،
شبکهٔ خاموش، ریشهٔ فقط‌خواندنی، tmpfs محدود و پاک‌سازی و نبود شش کانتینر.
شکست `created_container_isolation` حفظ شد؛ زیرگام دقیق آن هنوز طبقه‌بندی نشده.
دو بررسیِ فقط‌ساخت، قالب CAP در خروجی میزبان را روشن کردند؛ برنامه اجرا
نکردند. اصلاح فقط در مصرف‌کننده بود. گام صفر در همین حد شاهد پذیرفته شد.
رسیدهای UTC قبلی تاریخ خود را نگه می‌دارند؛ این متن در ۲۰۲۶-۱۰-۲۱ تطبیق شد.

پیش‌نویس PR 1 فقط آماده‌سازی نپذیرفتهٔ گام‌های ۱ تا ۳ است؛ در alpha.0 نیست.
فایل‌های تغییرکرده و فایل‌های ثبت‌نشده در درخت محلی ماندند؛ حذف در بررسی
خودکار رد شد. اجرای این درخت، مهاجرت **۱۸** را حتی با قابلیت خاموش اعمال
کند. برای alpha.0 بستهٔ تأییدشده یا خروجی پاکِ منبع دقیق لازم؛ آماده‌سازی فقط
روی پایگاه تازهٔ دورریختنی با طرح ۱۸ باشد.

در تلاش پیشین، پردازنده نبود و راه‌اندازی در بررسی خودکار رد شد؛ دلیل فقط
`blocked by policy`. آن نتیجه حفظ شد. در ۲۰۲۶-۱۰-۲۱، کاربر نصب و شروع را صریح
خواست؛ تلاش تازهٔ راه‌انداز سنجیده پاس شد. هر ۵۱ فایل و مدل، هویت و پورت
محلی درست؛ دو درخواست بستهٔ واقعی فارسی/انگلیسی نیز پاس، بی‌skip.
نامزد `alpha.1` به گیت کامل منبع پاک، هفت کار CI و پذیرش نسخه نیاز دارد.
گام‌های **۱ تا ۱۰**، کیفیت پاسخ و نسخهٔ پایدار ۲ هنوز بسته نشده‌اند.
