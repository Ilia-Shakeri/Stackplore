import type {
  AuditEntry,
  ProductAuditPage as ProductAuditResult,
  ProductNotificationPage as ProductNotificationResult,
  SystemReadiness,
} from '@stackplore/api-client';
import { Button, TechnicalText } from '@stackplore/ui';
import { Link } from '@tanstack/react-router';
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { useCursorPage } from './use-cursor-page';
import { useLocale } from '../../i18n';
import { useProductApi } from './product-context';
import {
  ObservedPageHeader,
  ProductState,
  type ProductLoadState as LoadState,
} from './exploration-ui';
import { alertSummary } from './alert-labels';
import { severityLabel } from './severity-labels';
export function ProductNotificationsPage() {
  const api = useProductApi();
  const { formatDate, t } = useLocale();
  const load = useCallback(
    (signal: AbortSignal, cursor?: string): Promise<ProductNotificationResult> =>
      api.listNotifications({ signal, ...(cursor === undefined ? {} : { cursor }) }),
    [api],
  );
  const page = useCursorPage(load);
  const { state, loadingMore, moreFailed, loadMore } = page;
  return (
    <ObservedPageHeader
      title={t('route.notificationCenter.title')}
      description={t('route.notificationCenter.description')}
    >
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? (
        <ProductState text={t('product.error')} onRetry={page.retry} />
      ) : null}
      {state.kind === 'ready' && state.data.items.length === 0 ? (
        <ProductState text={t('product.notification.empty')} />
      ) : null}
      {state.kind === 'ready' && state.data.items.length > 0 ? (
        <>
          <div className="notification-list" aria-label={t('product.notification.listLabel')}>
            {state.data.items.map((notification) => (
              <article key={notification.id}>
                <div>
                  <span className={`severity-chip severity-chip--${notification.severity}`}>
                    {severityLabel(notification.severity, t)}
                  </span>
                  <time dateTime={notification.created_at}>
                    {formatDate(Date.parse(notification.created_at), {
                      dateStyle: 'medium',
                      timeStyle: 'short',
                    })}
                  </time>
                </div>
                <strong>{alertSummary(notification.summary_code, t)}</strong>
                <TechnicalText>{notification.kind}</TechnicalText>
                <nav aria-label={t('product.notification.links')}>
                  {notification.source_id === undefined ||
                  notification.alert_id === undefined ? null : (
                    <Link
                      to="/alerts"
                      search={{ source: notification.source_id }}
                      hash={`alert-${notification.alert_id}`}
                    >
                      {t('product.notification.openAlert')}
                    </Link>
                  )}
                  {notification.source_id === undefined ||
                  notification.resource_id === undefined ? null : (
                    <Link
                      to="/resources"
                      search={{ source: notification.source_id, q: notification.resource_id }}
                    >
                      {t('product.notification.openResource')}
                    </Link>
                  )}
                </nav>
              </article>
            ))}
          </div>
          {moreFailed ? <p role="alert">{t('product.notification.moreError')}</p> : null}
          {page.capped ? <p role="status">{t('explore.page.limit')}</p> : null}
          {state.data.page.has_more && !page.capped ? (
            <Button loading={loadingMore} onClick={() => void loadMore()}>
              {t('product.notification.loadMore')}
            </Button>
          ) : null}
        </>
      ) : null}
    </ObservedPageHeader>
  );
}

