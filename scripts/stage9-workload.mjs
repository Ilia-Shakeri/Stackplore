export function validateWorkload(load, inference, source) {
  if (load?.source_revision !== source || load.status !== 'collected' || load.maximum_p95_ratio !== 1.1 || inference?.source_revision !== source || inference.stopped !== true)
    throw new Error('stage9_workload.receipt');
  for (const sample of [load.idle, load.busy]) {
    if (sample?.requests !== 80 || sample.parallel !== 8 || sample.rounds !== 10 || sample.spacing_ms !== 600 || !Number.isSafeInteger(sample.started_epoch_ms) || !Number.isSafeInteger(sample.finished_epoch_ms) || sample.finished_epoch_ms - sample.started_epoch_ms < 6000 || sample.finished_epoch_ms - sample.started_epoch_ms > 120000 || !Number.isInteger(sample.latency_p95_ms) || sample.latency_p95_ms < 1 || sample.latency_p95_ms > 10000)
      throw new Error('stage9_workload.sample');
  }
  if (load.idle.finished_epoch_ms > load.busy.started_epoch_ms || !Array.isArray(inference.calls)) throw new Error('stage9_workload.order');
  const overlapping = inference.calls.filter((call) => call.complete === true && call.fault === 'none' && call.input_match === true && Number.isSafeInteger(call.started_epoch_ms) && call.started_epoch_ms < load.busy.finished_epoch_ms && call.started_epoch_ms + call.elapsed_ms > load.busy.started_epoch_ms);
  if (overlapping.length !== 1) throw new Error('stage9_workload.generation_overlap');
  const idleOverlap = inference.calls.some((call) => call.started_epoch_ms < load.idle.finished_epoch_ms && call.started_epoch_ms + call.elapsed_ms > load.idle.started_epoch_ms);
  if (idleOverlap) throw new Error('stage9_workload.idle_overlap');
  const ratio = load.busy.latency_p95_ms / load.idle.latency_p95_ms;
  return { status: ratio <= 1.1 ? 'passed' : 'failed', source_revision: source, scope: load.scope, requests: 160, actual_generation_overlap: true, p95_ratio: ratio, maximum_p95_ratio: 1.1, idle_p95_ms: load.idle.latency_p95_ms, busy_p95_ms: load.busy.latency_p95_ms, release_accepted: false };
}

export function validateSoak(soak, source) {
  if (soak?.source_revision !== source || soak.status !== 'passed' || soak.expected_cycles !== 100 || soak.completed_cycles !== 100 || soak.successful_reads !== 8000 || soak.health_checks !== 200 || soak.cleared_views !== 100 || !Number.isSafeInteger(soak.duration_ms) || soak.duration_ms < 3600000 || soak.duration_ms > 4200000 || !Array.isArray(soak.samples) || soak.samples.length !== 100)
    throw new Error('stage9_workload.soak_incomplete');
  let previous = -1;
  for (const [index, sample] of soak.samples.entries()) {
    if (sample.cycle !== index + 1 || !Number.isSafeInteger(sample.elapsed_ms) || sample.elapsed_ms <= previous || sample.elapsed_ms > soak.duration_ms || !Number.isInteger(sample.read_p95_ms) || sample.read_p95_ms < 1 || sample.read_p95_ms > 10000)
      throw new Error('stage9_workload.soak_sample');
    previous = sample.elapsed_ms;
  }
  return { status: 'passed', source_revision: source, duration_ms: soak.duration_ms, completed_cycles: 100, successful_reads: 8000, health_checks: 200, cleared_views: 100, scope: soak.scope, release_accepted: false };
}
