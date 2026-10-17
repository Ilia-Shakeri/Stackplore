#!/usr/bin/env bash
set -euo pipefail

test -n "${RUNNER_TEMP:-}"
test -n "${GITHUB_SHA:-}"
release_dir="$RUNNER_TEMP/stackplore-release"
stage_dir="$RUNNER_TEMP/stackplore-release-staging"
# Refuse stale or overlapping output instead of replacing another run's data.
test ! -e "$release_dir"
test ! -e "$stage_dir"
mkdir -m 0755 "$release_dir" "$stage_dir"
version="$(node -p 'JSON.parse(require("fs").readFileSync("package.json", "utf8")).version')"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-z0-9.-]+)?$ ]]
if test "${GITHUB_REF_TYPE:-}" = tag; then test "$GITHUB_REF_NAME" = "v$version"; fi
test "$(git rev-parse HEAD)" = "$GITHUB_SHA"
epoch="$(git show -s --format=%ct HEAD)"
export SOURCE_DATE_EPOCH="$epoch"

corepack pnpm run check:version
corepack pnpm run build:web
node scripts/embed-web-assets.mjs

for target in linux_amd64 linux_arm64 windows_amd64 darwin_amd64 darwin_arm64; do
  target_os="${target%_*}"
  target_arch="${target#*_}"
  package_name="stackplore_${version}_${target}"
  package_dir="$stage_dir/$package_name"
  mkdir "$package_dir"
  extension=''
  if test "$target_os" = windows; then extension=.exe; fi
  for command in server storage operator; do
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go -C apps/api build -trimpath -buildvcs=true -ldflags='-s -w' -o "$package_dir/stackplore-$command$extension" "./cmd/$command"
  done
  cp LICENSE THIRD_PARTY_NOTICES.md "$package_dir/"
  mkdir -p "$package_dir/apps/api" "$package_dir/apps/web/public"
  cp -R apps/api/licenses "$package_dir/apps/api/"
  cp -R apps/web/public/licenses "$package_dir/apps/web/public/"
  trivy rootfs --scanners vuln --severity HIGH,CRITICAL --exit-code 1 --format json --output "$release_dir/${package_name}.scan.json" "$package_dir"
  trivy rootfs --scanners vuln --format spdx-json --output "$release_dir/${package_name}.spdx.json" "$package_dir"
  node scripts/release-scan.mjs native "$release_dir/${package_name}.scan.json" "$release_dir/${package_name}.spdx.json" "$extension"
  tar --sort=name --mtime="@$epoch" --owner=0 --group=0 --numeric-owner -C "$stage_dir" -cf - "$package_name" | gzip -n > "$release_dir/${package_name}.tar.gz"
done

git archive --format=tar --prefix="stackplore-$version/" HEAD | gzip -n > "$release_dir/stackplore_${version}_source.tar.gz"
helm package deploy/helm/stackplore --destination "$release_dir"
docker build --file deploy/Dockerfile.api --label "org.opencontainers.image.revision=$GITHUB_SHA" --label "org.opencontainers.image.version=$version" --tag "stackplore-api:$version" .
trivy image --scanners vuln --severity HIGH,CRITICAL --exit-code 1 --format json --output "$release_dir/image-scan.json" "stackplore-api:$version"
trivy image --scanners vuln --format spdx-json --output "$release_dir/stackplore_${version}_image.spdx.json" "stackplore-api:$version"
node scripts/release-scan.mjs image "$release_dir/image-scan.json" "$release_dir/stackplore_${version}_image.spdx.json"
docker image inspect --format '{{.Id}}' "stackplore-api:$version" > "$release_dir/image-id.txt"
docker save "stackplore-api:$version" | gzip -n > "$release_dir/stackplore_${version}_image.tar.gz"
node scripts/release.mjs metadata
