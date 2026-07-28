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

const files = findGoFiles(apiRoot);
const args = mode === '--write' ? ['-w', ...files] : ['-l', ...files];
const result = spawnSync('gofmt', args, { encoding: 'utf8' });

if (result.error) {
  process.stderr.write(`Could not run gofmt: ${result.error.message}\n`);
  process.exit(1);
}

if (result.status !== 0) {
  process.stderr.write(result.stderr);
  process.exit(result.status ?? 1);
}

if (mode === '--check' && result.stdout.trim()) {
  process.stderr.write(`Go files need formatting:\n${result.stdout}`);
  process.exit(1);
}
