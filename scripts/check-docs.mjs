import { readFile, readdir, stat } from 'node:fs/promises';
import { dirname, resolve, relative } from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const required = [
  'README.md',
  'README.fa.md',
  'SPEC.md',
  'PLAN.md',
  'PROJECT_GUIDE.md',
  'CHANGELOG.md',
  'CONTRIBUTING.md',
  'SECURITY.md',
  'THIRD_PARTY_NOTICES.md',
  'docs/README.md',
  'docs/roadmap.md',
  'docs/roadmap.fa.md',
  'docs/codebase-audit.md',
  'docs/implementation-plan.md',
  'docs/architecture.md',
  'docs/security.md',
  'docs/development.md',
  'docs/configuration.md',
  'docs/testing.md',
  'docs/user-guide.md',
  'docs/user-guide.fa.md',
  'docs/operations/deployment.md',
  'docs/operations/account-recovery.md',
  'docs/operations/backup-restore.md',
  'docs/operations/troubleshooting.md',
  'docs/operations/upgrade-rollback.md',
  'docs/releasing.md',
  'contracts/openapi.yaml',
];

const markdownFiles = async (directory) => {
  const entries = await readdir(directory, { withFileTypes: true });
  const paths = [];
  for (const entry of entries) {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) paths.push(...(await markdownFiles(path)));
    else if (entry.name.endsWith('.md')) paths.push(path);
  }
  return paths;
};

const checkDocs = async () => {
  for (const path of required) {
    const info = await stat(resolve(root, path));
    if (!info.isFile() || info.size === 0) throw new Error(`Required document missing: ${path}`);
  }
  const rootFiles = (await readdir(root)).filter((name) => name.endsWith('.md'));
  const files = [
    ...rootFiles.map((name) => resolve(root, name)),
    ...(await markdownFiles(resolve(root, 'docs'))),
  ];
  let links = 0;
  for (const file of files) {
    const source = await readFile(file, 'utf8');
    for (const match of source.matchAll(/\[[^\]\n]*\]\(<?([^\s)>]+)>?\)/g)) {
      const target = match[1].split('#')[0];
      if (!target || /^[a-z][a-z0-9+.-]*:|^\/\//i.test(target)) continue;
      const path = resolve(dirname(file), decodeURIComponent(target));
      const fromRoot = relative(root, path);
      if (fromRoot.startsWith('..'))
        throw new Error(`Document link leaves repo: ${relative(root, file)}`);
      try {
        await stat(path);
      } catch {
        throw new Error(`Broken link in ${relative(root, file)}: ${target}`);
      }
      links++;
    }
  }
  process.stdout.write(`Documentation inventory and ${links} local links pass.\n`);
};

try {
  await checkDocs();
} catch (error) {
  process.stderr.write(
    `${error instanceof Error ? error.message : 'Documentation check failed'}\n`,
  );
  process.exitCode = 1;
}
