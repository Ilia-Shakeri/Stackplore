#!/usr/bin/env bash
set -euo pipefail

operation="${1:-}"
fixture="${2:-}"
[[ "$operation" = start || "$operation" = stop ]]
[[ "$fixture" = code || "$fixture" = release ]]
[[ "${GITHUB_RUN_ID:-}" =~ ^[0-9]+$ ]]
[[ "${GITHUB_RUN_ATTEMPT:-}" =~ ^[0-9]+$ ]]
container="stackplore-${fixture}-postgres-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
owner="${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}-${fixture}"

if [[ "$operation" = stop ]]; then
  if ! timeout 15s docker container inspect "$container" >/dev/null 2>&1; then exit 0; fi
  test "$(timeout 15s docker container inspect --format '{{index .Config.Labels "stackplore.test-owner"}}' "$container")" = "$owner"
  timeout 30s docker container rm --force --volumes "$container" >/dev/null
  exit 0
fi

runtime_image="$(node --input-type=module -e 'import {readFile} from "node:fs/promises"; import {validatePostgresImagePolicy} from "./scripts/runtime-image-policy.mjs"; console.log(validatePostgresImagePolicy(JSON.parse(await readFile("deploy/postgres-image.json","utf8"))).runtime_image);')"
test "${STACKPLORE_TEST_POSTGRES_IMAGE:-$runtime_image}" = "$runtime_image"
if [[ "$fixture" = code ]]; then
  timeout 600s docker build --file deploy/Dockerfile.postgres --tag "$runtime_image" .
else
  timeout 15s docker image inspect "$runtime_image" >/dev/null
fi
test "$(timeout 30s docker run --rm --network none --read-only --cap-drop ALL --cap-add SETUID --cap-add SETGID --security-opt no-new-privileges --entrypoint gosu "$runtime_image" postgres id -u)" = 70
test "$(timeout 30s docker run --rm --network none --read-only --cap-drop ALL --cap-add SETUID --cap-add SETGID --security-opt no-new-privileges --entrypoint gosu "$runtime_image" postgres id -g)" = 70
if timeout 15s docker container inspect "$container" >/dev/null 2>&1; then
  printf '%s\n' 'Owned PostgreSQL fixture name already exists.' >&2
  exit 1
fi
database=postgres
if [[ "$fixture" = code ]]; then database=stackplore_test; fi
password="$(node --input-type=module -e 'import {randomBytes} from "node:crypto"; process.stdout.write(randomBytes(24).toString("hex"));')"
database_url="postgresql://postgres:${password}@127.0.0.1:5432/${database}?sslmode=disable"
if [[ -n "${GITHUB_ENV:-}" ]]; then
  printf '::add-mask::%s\n' "$password"
  printf '::add-mask::%s\n' "$database_url"
fi
timeout 30s docker run --detach --name "$container" --label "stackplore.test-owner=$owner" \
  --publish 127.0.0.1:5432:5432 --security-opt no-new-privileges \
  --env POSTGRES_DB="$database" --env POSTGRES_USER=postgres --env POSTGRES_PASSWORD="$password" \
  --health-cmd "pg_isready -U postgres -d $database" --health-interval 2s --health-timeout 3s --health-retries 30 \
  "$runtime_image" >/dev/null
ready=false
for attempt in $(seq 1 60); do
  if [[ "$(timeout 10s docker container inspect --format '{{.State.Health.Status}}' "$container")" = healthy ]]; then
    ready=true
    break
  fi
  sleep 2
done
test "$ready" = true
expected_version="$(node -p 'require("./deploy/postgres-image.json").server_version_num')"
test "$(timeout 15s docker exec "$container" psql -U postgres -d "$database" -Atc 'SHOW server_version_num')" = "$expected_version"
container_id="$(timeout 15s docker container inspect --format '{{.Id}}' "$container")"
[[ "$container_id" =~ ^[a-f0-9]{64}$ ]]
if [[ -n "${GITHUB_ENV:-}" ]]; then
  {
    printf 'STACKPLORE_TEST_POSTGRES_CONTAINER=%s\n' "$container"
    printf 'SP_RELEASE_POSTGRES_CONTAINER=%s\n' "$container_id"
    printf 'STACKPLORE_TEST_POSTGRES_URL=%s\n' "$database_url"
    printf 'SP_BROWSER_ADMIN_URL=%s\n' "$database_url"
    printf 'SP_RELEASE_ADMIN_URL=%s\n' "$database_url"
  } >> "$GITHUB_ENV"
fi
