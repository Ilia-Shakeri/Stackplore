import { parseAssistantRunResponse, type AssistantRun } from '@stackplore/api-client';
import { vi } from 'vitest';

import type { AssistantApi } from './assistant-api';

export const scope = {
  sourceId: 'src_demo_01',
  resourceId: `r1_${'A'.repeat(43)}`,
  windowMinutes: 15 as const,
  locale: 'en' as const,
};
export const resourceRun = (
  state: AssistantRun['state'] = 'completed',
  version = 3,
): AssistantRun => ({
  id: `ar_${'a'.repeat(64)}`,
  conversation_id: `ac_${'c'.repeat(64)}`,
  goal: 'explain_resource',
  state,
  version,
  scope: {
    source_id: scope.sourceId,
    resource_id: scope.resourceId,
    window_minutes: scope.windowMinutes,
    range: { from: '2026-10-28T10:00:00Z', to: '2026-10-28T10:15:00Z' },
  },
  created_at: '2026-10-28T10:15:00Z',
  updated_at: version === 1 ? '2026-10-28T10:15:00Z' : '2026-10-28T10:15:01Z',
  deadline: '2026-10-28T10:17:00Z',
  usage_uncertain: false,
  usage_conservative: false,
});
export const readyResponse = () => {
  const run = resourceRun();
  const citation = {
    id: `ev_${'b'.repeat(64)}`,
    kind: 'resource',
    source_id: scope.sourceId,
    resource_id: scope.resourceId,
    observed_at: run.created_at,
    range: run.scope.range,
  };
  return parseAssistantRunResponse(
    {
      run,
      content_state: 'ready',
      answer: {
        sections: [
          { kind: 'facts', text: 'Selected resource is available.', citations: [citation.id] },
        ],
      },
      citations: [citation],
    },
    run.id,
  );
};
export const lostResponse = (run = resourceRun()) =>
  parseAssistantRunResponse({ run, content_state: 'volatile_lost' }, run.id);
export const pendingResponse = () => {
  const run = resourceRun('queued', 1);
  return parseAssistantRunResponse({ run, content_state: 'pending' }, run.id);
};
export function controlledConversationApi() {
  let owner: symbol | null = Symbol();
  const listeners = new Set<() => void>();
  const api = {
    getOwner: () => owner,
    subscribeOwner(listener: () => void) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    admit: vi.fn<AssistantApi['admit']>().mockResolvedValue({ run: resourceRun() }),
    get: vi.fn<AssistantApi['get']>().mockResolvedValue(lostResponse()),
    cancel: vi
      .fn<AssistantApi['cancel']>()
      .mockResolvedValue({ run: { ...resourceRun(), state: 'cancelled', version: 4 } }),
    citation: vi
      .fn<AssistantApi['citation']>()
      .mockRejectedValue(new Error('Unexpected synthetic citation call')),
    status: vi.fn<AssistantApi['status']>().mockResolvedValue('ended'),
  } satisfies AssistantApi;
  return {
    api,
    listeners,
    setOwner: (next: symbol | null) => {
      owner = next;
      for (const listener of listeners) listener();
    },
  };
}
