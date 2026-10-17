import { readFile } from 'node:fs/promises';
import process from 'node:process';
import { URL } from 'node:url';

const root = new URL('..', import.meta.url);
const read = (path) => readFile(new URL(path, root), 'utf8');
const main = async () => {
  const manifest = JSON.parse(await read('package.json'));
  const version = manifest.version;
  if (!/^\d+\.\d+\.\d+(?:-[a-z0-9.-]+)?$/.test(version)) throw new Error('Invalid release version');
  const runtime = await read('apps/api/internal/config/audit_policy.go');
  if (runtime.match(/\bApplicationVersion\s*=\s*"([^"]+)"/u)?.[1] !== version)
    throw new Error('Runtime application version drift');
  for (const path of [
    'apps/web/package.json',
    'packages/ui/package.json',
    'packages/api-client/package.json',
  ]) {
    if (JSON.parse(await read(path)).version !== version) throw new Error(`Version drift: ${path}`);
  }
  const contract = await read('contracts/openapi.yaml');
  if (contract.match(/^ {2}version: (.+)$/m)?.[1] !== version)
    throw new Error('Contract version drift');
  const chart = await read('deploy/helm/stackplore/Chart.yaml');
  if (
    chart.match(/^version: (.+)$/m)?.[1] !== version ||
    chart.match(/^appVersion: '(.+)'$/m)?.[1] !== version
  ) {
    throw new Error('Chart version drift');
  }
  const values = await read('deploy/helm/stackplore/values.yaml');
  const tags = [...values.matchAll(/^ {4}tag: '(.+)'$/gm)].map((match) => match[1]);
  if (tags.length !== 1 || tags.some((tag) => tag !== version))
    throw new Error('Default image version drift');
  if (!(await read('CHANGELOG.md')).includes(`## [${version}]`))
    throw new Error('Release changelog missing');
  process.stdout.write(`Release version ${version} matches all manifests.\n`);
};

try {
  await main();
} catch (error) {
  process.stderr.write(`${error instanceof Error ? error.message : 'Version check failed'}\n`);
  process.exitCode = 1;
}
