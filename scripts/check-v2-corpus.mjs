import { Buffer } from 'node:buffer';
import { createHash } from 'node:crypto';
import { readFile, stat } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { pathToFileURL } from 'node:url';
import { TextDecoder } from 'node:util';

export const MAXIMUM_CORPUS_BYTES = 4 * 1024 * 1024;
export const CORPUS_DIRECTORY = 'tests/fixtures/assistant';
const providers = new Set(['demo', 'docker', 'kubernetes']);
const splits = ['development', 'validation', 'holdout'];
const vectors = new Set([
  'direct_instruction',
  'indirect_instruction',
  'tool_schema',
  'output_schema',
  'network',
  'authorization',
  'replay',
  'budget',
  'privacy',
  'lease',
  'restore',
  'context',
]);
const placements = new Set([
  'user_question',
  'resource_name',
  'log',
  'event',
  'runbook',
  'tool_result',
  'inference_result',
  'request',
  'network_response',
]);
const effects = new Set([
  'provider_write',
  'extra_tool',
  'cross_scope_read',
  'cross_owner_read',
  'credential_read',
  'external_transmission',
  'active_render',
  'fabricated_citation',
  'budget_overrun',
  'late_publication',
  'automatic_retry',
  'reservation_release',
]);

function reject(code) {
  throw new Error(`corpus invalid: ${code}`);
}

function closed(value, keys, code) {
  if (
    value === null ||
    typeof value !== 'object' ||
    Array.isArray(value) ||
    Object.keys(value).length !== keys.length ||
    keys.some((key) => !Object.hasOwn(value, key))
  ) {
    reject(code);
  }
}

function text(value, maximum, code) {
  if (
    typeof value !== 'string' ||
    !value.isWellFormed() ||
    value.trim() === '' ||
    Buffer.byteLength(value, 'utf8') > maximum ||
    [...value].some((character) => {
      const code = character.codePointAt(0);
      return (code < 32 && ![9, 10, 13].includes(code)) || code === 127;
    })
  ) {
    reject(code);
  }
}

function identifier(value, code) {
  if (typeof value !== 'string' || !/^[a-z][a-z0-9_-]{2,79}$/u.test(value)) reject(code);
}

function array(value, minimum, maximum, code) {
  if (!Array.isArray(value) || value.length < minimum || value.length > maximum) reject(code);
}

function paired(value, maximum, code) {
  closed(value, ['en', 'fa'], code);
  text(value.en, maximum, code);
  text(value.fa, maximum, code);
  if (!/[\u0600-\u06ff]/u.test(value.fa)) reject(`${code}_persian`);
}

function unique(values, code) {
  if (new Set(values).size !== values.length) reject(code);
}

function normalized(value) {
  return value.normalize('NFKC').toLowerCase().replace(/\s+/gu, ' ').trim();
}

function pairFingerprint(pair) {
  return JSON.stringify([normalized(pair.en), normalized(pair.fa)]);
}

function scenarioFingerprint(item) {
  const records = item.evidence.map((record) => ({
    type: record.type,
    observed_at: record.observed_at,
    availability: record.availability,
    completeness: record.completeness,
    facts: [...record.facts].sort((left, right) => left.key.localeCompare(right.key, 'en')),
  }));
  records.sort((left, right) => JSON.stringify(left).localeCompare(JSON.stringify(right), 'en'));
  return JSON.stringify([item.provider, item.category, item.expected.disposition, records]);
}

export function familySplit(family, salt) {
  identifier(family, 'family');
  identifier(salt, 'split_salt');
  const bucket =
    createHash('sha256')
      .update(`cause-family-sha256-v1\0${salt}\0${family}`)
      .digest()
      .readUInt32BE(0) % 100;
  return bucket < 60 ? 'development' : bucket < 80 ? 'validation' : 'holdout';
}

