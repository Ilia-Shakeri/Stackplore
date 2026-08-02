import type {
  AuthApi,
  CurrentUser,
  SessionContext,
  SetupStatus,
  UserRole,
} from '../features/auth/auth-api';
import { AuthStore, type AuthSnapshot } from '../features/auth/auth-store';

export const testProof = 'A'.repeat(43);

export function createTestUser(role: UserRole = 'administrator'): CurrentUser {
  return {
    createdAt: '2026-01-02T03:04:05Z',
    displayName: role === 'administrator' ? 'Operations' : 'Observer',
    enabled: true,
    id: role === 'administrator' ? 'usr_admin_01' : 'usr_viewer_01',
    loginName: role === 'administrator' ? 'operator' : 'observer',
    role,
    updatedAt: '2026-01-02T03:04:05Z',
    version: 1,
  };
}

export function createTestSession(role: UserRole = 'administrator'): SessionContext {
  return {
    absoluteExpiresAt: '2099-01-02T15:04:05Z',
    csrfProof: testProof,
    idleExpiresAt: '2099-01-02T03:34:05Z',
    user: createTestUser(role),
  };
}

const unexpected = (): Promise<never> => Promise.reject(new Error('Unexpected auth test call.'));

export function createTestAuthApi(overrides: Partial<AuthApi> = {}): AuthApi {
  return {
    changeOwnPassword: unexpected,
    createRecentAuthGrant: unexpected,
    getSession: unexpected,
    getSetupStatus: unexpected,
    login: unexpected,
    logout: unexpected,
    setup: unexpected,
    ...overrides,
  };
}

export function createAuthenticatedAuthStore(
  role: UserRole = 'administrator',
  overrides: Partial<AuthApi> = {},
) {
  const session = createTestSession(role);
  return new AuthStore(
    createTestAuthApi({ getSession: () => Promise.resolve(session), ...overrides }),
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
}

export function createResolvedAuthStore(
  snapshot: Exclude<AuthSnapshot, { readonly status: 'authenticated' }>,
  overrides: Partial<AuthApi> = {},
) {
  return new AuthStore(createTestAuthApi(overrides), { snapshot });
}

export const closedSetupStatus: SetupStatus = {
  bootstrapProofRequired: false,
  state: 'closed',
};
