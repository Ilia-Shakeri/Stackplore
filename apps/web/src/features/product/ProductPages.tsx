import type {
  AuditEntry,
  AvailabilityState,
  ProductAlert,
  ProductAlertPage,
  ProductAuditPage as ProductAuditResult,
  ProductEnvironmentPage,
  ProductEvent,
  ProductLogRecord,
  ProductMetricHistoryResult,
  ProductMetricResult,
  ProductNotificationPage as ProductNotificationResult,
  ProductRelationship,
  ProductResource,
  ProductResourceHealth,
  SourceSummary,
  SystemReadiness,
} from '@stackplore/api-client';
import { Button, TechnicalText } from '@stackplore/ui';
import { Link } from '@tanstack/react-router';
import {
  useEffect,
  useState,
  type Dispatch,
  type ReactNode,
  type SetStateAction,
  type SyntheticEvent,
} from 'react';

import { useProductApi, useProductSource } from './product-context';
import { useProductStream, type ProductStreamStatus } from './product-stream';
import { TopologyGraph } from './TopologyGraph';
import { useLocale, type TextMessageKey } from '../../i18n';
import { useAuth } from '../auth/auth-context';

type LoadState<T> =
  | { readonly kind: 'loading' }
  | { readonly kind: 'error' }
  | { readonly kind: 'unavailable'; readonly availability: AvailabilityState }
  | { readonly kind: 'ready'; readonly data: T };

const sourceOptions = (sourceId?: string) => (sourceId === undefined ? {} : { sourceId });

