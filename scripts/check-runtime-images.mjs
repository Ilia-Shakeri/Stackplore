import { readFile } from 'node:fs/promises';
import process from 'node:process';
import { fileURLToPath, URL } from 'node:url';
import path from 'node:path';
import {
  POSTGRES_IMAGE_POLICY_PATH,
  validatePostgresImagePolicy,
} from './runtime-image-policy.mjs';

export const RUNTIME_IMAGE_REFERENCE_COUNTS = Object.freeze({
  'compose.yaml': 1,
  '.github/workflows/ci.yml': 2,
  '.github/workflows/release.yml': 1,
});

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
    const references = [...source.matchAll(/^\s*(?:image|'image'|"image")\s*:\s*(.+)$/gmu)]
      .map((match) => scalarValue(match[1]))
      .filter((value) => typeof value === 'string' && /(?:^|\/)postgres(?=[:@\s]|$)/u.test(value));
    if (references.length !== expectedCount) {
      throw new Error(
        `Runtime PostgreSQL image reference count drift: ${file} (expected ${expectedCount}, found ${references.length})`,
      );
    }
    if (references.some((reference) => reference !== policy.image)) {
      throw new Error(`Runtime PostgreSQL image drift: ${file}`);
    }
  }
  return policy;
}

export async function checkRuntimeImages(root = new URL('..', import.meta.url)) {
  const read = (file) => readFile(new URL(file, root), 'utf8');
  const policy = validatePostgresImagePolicy(JSON.parse(await read(POSTGRES_IMAGE_POLICY_PATH)));
  const entries = await Promise.all(
    Object.keys(RUNTIME_IMAGE_REFERENCE_COUNTS).map(async (file) => [file, await read(file)]),
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
