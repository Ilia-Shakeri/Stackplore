import {
  type ApiClient,
  type AssistantRun,
  type AssistantStatusMessage,
  type readAssistantStatus,
} from '@stackplore/api-client';
import { describe, expect, it, vi } from 'vitest';

import { createAssistantApi } from './assistant-api';
import { AuthStateError, type AuthStore } from '../auth/auth-store';
import {
  createAuthenticatedAuthStore,
  createResolvedAuthStore,
  createTestSession,
} from '../../test/auth';

const run: AssistantRun = {
  id: `ar_${'a'.repeat(64)}`,
  conversation_id: `ac_${'c'.repeat(64)}`,
  goal: 'explain_resource',
  state: 'completed',
  version: 3,
  scope: {
    source_id: 'src_demo_01',
    resource_id: `r1_${'A'.repeat(43)}`,
    window_minutes: 15,
    range: { from: '2026-10-28T10:00:00Z', to: '2026-10-28T10:15:00Z' },
  },
  created_at: '2026-10-28T10:15:00Z',
  updated_at: '2026-10-28T10:15:01Z',
  deadline: '2026-10-28T10:17:00Z',
  usage_uncertain: false,
  usage_conservative: false,
};
const input = {
  source_id: run.scope.source_id,
  resource_id: run.scope.resource_id,
  window_minutes: 15 as const,
  locale: 'en' as const,
  question: 'Explain selected resource.',
};
const message: AssistantStatusMessage = {
  type: 'status',
  id: '3',
  data: { run_id: run.id, sequence: 3, version: 3, state: 'completed', at: run.updated_at },
};
const reply = () => ({
  status: 200,
  requestId: 'req_owner_test',
  data: { run, content_state: 'volatile_lost' },
});

function ownerOf(auth: AuthStore) {
  const owner = auth.getSessionOwner();
  if (owner === null) throw new Error('Missing synthetic session owner');
  return owner;
}

