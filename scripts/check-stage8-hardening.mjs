import { spawnSync } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import process from 'node:process';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { assertHostedProcessorEnvironment } from './check-hosted-processor-linux.mjs';

export const hardeningCases = Object.freeze({
  inference: [
    'TestStage8MalformedOutputMatrixNeverRetriesOrPublishes',
    'TestFailedDispatchCannotBeRetried',
    'TestGatewayRejectsUnsafeCountAndGeneration',
    'TestGatewayBoundsCancellationAndSafeErrors',
    'TestConcurrentDispatchHasOneWinner',
  ],
  'provider/inference': [
    'TestLocalRejectsMalformedUnknownAndOversizedProtocol',
    'TestLocalRejectsIdentityChangeBeforeDispatch',
    'TestLocalCancellationStopsOneActualDispatch',
    'TestLocalRechecksAndPinsDNSWithoutFollowingRedirect',
    'TestLocalDeadlineDNSBoundsAndNoAmbientProxy',
  ],
  assistantservice: [
    'TestRunShutdownFencesMemoryThenJoinsActiveProducer',
    'TestLateAdmissionCommitCannotRestoreClosedConversation',
    'TestUserRevocationAndReadAuthorizationFenceCompletedAnswer',
  ],
  postgres: [
    'TestAssistantNativeCrashBoundaries',
    'TestAssistantHotPathQueryPlansAndRetentionBoundsOnRealPostgreSQL',
    'TestAssistantUnknownSettlementCannotRefundConfirmedCalls',
    'TestAssistantInferenceRequiresReservedRemainingUnitsBeforeDispatch',
    'TestAssistantStatusFailureRollsBackAdmissionAndStepTransactions',
    'TestAssistantRestoreGenerationReadKeepsCurrentFence',
  ],
});

export function hardeningResults(stdout) {
  const expected = Object.entries(hardeningCases).flatMap(([pkg, names]) =>
    names.map((name) => ({
      package: `github.com/Ilia-Shakeri/stackplore/apps/api/internal/${pkg}`,
      name,
    })),
  );
  const observed = new Map();
  const children = new Map();
  const terminal = new Set();
  for (const line of stdout.split('\n').filter(Boolean)) {
    let event;
    try {
      event = JSON.parse(line);
    } catch {
      throw new Error('stage8_hardening.invalid_events');
    }
    if (!['pass', 'fail', 'skip'].includes(event.Action) || typeof event.Test !== 'string')
      continue;
    const parent = expected.find(
      (item) => item.package === event.Package && item.name === event.Test.split('/')[0],
    );
    if (!parent) continue;
    const key = JSON.stringify([event.Package, event.Test]);
    if (
      terminal.has(key) ||
      !Number.isFinite(event.Elapsed) ||
      event.Elapsed < 0 ||
      event.Elapsed > 360
    )
      throw new Error('stage8_hardening.invalid_events');
    terminal.add(key);
    if (event.Test !== parent.name) {
      const parentKey = JSON.stringify([parent.package, parent.name]);
      const counts = children.get(parentKey) ?? {
        subcases: 0,
        skipped_subcases: 0,
        failed_subcases: 0,
      };
      counts.subcases++;
      if (event.Action === 'skip') counts.skipped_subcases++;
      if (event.Action === 'fail') counts.failed_subcases++;
      children.set(parentKey, counts);
      continue;
    }
    observed.set(key, { status: event.Action, elapsed_seconds: event.Elapsed });
  }
  return expected.map((item) => ({
    ...item,
    ...(observed.get(JSON.stringify([item.package, item.name])) ?? {
      status: 'missing',
      elapsed_seconds: null,
    }),
    ...(children.get(JSON.stringify([item.package, item.name])) ?? {
      subcases: 0,
      skipped_subcases: 0,
      failed_subcases: 0,
    }),
  }));
}

async function main() {
  assertHostedProcessorEnvironment(process.platform, process.env);
  const root = resolve(import.meta.dirname, '..');
  const report = {
    schema_version: 1,
    source_revision: process.env.GITHUB_SHA,
    scope:
      'selected real database and deterministic fault regressions; not full security or pilot acceptance',
    status: 'failed',
    cases: [],
    fuzz: 'not_run',
  };
  try {
    const names = Object.values(hardeningCases).flat();
    const tested = spawnSync(
      'go',
      [
        '-C',
        'apps/api',
        'test',
        ...Object.keys(hardeningCases).map((name) => `./internal/${name}`),
        '-run',
        `^(${names.join('|')})$`,
        '-count=1',
        '-timeout=6m',
        '-json',
      ],
      { cwd: root, encoding: 'utf8', timeout: 390000, maxBuffer: 4 * 1024 * 1024 },
    );
    report.cases = hardeningResults(tested.stdout ?? '');
    if (
      tested.status !== 0 ||
      report.cases.some(
        (item) => item.status !== 'pass' || item.skipped_subcases > 0 || item.failed_subcases > 0,
      ) ||
      report.cases.find(
        (item) => item.name === 'TestStage8MalformedOutputMatrixNeverRetriesOrPublishes',
      )?.subcases !== 72 ||
      report.cases.find((item) => item.name === 'TestAssistantNativeCrashBoundaries')?.subcases !==
        3 ||
      (report.cases.find(
        (item) => item.name === 'TestAssistantHotPathQueryPlansAndRetentionBoundsOnRealPostgreSQL',
      )?.subcases ?? 0) < 7
    )
      throw new Error('stage8_hardening.case_failed');
    const fuzzStarted = Date.now();
    const fuzzed = spawnSync(
      'go',
      [
        '-C',
        'apps/api',
        'test',
        './internal/inference',
        '-run',
        '^$',
        '-fuzz',
        '^FuzzStage8ClosedAnswerBoundary$',
        '-fuzztime=10s',
        '-parallel=1',
        '-timeout=1m',
      ],
      { cwd: root, timeout: 75000, maxBuffer: 1024 * 1024 },
    );
    report.fuzz = fuzzed.status === 0 ? 'passed' : 'failed';
    report.fuzz_budget_seconds = 10;
    report.fuzz_elapsed_ms = Date.now() - fuzzStarted;
    if (fuzzed.status !== 0) throw new Error('stage8_hardening.fuzz_failed');
    report.status = 'passed';
  } catch (error) {
    report.error_code = /^stage8_hardening\.[a-z_]+$/u.test(error.message)
      ? error.message
      : 'stage8_hardening.operation_failed';
    process.exitCode = 1;
  } finally {
    await mkdir(resolve(root, '.tmp'), { recursive: true });
    await writeFile(
      resolve(root, '.tmp/stage8-hardening.json'),
      `${JSON.stringify(report, null, 2)}\n`,
      { flag: 'wx', mode: 0o600 },
    );
  }
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) await main();
