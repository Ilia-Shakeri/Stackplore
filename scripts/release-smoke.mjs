import { spawn, spawnSync } from 'node:child_process';
import { Buffer } from 'node:buffer';
import { randomBytes } from 'node:crypto';
import { lstat, mkdtemp, readFile, realpath, rm, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import { isAbsolute, join, relative, resolve } from 'node:path';
import process from 'node:process';
import { setTimeout as delay } from 'node:timers/promises';

// This gate executes extracted release tools, never a source build or an existing database.
if (process.platform !== 'linux' || process.arch !== 'x64')
  throw new Error('Native archive smoke requires Linux amd64.');
if (!isAbsolute(process.env.RUNNER_TEMP ?? ''))
  throw new Error('Native archive smoke requires owned runner temporary storage.');
const temporaryRoot = await realpath(process.env.RUNNER_TEMP);
const root = resolve(import.meta.dirname, '..');
const version = JSON.parse(await readFile(join(root, 'package.json'), 'utf8')).version;
if (!/^\d+\.\d+\.\d+(?:-[a-z0-9.-]+)?$/u.test(version))
  throw new Error('Native archive smoke version is invalid.');
const revision = process.env.GITHUB_SHA;
if (!/^[a-f0-9]{40}$/u.test(revision ?? ''))
  throw new Error('Native archive smoke requires the release source revision.');
const admin = new globalThis.URL(process.env.SP_RELEASE_ADMIN_URL ?? '');
if (
  admin.protocol !== 'postgresql:' ||
  !['127.0.0.1', 'localhost'].includes(admin.hostname) ||
  admin.pathname !== '/postgres'
)
  throw new Error('Native archive smoke requires an explicit disposable loopback administrator.');
const container = process.env.SP_RELEASE_POSTGRES_CONTAINER;
if (container !== undefined && !/^[a-f0-9]{12,64}$/u.test(container))
  throw new Error('Native archive smoke PostgreSQL container identity is invalid.');
const environment = { PATH: process.env.PATH, LANG: 'C.UTF-8', TZ: 'UTC' };
const pgEnvironment = {
  ...environment,
  HOME: process.env.HOME,
  PGHOST: container ? '127.0.0.1' : admin.hostname,
  PGPORT: container ? '5432' : admin.port || '5432',
  PGUSER: decodeURIComponent(admin.username),
  PGPASSWORD: decodeURIComponent(admin.password),
  PGDATABASE: 'postgres',
};
function sql(statement) {
  const args = ['--no-psqlrc', '--set', 'ON_ERROR_STOP=on', '-c', statement];
  const result = spawnSync(
    container ? 'docker' : 'psql',
    container
      ? [
          'exec',
          ...['PGHOST', 'PGPORT', 'PGUSER', 'PGPASSWORD', 'PGDATABASE'].flatMap((key) => [
            '--env',
            key,
          ]),
          container,
          'psql',
          ...args,
        ]
      : args,
    { env: pgEnvironment, encoding: 'utf8', timeout: 30_000, maxBuffer: 64 * 1024 },
  );
  if (result.status !== 0) throw new Error('Native smoke disposable database command failed.');
}
const packageName = `stackplore_${version}_linux_amd64`;
const releaseDirectory = join(temporaryRoot, 'stackplore-release');
const archive = join(releaseDirectory, `${packageName}.tar.gz`);
const archiveInfo = await lstat(archive);
if (!archiveInfo.isFile() || archiveInfo.isSymbolicLink() || archiveInfo.nlink !== 1)
  throw new Error('Native smoke archive must be a regular release file.');
const work = await mkdtemp(join(temporaryRoot, 'stackplore-native-smoke-'));
const ownedDatabasePrefix = `stackplore_release_${randomBytes(8).toString('hex')}`;
const results = [];
const startedAt = new Date().toISOString();
let failure;
try {
  // Extract only named regular-tool candidates from the archive into a fresh owned directory.
  const extraction = spawnSync(
    'tar',
    [
      '-xzf',
      archive,
      '--no-same-owner',
      '--no-same-permissions',
      '--keep-old-files',
      '-C',
      work,
      ...['server', 'storage', 'operator'].map((name) => `${packageName}/stackplore-${name}`),
    ],
    { env: environment, encoding: 'utf8', timeout: 30_000, maxBuffer: 64 * 1024 },
  );
  if (extraction.status !== 0) throw new Error('Native smoke archive extraction failed.');
  const binaries = {};
  for (const name of ['server', 'storage', 'operator']) {
    const path = join(work, packageName, `stackplore-${name}`);
    const info = await lstat(path);
    if (!info.isFile() || info.isSymbolicLink() || info.nlink !== 1 || !(info.mode & 0o111))
      throw new Error('Native smoke archive tool is not a regular executable.');
    binaries[name] = path;
  }
  for (const name of ['storage', 'operator']) {
    const result = spawnSync(binaries[name], [], {
      cwd: work,
      env: environment,
      stdio: 'ignore',
      timeout: 5000,
    });
    if (result.status !== 2) throw new Error('Native smoke tool did not reject missing arguments.');
  }
  for (const mode of ['guest', 'normal']) {
    const databaseName = `${ownedDatabasePrefix}_${mode}`;
    if (!/^stackplore_release_[a-f0-9]{16}_(?:guest|normal)$/u.test(databaseName))
      throw new Error('Native smoke owned database identity is invalid.');
    const database = new globalThis.URL(admin);
    database.pathname = `/${databaseName}`;
    const listener = createServer();
    await new Promise((done, reject) => {
      listener.once('error', reject);
      listener.listen(0, '127.0.0.1', done);
    });
    const port = listener.address().port;
    await new Promise((done, reject) =>
      listener.close((error) => (error ? reject(error) : done())),
    );
    const origin = `http://127.0.0.1:${port}`;
    const proof = randomBytes(32).toString('base64url');
    const cookies = new Map();
    let server;
    let created = false;
    let modeFailure;
    let spawnFailed = false;
    let startupReason = 'unknown';
    let startupField = 'unknown';
    let stopped = false;
    let resourceCount = 0;
    const checks = [];
    const request = async (path, { status = 200, method = 'GET', body, headers = {} } = {}) => {
      const response = await globalThis.fetch(`${origin}${path}`, {
        method,
        headers: {
          Origin: origin,
          Accept: 'application/json',
          ...(cookies.size
            ? { Cookie: [...cookies].map(([key, value]) => `${key}=${value}`).join('; ') }
            : {}),
          ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
          ...headers,
        },
        ...(body === undefined ? {} : { body: JSON.stringify(body) }),
        redirect: 'error',
        signal: globalThis.AbortSignal.timeout(3000),
      });
      if (response.status !== status) {
        await response.body?.cancel();
        throw new Error(`Native smoke ${mode} HTTP status check failed (${response.status}).`);
      }
      if (mode === 'guest' && response.headers.has('set-cookie'))
        throw new Error('Native smoke guest response created a cookie.');
      for (const raw of response.headers.getSetCookie()) {
        const pair = raw.split(';', 1)[0];
        const separator = pair.indexOf('=');
        const key = pair.slice(0, separator);
        const value = pair.slice(separator + 1);
        if (!/^[a-zA-Z0-9_-]+$/u.test(key)) throw new Error('Native smoke cookie name is invalid.');
        if (value) cookies.set(key, value);
        else cookies.delete(key);
      }
      const reader = response.body?.getReader();
      const chunks = [];
      let size = 0;
      if (reader) {
        try {
          for (;;) {
            const chunk = await reader.read();
            if (chunk.done) break;
            size += chunk.value.byteLength;
            if (size > 1024 * 1024)
              throw new Error('Native smoke response exceeded its read bound.');
            chunks.push(Buffer.from(chunk.value));
          }
        } finally {
          await reader.cancel();
        }
      }
      return { text: Buffer.concat(chunks).toString('utf8'), headers: response.headers };
    };
    const json = async (path, options) => JSON.parse((await request(path, options)).text);
    try {
      sql(`CREATE DATABASE ${databaseName}`);
      created = true;
      server = spawn(binaries.server, [], {
        cwd: work,
        env: {
          ...environment,
          STACKPLORE_ENVIRONMENT: 'test',
          STACKPLORE_HTTP_ADDRESS: `127.0.0.1:${port}`,
          STACKPLORE_PUBLIC_ORIGIN: origin,
          STACKPLORE_DATABASE_URL: database.toString(),
          STACKPLORE_DATA_DIRECTORY: join(work, `${mode}-runtime`),
          STACKPLORE_HTTP_SHUTDOWN_TIMEOUT: '3s',
          STACKPLORE_MUTATIONS_ENABLED: 'false',
          STACKPLORE_DOCKER_WRITE_ENABLED: 'false',
          STACKPLORE_KUBERNETES_WRITE_ENABLED: 'false',
          ...(mode === 'guest'
            ? {
                STACKPLORE_DEMO_DEPLOYMENT: 'true',
                STACKPLORE_GUEST_DEMO: 'true',
                STACKPLORE_DEMO_DATA_DIRECTORY: join(work, 'data-guest'),
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
      let pendingLog = '';
      server.stderr.on('data', (chunk) => {
        pendingLog = (pendingLog + chunk.toString('utf8')).slice(-16_384);
        const lines = pendingLog.split('\n');
        pendingLog = lines.pop() ?? '';
        for (const line of lines) {
          try {
            const record = JSON.parse(line);
            if (typeof record.reason === 'string' && /^[a-z_]{1,64}$/u.test(record.reason))
              startupReason = record.reason;
            if (typeof record.field === 'string' && /^STACKPLORE_[A-Z_]{1,96}$/u.test(record.field))
              startupField = record.field;
          } catch {
            // Retain no raw output; only fixed reason and configuration-field codes escape.
          }
        }
      });
      server.once('error', () => {
        spawnFailed = true;
      });
      let ready = false;
      for (let attempt = 0; attempt < 100; attempt++) {
        if (spawnFailed || server.exitCode !== null)
          throw new Error(
            `Native smoke server exited before readiness (${startupReason}; ${startupField}).`,
          );
        try {
          const response = await globalThis.fetch(`${origin}/readyz`, {
            signal: globalThis.AbortSignal.timeout(1000),
          });
          ready = response.ok;
          await response.body?.cancel();
          if (ready) break;
        } catch {
          // Readiness retries are bounded and contain no server output.
        }
        await delay(200);
      }
      if (!ready) throw new Error('Native smoke server readiness timed out.');
      checks.push('readiness');
      await request('/healthz');
      checks.push('liveness');
      for (const path of ['/', '/explore/resources']) {
        const response = await request(path, { headers: { Accept: 'text/html' } });
        if (
          !response.headers.get('content-type')?.includes('text/html') ||
          !response.text.includes('Stackplore')
        )
          throw new Error('Native smoke embedded UI or SPA fallback failed.');
      }
      checks.push('embedded-ui', 'spa-fallback');
      if (mode === 'guest') {
        const sources = await json('/api/v1/sources');
        if (!Array.isArray(sources) || sources.length === 0)
          throw new Error('Native smoke guest source inventory is empty.');
        let resources;
        const inventoryDeadline = Date.now() + 30_000;
        do {
          resources = await json('/api/v1/resources');
          if (Array.isArray(resources.items) && resources.items.length > 0) break;
          await delay(500);
        } while (Date.now() < inventoryDeadline);
        if (!Array.isArray(resources.items) || resources.items.length === 0)
          throw new Error('Native smoke guest resource inventory is empty.');
        resourceCount = resources.items.length;
        checks.push('guest-sources', 'guest-resources', 'guest-no-cookie');
        for (const path of ['/api/v1/auth/session', '/api/v1/settings/application'])
          await request(path, { status: 403 });
        await request('/api/v1/operations/previews', { method: 'POST', body: {}, status: 403 });
        checks.push('guest-private-denied', 'guest-operation-denied');
      } else {
        const preflight = await json('/api/v1/auth/preflight', {
          method: 'POST',
          body: { purpose: 'setup' },
          status: 201,
        });
        if (typeof preflight.preauth_proof !== 'string')
          throw new Error('Native smoke setup preflight failed.');
        await json('/api/v1/setup', {
          method: 'POST',
          status: 201,
          headers: { 'X-Stackplore-Preauth': preflight.preauth_proof },
          body: {
            login_name: 'smoke_operator',
            display_name: 'Smoke Operator',
            password: `Release-7-${randomBytes(32).toString('base64url')}!`,
            bootstrap_proof: proof,
          },
        });
        await json('/api/v1/auth/session');
        const settings = await json('/api/v1/settings/application');
        if (
          settings.application_version !== version ||
          settings.mutation_mode?.enabled !== false ||
          settings.mutation_mode?.docker_write_enabled !== false ||
          settings.mutation_mode?.kubernetes_write_enabled !== false
        )
          throw new Error('Native smoke version or disabled operation gates do not match.');
        checks.push(
          'setup-preflight',
          'initial-admin',
          'authenticated-session',
          'version',
          'all-operation-gates-disabled',
        );
      }
    } catch (error) {
      modeFailure = error;
    } finally {
      if (server && !spawnFailed && server.exitCode === null) {
        server.kill('SIGTERM');
        const exited = await Promise.race([
          new Promise((done) => server.once('exit', () => done(true))),
          delay(5000).then(() => false),
        ]);
        stopped = exited && server.exitCode === 0;
        if (!exited) {
          server.kill('SIGKILL');
          await Promise.race([new Promise((done) => server.once('exit', done)), delay(2000)]);
        }
      }
      if (created) {
        try {
          sql(`DROP DATABASE ${databaseName} WITH (FORCE)`);
        } catch {
          modeFailure ??= new Error('Native smoke owned database cleanup failed.');
        }
      }
      if (!stopped) modeFailure ??= new Error('Native smoke graceful server shutdown failed.');
      cookies.clear();
    }
    if (modeFailure) throw modeFailure;
    results.push({
      mode,
      checks,
      resourceCount,
      gracefulShutdown: stopped,
      disposableDatabaseRemoved: true,
    });
  }
} catch (error) {
  failure = error;
} finally {
  const ownedPath = relative(temporaryRoot, work);
  if (
    !ownedPath.startsWith('stackplore-native-smoke-') ||
    ownedPath.includes('..') ||
    isAbsolute(ownedPath)
  ) {
    failure ??= new Error('Native smoke cleanup path escaped runner storage.');
  } else {
    try {
      await rm(work, { recursive: true, force: true });
    } catch {
      failure ??= new Error('Native smoke private fixture cleanup failed.');
    }
  }
}
if (failure) throw failure;
await writeFile(
  join(releaseDirectory, 'native-smoke.json'),
  `${JSON.stringify(
    {
      schema: 1,
      version,
      revision,
      platform: 'linux/amd64',
      archive: `${packageName}.tar.gz`,
      startedAt,
      completedAt: new Date().toISOString(),
      toolUsageChecks: ['storage-rejects-missing-arguments', 'operator-rejects-missing-arguments'],
      modes: results,
      limits: [
        'No native execution of other cross-built targets.',
        'Backup/restore is a separate recovery gate.',
      ],
    },
    null,
    2,
  )}\n`,
);
process.stdout.write('Extracted Linux amd64 archive smoke passed; sanitized receipt retained.\n');
