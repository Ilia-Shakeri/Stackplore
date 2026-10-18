import assert from 'node:assert/strict';
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { pathToFileURL } from 'node:url';
import {
  checkRuntimeImages,
  RUNTIME_IMAGE_REFERENCE_COUNTS,
  validateRuntimeImageReferences,
} from './check-runtime-images.mjs';
import {
  POSTGRES_IMAGE_POLICY_PATH,
  validatePostgresImagePolicy,
} from './runtime-image-policy.mjs';

const policy = (version = '17.11') => ({
  schema_version: 1,
  image: `postgres:${version}-alpine@sha256:${'a'.repeat(64)}`,
  version,
  server_version_num: 170_000 + Number(version.split('.')[1]),
  platform: 'linux/amd64',
  minimum_native_tools: '17.11',
});

const sources = (pin) =>
  Object.fromEntries(
    Object.entries(RUNTIME_IMAGE_REFERENCE_COUNTS).map(([file, count]) => [
      file,
      Array.from(
        { length: count },
        (_, index) => `  service${index}:\n    image: ${pin.image}\n`,
      ).join(''),
    ]),
  );

test('PostgreSQL policy accepts 17.11 and later minor versions unchanged', () => {
  for (const version of ['17.11', '17.12', '17.100']) {
    const pin = policy(version);
    assert.equal(validatePostgresImagePolicy(pin), pin);
  }
});

test('PostgreSQL policy rejects vulnerable versions, major drift, and noncanonical versions', () => {
  for (const version of ['17.6', '17.10', '16.11', '18.11', '17.011', '17.11.1', '17.11\n']) {
    assert.throws(() => validatePostgresImagePolicy(policy(version)), /policy failed: version/u);
  }
});

test('PostgreSQL policy rejects wrong fields, numeric version, image, platform, and tool floor', () => {
  const pin = policy();
  for (const patch of [
    { schema_version: 2 },
    { schema_version: '1' },
    { version: null },
    { version: { toString: () => '17.11' } },
    { server_version_num: 170_006 },
    { server_version_num: '170011' },
    { image: `postgres:17.6-alpine@sha256:${'a'.repeat(64)}` },
    { image: 'postgres:17.11-alpine' },
    { image: `postgres:17.11-alpine@sha256:${'a'.repeat(63)}` },
    { image: `postgres:17.11-alpine@sha256:${'g'.repeat(64)}` },
    { image: `${pin.image}\n` },
    { platform: 'linux/arm64' },
    { minimum_native_tools: '17.6' },
    { minimum_native_tools: '18.0' },
    { unknown_field: true },
  ]) {
    assert.throws(() => validatePostgresImagePolicy({ ...pin, ...patch }), /policy failed/u);
  }
  for (const value of [null, [], '17.11', {}]) {
    assert.throws(() => validatePostgresImagePolicy(value), /policy failed/u);
  }
  const { image: removedImage, ...missingImage } = pin;
  assert.ok(removedImage);
  assert.throws(() => validatePostgresImagePolicy(missingImage), /policy fields/u);
});

test('runtime references accept the exact fixed counts and quoted scalar forms', () => {
  const pin = policy();
  const files = sources(pin);
  files['compose.yaml'] =
    `services:\n  postgres:\n    image: '${pin.image}' # pinned\n  app:\n    image: example/app:1\n`;
  files['.github/workflows/release.yml'] = `    "image": "${pin.image}" # pinned\n`;
  assert.equal(validateRuntimeImageReferences(pin, files), pin);
});

test('every fixed runtime source rejects missing references, extra references, and drift', () => {
  const pin = policy();
  for (const file of Object.keys(RUNTIME_IMAGE_REFERENCE_COUNTS)) {
    const absent = sources(pin);
    delete absent[file];
    assert.throws(() => validateRuntimeImageReferences(pin, absent), /source missing/u);
    for (const replacement of [
      '',
      `# image: ${pin.image}\n`,
      sources(pin)[file].replace(/image: /u, '# image: '),
      `${sources(pin)[file]}image: ${pin.image}\n`,
    ]) {
      assert.throws(
        () => validateRuntimeImageReferences(pin, { ...sources(pin), [file]: replacement }),
        /reference count drift/u,
      );
    }
    for (const image of [
      'postgres:17.11-alpine',
      policy('17.6').image,
      `postgres:17.11-alpine@sha256:${'b'.repeat(64)}`,
      `docker.io/library/${pin.image}`,
    ]) {
      const drift = sources(pin);
      drift[file] = drift[file].replace(pin.image, image);
      assert.throws(() => validateRuntimeImageReferences(pin, drift), /image drift/u);
    }
  }
});

test('runtime check reads canonical policy and all required files from disk', async (context) => {
  const root = await mkdtemp(path.join(tmpdir(), 'stackplore-runtime-images-'));
  context.after(() => rm(root, { recursive: true, force: true }));
  const pin = policy();
  for (const [file, content] of Object.entries({
    [POSTGRES_IMAGE_POLICY_PATH]: JSON.stringify(pin),
    ...sources(pin),
  })) {
    const target = path.join(root, file);
    await mkdir(path.dirname(target), { recursive: true });
    await writeFile(target, content);
  }
  const rootURL = pathToFileURL(`${root}${path.sep}`);
  assert.deepEqual(await checkRuntimeImages(rootURL), pin);
  await writeFile(path.join(root, POSTGRES_IMAGE_POLICY_PATH), JSON.stringify(policy('17.6')));
  await assert.rejects(checkRuntimeImages(rootURL), /policy failed: version/u);
  await writeFile(path.join(root, POSTGRES_IMAGE_POLICY_PATH), JSON.stringify(pin));
  await rm(path.join(root, 'compose.yaml'));
  await assert.rejects(checkRuntimeImages(rootURL), { code: 'ENOENT' });
});
