#!/usr/bin/env bash
set -euo pipefail

test "$(uname -s)" = Linux
test -n "${RUNNER_TEMP:-}"
proof_dir="$(mktemp -d "$RUNNER_TEMP/stackplore-runtime-proof.XXXXXX")"
case "$proof_dir" in "$RUNNER_TEMP"/stackplore-runtime-proof.*) ;; *) exit 1 ;; esac
trap 'rm -rf -- "$proof_dir"' EXIT
helm template runtime-proof deploy/helm/stackplore --show-only templates/api-deployment.yaml |
  awk '
    /^              umask 077$/ { capture = 1 }
    capture && /^          resources:/ { exit }
    capture { sub(/^              /, ""); print }
  ' > "$proof_dir/init.source.sh"
test -s "$proof_dir/init.source.sh"
grep --quiet --fixed-strings 'root=/runtime-volume' "$proof_dir/init.source.sh"
sed 's|root=/runtime-volume|root="${STACKPLORE_RUNTIME_TEST_ROOT:?}"|' "$proof_dir/init.source.sh" > "$proof_dir/init.sh"
bash -n "$proof_dir/init.sh"

run_init() {
  STACKPLORE_RUNTIME_TEST_ROOT="$1" /bin/sh -e "$proof_dir/init.sh"
}
new_root() {
  mkdir -m 0770 "$proof_dir/$1"
}
new_key() {
  # A fresh synthetic fixture is generated here; no application key is read.
  head -c 32 /dev/urandom > "$1"
  chmod 0660 "$1"
}
key_hash() {
  sha256sum "$1" | cut -d ' ' -f 1
}
reject_init() {
  if run_init "$1" >/dev/null 2>&1; then
    printf 'Unsafe runtime fixture accepted: %s\n' "${1##*/}" >&2
    exit 1
  fi
}

new_root fresh
run_init "$proof_dir/fresh"
test "$(stat -c %a "$proof_dir/fresh/private")" = 700
test ! -e "$proof_dir/fresh/private/.stackplore.auth-key"

new_root legacy
legacy="$proof_dir/legacy/.stackplore.auth-key"
key="$proof_dir/legacy/private/.stackplore.auth-key"
new_key "$legacy"
expected="$(key_hash "$legacy")"
run_init "$proof_dir/legacy"
test ! -e "$legacy"
test "$(key_hash "$key")" = "$expected"
test "$(stat -c %a "$key")" = 600
chmod 0770 "$proof_dir/legacy/private"
chmod 0660 "$key"
run_init "$proof_dir/legacy"
test "$(key_hash "$key")" = "$expected"
test "$(stat -c %a "$proof_dir/legacy/private")" = 700
test "$(stat -c %a "$key")" = 600

new_root duplicate
mkdir -m 0700 "$proof_dir/duplicate/private"
new_key "$proof_dir/duplicate/.stackplore.auth-key"
new_key "$proof_dir/duplicate/private/.stackplore.auth-key"
legacy_hash="$(key_hash "$proof_dir/duplicate/.stackplore.auth-key")"
private_hash="$(key_hash "$proof_dir/duplicate/private/.stackplore.auth-key")"
reject_init "$proof_dir/duplicate"
test "$(key_hash "$proof_dir/duplicate/.stackplore.auth-key")" = "$legacy_hash"
test "$(key_hash "$proof_dir/duplicate/private/.stackplore.auth-key")" = "$private_hash"

new_root symlink-directory
mkdir -m 0755 "$proof_dir/symlink-target"
ln -s "$proof_dir/symlink-target" "$proof_dir/symlink-directory/private"
reject_init "$proof_dir/symlink-directory"
test "$(stat -c %a "$proof_dir/symlink-target")" = 755

new_root symlink-key
new_key "$proof_dir/symlink-key-target"
ln -s "$proof_dir/symlink-key-target" "$proof_dir/symlink-key/.stackplore.auth-key"
reject_init "$proof_dir/symlink-key"
test -L "$proof_dir/symlink-key/.stackplore.auth-key"
test "$(stat -c %a "$proof_dir/symlink-key-target")" = 660

new_root hardlink
new_key "$proof_dir/hardlink/.stackplore.auth-key"
ln "$proof_dir/hardlink/.stackplore.auth-key" "$proof_dir/hardlink-alias"
reject_init "$proof_dir/hardlink"
test "$(stat -c %h "$proof_dir/hardlink/.stackplore.auth-key")" = 2
test ! -e "$proof_dir/hardlink/private/.stackplore.auth-key"

new_root wrong-size
printf fixture > "$proof_dir/wrong-size/.stackplore.auth-key"
reject_init "$proof_dir/wrong-size"
test ! -e "$proof_dir/wrong-size/private/.stackplore.auth-key"

printf 'Private runtime volume fresh, migration, restart and unsafe-file checks pass.\n'