function validateReview(review) {
  closed(review, ['status', 'authors', 'reviewers', 'receipt'], 'review_shape');
  if (!['pending', 'accepted'].includes(review.status)) reject('review_status');
  array(review.authors, 1, 10, 'review_authors');
  array(review.reviewers, 0, 10, 'review_reviewers');
  for (const person of [...review.authors, ...review.reviewers])
    identifier(person, 'review_identity');
  unique([...review.authors, ...review.reviewers], 'review_independence');
  if (review.status === 'pending') {
    if (review.receipt !== null) reject('pending_review_receipt');
  } else if (
    review.reviewers.length < 2 ||
    typeof review.receipt !== 'string' ||
    !/^docs\/evidence\/v2\/[a-z0-9_-]+\.json$/u.test(review.receipt)
  ) {
    reject('accepted_review_receipt');
  }
}

function validateManifest(manifest) {
  closed(
    manifest,
    [
      'schema_version',
      'corpus_id',
      'revision',
      'status',
      'corpus_file',
      'sha256',
      'target',
      'split',
      'review',
    ],
    'manifest_shape',
  );
  if (manifest.schema_version !== 1) reject('manifest_version');
  identifier(manifest.corpus_id, 'corpus_id');
  identifier(manifest.revision, 'revision');
  if (!['seed', 'candidate'].includes(manifest.status)) reject('manifest_status');
  if (manifest.corpus_file !== 'corpus.seed.json') reject('corpus_file');
  if (typeof manifest.sha256 !== 'string' || !/^[a-f0-9]{64}$/u.test(manifest.sha256))
    reject('digest');
  closed(manifest.target, ['paired_scenarios', 'adversarial_cases'], 'target_shape');
  if (manifest.target.paired_scenarios !== 200 || manifest.target.adversarial_cases !== 1000) {
    reject('target_floor');
  }
  closed(manifest.split, ['algorithm', 'salt', 'percentages'], 'split_shape');
  if (manifest.split.algorithm !== 'cause-family-sha256-v1') reject('split_algorithm');
  identifier(manifest.split.salt, 'split_salt');
  closed(manifest.split.percentages, splits, 'split_percentages');
  if (
    manifest.split.percentages.development !== 60 ||
    manifest.split.percentages.validation !== 20 ||
    manifest.split.percentages.holdout !== 20
  ) {
    reject('split_percentages');
  }
  validateReview(manifest.review);
}

function validateCaseBase(item, salt) {
  identifier(item.id, 'case_id');
  identifier(item.cause_family, 'cause_family');
  if (item.split !== familySplit(item.cause_family, salt)) reject('family_split');
  if (!providers.has(item.provider)) reject('provider');
}

function validateEvidence(evidence) {
  closed(
    evidence,
    ['id', 'type', 'observed_at', 'availability', 'completeness', 'synthetic', 'facts'],
    'evidence_shape',
  );
  identifier(evidence.id, 'evidence_id');
  if (!['health', 'metric', 'event', 'log', 'topology', 'permission'].includes(evidence.type)) {
    reject('evidence_type');
  }
  if (
    typeof evidence.observed_at !== 'string' ||
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$/u.test(evidence.observed_at) ||
    !Number.isFinite(Date.parse(evidence.observed_at))
  ) {
    reject('evidence_time');
  }
  const observed = new Date(evidence.observed_at);
  if (
    observed.getUTCFullYear() < 1970 ||
    observed.toISOString() !== `${evidence.observed_at.slice(0, 23)}Z`
  ) {
    reject('evidence_time');
  }
  if (
    !['available', 'stale', 'unavailable', 'unsupported', 'permission_denied'].includes(
      evidence.availability,
    )
  ) {
    reject('evidence_availability');
  }
  if (!['complete', 'partial', 'unknown'].includes(evidence.completeness))
    reject('evidence_completeness');
  if (evidence.synthetic !== true) reject('synthetic_only');
  array(evidence.facts, 1, 24, 'facts');
  for (const fact of evidence.facts) {
    closed(fact, ['key', 'value'], 'fact_shape');
    identifier(fact.key, 'fact_key');
    if (typeof fact.value === 'string') text(fact.value, 2000, 'fact_value');
    else if (
      typeof fact.value !== 'boolean' &&
      (typeof fact.value !== 'number' ||
        !Number.isFinite(fact.value) ||
        Math.abs(fact.value) > Number.MAX_SAFE_INTEGER)
    ) {
      reject('fact_value');
    }
  }
  unique(
    evidence.facts.map((fact) => fact.key),
    'duplicate_fact',
  );
}

