import assert from 'node:assert/strict';
import test from 'node:test';
import { hardeningCases, hardeningResults } from './check-stage8-hardening.mjs';

const events = () => Object.entries(hardeningCases).flatMap(([pkg, names]) => names.map((name) => ({
  Package: `github.com/Ilia-Shakeri/stackplore/apps/api/internal/${pkg}`, Test: name,
  Action: 'pass', Elapsed: 0.1,
})));
const encode = (values) => values.map((value) => JSON.stringify(value)).join('\n');
test('fault receipt requires every named database, authority and adapter boundary', () => {
  const values = events();
  assert.equal(values.length, 19);
  assert.ok(hardeningResults(encode(values)).every((row) => row.status === 'pass'));
  values[0].Action = 'skip'; values.pop();
  const summary = hardeningResults(encode(values));
  assert.equal(summary[0].status, 'skip');
  assert.equal(summary.at(-1).status, 'missing');
});
test('duplicate and malformed events reject without exposing private output', () => {
  const values = events(); values.push(values[0]);
  assert.throws(() => hardeningResults(encode(values)), { message: 'stage8_hardening.invalid_events' });
  assert.throws(() => hardeningResults('not-json private text'), { message: 'stage8_hardening.invalid_events' });
  assert.ok(hardeningResults(JSON.stringify({ Action: 'output', Output: 'private text' })).every((row) => !JSON.stringify(row).includes('private text')));
});
