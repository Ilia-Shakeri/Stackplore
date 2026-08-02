import { ApiProblemError, type ApiClient } from '@stackplore/api-client';
import { describe, expect, it, vi } from 'vitest';

import {
  classifyUsersFailure,
  createUsersApi,
  UsersResponseError,
  type RecentActionRunner,
} from './users-api';
import type { RecentAuthPurpose, SessionProtection } from '../auth/auth-api';
import { createTestUser, testProof } from '../../test/auth';

const requestId = 'req_test_01';
const protection: SessionProtection = {
  csrfToken: testProof,
  kind: 'session',
  recentAuthToken: `${'B'.repeat(42)}A`,
};

const wireUser = (version = 1) => {
  const user = createTestUser('viewer');
  return {
    created_at: user.createdAt,
    display_name: user.displayName,
    enabled: user.enabled,
    id: user.id,
    login_name: user.loginName,
    role: user.role,
    updated_at: user.updatedAt,
    version,
  };
};

const response = (data: unknown, status = 200) => ({ data, requestId, status });

function createRecentActions(calls: Array<[RecentAuthPurpose, string]>): RecentActionRunner {
  return {
    runRecentAction: (purpose, currentPassword, action) => {
      calls.push([purpose, currentPassword]);
      return action(protection);
    },
  };
}

describe('users API adapter', () => {
  it('parses bounded user pages and owns cursor requests', async () => {
    const request = vi.fn<ApiClient['request']>().mockResolvedValue(
      response({
        items: [wireUser()],
        page: { has_more: true, limit: 50, next_cursor: 'next_page_01' },
      }),
    );
    const api = createUsersApi(
      { request: request as ApiClient['request'] },
      createRecentActions([]),
    );

    await expect(api.listUsers({ cursor: 'page_01' })).resolves.toMatchObject({
      items: [{ enabled: true, loginName: 'observer', role: 'viewer' }],
      nextCursor: 'next_page_01',
    });
    expect(request).toHaveBeenCalledWith({
      path: '/users',
      query: { cursor: 'page_01', limit: 50 },
      signal: undefined,
    });
  });

  it('reauthenticates every write for its exact purpose and seals versions', async () => {
    const request = vi.fn<ApiClient['request']>();
    request
      .mockResolvedValueOnce(response(wireUser(), 201))
      .mockResolvedValueOnce(response(wireUser(2)))
      .mockResolvedValueOnce(response(undefined, 204))
      .mockResolvedValueOnce(response(undefined, 204))
      .mockResolvedValueOnce(response(undefined, 204));
    const recentCalls: Array<[RecentAuthPurpose, string]> = [];
    const api = createUsersApi(
      { request: request as ApiClient['request'] },
      createRecentActions(recentCalls),
    );
    const target = createTestUser('viewer');

    await api.createUser({
      currentPassword: 'admin-password',
      displayName: 'Observer',
      loginName: 'observer',
      password: 'viewer-password',
      role: 'viewer',
    });
    await api.updateUser(target, {
      currentPassword: 'admin-password',
      displayName: 'Night Observer',
      enabled: false,
      role: 'administrator',
    });
    await api.deleteUser(target, 'admin-password');
    await api.resetUserPassword(target, {
      currentPassword: 'admin-password',
      newPassword: 'replacement-password',
    });
    await api.revokeUserSessions(target, 'admin-password');

    expect(recentCalls).toEqual([
      ['user.create', 'admin-password'],
      ['user.update', 'admin-password'],
      ['user.delete', 'admin-password'],
      ['user.password_reset', 'admin-password'],
      ['user.sessions_revoke', 'admin-password'],
    ]);
    expect(request.mock.calls.map(([input]) => input)).toEqual([
      {
        body: {
          display_name: 'Observer',
          login_name: 'observer',
          password: 'viewer-password',
          role: 'viewer',
        },
        method: 'POST',
        path: '/users',
        protection,
      },
      {
        body: { display_name: 'Night Observer', enabled: false, role: 'administrator' },
        headers: { 'If-Match': '"1"' },
        method: 'PATCH',
        path: '/users/usr_viewer_01',
        protection,
      },
      {
        headers: { 'If-Match': '"1"' },
        method: 'DELETE',
        path: '/users/usr_viewer_01',
        protection,
      },
      {
        body: { new_password: 'replacement-password' },
        headers: { 'If-Match': '"1"' },
        method: 'POST',
        path: '/users/usr_viewer_01/password-reset',
        protection,
      },
      {
        method: 'DELETE',
        path: '/users/usr_viewer_01/sessions',
        protection,
      },
    ]);
  });

  it('rejects loose pages and classifies stale or last-admin conflicts', async () => {
    const client: ApiClient = {
      request: vi.fn().mockResolvedValue(
        response({
          items: [wireUser()],
          page: { extra: true, has_more: false, limit: 50 },
        }),
      ),
    };
    const api = createUsersApi(client, createRecentActions([]));

    await expect(api.listUsers()).rejects.toBeInstanceOf(UsersResponseError);
    expect(
      classifyUsersFailure(
        new ApiProblemError({
          code: 'conflict',
          request_id: requestId,
          status: 409,
          title: 'Request conflict',
          type: 'urn:stackplore:problem:conflict',
        }),
      ),
    ).toBe('conflict');
  });

  it('rejects unsafe user display text before it reaches the page', async () => {
    const unsafeUser = { ...wireUser(), display_name: 'Observer\nAdmin' };
    const client: ApiClient = {
      request: vi.fn().mockResolvedValue(
        response({
          items: [unsafeUser],
          page: { has_more: false, limit: 50 },
        }),
      ),
    };
    const api = createUsersApi(client, createRecentActions([]));

    await expect(api.listUsers()).rejects.toBeInstanceOf(UsersResponseError);
  });
});
