# V2 prerelease compatibility

Current reconciliation: **2026-10-21**. Design/brand checkpoint
**2.0.0-alpha.0** is published at immutable
`eb86ab7554db555f023c1de72c3119acf101226a`, schema **17**. Its
[public receipt](../evidence/2.0.0-alpha.0/published-release.json) closes stage 0
within its recorded scope, including independent public database execution.
The [evidence index](../evidence/2.0.0-alpha.0/README.md) separates hosted,
signature, native and archive checks. This is not acceptance of a running
helper, later checkpoints or stable 2.0.0.

The original compatibility inspection below used
`c018cf09c20456d27dfa6daa3f74e342fe46e27e`. Stable 1.0.3 remains bound to its
[own receipt](../evidence/1.0.3/published-release.json). Historical local probe
results retain their original 2026-10-19 scope and dates.

## Confirmed defect and narrow correction

The manifest and release parsers accept a prerelease suffix, but the shared
[runtime image policy](../../scripts/runtime-image-policy.mjs) accepted only
stable image tags. Since the manifest gate requires the database runtime image
to use the same product version, `2.0.0-alpha.1`, `2.0.0-beta.2`, and
`2.0.0-rc.3` could not traverse the complete chain.

The corrected policy permits canonical stable versions and only the planned
`alpha.N`, `beta.N`, and `rc.N` suffixes. Numeric components have no leading
zeroes. Arbitrary suffixes, build metadata, additional components, paths,
control characters, and trailing newlines remain rejected. PostgreSQL engine,
digest, platform, helper compiler, source identity, and exact-reference checks
remain unchanged. No version manifest changes are part of this correction.

The old policy loaded directly from the inspected Git revision rejects all
three planned suffixes. Its SHA-256 is
`640728105e31f24f135cacf59e2c8ffd1e96f5cd35b403f922f21d9f82eae425`.

## Automated regression scope

Run with the repository's pinned Node.js 24.18.0:

```powershell
node --test scripts/release.test.mjs scripts/runtime-images.test.mjs
```

Latest focused result: **35 tests passed, zero failed/skipped**, 8.963 seconds.
This is the focused source check; the complete local and hosted milestone gates
remain separate.

The suite now covers these paths for `2.0.0-alpha.1`, `2.0.0-beta.2`,
`2.0.0-rc.3`, and the `2.0.0` stable control:

- The actual manifest checker runs from isolated, complete version fixtures.
  A prerelease with a stable-only database image is rejected.
- Runtime policy and exact Compose/workflow reference validation accept the
  permitted suffixes. Noncanonical and unsafe tag variants are rejected.
- Release-note selection uses the exact prerelease section and immutable
  source links, without leaking another checkpoint's notes.
- The actual metadata, checksum, exact-tag verification, and publication code
  processes a complete synthetic 39-file release. The fixture binds 30 checksum
  entries, 31 non-signature provenance subjects, and seven archive SBOM bundle
  names. Upload names preserve every suffix.
- The release creation request starts as a draft. Prereleases set
  `prerelease: true`; the stable control sets it to `false`. Publication follows
  complete transfer. Mixed artifact versions, mismatched metadata, and a stable
  tag targeting a prerelease are rejected before release creation or upload.
- Existing missing/tampered database evidence, engine/compiler identity,
  connected SPDX graph, exact-tag job, and checksum tests stay mandatory.

Publication and hosted-job responses in these tests are intercepted local
fixtures. They do not call the public service, sign an asset, verify a signature,
build an image, or execute a released application. Synthetic scan documents
exercise validation; they are not vulnerability scan results.

The initial new fixture run exposed an escaping error in its generated request
capture hook. That harness error was fixed; no application gate was removed or
weakened. An initial chart probe also expected an unquoted image scalar; the
probe was corrected to assert the chart's actual quoted scalar.

## Actual local packaging probe

On 2026-10-19, Helm **3.17.3** packaged, inspected, and rendered the real chart
for all four versions. The probe used `helm package --version VERSION
--app-version VERSION`, followed by `helm show chart` and `helm template` with
the matching image-tag override. All four chart versions, application versions,
image references, and application-version labels matched.

| Version         | Chart archive                  | Result                            |
| --------------- | ------------------------------ | --------------------------------- |
| `2.0.0-alpha.1` | `stackplore-2.0.0-alpha.1.tgz` | Package, metadata and render pass |
| `2.0.0-beta.2`  | `stackplore-2.0.0-beta.2.tgz`  | Package, metadata and render pass |
| `2.0.0-rc.3`    | `stackplore-2.0.0-rc.3.tgz`    | Package, metadata and render pass |
| `2.0.0`         | `stackplore-2.0.0.tgz`         | Package, metadata and render pass |

The current chart's checked-in image default remains the current product
version. This CLI-override probe does not prove a future committed milestone's
version alignment or Kubernetes API admission.

Windows bsdtar **3.8.9** also created and listed four inert archive fixtures.
Each retained exactly `stackplore_VERSION_windows_amd64/` and its single
`fixture.txt` member. This proves suffix handling in those archive names and
member prefixes, not the Linux release builder's GNU tar behavior, executable
contents, native application startup, or reproducible archive hashes.

## Actual alpha.0 consumer chain

