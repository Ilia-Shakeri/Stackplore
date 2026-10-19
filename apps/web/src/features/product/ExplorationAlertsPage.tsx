import type { ProductAlertPage } from '@stackplore/api-client';
import { Button, TechnicalText } from '@stackplore/ui';
import { useEffect, useRef, useState } from 'react';
import { useLocale } from '../../i18n';
import { useAuth } from '../auth/auth-context';
import { useProductApi, useProductSource } from './product-context';
import { useProductStream } from './product-stream';
import {
  ObservedPageHeader,
  ProductState,
  useExplorationView,
  type ProductLoadState as LoadState,
} from './exploration-ui';
import { explorationItemLimit } from './use-cursor-page';
import { alertStateLabel, alertSummary } from './alert-labels';
import { severityLabel } from './severity-labels';
const sourceOptions = (sourceId?: string) => (sourceId === undefined ? {} : { sourceId });
export function ProductAlertsPage() {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const { formatDate, t } = useLocale();
  const stream = useProductStream('alerts', selectedSource);
  const view = useExplorationView();
  const filterState = view.read('alert_state');
  const [state, setState] = useState<LoadState<ProductAlertPage>>({ kind: 'loading' });
  const [revision, setRevision] = useState(0);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState(false);
  const [truncated, setTruncated] = useState(false);
  const [acknowledging, setAcknowledging] = useState<string | null>(null);
  const [ackFailed, setAckFailed] = useState<string | null>(null);
  const loadGeneration = useRef(0);
  const moreController = useRef<AbortController | null>(null);
  const { snapshot } = useAuth();
  const administrator =
    snapshot.status === 'authenticated' && snapshot.user.role === 'administrator';

  useEffect(() => {
    const controller = new AbortController();
    const generation = ++loadGeneration.current;
    setState({ kind: 'loading' });
    setLoadingMore(false);
    setMoreFailed(false);
    setTruncated(false);
    moreController.current?.abort();
    void api.listAlerts({ signal: controller.signal, ...sourceOptions(selectedSource) }).then(
      (page) => {
        if (!controller.signal.aborted && generation === loadGeneration.current) {
          setState({ kind: 'ready', data: page });
        }
      },
      (error: unknown) => {
        if (
          !controller.signal.aborted &&
          generation === loadGeneration.current &&
          !(error instanceof DOMException && error.name === 'AbortError')
        )
          setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
      moreController.current?.abort();
      loadGeneration.current++;
    };
  }, [api, selectedSource, stream.revision, revision]);

  const loadMore = async () => {
    if (
      state.kind !== 'ready' ||
      !state.data.page.has_more ||
      loadingMore ||
      state.data.items.length >= explorationItemLimit
    )
      return;
    const generation = loadGeneration.current;
    const cursor = state.data.page.next_cursor;
    const controller = new AbortController();
    moreController.current = controller;
    setLoadingMore(true);
    setMoreFailed(false);
    try {
      const next = await api.listAlerts({
        cursor,
        signal: controller.signal,
        ...sourceOptions(selectedSource),
      });
      if (generation !== loadGeneration.current) return;
      if (next.page.has_more && next.page.next_cursor === cursor)
        throw new Error('Alert page did not advance.');
      if (
        new Set([...state.data.items, ...next.items].map((item) => item.id)).size >
        explorationItemLimit
      )
        setTruncated(true);
      setState((current) =>
        current.kind === 'ready' &&
        current.data.page.has_more &&
        current.data.page.next_cursor === cursor
          ? {
              kind: 'ready',
              data: {
                items: [
                  ...new Map(
                    [...current.data.items, ...next.items].map((item) => [item.id, item]),
                  ).values(),
                ].slice(0, explorationItemLimit),
                page: next.page,
              },
            }
          : current,
      );
    } catch {
      if (!controller.signal.aborted && generation === loadGeneration.current) setMoreFailed(true);
    } finally {
      if (generation === loadGeneration.current) setLoadingMore(false);
    }
  };
  const acknowledge = async (alertId: string) => {
    if (state.kind !== 'ready' || acknowledging !== null) return;
    setAcknowledging(alertId);
    setAckFailed(null);
    try {
      const updated = await api.acknowledgeAlert(alertId);
      setState((current) =>
        current.kind === 'ready'
          ? {
              kind: 'ready',
              data: {
                ...current.data,
                items: current.data.items.map((item) =>
                  item.id === updated.id && item.state === 'firing' ? updated : item,
                ),
              },
            }
          : current,
      );
    } catch {
      setAckFailed(alertId);
    } finally {
      setAcknowledging(null);
    }
  };
  return (
    <ObservedPageHeader
      title={t('route.alerts.title')}
      description={t('route.alerts.description')}
      streamStatus={stream.status}
    >
      <div className="exploration-filters">
        <label>
          {t('explore.alert.state')}
          <select
            value={filterState}
            onChange={(event) => {
              view.write('alert_state', event.target.value);
            }}
          >
            <option value="">{t('explore.filter.all')}</option>
            {(['pending', 'firing', 'acknowledged', 'resolved'] as const).map((value) => (
              <option key={value} value={value}>
                {alertStateLabel(value, t)}
              </option>
            ))}
          </select>
        </label>
      </div>
      <p>{t('explore.alert.loadedFilter')}</p>
      {state.kind === 'ready' &&
      state.data.items.length > 0 &&
      !state.data.items.some((alert) => filterState === '' || alert.state === filterState) ? (
        <ProductState text={t('explore.alert.noLoadedMatches')} />
      ) : null}
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? (
        <ProductState
          text={t('product.error')}
          onRetry={() => {
            setRevision((current) => current + 1);
          }}
        />
      ) : null}
      {state.kind === 'ready' && state.data.items.length === 0 ? (
        <ProductState text={t('product.alert.empty')} />
      ) : null}
      {state.kind === 'ready' && state.data.items.length > 0 ? (
        <>
          <div className="alert-list">
            {state.data.items
              .filter((alert) => filterState === '' || alert.state === filterState)
              .map((alert) => (
                <article id={`alert-${alert.id}`} key={alert.id}>
                  <div>
                    <span className={`severity-chip severity-chip--${alert.severity}`}>
                      {severityLabel(alert.severity, t)}
                    </span>
                    <span className={`alert-state alert-state--${alert.state}`}>
                      {alertStateLabel(alert.state, t)}
                    </span>
                    <time dateTime={alert.started_at}>
                      {formatDate(Date.parse(alert.started_at), {
                        dateStyle: 'medium',
                        timeStyle: 'short',
                      })}
                    </time>
                  </div>
                  <strong>{alertSummary(alert.summary_code, t)}</strong>
                  {alert.resource_id === undefined ? null : (
                    <a
                      href={`/resources?${selectedSource === undefined ? '' : `source=${encodeURIComponent(selectedSource)}&`}q=${encodeURIComponent(alert.resource_id)}`}
                    >
                      <TechnicalText>{alert.resource_id}</TechnicalText>
                    </a>
                  )}
                  {administrator && alert.state === 'firing' ? (
                    <Button
                      loading={acknowledging === alert.id}
                      onClick={() => {
                        void acknowledge(alert.id);
                      }}
                    >
                      {t('product.alert.acknowledge')}
                    </Button>
                  ) : null}
                  {ackFailed === alert.id ? (
                    <p role="alert">{t('product.alert.ackError')}</p>
                  ) : null}
                </article>
              ))}
          </div>
          {moreFailed ? <p role="alert">{t('product.alert.moreError')}</p> : null}
          {state.data.page.has_more ? (
            <p role="status">{t('product.observation.partial')}</p>
          ) : null}
          {truncated ||
          (state.data.items.length >= explorationItemLimit && state.data.page.has_more) ? (
            <p role="status">{t('explore.page.limit')}</p>
          ) : null}
          {state.data.page.has_more && state.data.items.length < explorationItemLimit ? (
            <Button
              loading={loadingMore}
              onClick={() => {
                void loadMore();
              }}
            >
              {t('product.alert.loadMore')}
            </Button>
          ) : null}
        </>
      ) : null}
    </ObservedPageHeader>
  );
}
