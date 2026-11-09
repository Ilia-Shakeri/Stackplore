import { hostedProfileIdentity } from './check-hosted-processor-linux.mjs';
import { summarizeSeedEvaluation } from './seed-evaluation.mjs';

function reject() { throw new Error('seed_evaluation.invalid_parts'); }
export function combineSeedEvaluations(manifest, bytes, parts, revision) {
  if (typeof revision !== 'string' || revision.length !== 40 || !/^[a-f0-9]{40}$/u.test(revision) ||
      !Array.isArray(parts) || parts.length !== 3) reject();
  const seen = new Set();
  const observations = [];
  const measurements = [];
  for (const part of parts) {
    const passes = part?.seed_evaluation?.parameters?.pass_ids;
    if (part?.source_revision !== revision || part.status !== 'passed' ||
        part.stopped !== true || part.postgres_stopped !== true ||
        !Array.isArray(passes) || passes.length !== 1 || ![1, 2, 3].includes(passes[0]) || seen.has(passes[0]) ||
        Object.entries(hostedProfileIdentity).some(([key, value]) => part[key] !== value)) reject();
    seen.add(passes[0]);
    const summary = summarizeSeedEvaluation(manifest, bytes, part.seed_evaluation.observations, passes);
    if (summary.totals.missing !== 0 || summary.totals.not_run !== 0) reject();
    const usage = part.resources;
    const runner = part.runner;
    if (!usage || !Number.isSafeInteger(usage.peak_rss_bytes) || usage.peak_rss_bytes < 1 ||
        usage.peak_rss_bytes > 8 * 1024 ** 3 || !Number.isFinite(usage.cpu_seconds) || usage.cpu_seconds < 0 ||
        !runner || runner.platform !== 'linux' || runner.architecture !== 'x64' ||
        !Number.isInteger(runner.logical_cpus) || runner.logical_cpus < 1 || runner.logical_cpus > 256 ||
        typeof runner.processor !== 'string' || runner.processor.trim() !== runner.processor ||
        !/^[\w ().@+-]{1,128}$/u.test(runner.processor) ||
        !Number.isSafeInteger(runner.host_memory_bytes) || runner.host_memory_bytes < 1) reject();
    measurements.push({ pass: passes[0], peak_rss_bytes: usage.peak_rss_bytes, cpu_seconds: usage.cpu_seconds,
      runner: { platform: runner.platform, architecture: runner.architecture, logical_cpus: runner.logical_cpus,
        processor: runner.processor, host_memory_bytes: runner.host_memory_bytes } });
    observations.push(...part.seed_evaluation.observations);
  }
  return { schema_version: 1, source_revision: revision, scope: 'three independent hosted executions of public development material; not blind or human acceptance',
    profile: { ...hostedProfileIdentity }, measurements: measurements.sort((a, b) => a.pass - b.pass),
    seed_evaluation: summarizeSeedEvaluation(manifest, bytes, observations) };
}