describe('owned private transport', () => {
  it('keeps a pending read alive through an ordinary session refresh', async () => {
    let finish: (value: ReturnType<typeof reply>) => void = () => {
      throw new Error('Read not started');
    };
    const request = vi.fn<ApiClient['request']>().mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const auth = createAuthenticatedAuthStore();
    const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
    const pending = api.get(ownerOf(auth), run.id);
    await Promise.resolve();
    await auth.refreshSession();
    expect(request.mock.calls[0]?.[0].signal?.aborted).toBe(false);
    finish(reply());
    await expect(pending).resolves.toMatchObject({ content_state: 'volatile_lost' });
  });

  it('cancels before dispatch when the caller stops immediately after starting', async () => {
    const request = vi.fn<ApiClient['request']>();
    const auth = createAuthenticatedAuthStore();
    const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
    const controller = new AbortController();
    const pending = api.get(ownerOf(auth), run.id, controller.signal);
    const rejected = expect(pending).rejects.toMatchObject({ name: 'AbortError' });
    controller.abort();
    await rejected;
    expect(request).not.toHaveBeenCalled();
  });

  it('rechecks the owner before returning even when a subscription notification is missed', async () => {
    let finish: (value: ReturnType<typeof reply>) => void = () => {
      throw new Error('Read not started');
    };
    const request = vi.fn<ApiClient['request']>().mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const auth = createAuthenticatedAuthStore('administrator', { logout: async () => {} });
    vi.spyOn(auth, 'subscribe').mockReturnValue(() => {});
    const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
    const pending = api.get(ownerOf(auth), run.id);
    const rejected = expect(pending).rejects.toBeInstanceOf(AuthStateError);
    await Promise.resolve();
    await auth.logout();
    finish(reply());
    await rejected;
  });
  it('returns protected cancellation and a freshly checked citation', async () => {
    const citation = {
      id: `ev_${'b'.repeat(64)}`,
      kind: 'resource',
      source_id: run.scope.source_id,
      resource_id: run.scope.resource_id,
      observed_at: run.created_at,
      range: run.scope.range,
    };
    const request = vi
      .fn<ApiClient['request']>()
      .mockResolvedValueOnce({ status: 200, requestId: 'req_owner_test', data: { run } })
      .mockResolvedValueOnce({ status: 200, requestId: 'req_owner_test', data: citation });
    const auth = createAuthenticatedAuthStore();
    const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
    expect((await api.cancel(ownerOf(auth), run.id)).run.id).toBe(run.id);
    expect((await api.citation(ownerOf(auth), run.id, citation.id)).id).toBe(citation.id);
    expect(request.mock.calls[0]?.[0].protection).toMatchObject({ kind: 'session' });
    expect(request.mock.calls[1]?.[0].method).toBeUndefined();
    expect(request).toHaveBeenCalledTimes(2);
  });

  it('preserves a same-owner transport failure without a retry', async () => {
    const failure = new Error('Synthetic transport failure');
    const request = vi.fn<ApiClient['request']>().mockRejectedValue(failure);
    const auth = createAuthenticatedAuthStore();
    const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
    await expect(api.get(ownerOf(auth), run.id)).rejects.toBe(failure);
    expect(request).toHaveBeenCalledOnce();
  });

  it('ends one status read and rejects callbacks after its disposal', async () => {
    const auth = createAuthenticatedAuthStore();
    let publish: (value: AssistantStatusMessage) => unknown = () => undefined;
    const status = vi.fn<typeof readAssistantStatus>().mockImplementation((_run, options) => {
      publish = options.onMessage;
      publish(message);
      return Promise.resolve('ended');
    });
    const api = createAssistantApi({ request: vi.fn() }, auth, status);
    const shown = vi.fn();
    await expect(api.status(ownerOf(auth), run.id, shown)).resolves.toBe('ended');
    expect(shown).toHaveBeenCalledOnce();
    expect(status).toHaveBeenCalledOnce();
    expect(() => publish(message)).toThrow(AuthStateError);
    expect(shown).toHaveBeenCalledOnce();
  });

  it('rejects ownership changes made inside a synchronous status consumer', async () => {
    const auth = createAuthenticatedAuthStore();
    const status = vi.fn<typeof readAssistantStatus>().mockImplementation((_run, options) => {
      options.onMessage(message);
      return Promise.resolve('ended');
    });
    const api = createAssistantApi({ request: vi.fn() }, auth, status);
    await expect(
      api.status(ownerOf(auth), run.id, () => {
        void auth.retry();
      }),
    ).rejects.toBeInstanceOf(AuthStateError);
  });
  it('constructs without I/O and rejects a guest before every call', async () => {
    const request = vi.fn<ApiClient['request']>();
    const status = vi.fn();
    const api = createAssistantApi(
      { request: request as ApiClient['request'] },
      createResolvedAuthStore({
        status: 'guest_demo',
        fixtureVersion: 'demo-v2',
        scenarioStep: 0,
        observedAt: run.created_at,
      }),
      status,
    );
    expect(api.getOwner()).toBeNull();
    expect(request).not.toHaveBeenCalled();
    const owner = Symbol();
    await expect(api.get(owner, run.id)).rejects.toBeInstanceOf(AuthStateError);
    await expect(api.admit(owner, input, 'd'.repeat(64))).rejects.toBeInstanceOf(AuthStateError);
    await expect(api.cancel(owner, run.id)).rejects.toBeInstanceOf(AuthStateError);
    await expect(api.citation(owner, run.id, `ev_${'b'.repeat(64)}`)).rejects.toBeInstanceOf(
      AuthStateError,
    );
    await expect(api.status(owner, run.id, () => {})).rejects.toBeInstanceOf(AuthStateError);
    expect(request).not.toHaveBeenCalled();
    expect(status).not.toHaveBeenCalled();
  });

  it('returns a validated detached reply and cleans the ownership listener', async () => {
    const raw = reply();
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(raw);
    const auth = createAuthenticatedAuthStore();
    const unsubscribe = vi.fn();
    vi.spyOn(auth, 'subscribe').mockReturnValue(unsubscribe);
    const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
    const result = await api.get(ownerOf(auth), run.id);
    expect(result.content_state).toBe('volatile_lost');
    expect(result.run).not.toBe(raw.data.run);
    expect(unsubscribe).toHaveBeenCalledOnce();
    expect(request.mock.calls[0]?.[0].signal?.aborted).toBe(true);
  });

  it('pins mutation protection and preserves an explicitly reused nonce without retries', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue({
      status: 202,
      requestId: 'req_owner_test',
      data: { run },
    });
    const auth = createAuthenticatedAuthStore();
    const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
    const key = 'd'.repeat(64);
    await api.admit(ownerOf(auth), input, key);
    expect(request).toHaveBeenCalledOnce();
    expect(request.mock.calls[0]?.[0]).toMatchObject({
      body: input,
      headers: { 'Idempotency-Key': key },
      protection: { kind: 'session', csrfToken: createTestSession().csrfProof },
    });
  });

  it('rejects stale-owner cancellation before it can use the replacement session', async () => {
    const request = vi.fn<ApiClient['request']>();
    const auth = createAuthenticatedAuthStore('administrator', {
      login: () => Promise.resolve(createTestSession('viewer')),
    });
    const owner = ownerOf(auth);
    const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
    await auth.login({ loginName: 'observer', password: '' });
    await expect(api.cancel(owner, run.id)).rejects.toBeInstanceOf(AuthStateError);
    expect(request).not.toHaveBeenCalled();
  });

  it('settles promptly on owner loss even if transport ignores abort and later replies', async () => {
    let finish: (value: ReturnType<typeof reply>) => void = () => {
      throw new Error('Read not started');
    };
    const request = vi.fn<ApiClient['request']>().mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const auth = createAuthenticatedAuthStore('administrator', { logout: async () => {} });
    const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
    const pending = api.get(ownerOf(auth), run.id);
    const rejected = expect(pending).rejects.toBeInstanceOf(AuthStateError);
    await auth.logout();
    await rejected;
    expect(request.mock.calls[0]?.[0].signal?.aborted).toBe(true);
    finish(reply());
    await Promise.resolve();
    expect(request).toHaveBeenCalledOnce();
  });

  it('bounds an unresponsive read without automatic retry', async () => {
    vi.useFakeTimers();
    try {
      const request = vi.fn<ApiClient['request']>().mockImplementation(() => new Promise(() => {}));
      const auth = createAuthenticatedAuthStore();
      const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
      const pending = api.get(ownerOf(auth), run.id);
      const rejected = expect(pending).rejects.toMatchObject({ name: 'AbortError' });
      await vi.advanceTimersByTimeAsync(15_000);
      await rejected;
      expect(request).toHaveBeenCalledOnce();
      expect(request.mock.calls[0]?.[0].signal?.aborted).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });

  it('makes no request for a pre-aborted owner operation', async () => {
    const request = vi.fn<ApiClient['request']>();
    const auth = createAuthenticatedAuthStore();
    const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
    await expect(api.get(ownerOf(auth), run.id, AbortSignal.abort())).rejects.toMatchObject({
      name: 'AbortError',
    });
    expect(request).not.toHaveBeenCalled();
  });

  it('forwards a caller abort without erasing or replacing its owner', async () => {
    const request = vi.fn<ApiClient['request']>().mockImplementation(() => new Promise(() => {}));
    const auth = createAuthenticatedAuthStore();
    const owner = ownerOf(auth);
    const api = createAssistantApi({ request: request as ApiClient['request'] }, auth);
    const controller = new AbortController();
    const pending = api.get(owner, run.id, controller.signal);
    const rejected = expect(pending).rejects.toMatchObject({ name: 'AbortError' });
    await Promise.resolve();
    controller.abort();
    await rejected;
    expect(auth.getSessionOwner()).toBe(owner);
    expect(request.mock.calls[0]?.[0].signal?.aborted).toBe(true);
  });

  it('notifies only actual ownership changes and supports unsubscribe', async () => {
    const auth = createAuthenticatedAuthStore('administrator', { logout: async () => {} });
    const api = createAssistantApi({ request: vi.fn() }, auth);
    const listener = vi.fn();
    const unsubscribe = api.subscribeOwner(listener);
    await auth.refreshSession();
    expect(listener).not.toHaveBeenCalled();
    await auth.logout();
    expect(listener).toHaveBeenCalledOnce();
    unsubscribe();
  });

  it('blocks late status callbacks after observed owner loss', async () => {
    const auth = createAuthenticatedAuthStore('administrator', { logout: async () => {} });
    let publish: (value: AssistantStatusMessage) => unknown = () => undefined;
    const status = vi.fn<typeof readAssistantStatus>().mockImplementation((_run, options) => {
      publish = options.onMessage;
      return new Promise(() => {});
    });
    const api = createAssistantApi({ request: vi.fn() }, auth, status);
    const shown = vi.fn();
    const pending = api.status(ownerOf(auth), run.id, shown);
    const rejected = expect(pending).rejects.toBeInstanceOf(AuthStateError);
    await Promise.resolve();
    publish(message);
    expect(shown).toHaveBeenCalledOnce();
    await auth.logout();
    await rejected;
    expect(() => publish(message)).toThrow(AuthStateError);
    expect(shown).toHaveBeenCalledOnce();
  });

  it('rejects asynchronous status consumers instead of hiding their returned promise', async () => {
    const auth = createAuthenticatedAuthStore();
    const status = vi.fn<typeof readAssistantStatus>().mockImplementation((_run, options) => {
      options.onMessage(message);
      return Promise.resolve('ended');
    });
    const api = createAssistantApi({ request: vi.fn() }, auth, status);
    await expect(
      api.status(ownerOf(auth), run.id, () =>
        Promise.reject(new Error('Synthetic callback failure')),
      ),
    ).rejects.toMatchObject({ code: 'invalid_configuration' });
  });
});
