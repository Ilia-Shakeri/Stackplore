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
import { releaseNotes } from './release-notes.mjs';
import {
  checkDatabaseImageMetadata,
  checkDatabasePin,
  checkDatabaseScanIdentity,
  checkDatabaseSbomIdentity,
  checkReleaseScan,
  createDatabaseImageMetadata,
} from './release-scan.mjs';

const script = path.join(import.meta.dirname, 'release.mjs');

test('published notes pin repository document links to the exact source revision', () => {
  const revision = 'a'.repeat(40);
  const changelog =
    '# Changes\n\n## [1.2.3] - date\n\n[Receipt](docs/evidence/1.2.3/README.md#scope) and [Runbook](./docs/operations/release.md). [External](https://example.org/page)\n\n## [1.2.2] - old\nEarlier text';
  const notes = releaseNotes(changelog, '1.2.3', 'example/project', revision);
  assert.ok(
    notes.includes(
      `](https://github.com/example/project/blob/${revision}/docs/evidence/1.2.3/README.md#scope)`,
    ),
  );
  assert.ok(
    notes.includes(
      `](https://github.com/example/project/blob/${revision}/docs/operations/release.md)`,
    ),
  );
  assert.ok(notes.includes('](https://example.org/page)'));
  assert.ok(!notes.includes('Earlier text'));
  assert.throws(() => releaseNotes(changelog, '1.2.4', 'example/project', revision), /missing/);
});

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
  await mkdir(path.join(root, 'deploy'));
  await writeFile(path.join(root, 'deploy/postgres-image.json'), JSON.stringify(databasePin));
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
  assert.equal(metadata.images.length, 2);
  assert.equal(metadata.images[1].archive, 'stackplore_1.2.3_postgres_image.tar.gz');
  assert.equal(metadata.images[1].image, databasePin.runtime_image);
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
  runtime_image: 'stackplore-postgres:1.2.3',
  gosu_version: '1.19',
  gosu_source_commit: 'd'.repeat(40),
  gosu_source_sha256: 'f'.repeat(64),
  gosu_build_go: '1.26.9',
};
const databaseInventory = {
  spdxVersion: 'SPDX-2.3',
  SPDXID: 'SPDXRef-DOCUMENT',
  name: databasePin.runtime_image,
  packages: [
    {
      name: databasePin.runtime_image,
      SPDXID: 'SPDXRef-ContainerImage-fixture',
      primaryPackagePurpose: 'CONTAINER',
      annotations: [
        { comment: `ImageID: sha256:${'e'.repeat(64)}` },
        { comment: `Reference: ${databasePin.runtime_image}` },
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
    {
      name: 'usr/local/bin/gosu',
      SPDXID: 'SPDXRef-Application-gosu',
      primaryPackagePurpose: 'APPLICATION',
      annotations: [{ comment: 'Type: gobinary' }],
    },
    {
      name: 'stdlib',
      SPDXID: 'SPDXRef-Package-stdlib',
      versionInfo: 'v1.26.9',
      externalRefs: [{ referenceType: 'purl', referenceLocator: 'pkg:golang/stdlib@v1.26.9' }],
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
    {
      spdxElementId: 'SPDXRef-ContainerImage-fixture',
      relatedSpdxElement: 'SPDXRef-Application-gosu',
      relationshipType: 'CONTAINS',
    },
    {
      spdxElementId: 'SPDXRef-Application-gosu',
      relatedSpdxElement: 'SPDXRef-Package-stdlib',
      relationshipType: 'DEPENDS_ON',
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
const databaseRuntimeInspection = [
  {
    Os: 'linux',
    Architecture: 'amd64',
    Id: `sha256:${'e'.repeat(64)}`,
    RepoTags: [databasePin.runtime_image],
  },
];
const databaseMetadata = ({
  inspection = databaseInspection,
  index = databaseIndex,
  runtime = databaseRuntimeInspection,
  probe = 'postgres (PostgreSQL) 17.11\n',
  gosuProbe = '1.19 (go1.26.9 on linux/amd64; gc)\n',
} = {}) => createDatabaseImageMetadata(databasePin, inspection, index, runtime, probe, gosuProbe);
const databaseReport = () => ({
  ArtifactName: databasePin.runtime_image,
  Metadata: { ImageID: databaseRuntimeInspection[0].Id, OS: { Family: 'alpine', Name: '3.23.6' } },
  Results: [
    {
      Class: 'os-pkgs',
      Type: 'alpine',
      Packages: [{ Identifier: { PURL: 'pkg:apk/alpine/busybox@1.37.0-r30?arch=x86_64' } }],
    },
    {
      Class: 'lang-pkgs',
      Type: 'gobinary',
      Target: 'usr/local/bin/gosu',
      Packages: [
        { Name: 'stdlib', Version: 'v1.26.9', Identifier: { PURL: 'pkg:golang/stdlib@v1.26.9' } },
      ],
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
  assert.equal(metadata.image, databasePin.runtime_image);
  assert.equal(metadata.upstream_image, databasePin.image);
  assert.equal(metadata.upstream_platform_manifest_digest, databaseIndex.manifests[0].digest);
  assert.equal(metadata.upstream_config_digest, databaseInspection[0].Id);
  assert.equal(metadata.config_digest, databaseRuntimeInspection[0].Id);
  for (const probe of [
    'postgres (PostgreSQL) 17.6',
    'psql (PostgreSQL) 17.11',
    'postgres (PostgreSQL) 17.11\nextra',
  ]) {
    assert.throws(() => databaseMetadata({ probe }), /approved engine version/);
  }
  for (const change of [
    { Os: 'windows' },
    { Architecture: 'arm64' },
    { Id: 'invalid' },
    { RepoDigests: [`postgres@sha256:${'d'.repeat(64)}`] },
  ]) {
    assert.throws(
      () => databaseMetadata({ inspection: [{ ...databaseInspection[0], ...change }] }),
      /pinned upstream image/,
    );
  }
  assert.throws(() => databaseMetadata({ index: { manifests: [] } }), /pinned upstream image/);
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

test('database derivative requires the approved gosu source, compiler and actual binary inventory', () => {
  for (const gosuProbe of [
    '1.19 (go1.24.6 on linux/amd64; gc)',
    '1.18 (go1.26.9 on linux/amd64; gc)',
    '1.19 (go1.26.9 on linux/arm64; gc)',
  ]) {
    assert.throws(() => databaseMetadata({ gosuProbe }), /approved engine version/);
  }
  for (const runtime of [
    databaseInspection,
    [{ ...databaseRuntimeInspection[0], RepoTags: ['unrelated:latest'] }],
    [{ ...databaseRuntimeInspection[0], Architecture: 'arm64' }],
  ]) {
    assert.throws(() => databaseMetadata({ runtime }), /built derivative image/);
  }
  for (const alter of [
    (metadata) => {
      metadata.gosu_probe.go_version = '1.24.6';
    },
    (metadata) => {
      metadata.gosu_probe.network = 'default';
    },
    (metadata) => {
      metadata.build_inputs.gosu_source_commit = '0'.repeat(40);
    },
    (metadata) => {
      metadata.build_inputs.gosu_source_sha256 = '0'.repeat(64);
    },
    (metadata) => {
      metadata.config_digest = metadata.upstream_config_digest;
    },
  ]) {
    const metadata = databaseMetadata();
    alter(metadata);
    assert.throws(
      () => checkDatabaseImageMetadata(metadata, databasePin),
      /approved engine version/,
    );
  }
  for (const alter of [
    (report) => {
      report.Results.pop();
    },
    (report) => {
      report.Results[1].Packages = [];
    },
    (report) => {
      report.Results[1].Packages[0].Version = 'v1.24.6';
    },
    (report) => {
      report.Results[1].Packages[0].Identifier.PURL = 'pkg:golang/stdlib@v1.24.6';
    },
  ]) {
    const report = databaseReport();
    alter(report);
    assert.throws(
      () => checkDatabaseScanIdentity(report, databaseMetadata()),
      /approved Go standard library/,
    );
  }
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
  const dependencyStart = sbom.relationships.length;
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
  sbom.relationships.splice(dependencyStart, 1);
  assert.throws(
    () => checkDatabaseSbomIdentity(sbom, databaseMetadata(), report),
    /package inventory/,
  );
});

test('source-built gosu retains its versionless main PURL while Go dependencies require versions', () => {
  const sbom = globalThis.structuredClone(databaseInventory);
  const report = databaseReport();
  const additions = [
    ['github.com/tianon/gosu', null, 'pkg:golang/github.com/tianon/gosu'],
    ['github.com/moby/sys/user', 'v0.1.0', 'pkg:golang/github.com/moby/sys/user@v0.1.0'],
    ['golang.org/x/sys', 'v0.1.0', 'pkg:golang/golang.org/x/sys@v0.1.0'],
  ];
  for (const [index, [name, version, purl]] of additions.entries()) {
    const id = `SPDXRef-Package-source-${index}`;
    sbom.packages.push({
      name,
      SPDXID: id,
      ...(version ? { versionInfo: version } : {}),
      externalRefs: [{ referenceType: 'purl', referenceLocator: purl }],
    });
    sbom.relationships.push({
      spdxElementId: 'SPDXRef-Application-gosu',
      relatedSpdxElement: id,
      relationshipType: 'DEPENDS_ON',
    });
    report.Results[1].Packages.push({
      Name: name,
      ...(version ? { Version: version } : {}),
      Identifier: { PURL: purl },
    });
  }
  assert.equal(report.Results[1].Packages.length, 4);
  assert.doesNotThrow(() => checkDatabaseSbomIdentity(sbom, databaseMetadata(), report));
  assert.doesNotThrow(() => checkDatabaseScanIdentity(report, databaseMetadata()));
  for (const invalidPurl of [
    'pkg:golang/github.com/moby/sys/user',
    'pkg:golang/github.com/moby/sys/user@',
  ]) {
    const changedSbom = globalThis.structuredClone(sbom);
    const changedReport = globalThis.structuredClone(report);
    changedSbom.packages.find(
      (entry) => entry.name === 'github.com/moby/sys/user',
    ).externalRefs[0].referenceLocator = invalidPurl;
    changedReport.Results[1].Packages.find(
      (entry) => entry.Name === 'github.com/moby/sys/user',
    ).Identifier.PURL = invalidPurl;
    assert.throws(
      () => checkDatabaseSbomIdentity(changedSbom, databaseMetadata(), changedReport),
      /gosu inventory/,
    );
  }
  for (const alter of [
    (changed) => {
      changed.packages = changed.packages.filter(
        (entry) => entry.name !== 'github.com/tianon/gosu',
      );
    },
    (changed) => {
      changed.packages.find((entry) => entry.name === 'stdlib').externalRefs[0].referenceLocator =
        'pkg:golang/stdlib@v1.24.6';
    },
    (changed) => {
      changed.packages.find(
        (entry) => entry.name === 'github.com/tianon/gosu',
      ).externalRefs[0].referenceLocator = 'pkg:golang/example.org/other';
    },
  ]) {
    const changed = globalThis.structuredClone(sbom);
    alter(changed);
    assert.throws(
      () => checkDatabaseSbomIdentity(changed, databaseMetadata(), report),
      /gosu inventory/,
    );
  }
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
    'stackplore_1.2.3_postgres_image.tar.gz',
    'postgres-image-sbom.sigstore.json',
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
  assert.equal(expected.length, 39);
  assert.equal(expected.filter((name) => name.endsWith('-sbom.sigstore.json')).length, 7);
  assert.equal(expected.filter((name) => !name.endsWith('.sigstore.json')).length, 31);
  await Promise.all(
    expected.map((name) => writeFile(path.join(data.directory, name), 'fixture payload')),
  );
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

test('publication accepts exactly 39 assets with all database receipts and archive covered by checksums', async (context) => {
  const { root, run, hook, expected } = await completeReleaseFixture(context);
  const result = run('publish', { GITHUB_REF_TYPE: 'tag', GITHUB_REF_NAME: 'v1.2.3' }, hook);
  assert.equal(result.status, 0, result.stderr);
  const uploaded = (await readFile(path.join(root, 'uploads.jsonl'), 'utf8'))
    .trim()
    .split('\n')
    .map((line) => JSON.parse(line));
  assert.deepEqual(uploaded.sort(), expected.sort());
  const manifest = await readFile(path.join(root, 'stackplore-release/SHA256SUMS'), 'utf8');
  assert.equal(manifest.trim().split('\n').length, 30);
});

test('publication rejects missing, tampered or mismatched database evidence before upload', async (context) => {
  for (const filename of [
    'database-image-scan.json',
    'database.spdx.json',
    'database-image.json',
    'stackplore_1.2.3_postgres_image.tar.gz',
    'postgres-image-sbom.sigstore.json',
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
    (sbom) => {
      sbom.packages.splice(3);
      sbom.relationships.splice(3);
    },
    (sbom) => {
      sbom.packages[4].externalRefs[0].referenceLocator = 'pkg:golang/stdlib@v1.24.6';
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

test('publication rejects freshly checksummed wrong gosu compiler or build-source metadata', async (context) => {
  for (const alter of [
    (metadata) => {
      metadata.gosu_probe.output = '1.19 (go1.24.6 on linux/amd64; gc)';
    },
    (metadata) => {
      metadata.build_inputs.gosu_source_commit = '0'.repeat(40);
    },
  ]) {
    const { root, directory, run, hook } = await completeReleaseFixture(context);
    const metadata = databaseMetadata();
    alter(metadata);
    await writeFile(path.join(directory, 'database-image.json'), JSON.stringify(metadata));
    assert.equal(run('checksums').status, 0);
    const result = run('publish', { GITHUB_REF_TYPE: 'tag', GITHUB_REF_NAME: 'v1.2.3' }, hook);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /approved engine version/);
    await assert.rejects(readFile(path.join(root, 'uploads.jsonl')), { code: 'ENOENT' });
  }
});
