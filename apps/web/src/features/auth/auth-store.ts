import type {
  AuthApi,
  CurrentUser,
  LoginInput,
  RecentAuthPurpose,
  SessionContext,
  SessionProtection,
  SetupInput,
} from './auth-api';
import { isUnauthorizedFailure } from './auth-api';

export type AuthSnapshot =
  | { readonly status: 'loading' }
  | { readonly status: 'unavailable' }
  | {
      readonly bootstrapProofRequired: boolean;
      readonly status: 'setup_required';
    }
  | { readonly status: 'unauthenticated' }
  | {
      readonly status: 'guest_demo';
      readonly fixtureVersion: string;
      readonly scenarioStep: 0 | 1 | 2;
      readonly observedAt: string;
    }
  | {
      readonly absoluteExpiresAt: string;
      readonly idleExpiresAt: string;
      readonly status: 'authenticated';
      readonly user: CurrentUser;
    };

export interface AuthStoreInitialState {
  readonly csrfProof?: string;
  readonly snapshot: AuthSnapshot;
}

type AuthListener = () => void;
const maximumTimerDelay = 2_147_000_000;
const activityRefreshInterval = 60_000;
const activityRequestTimeout = 10_000;

export class AuthStateError extends Error {
  constructor() {
    super('The local session changed.');
    this.name = 'AuthStateError';
  }
}

export class AuthStore {
  readonly #api: AuthApi;
  #csrfProof: string | null;
  #expiryTimer: ReturnType<typeof globalThis.setTimeout> | null = null;
  #generation = 0;
  #inFlight: Promise<AuthSnapshot> | null = null;
  #activityRequest: Promise<void> | null = null;
  #lastActivityRefresh = Number.NEGATIVE_INFINITY;
  readonly #listeners = new Set<AuthListener>();
  #snapshot: AuthSnapshot;

  constructor(api: AuthApi, initial: AuthStoreInitialState = { snapshot: { status: 'loading' } }) {
    if (initial.snapshot.status === 'authenticated' && initial.csrfProof === undefined) {
      throw new Error('Authenticated store requires a CSRF proof.');
    }
    this.#api = api;
    this.#csrfProof = initial.csrfProof ?? null;
    this.#snapshot = initial.snapshot;
    if (this.#snapshot.status === 'authenticated') {
      const deadline = this.#sessionDeadline(this.#snapshot);
      if (deadline === null || deadline <= Date.now()) {
        this.#csrfProof = null;
        this.#snapshot = { status: 'unauthenticated' };
      } else {
        this.#scheduleExpiry(deadline);
      }
    }
  }

  getSnapshot = () => this.#snapshot;

  subscribe = (listener: AuthListener) => {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  };

