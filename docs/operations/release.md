# Release artifacts and verification

[فارسی](release.fa.md) · [Stage procedure](../releasing.md) · [Current evidence](../implementation-plan.md)

The `release` workflow builds candidate artifacts on manual dispatch and publishes
them on a matching `vVERSION` tag. Its definition is not proof that a release has
passed. Record the exact workflow run, source commit and verified artifacts in
the evidence plan after execution. A failed scan, signing step or verification
keeps publication closed.

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

The checked asset set contains exactly 37 files. Five native archives and the
application image archive retain six separate archive SBOM attestations. The
database adds three reports; its upstream image is pulled by digest, never
repacked or published as a Stackplore database archive.

| Asset | Contents and limit |
| --- | --- |
| `stackplore_VERSION_OS_ARCH.tar.gz` | Server with embedded UI, storage and operator tools, license/notices. Builds for Linux amd64/arm64, Windows amd64 and macOS amd64/arm64. Cross-compilation is not native execution evidence for every platform. |
| `stackplore_VERSION_image.tar.gz` | Docker-save archive of the non-root Linux amd64 application image. No registry is implicitly published. |
| `stackplore-VERSION.tgz` | Helm chart; external PostgreSQL and runtime secrets remain operator inputs. |
| `stackplore_VERSION_source.tar.gz` | Exact tracked source from the commit, including docs, lockfiles and deployment profiles. |
| `*.spdx.json` | SPDX JSON inventory for each native archive and image; `source.spdx.json` includes source dependency metadata, including the web lockfile. Binary analysis alone cannot inventory bundled web dependencies completely. |
| `dependency-scan.json`, `image-scan.json`, `*.scan.json`, `secret-scan.json` | Passed source dependency/image/native-binary vulnerability and Git history secret scan receipts. Findings can change with the vulnerability database. |
| `database-image-scan.json`, `database.spdx.json` | HIGH/CRITICAL vulnerability scan and Alpine package SPDX inventory of the exact pinned upstream PostgreSQL Linux amd64 image. The OS scanner may not detect its compiled PostgreSQL engine; the separate runtime probe checks that engine. |
| `database-image.json` | Approved upstream reference/index digest, resolved Linux amd64 platform manifest digest, Docker configuration digest and exact `postgres --version` output. This is a builder verification receipt, not an upstream origin attestation. |
| `release.json`, `image-id.txt` | Exact commit, ref, run, build targets/tool versions and local image configuration digest. The image ID is not an OCI registry manifest digest. |
| `native-smoke.json` | Sanitized receipt from executing the extracted Linux amd64 archive with separate disposable PostgreSQL databases: embedded UI, health/readiness, guest reads and denied private routes, first administrator/session, exact version, disabled operation gates and graceful shutdown. Storage/operator invocation checks cover missing-argument rejection; backup/restore remains a separate recovery gate. |
| `SHA256SUMS` | Hashes of payloads, SBOMs, metadata and passed scan receipts. |
| `provenance.sigstore.json`, `*-sbom.sigstore.json` | Signed provenance covering all pre-signing assets, including `SHA256SUMS`, plus separate archive SBOM attestations. Bundles do not hash themselves; their signatures establish integrity. |

Source dependency scans include development and build dependencies. Workflow
actions use reviewed commit pins. Scanner downloads use fixed versions
and upstream SHA-256 values before extraction. Trivy rejects all HIGH/CRITICAL
dependency and image findings, including unfixed findings; there is no automatic
exception or `ignore-unfixed`. Gitleaks scans full checked-out Git history and
redacts matches. Its checked-in ignore list contains only exact historical
commit/path/rule/line fingerprints from synthetic credential-redaction tests;
new findings still fail. Do not add a blanket test-directory exclusion.

The canonical database pin is `deploy/postgres-image.json`. Packaging pulls its
exact approved reference for Linux amd64, verifies the pulled image's upstream
digest and platform, and runs only `postgres --version` with no network, a
read-only filesystem, user `70:70`, all capabilities dropped and
`no-new-privileges`. The compiled engine version must exactly match the policy
(currently `17.11`). Both its Alpine OS result and Alpine package URLs must be
present; an empty successful scan is rejected. The scan must identify the same
image configuration as the runtime probe. The SPDX document must describe its matching container
root, record that image's configuration digest, and link its Alpine OS and package
inventory to the exact package URLs in the vulnerability report. All three database reports are covered
by `SHA256SUMS` and signed build provenance. There is no extra database archive
SBOM attestation and no claim that build provenance establishes upstream image
origin. Publication rechecks the database reports and policy after transfer.

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
other native targets or the image archive. Verify the matching `.spdx.json` using
the provenance bundle or its entry in the signed checksum manifest.

Only after verification, extract native tools or load the image with
`gzip -dc stackplore_VERSION_image.tar.gz | docker load`. Configure and deploy
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
