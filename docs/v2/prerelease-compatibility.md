# V2 prerelease compatibility

Status: **stage 0 partial proof, 2026-10-19**. This document does not accept a
prerelease, a running helper, or a new public release. The inspected baseline is
`c018cf09c20456d27dfa6daa3f74e342fe46e27e`; the published 1.0.3 binary remains
bound to its [own receipt](../evidence/1.0.3/published-release.json).

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

## Remaining consumer-chain gates

| Boundary                          | Current proof                                         | Still required before acceptance                                                                                    |
| --------------------------------- | ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| Version/metadata/manifest names   | Production code with isolated fixtures                | Full gate on the actual versioned milestone                                                                         |
| Chart packaging                   | Actual Helm package/show/template for four versions   | Actual corrected-source lint, typed rendering, API admission and installation                                       |
| Native archive prefix             | Actual inert Windows archive round trips              | Build and execute extracted application archives on the claimed platforms                                           |
| Application/database OCI archives | Policy and filename fixtures; build-script inspection | Actual build, scans, save/load, matching tag/configuration identity and database runtime/root-drop probes           |
| Hosted CI                         | Exact-tag six-job fixture                             | All six successful jobs on the exact real candidate/tag                                                             |
| Signing/publication               | Local request interception and checksum validation    | Signed trial, real provenance/SBOM verification, protected publication and immutable tag                            |
| Independent consumption           | Naming and validation fixtures                        | Download real public assets; independently verify checksums/signatures/source; perform native and database recovery |

The existing [release workflow](../../.github/workflows/release.yml),
[builder](../../scripts/release-build.sh), [native smoke gate](../../scripts/release-smoke.mjs),
and [publication gate](../../scripts/release.mjs) remain the execution path.
Their source definitions alone are not passing receipts. Keep the current
39-asset contract unless a separately reviewed change updates every producer,
validator, attestation, consumer, and runbook together.

Stage 0 remains incomplete until the required real prerelease consumer chain,
profile decisions, baseline measurements, and accepted scope decisions have
their own evidence. No alpha tag is created by these local tests.
