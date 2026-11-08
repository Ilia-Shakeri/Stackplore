import { describe, expect, it, vi } from 'vitest';
import { assistantHistoryInput, parseAssistantHistoryResponse } from './assistant-history';
import { createApiClient, createAssistantClient } from './index';

const cursor = `${'a'.repeat(50)}.${'b'.repeat(43)}`;
const entry = () => ({
  id: `ar_${'b'.repeat(64)}`,
  source_id: 'source_demo',
  resource_id: null,
  goal: 'investigate_incident' as const,
  state: 'completed' as const,
  created_at: '2026-11-08T06:00:00Z',
  updated_at: '2026-11-08T06:01:00Z',
  range: { from: '2026-11-08T05:45:00Z', to: '2026-11-08T06:00:00Z' },
  tool_calls: 7,
  inference_calls: 1,
  usage_uncertain: false,
});
const page = () => ({ entries: [entry()], next_cursor: null as string | null });

describe('owner-private metadata history', () => {
  it('reads only a bounded state and cursor without mutation or replay', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify(page()), {
        headers: { 'Content-Type': 'application/json', 'X-Request-ID': 'req_history_test' },
      }),
    );
    const client = createAssistantClient(createApiClient({ fetch: fetchMock }));
    const result = await client.history({ state: 'completed', limit: 1, cursor });
    expect(result.data).toEqual(page());
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe(`/api/v1/assistant/history?state=completed&limit=1&cursor=${cursor}`);
    expect(init).toMatchObject({ method: 'GET', credentials: 'same-origin', cache: 'no-store' });
    expect(init?.body).toBeUndefined();
    expect(new Headers(init?.headers).has('X-Stackplore-CSRF')).toBe(false);
    expect(assistantHistoryInput()).toEqual({ state: 'completed', limit: 10 });
  });

  it('rejects foreign scope, accessors and malformed filters before transport', async () => {
    const fetchMock = vi.fn<typeof fetch>();
    const client = createAssistantClient(createApiClient({ fetch: fetchMock }));
    for (const value of [
      { owner: 'other' },
      { state: 'all' },
      { limit: 11 },
      { limit: 1.5 },
      { cursor: `${cursor}\n` },
      { cursor: '' },
      Object.create({ state: 'completed' }) as unknown,
      Object.defineProperty({}, 'state', {
        get() {
          throw new Error('must not read accessor');
        },
      }),
    ]) {
      await expect(
        client.history(value as Parameters<typeof client.history>[0]),
      ).rejects.toMatchObject({ code: 'invalid_request' });
    }
    const stopped = new AbortController();
    stopped.abort();
    await expect(client.history({}, { signal: stopped.signal })).rejects.toBeDefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('distinguishes empty metadata from closed unavailable text', () => {
    expect(parseAssistantHistoryResponse({ entries: [], next_cursor: null }, 'completed')).toEqual({
      entries: [],
      next_cursor: null,
    });
    const value = page();
    expect(parseAssistantHistoryResponse(value, 'completed')).toEqual(value);
    const parsed = parseAssistantHistoryResponse(value, 'completed');
    const first = value.entries[0];
    if (first === undefined) throw new Error('Expected metadata row');
    first.range.from = 'changed';
    expect(parsed.entries[0]?.range.from).toBe('2026-11-08T05:45:00Z');
  });

  it('rejects hostile rows, content fields, malformed clocks and cursor loops', () => {
    const mutations: ((value: Record<string, unknown>) => void)[] = [
      (value) => { value.range = { from: '2026-11-08T06:00:00Z', to: '2026-11-08T06:00:00Z' }; },
      (value) => {
        value.question = 'private';
      },
      (value) => {
        value.id = 'foreign';
      },
      (value) => {
        value.source_id = '../source';
      },
      (value) => {
        value.resource_id = 'r1_bad';
      },
      (value) => {
        value.state = 'running';
      },
      (value) => {
        value.goal = 'execute';
      },
      (value) => {
        value.tool_calls = 13;
      },
      (value) => {
        value.inference_calls = -1;
      },
      (value) => {
        value.usage_uncertain = 'false';
      },
      (value) => {
        value.created_at = '2026-11-08T06:00:00Z\n';
      },
      (value) => {
        value.updated_at = '2026-11-08T05:59:00Z';
      },
      (value) => {
        value.range = { from: '2026-11-06T06:00:00Z', to: '2026-11-08T06:00:00Z' };
      },
      (value) => {
        value.range = { from: '2026-11-08T06:00:01Z', to: '2026-11-08T06:00:01Z' };
      },
    ];
    for (const mutate of mutations) {
      const value: Record<string, unknown> = entry();
      mutate(value);
      expect(() =>
        parseAssistantHistoryResponse({ entries: [value], next_cursor: null }, 'completed'),
      ).toThrow();
    }
    expect(() =>
      parseAssistantHistoryResponse({ entries: [], next_cursor: cursor }, 'completed', 1),
    ).toThrow();
    expect(() =>
      parseAssistantHistoryResponse({ ...page(), answer: 'private' }, 'completed'),
    ).toThrow();
    expect(() =>
      parseAssistantHistoryResponse({ ...page(), next_cursor: cursor }, 'completed', 1, cursor),
    ).toThrow();
    expect(() =>
      parseAssistantHistoryResponse(
        { entries: [entry(), entry()], next_cursor: null },
        'completed',
      ),
    ).toThrow();
    const a = { ...entry(), id: `ar_${'a'.repeat(64)}` };
    expect(() =>
      parseAssistantHistoryResponse({ entries: [a, entry()], next_cursor: null }, 'completed'),
    ).toThrow();
    expect(
      parseAssistantHistoryResponse({ entries: [entry(), a], next_cursor: null }, 'completed')
        .entries,
    ).toHaveLength(2);
  });
});
