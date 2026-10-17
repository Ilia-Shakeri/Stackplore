import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useSyncExternalStore,
  type ReactNode,
} from 'react';

import type { AuthStore } from './auth-store';

const AuthContext = createContext<AuthStore | null>(null);

const subscribeAbsent = () => () => {};
const snapshotAbsent = () => null;

export function useOptionalAuthSnapshot() {
  const store = useContext(AuthContext);
  return useSyncExternalStore(
    store?.subscribe ?? subscribeAbsent,
    store?.getSnapshot ?? snapshotAbsent,
    store?.getSnapshot ?? snapshotAbsent,
  );
}

export interface AuthProviderProps {
  readonly children: ReactNode;
  readonly store: AuthStore;
}

export function AuthProvider({ children, store }: AuthProviderProps) {
  useEffect(() => {
    void store.ensureLoaded();
    const activity = () => {
      void store.recordActivity();
    };
    window.addEventListener('pointerdown', activity, { passive: true });
    window.addEventListener('keydown', activity);
    window.addEventListener('wheel', activity, { passive: true });
    window.addEventListener('touchmove', activity, { passive: true });
    return () => {
      window.removeEventListener('pointerdown', activity);
      window.removeEventListener('keydown', activity);
      window.removeEventListener('wheel', activity);
      window.removeEventListener('touchmove', activity);
    };
  }, [store]);

  return <AuthContext value={store}>{children}</AuthContext>;
}

function useAuthStore() {
  const store = useContext(AuthContext);
  if (store === null) {
    throw new Error('useAuth must be used within AuthProvider.');
  }
  return store;
}

export function useAuth() {
  const store = useAuthStore();
  const snapshot = useSyncExternalStore(store.subscribe, store.getSnapshot, store.getSnapshot);
  return useMemo(
    () => ({
      changePassword: store.changePassword,
      login: store.login,
      logout: store.logout,
      refreshSession: store.refreshSession,
      retry: store.retry,
      runRecentAction: store.runRecentAction,
      setup: store.setup,
      snapshot,
    }),
    [snapshot, store],
  );
}
