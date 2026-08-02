import type { UsersApi } from '../features/users/users-api';

const unexpected = (): Promise<never> => Promise.reject(new Error('Unexpected user test call.'));

export function createTestUsersApi(overrides: Partial<UsersApi> = {}): UsersApi {
  return {
    createUser: unexpected,
    deleteUser: unexpected,
    listUsers: unexpected,
    resetUserPassword: unexpected,
    revokeUserSessions: unexpected,
    updateUser: unexpected,
    ...overrides,
  };
}
