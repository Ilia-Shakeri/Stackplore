#!/bin/sh

set -eu

case "${1:-}" in
  bootstrap|dev|verify|test|build|lint|typecheck|format)
    script_name="$1"
    ;;
  check-contracts)
    script_name="check:contracts"
    ;;
  check-locales)
    script_name="check:locales"
    ;;
  derive-api-client)
    script_name="derive:api-client"
    ;;
  test-api)
    script_name="test:api"
    ;;
  test-client)
    script_name="test:client"
    ;;
  test-web)
    script_name="test:web"
    ;;
  format-check)
    script_name="format:check"
    ;;
  *)
    echo "Usage: scripts/run.sh bootstrap|dev|verify|test|test-api|test-client|test-web|build|check-locales|check-contracts|derive-api-client|lint|typecheck|format|format-check" >&2
    exit 2
    ;;
esac

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repository_root"

if [ "$script_name" = "bootstrap" ]; then
  exec node scripts/bootstrap.mjs
fi

exec corepack pnpm run "$script_name"
