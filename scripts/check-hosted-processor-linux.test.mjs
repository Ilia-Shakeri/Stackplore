import assert from 'node:assert/strict';
import test from 'node:test';
import { assertHostedProcessorEnvironment, safeArchiveNames } from './check-hosted-processor-linux.mjs';

const hosted = {
  GITHUB_ACTIONS: 'true',
  RUNNER_ENVIRONMENT: 'github-hosted',
  GITHUB_SHA: 'a'.repeat(40),
  RUNNER_TEMP: '/tmp/owned-runner',
};

test('processor guard refuses workstation and incomplete source contexts before I/O', () => {
  assert.doesNotThrow(() => assertHostedProcessorEnvironment('linux', hosted));
  for (const [platform, changed] of [
    ['win32', {}],
    ['linux', { GITHUB_ACTIONS: undefined }],
    ['linux', { RUNNER_ENVIRONMENT: 'self-hosted' }],
    ['linux', { GITHUB_SHA: 'main' }],
    ['linux', { RUNNER_TEMP: 'relative' }],
    ['linux', { RUNNER_TEMP: undefined }],
  ]) {
    assert.throws(() => assertHostedProcessorEnvironment(platform, { ...hosted, ...changed }), /runner_required/u);
  }
});

test('archive entries cannot escape the owned extraction root', () => {
  assert.deepEqual(safeArchiveNames('./bin/llama-server\n./bin/libllama.so.0\n'), ['./bin/llama-server', './bin/libllama.so.0']);
  for (const listing of ['', '/tmp/escape', '../escape', 'bin/../../escape', 'bin\\escape', 'file\nwith space', Array.from({ length: 129 }, () => 'file').join('\n')]) {
    assert.throws(() => safeArchiveNames(listing), /archive_paths/u);
  }
});
