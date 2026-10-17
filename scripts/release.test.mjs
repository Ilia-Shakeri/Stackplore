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
import { checkReleaseScan } from './release-scan.mjs';

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
