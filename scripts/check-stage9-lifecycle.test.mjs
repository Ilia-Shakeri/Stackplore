import assert from 'node:assert/strict';
import test from 'node:test';
import { lifecycleCases, lifecycleIterations, lifecycleResults } from './check-stage9-lifecycle.mjs';

const pkg = 'github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice';
const event = (Action, Test, extra = {}) => ({ Package: pkg, Action, ...(Test ? { Test } : {}), ...extra });
const encode = (events) => events.map((item) => JSON.stringify(item)).join('\n');
function fixture() {
  const events = [];
  for (let iteration = 0; iteration < lifecycleIterations; iteration++) {
    for (const name of lifecycleCases) {
      events.push(event('run', name), event('output', name, { Output: 'private test text must never persist' }), event('pass', name, { Elapsed: 0.01 }));
    }
  }
  events.push(event('pass', undefined, { Elapsed: 1 }));
  return events;
}

test('all eight service boundaries require exactly one hundred completed observations each', () => {
  const report = lifecycleResults(encode(fixture()));
  assert.equal(report.status, 'passed');
  assert.equal(report.cases.length, 8);
  assert.equal(report.cases.reduce((sum, item) => sum + item.passed, 0), 800);
  assert.equal(JSON.stringify(report).includes('private test text'), false);
});

test('missing, incomplete, extra or skipped iterations cannot pass', () => {
  for (const mutate of [
    (events) => events.splice(0, 3),
    (events) => events.splice(2, 1),
    (events) => events.splice(-1, 0, event('run', lifecycleCases[0]), event('pass', lifecycleCases[0], { Elapsed: 0 })),
    (events) => { events[2].Action = 'skip'; },
    (events) => { events[2].Action = 'fail'; },
    (events) => events.pop(),
    (events) => { events.at(-1).Action = 'fail'; },
  ]) {
    const events = fixture();
    mutate(events);
    let status = 'failed';
    try { status = lifecycleResults(encode(events)).status; } catch { /* Malformed order fails closed. */ }
    assert.equal(status, 'failed');
  }
});

test('nested failures remain failures even if a parent claims success', () => {
  for (const Action of ['fail', 'skip']) {
    const events = fixture();
    const child = `${lifecycleCases[0]}/child`;
    events.splice(1, 0, event('run', child), event(Action, child, { Elapsed: 0 }));
    assert.equal(lifecycleResults(encode(events)).status, 'failed');
  }
});

test('duplicate terminals, unexpected packages, unknown names and invalid durations reject', () => {
  for (const mutate of [
    (events) => events.splice(3, 0, events[2]),
    (events) => { events[0].Package = 'other'; },
    (events) => { events[0].Test = 'UnknownTest'; },
    (events) => { events[2].Elapsed = -1; },
    (events) => { events[2].Elapsed = 601; },
    (events) => { events[2].Elapsed = null; },
  ]) {
    const events = fixture();
    mutate(events);
    assert.throws(() => lifecycleResults(encode(events)), /stage9_lifecycle.invalid_events/u);
  }
  assert.throws(() => lifecycleResults('{'), /stage9_lifecycle.invalid_events/u);
  assert.throws(() => lifecycleResults('x'.repeat(8 * 1024 * 1024 + 1)), /stage9_lifecycle.invalid_events/u);
});
