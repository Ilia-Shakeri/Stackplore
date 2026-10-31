import {
  ApiProblemError,
  ApiTransportError,
  parseAssistantRunResponse,
  type AssistantRunResponse,
  type AssistantStatusMessage,
} from '@stackplore/api-client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AssistantConversation } from './conversation';
import {
  controlledConversationApi,
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

describe('conversation failure boundaries', () => {
  it.each([401, 403, 404])(
    'erases after an explicit status transport denial %s',
    async (status) => {
      const { api, conversation } = make();
      api.admit.mockResolvedValue({ run: resourceRun('queued', 1) });
      api.status.mockRejectedValue(new ApiTransportError('unexpected_response', { status }));
      conversation.setDraft('Private input.');
      await conversation.submit();
      expect(conversation.getSnapshot()).toEqual({ state: 'unavailable', draft: '', turns: [] });
    },
  );
  it('does not relabel an already completed run as cancelled', async () => {
    const { api, conversation } = make();
    api.cancel.mockResolvedValue({ run: resourceRun() });
    conversation.setDraft('Private input.');
    await conversation.submit();
    await conversation.cancel();
    expect(conversation.getSnapshot()).toEqual({ state: 'already_terminal', draft: '', turns: [] });
  });
  it('ignores recovery controls when no run exists', async () => {
    const { api, conversation } = make();
    await conversation.refresh();
    await conversation.listen();
    await conversation.retryAdmission();
    await expect(
      conversation.resolveCitation(resourceRun().id, `ev_${'b'.repeat(64)}`),
    ).resolves.toBeNull();
    expect(api.get).not.toHaveBeenCalled();
    expect(api.status).not.toHaveBeenCalled();
    expect(api.citation).not.toHaveBeenCalled();
  });

  it('clears after a suspended clock crosses expiry before a timer can run', () => {
    vi.useFakeTimers();
    const { conversation } = make();
    conversation.setDraft('Private input.');
    vi.setSystemTime(Date.now() + 30 * 60_000);
    conversation.checkExpiry();
    expect(conversation.getSnapshot()).toEqual({ state: 'expired', draft: '', turns: [] });
  });

  it('rechecks the initial owner when its mounting effect connects late', () => {
    const controlled = controlledConversationApi();
    const conversation = new AssistantConversation(controlled.api, scope);
    disposals.push(conversation);
    conversation.setDraft('Private input.');
    controlled.setOwner(Symbol());
    conversation.connect();
    expect(conversation.getSnapshot()).toEqual({ state: 'unavailable', draft: '', turns: [] });
  });

  it.each(['invalid', 'denied', 'scope'] as const)(
    'handles confirmed admission %s without a hidden retry',
    async (mode) => {
      const { api, conversation } = make();
      if (mode === 'invalid') api.admit.mockRejectedValue(new ApiTransportError('invalid_request'));
      if (mode === 'denied') api.admit.mockRejectedValue(forbidden());
      if (mode === 'scope')
        api.admit.mockResolvedValue({
          run: { ...resourceRun(), scope: { ...resourceRun().scope, source_id: 'other_source' } },
        });
      conversation.setDraft('Private input.');
      await conversation.submit();
      expect(conversation.getSnapshot().state).toBe(
        mode === 'invalid'
          ? 'invalid_question'
          : mode === 'denied'
            ? 'unavailable'
            : 'admission_unknown',
      );
      expect(api.admit).toHaveBeenCalledOnce();
      expect(api.get).not.toHaveBeenCalled();
    },
  );

  it('retains pending metadata until a new explicit subscription', async () => {
    const { api, conversation } = make();
    const run = resourceRun('queued', 1);
    api.admit.mockResolvedValue({ run });
    api.get.mockResolvedValue(parseAssistantRunResponse({ run, content_state: 'pending' }, run.id));
    conversation.setDraft('Private input.');
    await conversation.submit();
    expect(conversation.getSnapshot().state).toBe('needs_refresh');
    expect(api.status).toHaveBeenCalledOnce();
    await conversation.listen();
    expect(api.status).toHaveBeenCalledTimes(2);
    expect(api.admit).toHaveBeenCalledOnce();
  });

  it('shows unavailable content without inventing an answer', async () => {
    const { api, conversation } = make();
    const run = resourceRun();
    api.get.mockResolvedValue(
      parseAssistantRunResponse({ run, content_state: 'unavailable' }, run.id),
    );
    conversation.setDraft('Private input.');
    await conversation.submit();
    expect(conversation.getSnapshot().state).toBe('unavailable');
    expect(conversation.getSnapshot().turns[0]?.response?.content_state).toBe('unavailable');
  });

  it('erases on status authority denial', async () => {
    const { api, conversation } = make();
    api.admit.mockResolvedValue({ run: resourceRun('queued', 1) });
    api.status.mockRejectedValue(forbidden());
    conversation.setDraft('Private input.');
    await conversation.submit();
    expect(conversation.getSnapshot()).toEqual({ state: 'unavailable', draft: '', turns: [] });
    expect(api.get).not.toHaveBeenCalled();
  });

  it.each(['resolve', 'reject'] as const)(
    'discards a late status %s after scope erasure',
    async (outcome) => {
      const { api, conversation } = make();
      let publish: (message: AssistantStatusMessage) => unknown = () => undefined;
      let finish: () => void = () => {
        throw new Error('Status did not start');
      };
      api.admit.mockResolvedValue({ run: resourceRun('queued', 1) });
      api.status.mockImplementation((_owner, _run, callback) => {
        publish = callback;
        return new Promise((resolve, reject) => {
          finish = () => {
            if (outcome === 'resolve') resolve('ended');
            else reject(new ApiTransportError('network_error'));
          };
        });
      });
      conversation.setDraft('Private input.');
      const pending = conversation.submit();
      await Promise.resolve();
      conversation.erase('expired');
      publish({ type: 'reset', data: { run_id: resourceRun().id, reason: 'replay_gap' } });
      finish();
      await pending;
      expect(conversation.getSnapshot()).toEqual({ state: 'expired', draft: '', turns: [] });
      expect(api.get).not.toHaveBeenCalled();
    },
  );

  it('discards a late read rejection after owner loss', async () => {
    const { api, conversation, setOwner } = make();
    let fail: () => void = () => {
      throw new Error('Read did not start');
    };
    api.get.mockImplementation(
      () =>
        new Promise<AssistantRunResponse>((_resolve, reject) => {
          fail = () => {
            reject(forbidden());
          };
        }),
    );
    conversation.setDraft('Private input.');
    const pending = conversation.submit();
    await Promise.resolve();
    setOwner(null);
    fail();
    await pending;
    expect(conversation.getSnapshot()).toEqual({ state: 'unavailable', draft: '', turns: [] });
  });

  it.each(['denied', 'unknown'] as const)(
    'does not call failed cancellation %s a success',
    async (mode) => {
      const { api, conversation } = make();
      api.cancel.mockRejectedValue(
        mode === 'denied' ? forbidden() : new ApiTransportError('network_error'),
      );
      conversation.setDraft('Private input.');
      await conversation.submit();
      await conversation.cancel();
      expect(conversation.getSnapshot()).toEqual({
        state: mode === 'denied' ? 'unavailable' : 'cancel_unknown',
        draft: '',
        turns: [],
      });
    },
  );

  it.each(['denied', 'unknown'] as const)(
    'fences a citation failure with %s authority',
    async (mode) => {
      const { api, conversation } = make();
      const ready = readyResponse();
      if (ready.content_state !== 'ready') throw new Error('Missing synthetic citation');
      api.get.mockResolvedValue(ready);
      api.citation.mockRejectedValue(
        mode === 'denied' ? forbidden() : new ApiTransportError('network_error'),
      );
      conversation.setDraft('Private input.');
      await conversation.submit();
      await expect(
        conversation.resolveCitation(ready.run.id, ready.citations[0].id),
      ).resolves.toBeNull();
      if (mode === 'denied') expect(conversation.getSnapshot().turns).toHaveLength(0);
      else expect(conversation.getSnapshot().state).toBe('ready');
    },
  );
});
