import {
  ApiProblemError,
  ApiTransportError,
  type ApiClient,
  type ApiRequestProtection,
} from '@stackplore/api-client';

const loginNamePattern = /^[A-Za-z0-9](?:[A-Za-z0-9._-]{1,62}[A-Za-z0-9])$/u;
const proofPattern = /^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/u;
const timestampPattern =
  /^(?!0000)[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,9})?Z$/u;
const outerWhitespacePattern = /^\p{White_Space}|\p{White_Space}$/u;
const unsafeDisplayPattern = /[\p{Cc}\p{Zl}\p{Zp}]/u;
const formatCharacterPattern = /\p{Cf}/u;
const visibleBasePattern = /[\p{L}\p{N}\p{P}\p{S}]/u;

export type UserRole = 'administrator' | 'viewer';
export type RecentAuthPurpose =
  | 'account.password_change'
  | 'user.create'
  | 'user.delete'
  | 'user.password_reset'
  | 'user.sessions_revoke'
  | 'user.update';
export type SessionProtection = Extract<ApiRequestProtection, { readonly kind: 'session' }>;

export interface CurrentUser {
  readonly createdAt: string;
  readonly displayName: string;
  readonly enabled: boolean;
  readonly id: string;
  readonly loginName: string;
  readonly role: UserRole;
  readonly updatedAt: string;
  readonly version: number;
}

export interface SessionContext {
  readonly absoluteExpiresAt: string;
  readonly csrfProof: string;
  readonly idleExpiresAt: string;
  readonly user: CurrentUser;
}

export interface SetupStatus {
  readonly bootstrapProofRequired: boolean;
  readonly state: 'closed' | 'required';
}

export interface LoginInput {
  readonly loginName: string;
  readonly password: string;
}

export interface SetupInput extends LoginInput {
  readonly bootstrapProof?: string;
  readonly displayName: string;
}

export interface AuthApi {
  changeOwnPassword(newPassword: string, protection: SessionProtection): Promise<SessionContext>;
  createRecentAuthGrant(
    currentPassword: string,
    purpose: RecentAuthPurpose,
    csrfProof: string,
  ): Promise<string>;
  getSession(): Promise<SessionContext>;
  getSetupStatus(): Promise<SetupStatus>;
  login(input: LoginInput): Promise<SessionContext>;
  logout(csrfProof: string): Promise<void>;
  setup(input: SetupInput): Promise<SessionContext>;
}

export type AuthAction = 'login' | 'logout' | 'session' | 'setup';
export type AuthFailureKind =
  | 'credentials'
  | 'forbidden'
  | 'rate_limited'
  | 'setup_closed'
  | 'unavailable'
  | 'unexpected'
  | 'validation';

export interface AuthFailure {
  readonly fields: readonly string[];
  readonly kind: AuthFailureKind;
}

export class AuthResponseError extends Error {
  constructor() {
    super('Authentication response was invalid.');
    this.name = 'AuthResponseError';
  }
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

const hasExactKeys = (
  value: Record<string, unknown>,
  required: readonly string[],
  optional: readonly string[] = [],
) => {
  const keys = Object.keys(value);
  const allowed = new Set([...required, ...optional]);
  return (
    required.every((key) => Object.hasOwn(value, key)) && keys.every((key) => allowed.has(key))
  );
};

const isTimestamp = (value: unknown): value is string =>
  typeof value === 'string' &&
  value.length >= 20 &&
  value.length <= 30 &&
  timestampPattern.test(value) &&
  Number.isFinite(Date.parse(value));

const isDisplayText = (value: unknown): value is string =>
  typeof value === 'string' &&
  Array.from(value).length >= 1 &&
  Array.from(value).length <= 256 &&
  !outerWhitespacePattern.test(value) &&
  visibleBasePattern.test(value) &&
  Array.from(value).every(
    (character) =>
      !unsafeDisplayPattern.test(character) &&
      (!formatCharacterPattern.test(character) || character === '\u200c' || character === '\u200d'),
  );

export const parseUser = (value: unknown): CurrentUser => {
  const required = [
    'id',
    'login_name',
    'display_name',
    'role',
    'enabled',
    'created_at',
    'updated_at',
    'version',
  ];
  if (
    !isRecord(value) ||
    !hasExactKeys(value, required) ||
    typeof value.id !== 'string' ||
    value.id.length < 1 ||
    value.id.length > 128 ||
    !/^[A-Za-z][A-Za-z0-9_-]{0,127}$/u.test(value.id) ||
    typeof value.login_name !== 'string' ||
    !loginNamePattern.test(value.login_name) ||
    !isDisplayText(value.display_name) ||
    (value.role !== 'administrator' && value.role !== 'viewer') ||
    typeof value.enabled !== 'boolean' ||
    !isTimestamp(value.created_at) ||
    !isTimestamp(value.updated_at) ||
    typeof value.version !== 'number' ||
    !Number.isSafeInteger(value.version) ||
    value.version < 1
  ) {
    throw new AuthResponseError();
  }
  return {
    createdAt: value.created_at,
    displayName: value.display_name,
    enabled: value.enabled,
    id: value.id,
    loginName: value.login_name,
    role: value.role,
    updatedAt: value.updated_at,
    version: value.version,
  };
};

const parseSession = (value: unknown): SessionContext => {
  const required = ['user', 'csrf_proof', 'idle_expires_at', 'absolute_expires_at'];
  if (
    !isRecord(value) ||
    !hasExactKeys(value, required) ||
    typeof value.csrf_proof !== 'string' ||
    !proofPattern.test(value.csrf_proof) ||
    !isTimestamp(value.idle_expires_at) ||
    !isTimestamp(value.absolute_expires_at)
  ) {
    throw new AuthResponseError();
  }
  const user = parseUser(value.user);
  if (!user.enabled) {
    throw new AuthResponseError();
  }
  return {
    absoluteExpiresAt: value.absolute_expires_at,
    csrfProof: value.csrf_proof,
    idleExpiresAt: value.idle_expires_at,
    user,
  };
};

const parseSetupStatus = (value: unknown): SetupStatus => {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['state', 'bootstrap_proof_required']) ||
    (value.state !== 'closed' && value.state !== 'required') ||
    typeof value.bootstrap_proof_required !== 'boolean'
  ) {
    throw new AuthResponseError();
  }
  return {
    bootstrapProofRequired: value.bootstrap_proof_required,
    state: value.state,
  };
};

