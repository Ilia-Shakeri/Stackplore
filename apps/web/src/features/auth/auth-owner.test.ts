import { describe, expect, it, vi } from 'vitest';

import { AuthStateError, AuthStore } from './auth-store';
import {
  createAuthenticatedAuthStore,
  createTestAuthApi,
  createTestSession,
} from '../../test/auth';

describe('volatile session ownership', () => {
  it('blocks a recent-auth dispatch when authority changes while its grant is pending', async () => {
    const session = createTestSession();
    let grant: (value: string) => void = () => {
      throw new Error('Grant did not start');
    };
    const store = createAuthenticatedAuthStore('administrator', {
      createRecentAuthGrant: () =>
        new Promise((resolve) => {
          grant = resolve;
        }),
      getSession: () => Promise.resolve({ ...session, user: { ...session.user, version: 2 } }),
    });
    const action = vi.fn(() => Promise.resolve(1));
    const pending = store.runRecentAction('account.password_change', '', action);
    const rejected = expect(pending).rejects.toBeInstanceOf(AuthStateError);
    await store.refreshSession();
    grant('G'.repeat(43));
    await rejected;
    expect(action).not.toHaveBeenCalled();
  });

  it('rejects a late recent-auth result after a same-proof authority change', async () => {
    const session = createTestSession();
    let finish: (value: number) => void = () => {
      throw new Error('Action did not start');
    };
    const store = createAuthenticatedAuthStore('administrator', {
      createRecentAuthGrant: () => Promise.resolve('G'.repeat(43)),
      getSession: () => Promise.resolve({ ...session, user: { ...session.user, version: 2 } }),
    });
    const pending = store.runRecentAction(
      'account.password_change',
      '',
      () =>
        new Promise<number>((resolve) => {
          finish = resolve;
        }),
    );
    const rejected = expect(pending).rejects.toBeInstanceOf(AuthStateError);
    await Promise.resolve();
    await store.refreshSession();
    finish(1);
    await rejected;
  });
  it('rotates on an explicit login even if a test server reuses its proof', async () => {
    const store = createAuthenticatedAuthStore('administrator', {
      login: () => Promise.resolve(createTestSession()),
    });
    const owner = store.getSessionOwner();
    await store.login({ loginName: 'operator', password: '' });
    expect(store.getSessionOwner()).not.toBe(owner);
  });

  it('rejects a late protected result after account replacement with an unchanged proof', async () => {
    const session = createTestSession();
    const store = createAuthenticatedAuthStore('administrator', {
      getSession: () =>
        Promise.resolve({ ...session, user: { ...session.user, id: 'usr_other_01' } }),
    });
    let finish: (value: number) => void = () => {
      throw new Error('Action did not start');
    };
    const pending = store.runSessionAction(
      () =>
        new Promise<number>((resolve) => {
          finish = resolve;
        }),
    );
    const rejected = expect(pending).rejects.toBeInstanceOf(AuthStateError);
    await store.refreshSession();
    finish(42);
    await rejected;
  });
  it('creates no owner for signed-out, loading or guest views', () => {
    const api = createTestAuthApi();
    expect(new AuthStore(api).getSessionOwner()).toBeNull();
    expect(
      new AuthStore(api, { snapshot: { status: 'unauthenticated' } }).getSessionOwner(),
    ).toBeNull();
    expect(
      new AuthStore(api, {
        snapshot: {
          status: 'guest_demo',
          fixtureVersion: 'demo-v2',
          scenarioStep: 0,
          observedAt: '2026-01-01T00:00:00Z',
        },
      }).getSessionOwner(),
    ).toBeNull();
  });

  it('keeps the owner stable across ordinary activity refreshes', async () => {
    const session = createTestSession();
    const store = createAuthenticatedAuthStore('administrator', {
      getSession: () => Promise.resolve({ ...session, idleExpiresAt: '2099-01-02T04:00:00Z' }),
    });
    const owner = store.getSessionOwner();
    expect(typeof owner).toBe('symbol');
    await store.refreshSession();
    expect(store.getSessionOwner()).toBe(owner);
    await store.recordActivity();
    expect(store.getSessionOwner()).toBe(owner);
  });

  it.each(['proof', 'account', 'role', 'revision', 'enabled'] as const)(
    'rotates on changed %s before subscribers run',
    async (field) => {
      const session = createTestSession();
      const next = {
        ...session,
        ...(field === 'proof' ? { csrfProof: 'B'.repeat(43) } : {}),
        user: {
          ...session.user,
          ...(field === 'account' ? { id: 'usr_other_01' } : {}),
          ...(field === 'role' ? { role: 'viewer' as const } : {}),
          ...(field === 'revision' ? { version: 2 } : {}),
          ...(field === 'enabled' ? { enabled: false } : {}),
        },
      };
      const store = createAuthenticatedAuthStore('administrator', {
        getSession: () => Promise.resolve(next),
      });
      const owner = store.getSessionOwner();
      const seen: (symbol | null)[] = [];
      const unsubscribe = store.subscribe(() => {
        seen.push(store.getSessionOwner());
      });
      await store.refreshSession();
      unsubscribe();
      expect(store.getSessionOwner()).not.toBe(owner);
      expect(seen).toEqual([store.getSessionOwner()]);
    },
  );

  it('clears on retry and never reuses the token after the same account returns', async () => {
    const session = createTestSession();
    const store = createAuthenticatedAuthStore('administrator', {
      getSetupStatus: () => Promise.resolve({ state: 'closed', bootstrapProofRequired: false }),
      getSession: () => Promise.resolve(session),
    });
    const owner = store.getSessionOwner();
    const pending = store.retry();
    expect(store.getSessionOwner()).toBeNull();
    await pending;
    expect(store.getSessionOwner()).not.toBe(owner);
    expect(typeof store.getSessionOwner()).toBe('symbol');
  });

  it('clears before logout subscribers observe signed-out state', async () => {
    const store = createAuthenticatedAuthStore('administrator', { logout: async () => {} });
    const seen = vi.fn(() => store.getSessionOwner());
    store.subscribe(seen);
    await store.logout();
    expect(seen).toHaveReturnedWith(null);
    expect(store.getSessionOwner()).toBeNull();
  });

  it('clears on the proven session deadline', async () => {
    vi.useFakeTimers();
    try {
      vi.setSystemTime(new Date('2026-10-31T00:00:00Z'));
      const session = { ...createTestSession(), idleExpiresAt: '2026-10-31T00:00:01Z' };
      const store = new AuthStore(createTestAuthApi(), {
        csrfProof: session.csrfProof,
        snapshot: {
          status: 'authenticated',
          user: session.user,
          idleExpiresAt: session.idleExpiresAt,
          absoluteExpiresAt: session.absoluteExpiresAt,
        },
      });
      expect(typeof store.getSessionOwner()).toBe('symbol');
      await vi.advanceTimersByTimeAsync(1001);
      expect(store.getSessionOwner()).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });
});