function validateScenario(item, salt) {
  closed(
    item,
    ['id', 'cause_family', 'split', 'provider', 'category', 'questions', 'evidence', 'expected'],
    'scenario_shape',
  );
  validateCaseBase(item, salt);
  if (
    !['health', 'resources', 'timeline', 'topology', 'permissions', 'metrics'].includes(
      item.category,
    )
  ) {
    reject('scenario_category');
  }
  paired(item.questions, 2000, 'questions');
  array(item.evidence, 1, 12, 'evidence');
  for (const evidence of item.evidence) validateEvidence(evidence);
  const evidenceIDs = item.evidence.map((evidence) => evidence.id);
  unique(evidenceIDs, 'duplicate_evidence');
  closed(
    item.expected,
    ['disposition', 'required_evidence_ids', 'assertions'],
    'expectation_shape',
  );
  if (!['grounded_answer', 'abstain'].includes(item.expected.disposition)) reject('disposition');
  array(item.expected.required_evidence_ids, 1, 12, 'required_evidence');
  unique(item.expected.required_evidence_ids, 'duplicate_required_evidence');
  if (item.expected.required_evidence_ids.some((id) => !evidenceIDs.includes(id)))
    reject('unknown_citation');
  array(item.expected.assertions, 1, 12, 'assertions');
  for (const assertion of item.expected.assertions) {
    closed(assertion, ['id', 'kind', 'statement'], 'assertion_shape');
    identifier(assertion.id, 'assertion_id');
    if (!['must_support', 'must_not_assert'].includes(assertion.kind)) reject('assertion_kind');
    paired(assertion.statement, 2000, 'assertion_statement');
  }
  unique(
    item.expected.assertions.map((assertion) => assertion.id),
    'duplicate_assertion',
  );
}

function validateAttack(item, salt) {
  closed(
    item,
    ['id', 'cause_family', 'split', 'provider', 'vector', 'placement', 'payload', 'expected'],
    'attack_shape',
  );
  validateCaseBase(item, salt);
  if (!vectors.has(item.vector)) reject('attack_vector');
  if (!placements.has(item.placement)) reject('attack_placement');
  paired(item.payload, 3000, 'attack_payload');
  closed(item.expected, ['decision', 'forbidden_effects'], 'attack_expectation');
  if (item.expected.decision !== 'deny_or_ignore') reject('attack_decision');
  array(item.expected.forbidden_effects, 1, effects.size, 'forbidden_effects');
  for (const effect of item.expected.forbidden_effects) {
    if (!effects.has(effect)) reject('forbidden_effect');
  }
  unique(item.expected.forbidden_effects, 'duplicate_forbidden_effect');
}

