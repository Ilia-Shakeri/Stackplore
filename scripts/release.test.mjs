import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import process from 'node:process';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import { pathToFileURL } from 'node:url';
import { isLegalMaterialName } from './legal-file-name.mjs';
import {
  checkDatabaseImageMetadata,
  checkDatabasePin,
  checkDatabaseScanIdentity,
  checkDatabaseSbomIdentity,
  checkReleaseScan,
  createDatabaseImageMetadata,
} from './release-scan.mjs';

const script = path.join(import.meta.dirname, 'release.mjs');

test('legal inventory retains notices and excludes similarly named source files', () => {
  for (const name of ['LICENSE', 'LICENCE', 'COPYING', 'NOTICE.txt', 'OFL.txt', 'LICENSE-MIT']) {
    assert.equal(isLegalMaterialName(name), true, name);
  }
  for (const name of [
    'copyright.mjs',
    'copyright.cjs',
    'copyright.jsx',
    'copyright.mts',
    'copyright.cts',
    'copyright.tsx',
    'license.json',
    'notice.css',
    'patents.go',
    'ofl.wasm',
    'index.js',
  ]) {
    assert.equal(isLegalMaterialName(name), false, name);
  }
});

const fixture = async (context) => {
  const root = await mkdtemp(path.join(tmpdir(), 'stackplore-release-test-'));
  context.after(() => rm(root, { recursive: true, force: true }));
  await writeFile(path.join(root, 'package.json'), '{"version":"1.2.3"}');
  const directory = path.join(root, 'stackplore-release');
  await mkdir(directory);
  const run = (command, extra = {}, hook) =>
    spawnSync(
      process.execPath,
      [...(hook ? ['--import', pathToFileURL(hook).href] : []), script, command],
      {
        cwd: root,
        encoding: 'utf8',
        env: {
          ...process.env,
          RUNNER_TEMP: root,
          GITHUB_REPOSITORY: 'example/project',
          GITHUB_SHA: '1'.repeat(40),
          GITHUB_REF_TYPE: 'branch',
          GITHUB_REF_NAME: 'main',
          GH_TOKEN: '',
          ...extra,
        },
      },
    );
  return { root, directory, run };
};

test('checksums hash payloads and exclude signature bundles and the manifest itself', async (context) => {
  const { directory, run } = await fixture(context);
  await writeFile(path.join(directory, 'payload.tar.gz'), 'release payload');
  await writeFile(path.join(directory, 'provenance.sigstore.json'), 'bundle');
  await writeFile(path.join(directory, 'SHA256SUMS'), 'old manifest');
  const result = run('checksums');
  assert.equal(result.status, 0, result.stderr);
  const expected = `${createHash('sha256').update('release payload').digest('hex')}  payload.tar.gz\n`;
  assert.equal(await readFile(path.join(directory, 'SHA256SUMS'), 'utf8'), expected);
  assert.equal(run('checksums').status, 0);
  assert.equal(await readFile(path.join(directory, 'SHA256SUMS'), 'utf8'), expected);
});

test('an unexpected directory prevents checksum publication', async (context) => {
  const { directory, run } = await fixture(context);
  await mkdir(path.join(directory, 'unexpected'));
  const result = run('checksums');
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Unexpected release artifact/);
});

test('branch and mismatched tags fail before any publication API request', async (context) => {
  const { run } = await fixture(context);
  for (const extra of [{}, { GITHUB_REF_TYPE: 'tag', GITHUB_REF_NAME: 'v1.2.4' }]) {
    const result = run('publish', extra);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /matching version tag/);
  }
});

test('matching tag refuses an incomplete artifact set before calling the API', async (context) => {
  const { run } = await fixture(context);
  const result = run('publish', { GITHUB_REF_TYPE: 'tag', GITHUB_REF_NAME: 'v1.2.3' });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /required release artifact is missing/);
});

test('metadata records exact source and supported build targets without credentials', async (context) => {
  const { directory, run } = await fixture(context);
  const result = run('metadata', { GH_TOKEN: 'fixture-value-never-recorded' });
  assert.equal(result.status, 0, result.stderr);
  const raw = await readFile(path.join(directory, 'release.json'), 'utf8');
  const metadata = JSON.parse(raw);
  assert.equal(metadata.revision, '1'.repeat(40));
  assert.equal(metadata.version, '1.2.3');
  assert.equal(metadata.targets.length, 5);
  assert.doesNotMatch(raw, /fixture-value-never-recorded/);
});

