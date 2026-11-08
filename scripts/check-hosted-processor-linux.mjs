import { Buffer } from 'node:buffer';
import { spawn, spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import {
  chmod,
  lstat,
  mkdir,
  mkdtemp,
  readFile,
  readdir,
  realpath,
  writeFile,
} from 'node:fs/promises';
import { createServer } from 'node:net';
import { isAbsolute, relative, resolve, sep } from 'node:path';
import process from 'node:process';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';

const archiveHash = 'ef0db60597825a63534a536bd60d3c290cf82631b2a48461278dc2c9eb436c25';
const modelHash = '061b54daade076b5d3362dac252678d17da8c68f07560be70818cace6590cb1a';
const templateHash = '57f1fd00f0013a2be96aa79b857391f27e23df5b5f847072b524c897e24d0361';
const modelRevision = '90862c4b9d2787eaed51d12237eafdfe7c5f6077';
const build = 'b11729-412a363c6';

export function validHostedIncidentReportMetadata(value) {
  if (
    value?.report_current !== true ||
    value?.report_cleared !== true ||
    !Number.isInteger(value.report_rows)
  )
    return false;
  if (value.goal === 'explain_resource')
    return value.report_rows === 0 && value.report_status === 0;
  return (
    value.goal === 'investigate_incident' &&
    value.report_rows >= 1 &&
    value.report_rows <= 64 &&
    value.report_status === 200
  );
}

// Metadata-only SQL acceptance: four explicitly admitted runs, no uncertain work.
export function validHostedDatabaseMetadata(value, source) {
  if (
    value?.status !== 'observed' ||
    value.source_revision !== source ||
    !Array.isArray(value.runs) ||
    value.runs.length !== 4 ||
    !Array.isArray(value.steps) ||
    value.steps.length !== 20
  )
    return false;
  for (const goal of ['explain_resource', 'investigate_incident']) {
    const runs = value.runs.filter((run) => run.goal === goal);
    if (
      runs.length !== 2 ||
      runs.some(
        (run) =>
          run.state !== 'completed' ||
          run.tool_calls !== (goal === 'explain_resource' ? 1 : 7) ||
          run.inference_calls !== 1 ||
          run.usage_unknown !== false ||
          run.usage_conservative !== false,
      )
    )
      return false;
  }
  const counts = {
    inventory: 4,
    health: 2,
    metrics: 2,
    events: 2,
    topology: 2,
    alert_history: 2,
    workload_history: 2,
    inference: 4,
  };
  for (const [kind, count] of Object.entries(counts)) {
    if (value.steps.filter((step) => step.kind === kind).length !== count) return false;
  }
  return value.steps.every(
    (step) =>
      step.state === 'completed' &&
      (step.kind === 'inference'
        ? Number.isInteger(step.units) &&
          Number.isInteger(step.maximum_units) &&
          step.units > 0 &&
          step.maximum_units <= 20096 &&
          step.units <= step.maximum_units
        : step.units === 0 && step.maximum_units === 0),
  );
}

// Guard before filesystem, network or subprocess work. Imported tests do no I/O.
export function assertHostedProcessorEnvironment(platform, environment) {
  if (
    platform !== 'linux' ||
    environment.GITHUB_ACTIONS !== 'true' ||
    environment.RUNNER_ENVIRONMENT !== 'github-hosted' ||
    !/^[a-f0-9]{40}$/u.test(environment.GITHUB_SHA ?? '') ||
    !isAbsolute(environment.RUNNER_TEMP ?? '')
  ) {
    throw new Error('hosted_processor.runner_required');
  }
}

export function safeArchiveNames(text) {
  const names = text.trim().split('\n');
  if (
    names.length < 1 ||
    names.length > 128 ||
    names.some(
      (name) =>
        !/^[A-Za-z0-9_./+-]+$/u.test(name) ||
        name.startsWith('/') ||
        name.split('/').includes('..'),
    )
  ) {
    throw new Error('hosted_processor.archive_paths');
  }
  return names;
}

// Reject link escapes and special entries before extraction, including hard links.
export function safeArchiveLinks(text) {
  for (const line of text.trim().split('\n')) {
    if (!['-', 'd', 'l'].includes(line[0])) {
      throw new Error('hosted_processor.archive_entry');
    }
    if (line[0] === 'l') {
      const parts = line.split(' -> ');
      if (parts.length !== 2) throw new Error('hosted_processor.archive_link');
      safeArchiveNames(parts[1]);
    }
  }
}

async function main() {
  assertHostedProcessorEnvironment(process.platform, process.env);
  const root = resolve(import.meta.dirname, '..');
  const temporary = await realpath(process.env.RUNNER_TEMP);
  const work = await mkdtemp(resolve(temporary, 'stackplore-profile-'));
  const output = resolve(root, '.tmp/hosted-processor.json');
  await mkdir(resolve(root, '.tmp'), { recursive: true });
  const report = {
    schema_version: 1,
    source_revision: process.env.GITHUB_SHA,
    recorded_at: new Date().toISOString(),
    platform: 'linux-x64',
    status: 'failed',
    phase: 'archive',
    stopped: false,
    postgres_stopped: false,
    scope:
      'Real bilingual protocol and desktop application with synthetic Demo evidence; not Windows runtime or general quality acceptance',
    archive_sha256: archiveHash,
    model_sha256: modelHash,
    model_revision: modelRevision,
    model_license: 'Apache-2.0',
    model_bytes: 1834426016,
    context_tokens: 20480,
    call_timeout_seconds: 40,
    build,
    template_sha256: templateHash,
    cases: [],
    evidence_probes: [],
    empty_probes: [],
  };
  let child;
  let exited;
  const run = (command, args, options = {}) => {
    const result = spawnSync(command, args, {
      cwd: root,
      timeout: 180_000,
      maxBuffer: 4 * 1024 * 1024,
      ...options,
    });
    if (result.status !== 0) throw new Error('hosted_processor.subprocess_failed');
    return result.stdout?.toString() ?? '';
  };
  const download = async (url, path, size, digest) => {
    const transfer = (source, headers = []) =>
      run(
        'curl',
        [
          '--fail',
          '--silent',
          '--location',
          '--proto',
          '=https',
          '--proto-redir',
          '=https',
          '--max-redirs',
          '5',
          '--retry',
          '2',
          '--retry-all-errors',
          '--retry-delay',
          '5',
          '--retry-max-time',
          '300',
          '--connect-timeout',
          '10',
          '--max-time',
          '300',
          '--max-filesize',
          String(size),
          '--output',
          path,
          ...headers,
          source,
        ],
        { timeout: 330_000, stdio: 'ignore' },
      );
    try {
      transfer(url);
    } catch (error) {
      // The same immutable release asset has a second official transport.
      if (
        url !==
        'https://github.com/ggml-org/llama.cpp/releases/download/b11729/llama-b11729-bin-ubuntu-x64.tar.gz'
      )
        throw error;
      transfer('https://api.github.com/repos/ggml-org/llama.cpp/releases/assets/656081240', [
        '--header',
        'Accept: application/octet-stream',
      ]);
    }
    const item = await lstat(path);
    if (!item.isFile() || item.size !== size) throw new Error('hosted_processor.asset_size');
    const hash = createHash('sha256');
    for await (const chunk of createReadStream(path)) hash.update(chunk);
    if (hash.digest('hex') !== digest) throw new Error('hosted_processor.asset_digest');
  };
  const getJSON = async (path) => {
    const response = await globalThis.fetch(`http://127.0.0.1:58102${path}`, {
      redirect: 'error',
      signal: globalThis.AbortSignal.timeout(1000),
    });
    if (!response.ok) throw new Error('hosted_processor.not_ready');
    const chunks = [];
    let bytes = 0;
    for await (const chunk of response.body) {
      bytes += chunk.length;
      if (bytes > 65536) throw new Error('hosted_processor.metadata_size');
      chunks.push(chunk);
    }
    return JSON.parse(Buffer.concat(chunks).toString('utf8'));
  };
  const metadata = async (name) => {
    const path = resolve(root, '.tmp', name);
    const item = await lstat(path);
    if (!item.isFile() || item.size > 65536) throw new Error('hosted_processor.metadata_size');
    const value = JSON.parse(await readFile(path, 'utf8'));
    if (value.source_revision !== process.env.GITHUB_SHA)
      throw new Error('hosted_processor.receipt_source');
    return value;
  };
  try {
    const archive = resolve(work, 'processor.tar.gz');
    const model = resolve(work, 'model.gguf');
    const runtime = resolve(work, 'runtime');
    await download(
      'https://github.com/ggml-org/llama.cpp/releases/download/b11729/llama-b11729-bin-ubuntu-x64.tar.gz',
      archive,
      17973345,
      archiveHash,
    );
    report.phase = 'model';
    await download(
      `https://huggingface.co/Qwen/Qwen3-1.7B-GGUF/resolve/${modelRevision}/Qwen3-1.7B-Q8_0.gguf?download=true`,
      model,
      1834426016,
      modelHash,
    );
    report.phase = 'extract';
    safeArchiveNames(run('tar', ['-tzf', archive]));
    safeArchiveLinks(run('tar', ['-tvzf', archive]));
    await mkdir(runtime, { mode: 0o700 });
    run('tar', ['-xzf', archive, '-C', runtime, '--no-same-owner', '--no-same-permissions']);
    const executables = [];
    let entries = 0;
    const inspect = async (directory) => {
      for (const entry of await readdir(directory, { withFileTypes: true })) {
        if (++entries > 128) throw new Error('hosted_processor.archive_inventory');
        const path = resolve(directory, entry.name);
        const target = await realpath(path);
        if (
          relative(runtime, target).startsWith(`..${sep}`) ||
          relative(runtime, target) === '..' ||
          isAbsolute(relative(runtime, target))
        )
          throw new Error('hosted_processor.archive_link');
        if (entry.isDirectory()) await inspect(path);
        else if (!entry.isFile() && !entry.isSymbolicLink())
          throw new Error('hosted_processor.archive_entry');
        if (entry.isFile() && entry.name === 'llama-server') executables.push(path);
      }
    };
    await inspect(runtime);
    if (executables.length !== 1) throw new Error('hosted_processor.executable');
    const executable = executables[0];
    await chmod(executable, 0o700);
    await new Promise((accept, reject) => {
      const reservation = createServer();
      reservation.once('error', reject);
      reservation.listen(58102, '127.0.0.1', () => reservation.close(accept));
    });
    report.phase = 'start';
    child = spawn(
      executable,
      [
        '--model',
        model,
        '--alias',
        `sha256-${modelHash}`,
        '--host',
        '127.0.0.1',
        '--port',
        '58102',
        '--cors-origins',
        'http://127.0.0.1:58102',
        '--parallel',
        '1',
        '--ctx-size',
        '20480',
        '--threads',
        '4',
        '--threads-batch',
        '4',
        '--threads-http',
        '2',
        '--device',
        'none',
        '--gpu-layers',
        '0',
        '--fit',
        'off',
        '--cache-type-k',
        'q8_0',
        '--cache-type-v',
        'q8_0',
        '--flash-attn',
        'on',
        '--batch-size',
        '512',
        '--ubatch-size',
        '256',
        '--predict',
        '4096',
        '--timeout',
        '45',
        '--offline',
        '--no-webui',
        '--no-agent',
        '--no-ui-mcp-proxy',
        '--no-slots',
        '--no-models-autoload',
        '--no-mmproj',
        '--no-cache-prompt',
        '--cache-ram',
        '0',
        '--no-cache-idle-slots',
        '--no-context-shift',
        '--ctx-checkpoints',
        '0',
        '--no-cont-batching',
        '--reasoning',
        'off',
        '--log-disable',
      ],
      {
        cwd: work,
        stdio: 'ignore',
        env: {
          PATH: process.env.PATH,
          LANG: 'C.UTF-8',
          LD_LIBRARY_PATH: resolve(executable, '..'),
        },
      },
    );
    exited = new Promise((accept) => {
      child.once('exit', accept);
      child.once('error', accept);
    });
    let ready = false;
    for (let attempt = 0; attempt < 60 && !ready; attempt++) {
      if (child.exitCode !== null) throw new Error('hosted_processor.start_failed');
      try {
        ready = (await getJSON('/health')).status === 'ok';
      } catch {
        /* Only readiness state; no body is emitted. */
      }
      if (!ready) await delay(1000);
    }
    if (!ready) throw new Error('hosted_processor.start_failed');
    const props = await getJSON('/props');
    const models = await getJSON('/v1/models');
    if (
      props.build_info !== build ||
      props.total_slots !== 1 ||
      props.default_generation_settings?.n_ctx !== 20480 ||
      createHash('sha256')
        .update(props.chat_template ?? '')
        .digest('hex') !== templateHash ||
      models.data?.length !== 1 ||
      models.data[0].id !== `sha256-${modelHash}`
    )
      throw new Error('hosted_processor.identity');
    const environment = {
      ...process.env,
      STACKPLORE_TEST_LOCAL_INFERENCE: 'true',
      STACKPLORE_TEST_INFERENCE_ORIGIN: 'http://127.0.0.1:58102',
      STACKPLORE_TEST_INFERENCE_ALIAS: `sha256-${modelHash}`,
      STACKPLORE_TEST_INFERENCE_BUILD: build,
      STACKPLORE_TEST_INFERENCE_TEMPLATE_SHA256: templateHash,
      STACKPLORE_TEST_INFERENCE_CONTEXT_TOKENS: '20480',
      STACKPLORE_TEST_INFERENCE_CALL_TIMEOUT: '40s',
    };
    report.phase = 'protocol';
    const tested = spawnSync(
      'go',
      [
        '-C',
        resolve(root, 'apps/api'),
        'test',
        './internal/provider/inference',
        '-run',
        '^TestPinnedLocal(Runtime|Evidence)Integration$',
        '-count=1',
        '-timeout=3m',
        '-json',
      ],
      {
        cwd: root,
        env: environment,
        timeout: 190_000,
        maxBuffer: 4 * 1024 * 1024,
        encoding: 'utf8',
      },
    );
    const events = (tested.stdout ?? '')
      .trim()
      .split('\n')
      .filter(Boolean)
      .map((line) => JSON.parse(line));
    for (const parent of [
      'TestPinnedLocalRuntimeIntegration',
      'TestPinnedLocalEvidenceIntegration',
    ]) {
      for (const name of [`${parent}/en`, `${parent}/fa`, parent]) {
        const results = events.filter(
          (event) => event.Test === name && ['pass', 'fail', 'skip'].includes(event.Action),
        );
        report.cases.push({
          name,
          status: results.length === 1 ? results[0].Action : 'missing_or_duplicate',
          elapsed_seconds: results[0]?.Elapsed ?? 0,
        });
      }
    }
    for (const [prefix, key, fields] of [
      [
        'evidence_probe=',
        'evidence_probes',
        [
          'locale',
          'backend_fault',
          'gateway_fault',
          'input_tokens',
          'output_tokens',
          'input_match',
          'complete',
          'json_valid',
          'sections',
          'citations_valid',
          'identity_matches',
          'identity_contained',
          'facts_kind',
          'text_bytes',
        ],
      ],
      [
        'empty_probe=',
        'empty_probes',
        [
          'locale',
          'gateway_fault',
          'sections',
          'gaps_kind',
          'empty_citations',
          'text_exact',
          'text_contains',
          'text_bytes',
        ],
      ],
    ]) {
      for (const event of events) {
        const offset = event.Output?.indexOf(prefix) ?? -1;
        if (offset >= 0) {
          const flags = JSON.parse(event.Output.slice(offset + prefix.length).trim());
          report[key].push(Object.fromEntries(fields.map((field) => [field, flags[field]])));
        }
      }
    }
    if (
      tested.status !== 0 ||
      report.cases.some((value) => value.status !== 'pass') ||
      report.evidence_probes.length !== 2 ||
      report.empty_probes.length !== 2
    )
      throw new Error('hosted_processor.protocol_failed');
    report.phase = 'web_build';
    run('corepack', ['pnpm', 'run', 'build:web'], { stdio: 'ignore' });
    run('node', ['scripts/embed-web-assets.mjs'], { stdio: 'ignore' });
    report.phase = 'application_browser';
    const browser = spawnSync('node', ['scripts/browser-gate.mjs'], {
      cwd: root,
      stdio: 'ignore',
      timeout: 600_000,
      env: {
        ...process.env,
        SP_BROWSER_MODES: 'normal-en,normal-fa',
        SP_BROWSER_REAL_ASSISTANT: 'true',
        SP_BROWSER_WORK_ROOT: temporary,
      },
    });
    report.application_browser = {
      cases: [],
      journeys: [],
      database_metadata: [],
      inference_metadata: [],
    };
    for (const locale of ['en', 'fa']) {
      const results = JSON.parse(
        await readFile(resolve(root, `.tmp/browser-results-normal-${locale}.json`), 'utf8'),
      );
      report.application_browser.cases.push({
        locale,
        ...Object.fromEntries(
          ['expected', 'unexpected', 'flaky', 'skipped'].map((key) => [key, results.stats[key]]),
        ),
      });
      for (const role of ['administrator', 'viewer']) {
        for (const goal of ['explain_resource', 'investigate_incident']) {
          try {
            const journey = await metadata(`hosted-live-${locale}-${role}-${goal}.json`);
            report.application_browser.journeys.push(
              Object.fromEntries(
                [
                  'locale',
                  'role',
                  'goal',
                  'coverage_valid',
                  'citation_count',
                  'admissions',
                  'admission_status',
                  'run_state',
                  'content_state',
                  'resource_identity_supported',
                  'identity_contained',
                  'identity_text_bytes',
                  'identity_shape',
                  'citation_current',
                  'citation_status',
                  'report_current',
                  'report_cleared',
                  'report_rows',
                  'report_status',
                  'cleared',
                  'view_state',
                ].map((key) => [key, journey[key]]),
              ),
            );
          } catch {
            /* A missing role or goal fails the complete journey below. */
          }
        }
      }
      report.application_browser.database_metadata.push(
        await metadata(`hosted-database-normal-${locale}.json`),
      );
      report.application_browser.inference_metadata.push(
        await metadata(`hosted-inference-normal-${locale}.json`),
      );
    }
    if (
      browser.status !== 0 ||
      report.application_browser.database_metadata.some(
        (value) => !validHostedDatabaseMetadata(value, process.env.GITHUB_SHA),
      ) ||
      report.application_browser.journeys.length !== 8 ||
      report.application_browser.journeys.some(
        (value) =>
          !validHostedIncidentReportMetadata(value) ||
          !value.coverage_valid ||
          value.admissions !== 1 ||
          value.admission_status !== 202 ||
          value.run_state !== 'completed' ||
          value.content_state !== 'ready' ||
          !value.resource_identity_supported ||
          !value.citation_current ||
          value.citation_status !== 200 ||
          !value.cleared,
      ) ||
      report.application_browser.cases.some(
        (value) =>
          value.expected !== 1 ||
          value.unexpected !== 0 ||
          value.flaky !== 0 ||
          value.skipped !== 0,
      )
    )
      throw new Error('hosted_processor.browser_failed');
    report.status = 'passed';
    report.phase = 'complete';
  } catch (error) {
    report.error_code = /^hosted_processor\.[a-z_]+$/u.test(error.message)
      ? error.message
      : 'hosted_processor.operation_failed';
    process.exitCode = 1;
  } finally {
    if (!child || child.pid === undefined) report.stopped = true;
    else {
      if (child.exitCode === null) child.kill('SIGTERM');
      await Promise.race([exited, delay(10_000)]);
      if (child.exitCode === null) {
        child.kill('SIGKILL');
        await Promise.race([exited, delay(5000)]);
      }
      report.stopped = child.exitCode !== null || child.signalCode !== null;
    }
    const stopped = spawnSync('bash', ['scripts/start-test-postgres.sh', 'stop', 'code'], {
      cwd: root,
      stdio: 'ignore',
      timeout: 60_000,
    });
    report.postgres_stopped = stopped.status === 0;
    if (!report.stopped || !report.postgres_stopped) {
      report.status = 'failed';
      report.cleanup_error_code = 'hosted_processor.cleanup_failed';
      process.exitCode = 1;
    }
    await writeFile(output, `${JSON.stringify(report, null, 2)}\n`, { flag: 'wx', mode: 0o600 });
  }
  if (report.status !== 'passed') globalThis.console.error('hosted_processor.verification_failed');
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) await main();
