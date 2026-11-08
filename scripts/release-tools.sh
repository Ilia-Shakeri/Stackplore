#!/usr/bin/env bash
set -euo pipefail

# Release tools are reviewed by version and the upstream release asset digest.
test -n "${RUNNER_TEMP:-}"
tool_dir="$RUNNER_TEMP/stackplore-release-tools"
mkdir -p "$tool_dir"
download() {
  local url="$1" digest="$2" filename="$3" fallback="$4" size="$5"
  local transfer=(--fail --silent --show-error --location --retry 2 --retry-delay 5 --retry-max-time 180 --connect-timeout 10 --max-time 180 --max-redirs 5 --max-filesize "$size" --proto '=https' --proto-redir '=https')
  if ! curl "${transfer[@]}" "$url" --output "$tool_dir/$filename"; then
    curl "${transfer[@]}" --header 'Accept: application/octet-stream' "$fallback" --output "$tool_dir/$filename"
  fi
  test "$(stat -c '%s' "$tool_dir/$filename")" = "$size"
  printf '%s  %s\n' "$digest" "$tool_dir/$filename" | sha256sum --check --strict -
}
download https://github.com/aquasecurity/trivy/releases/download/v0.76.0/trivy_0.76.0_Linux-64bit.tar.gz e24b1c51293c1b15e4ba8d45bbc7891ba053f90278ec2698f3562c993e145fe7 trivy.tar.gz https://api.github.com/repos/aquasecurity/trivy/releases/assets/640213415 52253735
download https://github.com/gitleaks/gitleaks/releases/download/v8.30.1/gitleaks_8.30.1_linux_x64.tar.gz 551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb gitleaks.tar.gz https://api.github.com/repos/gitleaks/gitleaks/releases/assets/378332058 8230402
tar -xzf "$tool_dir/trivy.tar.gz" -C "$tool_dir" trivy
tar -xzf "$tool_dir/gitleaks.tar.gz" -C "$tool_dir" gitleaks
printf '%s\n' "$tool_dir" >> "$GITHUB_PATH"
