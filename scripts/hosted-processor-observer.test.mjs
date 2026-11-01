import assert from 'node:assert/strict';
import test from 'node:test';
import {
  assertHostedObserverEnvironment,
  startHostedProcessorObserver,
} from './hosted-processor-observer.mjs';

test('metadata observer refuses workstation, untrusted runner and noncanonical source before I/O', async () => {
  const valid = {
    GITHUB_ACTIONS: 'true',
    RUNNER_ENVIRONMENT: 'github-hosted',
    GITHUB_SHA: 'a'.repeat(40),
  };
  assert.doesNotThrow(() => assertHostedObserverEnvironment(valid));
  for (const changed of [
    { GITHUB_ACTIONS: undefined },
    { RUNNER_ENVIRONMENT: 'self-hosted' },
    { GITHUB_SHA: 'main' },
  ]) {
    await assert.rejects(
      startHostedProcessorObserver({ ...valid, ...changed }),
      /observer_runner_required/u,
    );
  }
});
