import { ApiProblemError } from '@stackplore/api-client';
import { describe, expect, it, vi } from 'vitest';

import type { AuthApi } from './auth-api';
import { AuthStateError, AuthStore } from './auth-store';
import {
  closedSetupStatus,
  createTestAuthApi,
  createTestSession,
  testProof,
} from '../../test/auth';

const unauthorized = () =>
  new ApiProblemError({
    code: 'unauthenticated',
    request_id: 'req_test_01',
    status: 401,
    title: 'Sign in required',
    type: 'urn:stackplore:problem:unauthenticated',
  });

describe('auth store', () => {
  it('publishes setup state without probing a session', async () => {
    const getSession = vi.fn<AuthApi['getSession']>();
    const store = new AuthStore(
      createTestAuthApi({
        getSession,
        getSetupStatus: vi.fn().mockResolvedValue({
          bootstrapProofRequired: true,
          state: 'required',
        }),
      }),
    );

    await store.ensureLoaded();

    expect(store.getSnapshot()).toEqual({
      bootstrapProofRequired: true,
      status: 'setup_required',
    });
    expect(getSession).not.toHaveBeenCalled();
  });

  it('maps a closed installation with no session to signed out', async () => {
    const store = new AuthStore(
      createTestAuthApi({
        getSession: vi.fn().mockRejectedValue(unauthorized()),
        getSetupStatus: vi.fn().mockResolvedValue(closedSetupStatus),
      }),
    );

    await store.ensureLoaded();

    expect(store.getSnapshot()).toEqual({ status: 'unauthenticated' });
  });

  it('deduplicates concurrent boot calls and accepts one safe session', async () => {
    const session = createTestSession();
    let resolveStatus: ((value: typeof closedSetupStatus) => void) | undefined;
    const status = new Promise<typeof closedSetupStatus>((resolve) => {
      resolveStatus = resolve;
    });
    const getSetupStatus = vi.fn().mockReturnValue(status);
    const getSession = vi.fn().mockResolvedValue(session);
    const store = new AuthStore(createTestAuthApi({ getSession, getSetupStatus }));

    const first = store.ensureLoaded();
    const second = store.ensureLoaded();
    resolveStatus?.(closedSetupStatus);
    await Promise.all([first, second]);

    expect(getSetupStatus).toHaveBeenCalledTimes(1);
    expect(getSession).toHaveBeenCalledTimes(1);
    expect(store.getSnapshot()).toMatchObject({
      status: 'authenticated',
      user: { loginName: 'operator' },
    });
  });

  it('keeps the CSRF proof private and gives it only to logout', async () => {
    const session = createTestSession();
    const logout = vi.fn<AuthApi['logout']>().mockResolvedValue(undefined);
    const store = new AuthStore(createTestAuthApi({ logout }), {
      csrfProof: session.csrfProof,
      snapshot: {
        absoluteExpiresAt: session.absoluteExpiresAt,
        idleExpiresAt: session.idleExpiresAt,
        status: 'authenticated',
        user: session.user,
      },
    });

    expect(JSON.stringify(store.getSnapshot())).not.toContain(testProof);
    await store.logout();

    expect(logout).toHaveBeenCalledWith(testProof);
    expect(store.getSnapshot()).toEqual({ status: 'unauthenticated' });
  });

  it('keeps recent proofs out of state and gives one to one exact action', async () => {
    const session = createTestSession();
    const recentProof = `${'B'.repeat(42)}A`;
    const createRecentAuthGrant = vi.fn().mockResolvedValue(recentProof);
    const action = vi.fn().mockResolvedValue('done');
    const store = new AuthStore(createTestAuthApi({ createRecentAuthGrant }), {
      csrfProof: session.csrfProof,
      snapshot: {
        absoluteExpiresAt: session.absoluteExpiresAt,
        idleExpiresAt: session.idleExpiresAt,
        status: 'authenticated',
        user: session.user,
      },
    });

    await expect(store.runRecentAction('user.delete', 'admin-password', action)).resolves.toBe(
      'done',
    );

    expect(createRecentAuthGrant).toHaveBeenCalledWith('admin-password', 'user.delete', testProof);
    expect(action).toHaveBeenCalledWith({
      csrfToken: testProof,
      kind: 'session',
      recentAuthToken: recentProof,
    });
    expect(JSON.stringify(store.getSnapshot())).not.toContain(recentProof);
    expect(JSON.stringify(store.getSnapshot())).not.toContain(testProof);
  });

  it('does not run a recent action after the local session changes', async () => {
    const session = createTestSession();
    let issueGrant: ((value: string) => void) | undefined;
    const grant = new Promise<string>((resolve) => {
      issueGrant = resolve;
    });
    const action = vi.fn().mockResolvedValue(undefined);
    const store = new AuthStore(
      createTestAuthApi({
        createRecentAuthGrant: vi.fn().mockReturnValue(grant),
        logout: vi.fn().mockResolvedValue(undefined),
      }),
      {
        csrfProof: session.csrfProof,
        snapshot: {
          absoluteExpiresAt: session.absoluteExpiresAt,
          idleExpiresAt: session.idleExpiresAt,
          status: 'authenticated',
          user: session.user,
        },
      },
    );

    const pending = store.runRecentAction('user.update', 'admin-password', action);
    await store.logout();
    issueGrant?.(`${'B'.repeat(42)}A`);

    await expect(pending).rejects.toBeInstanceOf(AuthStateError);
    expect(action).not.toHaveBeenCalled();
  });

  it('keeps the current user when logout cannot reach the service', async () => {
    const session = createTestSession();
    const store = new AuthStore(
      createTestAuthApi({ logout: vi.fn().mockRejectedValue(new Error('offline')) }),
      {
        csrfProof: session.csrfProof,
        snapshot: {
          absoluteExpiresAt: session.absoluteExpiresAt,
          idleExpiresAt: session.idleExpiresAt,
          status: 'authenticated',
          user: session.user,
        },
      },
    );

    await expect(store.logout()).rejects.toThrow('offline');
    expect(store.getSnapshot().status).toBe('authenticated');
  });

  it('hides authenticated state when the local session deadline passes', async () => {
    vi.useFakeTimers();
    try {
      vi.setSystemTime(new Date('2026-08-02T10:00:00Z'));
      const session = {
        ...createTestSession(),
        absoluteExpiresAt: '2026-08-02T10:10:00Z',
        idleExpiresAt: '2026-08-02T10:00:10Z',
      };
      const store = new AuthStore(createTestAuthApi(), {
        csrfProof: session.csrfProof,
        snapshot: {
          absoluteExpiresAt: session.absoluteExpiresAt,
          idleExpiresAt: session.idleExpiresAt,
          status: 'authenticated',
          user: session.user,
        },
      });

      await vi.advanceTimersByTimeAsync(10_001);

      expect(store.getSnapshot()).toEqual({ status: 'unauthenticated' });
    } finally {
      vi.useRealTimers();
    }
  });

  it('rotates local session state after changing the current password', async () => {
    const initial = createTestSession();
    const rotated = {
      ...initial,
      csrfProof: `${'C'.repeat(42)}A`,
      idleExpiresAt: '2099-01-02T04:04:05Z',
    };
    const recentProof = `${'B'.repeat(42)}A`;
    const createRecentAuthGrant = vi.fn().mockResolvedValue(recentProof);
    const changeOwnPassword = vi.fn().mockResolvedValue(rotated);
    const store = new AuthStore(createTestAuthApi({ changeOwnPassword, createRecentAuthGrant }), {
      csrfProof: initial.csrfProof,
      snapshot: {
        absoluteExpiresAt: initial.absoluteExpiresAt,
        idleExpiresAt: initial.idleExpiresAt,
        status: 'authenticated',
        user: initial.user,
      },
    });

    await store.changePassword('current-password', 'replacement-password');

    expect(createRecentAuthGrant).toHaveBeenCalledWith(
      'current-password',
      'account.password_change',
      testProof,
    );
    expect(changeOwnPassword).toHaveBeenCalledWith('replacement-password', {
      csrfToken: testProof,
      kind: 'session',
      recentAuthToken: recentProof,
    });
    expect(store.getSnapshot()).toMatchObject({
      idleExpiresAt: rotated.idleExpiresAt,
      status: 'authenticated',
    });
    expect(JSON.stringify(store.getSnapshot())).not.toContain(rotated.csrfProof);
  });
});
