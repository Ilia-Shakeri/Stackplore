import {
  ApiProblemError,
  ApiTransportError,
  type AssistantRunReceipt,
  type AssistantRunResponse,
} from '@stackplore/api-client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AssistantConversation } from './conversation';
import {
  controlledConversationApi,
  pendingResponse,
  readyResponse,
  resourceRun,
  scope,
} from './conversation.test-fixtures';

const disposals: AssistantConversation[] = [];
const make = () => {
  const controlled = controlledConversationApi();
  const conversation = new AssistantConversation(controlled.api, scope);
  conversation.connect();
  disposals.push(conversation);
  return { ...controlled, conversation };
};
afterEach(() => {
  for (const item of disposals.splice(0)) item.dispose();
  vi.useRealTimers();
});
const forbidden = () =>
  new ApiProblemError({
    code: 'forbidden',
    status: 403,
    title: 'Access denied',
    type: 'urn:stackplore:problem:forbidden',
    request_id: 'req_conversation_test',
  });

describe('volatile owning conversation', () => {
  it('clears a rejected-input state when the user supplies valid text, without dispatch', () => {
    const { api, conversation } = make();
    conversation.setDraft('\0');
    expect(conversation.getSnapshot().state).toBe('invalid_question');
    conversation.setDraft('New explicit input.');
    expect(conversation.getSnapshot().state).toBe('idle');
    expect(api.admit).not.toHaveBeenCalled();
  });
  it('connects without dispatch or content and disposes its subscription', () => {
    const { api, conversation, listeners } = make();
    expect(conversation.getSnapshot()).toEqual({ state: 'idle', draft: '', turns: [] });
    expect(api.admit).not.toHaveBeenCalled();
    expect(api.status).not.toHaveBeenCalled();
    expect(listeners.size).toBe(1);
    conversation.connect();
    expect(listeners.size).toBe(1);
    conversation.dispose();
    expect(listeners.size).toBe(0);
    expect(JSON.stringify(conversation)).toBe('{}');
  });

  it('submits one explicit scoped turn and preserves volatile loss truthfully', async () => {
    const { api, conversation } = make();
    conversation.setDraft('Explain selected resource.');
    expect(api.admit).not.toHaveBeenCalled();
    await conversation.submit();
    expect(api.admit).toHaveBeenCalledOnce();
    expect(api.admit.mock.calls[0]?.[1]).toEqual({
      source_id: scope.sourceId,
      resource_id: scope.resourceId,
      window_minutes: 15,
      locale: 'en',
      question: 'Explain selected resource.',
    });
    expect(api.admit.mock.calls[0]?.[2]).toMatch(/^[a-f0-9]{64}$/u);
    expect(api.get).toHaveBeenCalledOnce();
    expect(api.status).not.toHaveBeenCalled();
    expect(conversation.getSnapshot()).toMatchObject({
      state: 'volatile_lost',
      draft: '',
      turns: [
        { question: 'Explain selected resource.', response: { content_state: 'volatile_lost' } },
      ],
    });
  });

  it('retries an ambiguous admission only on request, with the same body and nonce', async () => {
    const { api, conversation } = make();
    api.admit.mockRejectedValueOnce(new ApiTransportError('network_error'));
    conversation.setDraft('Explain selected resource.');
    await conversation.submit();
    expect(conversation.getSnapshot().state).toBe('admission_unknown');
    conversation.setDraft('Replacement must not be sent.');
    await conversation.submit();
    expect(api.admit).toHaveBeenCalledOnce();
    await conversation.retryAdmission();
    expect(api.admit).toHaveBeenCalledTimes(2);
    expect(api.admit.mock.calls[1]?.[1]).toBe(api.admit.mock.calls[0]?.[1]);
    expect(api.admit.mock.calls[1]?.[2]).toBe(api.admit.mock.calls[0]?.[2]);
    expect(conversation.getSnapshot().turns).toHaveLength(1);
  });

  it('bounds the conversation to eight submitted turns and new explicit nonces', async () => {
    const { api, conversation } = make();
    api.get.mockResolvedValue(readyResponse());
    for (let index = 0; index < 8; index += 1) {
      conversation.setDraft(`Selected resource check ${String(index)}.`);
      await conversation.submit();
    }
    expect(api.admit).toHaveBeenCalledTimes(8);
    expect(new Set(api.admit.mock.calls.map((call) => call[2])).size).toBe(8);
    expect(api.admit.mock.calls[1]?.[1].conversation_id).toBe(resourceRun().conversation_id);
    conversation.setDraft('One turn too many.');
    await conversation.submit();
    expect(conversation.getSnapshot().state).toBe('limit');
    expect(api.admit).toHaveBeenCalledTimes(8);
  });

  it('expires from first private input without extending the thirty-minute bound', async () => {
    vi.useFakeTimers();
    const { conversation } = make();
    conversation.setDraft('First private input.');
    await vi.advanceTimersByTimeAsync(29 * 60_000);
    conversation.setDraft('Changed private input.');
    await vi.advanceTimersByTimeAsync(60_000);
    expect(conversation.getSnapshot()).toEqual({ state: 'expired', draft: '', turns: [] });
  });

  it('rejects a late admission after expiry even if its transport ignores abort', async () => {
    vi.useFakeTimers();
    const { api, conversation } = make();
    let finish: (value: AssistantRunReceipt) => void = () => {
      throw new Error('Admission did not start');
    };
    api.admit.mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    conversation.setDraft('Private input.');
    const pending = conversation.submit();
    await vi.advanceTimersByTimeAsync(30 * 60_000);
    expect(conversation.getSnapshot().turns).toHaveLength(0);
    expect(api.admit.mock.calls[0]?.[3]?.aborted).toBe(true);
    finish({ run: resourceRun() });
    await pending;
    expect(conversation.getSnapshot().state).toBe('expired');
    expect(api.get).not.toHaveBeenCalled();
  });

  it('wipes immediately on owner change and rejects late private reads', async () => {
    const { api, conversation, setOwner } = make();
    let finish: (value: AssistantRunResponse) => void = () => {
      throw new Error('Read did not start');
    };
    api.get.mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    conversation.setDraft('Private input.');
    const pending = conversation.submit();
    await Promise.resolve();
    setOwner(Symbol());
    expect(conversation.getSnapshot()).toEqual({ state: 'unavailable', draft: '', turns: [] });
    finish(readyResponse());
    await pending;
    expect(conversation.getSnapshot().turns).toHaveLength(0);
  });

  it('supports clean effect remount without reviving an old draft', () => {
    const { conversation, listeners } = make();
    conversation.setDraft('Private input.');
    conversation.dispose();
    conversation.connect();
    expect(listeners.size).toBe(1);
    expect(conversation.getSnapshot()).toEqual({ state: 'idle', draft: '', turns: [] });
  });

  it('refreshes reset metadata once without reconnecting or resending the question', async () => {
    const { api, conversation } = make();
    api.admit.mockResolvedValue({ run: resourceRun('queued', 1) });
    api.status.mockImplementation((_owner, runId, publish) => {
      publish({ type: 'reset', data: { run_id: runId, reason: 'replay_gap' } });
      return Promise.resolve('reset');
    });
    conversation.setDraft('Private input.');
    await conversation.submit();
    expect(api.admit).toHaveBeenCalledOnce();
    expect(api.status).toHaveBeenCalledOnce();
    expect(api.get).toHaveBeenCalledOnce();
    expect(conversation.getSnapshot().state).toBe('volatile_lost');
  });

  it('requires explicit recovery after stream failure and rejects content older than observed status', async () => {
    const { api, conversation } = make();
    api.admit.mockResolvedValue({ run: resourceRun('queued', 1) });
    api.status.mockImplementation((_owner, runId, publish) => {
      publish({
        type: 'status',
        id: '4',
        data: {
          run_id: runId,
          sequence: 4,
          version: 4,
          state: 'completed',
          at: resourceRun().updated_at,
        },
      });
      return Promise.reject(new ApiTransportError('network_error'));
    });
    conversation.setDraft('Private input.');
    await conversation.submit();
    expect(conversation.getSnapshot().state).toBe('needs_refresh');
    expect(api.get).not.toHaveBeenCalled();
    await conversation.refresh();
    expect(conversation.getSnapshot().state).toBe('needs_refresh');
    expect(conversation.getSnapshot().turns[0]?.response).toBeUndefined();
    expect(api.status).toHaveBeenCalledOnce();
  });

  it('erases local text before a cancellation acknowledgement', async () => {
    const { api, conversation } = make();
    api.admit.mockResolvedValue({ run: resourceRun('queued', 1) });
    api.get.mockResolvedValue(pendingResponse());
    let finish: (value: AssistantRunReceipt) => void = () => {
      throw new Error('Cancel did not start');
    };
    api.cancel.mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    conversation.setDraft('Private input.');
    await conversation.submit();
    const pending = conversation.cancel();
    conversation.setDraft('Must wait for cancellation.');
    await conversation.cancel();
    expect(conversation.getSnapshot().turns).toHaveLength(0);
    expect(conversation.getSnapshot().draft).toBe('');
    expect(api.cancel).toHaveBeenCalledOnce();
    finish({ run: resourceRun('cancelled', 4) });
    await pending;
    expect(conversation.getSnapshot().state).toBe('cancelled');
  });

  it('requires explicit new input after volatile loss and starts a fresh server conversation', async () => {
    const { api, conversation } = make();
    conversation.setDraft('Private input.');
    await conversation.submit();
    expect(conversation.getSnapshot().state).toBe('volatile_lost');
    expect(api.admit).toHaveBeenCalledOnce();
    conversation.setDraft('User re-enters a new question.');
    await conversation.submit();
    expect(api.admit.mock.calls[1]?.[1].conversation_id).toBeUndefined();
    expect(api.admit.mock.calls[1]?.[1].question).toBe('User re-enters a new question.');
  });

  it('never claims cancellation when admission has an unknown result and no run ID', async () => {
    const { api, conversation } = make();
    api.admit.mockRejectedValue(new ApiTransportError('network_error'));
    conversation.setDraft('Private input.');
    await conversation.submit();
    await conversation.cancel();
    expect(conversation.getSnapshot()).toEqual({ state: 'cancel_unknown', draft: '', turns: [] });
    expect(api.cancel).not.toHaveBeenCalled();
  });

  it('resolves only a displayed citation and checks its current resource/range', async () => {
    const { api, conversation } = make();
    const ready = readyResponse();
    if (ready.content_state !== 'ready') throw new Error('Missing synthetic citation');
    const citation = ready.citations[0];
    api.get.mockResolvedValue(ready);
    api.citation.mockResolvedValue(citation);
    conversation.setDraft('Private input.');
    await conversation.submit();
    await expect(
      conversation.resolveCitation(ready.run.id, `ev_${'d'.repeat(64)}`),
    ).resolves.toBeNull();
    expect(api.citation).not.toHaveBeenCalled();
    await expect(conversation.resolveCitation(ready.run.id, citation.id)).resolves.toEqual(
      citation,
    );
    api.citation.mockResolvedValue({ ...citation, resource_id: `r1_${'B'.repeat(42)}A` });
    await expect(conversation.resolveCitation(ready.run.id, citation.id)).resolves.toBeNull();
  });

  it('erases private turns on a confirmed authority denial', async () => {
    const { api, conversation } = make();
    api.get.mockRejectedValue(forbidden());
    conversation.setDraft('Private input.');
    await conversation.submit();
    expect(conversation.getSnapshot()).toEqual({ state: 'unavailable', draft: '', turns: [] });
    await conversation.retryAdmission();
    expect(api.admit).toHaveBeenCalledOnce();
  });

  it.each([' ', '\0', '\ud800', '\udfff', 'ف'.repeat(4097)])(
    'rejects invalid or oversized input without dispatch (%#)',
    async (draft) => {
      const { api, conversation } = make();
      conversation.setDraft(draft);
      await conversation.submit();
      expect(conversation.getSnapshot().state).toBe('invalid_question');
      expect(api.admit).not.toHaveBeenCalled();
    },
  );

  it('freezes a selected scope instead of following caller mutation', async () => {
    const controlled = controlledConversationApi();
    const mutableScope = { ...scope };
    const conversation = new AssistantConversation(controlled.api, mutableScope);
    disposals.push(conversation);
    mutableScope.sourceId = 'another_source';
    conversation.setDraft('Private input.');
    await conversation.submit();
    expect(controlled.api.admit.mock.calls[0]?.[1].source_id).toBe(scope.sourceId);
  });

  it('does not dispatch for a guest or a disposed view', async () => {
    const controlled = controlledConversationApi();
    controlled.setOwner(null);
    const conversation = new AssistantConversation(controlled.api, scope);
    disposals.push(conversation);
    conversation.setDraft('Private input.');
    await conversation.submit();
    expect(conversation.getSnapshot().state).toBe('unavailable');
    conversation.dispose();
    await conversation.submit();
    expect(controlled.api.admit).not.toHaveBeenCalled();
  });
});
