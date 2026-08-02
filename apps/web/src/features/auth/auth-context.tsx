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

export interface AuthProviderProps {
  readonly children: ReactNode;
  readonly store: AuthStore;
}

export function AuthProvider({ children, store }: AuthProviderProps) {
  useEffect(() => {
    void store.ensureLoaded();
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
