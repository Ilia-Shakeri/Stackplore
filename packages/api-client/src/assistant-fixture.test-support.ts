export const assistantRunId = `ar_${'a'.repeat(64)}`;
export const assistantCitationId = `ev_${'b'.repeat(64)}`;
export const assistantScopeFixture = () => ({
  source_id: 'src_demo_01',
  resource_id: `r1_${'A'.repeat(43)}`,
  window_minutes: 15,
  range: { from: '2026-10-28T10:00:00.000001Z', to: '2026-10-28T10:15:00.000001Z' },
});
export const assistantRunFixture = () => ({
  id: assistantRunId,
  conversation_id: `ac_${'c'.repeat(64)}`,
  goal: 'explain_resource',
  state: 'completed',
  version: 3,
  scope: assistantScopeFixture(),
  created_at: '2026-10-28T10:15:00Z',
  updated_at: '2026-10-28T10:15:01Z',
  deadline: '2026-10-28T10:17:00Z',
  usage_uncertain: false,
  usage_conservative: false,
});
export const assistantCitationFixture = () => ({
  id: assistantCitationId,
  kind: 'resource',
  source_id: assistantScopeFixture().source_id,
  resource_id: assistantScopeFixture().resource_id,
  range: assistantScopeFixture().range,
  observed_at: '2026-10-28T10:15:00Z',
});
export const assistantReadyFixture = () => ({
  run: assistantRunFixture(),
  content_state: 'ready',
  answer: {
    sections: [
      { kind: 'facts', text: 'Selected resource is available.', citations: [assistantCitationId] },
    ],
  },
  citations: [assistantCitationFixture()],
});
