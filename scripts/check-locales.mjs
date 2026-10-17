import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import process from 'node:process';

const catalogPairs = [
  [
    'metadata',
    'apps/web/src/features/product/metadata.messages.en.json',
    'apps/web/src/features/product/metadata.messages.fa.json',
  ],
  [
    'demo',
    'apps/web/src/features/demo/demo.messages.en.json',
    'apps/web/src/features/demo/demo.messages.fa.json',
  ],
  [
    'trend',
    'apps/web/src/features/product/trend.messages.en.json',
    'apps/web/src/features/product/trend.messages.fa.json',
  ],
  [
    'rules',
    'apps/web/src/features/product/rules.messages.en.json',
    'apps/web/src/features/product/rules.messages.fa.json',
  ],
  ['app', 'apps/web/src/app/messages.en.json', 'apps/web/src/app/messages.fa.json'],
  ['shell', 'apps/web/src/app/shell/messages.en.json', 'apps/web/src/app/shell/messages.fa.json'],
  ['routes', 'apps/web/src/routes/messages.en.json', 'apps/web/src/routes/messages.fa.json'],
  [
    'topology',
    'apps/web/src/features/product/topology.messages.en.json',
    'apps/web/src/features/product/topology.messages.fa.json',
  ],
  [
    'exploration',
    'apps/web/src/features/product/exploration.messages.en.json',
    'apps/web/src/features/product/exploration.messages.fa.json',
  ],
  [
    'showcase',
    'apps/web/src/routes/design-system.messages.en.json',
    'apps/web/src/routes/design-system.messages.fa.json',
  ],
];

const pluralCategories = new Set(['zero', 'one', 'two', 'few', 'many', 'other']);
const placeholderPattern = /\{([A-Za-z][A-Za-z0-9_]*)\}/g;

function placeholders(value) {
  return [...value.matchAll(placeholderPattern)].map((match) => match[1]).sort();
}

function sameItems(left, right) {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

function validateMessage(label, key, reference, candidate, issues) {
  if (
    typeof reference !== typeof candidate ||
    Array.isArray(reference) !== Array.isArray(candidate)
  ) {
    issues.push(`${label}: ${key} has a different message kind`);
    return;
  }

  if (typeof reference === 'string') {
    if (reference.trim() === '' || candidate.trim() === '') {
      issues.push(`${label}: ${key} has an empty message`);
    }
    if (!sameItems(placeholders(reference), placeholders(candidate))) {
      issues.push(`${label}: ${key} has different placeholders`);
    }
    return;
  }

  if (
    reference === null ||
    candidate === null ||
    typeof reference !== 'object' ||
    typeof candidate !== 'object'
  ) {
    issues.push(`${label}: ${key} is not a valid message`);
    return;
  }

  const referenceCategories = Object.keys(reference).sort();
  const candidateCategories = Object.keys(candidate).sort();
  if (!sameItems(referenceCategories, candidateCategories)) {
    issues.push(`${label}: ${key} has different plural categories`);
    return;
  }
  if (!referenceCategories.includes('other')) {
    issues.push(`${label}: ${key} has no other plural form`);
  }

  for (const category of referenceCategories) {
    if (!pluralCategories.has(category)) {
      issues.push(`${label}: ${key} has unknown plural form ${category}`);
      continue;
    }
    const referenceForm = reference[category];
    const candidateForm = candidate[category];
    if (typeof referenceForm !== 'string' || typeof candidateForm !== 'string') {
      issues.push(`${label}: ${key}.${category} is not text`);
      continue;
    }
    validateMessage(label, `${key}.${category}`, referenceForm, candidateForm, issues);
  }
}

async function readCatalog(path, issues) {
  const source = await readFile(resolve(path), 'utf8');
  const topLevelKeys = [...source.matchAll(/^ {2}"([^"]+)":/gmu)].map((match) => match[1]);
  const duplicateKeys = topLevelKeys.filter((key, index) => topLevelKeys.indexOf(key) !== index);
  for (const key of new Set(duplicateKeys)) {
    issues.push(`${path}: duplicate key ${key}`);
  }
  return JSON.parse(source);
}

const issues = [];
const allEnglishKeys = new Set();
const allPersianKeys = new Set();

for (const [label, englishPath, persianPath] of catalogPairs) {
  const english = await readCatalog(englishPath, issues);
  const persian = await readCatalog(persianPath, issues);
  const englishKeys = Object.keys(english).sort();
  const persianKeys = Object.keys(persian).sort();

  if (!sameItems(englishKeys, persianKeys)) {
    const missing = englishKeys.filter((key) => !persianKeys.includes(key));
    const extra = persianKeys.filter((key) => !englishKeys.includes(key));
    if (missing.length > 0) {
      issues.push(`${label}: Persian keys missing: ${missing.join(', ')}`);
    }
    if (extra.length > 0) {
      issues.push(`${label}: Persian keys extra: ${extra.join(', ')}`);
    }
  }

  for (const key of englishKeys) {
    if (allEnglishKeys.has(key)) {
      issues.push(`${label}: duplicate English key ${key}`);
    }
    allEnglishKeys.add(key);

    if (!(key in persian)) {
      continue;
    }
    validateMessage(label, key, english[key], persian[key], issues);
  }

  for (const key of persianKeys) {
    if (allPersianKeys.has(key)) {
      issues.push(`${label}: duplicate Persian key ${key}`);
    }
    allPersianKeys.add(key);
  }
}

if (issues.length > 0) {
  for (const issue of issues) {
    process.stderr.write(`${issue}\n`);
  }
  process.exitCode = 1;
} else {
  process.stdout.write(`Locale catalogs match: ${allEnglishKeys.size} messages.\n`);
}
