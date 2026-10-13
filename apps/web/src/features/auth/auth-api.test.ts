import type { ApiClient, ApiRequest } from '@stackplore/api-client';
import { describe, expect, it, vi } from 'vitest';

import { AuthResponseError, createAuthApi } from './auth-api';
import { createTestSession, testProof } from '../../test/auth';

const requestId = 'req_test_01';

const wireSession = () => {
  const session = createTestSession();
  return {
    absolute_expires_at: session.absoluteExpiresAt,
    csrf_proof: session.csrfProof,
    idle_expires_at: session.idleExpiresAt,
    user: {
      created_at: session.user.createdAt,
      display_name: session.user.displayName,
      enabled: true,
      id: session.user.id,
      login_name: session.user.loginName,
      role: session.user.role,
      updated_at: session.user.updatedAt,
      version: session.user.version,
    },
  };
};

const response = (data: unknown, status = 200) => ({ data, requestId, status });

describe('auth API adapter', () => {
  it.each(['user.delete', 'source.update'] as const)(
    'binds a recent grant to %s and current session proof',
    async (purpose) => {
      const recentProof = `${'B'.repeat(42)}A`;
      const request = vi.fn<ApiClient['request']>().mockResolvedValue(
        response(
          {
            expires_at: '2026-01-02T03:09:05Z',
            purpose,
            recent_auth_proof: recentProof,
          },
          201,
        ),
      );
      const api = createAuthApi({ request: request as ApiClient['request'] });

      await expect(api.createRecentAuthGrant('admin-password', purpose, testProof)).resolves.toBe(
        recentProof,
      );
      expect(request).toHaveBeenCalledWith({
        body: { current_password: 'admin-password', purpose },
        method: 'POST',
        path: '/auth/recent-grants',
        protection: { csrfToken: testProof, kind: 'session' },
      });
    },
  );

  it('rejects a source grant returned for another purpose', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response(
        {
          expires_at: '2026-01-02T03:09:05Z',
          purpose: 'user.update',
          recent_auth_proof: testProof,
        },
        201,
      ),
    );
    const api = createAuthApi({ request: request as ApiClient['request'] });
    await expect(
      api.createRecentAuthGrant('admin-password', 'source.update', testProof),
    ).rejects.toBeInstanceOf(AuthResponseError);
  });

  it('uses the exact public session paths and protects logout', async () => {
    const request = vi.fn<ApiClient['request']>();
    request
      .mockResolvedValueOnce(response({ bootstrap_proof_required: false, state: 'closed' }))
      .mockResolvedValueOnce(response(wireSession()))
      .mockResolvedValueOnce(response(undefined, 204));
    const api = createAuthApi({ request: request as ApiClient['request'] });

    await expect(api.getSetupStatus()).resolves.toEqual({
      bootstrapProofRequired: false,
      state: 'closed',
    });
    await expect(api.getSession()).resolves.toMatchObject({
      user: { loginName: 'operator' },
    });
    await api.logout(testProof);

    expect(request.mock.calls.map(([input]) => input)).toEqual([
      { path: '/setup/status' },
      { path: '/auth/session' },
      {
        method: 'POST',
        path: '/auth/logout',
        protection: { csrfToken: testProof, kind: 'session' },
      },
    ]);
  });

  it('changes the current password with the exact recent session proof', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(response(wireSession()));
    const api = createAuthApi({ request: request as ApiClient['request'] });
    const recentProof = `${'B'.repeat(42)}A`;

    await expect(
      api.changeOwnPassword('replacement-password', {
        csrfToken: testProof,
        kind: 'session',
        recentAuthToken: recentProof,
      }),
    ).resolves.toMatchObject({ user: { loginName: 'operator' } });

    expect(request).toHaveBeenCalledWith({
      body: { new_password: 'replacement-password' },
      method: 'PATCH',
      path: '/auth/password',
      protection: {
        csrfToken: testProof,
        kind: 'session',
        recentAuthToken: recentProof,
      },
    });
  });

  it('gets a fresh pre-auth proof for every login and seals it in protection', async () => {
    const request = vi.fn<ApiClient['request']>();
    request
      .mockResolvedValueOnce(
        response({ expires_at: '2026-01-02T03:09:05Z', preauth_proof: testProof }, 201),
      )
      .mockResolvedValueOnce(response(wireSession()))
      .mockResolvedValueOnce(
        response({ expires_at: '2026-01-02T03:09:05Z', preauth_proof: testProof }, 201),
      )
      .mockResolvedValueOnce(response(wireSession()));
    const api = createAuthApi({ request: request as ApiClient['request'] });

    await api.login({ loginName: 'operator', password: 'long-password' });
    await api.login({ loginName: 'operator', password: 'long-password' });

    expect(request).toHaveBeenCalledTimes(4);
    for (const index of [0, 2]) {
      expect(request.mock.calls[index]?.[0]).toEqual({
        body: { purpose: 'login' },
        method: 'POST',
        path: '/auth/preflight',
      });
    }
    for (const index of [1, 3]) {
      expect(request.mock.calls[index]?.[0]).toMatchObject({
        method: 'POST',
        path: '/auth/login',
        protection: { kind: 'preauth', token: testProof },
      });
    }
  });

  it('sends the bootstrap proof only when supplied', async () => {
    const requests: ApiRequest<unknown>[] = [];
    const client: ApiClient = {
      request: (request) => {
        requests.push(request);
        if (request.path === '/auth/preflight') {
          return Promise.resolve(
            response(
              { expires_at: '2026-01-02T03:09:05Z', preauth_proof: testProof },
              201,
            ) as never,
          );
        }
        return Promise.resolve(response(wireSession(), 201) as never);
      },
    };
    const api = createAuthApi(client);

    await api.setup({
      bootstrapProof: 'operator-proof-value',
      displayName: 'Operations',
      loginName: 'operator',
      password: 'long-password',
    });

    expect(requests[1]).toMatchObject({
      body: {
        bootstrap_proof: 'operator-proof-value',
        display_name: 'Operations',
        login_name: 'operator',
        password: 'long-password',
      },
      path: '/setup',
      protection: { kind: 'preauth', token: testProof },
    });
  });

  it('rejects an unsealed session reply with a safe local error', async () => {
    const client: ApiClient = {
      request: vi.fn().mockResolvedValue(response({ ...wireSession(), extra: testProof })),
    };
    const api = createAuthApi(client);

    const error = await api.getSession().catch((reason: unknown) => reason);

    expect(error).toBeInstanceOf(AuthResponseError);
    expect((error as Error).message).not.toContain(testProof);
  });

  it('rejects a non-canonical 32-byte proof tail', async () => {
    const client: ApiClient = {
      request: vi.fn().mockResolvedValue(
        response({
          ...wireSession(),
          csrf_proof: 'B'.repeat(43),
        }),
      ),
    };

    await expect(createAuthApi(client).getSession()).rejects.toBeInstanceOf(AuthResponseError);
  });
});