The local tests above did not create a release. Subsequent exact-source main CI
[39258522963](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39258522963)
and tag CI [39262174884](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174884)
each pass all six jobs. Signed main trial `39258604051` passed, followed by
release run [39262174841](https://github.com/Ilia-Shakeri/Stackplore/actions/runs/39262174841)
and public prerelease [417357745](https://github.com/Ilia-Shakeri/Stackplore/releases/tag/v2.0.0-alpha.0).

| Boundary                        | Actual alpha.0 proof                                                                                                                              | Limit or remaining gate                                                                   |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Version/metadata/manifest names | Full local gate and exact main/tag hosted gates pass on the versioned checkpoint                                                                  | No later alpha/beta/rc/stable source inherits this result                                 |
| Chart packaging                 | Exact-source hosted chart/deployment and external-TLS Helm installation jobs pass                                                                 | Customer ingress, TLS and PostgreSQL require their own operator checks                    |
| Native archives                 | Signed Linux amd64 startup proof; public Windows archive is byte-identical to the actual schema-17 recovery/replay/ownership-tested trial         | Other platform outputs are cross-built/scanned                                            |
| Application OCI archive         | Public archive load and exact image identity pass; five layers and their exact derived legacy documents validated                                 | No application container was started; two earlier consumer failures remain failed         |
| Database OCI archive            | Independent public load/identity, PostgreSQL 17.11/helper/root-drop, isolated bounded read-only SQL and six-container cleanup/absence checks pass | The original consumer failure and both create-only diagnostics remain separately recorded |
| Hosted CI                       | Six successful jobs on exact main and tag source                                                                                                  | Later documentation/preparation commits need their own hosted checks                      |
| Signing/publication             | Signed trial and protected public prerelease pass                                                                                                 | Publication alone does not prove execution                                                |
| Independent trust checks        | All 39 public files, 30 checksum entries, 31 provenance subjects and 7 SBOM attestations verify                                                   | Actual archive execution keeps its own result and scope                                   |

The existing [release workflow](../../.github/workflows/release.yml),
[builder](../../scripts/release-build.sh), [native smoke gate](../../scripts/release-smoke.mjs),
and [publication gate](../../scripts/release.mjs) remain the execution path.
Their source definitions alone are not passing receipts. Keep the current
39-asset contract unless a separately reviewed change updates every producer,
validator, attestation, consumer, and runbook together.

The [public receipt](../evidence/2.0.0-alpha.0/published-release.json) binds the
passing public database execution, not the create-only diagnostics, to the
immutable source. The earlier `created_container_isolation` failure retains its
unclassified precise subphase. Separate stopped-container probes reproduced the
daemon's exact prefixed capability pair. The consumer now validates either
complete exact known pair, rejecting mixed forms, duplicates and extra rights;
28 pure tests pass. This correction does not change product source or erase the
failed attempt. Stage 0 is accepted only within its recorded scope.
The [separate preparation status](preparation-status.md) covers unaccepted
draft PR 1 at `e660e16bd0b3c56f0096bb43b5688195135892b1`. No compatible processor
is running and automatic review rejected its candidate launch. Stages **1–10**
remain incomplete.

The local checkout also retains two modified existing files and untracked
preparation after cleanup was review-rejected. Migration **18** runs on normal
startup even with the capability disabled. Use a verified alpha.0 release or
clean exact-source export for schema 17; preparation requires a fresh disposable
schema-18 test database. The local future-version fixtures above do not accept
those prepared files or any future checkpoint.

## فارسی

در ۲۰۲۶-۱۰-۲۱، وضعیت با رسیدها تطبیق شد؛ تاریخ واقعی UTC رسیدهای قبلی عوض
نشد. `alpha.0` روی منبع ثابت `eb86ab7` با طرح **۱۷** منتشر شده. شش کار
main و شش کار تگ، بستهٔ امضاشده و بررسی مستقل **۳۹ فایل، ۳۰ checksum، ۳۱
موضوع منشأ و ۷ گواهی SBOM** پاس شدند. بارگیری و هویت تصویر برنامه پاس؛
کانتینر برنامه اجرا نشد. اجرای آرشیو عمومی پایگاه مستقل پاس شد: هویت،
PostgreSQL 17.11، افت دسترسی و UID سرور ۷۰، SQL محدود و پاک‌سازی و نبود شش
کانتینر. شکست `created_container_isolation` نتیجهٔ قبلی خود را نگه می‌دارد و
زیرگام دقیقش هنوز طبقه‌بندی نشده. دو بررسیِ فقط‌ساخت، برنامه اجرا نکردند؛
شاهد اجرای پایگاه، تلاش نهایی جداست. گام صفر فقط در حد رسید پذیرفته شد.

تست نام‌های alpha/beta/rc یا بستهٔ مصنوعی، انتشار نسخه‌های بعد نیست. PR 1
فقط آماده‌سازی نپذیرفتهٔ گام‌های ۱ تا ۳ است؛ گام‌های **۱ تا ۱۰ کامل نیستند**.
پردازندهٔ سازگار در حال اجرا نیست و راه‌اندازی نامزد در بررسی خودکار رد شد.
درخت محلی هم پاک نیست؛ مهاجرت **۱۸** حتی با قابلیت خاموش اجرا شود. برای
alpha.0 از بستهٔ تأییدشده یا خروجی پاکِ منبع دقیق استفاده شود؛ آماده‌سازی فقط
پایگاه تازهٔ دورریختنی با طرح ۱۸ داشته باشد.