export function ProductOverviewPage() {
  const api = useProductApi();
  const { selectSource, selectedSource } = useProductSource();
  const { formatDate, formatNumber, t } = useLocale();
  const stream = useProductStream('metrics', selectedSource);
  const [state, setState] = useState<
    LoadState<{
      readonly alerts: readonly ProductAlert[];
      readonly environments: ProductEnvironmentPage;
      readonly history: ProductMetricHistoryResult;
      readonly metrics: ProductMetricResult;
      readonly sources: readonly SourceSummary[];
    }>
  >({ kind: 'loading' });

  useEffect(() => {
    const controller = new AbortController();
    setState({ kind: 'loading' });
    void Promise.all([
      api.listSources(controller.signal),
      api.listMetrics({ signal: controller.signal, ...sourceOptions(selectedSource) }),
      api.listAlerts({ signal: controller.signal, ...sourceOptions(selectedSource) }),
      api.listEnvironments({ signal: controller.signal, ...sourceOptions(selectedSource) }),
      api.listMetricHistory({ signal: controller.signal, ...sourceOptions(selectedSource) }),
    ]).then(
      ([sources, metrics, alerts, environments, history]) => {
        if (controller.signal.aborted) return;
        setState({
          kind: 'ready',
          data: {
            alerts: alerts.items,
            environments,
            history,
            metrics,
            sources,
          },
        });
      },
      (error: unknown) => {
        if (!(error instanceof DOMException && error.name === 'AbortError'))
          setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api, selectedSource, stream.revision]);

  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <p className="route-page__eyebrow">{t('route.overview.eyebrow')}</p>
        <h1>{t('route.overview.title')}</h1>
        <p>{t('route.overview.description')}</p>
        <ProductStreamBadge status={stream.status} />
      </header>
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {state.kind === 'ready' ? (
        <>
          <label className="product-source-select">
            <span>{t('product.source.label')}</span>
            <select
              onChange={(event) => {
                selectSource(event.target.value);
              }}
              value={selectedSource ?? state.data.sources[0]?.id ?? ''}
            >
              {state.data.sources.map((source) => (
                <option key={source.id} value={source.id}>
                  {source.name}
                </option>
              ))}
            </select>
          </label>
          <MetricSummary
            alerts={state.data.alerts}
            environments={state.data.environments}
            metrics={state.data.metrics}
            formatNumber={formatNumber}
            t={t}
          />
          <div className="trend-grid">
            <MetricTrend
              formatDate={formatDate}
              formatNumber={formatNumber}
              history={state.data.history}
              metric="cpu.usage.cores"
              title={t('product.trend.cpu')}
              unit={t('product.metric.cores')}
              t={t}
            />
            <MetricTrend
              divisor={1024 * 1024}
              formatDate={formatDate}
              formatNumber={formatNumber}
              history={state.data.history}
              metric="memory.usage.bytes"
              title={t('product.trend.memory')}
              unit={t('product.metric.mib')}
              t={t}
            />
          </div>
          <div className="source-grid">
            {state.data.sources.map((source) => (
              <article className="source-card" key={source.id}>
                <div className="source-card__heading">
                  <h2>{source.name}</h2>
                  <div>
                    <span>{source.synthetic ? t('product.demoBadge') : source.provider}</span>
                    <span className={`source-state source-state--${source.availability}`}>
                      {availabilityLabel(source.availability, t)}
                    </span>
                  </div>
                </div>
                <p>{t('product.resourcesCount', { count: source.resource_count })}</p>
                <dl className="health-grid">
                  <HealthCount
                    label={t('product.health.healthy')}
                    value={source.health.healthy}
                    tone="healthy"
                  />
                  <HealthCount
                    label={t('product.health.degraded')}
                    value={source.health.degraded}
                    tone="degraded"
                  />
                  <HealthCount
                    label={t('product.health.unhealthy')}
                    value={source.health.unhealthy}
                    tone="unhealthy"
                  />
                  <HealthCount
                    label={t('product.health.unknown')}
                    value={source.health.unknown}
                    tone="unknown"
                  />
                </dl>
              </article>
            ))}
          </div>
        </>
      ) : null}
    </div>
  );
}

export function ProductTopologyPage() {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const { t } = useLocale();
  const [state, setState] = useState<
    LoadState<{
      readonly relationships: readonly ProductRelationship[];
      readonly resources: readonly ProductResource[];
    }>
  >({ kind: 'loading' });

  useEffect(() => {
    const controller = new AbortController();
    void Promise.all([
      api.listTopology({ signal: controller.signal, ...sourceOptions(selectedSource) }),
      api.listResources({ signal: controller.signal, ...sourceOptions(selectedSource) }),
    ]).then(
      ([topology, resources]) => {
        if (controller.signal.aborted) return;
        if (!dataAllowed(topology.availability) || !dataAllowed(resources.availability)) {
          setState({
            kind: 'unavailable',
            availability: !dataAllowed(topology.availability)
              ? topology.availability
              : resources.availability,
          });
          return;
        }
        setState({
          kind: 'ready',
          data: { relationships: topology.items, resources: resources.items },
        });
      },
      (error: unknown) => {
        if (!(error instanceof DOMException && error.name === 'AbortError'))
          setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api, selectedSource]);

  const names =
    state.kind === 'ready' ? resourceNameMap(state.data.resources) : new Map<string, string>();
  return (
    <ObservedPageHeader
      title={t('route.topology.title')}
      description={t('route.topology.description')}
    >
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {state.kind === 'unavailable' ? (
        <ProductState text={availabilityLabel(state.availability, t)} />
      ) : null}
      {state.kind === 'ready' ? (
        <>
          <TopologyGraph
            relationships={state.data.relationships}
            resources={state.data.resources}
          />
          <details className="topology-list-view">
            <summary>{t('product.topology.listLabel')}</summary>
            <div className="observed-list">
              {state.data.relationships.map((relationship) => (
                <article key={relationship.id}>
                  <strong>{names.get(relationship.from) ?? relationship.from}</strong>
                  <span>{relationshipLabel(relationship.kind, t)}</span>
                  <strong>{names.get(relationship.to) ?? relationship.to}</strong>
                  <small>
                    {t('product.topology.confidence', {
                      count: Math.round(relationship.confidence * 100),
                    })}
                  </small>
                </article>
              ))}
            </div>
          </details>
        </>
      ) : null}
    </ObservedPageHeader>
  );
}

export function ProductLogsPage() {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const { formatDate, t } = useLocale();
  const stream = useProductStream('logs', selectedSource);
  const [state, setState] = useState<LoadState<readonly ProductLogRecord[]>>({ kind: 'loading' });

  useEffect(
    () =>
      loadObserved(
        (signal) => api.listLogs({ signal, ...sourceOptions(selectedSource) }),
        setState,
      ),
    [api, selectedSource, stream.revision],
  );
  return (
    <ObservedPageHeader
      title={t('route.logs.title')}
      description={t('route.logs.description')}
      streamStatus={stream.status}
    >
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {state.kind === 'unavailable' ? (
        <ProductState text={availabilityLabel(state.availability, t)} />
      ) : null}
      {state.kind === 'ready' ? (
        <div className="log-list" aria-label={t('product.logs.listLabel')}>
          {state.data.map((record) => (
            <article key={record.id}>
              <div>
                <span className={`severity-chip severity-chip--${record.severity}`}>
                  {record.severity}
                </span>
                <time dateTime={record.timestamp}>
                  {formatDate(Date.parse(record.timestamp), { timeStyle: 'medium' })}
                </time>
                <TechnicalText>{record.resource_id}</TechnicalText>
              </div>
              <pre dir="ltr" data-ui-technical>
                {record.message}
              </pre>
            </article>
          ))}
        </div>
      ) : null}
    </ObservedPageHeader>
  );
}

export function ProductEventsPage() {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const { formatDate, t } = useLocale();
  const stream = useProductStream('events', selectedSource);
  const [state, setState] = useState<LoadState<readonly ProductEvent[]>>({ kind: 'loading' });

  useEffect(
    () =>
      loadObserved(
        (signal) => api.listEvents({ signal, ...sourceOptions(selectedSource) }),
        setState,
      ),
    [api, selectedSource, stream.revision],
  );
  return (
    <ObservedPageHeader
      title={t('route.events.title')}
      description={t('route.events.description')}
      streamStatus={stream.status}
    >
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {state.kind === 'unavailable' ? (
        <ProductState text={availabilityLabel(state.availability, t)} />
      ) : null}
      {state.kind === 'ready' ? (
        <div className="event-list">
          {state.data.map((event) => (
            <article key={event.id}>
              <div>
                <span className={`severity-chip severity-chip--${event.severity}`}>
                  {event.severity}
                </span>
                <time dateTime={event.occurred_at}>
                  {formatDate(Date.parse(event.occurred_at), {
                    dateStyle: 'medium',
                    timeStyle: 'short',
                  })}
                </time>
              </div>
              <strong>{event.summary}</strong>
              <small>{event.reason_code}</small>
            </article>
          ))}
        </div>
      ) : null}
    </ObservedPageHeader>
  );
}

export function ProductAlertsPage() {
  const api = useProductApi();
  const { selectedSource } = useProductSource();
  const { formatDate, t } = useLocale();
  const stream = useProductStream('alerts', selectedSource);
  const [state, setState] = useState<LoadState<ProductAlertPage>>({ kind: 'loading' });
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState(false);
  const [acknowledging, setAcknowledging] = useState<string | null>(null);
  const [ackFailed, setAckFailed] = useState<string | null>(null);
  const { snapshot } = useAuth();
  const administrator =
    snapshot.status === 'authenticated' && snapshot.user.role === 'administrator';

  useEffect(() => {
    const controller = new AbortController();
    setState({ kind: 'loading' });
    void api.listAlerts({ signal: controller.signal, ...sourceOptions(selectedSource) }).then(
      (page) => {
        setState({ kind: 'ready', data: page });
      },
      (error: unknown) => {
        if (!(error instanceof DOMException && error.name === 'AbortError'))
          setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api, selectedSource, stream.revision]);

  const loadMore = async () => {
    if (state.kind !== 'ready' || !state.data.page.has_more || loadingMore) return;
    setLoadingMore(true);
    setMoreFailed(false);
    try {
      const next = await api.listAlerts({
        cursor: state.data.page.next_cursor,
        ...sourceOptions(selectedSource),
      });
      setState({
        kind: 'ready',
        data: { items: [...state.data.items, ...next.items], page: next.page },
      });
    } catch {
      setMoreFailed(true);
    } finally {
      setLoadingMore(false);
    }
  };
  const acknowledge = async (alertId: string) => {
    if (state.kind !== 'ready' || acknowledging !== null) return;
    setAcknowledging(alertId);
    setAckFailed(null);
    try {
      const updated = await api.acknowledgeAlert(alertId);
      setState({
        kind: 'ready',
        data: {
          ...state.data,
          items: state.data.items.map((item) => (item.id === updated.id ? updated : item)),
        },
      });
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
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {state.kind === 'ready' && state.data.items.length === 0 ? (
        <ProductState text={t('product.alert.empty')} />
      ) : null}
      {state.kind === 'ready' && state.data.items.length > 0 ? (
        <>
          <div className="alert-list">
            {state.data.items.map((alert) => (
              <article id={`alert-${alert.id}`} key={alert.id}>
                <div>
                  <span className={`severity-chip severity-chip--${alert.severity}`}>
                    {alert.severity}
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
                  <TechnicalText>{alert.resource_id}</TechnicalText>
                )}
                {administrator && !alert.synthetic && alert.state === 'firing' ? (
                  <Button
                    loading={acknowledging === alert.id}
                    onClick={() => {
                      void acknowledge(alert.id);
                    }}
                  >
                    {t('product.alert.acknowledge')}
                  </Button>
                ) : null}
                {ackFailed === alert.id ? <p role="alert">{t('product.alert.ackError')}</p> : null}
              </article>
            ))}
          </div>
          {moreFailed ? <p role="alert">{t('product.alert.moreError')}</p> : null}
          {state.data.page.has_more ? (
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

export function ProductNotificationsPage() {
  const api = useProductApi();
  const { formatDate, t } = useLocale();
  const [state, setState] = useState<LoadState<ProductNotificationResult>>({ kind: 'loading' });
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    void api.listNotifications({ signal: controller.signal }).then(
      (page) => {
        setState({ kind: 'ready', data: page });
      },
      (error: unknown) => {
        if (!(error instanceof DOMException && error.name === 'AbortError'))
          setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api]);

  const loadMore = async () => {
    if (state.kind !== 'ready' || !state.data.page.has_more || loadingMore) return;
    setLoadingMore(true);
    setMoreFailed(false);
    try {
      const next = await api.listNotifications({ cursor: state.data.page.next_cursor });
      setState({
        kind: 'ready',
        data: { items: [...state.data.items, ...next.items], page: next.page },
      });
    } catch {
      setMoreFailed(true);
    } finally {
      setLoadingMore(false);
    }
  };

  return (
    <ObservedPageHeader
      title={t('route.notificationCenter.title')}
      description={t('route.notificationCenter.description')}
    >
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
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
                    {notification.severity}
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
                      search={{ source: notification.source_id }}
                      hash={`resource-${notification.resource_id}`}
                    >
                      {t('product.notification.openResource')}
                    </Link>
                  )}
                </nav>
              </article>
            ))}
          </div>
          {moreFailed ? <p role="alert">{t('product.notification.moreError')}</p> : null}
          {state.data.page.has_more ? (
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
  const [state, setState] = useState<LoadState<ProductAuditResult>>({ kind: 'loading' });
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    void api.listAudit({ signal: controller.signal }).then(
      (page) => {
        setState({ kind: 'ready', data: page });
      },
      (error: unknown) => {
        if (!(error instanceof DOMException && error.name === 'AbortError'))
          setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api]);

  const loadMore = async () => {
    if (state.kind !== 'ready' || !state.data.page.has_more || loadingMore) return;
    setLoadingMore(true);
    setMoreFailed(false);
    try {
      const next = await api.listAudit({ cursor: state.data.page.next_cursor });
      setState({
        kind: 'ready',
        data: { items: [...state.data.items, ...next.items], page: next.page },
      });
    } catch {
      setMoreFailed(true);
    } finally {
      setLoadingMore(false);
    }
  };

  return (
    <ObservedPageHeader title={t('route.audit.title')} description={t('route.audit.description')}>
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
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
          {state.data.page.has_more ? (
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

  useEffect(() => {
    const controller = new AbortController();
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
  }, [api]);

  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <h1>{t('route.settings.title')}</h1>
        <p>{t('route.settings.description')}</p>
      </header>
      {toolbar}
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
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

export function ProductConnectionsSettingsPage({ toolbar }: { readonly toolbar: ReactNode }) {
  const api = useProductApi();
  const { t } = useLocale();
  const [state, setState] = useState<LoadState<readonly SourceSummary[]>>({ kind: 'loading' });

  useEffect(() => {
    const controller = new AbortController();
    void api.listSources(controller.signal).then(
      (sources) => {
        setState({ kind: 'ready', data: sources });
      },
      (error: unknown) => {
        if (!(error instanceof DOMException && error.name === 'AbortError'))
          setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api]);

  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <h1>{t('route.connections.title')}</h1>
        <p>{t('route.connections.description')}</p>
      </header>
      {toolbar}
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {state.kind === 'ready' && state.data.length === 0 ? (
        <ProductState text={t('connections.empty')} />
      ) : null}
      {state.kind === 'ready' && state.data.length > 0 ? (
        <section className="connection-list" aria-label={t('connections.listLabel')}>
          {state.data.map((source) => (
            <article key={source.id}>
              <header>
                <div>
                  <h2>{source.name}</h2>
                  <TechnicalText>{source.id}</TechnicalText>
                </div>
                <span className={`source-state source-state--${source.availability}`}>
                  {availabilityLabel(source.availability, t)}
                </span>
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
                    {source.synthetic ? t('connections.synthetic') : t('connections.observed')}
                  </dd>
                </div>
                <div>
                  <dt>{t('connections.resources')}</dt>
                  <dd>{t('product.resourcesCount', { count: source.resource_count })}</dd>
                </div>
              </dl>
            </article>
          ))}
        </section>
      ) : null}
    </div>
  );
}

export function ProductSearchPage({ query }: { readonly query: string }) {
  return <ProductResourcesPage initialQuery={query} searchMode />;
}

export function ProductResourcesPage({
  initialQuery,
  searchMode = false,
}: { readonly initialQuery?: string; readonly searchMode?: boolean } = {}) {
  const api = useProductApi();
  const { selectedSource, selectSource, query, selectQuery } = useProductSource();
  const { t } = useLocale();
  const initialSearch = initialQuery ?? query;
  const [search, setSearch] = useState(initialSearch);
  const [appliedSearch, setAppliedSearch] = useState(initialSearch);
  const [state, setState] = useState<LoadState<readonly ProductResource[]>>({ kind: 'loading' });
  const [sources, setSources] = useState<readonly SourceSummary[]>([]);

  useEffect(() => {
    setSearch(initialSearch);
    setAppliedSearch(initialSearch);
  }, [initialSearch]);

  useEffect(() => {
    const controller = new AbortController();
    void api.listSources(controller.signal).then(setSources, () => {
      setSources([]);
    });
    return () => {
      controller.abort();
    };
  }, [api]);

  useEffect(() => {
    const controller = new AbortController();
    setState({ kind: 'loading' });
    void api
      .listResources({
        ...(appliedSearch === '' ? {} : { search: appliedSearch }),
        ...sourceOptions(selectedSource),
        signal: controller.signal,
      })
      .then(
        (page) => {
          if (controller.signal.aborted) return;
          if (!dataAllowed(page.availability)) {
            setState({ kind: 'unavailable', availability: page.availability });
            return;
          }
          setState({ kind: 'ready', data: page.items });
        },
        (error: unknown) => {
          if (!(error instanceof DOMException && error.name === 'AbortError'))
            setState({ kind: 'error' });
        },
      );
    return () => {
      controller.abort();
    };
  }, [api, appliedSearch, selectedSource]);

  const submit = (event: SyntheticEvent<HTMLFormElement, SubmitEvent>) => {
    event.preventDefault();
    const value = search.trim();
    setAppliedSearch(value);
    selectQuery(value);
  };
  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <h1>{t(searchMode ? 'route.search.title' : 'route.resources.title')}</h1>
        <p>{t(searchMode ? 'route.search.description' : 'route.resources.description')}</p>
      </header>
      <form className="resource-search" onSubmit={submit} role="search">
        <label htmlFor="resource-source">{t('product.source.label')}</label>
        <select
          id="resource-source"
          onChange={(event) => {
            selectSource(event.target.value);
          }}
          value={selectedSource ?? sources[0]?.id ?? ''}
        >
          {sources.map((source) => (
            <option key={source.id} value={source.id}>
              {source.name}
            </option>
          ))}
        </select>
        <label htmlFor="resource-search">{t('product.search.label')}</label>
        <div>
          <input
            id="resource-search"
            maxLength={200}
            onChange={(event) => {
              setSearch(event.target.value);
            }}
            placeholder={t('product.search.placeholder')}
            value={search}
          />
          <Button type="submit">{t('product.search.submit')}</Button>
        </div>
      </form>
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {state.kind === 'unavailable' ? (
        <ProductState text={availabilityLabel(state.availability, t)} />
      ) : null}
      {state.kind === 'ready' && state.data.length === 0 ? (
        <ProductState text={t('product.empty')} />
      ) : null}
      {state.kind === 'ready' && state.data.length > 0 ? (
        <div className="resource-list">
          {state.data.map((resource) => (
            <a
              className="resource-row ui-focus-ring"
              href={`/resources/${encodeURIComponent(resource.source_id)}/${encodeURIComponent(resource.provider)}/${encodeURIComponent(resource.kind)}/${encodeURIComponent(resource.id)}?source=${encodeURIComponent(resource.source_id)}`}
              id={`resource-${resource.id}`}
              key={resource.id}
            >
              <span>
                <strong>{resource.name}</strong>
                <small>{resource.kind}</small>
              </span>
              <span className="demo-chip">
                {resource.synthetic ? t('product.demoBadge') : resource.provider}
              </span>
            </a>
          ))}
        </div>
      ) : null}
    </div>
  );
}

interface ProductResourceDetailPageProps {
  readonly id: string;
  readonly kind: string;
  readonly provider: string;
  readonly source: string;
}

interface ResourceDetailData {
  readonly health: ProductResourceHealth;
  readonly resource: ProductResource;
}

export function ProductResourceDetailPage({
  id,
  kind,
  provider,
  source,
}: ProductResourceDetailPageProps) {
  const api = useProductApi();
  const { formatDate, t } = useLocale();
  const [state, setState] = useState<LoadState<ResourceDetailData>>({ kind: 'loading' });

  useEffect(() => {
    const controller = new AbortController();
    setState({ kind: 'loading' });
    void Promise.all([
      api.getResource(id, controller.signal, source),
      api.getResourceHealth(id, controller.signal, source),
    ]).then(
      ([resource, health]) => {
        if (controller.signal.aborted) return;
        if (
          resource.id !== id ||
          resource.source_id !== source ||
          resource.kind !== kind ||
          resource.provider !== provider
        ) {
          setState({ kind: 'error' });
          return;
        }
        setState({ kind: 'ready', data: { health, resource } });
      },
      (error: unknown) => {
        if (!(error instanceof DOMException && error.name === 'AbortError'))
          setState({ kind: 'error' });
      },
    );
    return () => {
      controller.abort();
    };
  }, [api, id, kind, provider, source]);

  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <h1>{t('route.resourceDetail.title')}</h1>
        <p>{t('route.resourceDetail.description')}</p>
      </header>
      <dl className="resource-identity" aria-label={t('route.resourceDetail.identityLabel')}>
        <Identity label={t('route.resourceDetail.provider')} value={provider} />
        <Identity label={t('route.resourceDetail.kind')} value={kind} />
        <Identity label={t('route.resourceDetail.identifier')} value={id} />
      </dl>
      {state.kind === 'loading' ? <ProductState text={t('product.loading')} /> : null}
      {state.kind === 'error' ? <ProductState text={t('product.error')} /> : null}
      {state.kind === 'ready' ? (
        <section className="resource-detail-card" aria-labelledby="resource-detail-name">
          <div className="resource-detail-card__heading">
            <div>
              <p>{t('product.detail.name')}</p>
              <h2 id="resource-detail-name">{state.data.resource.name}</h2>
            </div>
            <span className={`health-pill health-pill--${state.data.health.state}`}>
              {healthLabel(state.data.health.state, t)}
            </span>
          </div>
          <dl>
            <Identity
              label={t('product.detail.scope')}
              value={`${state.data.resource.scope_kind}:${state.data.resource.scope_name ?? '-'}`}
            />
            <div>
              <dt>{t('product.detail.observed')}</dt>
              <dd>
                {formatDate(Date.parse(state.data.resource.observed_at), {
                  dateStyle: 'medium',
                  timeStyle: 'medium',
                })}
              </dd>
            </div>
            <div>
              <dt>{t('product.detail.reasons')}</dt>
              <dd>
                {state.data.health.reason_codes.length === 0
                  ? t('product.detail.noReasons')
                  : state.data.health.reason_codes.join(', ')}
              </dd>
            </div>
          </dl>
        </section>
      ) : null}
    </div>
  );
}

function ProductState({ text }: { readonly text: string }) {
  return (
    <section className="product-state" aria-live="polite">
      <p>{text}</p>
    </section>
  );
}

function Identity({ label, value }: { readonly label: string; readonly value: string }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>
        <TechnicalText>{value}</TechnicalText>
      </dd>
    </div>
  );
}

function healthLabel(state: ProductResourceHealth['state'], t: ReturnType<typeof useLocale>['t']) {
  switch (state) {
    case 'healthy':
      return t('product.health.healthy');
    case 'degraded':
      return t('product.health.degraded');
    case 'unhealthy':
      return t('product.health.unhealthy');
    case 'unknown':
      return t('product.health.unknown');
  }
}

function availabilityLabel(state: AvailabilityState, t: ReturnType<typeof useLocale>['t']) {
  switch (state) {
    case 'available':
      return t('product.source.available');
    case 'stale':
      return t('product.source.stale');
    case 'unavailable':
      return t('product.source.unavailable');
    case 'unsupported':
      return t('product.source.unsupported');
    case 'permission_denied':
      return t('product.source.permissionDenied');
  }
}

function dataAllowed(state: AvailabilityState) {
  return state === 'available' || state === 'stale';
}

function HealthCount({
  label,
  tone,
  value,
}: {
  readonly label: string;
  readonly tone: string;
  readonly value: number;
}) {
  return (
    <div className={`health-count health-count--${tone}`}>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

function ObservedPageHeader({
  children,
  description,
  streamStatus,
  title,
}: {
  readonly children: ReactNode;
  readonly description: string;
  readonly streamStatus?: ProductStreamStatus;
  readonly title: string;
}) {
  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <h1>{title}</h1>
        <p>{description}</p>
        {streamStatus === undefined ? null : <ProductStreamBadge status={streamStatus} />}
      </header>
      {children}
    </div>
  );
}

function ProductStreamBadge({ status }: { readonly status: ProductStreamStatus }) {
  const { t } = useLocale();
  const label = t(`product.stream.${status}`);
  return (
    <span className={`stream-state stream-state--${status}`} role="status">
      <span aria-hidden="true" />
      {label}
    </span>
  );
}

function MetricSummary({
  alerts,
  environments,
  formatNumber,
  metrics,
  t,
}: {
  readonly alerts: readonly ProductAlert[];
  readonly environments: ProductEnvironmentPage;
  readonly formatNumber: ReturnType<typeof useLocale>['formatNumber'];
  readonly metrics: ProductMetricResult;
  readonly t: ReturnType<typeof useLocale>['t'];
}) {
  const unavailable = t('product.source.unavailable');
  const sum = (name: string) => {
    if (!dataAllowed(metrics.availability)) return null;
    const samples = metrics.items.filter((sample) => sample.name === name);
    return samples.length === 0 ? null : samples.reduce((total, sample) => total + sample.value, 0);
  };
  const formatMetric = (name: string, divisor = 1, unit?: string, maximumFractionDigits = 0) => {
    const total = sum(name);
    if (total === null)
      return dataAllowed(metrics.availability)
        ? unavailable
        : availabilityLabel(metrics.availability, t);
    const value = formatNumber(total / divisor, { maximumFractionDigits });
    return unit === undefined ? value : `${value} ${unit}`;
  };
  const values = [
    {
      label: t('product.metric.environments'),
      value: dataAllowed(environments.availability)
        ? formatNumber(environments.items.length)
        : availabilityLabel(environments.availability, t),
    },
    {
      label: t('product.metric.cpu'),
      value: formatMetric('cpu.usage.cores', 1, t('product.metric.cores'), 2),
    },
    {
      label: t('product.metric.memory'),
      value: formatMetric('memory.usage.bytes', 1024 * 1024, t('product.metric.mib')),
    },
    { label: t('product.metric.restarts'), value: formatMetric('restart.count') },
    {
      label: t('product.metric.activeAlerts'),
      value: formatNumber(alerts.filter((alert) => alert.state !== 'resolved').length),
    },
  ];
  return (
    <section className="metric-summary" aria-label={t('product.metric.label')}>
      {metrics.availability === 'stale' ||
      metrics.completeness === 'partial' ||
      environments.availability === 'stale' ||
      environments.completeness === 'partial' ||
      environments.page.has_more ? (
        <p role="status">
          {metrics.availability === 'stale' || environments.availability === 'stale'
            ? t('product.source.stale')
            : null}{' '}
          {metrics.completeness === 'partial' ||
          environments.completeness === 'partial' ||
          environments.page.has_more
            ? t('product.observation.partial')
            : null}
        </p>
      ) : null}
      {values.map((metric) => (
        <div key={metric.label}>
          <span>{metric.label}</span>
          <strong>{metric.value}</strong>
        </div>
      ))}
    </section>
  );
}

function MetricTrend({
  divisor = 1,
  formatDate,
  formatNumber,
  history,
  metric,
  t,
  title,
  unit,
}: {
  readonly divisor?: number;
  readonly formatDate: ReturnType<typeof useLocale>['formatDate'];
  readonly formatNumber: ReturnType<typeof useLocale>['formatNumber'];
  readonly history: ProductMetricHistoryResult;
  readonly metric: ProductMetricHistoryResult['items'][number]['name'];
  readonly t: ReturnType<typeof useLocale>['t'];
  readonly title: string;
  readonly unit: string;
}) {
  const totals = new Map<string, number>();
  for (const sample of dataAllowed(history.availability) ? history.items : []) {
    if (sample.name === metric) {
      totals.set(sample.sampled_at, (totals.get(sample.sampled_at) ?? 0) + sample.value / divisor);
    }
  }
  const points = [...totals].sort(([left], [right]) => left.localeCompare(right));
  const maximum = Math.max(1, ...points.map(([, value]) => value));
  const polyline = points
    .map(([, value], index) => {
      const x = points.length === 1 ? 160 : (index / (points.length - 1)) * 320;
      const y = 88 - (value / maximum) * 80;
      return `${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(' ');
  return (
    <section className="trend-card">
      <header>
        <div>
          <h2>{title}</h2>
          <p>
            {dataAllowed(history.availability)
              ? t('product.trend.lastHour')
              : availabilityLabel(history.availability, t)}
          </p>
          {history.availability === 'stale' ? <p>{t('product.source.stale')}</p> : null}
          {history.completeness === 'partial' ? <p>{t('product.observation.partial')}</p> : null}
        </div>
        <strong>
          {points.length === 0
            ? t('product.trend.empty')
            : `${formatNumber(points.at(-1)?.[1] ?? 0, { maximumFractionDigits: 2 })} ${unit}`}
        </strong>
      </header>
      {points.length === 0 ? null : (
        <svg viewBox="0 0 320 96" role="img" aria-label={title} preserveAspectRatio="none">
          <title>{title}</title>
          <line x1="0" x2="320" y1="88" y2="88" />
          <polyline points={polyline} />
        </svg>
      )}
      <details>
        <summary>{t('product.trend.table')}</summary>
        <div className="trend-table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col">{t('product.trend.time')}</th>
                <th scope="col">{t('product.trend.value')}</th>
              </tr>
            </thead>
            <tbody>
              {points.map(([timestamp, value]) => (
                <tr key={timestamp}>
                  <td>
                    <time dateTime={timestamp}>
                      {formatDate(Date.parse(timestamp), { timeStyle: 'short' })}
                    </time>
                  </td>
                  <td>{`${formatNumber(value, { maximumFractionDigits: 2 })} ${unit}`}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
    </section>
  );
}

function alertStateLabel(state: ProductAlert['state'], t: ReturnType<typeof useLocale>['t']) {
  switch (state) {
    case 'pending':
      return t('product.alert.pending');
    case 'firing':
      return t('product.alert.firing');
    case 'acknowledged':
      return t('product.alert.acknowledged');
    case 'resolved':
      return t('product.alert.resolved');
  }
}

function alertSummary(code: string, t: ReturnType<typeof useLocale>['t']) {
  const messages: Readonly<Record<string, TextMessageKey>> = {
    'demo.alert.checkout_unhealthy': 'product.alert.checkoutUnhealthy',
    'alert.resource_unavailable.matched': 'product.alert.resourceUnavailable',
    'alert.resource_unavailable.clear': 'product.alert.resourceUnavailableCleared',
    'alert.resource_unhealthy.matched': 'product.alert.resourceUnhealthy',
    'alert.resource_unhealthy.clear': 'product.alert.resourceUnhealthyCleared',
    'alert.restart_threshold.matched': 'product.alert.restartThreshold',
    'alert.restart_threshold.clear': 'product.alert.restartThresholdCleared',
    'alert.cpu_threshold.matched': 'product.alert.cpuThreshold',
    'alert.cpu_threshold.clear': 'product.alert.cpuThresholdCleared',
    'alert.memory_threshold.matched': 'product.alert.memoryThreshold',
    'alert.memory_threshold.clear': 'product.alert.memoryThresholdCleared',
    'alert.workload_unavailable_replicas.matched': 'product.alert.workloadUnavailableReplicas',
    'alert.workload_unavailable_replicas.clear': 'product.alert.workloadUnavailableReplicasCleared',
    'alert.event_severity_match.matched': 'product.alert.eventSeverity',
    'alert.event_severity_match.clear': 'product.alert.eventSeverityCleared',
    'alert.provider_connection_lost.matched': 'product.alert.providerConnectionLost',
    'alert.provider_connection_lost.clear': 'product.alert.providerConnectionLostCleared',
  };
  return t(
    Object.hasOwn(messages, code)
      ? (messages[code] ?? 'product.alert.unknown')
      : 'product.alert.unknown',
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

function resourceNameMap(resources: readonly ProductResource[]) {
  return new Map(resources.map((resource) => [resource.id, resource.name]));
}

function relationshipLabel(
  kind: ProductRelationship['kind'],
  t: ReturnType<typeof useLocale>['t'],
) {
  switch (kind) {
    case 'owns':
      return t('product.topology.owns');
    case 'selects':
      return t('product.topology.selects');
    case 'routes_to':
      return t('product.topology.routesTo');
    case 'depends_on':
      return t('product.topology.dependsOn');
    case 'mounts':
      return t('product.topology.mounts');
    case 'references':
      return t('product.topology.references');
    case 'member_of':
      return t('product.topology.memberOf');
  }
}

function loadObserved<T>(
  load: (
    signal: AbortSignal,
  ) => Promise<{ readonly availability?: AvailabilityState; readonly items: readonly T[] }>,
  setState: Dispatch<SetStateAction<LoadState<readonly T[]>>>,
) {
  const controller = new AbortController();
  void load(controller.signal).then(
    (page) => {
      if (controller.signal.aborted) return;
      if (page.availability !== undefined && !dataAllowed(page.availability)) {
        setState({ kind: 'unavailable', availability: page.availability });
        return;
      }
      setState({ kind: 'ready', data: page.items });
    },
    (error: unknown) => {
      if (!(error instanceof DOMException && error.name === 'AbortError'))
        setState({ kind: 'error' });
    },
  );
  return () => {
    controller.abort();
  };
}