  #publish(snapshot: AuthSnapshot) {
    this.#snapshot = snapshot;
    for (const listener of this.#listeners) {
      listener();
    }
  }

  #clearExpiryTimer() {
    if (this.#expiryTimer !== null) {
      globalThis.clearTimeout(this.#expiryTimer);
      this.#expiryTimer = null;
    }
  }

  #sessionDeadline(session: {
    readonly absoluteExpiresAt: string;
    readonly idleExpiresAt: string;
  }) {
    const absolute = Date.parse(session.absoluteExpiresAt);
    const idle = Date.parse(session.idleExpiresAt);
    if (!Number.isFinite(absolute) || !Number.isFinite(idle)) {
      return null;
    }
    return Math.min(absolute, idle);
  }

  #scheduleExpiry(deadline: number) {
    this.#clearExpiryTimer();
    const generation = this.#generation;
    const check = () => {
      this.#expiryTimer = null;
      if (generation !== this.#generation || this.#snapshot.status !== 'authenticated') {
        return;
      }
      const remaining = deadline - Date.now();
      if (remaining <= 0) {
        this.#generation += 1;
        this.#inFlight = null;
        this.#setUnauthenticated();
        return;
      }
      this.#expiryTimer = globalThis.setTimeout(check, Math.min(remaining, maximumTimerDelay));
    };
    this.#expiryTimer = globalThis.setTimeout(
      check,
      Math.min(Math.max(deadline - Date.now(), 0), maximumTimerDelay),
    );
  }

  #publishWithoutSession(snapshot: Exclude<AuthSnapshot, { readonly status: 'authenticated' }>) {
    this.#csrfProof = null;
    this.#clearExpiryTimer();
    this.#publish(snapshot);
  }

  #acceptSession(session: SessionContext) {
    const deadline = this.#sessionDeadline(session);
    if (deadline === null || deadline <= Date.now()) {
      this.#setUnauthenticated();
      return;
    }
    this.#csrfProof = session.csrfProof;
    this.#publish({
      absoluteExpiresAt: session.absoluteExpiresAt,
      idleExpiresAt: session.idleExpiresAt,
      status: 'authenticated',
      user: session.user,
    });
    this.#scheduleExpiry(deadline);
  }

  #setUnauthenticated() {
    this.#publishWithoutSession({ status: 'unauthenticated' });
  }

  async #load(generation: number) {
    try {
      const setupStatus = await this.#api.getSetupStatus();
      if (generation !== this.#generation) {
        return this.#snapshot;
      }
      if (setupStatus.state === 'guest_demo') {
        this.#publishWithoutSession({
          status: 'guest_demo',
          fixtureVersion: setupStatus.fixtureVersion,
          scenarioStep: setupStatus.scenarioStep,
          observedAt: setupStatus.observedAt,
        });
        return this.#snapshot;
      }
      if (setupStatus.state === 'required') {
        this.#publishWithoutSession({
          bootstrapProofRequired: setupStatus.bootstrapProofRequired,
          status: 'setup_required',
        });
        return this.#snapshot;
      }

      try {
        const session = await this.#api.getSession();
        if (generation === this.#generation) {
          this.#acceptSession(session);
        }
      } catch (error) {
        if (generation !== this.#generation) {
          return this.#snapshot;
        }
        if (isUnauthorizedFailure(error)) {
          this.#setUnauthenticated();
        } else {
          this.#publishWithoutSession({ status: 'unavailable' });
        }
      }
    } catch {
      if (generation === this.#generation) {
        this.#publishWithoutSession({ status: 'unavailable' });
      }
    }
    return this.#snapshot;
  }

  ensureLoaded = () => {
    if (this.#snapshot.status !== 'loading') {
      return Promise.resolve(this.#snapshot);
    }
    if (this.#inFlight !== null) {
      return this.#inFlight;
    }
    const generation = this.#generation;
    const request = this.#load(generation).finally(() => {
      if (this.#inFlight === request) {
        this.#inFlight = null;
      }
    });
    this.#inFlight = request;
    return request;
  };

  retry = () => {
    this.#generation += 1;
    this.#inFlight = null;
    this.#publishWithoutSession({ status: 'loading' });
    return this.ensureLoaded();
  };

  login = async (input: LoginInput) => {
    if (this.#snapshot.status === 'guest_demo') throw new AuthStateError();
    const session = await this.#api.login(input);
    this.#generation += 1;
    this.#acceptSession(session);
  };

  setup = async (input: SetupInput) => {
    if (this.#snapshot.status === 'guest_demo') throw new AuthStateError();
    const session = await this.#api.setup(input);
    this.#generation += 1;
    this.#acceptSession(session);
  };

  refreshSession = async () => {
    if (this.#snapshot.status !== 'authenticated') {
      throw new AuthStateError();
    }
    const generation = this.#generation;
    try {
      const session = await this.#api.getSession();
      if (generation !== this.#generation) {
        throw new AuthStateError();
      }
      this.#acceptSession(session);
    } catch (error) {
      if (generation === this.#generation && isUnauthorizedFailure(error)) {
        this.#generation += 1;
        this.#setUnauthenticated();
      }
      throw error;
    }
  };

  recordActivity = (): Promise<void> => {
    if (this.#snapshot.status !== 'authenticated') return Promise.resolve();
    if (this.#activityRequest !== null) return this.#activityRequest;
    const deadline = this.#sessionDeadline(this.#snapshot);
    if (deadline === null || deadline <= Date.now()) return Promise.resolve();
    const interval = Math.min(
      activityRefreshInterval,
      Math.max(1_000, (deadline - Date.now()) / 2),
    );
    if (Date.now() - this.#lastActivityRefresh < interval) return Promise.resolve();
    this.#lastActivityRefresh = Date.now();
    const generation = this.#generation;
    const controller = new AbortController();
    const timer = globalThis.setTimeout(() => {
      controller.abort();
    }, activityRequestTimeout);
    const request = this.#api
      .getSession(controller.signal)
      .then(
        (session) => {
          if (generation === this.#generation) this.#acceptSession(session);
        },
        (error: unknown) => {
          if (generation === this.#generation && isUnauthorizedFailure(error)) {
            this.#generation += 1;
            this.#setUnauthenticated();
          }
        },
      )
      .finally(() => {
        globalThis.clearTimeout(timer);
        if (this.#activityRequest === request) this.#activityRequest = null;
      });
    this.#activityRequest = request;
    return request;
  };

  runRecentAction = async <Result>(
    purpose: RecentAuthPurpose,
    currentPassword: string,
    action: (protection: SessionProtection) => Promise<Result>,
  ): Promise<Result> => {
    if (this.#snapshot.status !== 'authenticated' || this.#csrfProof === null) {
      throw new AuthStateError();
    }
    const csrfProof = this.#csrfProof;
    const generation = this.#generation;
    const recentAuthToken = await this.#api.createRecentAuthGrant(
      currentPassword,
      purpose,
      csrfProof,
    );
    if (generation !== this.#generation || this.#csrfProof !== csrfProof) {
      throw new AuthStateError();
    }
    const result = await action({ csrfToken: csrfProof, kind: 'session', recentAuthToken });
    if (generation !== this.#generation || this.#csrfProof !== csrfProof) {
      throw new AuthStateError();
    }
    return result;
  };

  runSessionAction = async <Result>(
    action: (protection: SessionProtection) => Promise<Result>,
  ): Promise<Result> => {
    if (this.#snapshot.status !== 'authenticated' || this.#csrfProof === null) {
      throw new AuthStateError();
    }
    const csrfProof = this.#csrfProof;
    const generation = this.#generation;
    const result = await action({ csrfToken: csrfProof, kind: 'session' });
    if (generation !== this.#generation || this.#csrfProof !== csrfProof) {
      throw new AuthStateError();
    }
    return result;
  };

  changePassword = async (currentPassword: string, newPassword: string) => {
    const session = await this.runRecentAction(
      'account.password_change',
      currentPassword,
      (protection) => this.#api.changeOwnPassword(newPassword, protection),
    );
    this.#generation += 1;
    this.#acceptSession(session);
  };

  logout = async () => {
    if (this.#snapshot.status === 'guest_demo') throw new AuthStateError();
    if (this.#snapshot.status !== 'authenticated' || this.#csrfProof === null) {
      this.#setUnauthenticated();
      return;
    }
    try {
      await this.#api.logout(this.#csrfProof);
    } catch (error) {
      if (!isUnauthorizedFailure(error)) {
        throw error;
      }
    }
    this.#generation += 1;
    this.#setUnauthenticated();
  };
}
