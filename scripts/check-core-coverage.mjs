import { spawnSync } from 'node:child_process';
import process from 'node:process';
import { URL } from 'node:url';

for (const name of ['domain', 'alerting']) {
  const result = spawnSync('go', ['-C', 'apps/api', 'test', `./internal/${name}`, '-cover'], {
    cwd: new URL('..', import.meta.url),
    encoding: 'utf8',
    timeout: 180_000,
  });
  const output = `${result.stdout ?? ''}${result.stderr ?? ''}`;
  process.stdout.write(output);
  const percent = Number(output.match(/coverage:\s*([\d.]+)%/u)?.[1]);
  if (result.status !== 0 || !Number.isFinite(percent) || percent < 80) {
    throw new Error(`${name} must pass with at least 80 percent measured statement coverage`);
  }
}
