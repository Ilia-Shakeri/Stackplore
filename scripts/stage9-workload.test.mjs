import assert from 'node:assert/strict';
import test from 'node:test';
import { validateWorkload, validateSoak } from './stage9-workload.mjs';
import { processResources, validateResourceSamples } from './stage9-resources.mjs';

const source = 'a'.repeat(40);
const sample = (started, p95) => ({ requests: 80, parallel: 8, rounds: 10, spacing_ms: 600, started_epoch_ms: started, finished_epoch_ms: started + 6000, latency_p95_ms: p95 });
const fixture = () => ({ load: { source_revision: source, status: 'collected', maximum_p95_ratio: 1.1, idle: sample(100000, 20), busy: sample(110000, 22) }, inference: { source_revision: source, stopped: true, calls: [{ started_epoch_ms: 111000, elapsed_ms: 20000, complete: true, input_match: true, fault: 'none' }] } });
test('actual overlap and fixed workload denominators precede the unchanged ten-percent threshold', () => {
  const { load, inference } = fixture();
  assert.equal(validateWorkload(load, inference, source).status, 'passed');
  load.busy.latency_p95_ms = 23;
  assert.equal(validateWorkload(load, inference, source).status, 'failed');
});
test('unproven overlap, source, counts, cleanup and truncated samples reject', () => {
  for (const change of [
    (value) => { value.load.source_revision = 'b'.repeat(40); },
    (value) => { value.load.busy.requests = 79; },
    (value) => { value.load.busy.finished_epoch_ms = 115999; },
    (value) => { value.inference.stopped = false; },
    (value) => { value.inference.calls[0].complete = false; },
    (value) => { value.inference.calls[0].started_epoch_ms = 117000; },
    (value) => { value.inference.calls.push({ ...value.inference.calls[0] }); },
    (value) => { value.inference.calls.push({ ...value.inference.calls[0], started_epoch_ms: 99000 }); },
  ]) {
    const value = fixture(); change(value);
    assert.throws(() => validateWorkload(value.load, value.inference, source), /stage9_workload/u);
  }
});
const soakFixture = () => ({ source_revision: source, status: 'passed', expected_cycles: 100, completed_cycles: 100, successful_reads: 8000, health_checks: 200, cleared_views: 100, duration_ms: 3600000, samples: Array.from({ length: 100 }, (_, i) => ({ cycle: i + 1, elapsed_ms: (i + 1) * 36000, read_p95_ms: 20 })) });
test('a real hour and all one hundred distinct cycle receipts are mandatory', () => {
  assert.equal(validateSoak(soakFixture(), source).status, 'passed');
  for (const change of [
    (value) => { value.duration_ms = 3599999; },
    (value) => { value.successful_reads = 7999; },
    (value) => { value.samples.pop(); },
    (value) => { value.samples[1].cycle = 1; },
    (value) => { value.samples[1].elapsed_ms = 0; },
    (value) => { value.health_checks = 199; },
  ]) {
    const value = soakFixture(); change(value);
    assert.throws(() => validateSoak(value, source), /stage9_workload/u);
  }
});

test('owned process samples reject invalid measurements, memory and descriptor growth', () => {
  assert.deepEqual(processResources('VmRSS: 1024 kB\nVmHWM: 2048 kB\n', 5), { rss_bytes: 1048576, peak_rss_bytes: 2097152, descriptors: 5 });
  assert.throws(() => processResources('VmRSS: 0 kB\nVmHWM: 0 kB\n', 5));
  const samples = Array.from({ length: 121 }, (_, index) => ({ elapsed_ms: index * 30000, application: { rss_bytes: 30 * 1024 * 1024, peak_rss_bytes: 40 * 1024 * 1024, descriptors: 12 }, processor: { rss_bytes: 3 * 1024 * 1024 * 1024, peak_rss_bytes: 3 * 1024 * 1024 * 1024, descriptors: 12 } }));
  assert.equal(validateResourceSamples(samples, true).status, 'passed');
  assert.throws(() => validateResourceSamples(samples.slice(1), true), /duration/u);
  samples.at(-1).application.descriptors = 100;
  assert.throws(() => validateResourceSamples(samples, true), /growth/u);
  samples.at(-1).application.descriptors = 12;
  samples.at(-1).application.peak_rss_bytes = 513 * 1024 * 1024;
  assert.throws(() => validateResourceSamples(samples, true), /budget/u);
});
