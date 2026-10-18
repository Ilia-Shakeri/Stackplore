import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { lstat, readFile, readdir, writeFile } from 'node:fs/promises';
import { basename, join } from 'node:path';
import process from 'node:process';
import { releaseNotes } from './release-notes.mjs';
import { setTimeout as delay } from 'node:timers/promises';
import {
  checkDatabaseImageMetadata,
  checkDatabaseScanIdentity,
  checkDatabaseSbomIdentity,
  checkDatabasePin,
  checkReleaseScan,
} from './release-scan.mjs';

const command = process.argv[2];
const repository = process.env.GITHUB_REPOSITORY;
const revision = process.env.GITHUB_SHA;
const refName = process.env.GITHUB_REF_NAME;
const releaseDirectory = join(process.env.RUNNER_TEMP ?? '', 'stackplore-release');
const version = JSON.parse(await readFile('package.json', 'utf8')).version;
if (!/^\d+\.\d+\.\d+(?:-[a-z0-9.-]+)?$/.test(version)) throw new Error('Invalid release version.');
if (!/^[\w.-]+\/[\w.-]+$/.test(repository ?? '') || !/^[a-f0-9]{40}$/.test(revision ?? '')) {
  throw new Error('Release repository/revision is missing.');
}

const request = async (path, options = {}) => {
  const response = await globalThis.fetch(`https://api.github.com/repos/${repository}/${path}`, {
    ...options,
    headers: {
      Accept: 'application/vnd.github+json',
      Authorization: `Bearer ${process.env.GH_TOKEN ?? ''}`,
      'X-GitHub-Api-Version': '2022-11-28',
      ...options.headers,
    },
    signal: globalThis.AbortSignal.any([
      globalThis.AbortSignal.timeout(60_000),
      ...(options.signal ? [options.signal] : []),
    ]),
  });
  if (!response.ok) throw new Error(`Release API request failed (${response.status}).`);
  return response.json();
};

const assertTag = () => {
  if (process.env.GITHUB_REF_TYPE !== 'tag' || refName !== `v${version}`) {
    throw new Error('Publication requires the matching version tag.');
  }
};

const awaitVerification = async () => {
  assertTag();
  const signal = globalThis.AbortSignal.timeout(30 * 60_000);
  let reference = (await request(`git/ref/tags/${encodeURIComponent(refName)}`, { signal })).object;
  for (let depth = 0; reference.type === 'tag' && depth < 5; depth++) {
    reference = (await request(`git/tags/${reference.sha}`, { signal })).object;
  }
  if (reference.type !== 'commit' || reference.sha !== revision) {
    throw new Error('Published tag no longer identifies this source revision.');
  }
  const required = [
    'code',
    'deploy',
    'compose-install',
    'helm-install',
    'docker-provider',
    'kubernetes-provider',
  ];
  for (let attempt = 0; attempt < 90; attempt++) {
    const response = await request(
      `actions/workflows/ci.yml/runs?head_sha=${revision}&event=push&per_page=100`,
      { signal },
    );
    const run = response.workflow_runs.find(
      (entry) => entry.head_sha === revision && entry.head_branch === refName,
    );
    if (run?.status === 'completed') {
      if (run.conclusion !== 'success') throw new Error('Exact tag verification failed.');
      const result = await request(`actions/runs/${run.id}/jobs?per_page=100`, { signal });
      if (
        !required.every((name) =>
          result.jobs.some((job) => job.name === name && job.conclusion === 'success'),
        )
      ) {
        throw new Error('A required exact-tag verification job did not pass.');
      }
      process.stdout.write(`Exact-tag verification passed: ${run.html_url}\n`);
      return run;
    }
    await delay(20_000, undefined, { signal });
  }
  throw new Error('Exact-tag verification timed out.');
};

