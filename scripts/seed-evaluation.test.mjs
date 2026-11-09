import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { URL } from 'node:url';
import { summarizeSeedEvaluation } from './seed-evaluation.mjs';
import { hostedAssistantOutputTokens } from './hosted-assistant-profile.mjs';

const root = new URL('../tests/fixtures/assistant/', import.meta.url);
const manifest = JSON.parse(await readFile(new URL('corpus.manifest.json', root), 'utf8'));
const bytes = await readFile(new URL('corpus.seed.json', root));
const corpus = JSON.parse(bytes.toString('utf8'));
const rows = () =>
  [1, 2, 3].flatMap((pass) =>
    corpus.scenarios.flatMap((item) =>
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
    ),
  );

test('three complete synthetic passes retain denominators and never claim human or blind acceptance', () => {
  const result = summarizeSeedEvaluation(manifest, bytes, rows());
  assert.equal(result.totals.expected, 72);
  assert.equal(result.parameters.output_token_limit, 512);
  assert.equal(result.parameters.output_token_limit, hostedAssistantOutputTokens);
  assert.equal(result.parameters.pass_deadline_ms, 1080000);
  assert.equal(result.totals.completed, 72);
  assert.equal(result.structural_threshold_met, true);
  assert.equal(result.release_accepted, false);
  assert.equal(result.independent_holdout, false);
  assert.equal(result.human_grounding, 'pending');
  assert.equal(result.correct_abstention, 'pending');
  assert.equal(result.groups.length, 6);
  assert.equal(result.observations.length, 72);
  assert.equal(result.observations[0].case_id, corpus.scenarios[0].id);
  assert.ok(result.passes.every((pass) => pass.expected === 24 && pass.completed === 24));
  assert.ok(
    result.groups.every((group) => group.expected > 0 && group.recorded === group.expected),
  );
});

test('rejections, missing results and budget exhaustion cannot disappear from quality denominator', () => {
  const values = rows();
  values[0] = {
    ...values[0],
    status: 'rejected',
    fault: 'uncertain',
    sections: 0,
    citations_valid: false,
    input_tokens: 0,
    output_tokens: 0,
    elapsed_ms: 40000,
  };
  values[1] = {
    ...values[1],
    status: 'not_run',
    fault: 'budget',
    generation_observed: false,
    generated_tokens: 0,
    generation_complete: false,
    input_match: false,
    sections: 0,
    citations_valid: false,
    input_tokens: 0,
    output_tokens: 0,
    elapsed_ms: 0,
  };
  values.pop();
  const result = summarizeSeedEvaluation(manifest, bytes, values);
  assert.equal(result.totals.expected, 72);
  assert.equal(result.totals.rejected, 1);
  assert.equal(result.totals.not_run, 1);
  assert.equal(result.totals.missing, 1);
  assert.equal(result.totals.uncertain_usage_cases, 1);
  assert.equal(result.totals.structural_completion_rate, 69 / 72);
  assert.equal(result.structural_threshold_met, false);
  assert.equal(result.totals.latency_p95_ms, 100);
  assert.equal(result.totals.validated_input_tokens, 690);
});

test('unknown, duplicate, extra-field and inconsistent measurements fail without echoing input', () => {
  for (const change of [
    (value) => {
      value.case_id = 'foreign_case';
    },
    (value) => {
      value.pass = 4;
    },
    (value) => {
      value.locale = 'de';
    },
    (value) => {
      value.output_tokens = 513;
    },
    (value) => {
      value.input_tokens = -1;
    },
    (value) => {
      value.elapsed_ms = Number.NaN;
    },
    (value) => {
      value.citations_valid = 'true';
    },
    (value) => {
      value.fault = 'unavailable';
    },
    (value) => {
      value.generation_complete = false;
    },
    (value) => {
      value.generation_observed = false;
    },
    (value) => {
      value.generated_tokens = value.output_tokens + 1;
    },
    (value) => {
      value.private_text = 'never echoed';
    },
  ]) {
    const values = rows();
    change(values[0]);
    assert.throws(() => summarizeSeedEvaluation(manifest, bytes, values), {
      message: 'seed_evaluation.invalid_metadata',
    });
  }
  const values = rows();
  values[1] = values[0];
  assert.throws(() => summarizeSeedEvaluation(manifest, bytes, values), {
    message: 'seed_evaluation.invalid_metadata',
  });
});

test('empty receipt reports absent passes and null timings without granting quality acceptance', () => {
  const result = summarizeSeedEvaluation(manifest, bytes, []);
  assert.equal(result.totals.missing, 72);
  assert.equal(result.totals.latency_p95_ms, null);
  assert.equal(result.structural_threshold_met, false);
  assert.equal(result.release_accepted, false);
});

test('one selected pass has its own complete denominator and rejects foreign-pass rows', () => {
  const selected = rows().filter((row) => row.pass === 2);
  const result = summarizeSeedEvaluation(manifest, bytes, selected, [2]);
  assert.equal(result.totals.expected, 24);
  assert.deepEqual(result.parameters.pass_ids, [2]);
  for (const passes of [[], [1, 1], [4]])
    assert.throws(() => summarizeSeedEvaluation(manifest, bytes, selected, passes), {
      message: 'seed_evaluation.invalid_metadata',
    });
});
