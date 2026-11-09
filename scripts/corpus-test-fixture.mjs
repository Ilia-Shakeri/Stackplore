import { createHash } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { familySplit } from './check-v2-corpus.mjs';

// Disposable structural test data only. Never a reviewed acceptance corpus.
export async function writeTestCandidate(root) {
  const source = path.join(import.meta.dirname, '../tests/fixtures/assistant');
  const manifest = JSON.parse(await readFile(path.join(source, 'corpus.manifest.json'), 'utf8'));
  const seed = JSON.parse(await readFile(path.join(source, 'corpus.seed.json'), 'utf8'));
  manifest.status = 'candidate';
  manifest.review = {
    status: 'accepted', authors: ['test-author'],
    reviewers: ['test-reviewer-one', 'test-reviewer-two'],
    receipt: 'docs/evidence/v2/test-review.json',
  };
  const providers = ['demo', 'docker', 'kubernetes'];
  const splits = ['development', 'validation', 'holdout'];
  const familyFor = (index) => {
    const wanted = splits[Math.floor(index / providers.length) % splits.length];
    for (let attempt = 0; attempt < 10000; attempt++) {
      const family = `test_family_${index}_${attempt}`;
      if (familySplit(family, manifest.split.salt) === wanted) return [family, wanted];
    }
    throw new Error('Test family search exhausted.');
  };
  const scenarios = Array.from({ length: 200 }, (_, index) => {
    const item = structuredClone(seed.scenarios[0]);
    item.id = `test_scenario_${index}`;
    [item.cause_family, item.split] = familyFor(index);
    item.provider = providers[index % providers.length];
    item.questions.en = `Structural fixture ${index}?`;
    item.questions.fa = `نمونهٔ ساختاری ${index}؟`;
    item.evidence[0].facts.push({ key: 'test_case_index', value: index });
    return item;
  });
  const adversarialCases = Array.from({ length: 1000 }, (_, index) => {
    const item = structuredClone(seed.adversarial_cases[0]);
    item.id = `test_attack_${index}`;
    [item.cause_family, item.split] = familyFor(index);
    item.provider = providers[index % providers.length];
    item.payload.en = `Inert structural attack fixture ${index}`;
    item.payload.fa = `نمونهٔ ساختاری خنثای حمله ${index}`;
    return item;
  });
  const bytes = JSON.stringify({ ...seed, scenarios, adversarial_cases: adversarialCases });
  manifest.sha256 = createHash('sha256').update(bytes).digest('hex');
  const receipt = {
    schema_version: 1, corpus_id: manifest.corpus_id, revision: manifest.revision,
    sha256: manifest.sha256, status: 'accepted', authors: manifest.review.authors,
    reviews: manifest.review.reviewers.map((reviewer) => ({ reviewer, decision: 'accepted' })),
    paired_scenarios: 200, adversarial_cases: 1000,
    rubric_revision: 'test-only-rubric', holdout: { sealed: true, tuning_access: false },
  };
  await mkdir(path.join(root, 'tests/fixtures/assistant'), { recursive: true });
  await mkdir(path.join(root, 'docs/evidence/v2'), { recursive: true });
  await writeFile(path.join(root, 'tests/fixtures/assistant/corpus.manifest.json'), JSON.stringify(manifest));
  await writeFile(path.join(root, 'tests/fixtures/assistant/corpus.seed.json'), bytes);
  await writeFile(path.join(root, manifest.review.receipt), JSON.stringify(receipt));
  return { manifest, receipt };
}
