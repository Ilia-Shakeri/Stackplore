import { Buffer } from 'node:buffer';
import { createHash } from 'node:crypto';
import { validateCorpus } from './check-v2-corpus.mjs';

const faults = new Set([
  'none',
  'invalid_output',
  'output_limit',
  'input_limit',
  'identity',
  'cancelled',
  'uncertain',
  'unavailable',
  'budget',
]);
const keys = [
  'case_id',
  'locale',
  'pass',
  'status',
  'fault',
  'input_tokens',
  'output_tokens',
  'elapsed_ms',
  'sections',
  'citations_valid',
  'gap_section',
];
function reject() {
  throw new Error('seed_evaluation.invalid_metadata');
}
function percentile(values, fraction) {
  if (!values.length) return null;
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[Math.max(0, Math.ceil(sorted.length * fraction) - 1)];
}

// Public development material cannot establish blind or human-reviewed quality.
export function summarizeSeedEvaluation(manifest, bytes, observations, selectedPasses = [1, 2, 3]) {
  validateCorpus(manifest, bytes);
  const corpus = JSON.parse(bytes.toString('utf8'));
  if (!Array.isArray(selectedPasses) || selectedPasses.length < 1 || selectedPasses.length > 3 ||
      new Set(selectedPasses).size !== selectedPasses.length || selectedPasses.some((pass) => ![1, 2, 3].includes(pass))) reject();
  if (!Array.isArray(observations) || observations.length > corpus.scenarios.length * 2 * selectedPasses.length) reject();
  const cases = new Map(corpus.scenarios.map((item) => [item.id, item]));
  const seen = new Set();
  for (const row of observations) {
    if (
      !row ||
      typeof row !== 'object' ||
      Array.isArray(row) ||
      Object.keys(row).length !== keys.length ||
      keys.some((key) => !Object.hasOwn(row, key)) ||
      !cases.has(row.case_id) ||
      !['en', 'fa'].includes(row.locale) ||
      !selectedPasses.includes(row.pass) ||
      !['completed', 'rejected', 'not_run'].includes(row.status) ||
      !faults.has(row.fault) ||
      !Number.isInteger(row.input_tokens) ||
      row.input_tokens < 0 ||
      row.input_tokens > 16000 ||
      !Number.isInteger(row.output_tokens) ||
      row.output_tokens < 0 ||
      row.output_tokens > 256 ||
      !Number.isInteger(row.elapsed_ms) ||
      row.elapsed_ms < 0 ||
      row.elapsed_ms > 80000 ||
      !Number.isInteger(row.sections) ||
      row.sections < 0 ||
      row.sections > 12 ||
      typeof row.citations_valid !== 'boolean' ||
      typeof row.gap_section !== 'boolean'
    )
      reject();
    if (
      row.status === 'completed'
        ? row.fault !== 'none' ||
          !row.citations_valid ||
          row.sections < 1 ||
          row.input_tokens < 1 ||
          row.output_tokens < 1
        : row.fault === 'none' || row.citations_valid || row.sections !== 0 || row.gap_section
    )
      reject();
    if (
      row.status === 'not_run' &&
      (row.fault !== 'budget' ||
        row.elapsed_ms !== 0 ||
        row.input_tokens !== 0 ||
        row.output_tokens !== 0)
    )
      reject();
    const key = JSON.stringify([row.case_id, row.locale, row.pass]);
    if (seen.has(key)) reject();
    seen.add(key);
  }
  const expected = corpus.scenarios.length * 2 * selectedPasses.length;
  const aggregate = (rows, expectedCount) => {
    const completed = rows.filter((row) => row.status === 'completed');
    const attempted = rows.filter((row) => row.status !== 'not_run');
    return {
      expected: expectedCount,
      recorded: rows.length,
      missing: expectedCount - rows.length,
      completed: completed.length,
      rejected: rows.filter((row) => row.status === 'rejected').length,
      not_run: rows.filter((row) => row.status === 'not_run').length,
      structural_completion_rate: expectedCount ? completed.length / expectedCount : null,
      latency_p50_ms: percentile(
        attempted.map((row) => row.elapsed_ms),
        0.5,
      ),
      latency_p95_ms: percentile(
        attempted.map((row) => row.elapsed_ms),
        0.95,
      ),
      validated_input_tokens: completed.reduce((sum, row) => sum + row.input_tokens, 0),
      validated_output_tokens: completed.reduce((sum, row) => sum + row.output_tokens, 0),
      uncertain_usage_cases: rows.filter((row) => row.status === 'rejected').length,
      gap_sections_observed: completed.filter((row) => row.gap_section).length,
    };
  };
  const totals = aggregate(observations, expected);
  return {
    schema_version: 1,
    scope: 'public-seed-development-only',
    parameters: {
      passes: selectedPasses.length,
      pass_ids: [...selectedPasses],
      output_token_limit: 256,
      per_case_deadline_ms: 40000,
      pass_deadline_ms: 1080000,
    },
    corpus_id: manifest.corpus_id,
    corpus_revision: manifest.revision,
    corpus_sha256: createHash('sha256').update(Buffer.from(bytes)).digest('hex'),
    passes: selectedPasses.map((pass) => ({
      pass,
      ...aggregate(
        observations.filter((row) => row.pass === pass),
        corpus.scenarios.length * 2,
      ),
    })),
    groups: ['demo', 'docker', 'kubernetes'].flatMap((provider) =>
      ['en', 'fa'].map((locale) => ({
        provider,
        locale,
        ...aggregate(
          observations.filter(
            (row) => row.locale === locale && cases.get(row.case_id).provider === provider,
          ),
              corpus.scenarios.filter((item) => item.provider === provider).length * selectedPasses.length,
        ),
      })),
    ),
    totals,
    structural_threshold_met: totals.completed === expected,
    human_grounding: 'pending',
    correct_abstention: 'pending',
    adversarial_profile_evaluation: 'pending',
    independent_holdout: false,
    release_accepted: false,
    observations: observations.map((row) => Object.fromEntries(keys.map((key) => [key, row[key]]))),
  };
}
