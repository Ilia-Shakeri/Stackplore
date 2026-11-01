import { spawn, spawnSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { createServer, request } from 'node:http';
import { isAbsolute, resolve } from 'node:path';
import { createRequire } from 'node:module';
import process from 'node:process';
import { hostedAssistantProfile } from './hosted-assistant-profile.mjs';

const root = resolve(import.meta.dirname, '..');
const assistantProfile = hostedAssistantProfile(process.env);
const realAssistant = assistantProfile.STACKPLORE_ASSISTANT_ENABLED === 'true';
const admin = new globalThis.URL(process.env.SP_BROWSER_ADMIN_URL ?? '');
if (admin.protocol !== 'postgresql:' || !['127.0.0.1', 'localhost'].includes(admin.hostname))
  throw new Error('Browser gate requires an explicit disposable loopback PostgreSQL administrator');
const runID = `${Date.now()}_${process.pid}`;
const workRoot = process.env.SP_BROWSER_WORK_ROOT ?? resolve(root, '.tmp');
if (!isAbsolute(workRoot)) throw new Error('Browser scratch directory must be absolute');
const work = resolve(workRoot, `stackplore_browser_${runID}`);
await mkdir(work, { recursive: true, mode: 0o700 });
const privateEnvironment = Object.fromEntries(
  Object.entries(process.env).filter(
    ([key]) =>
      !key.startsWith('STACKPLORE_') &&
      (!key.startsWith('SP_BROWSER_') || key === 'SP_BROWSER_EXECUTABLE'),
  ),
);
const pgEnvironment = {
  ...privateEnvironment,
  PGHOST: admin.hostname,
  PGPORT: admin.port || '5432',
  PGUSER: decodeURIComponent(admin.username),
  PGPASSWORD: decodeURIComponent(admin.password),
  PGDATABASE: admin.pathname.slice(1),
};
function sql(command) {
  const result = spawnSync('psql', ['--no-psqlrc', '--set', 'ON_ERROR_STOP=on', '-c', command], {
    env: pgEnvironment,
    encoding: 'utf8',
    timeout: 110_000,
  });
  if (result.status !== 0)
    throw new Error(
      result.error?.code === 'ETIMEDOUT'
        ? 'Disposable browser database command timed out'
        : 'Disposable browser database command failed',
    );
}

async function recordHostedDatabase(databaseName, mode) {
  const query = `SELECT json_build_object(
    'runs', COALESCE((SELECT json_agg(row_to_json(r)) FROM (
      SELECT goal, state, tool_calls, inference_calls, usage_unknown, usage_conservative
      FROM public.assistant_runs ORDER BY created_at_us LIMIT 8
    ) r), '[]'::json),
    'steps', COALESCE((SELECT json_agg(row_to_json(s)) FROM (
      SELECT kind, state, units, maximum_units
      FROM public.assistant_steps ORDER BY started_at_us LIMIT 32
    ) s), '[]'::json)
  )`;
  const result = spawnSync(
    'psql',
    ['--no-psqlrc', '--set', 'ON_ERROR_STOP=on', '--tuples-only', '--no-align', '-c', query],
    {
      env: { ...pgEnvironment, PGDATABASE: databaseName, PGOPTIONS: '-c statement_timeout=3000' },
      encoding: 'utf8',
      timeout: 5000,
      maxBuffer: 16_384,
    },
  );
  let snapshot = { status: 'unavailable' };
  if (result.status === 0) {
    try {
      snapshot = { status: 'observed', ...JSON.parse(result.stdout) };
    } catch {
      // No SQL error, row identity or private payload is emitted.
    }
  }
  await writeFile(
    resolve(root, '.tmp', `hosted-database-${mode}.json`),
    `${JSON.stringify({ source_revision: process.env.GITHUB_SHA, ...snapshot })}\n`,
    { mode: 0o600 },
  );
}
const binary = resolve(work, process.platform === 'win32' ? 'server.exe' : 'server');
const build = spawnSync(
  'go',
  ['-C', resolve(root, 'apps/api'), 'build', '-o', binary, './cmd/server'],
  {
    env: privateEnvironment,
    stdio: 'inherit',
    timeout: 180_000,
  },
);
if (build.status !== 0) throw new Error('Browser server build failed');
const version = JSON.parse(await readFile(resolve(root, 'package.json'), 'utf8')).version;
const requireWeb = createRequire(resolve(root, 'apps/web/package.json'));
const cli = resolve(requireWeb.resolve('@playwright/test/package.json'), '..', 'cli.js');
const requestedModes = (process.env.SP_BROWSER_MODES ?? 'guest,normal-en,normal-fa').split(',');
if (
  requestedModes.length === 0 ||
  requestedModes.some((mode) => !['guest', 'normal-en', 'normal-fa'].includes(mode)) ||
  new Set(requestedModes).size !== requestedModes.length
)
  throw new Error('Browser gate mode is invalid');
let exitCode = 0;
for (const mode of requestedModes) {
  const guest = mode === 'guest';
  const locale = mode === 'normal-fa' ? 'fa' : 'en';
  const ownedDatabase = `stackplore_browser_${runID}_${mode.replace('-', '_')}`;
  const database = new globalThis.URL(admin);
  database.pathname = `/${ownedDatabase}`;
  const port = guest ? 58097 : 58099;
  const serverPort = guest ? 58096 : port;
  const origin = `http://127.0.0.1:${port}`;
  const workerOverride = resolve(work, 'public-worker-update.js');
  const proof = guest ? '' : randomBytes(32).toString('base64url');
  let server;
  let proxy;
  let startupReason = 'unknown';
  let created = false;
  let failure;
  try {
    sql(`CREATE DATABASE ${ownedDatabase}`);
    created = true;
    if (guest) {
      const evidence = resolve(root, 'docs/evidence', version);
      await mkdir(evidence, { recursive: true });
      await writeFile(
        resolve(evidence, 'browser-measurements.json'),
        `${JSON.stringify({ version, startedAt: new Date().toISOString(), fixture: 'Disposable loopback PostgreSQL and synthetic guest data.' }, null, 2)}\n`,
      );
      proxy = createServer(async (incoming, outgoing) => {
        if (incoming.method === 'GET' && incoming.url === '/sw.js') {
          try {
            const script = await readFile(workerOverride);
            outgoing.writeHead(200, {
              'Content-Type': 'application/javascript',
              'Cache-Control': 'no-cache',
              'Service-Worker-Allowed': '/',
            });
            outgoing.end(script);
            return;
          } catch {
            /* No override means the production worker passes through unchanged. */
          }
        }
        const upstream = request(
          {
            hostname: '127.0.0.1',
            port: serverPort,
            method: incoming.method,
            path: incoming.url,
            headers: incoming.headers,
          },
          (response) => {
            outgoing.writeHead(response.statusCode ?? 502, response.headers);
            response.pipe(outgoing);
          },
        );
        upstream.on('error', () => {
          if (!outgoing.headersSent) outgoing.writeHead(502);
          outgoing.end();
        });
        incoming.on('aborted', () => upstream.destroy());
        outgoing.on('close', () => upstream.destroy());
        incoming.pipe(upstream);
      });
      proxy.maxConnections = 32;
      await new Promise((done, reject) => {
        proxy.once('error', reject);
        proxy.listen(port, '127.0.0.1', done);
      });
    }
    server = spawn(binary, [], {
      cwd: root,
      env: {
        ...privateEnvironment,
        STACKPLORE_ENVIRONMENT: 'test',
        STACKPLORE_HTTP_ADDRESS: `127.0.0.1:${serverPort}`,
        STACKPLORE_PUBLIC_ORIGIN: origin,
        STACKPLORE_DATABASE_URL: database.toString(),
        STACKPLORE_DATA_DIRECTORY: resolve(work, `${mode}-runtime`),
        ...assistantProfile,
        ...(guest
          ? {
              STACKPLORE_DEMO_DEPLOYMENT: 'true',
              STACKPLORE_GUEST_DEMO: 'true',
              STACKPLORE_DEMO_DATA_DIRECTORY: resolve(work, 'guest'),
            }
          : {
              STACKPLORE_SETUP_BOOTSTRAP_PROOF: proof,
              STACKPLORE_SETUP_BOOTSTRAP_EXPIRES_AT: new Date(Date.now() + 30 * 60_000)
                .toISOString()
                .replace(/\.\d{3}Z$/u, 'Z'),
            }),
      },
      stdio: ['ignore', 'ignore', 'pipe'],
    });
    server.stderr.on('data', (chunk) => {
      for (const line of chunk.toString('utf8').slice(0, 16_384).split('\n')) {
        try {
          const record = JSON.parse(line);
          if (typeof record.reason === 'string' && /^[a-z_]{1,64}$/u.test(record.reason))
            startupReason = record.reason;
        } catch {
          /* Startup output stays private; only a bounded reason code may escape. */
        }
      }
    });
    let ready = false;
    for (let attempt = 0; attempt < 100; attempt++) {
      if (server.exitCode !== null)
        throw new Error(`Disposable browser server exited before readiness (${startupReason})`);
      try {
        ready = (
          await globalThis.fetch(`${origin}/readyz`, {
            signal: globalThis.AbortSignal.timeout(1000),
          })
        ).ok;
        if (ready) break;
      } catch {
        /* Startup has a bounded retry window. */
      }
      await new Promise((done) => globalThis.setTimeout(done, 200));
    }
    if (!ready) throw new Error('Disposable browser server not ready');
    const test = spawn(process.execPath, [cli, 'test', '--config', 'playwright.config.ts'], {
      cwd: resolve(root, 'apps/web'),
      env: {
        ...privateEnvironment,
        SP_BROWSER_ORIGIN: origin,
        SP_BROWSER_VERSION: version,
        SP_BROWSER_MODE: guest ? 'guest' : 'normal',
        SP_BROWSER_LOCALE: locale,
        SP_BROWSER_RESULTS: `../../.tmp/browser-results${guest ? '' : `-${mode}`}.json`,
        SP_BROWSER_OUTPUT: resolve(work, 'test-results', mode),
        ...(realAssistant ? { SP_BROWSER_REAL_ASSISTANT: 'true' } : {}),
        ...(guest ? { SP_BROWSER_WORKER_OVERRIDE: workerOverride } : {}),
        ...(guest ? {} : { SP_BROWSER_BOOTSTRAP_PROOF: proof }),
      },
      stdio: 'inherit',
    });
    const result = await new Promise((done) => {
      test.on('exit', (code) => done(code ?? 1));
      test.on('error', () => done(1));
    });
    if (result !== 0) exitCode = 1;
  } catch (error) {
    failure = error;
  } finally {
    if (proxy) {
      proxy.closeAllConnections();
      await new Promise((done) => proxy.close(done));
    }
    if (server && server.exitCode === null) {
      server.kill();
      await new Promise((done) => {
        server.once('exit', done);
        globalThis.setTimeout(done, 5000).unref();
      });
    }
    if (created) {
      if (realAssistant) {
        try {
          await recordHostedDatabase(ownedDatabase, mode);
        } catch {
          process.stderr.write('Hosted aggregate database metadata unavailable.\n');
        }
      }
      try {
        sql(`DROP DATABASE ${ownedDatabase} WITH (FORCE)`);
      } catch (error) {
        if (failure === undefined) failure = error;
        else process.stderr.write('Disposable browser database cleanup also failed.\n');
      }
    }
  }
  if (failure !== undefined) throw failure;
}
process.exitCode = exitCode;
