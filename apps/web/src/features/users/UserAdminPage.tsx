import {
  Badge,
  Button,
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFrame,
  TableHead,
  TableHeader,
  TableRow,
  TechnicalText,
  useToast,
} from '@stackplore/ui';
import { useRouter } from '@tanstack/react-router';
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';

import { UserActionDialog, type UserAction } from './UserActionDialog';
import { classifyUsersFailure } from './users-api';
import { useUsersApi } from './users-context';
import type { CurrentUser } from '../auth/auth-api';
import { useAuth } from '../auth/auth-context';
import { useLocale, type TextMessageKey } from '../../i18n';

type UsersPageState =
  | { readonly status: 'error' }
  | { readonly status: 'loading' }
  | {
      readonly items: readonly CurrentUser[];
      readonly loadingMore: boolean;
      readonly moreError: boolean;
      readonly nextCursor?: string;
      readonly refreshing: boolean;
      readonly status: 'ready';
    };

export interface UserAdminPageProps {
  readonly toolbar?: ReactNode;
}

const successMessage = (action: UserAction): TextMessageKey => {
  switch (action.kind) {
    case 'create':
      return 'users.create.success';
    case 'edit':
      return 'users.edit.success';
    case 'delete':
      return 'users.delete.success';
    case 'reset_password':
      return 'users.reset.success';
    case 'revoke_sessions':
      return 'users.revoke.success';
  }
};

const actionKey = (action: UserAction) =>
  action.kind === 'create'
    ? action.kind
    : `${action.kind}:${action.user.id}:${String(action.user.version)}`;

const isAbort = (error: unknown) =>
  (error instanceof DOMException || error instanceof Error) && error.name === 'AbortError';

