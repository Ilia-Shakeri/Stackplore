import assert from 'node:assert/strict';
import test from 'node:test';
import { hostedAssistantProfile } from './hosted-assistant-profile.mjs';

const hosted = {
  SP_BROWSER_REAL_ASSISTANT: 'true',
  GITHUB_ACTIONS: 'true',
  RUNNER_ENVIRONMENT: 'github-hosted',
  SP_BROWSER_MODES: 'normal-en,normal-fa',
};

test('default browser composition forwards no processor capability', () => {
  assert.deepEqual(hostedAssistantProfile({}), {});
  assert.deepEqual(hostedAssistantProfile({ STACKPLORE_ASSISTANT_ENABLED: 'true' }), {});
});

test('real profile rejects accidental workstation, guest and partial locale use', () => {
  for (const altered of [
    { SP_BROWSER_REAL_ASSISTANT: 'false' },
    { GITHUB_ACTIONS: undefined },
    { RUNNER_ENVIRONMENT: 'self-hosted' },
    { SP_BROWSER_MODES: 'guest,normal-en,normal-fa' },
    { SP_BROWSER_MODES: 'normal-en' },
  ]) {
    assert.throws(() => hostedAssistantProfile({ ...hosted, ...altered }));
  }
});

test('hosted profile ignores ambient endpoint and identity overrides', () => {
  const profile = hostedAssistantProfile({
    ...hosted,
    STACKPLORE_ASSISTANT_ORIGIN: 'https://unapproved.invalid',
    STACKPLORE_TEST_INFERENCE_ALIAS: 'unapproved',
  });
  assert.equal(profile.STACKPLORE_ASSISTANT_ORIGIN, 'http://127.0.0.1:58102');
  assert.equal(profile.STACKPLORE_ASSISTANT_OUTPUT_TOKENS, '512');
  assert.equal(Object.isFrozen(profile), true);
  assert.throws(() => {
    profile.STACKPLORE_ASSISTANT_ORIGIN = 'http://127.0.0.1:1';
  });
});
