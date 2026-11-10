import { Buffer } from 'node:buffer';
import { spawnSync } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';
import { assertHostedProcessorEnvironment } from './check-hosted-processor-linux.mjs';

export const lifecycleCases = Object.freeze([
  'TestRunShutdownFencesMemoryThenJoinsActiveProducer',
  'TestConcurrentCloseAndCancellationCannotReadClearedOwner',
  'TestLateAdmissionCommitCannotRestoreClosedConversation',
  'TestExplicitCancelAndShutdownLeaveNoWorker',
  'TestCancelPendingAdmissionClosesReadinessWithoutReopeningMemory',
  'TestLifetimeLossClearsQueuedAndCompletedVolatileStateBeforeRun',
  'TestMemoryExpiryCancelsActiveWorkAndCannotBeReadBack',
  'TestUserRevocationAndReadAuthorizationFenceCompletedAnswer',
]);
export const lifecycleIterations = 100;
const packageName = 'github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice';

// Only project fixed names and counters. Test output can contain private text.
export function lifecycleResults(stdout) {
  if (typeof stdout !== 'string' || Buffer.byteLength(stdout) > 8 * 1024 * 1024)
    throw new Error('stage9_lifecycle.invalid_events');
  const cases = new Map(lifecycleCases.map((name) => [name, {
    name, started: 0, passed: 0, failed: 0, skipped: 0, elapsed_seconds: 0,
  }]));
  const active = new Set();
  let packagePassed = false;
  let unsafe = false;
  for (const line of stdout.split('\n').filter(Boolean)) {
    let event;
    try { event = JSON.parse(line); } catch { throw new Error('stage9_lifecycle.invalid_events'); }
    if (event === null || typeof event !== 'object' || event.Package !== packageName) throw new Error('stage9_lifecycle.invalid_events');
    if (event.Test === undefined) {
      if (['fail', 'skip'].includes(event.Action)) unsafe = true;
      if (event.Action === 'pass') {
        if (packagePassed || active.size > 0) unsafe = true;
        packagePassed = true;
      }
      continue;
    }
    if (typeof event.Test !== 'string') throw new Error('stage9_lifecycle.invalid_events');
    const parent = cases.get(event.Test.split('/')[0]);
    if (!parent || packagePassed) throw new Error('stage9_lifecycle.invalid_events');
    if (event.Action === 'run') {
      if (active.has(event.Test)) unsafe = true;
      if (event.Test !== parent.name && !active.has(parent.name)) unsafe = true;
      active.add(event.Test);
      if (event.Test === parent.name) parent.started++;
    } else if (['pass', 'fail', 'skip'].includes(event.Action)) {
      if (!active.delete(event.Test) || !Number.isFinite(event.Elapsed) || event.Elapsed < 0 || event.Elapsed > 600)
        throw new Error('stage9_lifecycle.invalid_events');
      if (event.Action !== 'pass') unsafe = true;
      if (event.Test === parent.name) {
        parent[event.Action === 'pass' ? 'passed' : event.Action === 'fail' ? 'failed' : 'skipped']++;
        parent.elapsed_seconds += event.Elapsed;
      }
    }
  }
  const observed = [...cases.values()];
  const passed = packagePassed && !unsafe && active.size === 0 && observed.every((item) =>
    item.started === lifecycleIterations && item.passed === lifecycleIterations && item.failed === 0 && item.skipped === 0);
  return { status: passed ? 'passed' : 'failed', package_passed: packagePassed, cases: observed };
}

async function main() {
  assertHostedProcessorEnvironment(process.platform, process.env);
  if (process.env.GITHUB_WORKFLOW !== 'stage9-lifecycle') throw new Error('stage9_lifecycle.workflow_required');
  const root = resolve(import.meta.dirname, '..');
  const report = {
    schema_version: 1, source_revision: process.env.GITHUB_SHA,
    scope: 'Repeated service boundary regressions with deterministic dependencies; not real processor, database restart, reconnect, soak or pilot acceptance',
    status: 'failed', release_accepted: false, iterations_per_case: lifecycleIterations,
    expected_observations: lifecycleCases.length * lifecycleIterations,
    race_detector: true, maximum_parallel_tests: 2, shuffle_seed: 214,
    timeout_seconds: 600, cases: [],
  };
  const started = Date.now();
  try {
    const result = spawnSync('go', [
      '-C', 'apps/api', 'test', './internal/assistantservice', '-race',
      '-run', `^(${lifecycleCases.join('|')})$`, `-count=${lifecycleIterations}`,
      '-parallel=2', '-shuffle=214', '-timeout=10m', '-json',
    ], { cwd: root, encoding: 'utf8', timeout: 660000, maxBuffer: 8 * 1024 * 1024 });
    const observed = lifecycleResults(result.stdout ?? '');
    report.cases = observed.cases;
    report.package_passed = observed.package_passed;
    report.completed_observations = observed.cases.reduce((sum, item) => sum + item.passed, 0);
    if (result.status !== 0 || observed.status !== 'passed') throw new Error('stage9_lifecycle.case_failed');
    report.status = 'passed';
  } catch (error) {
    report.error_code = /^stage9_lifecycle\.[a-z_]+$/u.test(error.message) ? error.message : 'stage9_lifecycle.operation_failed';
    process.exitCode = 1;
  } finally {
    report.elapsed_ms = Date.now() - started;
    await mkdir(resolve(root, '.tmp'), { recursive: true });
    await writeFile(resolve(root, '.tmp/stage9-lifecycle.json'), `${JSON.stringify(report, null, 2)}\n`, { flag: 'wx', mode: 0o600 });
  }
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) await main();
