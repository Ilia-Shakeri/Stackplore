import type { SourceSettingsPage as SourceSettingsResult } from '@stackplore/api-client';
import { Badge, Button, TechnicalText } from '@stackplore/ui';
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';

import { useProductApi } from './product-context';
import { classifySourceSettingsFailure, type SourceSettingsFailure } from './source-settings-api';
import { sourceSettingsFailureKey } from './source-settings-messages';
import { SourceUpdateDialog, type SourceUpdateIntent } from './SourceUpdateDialog';
import { useAuth } from '../auth/auth-context';
import { useLocale } from '../../i18n';

type PageState =
  | { readonly kind: 'loading' }
  | { readonly kind: 'error'; readonly failure: SourceSettingsFailure }
  | {
      readonly kind: 'ready';
      readonly page: SourceSettingsResult;
      readonly refreshing: boolean;
      readonly failure?: SourceSettingsFailure;
    };
const maximumProfilePages = 64;

export function SourceSettingsPage({ toolbar }: { readonly toolbar?: ReactNode }) {
  const api = useProductApi();
  const { snapshot } = useAuth();
  const { t } = useLocale();
  const administrator =
    snapshot.status === 'authenticated' && snapshot.user.role === 'administrator';
  const actorId = snapshot.status === 'authenticated' ? snapshot.user.id : undefined;
  const [state, setState] = useState<PageState>({ kind: 'loading' });
  const [intent, setIntent] = useState<SourceUpdateIntent | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [cursor, setCursor] = useState<string | undefined>();
  const [previousCursors, setPreviousCursors] = useState<Array<string | undefined>>([]);
  const loadGeneration = useRef(0);
  const loadController = useRef<AbortController | null>(null);
  const actionButton = useRef<HTMLButtonElement | null>(null);
  const heading = useRef<HTMLHeadingElement>(null);

  const loadPage = useCallback(
    async (nextCursor?: string) => {
      loadController.current?.abort();
      const controller = new AbortController();
      loadController.current = controller;
      const generation = ++loadGeneration.current;
      setState((current) =>
        current.kind === 'ready'
          ? { kind: 'ready', page: current.page, refreshing: true }
          : { kind: 'loading' },
      );
      try {
        const page = await api.listSourceSettings({
          ...(nextCursor === undefined ? {} : { cursor: nextCursor }),
          signal: controller.signal,
        });
        if (controller.signal.aborted || generation !== loadGeneration.current) return false;
        setState({ kind: 'ready', page, refreshing: false });
        setCursor(nextCursor);
        return true;
      } catch (error) {
        if (controller.signal.aborted || generation !== loadGeneration.current) return false;
        const failure = classifySourceSettingsFailure(error);
        setState((current) =>
          current.kind === 'ready'
            ? { ...current, refreshing: false, failure }
            : { kind: 'error', failure },
        );
        return false;
      }
    },
    [api],
  );

  useEffect(() => {
    setIntent(null);
    setSuccess(null);
    setPreviousCursors([]);
    if (administrator) void loadPage();
    else setState({ kind: 'loading' });
    return () => {
      loadController.current?.abort();
      loadGeneration.current++;
    };
  }, [administrator, actorId, loadPage]);

  const restoreFocus = () => {
    if (actionButton.current?.isConnected && !actionButton.current.disabled)
      actionButton.current.focus();
    else heading.current?.focus();
  };

  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <h1 ref={heading} tabIndex={-1}>
          {t('route.connections.title')}
        </h1>
        <p>{t('route.connections.description')}</p>
      </header>
      {toolbar}
      {!administrator ? (
        <section className="product-state" role="alert">
          <p>{t('connections.error.denied')}</p>
        </section>
      ) : (
        <>
          <p>{t('connections.operatorNote')}</p>
          {success === null ? null : (
            <p role="status">{t('connections.update.success', { name: success })}</p>
          )}
          {state.kind === 'loading' || (state.kind === 'ready' && state.refreshing) ? (
            <p role="status">{t('connections.loading')}</p>
          ) : null}
          {state.kind === 'error' || (state.kind === 'ready' && state.failure !== undefined) ? (
            <section className="product-state" role="alert">
              <p>
                {t(
                  sourceSettingsFailureKey(
                    state.kind === 'error' ? state.failure : (state.failure ?? 'unexpected'),
                  ),
                )}
              </p>
              {state.kind === 'error' ? (
                <Button
                  onClick={() => {
                    void loadPage(cursor);
                  }}
                >
                  {t('connections.reload')}
                </Button>
              ) : null}
            </section>
          ) : null}
          {state.kind === 'ready' ? (
            <>
              <Button
                loading={state.refreshing}
                variant="outline"
                onClick={() => {
                  void loadPage(cursor);
                }}
              >
                {t('connections.reload')}
              </Button>
              {state.page.items.length === 0 ? (
                <p role="status">{t('connections.empty')}</p>
              ) : (
                <section
                  className="connection-list"
                  aria-label={t('connections.listLabel')}
                  aria-busy={state.refreshing || undefined}
                >
                  {state.page.items.map((source) => (
                    <article key={source.id}>
                      <header>
                        <div>
                          <h2>{source.name}</h2>
                          <TechnicalText>{source.id}</TechnicalText>
                        </div>
                        <Badge tone={source.enabled ? 'success' : 'neutral'}>
                          {t(source.enabled ? 'connections.enabled' : 'connections.disabled')}
                        </Badge>
                      </header>
                      <dl>
                        <div>
                          <dt>{t('connections.provider')}</dt>
                          <dd>
                            <TechnicalText>{source.provider}</TechnicalText>
                          </dd>
                        </div>
                        <div>
                          <dt>{t('connections.dataKind')}</dt>
                          <dd>
                            {t(source.synthetic ? 'connections.synthetic' : 'connections.observed')}
                          </dd>
                        </div>
                        <div>
                          <dt>{t('connections.profile')}</dt>
                          <dd>
                            {t(
                              source.profile_available
                                ? 'connections.profile.available'
                                : 'connections.profile.missing',
                            )}
                          </dd>
                        </div>
                      </dl>
                      <Button
                        variant={source.enabled ? 'outline' : 'primary'}
                        disabled={
                          state.refreshing ||
                          state.failure !== undefined ||
                          (!source.enabled && !source.profile_available)
                        }
                        aria-describedby={
                          source.profile_available ? undefined : `profile-${source.id}-missing`
                        }
                        aria-label={t(
                          source.enabled ? 'connections.disable.label' : 'connections.enable.label',
                          { name: source.name },
                        )}
                        onClick={(event) => {
                          actionButton.current = event.currentTarget;
                          setSuccess(null);
                          setIntent({ source, enabled: !source.enabled });
                        }}
                      >
                        {t(source.enabled ? 'connections.disable' : 'connections.enable')}
                      </Button>
                      {!source.profile_available ? (
                        <p id={`profile-${source.id}-missing`}>{t('connections.profile.help')}</p>
                      ) : null}
                    </article>
                  ))}
                </section>
              )}
              <nav className="users-page__actions" aria-label={t('connections.pagination')}>
                <Button
                  variant="outline"
                  disabled={
                    state.refreshing || state.failure !== undefined || previousCursors.length === 0
                  }
                  onClick={() => {
                    const next = previousCursors.at(-1);
                    void loadPage(next).then((loaded) => {
                      if (loaded) setPreviousCursors((current) => current.slice(0, -1));
                    });
                  }}
                >
                  {t('connections.previous')}
                </Button>
                <Button
                  variant="outline"
                  disabled={
                    state.refreshing ||
                    state.failure !== undefined ||
                    !state.page.page.has_more ||
                    previousCursors.length >= maximumProfilePages
                  }
                  onClick={() => {
                    if (!state.page.page.has_more) return;
                    void loadPage(state.page.page.next_cursor).then((loaded) => {
                      if (loaded) setPreviousCursors((current) => [...current, cursor]);
                    });
                  }}
                >
                  {t('connections.next')}
                </Button>
              </nav>
            </>
          ) : null}
        </>
      )}
      {administrator && intent !== null ? (
        <SourceUpdateDialog
          key={`${intent.source.id}:${String(intent.source.version)}`}
          intent={intent}
          onClose={() => {
            setIntent(null);
          }}
          onReturnFocus={restoreFocus}
          onReload={() => {
            setIntent(null);
            void loadPage(cursor);
          }}
          onUpdated={(source) => {
            setIntent(null);
            setSuccess(source.name);
            void loadPage(cursor);
          }}
        />
      ) : null}
    </div>
  );
}
