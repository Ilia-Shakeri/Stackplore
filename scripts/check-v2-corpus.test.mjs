import assert from 'node:assert/strict';
import { Buffer } from 'node:buffer';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { mkdir, mkdtemp, readFile, rm, symlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import process from 'node:process';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import {
  checkCorpus,
  familySplit,
  MAXIMUM_CORPUS_BYTES,
  validateCorpus,
  validateReviewReceipt,
} from './check-v2-corpus.mjs';

const directory = new URL('../tests/fixtures/assistant/', import.meta.url);
const seedManifest = JSON.parse(await readFile(new URL('corpus.manifest.json', directory), 'utf8'));
const seedBytes = await readFile(new URL('corpus.seed.json', directory));

const clone = (value) => JSON.parse(JSON.stringify(value));

function fixture(edit = () => {}) {
  const manifest = clone(seedManifest);
  const corpus = JSON.parse(seedBytes.toString('utf8'));
  edit(manifest, corpus);
  const bytes = Buffer.from(JSON.stringify(corpus));
  manifest.sha256 = createHash('sha256').update(bytes).digest('hex');
  return { manifest, bytes };
}

function invalid(edit, code, options = {}) {
  const { manifest, bytes } = fixture(edit);
  assert.throws(() => validateCorpus(manifest, bytes, options), {
    message: `corpus invalid: ${code}`,
  });
}

test('curated seed reports actual counts and pending proof without accepting release', async () => {
  const result = await checkCorpus(fileURLToPath(new URL('../', import.meta.url)));
  assert.equal(result.paired_scenarios, 12);
  assert.equal(result.prompts, 24);
  assert.equal(result.adversarial_cases, 18);
  assert.equal(result.target_complete, false);
  assert.equal(result.semantic_review, 'pending');
  assert.equal(result.profile_evaluation, 'pending');
  assert.throws(() => validateCorpus(seedManifest, seedBytes, { release: true }), {
    message: 'corpus invalid: release_status',
  });
});

test('cause-family partition is stable and moving a case or translation cannot change its split', () => {
  const salt = seedManifest.split.salt;
  const family = 'workload_backoff';
  assert.equal(familySplit(family, salt), familySplit(family, salt));
  invalid((_, corpus) => {
    const item = corpus.scenarios.find((scenario) => scenario.cause_family === family);
    item.split = item.split === 'holdout' ? 'development' : 'holdout';
  }, 'family_split');
  invalid((_, corpus) => {
    corpus.scenarios[0].questions.de = 'Zusatz';
  }, 'questions');
});

test('digest binds exact fixture bytes and invalid encodings fail closed', () => {
  assert.throws(() => validateCorpus(seedManifest, Buffer.concat([seedBytes, Buffer.from(' ')])), {
    message: 'corpus invalid: digest_mismatch',
  });
  const bytes = Buffer.from([0xff]);
  const manifest = clone(seedManifest);
  manifest.sha256 = createHash('sha256').update(bytes).digest('hex');
  assert.throws(() => validateCorpus(manifest, bytes), { message: 'corpus invalid: corpus_json' });
});

test('unknown fields, shortened target floors and unsafe fixture paths are rejected', () => {
  invalid((manifest) => {
    manifest.corpus_file = '../../outside.json';
  }, 'corpus_file');
  invalid((manifest) => {
    manifest.target.adversarial_cases = 18;
  }, 'target_floor');
  invalid((_, corpus) => {
    corpus.measured_quality = 1;
  }, 'corpus_shape');
  invalid((_, corpus) => {
    corpus.scenarios[0].provider = 'unknown';
  }, 'provider');
});

test('oversized payloads and corpus bytes are rejected before evaluation', () => {
  invalid((_, corpus) => {
    corpus.adversarial_cases[0].payload.en = 'x'.repeat(3001);
  }, 'attack_payload');
  assert.throws(() => validateCorpus(seedManifest, Buffer.alloc(MAXIMUM_CORPUS_BYTES + 1)), {
    message: 'corpus invalid: corpus_bytes',
  });
});

test('citations must resolve inside a unique synthetic evidence bundle', () => {
  invalid((_, corpus) => {
    corpus.scenarios[0].expected.required_evidence_ids = ['unissued'];
  }, 'unknown_citation');
  invalid((_, corpus) => {
    corpus.scenarios[0].evidence[0].synthetic = false;
  }, 'synthetic_only');
  invalid((_, corpus) => {
    corpus.scenarios[0].evidence.push(clone(corpus.scenarios[0].evidence[0]));
  }, 'duplicate_evidence');
});

test('duplicate names, normalized questions and renamed duplicate incidents cannot inflate counts', () => {
  invalid((_, corpus) => {
    corpus.adversarial_cases[0].id = corpus.scenarios[0].id;
  }, 'duplicate_case');
  invalid((_, corpus) => {
    const duplicate = clone(corpus.scenarios[0]);
    duplicate.id = 'duplicate_question';
    duplicate.questions.en = ` ${duplicate.questions.en.toUpperCase()} `;
    corpus.scenarios.push(duplicate);
  }, 'duplicate_question');
  invalid((_, corpus) => {
    const duplicate = clone(corpus.scenarios[0]);
    duplicate.id = 'renamed_incident';
    duplicate.questions.en = 'A different wording of the same incident?';
    duplicate.questions.fa = 'واژهٔ دیگر برای همان رخداد؟';
    duplicate.evidence[0].id = 'renamed_evidence';
    duplicate.expected.required_evidence_ids = ['renamed_evidence'];
    corpus.scenarios.push(duplicate);
  }, 'duplicate_incident');
  invalid((_, corpus) => {
    const duplicate = clone(corpus.adversarial_cases[0]);
    duplicate.id = 'renamed_attack';
    corpus.adversarial_cases.push(duplicate);
  }, 'duplicate_attack');
});

test('unreviewed short corpus cannot be promoted by changing its status', () => {
  invalid(
    (manifest) => {
      manifest.status = 'candidate';
    },
    'release_target_incomplete',
    { release: true },
  );
  invalid((manifest) => {
    manifest.review = {
      status: 'accepted',
      authors: ['same-editor'],
      reviewers: ['same-editor', 'other-reviewer'],
      receipt: 'docs/evidence/v2/review.json',
    };
  }, 'review_independence');
  invalid((manifest) => {
    manifest.review = {
      status: 'accepted',
      authors: ['seed-editor'],
      reviewers: ['first-reviewer'],
      receipt: 'docs/evidence/v2/review.json',
    };
  }, 'accepted_review_receipt');
});

test('Persian pair must contain Persian text and meaningful expectations remain closed', () => {
  invalid((_, corpus) => {
    corpus.scenarios[0].questions.fa = 'English only';
  }, 'questions_persian');
  invalid((_, corpus) => {
    corpus.scenarios[0].expected.assertions[0].kind = 'arbitrary';
  }, 'assertion_kind');
  invalid((_, corpus) => {
    corpus.adversarial_cases[0].expected.forbidden_effects = ['arbitrary'];
  }, 'forbidden_effect');
});

test('noncanonical dates, unsafe scalar values and invalid Unicode cannot enter evidence', () => {
  invalid((_, corpus) => {
    corpus.scenarios[0].evidence[0].observed_at = '2026-02-30T08:00:00.000000Z';
  }, 'evidence_time');
  invalid((_, corpus) => {
    corpus.scenarios[0].evidence[0].facts[0].value = { nested: 'not-scalar' };
  }, 'fact_value');
  invalid((_, corpus) => {
    corpus.scenarios[0].questions.en = String.fromCharCode(0xd800);
  }, 'questions');
});

test('command rejects unknown arguments with only a fixed safe error', () => {
  const result = spawnSync(
    process.execPath,
    [
      fileURLToPath(new URL('./check-v2-corpus.mjs', import.meta.url)),
      '--synthetic-untrusted-input',
    ],
    { encoding: 'utf8', timeout: 5000, maxBuffer: 65536 },
  );
  assert.equal(result.error, undefined);
  assert.equal(result.status, 1);
  assert.equal(result.stdout, '');
  assert.equal(result.stderr.trim(), 'corpus invalid: arguments');
});

function reviewedFixture() {
  const { manifest, bytes } = fixture((manifest) => {
    manifest.review = {
      status: 'accepted', authors: ['seed-editor'],
      reviewers: ['first-reviewer', 'second-reviewer'],
      receipt: 'docs/evidence/v2/fixture-review.json',
    };
  });
  const summary = validateCorpus(manifest, bytes);
  const receipt = {
    schema_version: 1, corpus_id: manifest.corpus_id, revision: manifest.revision,
    sha256: manifest.sha256, status: 'accepted', authors: ['seed-editor'],
    reviews: [
      { reviewer: 'first-reviewer', decision: 'accepted' },
      { reviewer: 'second-reviewer', decision: 'accepted' },
    ],
    paired_scenarios: summary.paired_scenarios, adversarial_cases: summary.adversarial_cases,
    rubric_revision: 'fixture-rubric-v1', holdout: { sealed: true, tuning_access: false },
  };
  return { manifest, bytes, summary, receipt };
}

test('review receipt binds exact bytes, revision, counts, distinct reviewers and sealed scope', () => {
  const { manifest, summary, receipt } = reviewedFixture();
  assert.doesNotThrow(() => validateReviewReceipt(manifest, summary, receipt));
  for (const [edit, code] of [
    [(value) => { value.sha256 = 'a'.repeat(64); }, 'receipt_identity'],
    [(value) => { value.revision = 'other-revision'; }, 'receipt_identity'],
    [(value) => { value.paired_scenarios++; }, 'receipt_counts'],
    [(value) => { value.adversarial_cases++; }, 'receipt_counts'],
    [(value) => { value.authors = ['other-author']; }, 'receipt_authors'],
    [(value) => { value.reviews[1].reviewer = 'first-reviewer'; }, 'receipt_reviewers'],
    [(value) => { value.reviews[1].reviewer = 'seed-editor'; }, 'receipt_reviewers'],
    [(value) => { value.reviews[1].decision = 'rejected'; }, 'receipt_decision'],
    [(value) => { value.holdout.tuning_access = true; }, 'receipt_holdout'],
    [(value) => { value.holdout.sealed = false; }, 'receipt_holdout'],
    [(value) => { value.rubric_revision += '\n'; }, 'receipt_rubric'],
    [(value) => { value.extra = true; }, 'receipt_shape'],
  ]) {
    const changed = clone(receipt);
    edit(changed);
    assert.throws(() => validateReviewReceipt(manifest, summary, changed), {
      message: `corpus invalid: ${code}`,
    });
  }
});

test('final newlines cannot alias corpus identities, digests or review paths', () => {
  invalid((manifest) => { manifest.corpus_id += '\n'; }, 'corpus_id');
  invalid((manifest) => { manifest.split.salt += '\n'; }, 'split_salt');
  const { manifest, bytes } = reviewedFixture();
  manifest.sha256 += '\n';
  assert.throws(() => validateCorpus(manifest, bytes), { message: 'corpus invalid: digest' });
  manifest.sha256 = manifest.sha256.trim();
  manifest.review.receipt += '\n';
  assert.throws(() => validateCorpus(manifest, bytes), {
    message: 'corpus invalid: accepted_review_receipt',
  });
});

async function diskFixture(context) {
  const root = await mkdtemp(path.join(tmpdir(), 'stackplore-corpus-test-'));
  context.after(() => {
    assert.equal(path.dirname(root), path.resolve(tmpdir()));
    assert.match(path.basename(root), /^stackplore-corpus-test-[a-z0-9]+$/iu);
    return rm(root, { recursive: true, force: true });
  });
  const value = reviewedFixture();
  await mkdir(path.join(root, 'tests/fixtures/assistant'), { recursive: true });
  await mkdir(path.join(root, 'docs/evidence/v2'), { recursive: true });
  await writeFile(path.join(root, 'tests/fixtures/assistant/corpus.manifest.json'), JSON.stringify(value.manifest));
  await writeFile(path.join(root, 'tests/fixtures/assistant/corpus.seed.json'), value.bytes);
  const receiptPath = path.join(root, value.manifest.review.receipt);
  return { root, receiptPath, ...value };
}

test('accepted review requires a readable bounded matching receipt; pending seed needs none', async (context) => {
  const { root, receiptPath, receipt } = await diskFixture(context);
  await assert.rejects(checkCorpus(root), { message: 'corpus invalid: receipt_read' });
  await writeFile(receiptPath, JSON.stringify(receipt));
  assert.equal((await checkCorpus(root)).semantic_review, 'accepted');
  await assert.rejects(checkCorpus(root, { release: true }), { message: 'corpus invalid: release_status' });
  await writeFile(receiptPath, Buffer.from([0xff]));
  await assert.rejects(checkCorpus(root), { message: 'corpus invalid: receipt_read' });
  await writeFile(receiptPath, 'x'.repeat(65537));
  await assert.rejects(checkCorpus(root), { message: 'corpus invalid: receipt_read' });
  await writeFile(receiptPath, JSON.stringify({ ...receipt, revision: 'wrong-revision' }));
  await assert.rejects(checkCorpus(root), { message: 'corpus invalid: receipt_identity' });
});

test('receipt links, linked parent directories and non-regular corpus files fail closed', async (context) => {
  const { root, receiptPath, receipt } = await diskFixture(context);
  const actual = path.join(root, 'review.json');
  await writeFile(actual, JSON.stringify(receipt));
  await symlink(actual, receiptPath);
  await assert.rejects(checkCorpus(root), { message: 'corpus invalid: receipt_read' });
  await rm(receiptPath);
  const parent = path.dirname(receiptPath);
  await rm(parent, { recursive: true });
  await symlink(root, parent, process.platform === 'win32' ? 'junction' : 'dir');
  await assert.rejects(checkCorpus(root), { message: 'corpus invalid: receipt_read' });
  const corpusPath = path.join(root, 'tests/fixtures/assistant/corpus.seed.json');
  await rm(corpusPath);
  await mkdir(corpusPath);
  await assert.rejects(checkCorpus(root), { message: 'corpus invalid: file_type' });
});
