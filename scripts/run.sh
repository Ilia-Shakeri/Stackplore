#!/bin/sh

set -eu

case "${1:-}" in
  bootstrap|dev|verify|test|build|lint|typecheck|format)
    script_name="$1"
    ;;
  test-api)
    script_name="test:api"
    ;;
  test-web)
    script_name="test:web"
    ;;
  format-check)
    script_name="format:check"
    ;;
  *)
    echo "Usage: scripts/run.sh bootstrap|dev|verify|test|test-api|test-web|build|lint|typecheck|format|format-check" >&2
    exit 2
    ;;
esac

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repository_root"

if [ "$script_name" = "bootstrap" ]; then
  exec node scripts/bootstrap.mjs
fi

exec corepack pnpm run "$script_name"
