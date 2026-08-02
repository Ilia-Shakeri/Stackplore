import { ApiProblemError, ApiTransportError, type ApiClient } from '@stackplore/api-client';

import {
  AuthResponseError,
  parseUser,
  type CurrentUser,
  type RecentAuthPurpose,
  type SessionProtection,
  type UserRole,
} from '../auth/auth-api';

const cursorPattern = /^[A-Za-z0-9_-]{1,1024}$/u;
const userIdPattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;
const pageLimit = 50;

export interface UserPage {
  readonly items: readonly CurrentUser[];
  readonly nextCursor?: string;
}

export interface CreateUserInput {
  readonly currentPassword: string;
  readonly displayName: string;
  readonly loginName: string;
  readonly password: string;
  readonly role: UserRole;
}

export interface UpdateUserInput {
  readonly currentPassword: string;
  readonly displayName?: string;
  readonly enabled?: boolean;
  readonly loginName?: string;
  readonly role?: UserRole;
}

export interface ResetUserPasswordInput {
  readonly currentPassword: string;
  readonly newPassword: string;
}

export interface RecentActionRunner {
  runRecentAction<Result>(
    purpose: RecentAuthPurpose,
    currentPassword: string,
    action: (protection: SessionProtection) => Promise<Result>,
  ): Promise<Result>;
}

export interface UsersApi {
  createUser(input: CreateUserInput): Promise<CurrentUser>;
  deleteUser(user: CurrentUser, currentPassword: string): Promise<void>;
  listUsers(options?: {
    readonly cursor?: string;
    readonly signal?: AbortSignal;
  }): Promise<UserPage>;
  resetUserPassword(user: CurrentUser, input: ResetUserPasswordInput): Promise<void>;
  revokeUserSessions(user: CurrentUser, currentPassword: string): Promise<void>;
  updateUser(user: CurrentUser, input: UpdateUserInput): Promise<CurrentUser>;
}

export type UsersFailureKind =
  | 'conflict'
  | 'forbidden'
  | 'not_found'
  | 'rate_limited'
  | 'session'
  | 'unavailable'
  | 'unexpected'
  | 'validation';

export class UsersResponseError extends Error {
  constructor() {
    super('User response was invalid.');
    this.name = 'UsersResponseError';
  }
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

const hasExactKeys = (value: Record<string, unknown>, required: readonly string[]) => {
  const keys = Object.keys(value);
  return keys.length === required.length && required.every((key) => Object.hasOwn(value, key));
};

const parseUserPage = (value: unknown): UserPage => {
  if (!isRecord(value) || !hasExactKeys(value, ['items', 'page']) || !Array.isArray(value.items)) {
    throw new UsersResponseError();
  }
  const page = value.page;
  if (
    value.items.length > 200 ||
    !isRecord(page) ||
    typeof page.limit !== 'number' ||
    !Number.isInteger(page.limit) ||
    page.limit < 1 ||
    page.limit > 200 ||
    value.items.length > page.limit ||
    typeof page.has_more !== 'boolean'
  ) {
    throw new UsersResponseError();
  }

  let nextCursor: string | undefined;
  if (page.has_more) {
    if (
      !hasExactKeys(page, ['limit', 'has_more', 'next_cursor']) ||
      typeof page.next_cursor !== 'string' ||
      !cursorPattern.test(page.next_cursor)
    ) {
      throw new UsersResponseError();
    }
    nextCursor = page.next_cursor;
  } else if (!hasExactKeys(page, ['limit', 'has_more'])) {
    throw new UsersResponseError();
  }

  try {
    return {
      items: value.items.map((item) => parseUser(item)),
      ...(nextCursor === undefined ? {} : { nextCursor }),
    };
  } catch (error) {
    if (error instanceof AuthResponseError) {
      throw new UsersResponseError();
    }
    throw error;
  }
};

const userPath = (id: string) => {
  if (!userIdPattern.test(id)) {
    throw new UsersResponseError();
  }
  return `/users/${id}` as const;
};

const ifMatch = (version: number) => {
  if (!Number.isSafeInteger(version) || version < 1) {
    throw new UsersResponseError();
  }
  return `"${String(version)}"`;
};

export function classifyUsersFailure(error: unknown): UsersFailureKind {
  if (error instanceof ApiProblemError) {
    switch (error.problem.status) {
      case 400:
      case 413:
      case 415:
        return 'validation';
      case 401:
        return 'session';
      case 403:
        return 'forbidden';
      case 404:
        return 'not_found';
      case 409:
        return 'conflict';
      case 429:
        return 'rate_limited';
      case 500:
      case 503:
        return 'unavailable';
      default:
        return 'unexpected';
    }
  }
  if (error instanceof ApiTransportError && error.code === 'network_error') {
    return 'unavailable';
  }
  return 'unexpected';
}

export function createUsersApi(client: ApiClient, recentActions: RecentActionRunner): UsersApi {
  const runMutation = <Result>(
    purpose: RecentAuthPurpose,
    currentPassword: string,
    action: (protection: SessionProtection) => Promise<Result>,
  ) => recentActions.runRecentAction(purpose, currentPassword, action);

  return {
    async createUser(input) {
      return runMutation('user.create', input.currentPassword, async (protection) => {
        const response = await client.request<
          unknown,
          {
            readonly display_name: string;
            readonly login_name: string;
            readonly password: string;
            readonly role: UserRole;
          }
        >({
          body: {
            display_name: input.displayName,
            login_name: input.loginName,
            password: input.password,
            role: input.role,
          },
          method: 'POST',
          path: '/users',
          protection,
        });
        return parseUser(response.data);
      });
    },
    async deleteUser(user, currentPassword) {
      await runMutation('user.delete', currentPassword, async (protection) => {
        await client.request<undefined>({
          headers: { 'If-Match': ifMatch(user.version) },
          method: 'DELETE',
          path: userPath(user.id),
          protection,
        });
      });
    },
    async listUsers(options = {}) {
      const response = await client.request<unknown>({
        path: '/users',
        query: { cursor: options.cursor, limit: pageLimit },
        ...(options.signal === undefined ? {} : { signal: options.signal }),
      });
      return parseUserPage(response.data);
    },
    async resetUserPassword(user, input) {
      await runMutation('user.password_reset', input.currentPassword, async (protection) => {
        await client.request<undefined, { readonly new_password: string }>({
          body: { new_password: input.newPassword },
          headers: { 'If-Match': ifMatch(user.version) },
          method: 'POST',
          path: `${userPath(user.id)}/password-reset`,
          protection,
        });
      });
    },
    async revokeUserSessions(user, currentPassword) {
      await runMutation('user.sessions_revoke', currentPassword, async (protection) => {
        await client.request<undefined>({
          method: 'DELETE',
          path: `${userPath(user.id)}/sessions`,
          protection,
        });
      });
    },
    async updateUser(user, input) {
      return runMutation('user.update', input.currentPassword, async (protection) => {
        const body = {
          ...(input.displayName === undefined ? {} : { display_name: input.displayName }),
          ...(input.enabled === undefined ? {} : { enabled: input.enabled }),
          ...(input.loginName === undefined ? {} : { login_name: input.loginName }),
          ...(input.role === undefined ? {} : { role: input.role }),
        };
        if (Object.keys(body).length === 0) {
          throw new UsersResponseError();
        }
        const response = await client.request<unknown, typeof body>({
          body,
          headers: { 'If-Match': ifMatch(user.version) },
          method: 'PATCH',
          path: userPath(user.id),
          protection,
        });
        return parseUser(response.data);
      });
    },
  };
}