export function validateCorpus(manifest, corpusBytes, { release = false } = {}) {
  validateManifest(manifest);
  if (
    !Buffer.isBuffer(corpusBytes) ||
    corpusBytes.length === 0 ||
    corpusBytes.length > MAXIMUM_CORPUS_BYTES
  ) {
    reject('corpus_bytes');
  }
  if (createHash('sha256').update(corpusBytes).digest('hex') !== manifest.sha256)
    reject('digest_mismatch');
  let corpus;
  try {
    corpus = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(corpusBytes));
  } catch {
    reject('corpus_json');
  }
  closed(
    corpus,
    ['schema_version', 'corpus_id', 'revision', 'scenarios', 'adversarial_cases'],
    'corpus_shape',
  );
  if (
    corpus.schema_version !== 1 ||
    corpus.corpus_id !== manifest.corpus_id ||
    corpus.revision !== manifest.revision
  ) {
    reject('corpus_identity');
  }
  array(corpus.scenarios, 1, 2000, 'scenarios');
  array(corpus.adversarial_cases, 1, 10000, 'adversarial_cases');
  for (const scenario of corpus.scenarios) validateScenario(scenario, manifest.split.salt);
  for (const attack of corpus.adversarial_cases) validateAttack(attack, manifest.split.salt);
  unique(
    [...corpus.scenarios, ...corpus.adversarial_cases].map((item) => item.id),
    'duplicate_case',
  );
  unique(
    corpus.scenarios.map((item) => pairFingerprint(item.questions)),
    'duplicate_question',
  );
  unique(corpus.scenarios.map(scenarioFingerprint), 'duplicate_incident');
  unique(
    corpus.adversarial_cases.map((item) => pairFingerprint(item.payload)),
    'duplicate_attack',
  );

  const partitionCounts = Object.fromEntries(
    splits.map((split) => [split, { scenarios: 0, attacks: 0 }]),
  );
  for (const scenario of corpus.scenarios) partitionCounts[scenario.split].scenarios += 1;
  for (const attack of corpus.adversarial_cases) partitionCounts[attack.split].attacks += 1;
  const targetComplete =
    corpus.scenarios.length >= manifest.target.paired_scenarios &&
    corpus.adversarial_cases.length >= manifest.target.adversarial_cases;
  if (release) {
    if (manifest.status !== 'candidate') reject('release_status');
    if (!targetComplete) reject('release_target_incomplete');
    if (manifest.review.status !== 'accepted') reject('release_review_pending');
    for (const provider of providers) {
      for (const split of splits) {
        if (!corpus.scenarios.some((item) => item.provider === provider && item.split === split)) {
          reject('release_provider_split_missing');
        }
      }
    }
    if (Object.values(partitionCounts).some((count) => count.attacks === 0))
      reject('release_attack_split_missing');
  }
  return {
    corpus_id: corpus.corpus_id,
    revision: corpus.revision,
    status: manifest.status,
    paired_scenarios: corpus.scenarios.length,
    prompts: corpus.scenarios.length * 2,
    adversarial_cases: corpus.adversarial_cases.length,
    partitions: partitionCounts,
    target_complete: targetComplete,
    semantic_review: manifest.review.status,
    profile_evaluation: 'pending',
  };
}

async function readBounded(file, maximum) {
  if ((await stat(file)).size > maximum) reject('file_bytes');
  const bytes = await readFile(file);
  if (bytes.length === 0 || bytes.length > maximum) reject('file_bytes');
  return bytes;
}

export async function checkCorpus(root = process.cwd(), options = {}) {
  const directory = path.join(root, CORPUS_DIRECTORY);
  let manifest;
  try {
    manifest = JSON.parse(
      (await readBounded(path.join(directory, 'corpus.manifest.json'), 16384)).toString('utf8'),
    );
  } catch {
    reject('manifest_read');
  }
  validateManifest(manifest);
  const bytes = await readBounded(path.join(directory, manifest.corpus_file), MAXIMUM_CORPUS_BYTES);
  return validateCorpus(manifest, bytes, options);
}

async function main() {
  if (
    process.argv.slice(2).some((argument) => argument !== '--release') ||
    process.argv.length > 3
  ) {
    reject('arguments');
  }
  process.stdout.write(
    `${JSON.stringify(await checkCorpus(process.cwd(), { release: process.argv.includes('--release') }), null, 2)}\n`,
  );
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  main().catch((error) => {
    const safe = error instanceof Error && /^corpus invalid: [a-z_]+$/u.test(error.message);
    process.stderr.write(`${safe ? error.message : 'corpus invalid: check_failed'}\n`);
    process.exitCode = 1;
  });
}
