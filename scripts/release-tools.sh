#!/usr/bin/env bash
set -euo pipefail

# Release tools are reviewed by version and the upstream release asset digest.
test -n "${RUNNER_TEMP:-}"
tool_dir="$RUNNER_TEMP/stackplore-release-tools"
mkdir -p "$tool_dir"
download() {
  local url="$1" digest="$2" filename="$3"
  curl --fail --silent --show-error --location --retry 2 --max-time 180 "$url" --output "$tool_dir/$filename"
  printf '%s  %s\n' "$digest" "$tool_dir/$filename" | sha256sum --check --strict -
}
download https://github.com/aquasecurity/trivy/releases/download/v0.76.0/trivy_0.76.0_Linux-64bit.tar.gz e24b1c51293c1b15e4ba8d45bbc7891ba053f90278ec2698f3562c993e145fe7 trivy.tar.gz
download https://github.com/gitleaks/gitleaks/releases/download/v8.30.1/gitleaks_8.30.1_linux_x64.tar.gz 551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb gitleaks.tar.gz
tar -xzf "$tool_dir/trivy.tar.gz" -C "$tool_dir" trivy
tar -xzf "$tool_dir/gitleaks.tar.gz" -C "$tool_dir" gitleaks
printf '%s\n' "$tool_dir" >> "$GITHUB_PATH"
