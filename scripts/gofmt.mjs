import { readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import process from 'node:process';
import { spawnSync } from 'node:child_process';
import { fileURLToPath, URL } from 'node:url';

const mode = process.argv[2];

if (mode !== '--check' && mode !== '--write') {
  process.stderr.write('Usage: node scripts/gofmt.mjs --check|--write\n');
  process.exit(2);
}

const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
const apiRoot = resolve(root, 'apps/api');

const findGoFiles = (directory) =>
  readdirSync(directory, { withFileTypes: true })
    .flatMap((entry) => {
      const path = resolve(directory, entry.name);
      return entry.isDirectory() ? findGoFiles(path) : path.endsWith('.go') ? [path] : [];
    })
    .sort();

const files = [...findGoFiles(apiRoot), ...findGoFiles(resolve(root, 'scripts'))];
const unformatted = [];
// Keep each command below the Windows process argument limit as the tree grows.
for (let offset = 0; offset < files.length; offset += 32) {
  const args = [mode === '--write' ? '-w' : '-l', ...files.slice(offset, offset + 32)];
  const result = spawnSync('gofmt', args, { encoding: 'utf8', timeout: 60_000 });
  if (result.error) {
    process.stderr.write(`Could not run gofmt: ${result.error.message}\n`);
    process.exit(1);
  }
  if (result.status !== 0) {
    process.stderr.write(result.stderr);
    process.exit(result.status ?? 1);
  }
  if (mode === '--check' && result.stdout.trim()) unformatted.push(result.stdout.trimEnd());
}
if (unformatted.length) {
  process.stderr.write(`Go files need formatting:\n${unformatted.join('\n')}\n`);
  process.exit(1);
}
