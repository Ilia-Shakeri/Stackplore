import { createContext, useContext, type ReactNode } from 'react';

import type { UsersApi } from './users-api';

const UsersContext = createContext<UsersApi | null>(null);

export interface UsersProviderProps {
  readonly api: UsersApi;
  readonly children: ReactNode;
}

export function UsersProvider({ api, children }: UsersProviderProps) {
  return <UsersContext value={api}>{children}</UsersContext>;
}

export function useUsersApi() {
  const api = useContext(UsersContext);
  if (api === null) {
    throw new Error('useUsersApi must be used within UsersProvider.');
  }
  return api;
}