export function ProductAuditPage() {
  const api = useProductApi();
  const { formatDate, t } = useLocale();
  const load = useCallback(
    (signal: AbortSignal, cursor?: string): Promise<ProductAuditResult> =>
      api.listAudit({ signal, ...(cursor === undefined ? {} : { cursor }) }),
    [api],
  );
  const page = useCursorPage(load);
  const { state, loadingMore, moreFailed, loadMore } = page;
  return (
    <ObservedPageHeader title={t('route.audit.title')} description={t('route.audit.description')}>
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? (
        <ProductState text={t('product.error')} onRetry={page.retry} />
      ) : null}
      {state.kind === 'ready' && state.data.items.length === 0 ? (
        <ProductState text={t('product.audit.empty')} />
      ) : null}
      {state.kind === 'ready' && state.data.items.length > 0 ? (
        <>
          <div className="audit-list" aria-label={t('product.audit.listLabel')}>
            {state.data.items.map((entry) => (
              <article key={entry.id}>
                <div>
                  <span className={`audit-outcome audit-outcome--${entry.outcome}`}>
                    {auditOutcomeLabel(entry.outcome, t)}
                  </span>
                  <time dateTime={entry.occurred_at}>
                    {formatDate(Date.parse(entry.occurred_at), {
                      dateStyle: 'medium',
                      timeStyle: 'short',
                    })}
                  </time>
                </div>
                <strong>
                  <TechnicalText>{entry.action}</TechnicalText>
                </strong>
                <dl>
                  <div>
                    <dt>{t('product.audit.actor')}</dt>
                    <dd>
                      <TechnicalText>{auditActor(entry)}</TechnicalText>
                    </dd>
                  </div>
                  <div>
                    <dt>{t('product.audit.target')}</dt>
                    <dd>
                      <TechnicalText>{auditTarget(entry)}</TechnicalText>
                    </dd>
                  </div>
                  <div>
                    <dt>{t('product.audit.summary')}</dt>
                    <dd>
                      <TechnicalText>{entry.summary_code}</TechnicalText>
                    </dd>
                  </div>
                  <div>
                    <dt>{t('product.audit.request')}</dt>
                    <dd>
                      <TechnicalText>{entry.request_id}</TechnicalText>
                    </dd>
                  </div>
                </dl>
              </article>
            ))}
          </div>
          {moreFailed ? <p role="alert">{t('product.audit.moreError')}</p> : null}
          {page.capped ? <p role="status">{t('explore.page.limit')}</p> : null}
          {state.data.page.has_more && !page.capped ? (
            <Button loading={loadingMore} onClick={() => void loadMore()}>
              {t('product.audit.loadMore')}
            </Button>
          ) : null}
        </>
      ) : null}
    </ObservedPageHeader>
  );
}

export function ProductSystemSettingsPage({ toolbar }: { readonly toolbar: ReactNode }) {
  const api = useProductApi();
  const { t } = useLocale();
  const [state, setState] = useState<LoadState<SystemReadiness>>({ kind: 'loading' });
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setState({ kind: 'loading' });
    void api.getSystemReadiness(controller.signal).then(
      (report) => {
        setState({ kind: 'ready', data: report });
      },
      (error: unknown) => {
        if (!(error instanceof DOMException && error.name === 'AbortError'))
          setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api, revision]);

  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <h1>{t('route.settings.title')}</h1>
        <p>{t('route.settings.description')}</p>
      </header>
      {toolbar}
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? (
        <ProductState
          text={t('product.error')}
          onRetry={() => {
            setRevision((current) => current + 1);
          }}
        />
      ) : null}
      {state.kind === 'ready' ? (
        <section className="readiness-panel" aria-labelledby="readiness-title">
          <header>
            <h2 id="readiness-title">{t('settings.readiness.title')}</h2>
            <span className={`readiness-state readiness-state--${state.data.state}`}>
              {readinessStateLabel(state.data.state, t)}
            </span>
          </header>
          <dl>
            {state.data.checks.map((check) => (
              <div key={check.name}>
                <dt>{t(`settings.readiness.check.${check.name}`)}</dt>
                <dd className={`readiness-state readiness-state--${check.state}`}>
                  {readinessStateLabel(check.state, t)}
                </dd>
              </div>
            ))}
          </dl>
        </section>
      ) : null}
    </div>
  );
}

function auditOutcomeLabel(outcome: AuditEntry['outcome'], t: ReturnType<typeof useLocale>['t']) {
  return t(`product.audit.outcome.${outcome}`);
}

function auditActor(entry: AuditEntry) {
  return 'id' in entry.actor ? `${entry.actor.kind}:${entry.actor.id}` : entry.actor.kind;
}

function auditTarget(entry: AuditEntry) {
  return entry.target.id === undefined
    ? entry.target.kind
    : `${entry.target.kind}:${entry.target.id}`;
}

function readinessStateLabel(
  state: SystemReadiness['state'],
  t: ReturnType<typeof useLocale>['t'],
) {
  return state === 'ready' ? t('settings.readiness.ready') : t('settings.readiness.notReady');
}