const verificationFixture = async (context, { moved = false, missing = false } = {}) => {
  const fixtureData = await fixture(context);
  const hook = path.join(fixtureData.root, 'mock-github.mjs');
  await writeFile(
    hook,
    `
    import assert from 'node:assert/strict';
    globalThis.fetch = async (url) => {
      const path = new URL(url).pathname;
      let result;
      if (path.endsWith('/git/ref/tags/v1.2.3')) result = {object:{type:'tag',sha:'2'.repeat(40)}};
      else if (path.endsWith('/git/tags/'+'2'.repeat(40))) result = {object:{type:'commit',sha:'${moved ? '3' : '1'}'.repeat(40)}};
      else if (path.endsWith('/actions/workflows/ci.yml/runs')) result = {workflow_runs:[
        {id:4,head_sha:'1'.repeat(40),head_branch:'main',status:'completed',conclusion:'success'},
        {id:5,head_sha:'1'.repeat(40),head_branch:'v1.2.3',status:'completed',conclusion:'success',html_url:'https://github.com/example/project/actions/runs/5'}
      ]};
      else {
        assert.ok(path.endsWith('/actions/runs/5/jobs'), 'must use exact tag run');
        result = {jobs:${JSON.stringify(['code', 'deploy', 'compose-install', 'helm-install', 'docker-provider', ...(missing ? [] : ['kubernetes-provider'])])}.map(name=>({name,conclusion:'success'}))};
      }
      return new Response(JSON.stringify(result),{status:200,headers:{'Content-Type':'application/json'}});
    };
  `,
  );
  return { ...fixtureData, hook };
};

test('annotated matching tag requires its own complete successful verification run', async (context) => {
  const { run, hook } = await verificationFixture(context);
  const result = run('check-ci', { GITHUB_REF_TYPE: 'tag', GITHUB_REF_NAME: 'v1.2.3' }, hook);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /actions\/runs\/5/);
});

test('moved annotated tag and missing exact-tag provider check keep publication closed', async (context) => {
  for (const options of [{ moved: true }, { missing: true }]) {
    const { run, hook } = await verificationFixture(context, options);
    const result = run('check-ci', { GITHUB_REF_TYPE: 'tag', GITHUB_REF_NAME: 'v1.2.3' }, hook);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, options.moved ? /no longer identifies/ : /required exact-tag/);
  }
});

test('native archive smoke refuses unsupported platform or absent owned storage before I/O', () => {
  const result = spawnSync(
    process.execPath,
    [path.join(import.meta.dirname, 'release-smoke.mjs')],
    {
      encoding: 'utf8',
      env: { ...process.env, RUNNER_TEMP: '' },
    },
  );
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /requires Linux amd64|requires owned runner temporary storage/);
});

test('native and image scan gates require all shipped binaries, OS inventory and Go SBOM packages', () => {
  const packages = {
    packages: [
      {
        externalRefs: [
          { referenceType: 'purl', referenceLocator: 'pkg:golang/example.org/module@v1.0.0' },
        ],
      },
    ],
  };
  const result = (extension = '') =>
    ['server', 'storage', 'operator'].map((name) => ({
      Class: 'lang-pkgs',
      Type: 'gobinary',
      Target: `usr/local/bin/stackplore-${name}${extension} (gobinary)`,
    }));
  assert.doesNotThrow(() => checkReleaseScan({ Results: result() }, packages));
  assert.doesNotThrow(() =>
    checkReleaseScan({ Results: result('.exe') }, packages, { extension: '.exe' }),
  );
  assert.doesNotThrow(() =>
    checkReleaseScan({ Results: [result()[0], { Class: 'os-pkgs', Type: 'alpine' }] }, packages, {
      image: true,
    }),
  );
  for (const Results of [[], result().slice(0, 2)])
    assert.throws(() => checkReleaseScan({ Results }, packages), /every shipped Go executable/);
  assert.throws(
    () =>
      checkReleaseScan({ Results: [{ Class: 'os-pkgs', Type: 'alpine' }] }, packages, {
        image: true,
      }),
    /every shipped Go executable/,
  );
  assert.throws(
    () => checkReleaseScan({ Results: result() }, packages, { image: true }),
    /operating-system package/,
  );
  assert.throws(
    () => checkReleaseScan({ Results: result() }, { packages: [] }),
    /Go module package URLs/,
  );
  assert.throws(
    () =>
      checkReleaseScan(
        { Results: [...result(), { Vulnerabilities: [{ Severity: 'HIGH' }] }] },
        packages,
      ),
    /blocking vulnerability/,
  );
});

