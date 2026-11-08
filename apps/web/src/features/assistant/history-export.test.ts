import type { AssistantHistoryEntry } from '@stackplore/api-client';
import { describe, expect, it } from 'vitest';
import { controlledConversationApi, scope } from './conversation.test-fixtures';
import { freshHistoryExport } from './history-export';

const entry = (digit = 'a'): AssistantHistoryEntry => ({
  id: `ar_${digit.repeat(64)}`,
  source_id: scope.sourceId,
  resource_id: scope.resourceId,
  goal: 'investigate_incident',
  state: 'completed',
  created_at: '2026-11-08T06:00:00Z',
  updated_at: '2026-11-08T06:01:00Z',
  range: { from: '2026-11-08T05:45:00Z', to: '2026-11-08T06:00:00Z' },
  tool_calls: 7,
  inference_calls: 1,
  usage_uncertain: false,
});
const ownerOf = (controlled: ReturnType<typeof controlledConversationApi>) => {
  const owner = controlled.api.getOwner();
  if (owner === null) throw new Error('Missing test owner');
  return owner;
};

describe('selected private metadata export', () => {
  it('reads again and projects only selected whitelisted metadata', async () => {
    const controlled = controlledConversationApi();
    const row = {
      ...entry(),
      question: 'Never export this question',
      answer: 'Never export this answer',
      session_id: 'Never export this session',
    };
    controlled.api.history.mockResolvedValue({ entries: [row, entry('b')], next_cursor: null });
    const signal = new AbortController().signal;
    const text = await freshHistoryExport(
      controlled.api,
      ownerOf(controlled),
      { state: 'completed', limit: 10 },
      [row.id],
      signal,
    );
    const value: unknown = JSON.parse(text);
    expect(value).toMatchObject({ schema_version: 1, entries: [entry()] });
    expect(text).not.toContain('Never export');
    expect(text).not.toContain(entry('b').id);
    expect(text).not.toContain('next_cursor');
    expect(controlled.api.history).toHaveBeenCalledTimes(1);
    expect(controlled.api.history).toHaveBeenCalledWith(
      ownerOf(controlled),
      { state: 'completed', limit: 10 },
      signal,
    );
  });
  it('rejects invalid selection before any read and refuses changed pages without partial export', async () => {
    const controlled = controlledConversationApi();
    const owner = ownerOf(controlled);
    const signal = new AbortController().signal;
    for (const ids of [
      [],
      [entry().id, entry().id],
      ['other'],
      Array.from({ length: 11 }, (_, index) => `ar_${index.toString(16).padStart(64, '0')}`),
    ])
      await expect(freshHistoryExport(controlled.api, owner, {}, ids, signal)).rejects.toThrow();
    expect(controlled.api.history).not.toHaveBeenCalled();
    controlled.api.history.mockResolvedValue({ entries: [entry()], next_cursor: null });
    await expect(
      freshHistoryExport(controlled.api, owner, {}, [entry().id, entry('b').id], signal),
    ).rejects.toThrow('Metadata selection changed');
  });
  it('drops late metadata after cancellation or changed owner', async () => {
    for (const revoke of ['cancel', 'owner'] as const) {
      const controlled = controlledConversationApi();
      const owner = ownerOf(controlled);
      const controller = new AbortController();
      controlled.api.history.mockImplementation(() => {
        if (revoke === 'cancel') controller.abort();
        else controlled.setOwner(Symbol());
        return Promise.resolve({ entries: [entry()], next_cursor: null });
      });
      await expect(
        freshHistoryExport(controlled.api, owner, {}, [entry().id], controller.signal),
      ).rejects.toMatchObject({ name: 'AbortError' });
    }
  });
});