const files = async () => {
  if (!process.env.RUNNER_TEMP)
    throw new Error('Release directory requires runner temporary storage.');
  const names = (await readdir(releaseDirectory)).sort();
  for (const name of names) {
    const entry = await lstat(join(releaseDirectory, name));
    if (!/^[a-zA-Z0-9_.-]+$/.test(name) || !entry.isFile() || entry.isSymbolicLink()) {
      throw new Error('Unexpected release artifact.');
    }
  }
  return names;
};

const readDatabaseDocument = async (name, maximum) => {
  const path = join(releaseDirectory, name);
  const info = await lstat(path);
  if (!info.isFile() || info.isSymbolicLink() || info.size > maximum)
    throw new Error('Database release document is not a bounded regular file.');
  return JSON.parse(await readFile(path, 'utf8'));
};

if (command === 'check-ci') {
  await awaitVerification();
} else if (command === 'metadata') {
  const database = checkDatabasePin(
    JSON.parse(await readFile('deploy/postgres-image.json', 'utf8')),
  );
  await writeFile(
    join(releaseDirectory, 'release.json'),
    `${JSON.stringify(
      {
        version,
        repository,
        revision,
        ref: process.env.GITHUB_REF,
        workflow: '.github/workflows/release.yml',
        run: `${process.env.GITHUB_SERVER_URL}/${repository}/actions/runs/${process.env.GITHUB_RUN_ID}`,
        targets: ['linux/amd64', 'linux/arm64', 'windows/amd64', 'darwin/amd64', 'darwin/arm64'],
        imagePlatform: 'linux/amd64',
        images: [
          {
            image: `stackplore-api:${version}`,
            platform: 'linux/amd64',
            archive: `stackplore_${version}_image.tar.gz`,
          },
          {
            image: database.runtime_image,
            platform: database.platform,
            archive: `stackplore_${version}_postgres_image.tar.gz`,
          },
        ],
        tools: {
          node: '24.18.0',
          pnpm: '11.17.0',
          go: '1.26.9',
          helm: '3.17.3',
          trivy: '0.76.0',
          gitleaks: '8.30.1',
        },
      },
      null,
      2,
    )}\n`,
  );
} else if (command === 'checksums') {
  const lines = [];
  for (const name of await files()) {
    if (name === 'SHA256SUMS' || name.endsWith('.sigstore.json')) continue;
    const digest = createHash('sha256');
    for await (const chunk of createReadStream(join(releaseDirectory, name))) digest.update(chunk);
    lines.push(`${digest.digest('hex')}  ${name}`);
  }
  if (lines.length === 0) throw new Error('No release artifacts.');
  await writeFile(join(releaseDirectory, 'SHA256SUMS'), `${lines.join('\n')}\n`);
} else if (command === 'publish') {
  assertTag();
  const names = await files();
  const targets = ['linux_amd64', 'linux_arm64', 'windows_amd64', 'darwin_amd64', 'darwin_arm64'];
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
    `stackplore_${version}_postgres_image.tar.gz`,
    'postgres-image-sbom.sigstore.json',
    'secret-scan.json',
    'image-id.txt',
    'source.spdx.json',
    `stackplore-${version}.tgz`,
    `stackplore_${version}_source.tar.gz`,
    `stackplore_${version}_image.tar.gz`,
    `stackplore_${version}_image.spdx.json`,
    'image-sbom.sigstore.json',
    ...targets.flatMap((target) => [
      `stackplore_${version}_${target}.tar.gz`,
      `stackplore_${version}_${target}.scan.json`,
      `stackplore_${version}_${target}.spdx.json`,
      `${target.replace('_', '-')}-sbom.sigstore.json`,
    ]),
  ];
  for (const required of expected) {
    if (!names.includes(required)) throw new Error('A required release artifact is missing.');
  }
  if (names.length !== expected.length) throw new Error('An unreviewed release asset is present.');
  const metadata = JSON.parse(await readFile(join(releaseDirectory, 'release.json'), 'utf8'));
  if (
    metadata.version !== version ||
    metadata.revision !== revision ||
    metadata.repository !== repository ||
    metadata.ref !== `refs/tags/${refName}`
  ) {
    throw new Error('Release metadata does not match this exact tagged source.');
  }
  const manifest = await readFile(join(releaseDirectory, 'SHA256SUMS'), 'utf8');
  const checksums = new Map();
  for (const line of manifest.trim().split('\n')) {
    const match = /^([a-f0-9]{64}) {2}([a-zA-Z0-9_.-]+)$/.exec(line);
    if (!match || checksums.has(match[2])) throw new Error('Release checksum manifest is invalid.');
    checksums.set(match[2], match[1]);
  }
  for (const name of expected) {
    if (name === 'SHA256SUMS' || name.endsWith('.sigstore.json')) continue;
    const digest = createHash('sha256');
    for await (const chunk of createReadStream(join(releaseDirectory, name))) digest.update(chunk);
    if (digest.digest('hex') !== checksums.get(name))
      throw new Error('Release artifact checksum mismatch.');
    checksums.delete(name);
  }
  if (checksums.size !== 0) throw new Error('Release manifest has unexpected entries.');
  const database = await readDatabaseDocument('database-image.json', 16 * 1024);
  const databaseScan = await readDatabaseDocument('database-image-scan.json', 32 * 1024 * 1024);
  const databaseSbom = await readDatabaseDocument('database.spdx.json', 16 * 1024 * 1024);
  checkDatabaseImageMetadata(
    database,
    JSON.parse(await readFile('deploy/postgres-image.json', 'utf8')),
  );
  checkReleaseScan(databaseScan, databaseSbom, { database: true });
  checkDatabaseScanIdentity(databaseScan, database);
  checkDatabaseSbomIdentity(databaseSbom, database, databaseScan);
  const verified = await awaitVerification();
  const changelog = await readFile('CHANGELOG.md', 'utf8');
  const notes = releaseNotes(changelog, version, repository, revision);
  // Existing releases are never replaced, and partial uploads stay drafts for review.
  const releases = await request('releases?per_page=100');
  if (releases.some((release) => release.tag_name === refName))
    throw new Error('Release already exists; inspect it without overwriting.');
  const release = await request('releases', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      tag_name: refName,
      target_commitish: revision,
      name: `Stackplore ${version}`,
      body: `${notes}\n\nSource: ${revision}\nExact tag checks: ${verified.html_url}\n\nVerify SHA256SUMS and signed provenance before use; see [release verification](https://github.com/${repository}/blob/${revision}/docs/operations/release.md).`,
      draft: true,
      prerelease: version.startsWith('0.') || version.includes('-'),
    }),
  });
  const uploadURL = new globalThis.URL(release.upload_url.replace(/\{.*$/, ''));
  if (
    uploadURL.protocol !== 'https:' ||
    uploadURL.hostname !== 'uploads.github.com' ||
    uploadURL.username ||
    uploadURL.password ||
    uploadURL.search ||
    uploadURL.hash ||
    uploadURL.pathname !== `/repos/${repository}/releases/${release.id}/assets`
  )
    throw new Error('Unexpected release upload destination; draft retained.');
  for (const name of names) {
    const path = join(releaseDirectory, name);
    const info = await lstat(path);
    const upload = await globalThis.fetch(
      `${uploadURL.href}?name=${encodeURIComponent(basename(name))}`,
      {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${process.env.GH_TOKEN ?? ''}`,
          'Content-Type': 'application/octet-stream',
          'Content-Length': String(info.size),
          'X-GitHub-Api-Version': '2022-11-28',
        },
        body: createReadStream(path),
        duplex: 'half',
        redirect: 'error',
        signal: globalThis.AbortSignal.timeout(300_000),
      },
    );
    if (!upload.ok) throw new Error(`Release upload failed (${upload.status}); draft retained.`);
  }
  await request(`releases/${release.id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ draft: false }),
  });
  process.stdout.write(`Published ${release.html_url}\n`);
} else {
  throw new Error('Expected release command: check-ci, metadata, checksums, or publish.');
}