const databasePin = {
  schema_version: 1,
  image: `postgres:17.11-alpine@sha256:${'a'.repeat(64)}`,
  version: '17.11',
  server_version_num: 170011,
  platform: 'linux/amd64',
  minimum_native_tools: '17.11',
};
const databaseInventory = {
  spdxVersion: 'SPDX-2.3',
  SPDXID: 'SPDXRef-DOCUMENT',
  name: databasePin.image,
  packages: [
    {
      name: databasePin.image,
      SPDXID: 'SPDXRef-ContainerImage-fixture',
      primaryPackagePurpose: 'CONTAINER',
      annotations: [
        { comment: `ImageID: sha256:${'b'.repeat(64)}` },
        { comment: `Reference: ${databasePin.image}` },
      ],
    },
    {
      name: 'alpine',
      SPDXID: 'SPDXRef-OperatingSystem-fixture',
      primaryPackagePurpose: 'OPERATING-SYSTEM',
      versionInfo: '3.23.6',
    },
    {
      name: 'busybox',
      SPDXID: 'SPDXRef-Package-fixture',
      versionInfo: '1.37.0-r30',
      externalRefs: [
        {
          referenceType: 'purl',
          referenceLocator: 'pkg:apk/alpine/busybox@1.37.0-r30?arch=x86_64',
        },
      ],
    },
  ],
  relationships: [
    {
      spdxElementId: 'SPDXRef-DOCUMENT',
      relatedSpdxElement: 'SPDXRef-ContainerImage-fixture',
      relationshipType: 'DESCRIBES',
    },
    {
      spdxElementId: 'SPDXRef-ContainerImage-fixture',
      relatedSpdxElement: 'SPDXRef-OperatingSystem-fixture',
      relationshipType: 'CONTAINS',
    },
    {
      spdxElementId: 'SPDXRef-OperatingSystem-fixture',
      relatedSpdxElement: 'SPDXRef-Package-fixture',
      relationshipType: 'CONTAINS',
    },
  ],
};
const databaseInspection = [
  {
    Os: 'linux',
    Architecture: 'amd64',
    Id: `sha256:${'b'.repeat(64)}`,
    RepoDigests: [`postgres@sha256:${'a'.repeat(64)}`],
  },
];
const databaseIndex = {
  manifests: [
    { digest: `sha256:${'c'.repeat(64)}`, platform: { os: 'linux', architecture: 'amd64' } },
  ],
};
const databaseMetadata = () =>
  createDatabaseImageMetadata(
    databasePin,
    databaseInspection,
    databaseIndex,
    'postgres (PostgreSQL) 17.11\n',
  );
const databaseReport = () => ({
  ArtifactName: databasePin.image,
  Metadata: { ImageID: databaseInspection[0].Id, OS: { Family: 'alpine', Name: '3.23.6' } },
  Results: [
    {
      Class: 'os-pkgs',
      Type: 'alpine',
      Packages: [{ Identifier: { PURL: 'pkg:apk/alpine/busybox@1.37.0-r30?arch=x86_64' } }],
    },
  ],
});

test('database gate requires Alpine scan and package URLs without inventing Go executable coverage', () => {
  assert.doesNotThrow(() =>
    checkReleaseScan(databaseReport(), databaseInventory, { database: true }),
  );
  for (const Results of [
    [],
    [{ Class: 'os-pkgs', Type: 'debian' }],
    [{ Class: 'lang-pkgs', Type: 'gobinary' }],
  ]) {
    assert.throws(
      () => checkReleaseScan({ Results }, databaseInventory, { database: true }),
      /operating-system package/,
    );
  }
  for (const packages of [
    [],
    [
      {
        externalRefs: [
          { referenceType: 'purl', referenceLocator: 'pkg:golang/example.org/module@v1.0.0' },
        ],
      },
    ],
  ]) {
    assert.throws(
      () => checkReleaseScan(databaseReport(), { packages }, { database: true }),
      /Alpine package URLs/,
    );
  }
  for (const Severity of ['HIGH', 'CRITICAL']) {
    const report = databaseReport();
    report.Results[0].Vulnerabilities = [{ Severity }];
    assert.throws(
      () => checkReleaseScan(report, databaseInventory, { database: true }),
      /blocking vulnerability/,
    );
  }
});

