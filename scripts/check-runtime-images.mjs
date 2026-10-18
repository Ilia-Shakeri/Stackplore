import { readFile } from 'node:fs/promises';
import process from 'node:process';
import { fileURLToPath, URL } from 'node:url';
import path from 'node:path';
import {
  POSTGRES_IMAGE_POLICY_PATH,
  GOSU_BUILD_IMAGE,
  validatePostgresImagePolicy,
} from './runtime-image-policy.mjs';

export const RUNTIME_IMAGE_REFERENCE_COUNTS = Object.freeze({
  'compose.yaml': 1,
  '.github/workflows/ci.yml': 2,
  '.github/workflows/release.yml': 1,
});
export const POSTGRES_DOCKERFILE_PATH = 'deploy/Dockerfile.postgres';

const scalarValue = (raw) => {
  const value = raw.trim();
  if (value.startsWith("'")) return value.match(/^'((?:[^']|'')*)'(?:\s+#.*)?$/u)?.[1];
  if (value.startsWith('"')) return value.match(/^"([^"\\]*)"(?:\s+#.*)?$/u)?.[1];
  return value.replace(/\s+#.*$/u, '').trim();
};

export function validateRuntimeImageReferences(policy, sources) {
  validatePostgresImagePolicy(policy);
  for (const [file, expectedCount] of Object.entries(RUNTIME_IMAGE_REFERENCE_COUNTS)) {
    const source = sources[file];
    if (typeof source !== 'string') throw new Error(`Runtime image source missing: ${file}`);
    const references = [
      ...source.matchAll(
        /^\s*(?:image|'image'|"image"|STACKPLORE_TEST_POSTGRES_IMAGE)\s*:\s*(.+)$/gmu,
      ),
    ]
      .map((match) => scalarValue(match[1]))
      .filter(
        (value) =>
          typeof value === 'string' &&
          /(?:^|\/)(?:stackplore-postgres|postgres)(?=[:@\s]|$)/u.test(value),
      );
    if (references.length !== expectedCount) {
      throw new Error(
        `Runtime PostgreSQL image reference count drift: ${file} (expected ${expectedCount}, found ${references.length})`,
      );
    }
    if (references.some((reference) => reference !== policy.runtime_image)) {
      throw new Error(`Runtime PostgreSQL image drift: ${file}`);
    }
  }
  const dockerfile = sources[POSTGRES_DOCKERFILE_PATH];
  if (typeof dockerfile !== 'string')
    throw new Error(`Runtime image source missing: ${POSTGRES_DOCKERFILE_PATH}`);
  const from = [...dockerfile.matchAll(/^FROM\s+(\S+)(?:\s+AS\s+\S+)?\s*$/gmu)].map(
    (match) => match[1],
  );
  const sourceLine = `ADD --checksum=sha256:${policy.gosu_source_sha256} https://codeload.github.com/tianon/gosu/tar.gz/${policy.gosu_source_commit} /tmp/gosu.tar.gz`;
  const lines = dockerfile.split(/\r?\n/u);
  if (
    from.length !== 2 ||
    from[0] !== GOSU_BUILD_IMAGE ||
    from[1] !== policy.image ||
    !lines.includes(sourceLine) ||
    !dockerfile.includes(`test "$(go env GOVERSION)" = 'go${policy.gosu_build_go}'`) ||
    !dockerfile.includes(`grep -qx 'const Version = "${policy.gosu_version}"' version.go`) ||
    !dockerfile.includes('CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/gosu .') ||
    !lines.includes('COPY --from=gosu-build --chmod=0755 /out/gosu /usr/local/bin/gosu') ||
    !lines.includes(
      'COPY --from=gosu-build /out/licenses/ /usr/share/licenses/stackplore-postgres/',
    ) ||
    /^\s*(?:ENTRYPOINT|CMD|USER)\s/gimu.test(dockerfile) ||
    /^#\s*syntax\s*=/gimu.test(dockerfile)
  )
    throw new Error('Runtime PostgreSQL Dockerfile build-input drift');
  return policy;
}

export async function checkRuntimeImages(root = new URL('..', import.meta.url)) {
  const read = (file) => readFile(new URL(file, root), 'utf8');
  const policy = validatePostgresImagePolicy(JSON.parse(await read(POSTGRES_IMAGE_POLICY_PATH)));
  const entries = await Promise.all(
    [...Object.keys(RUNTIME_IMAGE_REFERENCE_COUNTS), POSTGRES_DOCKERFILE_PATH].map(async (file) => [
      file,
      await read(file),
    ]),
  );
  return validateRuntimeImageReferences(policy, Object.fromEntries(entries));
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const policy = await checkRuntimeImages();
    process.stdout.write(`Runtime PostgreSQL ${policy.version} images match the pinned policy.\n`);
  } catch (error) {
    process.stderr.write(
      `${error instanceof Error ? error.message : 'Runtime image check failed'}\n`,
    );
    process.exitCode = 1;
  }
}
