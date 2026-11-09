import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { URL } from 'node:url';
import test from 'node:test';
import { combineSeedEvaluations } from './combine-seed-evaluations.mjs';
import { hostedProfileIdentity } from './check-hosted-processor-linux.mjs';
import { summarizeSeedEvaluation } from './seed-evaluation.mjs';

const root = new URL('../tests/fixtures/assistant/', import.meta.url);
const manifest = JSON.parse(await readFile(new URL('corpus.manifest.json', root), 'utf8'));
const bytes = await readFile(new URL('corpus.seed.json', root));
const cases = JSON.parse(bytes.toString('utf8')).scenarios;
const revision = 'a'.repeat(40);
function fixture() {
  return [1, 2, 3].map((pass) => {
    const rows = cases.flatMap((item) =>
      ['en', 'fa'].map((locale) => ({
        case_id: item.id,
        locale,
        pass,
        status: 'completed',
        fault: 'none',
        input_tokens: 10,
        output_tokens: 5,
        elapsed_ms: 100,
        sections: 1,
        citations_valid: true,
        gap_section: false,
        generation_observed: true,
        generated_tokens: 5,
        generation_complete: true,
        input_match: true,
      })),
    );
    return {
      ...hostedProfileIdentity,
      source_revision: revision,
      status: 'passed',
      stopped: true,
      postgres_stopped: true,
      resources: { peak_rss_bytes: 1024, cpu_seconds: 1 },
      runner: {
        platform: 'linux',
        architecture: 'x64',
        logical_cpus: 4,
        processor: 'Test CPU',
        host_memory_bytes: 16384,
      },
      seed_evaluation: summarizeSeedEvaluation(manifest, bytes, rows, [pass]),
    };
  });
}
test('three disjoint hosted parts combine without pretending independent blind quality', () => {
  const combined = combineSeedEvaluations(manifest, bytes, fixture(), revision);
  assert.equal(combined.seed_evaluation.totals.completed, 72);
  assert.equal(combined.measurements.length, 3);
  assert.equal(combined.seed_evaluation.release_accepted, false);
});
test('missing, duplicate, stale, wrong-profile and unclean parts cannot be combined', () => {
  for (const change of [
    (parts) => {
      parts.pop();
    },
    (parts) => {
      parts[1] = parts[0];
    },
    (parts) => {
      parts[0].source_revision = 'b'.repeat(40);
    },
    (parts) => {
      parts[0].model_sha256 = 'b'.repeat(64);
    },
    (parts) => {
      parts[0].stopped = false;
    },
    (parts) => {
      parts[0].seed_evaluation.corpus_sha256 = 'b'.repeat(64);
    },
    (parts) => {
      parts[0].seed_evaluation.parameters.output_token_limit = 256;
    },
    (parts) => {
      parts[0].seed_evaluation.parameters.per_case_deadline_ms = 80000;
    },
    (parts) => {
      parts[0].resources.peak_rss_bytes = 9 * 1024 ** 3;
    },
    (parts) => {
      parts[0].seed_evaluation.observations.pop();
    },
  ]) {
    const parts = fixture();
    change(parts);
    assert.throws(() => combineSeedEvaluations(manifest, bytes, parts, revision), {
      message: 'seed_evaluation.invalid_parts',
    });
  }
});