export function UserAdminPage({ toolbar }: UserAdminPageProps) {
  const api = useUsersApi();
  const { refreshSession, retry, snapshot } = useAuth();
  const { formatDate, t } = useLocale();
  const { notify } = useToast();
  const router = useRouter();
  const loadGenerationRef = useRef(0);
  const loadMoreLockRef = useRef(false);
  const [action, setAction] = useState<UserAction | null>(null);
  const [state, setState] = useState<UsersPageState>({ status: 'loading' });
  const administrator =
    snapshot.status === 'authenticated' && snapshot.user.role === 'administrator';

  const refreshLostSession = useCallback(
    async (error: unknown) => {
      const kind = classifyUsersFailure(error);
      if (kind !== 'session' && kind !== 'forbidden') {
        return false;
      }
      setAction(null);
      await retry();
      await router.invalidate();
      return true;
    },
    [retry, router],
  );

  const loadUsers = useCallback(
    async (signal?: AbortSignal) => {
      const generation = loadGenerationRef.current + 1;
      loadGenerationRef.current = generation;
      setState((current) =>
        current.status === 'ready'
          ? { ...current, moreError: false, refreshing: true }
          : { status: 'loading' },
      );
      try {
        const page = await api.listUsers(signal === undefined ? {} : { signal });
        await refreshSession();
        if (signal?.aborted || generation !== loadGenerationRef.current) {
          return;
        }
        setState({
          items: page.items,
          loadingMore: false,
          moreError: false,
          ...(page.nextCursor === undefined ? {} : { nextCursor: page.nextCursor }),
          refreshing: false,
          status: 'ready',
        });
      } catch (error) {
        if (
          !isAbort(error) &&
          signal?.aborted !== true &&
          generation === loadGenerationRef.current
        ) {
          if (await refreshLostSession(error)) {
            return;
          }
          setState({ status: 'error' });
        }
      }
    },
    [api, refreshLostSession, refreshSession],
  );

  useEffect(() => {
    if (!administrator) {
      return;
    }
    const controller = new AbortController();
    void loadUsers(controller.signal);
    return () => {
      controller.abort();
    };
  }, [administrator, loadUsers]);

  if (!administrator) {
    return null;
  }

  const loadMore = async () => {
    if (
      state.status !== 'ready' ||
      state.nextCursor === undefined ||
      loadMoreLockRef.current ||
      state.loadingMore ||
      state.refreshing
    ) {
      return;
    }
    loadMoreLockRef.current = true;
    const generation = loadGenerationRef.current;
    const cursor = state.nextCursor;
    setState({ ...state, loadingMore: true, moreError: false });
    try {
      const page = await api.listUsers({ cursor });
      await refreshSession();
      if (generation !== loadGenerationRef.current) {
        return;
      }
      setState((current) => {
        if (current.status !== 'ready') {
          return current;
        }
        const users = new Map(current.items.map((user) => [user.id, user]));
        for (const user of page.items) {
          users.set(user.id, user);
        }
        return {
          items: [...users.values()],
          loadingMore: false,
          moreError: false,
          ...(page.nextCursor === undefined ? {} : { nextCursor: page.nextCursor }),
          refreshing: false,
          status: 'ready',
        };
      });
    } catch (error) {
      if (await refreshLostSession(error)) {
        return;
      }
      setState((current) =>
        current.status === 'ready' ? { ...current, loadingMore: false, moreError: true } : current,
      );
    } finally {
      loadMoreLockRef.current = false;
    }
  };

  const handleSessionLost = async () => {
    setAction(null);
    await retry();
    await router.invalidate();
  };

  const handleSuccess = async (completedAction: UserAction) => {
    setAction(null);
    notify({ title: t(successMessage(completedAction)), tone: 'success' });
    if (completedAction.kind !== 'create' && completedAction.user.id === snapshot.user.id) {
      await retry();
      await router.invalidate();
      return;
    }
    await loadUsers();
  };

  const items = state.status === 'ready' ? state.items : [];
  const finalPage = state.status === 'ready' && state.nextCursor === undefined;
  const enabledAdministratorCount = items.filter(
    (user) => user.enabled && user.role === 'administrator',
  ).length;
  const isLastEnabledAdministrator = (user: CurrentUser) =>
    finalPage && user.enabled && user.role === 'administrator' && enabledAdministratorCount === 1;

  return (
    <div className="route-page users-page min-w-0">
      <header className="route-page__header users-page__header">
        <div>
          <p className="route-page__eyebrow">{t('users.page.eyebrow')}</p>
          <h1>{t('route.users.title')}</h1>
          <p>{t('route.users.description')}</p>
        </div>
        <div className="users-page__actions">
          <Button
            loading={state.status === 'ready' && state.refreshing}
            variant="outline"
            onClick={() => void loadUsers()}
          >
            {t('users.page.refresh')}
          </Button>
          <Button
            onClick={() => {
              setAction({ kind: 'create' });
            }}
          >
            {t('users.page.add')}
          </Button>
        </div>
      </header>

      {toolbar}

      {state.status === 'loading' ? (
        <section className="users-state" aria-live="polite">
          <h2>{t('users.loading.title')}</h2>
          <p>{t('users.loading.body')}</p>
        </section>
      ) : null}

      {state.status === 'error' ? (
        <section className="users-state users-state--error" role="alert">
          <h2>{t('users.loadError.title')}</h2>
          <p>{t('users.loadError.body')}</p>
          <Button onClick={() => void loadUsers()}>{t('users.loadError.retry')}</Button>
        </section>
      ) : null}

      {state.status === 'ready' && state.items.length === 0 ? (
        <section className="users-state">
          <h2>{t('users.empty.title')}</h2>
          <p>{t('users.empty.body')}</p>
        </section>
      ) : null}

      {state.status === 'ready' && state.items.length > 0 ? (
        <section className="users-list" aria-busy={state.refreshing || undefined}>
          {state.refreshing ? (
            <p className="users-list__refreshing">{t('users.refreshing')}</p>
          ) : null}
          <TableFrame aria-label={t('users.table.label')} className="users-table-frame">
            <Table className="users-table">
              <TableCaption className="ui-sr-only">{t('users.table.caption')}</TableCaption>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('users.table.user')}</TableHead>
                  <TableHead>{t('users.table.role')}</TableHead>
                  <TableHead>{t('users.table.status')}</TableHead>
                  <TableHead>{t('users.table.updated')}</TableHead>
                  <TableHead>{t('users.table.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {state.items.map((user) => {
                  const lastAdministrator = isLastEnabledAdministrator(user);
                  return (
                    <TableRow key={user.id}>
                      <TableCell data-label={t('users.table.user')}>
                        <span className="users-table__identity">
                          <strong>{user.displayName}</strong>
                          <TechnicalText>{user.loginName}</TechnicalText>
                          {user.id === snapshot.user.id ? (
                            <Badge tone="info">{t('users.current')}</Badge>
                          ) : null}
                        </span>
                      </TableCell>
                      <TableCell data-label={t('users.table.role')}>
                        <Badge tone={user.role === 'administrator' ? 'info' : 'neutral'}>
                          {t(
                            user.role === 'administrator'
                              ? 'auth.role.administrator'
                              : 'auth.role.viewer',
                          )}
                        </Badge>
                      </TableCell>
                      <TableCell data-label={t('users.table.status')}>
                        <Badge tone={user.enabled ? 'success' : 'warning'}>
                          {t(user.enabled ? 'users.status.enabled' : 'users.status.disabled')}
                        </Badge>
                        {lastAdministrator ? (
                          <span className="users-table__safety">
                            {t('users.lastAdministrator.badge')}
                          </span>
                        ) : null}
                      </TableCell>
                      <TableCell data-label={t('users.table.updated')}>
                        <time dateTime={user.updatedAt}>
                          {formatDate(Date.parse(user.updatedAt), {
                            dateStyle: 'medium',
                            timeStyle: 'short',
                          })}
                        </time>
                      </TableCell>
                      <TableCell data-label={t('users.table.actions')}>
                        <div className="users-table__actions">
                          <Button
                            aria-label={t('users.edit.openLabel', { name: user.displayName })}
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setAction({ kind: 'edit', user });
                            }}
                          >
                            {t('users.action.edit')}
                          </Button>
                          <Button
                            aria-label={t('users.reset.openLabel', { name: user.displayName })}
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setAction({ kind: 'reset_password', user });
                            }}
                          >
                            {t('users.action.reset')}
                          </Button>
                          <Button
                            aria-label={t('users.revoke.openLabel', { name: user.displayName })}
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setAction({ kind: 'revoke_sessions', user });
                            }}
                          >
                            {t('users.action.revoke')}
                          </Button>
                          <Button
                            aria-label={
                              lastAdministrator
                                ? t('users.delete.blockedLabel', { name: user.displayName })
                                : t('users.delete.openLabel', { name: user.displayName })
                            }
                            disabled={lastAdministrator}
                            size="sm"
                            variant="danger"
                            onClick={() => {
                              setAction({ kind: 'delete', user });
                            }}
                          >
                            {t('users.action.delete')}
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TableFrame>

          {state.moreError ? (
            <p className="users-list__more-error" role="alert">
              {t('users.moreError')}
            </p>
          ) : null}
          {state.nextCursor === undefined ? null : (
            <Button
              className="users-list__more"
              loading={state.loadingMore}
              variant="outline"
              onClick={() => void loadMore()}
            >
              {t(state.loadingMore ? 'users.loadingMore' : 'users.loadMore')}
            </Button>
          )}
        </section>
      ) : null}

      {action === null ? null : (
        <UserActionDialog
          key={actionKey(action)}
          action={action}
          lastEnabledAdministrator={
            action.kind !== 'create' && isLastEnabledAdministrator(action.user)
          }
          open
          onOpenChange={(open) => {
            if (!open) {
              setAction(null);
            }
          }}
          onSessionLost={handleSessionLost}
          onSuccess={handleSuccess}
        />
      )}
    </div>
  );
}
