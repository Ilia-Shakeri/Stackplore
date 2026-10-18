# Release artifacts and verification

[فارسی](release.fa.md) · [Stage procedure](../releasing.md) · [Current evidence](../implementation-plan.md)

The `release` workflow builds candidate artifacts on manual dispatch and publishes
them on a matching `vVERSION` tag. Its definition is not proof that a release has
passed. Record the exact workflow run, source commit and verified artifacts in
the evidence plan after execution. A failed scan, signing step or verification
keeps publication closed.

Release notes link project guides and runbooks to their immutable source
revision, rather than a moving branch. Keep those source-bound links when
copying notes or consuming an older archive. Current release acceptance is
tracked in [1.0.2 evidence](../evidence/1.0.2/README.md). Immutable `v1.0.2`
at `376ed03` passes exact main/tag CI and signed trial with independent actual
archive recovery; signed publication and independent public/archive verification
also pass in the [published receipt](../evidence/1.0.2/published-release.json). The first
tag/blocked-release attempts remain failed, and the unchanged-tag retry passed.
The later main-only harness/report-retention fix is separate from released source.

## Maintainer sequence

1. Complete the milestone's local gate with real PostgreSQL and all named browser,
   provider, deployment and recovery checks. Review the diff and changelog; keep
   local tools, private data and unknown images outside the commit.
2. Commit and push the candidate. Check `verify` on that exact commit. Optionally
   dispatch `release` on the candidate branch to test packaging/scans/signing.
   Dispatch retains trial artifacts for 30 days; it does not create a release.
3. Create an annotated matching tag on the checked commit, then push that tag.
   Never move an existing published tag. The workflow checks that the tag still
   resolves to its own source commit, waits at most 30 minutes for its exact-tag
   `verify` run, and requires successful `code`, `deploy`, `compose-install`,
   `helm-install`, `docker-provider` and `kubernetes-provider` jobs.
4. Inspect both the tag verification and release workflow. Package work uses
   read-only source permissions plus short-lived signing identity. Only the
   separate publish job can create releases. Publication checks exact-tag
   verification again and verifies all payload checksums after artifact transfer.
5. Independently download the public assets and use the checks below. Save the
   successful verification result with the commit/run identity. Versions below
   `1.0.0` and versions containing a prerelease suffix are marked prerelease.

The publish script creates a draft, uploads every asset, and publishes only when
all uploads succeed. It refuses an existing release. A failed upload leaves a
draft for inspection; it never silently replaces assets or deletes a release.
Inspect that draft and the failure before any manual recovery. If source changes
are required after a published tag, make a new version and tag.

## Assets and scope

The checked asset set contains exactly 39 files. Five native archives and two
image archives retain seven separate archive SBOM attestations. `SHA256SUMS`
contains 30 payload entries; signed provenance covers those payloads plus the
manifest. The PostgreSQL runtime is a derivative of its pinned upstream image,
with the same engine and a rebuilt `gosu` helper.

