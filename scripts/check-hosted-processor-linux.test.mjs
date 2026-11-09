import assert from 'node:assert/strict';
import test from 'node:test';
import {
  assertHostedProcessorEnvironment,
  safeArchiveNames,
  safeArchiveLinks,
  validHostedDatabaseMetadata,
  validHostedGuidanceMetadata,
  validHostedIncidentReportMetadata,
  parseProcessMeasurement,
  safeSystemError,
} from './check-hosted-processor-linux.mjs';

test('system diagnostics expose only fixed codes and never private paths or messages', () => {
  assert.equal(safeSystemError({ code: 'EADDRINUSE', message: 'private path' }), 'EADDRINUSE');
  assert.equal(
    safeSystemError({ code: 'private path', message: 'private text' }),
    'not_classified',
  );
  assert.equal(safeSystemError(null), 'not_classified');
});

test('owned process measurement keeps RSS high-water and CPU ticks with spaced process names', () => {
  const fields = ['S', ...Array(10).fill('0'), '125', '75', ...Array(10).fill('0')];
  const stat = `123 (worker with space) ${fields.join(' ')}`;
  assert.deepEqual(parseProcessMeasurement('Name:\tworker\nVmHWM:\t2048 kB\n', stat, 100), {
    peak_rss_bytes: 2097152,
    cpu_user_ticks: 125,
    cpu_system_ticks: 75,
    ticks_per_second: 100,
    cpu_seconds: 2,
  });
  for (const [status, changedStat, ticks] of [
    ['', stat, 100],
    ['VmHWM: 0 kB', stat, 100],
    ['VmHWM: 2048 kB', 'invalid', 100],
    ['VmHWM: 2048 kB', stat, 0],
    ['VmHWM: 2048 kB', stat, Number.NaN],
  ])
    assert.throws(() => parseProcessMeasurement(status, changedStat, ticks), {
      message: 'hosted_processor.measurement_invalid',
    });
});

test('guidance receipts require exact source, role, locale and literal passing assertions', () => {
  const source = 'a'.repeat(40);
  const value = {
    source_revision: source,
    locale: 'fa',
    role: 'viewer',
    status: 'passed',
    consent_sent: true,
    document_current: true,
    not_facts: true,
    coverage_valid: true,
    guide_supported: true,
    witness_current: true,
    cleared: true,
  };
  assert.equal(validHostedGuidanceMetadata(value, source, 'fa', 'viewer'), true);
  for (const change of [
    { source_revision: 'b'.repeat(40) },
    { locale: 'en' },
    { role: 'administrator' },
    { consent_sent: 'true' },
    { witness_current: 1 },
    { not_facts: false },
    { cleared: null },
    { coverage_valid: false },
    { guide_supported: false },
  ]) {
    assert.equal(
      validHostedGuidanceMetadata({ ...value, ...change }, source, 'fa', 'viewer'),
      false,
    );
  }
});

test('real report receipt requires rendered current authorized content and owned clearing', () => {
  const value = {
    goal: 'investigate_incident',
    report_current: true,
    report_cleared: true,
    report_rows: 4,
    report_status: 200,
  };
  assert.equal(validHostedIncidentReportMetadata(value), true);
  for (const change of [
    { report_current: false },
    { report_current: 'true' },
    { report_cleared: false },
    { report_rows: 0 },
    { report_rows: 65 },
    { report_rows: 1.5 },
    { report_rows: '4' },
    { report_status: 503 },
    { goal: 'shell' },
  ]) {
    assert.equal(validHostedIncidentReportMetadata({ ...value, ...change }), false);
  }
  assert.equal(
    validHostedIncidentReportMetadata({
      ...value,
      goal: 'explain_resource',
      report_rows: 0,
      report_status: 0,
    }),
    true,
  );
  assert.equal(validHostedIncidentReportMetadata({ ...value, goal: 'explain_resource' }), false);
});

