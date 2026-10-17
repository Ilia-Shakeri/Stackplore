import { spawnSync } from 'node:child_process';
import { dirname, resolve } from 'node:path';
import process from 'node:process';
import { URL } from 'node:url';

const requiredVersions = {
  corepack: '0.35.0',
  go: '1.26.9',
  node: '24.18.0',
  pnpm: '11.17.0',
};

const run = (command, args, options = {}) => {
  const label = options.label ?? command;
  const result = spawnSync(command, args, {
    cwd: new URL('..', import.meta.url),
    encoding: 'utf8',
    stdio: options.capture ? 'pipe' : 'inherit',
  });

  if (result.error) {
    throw new Error(`Could not run ${label}: ${result.error.message}`);
  }

  if (result.status !== 0) {
    if (options.capture) {
      process.stderr.write(result.stderr);
    }
    throw new Error(`${label} exited with status ${result.status}.`);
  }

  return options.capture ? result.stdout.trim() : '';
};

const assertVersion = (tool, actual, expected) => {
  if (actual !== expected) {
    throw new Error(`${tool} ${expected} is required; found ${actual}.`);
  }
};

const main = () => {
  assertVersion('Node.js', process.versions.node, requiredVersions.node);

  const isWindows = process.platform === 'win32';
  const corepackCommand = isWindows ? process.execPath : 'corepack';
  const corepackPrefix = isWindows
    ? [resolve(dirname(process.execPath), 'node_modules/corepack/dist/corepack.js')]
    : [];
  const runCorepack = (args, options = {}) =>
    run(corepackCommand, [...corepackPrefix, ...args], {
      ...options,
      label: 'Corepack',
    });

  const corepackVersion = runCorepack(['--version'], { capture: true });
  assertVersion('Corepack', corepackVersion, requiredVersions.corepack);

  const goVersionOutput = run('go', ['version'], { capture: true });
  const goVersion = /\bgo(\d+\.\d+\.\d+)\b/u.exec(goVersionOutput)?.[1];
  assertVersion('Go', goVersion ?? 'unknown', requiredVersions.go);

  const pnpmVersion = runCorepack(['pnpm', '--version'], { capture: true });
  assertVersion('pnpm', pnpmVersion, requiredVersions.pnpm);

  runCorepack(['pnpm', 'install', '--frozen-lockfile']);
  run('go', ['-C', 'apps/api', 'mod', 'download']);

  process.stdout.write('Bootstrap complete.\n');
};

try {
  main();
} catch (error) {
  const message = error instanceof Error ? error.message : String(error);
  process.stderr.write(`Bootstrap failed: ${message}\n`);
  process.exitCode = 1;
}
