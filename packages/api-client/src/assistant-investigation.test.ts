import { describe, expect, it, vi } from 'vitest';
import { assistantReadyFixture, assistantRunId } from './assistant-fixture.test-support';
import { createApiClient, createAssistantClient, parseAssistantRunResponse, type AssistantRunRequest } from './index';

const fixture = () => {
  const base = assistantReadyFixture();
  const resource = base.citations[0];
  if (resource === undefined) throw new Error('Missing selected-resource fixture');
  const health = { ...resource, id: `ev_${'d'.repeat(64)}`, kind: 'health' };
  return {
    ...base,
    run: { ...base.run, goal: 'investigate_incident' },
    citations: [resource, health],
    coverage: ['inventory', 'health', 'metrics', 'events', 'topology'].map((tool, index) => ({
      tool, availability: index < 2 ? 'available' : 'unsupported', completeness: 'complete',
      records: index < 2 ? 1 : 0, more_available: false,
    })),
    answer: { sections: [{ kind: 'facts', text: 'Selected facts.', citations: [resource.id, health.id] }] },
  };
};

describe('bounded multi-family investigation transport', () => {
  it('accepts only actual scoped references and detaches coverage', () => {
    const value = fixture();
    const result = parseAssistantRunResponse(value, assistantRunId);
    expect(result).toEqual(value);
    if (value.coverage[0] === undefined || result.content_state !== 'ready') throw new Error('Missing coverage fixture');
    value.coverage[0].records = 7;
    expect(result.coverage?.[0]?.records).toBe(1);
  });

  it.each([
    (value: ReturnType<typeof fixture>) => { value.coverage = []; },
    (value: ReturnType<typeof fixture>) => { value.coverage.reverse(); },
    (value: ReturnType<typeof fixture>) => { value.coverage.push({ tool: 'shell', availability: 'available', completeness: 'complete', records: 0, more_available: false }); },
    (value: ReturnType<typeof fixture>) => { const item = value.coverage[1]; if (item !== undefined) item.records = 0; },
    (value: ReturnType<typeof fixture>) => { const item = value.coverage[1]; if (item !== undefined) item.availability = 'unsupported'; },
    (value: ReturnType<typeof fixture>) => { const item = value.citations[1]; if (item !== undefined) item.kind = 'log'; },
    (value: ReturnType<typeof fixture>) => { const item = value.citations[1]; if (item !== undefined) item.source_id = 'other_source'; },
    (value: ReturnType<typeof fixture>) => { value.run.goal = 'explain_resource'; },
  ])('rejects forged or inconsistent coverage %#', (change) => {
    const value = fixture(); change(value);
    expect(() => parseAssistantRunResponse(value, assistantRunId)).toThrow('unexpected_response');
  });

  it('does not execute coercion hooks in rejected metadata', () => {
    const value = fixture();
    const coercion = vi.fn(() => 'available');
    const item = value.coverage[0];
    if (item === undefined) throw new Error('Missing coverage fixture');
    Object.defineProperty(item, 'availability', { value: { toString: coercion }, enumerable: true });
    expect(() => parseAssistantRunResponse(value, assistantRunId)).toThrow('unexpected_response');
    expect(coercion).not.toHaveBeenCalled();
  });

  it('sends explicit consent only with the fixed goal and rejects response goal changes', async () => {
    const input: AssistantRunRequest = { source_id: 'src_demo_01', resource_id: `r1_${'A'.repeat(43)}`, window_minutes: 15, locale: 'en', question: 'Check scoped facts.', goal: 'investigate_incident', log_consent: true };
    const fetcher = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ run: fixture().run }), { status: 202, headers: { 'Content-Type': 'application/json', 'X-Request-ID': 'req_investigation_test' } })));
    const client = createAssistantClient(createApiClient({ fetch: fetcher }));
    const options = { csrfToken: 'A'.repeat(43), idempotencyKey: 'c'.repeat(64) };
    await client.admit(input, options);
    expect(JSON.parse(String(fetcher.mock.calls[0]?.[1]?.body))).toEqual(input);
    for (const invalid of [{ goal: 'shell' }, { goal: 'explain_resource' }, { window_minutes: 60 }, { log_consent: null }, { log_consent: 'true' }, { tools: ['shell'] }]) {
      expect(() => client.admit({ ...input, ...invalid } as never, options)).toThrow('invalid_request');
    }
    expect(fetcher).toHaveBeenCalledOnce();
    await expect(client.admit({ ...input, goal: 'explain_resource', log_consent: false }, options)).rejects.toThrow('unexpected_response');
  });
});