test('real investigation SQL proof requires exact reservations and known single dispatch', () => {
  const source = 'a'.repeat(40);
  const value = {
    source_revision: source,
    status: 'observed',
    runs: [
      'explain_resource',
      'investigate_incident',
      'explain_resource',
      'investigate_incident',
      'explain_resource',
      'explain_resource',
    ].map((goal, index) => ({
      goal,
      state: 'completed',
      tool_calls: goal === 'explain_resource' ? (index >= 4 ? 2 : 1) : 7,
      inference_calls: 1,
      usage_unknown: false,
      usage_conservative: false,
    })),
    steps: Object.entries({
      inventory: 6,
      health: 2,
      metrics: 2,
      events: 2,
      topology: 2,
      alert_history: 2,
      workload_history: 2,
      inference: 6,
      reviewed_document: 2,
    }).flatMap(([kind, count]) =>
      Array.from({ length: count }, () => ({
        kind,
        state: 'completed',
        units: kind === 'inference' ? 10 : 0,
        maximum_units: kind === 'inference' ? 16512 : 0,
      })),
    ),
  };
  assert.equal(validHostedDatabaseMetadata(value, source), true);
  for (const mutate of [
    (v) => {
      v.runs[0].usage_unknown = true;
    },
    (v) => {
      v.runs[1].tool_calls = 6;
    },
    (v) => {
      v.runs[1].inference_calls = 2;
    },
    (v) => {
      v.steps[0].maximum_units = 1;
    },
    (v) => {
      v.steps.pop();
    },
    (v) => {
      v.steps.at(-1).units = 20000;
    },
    (v) => {
      v.source_revision = 'b'.repeat(40);
    },
  ]) {
    const changed = globalThis.structuredClone(value);
    mutate(changed);
    assert.equal(validHostedDatabaseMetadata(changed, source), false);
  }
});

const hosted = {
  GITHUB_ACTIONS: 'true',
  RUNNER_ENVIRONMENT: 'github-hosted',
  GITHUB_SHA: 'a'.repeat(40),
  RUNNER_TEMP: '/tmp/owned-runner',
};

test('processor guard refuses workstation and incomplete source contexts before I/O', () => {
  assert.doesNotThrow(() => assertHostedProcessorEnvironment('linux', hosted));
  for (const [platform, changed] of [
    ['win32', {}],
    ['linux', { GITHUB_ACTIONS: undefined }],
    ['linux', { RUNNER_ENVIRONMENT: 'self-hosted' }],
    ['linux', { GITHUB_SHA: 'main' }],
    ['linux', { RUNNER_TEMP: 'relative' }],
    ['linux', { RUNNER_TEMP: undefined }],
  ]) {
    assert.throws(
      () => assertHostedProcessorEnvironment(platform, { ...hosted, ...changed }),
      /runner_required/u,
    );
  }
});

test('archive entries cannot escape the owned extraction root', () => {
  assert.deepEqual(safeArchiveNames('./bin/llama-server\n./bin/libllama.so.0\n'), [
    './bin/llama-server',
    './bin/libllama.so.0',
  ]);
  for (const listing of [
    '',
    '/tmp/escape',
    '../escape',
    'bin/../../escape',
    'bin\\escape',
    'file\nwith space',
    Array.from({ length: 129 }, () => 'file').join('\n'),
  ]) {
    assert.throws(() => safeArchiveNames(listing), /archive_paths/u);
  }
});

test('archive links and special files are checked before extraction', () => {
  const safeLink = 'lrwxrwxrwx owner/group 0 date bin/lib.so -> lib.so.0';
  assert.doesNotThrow(() => safeArchiveLinks(safeLink));
  for (const listing of [
    'lrwxrwxrwx owner/group 0 date bin/lib.so -> /tmp/outside',
    'lrwxrwxrwx owner/group 0 date bin/lib.so -> ../outside',
    'hrwxrwxrwx owner/group 0 date bin/file link to /tmp/outside',
    'crw-rw-rw- owner/group 0 date bin/device',
  ]) {
    assert.throws(() => safeArchiveLinks(listing));
  }
});
