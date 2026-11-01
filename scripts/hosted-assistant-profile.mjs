// Trusted test composition only. Product defaults remain disabled.
export function hostedAssistantProfile(environment) {
  const enabled = environment.SP_BROWSER_REAL_ASSISTANT;
  if (enabled === undefined) return Object.freeze({});
  if (
    enabled !== 'true' ||
    environment.GITHUB_ACTIONS !== 'true' ||
    environment.RUNNER_ENVIRONMENT !== 'github-hosted' ||
    environment.SP_BROWSER_MODES !== 'normal-en,normal-fa'
  ) {
    throw new Error('Real browser profile requires the complete hosted normal-mode gate');
  }
  return Object.freeze({
    STACKPLORE_ASSISTANT_ENABLED: 'true',
    STACKPLORE_ASSISTANT_ORIGIN: 'http://127.0.0.1:58102',
    STACKPLORE_ASSISTANT_MODEL_SHA256:
      '061b54daade076b5d3362dac252678d17da8c68f07560be70818cace6590cb1a',
    STACKPLORE_ASSISTANT_BUILD: 'b11729-412a363c6',
    STACKPLORE_ASSISTANT_TEMPLATE_SHA256:
      '57f1fd00f0013a2be96aa79b857391f27e23df5b5f847072b524c897e24d0361',
    STACKPLORE_ASSISTANT_CONTEXT_TOKENS: '20480',
    STACKPLORE_ASSISTANT_GENERATION: '1',
    STACKPLORE_ASSISTANT_OUTPUT_TOKENS: '512',
    STACKPLORE_ASSISTANT_CALL_TIMEOUT: '40s',
  });
}
