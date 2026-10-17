# Third-Party Notices

Stackplore bundles the following font packages for local, offline use. Their
licenses remain separate from the project's Apache License 2.0.

The Go service also links the runtime modules listed below. Their licenses
remain separate from the project's Apache License 2.0.

## Go runtime modules

The Go service runtime links the modules below. License copies match the
pinned module versions. Development-only transitive test tools are not shipped
in the service binary. Font notices remain below.

| Module | Version | License | Copy |
| --- | --- | --- | --- |
| `github.com/go-chi/chi/v5` | `5.3.1` | MIT | [License](apps/api/licenses/chi-LICENSE.txt) |
| `github.com/moby/moby/client` | `0.6.2` | Apache-2.0 | [Exact module license](apps/api/licenses/moby-client-LICENSE.txt), [module source](https://pkg.go.dev/github.com/moby/moby/client@v0.6.2) |
| `github.com/moby/moby/api` | `1.56.1` | Apache-2.0 | [Exact module license](apps/api/licenses/moby-api-LICENSE.txt), [module source](https://pkg.go.dev/github.com/moby/moby/api@v1.56.1) |
| `github.com/containerd/errdefs` | `1.0.0` | Apache-2.0 | [Exact module license](apps/api/licenses/containerd-errdefs-LICENSE.txt) |
| `k8s.io/api` | `0.37.1` | Apache-2.0 | [Exact module license](apps/api/licenses/kubernetes-api-LICENSE.txt) |
| `k8s.io/apimachinery` | `0.37.1` | Apache-2.0 | [Exact module license](apps/api/licenses/kubernetes-apimachinery-LICENSE.txt) |
| `k8s.io/client-go` | `0.37.1` | Apache-2.0 | [Exact module license](apps/api/licenses/kubernetes-client-go-LICENSE.txt) |
| `k8s.io/metrics` | `0.37.1` | Apache-2.0 | [Exact module license](apps/api/licenses/kubernetes-metrics-LICENSE.txt) |
| `github.com/jackc/pgx/v5` | `5.10.0` | MIT | [License](apps/api/licenses/pgx-LICENSE.txt) |
| `github.com/jackc/pgpassfile` | `1.0.0` | MIT | [License](apps/api/licenses/pgpassfile-LICENSE.txt) |
| `github.com/jackc/pgservicefile` | `5a60cdf6a761` | MIT | [License](apps/api/licenses/pgservicefile-LICENSE.txt) |
| `github.com/jackc/puddle/v2` | `2.2.2` | MIT | [License](apps/api/licenses/puddle-LICENSE.txt) |
| `golang.org/x/crypto` | `0.54.0` | BSD 3-Clause | [License](apps/api/licenses/golang-x-crypto-BSD-3-Clause.txt) |
| `golang.org/x/sys` | `0.47.0` | BSD 3-Clause | [License](apps/api/licenses/golang-x-sys-BSD-3-Clause.txt) |
| `golang.org/x/sync` | `0.22.0` | BSD 3-Clause | [License](apps/api/licenses/golang-x-sync-LICENSE.txt) |
| `golang.org/x/text` | `0.40.0` | BSD 3-Clause | [License](apps/api/licenses/golang-x-text-LICENSE.txt) |

## Complete linked Go module inventory

The [generated manifest](apps/api/licenses/modules/manifest.json) records the
union of external modules linked by `cmd/server`, `cmd/operator` and
`cmd/storage` for Linux/amd64 and Windows/amd64 with `CGO_ENABLED=0`. It includes
transitive modules, exact versions, module checksums, target membership and
repository-relative license material paths with SHA-256 hashes. The files are
byte-for-byte copies of the pinned upstream module's LICENSE, NOTICE, COPYING,
COPYRIGHT and PATENTS materials, including nested materials and LICENSES
directories. Existing direct-dependency copies above remain available.

The manifest also records the exact root LICENSE from the pinned Go `1.26.5`
distribution for the linked standard library, copied as
[Go distribution license](apps/api/licenses/modules/go-toolchain/LICENSE).
This inventory covers external Go modules for those command builds and that
Go distribution license. Tests, JavaScript packages, fonts and container images
have separate scopes; the manifest does not claim to inventory them.

After bootstrap, `npm run check:go-notices` verifies both target graphs, pinned
versions, upstream material sets and every copied file hash without downloading
modules. `npm run derive:go-notices` regenerates the manifest and exact copies
from the populated local module cache. Keep generated material outside text
formatting so upstream license bytes remain intact.

Git also preserves these copied legal files without line-ending conversion or
whitespace cleanup. Upstream trailing spaces and blank lines are intentional
copies. The generated manifest keeps the repository's normal text checks;
its SHA-256 entries identify the exact legal file bytes.

## Inter

- Package: `@fontsource-variable/inter` version `5.3.0`
- Package source: <https://www.npmjs.com/package/@fontsource-variable/inter>
- Font source: <https://github.com/rsms/inter>
- Copyright: Copyright 2016 The Inter Project Authors
- License: SIL Open Font License, Version 1.1
- License copy:
  [apps/web/public/licenses/fonts/inter-OFL-1.1.txt](apps/web/public/licenses/fonts/inter-OFL-1.1.txt)

## Vazirmatn

- Package: `@fontsource-variable/vazirmatn` version `5.3.0`
- Package source:
  <https://www.npmjs.com/package/@fontsource-variable/vazirmatn>
- Font source: <https://github.com/rastikerdar/vazirmatn>
- Copyright: Copyright 2015 The Vazirmatn Project Authors
- License: SIL Open Font License, Version 1.1
- License copy:
  [apps/web/public/licenses/fonts/vazirmatn-OFL-1.1.txt](apps/web/public/licenses/fonts/vazirmatn-OFL-1.1.txt)