test('database pin binds the official version, digest, platform and native tool floor', () => {
  assert.doesNotThrow(() => checkDatabasePin(databasePin));
  for (const change of [
    { image: databasePin.image.replace('postgres:', 'other/postgres:') },
    { image: 'postgres:17.11-alpine' },
    { image: databasePin.image.replace('17.11', '17.6') },
    { platform: 'linux/arm64' },
    { server_version_num: 170006 },
    { minimum_native_tools: '17.6' },
  ])
    assert.throws(() => checkDatabasePin({ ...databasePin, ...change }), /policy/);
});

test('database receipt binds the pulled upstream index, platform and actual compiled engine version', () => {
  const metadata = databaseMetadata();
  assert.equal(metadata.image, databasePin.image);
  assert.equal(metadata.platform_manifest_digest, databaseIndex.manifests[0].digest);
  assert.equal(metadata.config_digest, databaseInspection[0].Id);
  for (const probe of [
    'postgres (PostgreSQL) 17.6',
    'psql (PostgreSQL) 17.11',
    'postgres (PostgreSQL) 17.11\nextra',
  ]) {
    assert.throws(
      () => createDatabaseImageMetadata(databasePin, databaseInspection, databaseIndex, probe),
      /approved engine version/,
    );
  }
  for (const change of [
    { Os: 'windows' },
    { Architecture: 'arm64' },
    { Id: 'invalid' },
    { RepoDigests: [`postgres@sha256:${'d'.repeat(64)}`] },
  ]) {
    assert.throws(
      () =>
        createDatabaseImageMetadata(
          databasePin,
          [{ ...databaseInspection[0], ...change }],
          databaseIndex,
          'postgres (PostgreSQL) 17.11',
        ),
      /pinned upstream image/,
    );
  }
  assert.throws(
    () =>
      createDatabaseImageMetadata(
        databasePin,
        databaseInspection,
        { manifests: [] },
        'postgres (PostgreSQL) 17.11',
      ),
    /pinned upstream image/,
  );
  for (const change of [
    { network: 'default' },
    { read_only: false },
    { user: '0:0' },
    { exit_code: 1 },
    { no_new_privileges: false },
  ]) {
    assert.throws(
      () =>
        checkDatabaseImageMetadata(
          { ...metadata, runtime_probe: { ...metadata.runtime_probe, ...change } },
          databasePin,
        ),
      /approved engine version/,
    );
  }
  assert.doesNotThrow(() => checkDatabaseScanIdentity(databaseReport(), metadata));
  for (const report of [
    { ...databaseReport(), ArtifactName: 'postgres:17.6-alpine' },
    {
      ...databaseReport(),
      Metadata: { ImageID: `sha256:${'d'.repeat(64)}`, OS: { Family: 'alpine' } },
    },
    {
      ...databaseReport(),
      Metadata: { ImageID: metadata.config_digest, OS: { Family: 'debian' } },
    },
  ])
    assert.throws(() => checkDatabaseScanIdentity(report, metadata), /probed image configuration/);
});

test('database SPDX binds the container root, config digest, document and OS package graph', () => {
  const metadata = databaseMetadata();
  assert.doesNotThrow(() =>
    checkDatabaseSbomIdentity(databaseInventory, metadata, databaseReport()),
  );
  for (const alter of [
    (sbom) => {
      sbom.name = 'unrelated:latest';
    },
    (sbom) => {
      sbom.packages[0].name = 'unrelated:latest';
    },
    (sbom) => {
      sbom.packages[0].annotations[0].comment = `ImageID: sha256:${'d'.repeat(64)}`;
    },
    (sbom) => {
      sbom.packages[0].annotations[1].comment = 'Reference: unrelated:latest';
    },
    (sbom) => {
      sbom.relationships.shift();
    },
    (sbom) => {
      sbom.relationships[0].relatedSpdxElement = 'SPDXRef-unrelated';
    },
    (sbom) => {
      sbom.relationships.splice(1, 1);
    },
    (sbom) => {
      sbom.relationships.pop();
    },
    (sbom) => {
      sbom.packages[1].versionInfo = '3.22.0';
    },
    (sbom) => {
      sbom.packages[2].externalRefs[0].referenceLocator = 'pkg:apk/alpine/busybox@1.0';
    },
    (sbom) => {
      sbom.packages.push({
        ...sbom.packages[2],
        SPDXID: 'SPDXRef-orphan',
        externalRefs: [{ referenceType: 'purl', referenceLocator: 'pkg:apk/alpine/orphan@1.0' }],
      });
    },
  ]) {
    const sbom = globalThis.structuredClone(databaseInventory);
    alter(sbom);
    assert.throws(
      () => checkDatabaseSbomIdentity(sbom, metadata, databaseReport()),
      /Database SPDX/,
    );
  }
  const report = databaseReport();
  delete report.Results[0].Packages;
  assert.throws(
    () => checkDatabaseSbomIdentity(databaseInventory, metadata, report),
    /package inventory/,
  );
});

