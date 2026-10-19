import type {
  ProductAlertPage,
  ProductEnvironmentPage,
  ProductMetricHistoryResult,
  ProductMetricResult,
  ProductEventPage,
  ProductResourcePage,
  SourceSummary,
} from '@stackplore/api-client';
import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocale } from '../../i18n';
import { useProductApi, useProductSource } from './product-context';
import { useProductStream } from './product-stream';
import {
  ProductState,
  dataAllowed,
  availabilityLabel,
  ProductStreamBadge,
  type ProductLoadState as LoadState,
  useExplorationView,
} from './exploration-ui';
import { TimeRangeControl } from './exploration-time';
import { timeQuery } from './product-api-common';
import { OverviewInsights } from './OverviewInsights';
export { ProductTopologyPage } from './ExplorationTopologyPage';
export { ProductLogsPage } from './ExplorationLogsPage';
export { ProductEventsPage } from './ExplorationEventsPage';
export { ProductResourcesPage, ProductSearchPage } from './ExplorationResourcesPage';
export { ProductResourceDetailPage } from './ExplorationResourceDetailPage';
export { ProductAlertsPage } from './ExplorationAlertsPage';
export {
  ProductAuditPage,
  ProductNotificationsPage,
  ProductSystemSettingsPage,
} from './ProductHistoryPages';
const MetricTrend = lazy(() =>
  import('./MetricTrend').then((module) => ({ default: module.MetricTrend })),
);
const sourceOptions = (sourceId?: string) => (sourceId === undefined ? {} : { sourceId });
export function ProductOverviewPage() {
  const api = useProductApi();
  const { selectSource, selectedSource } = useProductSource();
  const { formatDate, formatNumber, t } = useLocale();
  const stream = useProductStream('metrics', selectedSource);
  const view = useExplorationView();
  const from = view.read('from', 30);
  const to = view.read('to', 30);
  const selectedMinutes = Number(view.read('minutes'));
  const minutes = selectedMinutes === 360 || selectedMinutes === 1440 ? selectedMinutes : 60;
  const absoluteRange = useMemo(() => {
    if (from === '' || to === '') return undefined;
    try {
      return timeQuery({ from, to }, 1);
    } catch {
      return undefined;
    }
  }, [from, to]);
  const sourceClock = useRef<number | null>(null);
  const [revision, setRevision] = useState(0);
  const activeRead = useRef(false);
  const refreshPending = useRef(false);
  const refreshTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const streamRevision = useRef(stream.revision);
  const firstSnapshot = useRef(true);
  const queueRefresh = useCallback(() => {
    if (refreshTimer.current !== null) return;
    refreshTimer.current = setTimeout(() => {
      refreshTimer.current = null;
      if (!refreshPending.current || activeRead.current) return;
      refreshPending.current = false;
      setRevision((current) => current + 1);
    }, 5000);
  }, []);
  useEffect(() => {
    sourceClock.current = null;
    firstSnapshot.current = true;
    refreshPending.current = false;
    return () => {
      if (refreshTimer.current !== null) clearTimeout(refreshTimer.current);
      refreshTimer.current = null;
      refreshPending.current = false;
    };
  }, [selectedSource]);
  useEffect(() => {
    if (stream.revision === streamRevision.current) return;
    const changes = stream.revision - streamRevision.current;
    streamRevision.current = stream.revision;
    const initial = firstSnapshot.current;
    firstSnapshot.current = false;
    // The initial snapshot describes the read already in progress. A replay
    // reset still invalidates it; later bursts share one bounded refresh.
    if (initial && changes === 1 && stream.status !== 'stale') return;
    refreshPending.current = true;
    queueRefresh();
  }, [stream.revision, stream.status, queueRefresh]);
  const selectTime = (value: number) => {
    const now = sourceClock.current ?? Date.now();
    view.write('minutes', String(value));
    view.write('from', new Date(now - value * 60000).toISOString());
    view.write('to', new Date(now).toISOString());
  };
  const [state, setState] = useState<
    LoadState<{
      readonly alerts: ProductAlertPage;
      readonly environments: ProductEnvironmentPage;
      readonly history: ProductMetricHistoryResult;
      readonly metrics: ProductMetricResult;
      readonly sources: readonly SourceSummary[];
      readonly events: ProductEventPage | null;
      readonly resources: ProductResourcePage | null;
    }>
  >({ kind: 'loading' });

  useEffect(() => {
    const controller = new AbortController();
    activeRead.current = true;
    setState({ kind: 'loading' });
    const read = async () => {
      const resources = api
        .listResources({ signal: controller.signal, ...sourceOptions(selectedSource), limit: 100 })
        .catch(() => null);
      const [sources, metrics, alerts, environments] = await Promise.all([
        api.listSources(controller.signal),
        api.listMetrics({ signal: controller.signal, ...sourceOptions(selectedSource) }),
        api.listAlerts({ signal: controller.signal, ...sourceOptions(selectedSource) }),
        api.listEnvironments({ signal: controller.signal, ...sourceOptions(selectedSource) }),
      ]);
      if (controller.signal.aborted) return;
      const source =
        selectedSource === undefined
          ? sources[0]
          : sources.find((item) => item.id === selectedSource);
      const observed = source?.synthetic === true ? Date.parse(source.observed_at) : Number.NaN;
      sourceClock.current = Number.isFinite(observed) ? observed + 1 : null;
      const now = sourceClock.current ?? Date.now();
      const range = absoluteRange ?? {
        from: new Date(now - minutes * 60000).toISOString(),
        to: new Date(now).toISOString(),
      };
      const [history, events, resourcePage] = await Promise.all([
        api.listMetricHistory({
          signal: controller.signal,
          ...sourceOptions(selectedSource),
          ...range,
        }),
        api
          .listEvents({
            signal: controller.signal,
            ...sourceOptions(selectedSource),
            ...range,
            limit: 10,
          })
          .catch(() => null),
        resources,
      ]);
      return { sources, metrics, alerts, environments, history, events, resources: resourcePage };
    };
    void read()
      .then(
        (data) => {
          if (data === undefined || controller.signal.aborted) return;
          setState({ kind: 'ready', data });
        },
        () => {
          if (controller.signal.aborted) return;
          setState({ kind: 'error' });
        },
      )
      .finally(() => {
        if (controller.signal.aborted) return;
        activeRead.current = false;
        if (refreshPending.current) queueRefresh();
      });
    return () => {
      controller.abort();
    };
  }, [api, selectedSource, revision, absoluteRange, minutes, queueRefresh]);

  return (
    <div className="route-page product-page">
      <header className="route-page__header">
        <p className="route-page__eyebrow">{t('route.overview.eyebrow')}</p>
        <h1>{t('route.overview.title')}</h1>
        <p>{t('route.overview.description')}</p>
        <ProductStreamBadge status={stream.status} />
      </header>
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
          <div className="exploration-filters">
            <TimeRangeControl minutes={minutes} select={selectTime} />
          </div>
          <Suspense fallback={<ProductState text={t('product.loading')} />}>
            <div className="trend-grid">
              <MetricTrend
                formatDate={formatDate}
                formatNumber={formatNumber}
                history={state.data.history}
                metric="cpu.usage.cores"
                title={t('product.trend.cpu')}
                unit={t('product.metric.cores')}
                t={t}
                minutes={minutes}
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
                minutes={minutes}
              />
              {(
                [
                  'restart.count',
                  'network.receive.bytes_total',
                  'network.send.bytes_total',
                ] as const
              ).map((metric) => (
                <MetricTrend
                  key={metric}
                  formatDate={formatDate}
                  formatNumber={formatNumber}
                  history={state.data.history}
                  metric={metric}
                  title={t(
                    metric === 'restart.count'
                      ? 'insight.restarts'
                      : metric === 'network.receive.bytes_total'
                        ? 'insight.receive'
                        : 'insight.send',
                  )}
                  unit={t(metric === 'restart.count' ? 'insight.count' : 'insight.bytes')}
                  t={t}
                  minutes={minutes}
                />
              ))}
            </div>
          </Suspense>
          <OverviewInsights
            environments={state.data.environments}
            events={state.data.events}
            metrics={state.data.metrics}
            resources={state.data.resources}
            {...(selectedSource === undefined ? {} : { source: selectedSource })}
          />
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
                <p>
                  {dataAllowed(source.availability)
                    ? t('product.resourcesCount', { count: source.resource_count })
                    : t('explore.count.unavailable')}
                </p>
                {source.completeness === 'partial' ? (
                  <p role="status">{t('explore.count.partial')}</p>
                ) : null}
                {dataAllowed(source.availability) ? (
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
                ) : null}
              </article>
            ))}
          </div>
        </>
      ) : null}
    </div>
  );
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

function MetricSummary({
  alerts,
  environments,
  formatNumber,
  metrics,
  t,
}: {
  readonly alerts: ProductAlertPage;
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
      value: `${alerts.page.has_more ? '≥ ' : ''}${formatNumber(alerts.items.filter((alert) => alert.state !== 'resolved').length)}`,
    },
  ];
  return (
    <section className="metric-summary" aria-label={t('product.metric.label')}>
      {metrics.availability === 'stale' ||
      metrics.completeness === 'partial' ||
      environments.availability === 'stale' ||
      environments.completeness === 'partial' ||
      environments.page.has_more ||
      alerts.page.has_more ? (
        <p role="status">
          {metrics.availability === 'stale' || environments.availability === 'stale'
            ? t('product.source.stale')
            : null}{' '}
          {metrics.completeness === 'partial' ||
          environments.completeness === 'partial' ||
          environments.page.has_more ||
          alerts.page.has_more
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