const parsePreauthProof = (value: unknown) => {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['preauth_proof', 'expires_at']) ||
    typeof value.preauth_proof !== 'string' ||
    !proofPattern.test(value.preauth_proof) ||
    !isTimestamp(value.expires_at)
  ) {
    throw new AuthResponseError();
  }
  return value.preauth_proof;
};

const parseRecentAuthProof = (value: unknown, expectedPurpose: RecentAuthPurpose) => {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['recent_auth_proof', 'purpose', 'expires_at']) ||
    typeof value.recent_auth_proof !== 'string' ||
    !proofPattern.test(value.recent_auth_proof) ||
    value.purpose !== expectedPurpose ||
    !isTimestamp(value.expires_at)
  ) {
    throw new AuthResponseError();
  }
  return value.recent_auth_proof;
};

export function isUnauthorizedFailure(error: unknown) {
  return error instanceof ApiProblemError && error.problem.status === 401;
}

export function classifyAuthFailure(error: unknown, action: AuthAction): AuthFailure {
  if (error instanceof ApiProblemError) {
    const fields =
      error.problem.invalid_parameters
        ?.filter((item) => item.location === 'body')
        .map((item) => item.name) ?? [];
    if (error.problem.status === 429) {
      return { fields, kind: 'rate_limited' };
    }
    if (
      error.problem.status === 400 ||
      error.problem.status === 413 ||
      error.problem.status === 415
    ) {
      return { fields, kind: 'validation' };
    }
    if (error.problem.status === 401) {
      return { fields, kind: action === 'login' ? 'credentials' : 'unexpected' };
    }
    if (error.problem.status === 403) {
      return { fields, kind: 'forbidden' };
    }
    if (error.problem.status === 409 && action === 'setup') {
      return { fields, kind: 'setup_closed' };
    }
    if (error.problem.status === 500 || error.problem.status === 503) {
      return { fields, kind: 'unavailable' };
    }
    return { fields, kind: 'unexpected' };
  }
  if (error instanceof ApiTransportError) {
    return {
      fields: [],
      kind: error.code === 'network_error' ? 'unavailable' : 'unexpected',
    };
  }
  return { fields: [], kind: 'unexpected' };
}

export function createAuthApi(client: ApiClient): AuthApi {
  const createPreauthProof = async (purpose: 'login' | 'setup') => {
    const response = await client.request<unknown, { readonly purpose: 'login' | 'setup' }>({
      body: { purpose },
      method: 'POST',
      path: '/auth/preflight',
    });
    return parsePreauthProof(response.data);
  };

  return {
    async changeOwnPassword(newPassword, protection) {
      const response = await client.request<unknown, { readonly new_password: string }>({
        body: { new_password: newPassword },
        method: 'PATCH',
        path: '/auth/password',
        protection,
      });
      return parseSession(response.data);
    },
    async createRecentAuthGrant(currentPassword, purpose, csrfProof) {
      const response = await client.request<
        unknown,
        { readonly current_password: string; readonly purpose: RecentAuthPurpose }
      >({
        body: { current_password: currentPassword, purpose },
        method: 'POST',
        path: '/auth/recent-grants',
        protection: { csrfToken: csrfProof, kind: 'session' },
      });
      return parseRecentAuthProof(response.data, purpose);
    },
    async getSession() {
      const response = await client.request<unknown>({ path: '/auth/session' });
      return parseSession(response.data);
    },
    async getSetupStatus() {
      const response = await client.request<unknown>({ path: '/setup/status' });
      return parseSetupStatus(response.data);
    },
    async login(input) {
      const preauthProof = await createPreauthProof('login');
      const response = await client.request<
        unknown,
        { readonly login_name: string; readonly password: string }
      >({
        body: { login_name: input.loginName, password: input.password },
        method: 'POST',
        path: '/auth/login',
        protection: { kind: 'preauth', token: preauthProof },
      });
      return parseSession(response.data);
    },
    async logout(csrfProof) {
      await client.request<undefined>({
        method: 'POST',
        path: '/auth/logout',
        protection: { csrfToken: csrfProof, kind: 'session' },
      });
    },
    async setup(input) {
      const preauthProof = await createPreauthProof('setup');
      const response = await client.request<
        unknown,
        {
          readonly bootstrap_proof?: string;
          readonly display_name: string;
          readonly login_name: string;
          readonly password: string;
        }
      >({
        body: {
          display_name: input.displayName,
          login_name: input.loginName,
          password: input.password,
          ...(input.bootstrapProof === undefined ? {} : { bootstrap_proof: input.bootstrapProof }),
        },
        method: 'POST',
        path: '/setup',
        protection: { kind: 'preauth', token: preauthProof },
      });
      return parseSession(response.data);
    },
  };
}