test('database SPDX traverses transitive OS dependencies and terminates dependency cycles', () => {
  const sbom = globalThis.structuredClone(databaseInventory);
  const report = databaseReport();
  for (const name of ['libapk', 'musl']) {
    const purl = `pkg:apk/alpine/${name}@1.0?arch=x86_64`;
    sbom.packages.push({
      name,
      SPDXID: `SPDXRef-Package-${name}`,
      versionInfo: '1.0',
      externalRefs: [{ referenceType: 'purl', referenceLocator: purl }],
    });
    report.Results[0].Packages.push({ Identifier: { PURL: purl } });
  }
  sbom.relationships.push(
    {
      spdxElementId: 'SPDXRef-Package-fixture',
      relatedSpdxElement: 'SPDXRef-Package-libapk',
      relationshipType: 'DEPENDS_ON',
    },
    {
      spdxElementId: 'SPDXRef-Package-libapk',
      relatedSpdxElement: 'SPDXRef-Package-musl',
      relationshipType: 'DEPENDS_ON',
    },
    {
      spdxElementId: 'SPDXRef-Package-musl',
      relatedSpdxElement: 'SPDXRef-Package-fixture',
      relationshipType: 'DEPENDS_ON',
    },
  );
  assert.doesNotThrow(() => checkDatabaseSbomIdentity(sbom, databaseMetadata(), report));
  sbom.relationships.splice(3, 1);
  assert.throws(
    () => checkDatabaseSbomIdentity(sbom, databaseMetadata(), report),
    /package inventory/,
  );
});

const completeReleaseFixture = async (context) => {
  const data = await verificationFixture(context);
  const expected = [
    'SHA256SUMS',
    'provenance.sigstore.json',
    'release.json',
    'native-smoke.json',
    'dependency-scan.json',
    'image-scan.json',
    'database-image-scan.json',
    'database.spdx.json',
    'database-image.json',
    'secret-scan.json',
    'image-id.txt',
    'source.spdx.json',
    'stackplore-1.2.3.tgz',
    'stackplore_1.2.3_source.tar.gz',
    'stackplore_1.2.3_image.tar.gz',
    'stackplore_1.2.3_image.spdx.json',
    'image-sbom.sigstore.json',
    ...['linux_amd64', 'linux_arm64', 'windows_amd64', 'darwin_amd64', 'darwin_arm64'].flatMap(
      (target) => [
        `stackplore_1.2.3_${target}.tar.gz`,
        `stackplore_1.2.3_${target}.scan.json`,
        `stackplore_1.2.3_${target}.spdx.json`,
        `${target.replace('_', '-')}-sbom.sigstore.json`,
      ],
    ),
  ];
  assert.equal(expected.length, 37);
  await Promise.all(
    expected.map((name) => writeFile(path.join(data.directory, name), 'fixture payload')),
  );
  await mkdir(path.join(data.root, 'deploy'));
  await writeFile(path.join(data.root, 'deploy/postgres-image.json'), JSON.stringify(databasePin));
  await writeFile(
    path.join(data.directory, 'database-image.json'),
    JSON.stringify(databaseMetadata()),
  );
  await writeFile(
    path.join(data.directory, 'database-image-scan.json'),
    JSON.stringify(databaseReport()),
  );
  await writeFile(
    path.join(data.directory, 'database.spdx.json'),
    JSON.stringify(databaseInventory),
  );
  await writeFile(path.join(data.root, 'CHANGELOG.md'), '## [1.2.3]\n\nRelease fixture.\n');
  assert.equal(data.run('metadata', { GITHUB_REF: 'refs/tags/v1.2.3' }).status, 0);
  assert.equal(data.run('checksums').status, 0);
  const hook = `${await readFile(data.hook, 'utf8')}
    import { appendFile } from 'node:fs/promises';
    const verificationFetch = globalThis.fetch;
    globalThis.fetch = async (url, options = {}) => {
      const target = new URL(url);
      if (target.hostname === 'uploads.github.com') {
        assert.equal(target.pathname, '/repos/example/project/releases/6/assets');
        await appendFile(${JSON.stringify(path.join(data.root, 'uploads.jsonl'))}, JSON.stringify(target.searchParams.get('name'))+'\\n');
        return new Response('{}', {status:201});
      }
      if (target.pathname.endsWith('/releases')) return Response.json(options.method === 'POST' ? {
        id:6, upload_url:'https://uploads.github.com/repos/example/project/releases/6/assets{?name,label}',
        html_url:'https://github.com/example/project/releases/tag/v1.2.3'
      } : []);
      if (target.pathname.endsWith('/releases/6')) {
        assert.equal(options.method, 'PATCH'); assert.deepEqual(JSON.parse(options.body), {draft:false});
        return Response.json({});
      }
      return verificationFetch(url, options);
    };
  `;
  await writeFile(data.hook, hook);
  return { ...data, expected };
};