| Asset                                                                        | Contents and limit                                                                                                                                                                                                                                                                                                                                                                                        |
| ---------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `stackplore_VERSION_OS_ARCH.tar.gz`                                          | Server with embedded UI, storage and operator tools, license/notices. Builds for Linux amd64/arm64, Windows amd64 and macOS amd64/arm64. Cross-compilation is not native execution evidence for every platform.                                                                                                                                                                                           |
| `stackplore_VERSION_image.tar.gz`                                            | Docker-save archive of the non-root Linux amd64 application image. No registry is implicitly published.                                                                                                                                                                                                                                                                                                   |
| `stackplore_VERSION_postgres_image.tar.gz`                                   | Docker-save archive of `stackplore-postgres:VERSION` for Linux amd64. It retains the pinned PostgreSQL engine and replaces `gosu` 1.19 with the same pinned source built using Go 1.26.9; helper, dependency and Go license materials are included. This is a Stackplore runtime derivative.                                                                                                              |
| `stackplore-VERSION.tgz`                                                     | Helm chart; external PostgreSQL and runtime secrets remain operator inputs.                                                                                                                                                                                                                                                                                                                               |
| `stackplore_VERSION_source.tar.gz`                                           | Exact tracked source from the commit, including docs, lockfiles and deployment profiles.                                                                                                                                                                                                                                                                                                                  |
| `*.spdx.json`                                                                | SPDX JSON inventory for each native archive and image; `source.spdx.json` includes source dependency metadata, including the web lockfile. Binary analysis alone cannot inventory bundled web dependencies completely.                                                                                                                                                                                    |
| `dependency-scan.json`, `image-scan.json`, `*.scan.json`, `secret-scan.json` | Passed source dependency/image/native-binary vulnerability and Git history secret scan receipts. Findings can change with the vulnerability database.                                                                                                                                                                                                                                                     |
| `database-image-scan.json`, `database.spdx.json`                             | HIGH/CRITICAL vulnerability scan and SPDX inventory of the built PostgreSQL runtime, including Alpine packages and `gosu` Go dependencies. The OS scanner may not detect the compiled PostgreSQL engine; a separate runtime probe checks that engine.                                                                                                                                                     |
| `database-image.json`                                                        | Upstream reference/index/platform/configuration digests, actual runtime image/configuration digest, pinned helper source/compiler build inputs, and isolated `postgres --version` and `gosu --version` results. This is a builder verification receipt, not an upstream origin attestation.                                                                                                               |
| `release.json`, `image-id.txt`                                               | Exact commit, ref, run, five native build targets, both image archive identities/platforms, tool versions and local application image configuration digest. Image IDs are not OCI registry manifest digests.                                                                                                                                                                                              |
| `native-smoke.json`                                                          | Sanitized receipt from executing the extracted Linux amd64 archive with separate disposable PostgreSQL databases: embedded UI, health/readiness, guest reads and denied private routes, first administrator/session, exact version, disabled operation gates and graceful shutdown. Storage/operator invocation checks cover missing-argument rejection; backup/restore remains a separate recovery gate. |
| `SHA256SUMS`                                                                 | Hashes of payloads, SBOMs, metadata and passed scan receipts.                                                                                                                                                                                                                                                                                                                                             |
| `provenance.sigstore.json`, `*-sbom.sigstore.json`                           | Signed provenance covering all pre-signing assets, including `SHA256SUMS`, plus separate archive SBOM attestations. Bundles do not hash themselves; their signatures establish integrity.                                                                                                                                                                                                                 |

Source dependency scans include development and build dependencies. Workflow
actions use reviewed commit pins. Scanner downloads use fixed versions
and upstream SHA-256 values before extraction. Trivy rejects all HIGH/CRITICAL
dependency and image findings, including unfixed findings; there is no automatic
exception or `ignore-unfixed`. Gitleaks scans full checked-out Git history and
redacts matches. Its checked-in ignore list contains only exact historical
commit/path/rule/line fingerprints from synthetic credential-redaction tests;
new findings still fail. Do not add a blanket test-directory exclusion.

The canonical database policy is `deploy/postgres-image.json`. Packaging verifies
the pinned upstream Linux amd64 image, then builds `deploy/Dockerfile.postgres`
with its checked source and compiler pins. It probes the resulting runtime with
only `postgres --version` and `gosu --version`: no network, a read-only filesystem,
user `70:70`, all capabilities dropped and `no-new-privileges`. The engine must
report exactly `17.11`; `gosu` must report `1.19` built using Go `1.26.9` for
Linux amd64. Trivy must separately inventory that Go standard library in the
actual `gosu` executable. Its Alpine OS result, Alpine package URLs and Go helper
inventory must all be present; empty successful scans are rejected.

The scan and SPDX must identify the runtime configuration that was probed. The
SPDX document must describe that container root, record its configuration digest,
and link its Alpine and `gosu` application package graphs to the exact package
URLs in the vulnerability report. All three database reports and the runtime
archive are covered by checksums and signed build provenance. The database
archive has its own `postgres-image-sbom.sigstore.json` attestation. Build
provenance establishes the derivative build, not upstream image origin.
Publication rechecks the reports and policy after transfer. Native archive smoke
uses an owned disposable fixture started from this already scanned runtime, then
removes that fixture before signing.

