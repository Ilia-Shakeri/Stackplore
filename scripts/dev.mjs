import { existsSync } from 'node:fs';
import { resolve } from 'node:path';
import process from 'node:process';
import { fileURLToPath, URL } from 'node:url';

import concurrently from 'concurrently';

const repositoryRoot = fileURLToPath(new URL('..', import.meta.url));
const environmentPath = resolve(repositoryRoot, '.env');

if (existsSync(environmentPath)) {
  process.loadEnvFile(environmentPath);
}

process.env.STACKPLORE_PUBLIC_ORIGIN ??= 'http://127.0.0.1:5173';

const { result } = concurrently(
  [
    {
      command: 'go -C apps/api run ./cmd/server',
      name: 'api',
    },
    {
      command: 'corepack pnpm --filter @stackplore/web dev',
      name: 'web',
    },
  ],
  {
    cwd: repositoryRoot,
    killOthersOn: ['failure'],
    prefix: 'name',
  },
);

try {
  await result;
} catch {
  process.exitCode = 1;
}