test('publication accepts exactly 37 assets with all database receipts covered by checksums', async (context) => {
  const { root, run, hook, expected } = await completeReleaseFixture(context);
  const result = run('publish', { GITHUB_REF_TYPE: 'tag', GITHUB_REF_NAME: 'v1.2.3' }, hook);
  assert.equal(result.status, 0, result.stderr);
  const uploaded = (await readFile(path.join(root, 'uploads.jsonl'), 'utf8'))
    .trim()
    .split('\n')
    .map((line) => JSON.parse(line));
  assert.deepEqual(uploaded.sort(), expected.sort());
});

test('publication rejects missing, tampered or mismatched database evidence before upload', async (context) => {
  for (const filename of [
    'database-image-scan.json',
    'database.spdx.json',
    'database-image.json',
  ]) {
    const { directory, run, hook } = await completeReleaseFixture(context);
    await rm(path.join(directory, filename));
    const result = run('publish', { GITHUB_REF_TYPE: 'tag', GITHUB_REF_NAME: 'v1.2.3' }, hook);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /required release artifact is missing/);
  }
  for (const refreshManifest of [false, true]) {
    const { directory, run, hook } = await completeReleaseFixture(context);
    const metadata = databaseMetadata();
    metadata.runtime_probe.output = 'postgres (PostgreSQL) 17.6';
    await writeFile(path.join(directory, 'database-image.json'), JSON.stringify(metadata));
    if (refreshManifest) assert.equal(run('checksums').status, 0);
    const result = run('publish', { GITHUB_REF_TYPE: 'tag', GITHUB_REF_NAME: 'v1.2.3' }, hook);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, refreshManifest ? /approved engine version/ : /checksum mismatch/);
  }
});

test('publication rejects unrelated or disconnected database SPDX after checksum recomputation', async (context) => {
  for (const alter of [
    (sbom) => {
      sbom.name = 'unrelated:latest';
    },
    (sbom) => {
      sbom.packages[0].annotations[0].comment = `ImageID: sha256:${'d'.repeat(64)}`;
    },
    (sbom) => {
      sbom.relationships = [];
    },
    (sbom) => {
      sbom.packages[2].externalRefs[0].referenceLocator = 'pkg:apk/alpine/busybox@1.0';
    },
  ]) {
    const { root, directory, run, hook } = await completeReleaseFixture(context);
    const sbom = globalThis.structuredClone(databaseInventory);
    alter(sbom);
    await writeFile(path.join(directory, 'database.spdx.json'), JSON.stringify(sbom));
    assert.equal(run('checksums').status, 0);
    const result = run('publish', { GITHUB_REF_TYPE: 'tag', GITHUB_REF_NAME: 'v1.2.3' }, hook);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /Database SPDX/);
    await assert.rejects(readFile(path.join(root, 'uploads.jsonl')), { code: 'ENOENT' });
  }
});