If the package gate fails, `stackplore-database-diagnostics-COMMIT` retains only
the available `database-image.json`, `database-image-scan.json` and
`database.spdx.json` for seven days. These are failure diagnostics, outside the
39 release assets and unsigned. Missing
reports are allowed in this diagnostic upload; the failed gate still blocks
release publication. No inspection files, probe files, environment or staging
directory are uploaded.

The native tar metadata is normalized to the source commit time. Tool downloads,
the vulnerability database, container package repositories and the hosted runner
still need network access. No bit-for-bit reproducibility or absence of every
possible vulnerability is claimed. Scanner/signing/network unavailability fails
the release; it does not count as a passing check.

## Verify before extraction or loading

Use a trusted installed GitHub CLI that supports artifact attestation verification.
Get the expected tag and commit from the reviewed source, independently of the
downloaded `release.json`. Download the selected archive, `SHA256SUMS` and
`provenance.sigstore.json` from that exact release. Replace the values below with
that tag, its full commit and the selected archive filename.

```sh
tag=vVERSION
commit=FULL_REVIEWED_COMMIT_SHA
artifact=stackplore_VERSION_linux_amd64.tar.gz
identity="https://github.com/Ilia-Shakeri/Stackplore/.github/workflows/release.yml@refs/tags/$tag"
gh attestation verify SHA256SUMS \
  --repo Ilia-Shakeri/Stackplore --bundle provenance.sigstore.json \
  --cert-identity "$identity" --source-ref "refs/tags/$tag" \
  --source-digest "$commit" --deny-self-hosted-runners
gh attestation verify "$artifact" \
  --repo Ilia-Shakeri/Stackplore --bundle provenance.sigstore.json \
  --cert-identity "$identity" --source-ref "refs/tags/$tag" \
  --source-digest "$commit" --deny-self-hosted-runners
sha256sum --check --ignore-missing SHA256SUMS
```

Require both attestation commands and the selected file's checksum to succeed.
If downloading all assets, use `sha256sum --check --strict SHA256SUMS` instead.
On Windows, use `Get-FileHash -Algorithm SHA256` for the selected archive and
compare its complete value with the signed manifest; run the same `gh` checks.
Reject a branch-dispatch identity when installing a tagged release.

To verify the SBOM attestation for that Linux archive, also download
`linux-amd64-sbom.sigstore.json` and run the same identity/source checks with
`--bundle linux-amd64-sbom.sigstore.json` and
`--predicate-type https://spdx.dev/Document/v2.3`. Choose the matching bundle for
other native targets or either image archive. Use `postgres-image-sbom.sigstore.json`
and `database.spdx.json` for the PostgreSQL runtime archive. Verify the matching `.spdx.json` using
the provenance bundle or its entry in the signed checksum manifest.

Only after verification, extract native tools or load the image with
`gzip -dc stackplore_VERSION_image.tar.gz | docker load`. Verify and load the
PostgreSQL runtime archive separately with
`gzip -dc stackplore_VERSION_postgres_image.tar.gz | docker load`. Configure and deploy
through the [deployment runbook](deployment.md), then repeat readiness and
application-specific checks. Artifact verification establishes origin/integrity;
it does not establish target-host suitability or production health.

## Signing support and references

Signing requires GitHub artifact attestations and short-lived OIDC identity.
Public repositories support this on current plans; private/internal repositories
require an eligible Enterprise Cloud plan. There is no unsigned fallback.
Offline verification also needs a separately trusted root; saving a bundle alone
does not supply that trust root.

When the verifier cannot retrieve its trust metadata, restore access to the
official TUF endpoints through the operator's normal trusted network path. Do
not bypass signature or identity checks. A connected trusted machine can run
`gh attestation trusted-root > trusted_root.jsonl`; transfer that independently
verified root and add `--custom-trusted-root trusted_root.jsonl` to the same
verification commands. A root bundled only with the unverified download is not
an independent trust source. See the [trusted-root command](https://cli.github.com/manual/gh_attestation_trusted-root).

The implementation follows the primary [artifact-attestation guide](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations),
the [verification command reference](https://cli.github.com/manual/gh_attestation_verify),
the [Trivy SBOM reference](https://trivy.dev/docs/latest/supply-chain/sbom/), and the
[Gitleaks CLI reference](https://github.com/gitleaks/gitleaks#usage).
