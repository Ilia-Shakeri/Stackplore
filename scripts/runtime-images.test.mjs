import assert from 'node:assert/strict';
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { pathToFileURL } from 'node:url';
import {
  checkRuntimeImages,
  POSTGRES_DOCKERFILE_PATH,
  RUNTIME_IMAGE_REFERENCE_COUNTS,
  validateRuntimeImageReferences,
} from './check-runtime-images.mjs';
import {
  POSTGRES_IMAGE_POLICY_PATH,
  GOSU_BUILD_IMAGE,
  validatePostgresImagePolicy,
} from './runtime-image-policy.mjs';

const policy = (version = '17.11') => ({
  schema_version: 1,
  image: `postgres:${version}-alpine@sha256:${'a'.repeat(64)}`,
  version,
  server_version_num: 170_000 + Number(version.split('.')[1]),
  platform: 'linux/amd64',
  minimum_native_tools: '17.11',
  runtime_image: 'stackplore-postgres:1.0.1',
  gosu_version: '1.19',
  gosu_source_commit: 'c'.repeat(40),
  gosu_source_sha256: 'd'.repeat(64),
  gosu_build_go: '1.26.9',
});

const dockerfile = (pin) => `FROM ${GOSU_BUILD_IMAGE} AS gosu-build
ADD --checksum=sha256:${pin.gosu_source_sha256} https://codeload.github.com/tianon/gosu/tar.gz/${pin.gosu_source_commit} /tmp/gosu.tar.gz
RUN test "$(go env GOVERSION)" = 'go${pin.gosu_build_go}' && \\
    grep -qx 'const Version = "${pin.gosu_version}"' version.go && \\
    CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/gosu .
FROM ${pin.image}
COPY --from=gosu-build --chmod=0755 /out/gosu /usr/local/bin/gosu
COPY --from=gosu-build /out/licenses/ /usr/share/licenses/stackplore-postgres/
`;

const sources = (pin) => ({
  ...Object.fromEntries(
    Object.entries(RUNTIME_IMAGE_REFERENCE_COUNTS).map(([file, count]) => [
      file,
      Array.from(
        { length: count },
        (_, index) => `  service${index}:\n    image: ${pin.runtime_image}\n`,
      ).join(''),
    ]),
  ),
  [POSTGRES_DOCKERFILE_PATH]: dockerfile(pin),
});

test('PostgreSQL policy accepts 17.11 and later minor versions unchanged', () => {
  for (const version of ['17.11', '17.12', '17.100']) {
    const pin = policy(version);
    assert.equal(validatePostgresImagePolicy(pin), pin);
  }
});

test('runtime image policy accepts canonical stable, alpha, beta and candidate tags', () => {
  for (const version of ['0.1.0', '2.0.0', '2.0.0-alpha.1', '2.0.0-beta.2', '2.0.0-rc.3']) {
    const pin = { ...policy(), runtime_image: `stackplore-postgres:${version}` };
    assert.equal(validatePostgresImagePolicy(pin), pin);
    assert.equal(validateRuntimeImageReferences(pin, sources(pin)), pin);
  }
});

test('runtime image policy rejects noncanonical or unsafe prerelease tags', () => {
  for (const version of [
    '02.0.0-alpha.1',
    '2.00.0-beta.1',
    '2.0.00-rc.1',
    '2.0.0-alpha.01',
    '2.0.0-beta.-1',
    '2.0.0-rc',
    '2.0.0-alpha.',
    '2.0.0-alpha.1.extra',
    '2.0.0-preview.1',
    '2.0.0-ALPHA.1',
    '2.0.0-rc.1+build',
    '2.0.0-rc.1/other',
    '2.0.0-rc.1\n',
    '2.0.0\n',
  ]) {
    assert.throws(
      () =>
        validatePostgresImagePolicy({
          ...policy(),
          runtime_image: `stackplore-postgres:${version}`,
        }),
      /runtime_image/u,
      version,
    );
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
    { runtime_image: 'postgres:17.11-alpine' },
    { runtime_image: 'stackplore-postgres:latest' },
    { runtime_image: 'stackplore-postgres:01.0.1' },
    { gosu_version: '1.18' },
    { gosu_source_commit: 'c'.repeat(39) },
    { gosu_source_sha256: 'd'.repeat(63) },
    { gosu_build_go: '1.24.6' },
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
    `services:\n  postgres:\n    image: '${pin.runtime_image}' # pinned\n  app:\n    image: example/app:1\n`;
  files['.github/workflows/release.yml'] = `    "image": "${pin.runtime_image}" # pinned\n`;
  files['.github/workflows/ci.yml'] =
    `env:\n  STACKPLORE_TEST_POSTGRES_IMAGE: ${pin.runtime_image}\npod:\n  image: ${pin.runtime_image}\n`;
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
      `# image: ${pin.runtime_image}\n`,
      sources(pin)[file].replace(/image: /u, '# image: '),
      `${sources(pin)[file]}image: ${pin.runtime_image}\n`,
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
      'stackplore-postgres:1.0.0',
    ]) {
      const drift = sources(pin);
      drift[file] = drift[file].replace(pin.runtime_image, image);
      assert.throws(() => validateRuntimeImageReferences(pin, drift), /image drift/u);
    }
  }
});

test('runtime Dockerfile rejects unpinned, missing, or drifted base and helper build inputs', () => {
  const pin = policy();
  const original = sources(pin);
  const missing = { ...original };
  delete missing[POSTGRES_DOCKERFILE_PATH];
  assert.throws(() => validateRuntimeImageReferences(pin, missing), /source missing/u);
  for (const content of [
    dockerfile(pin).replace(pin.image, 'postgres:17.11-alpine'),
    dockerfile(pin).replace(GOSU_BUILD_IMAGE, 'golang:1.24.6-alpine'),
    dockerfile(pin).replace(pin.gosu_source_commit, 'e'.repeat(40)),
    dockerfile(pin).replace(pin.gosu_source_sha256, 'f'.repeat(64)),
    dockerfile(pin).replace('go1.26.9', 'go1.24.6'),
    dockerfile(pin).replace('const Version = "1.19"', 'const Version = "1.18"'),
    dockerfile(pin).replace('CGO_ENABLED=0', 'CGO_ENABLED=1'),
    dockerfile(pin).replace('/usr/local/bin/gosu', '/usr/local/bin/other'),
    dockerfile(pin).replace('/usr/share/licenses/stackplore-postgres/', '/discard/'),
    dockerfile(pin).replace(
      'COPY --from=gosu-build /out/licenses/',
      '# COPY --from=gosu-build /out/licenses/',
    ),
    `# syntax=docker/dockerfile:latest\n${dockerfile(pin)}`,
    `${dockerfile(pin)}FROM ${pin.image}\n`,
    `${dockerfile(pin)}ENTRYPOINT ["sh"]\n`,
    `${dockerfile(pin)}USER 70\n`,
  ]) {
    assert.throws(
      () =>
        validateRuntimeImageReferences(pin, { ...original, [POSTGRES_DOCKERFILE_PATH]: content }),
      /Dockerfile build-input drift/u,
    );
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
