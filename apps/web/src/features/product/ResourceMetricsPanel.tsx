import type {
  ProductHealthHistoryResult,
  ProductMetricHistoryResult,
} from '@stackplore/api-client';
import { Button, TechnicalText } from '@stackplore/ui';
import { lazy, Suspense, useEffect, useState } from 'react';
import { useLocale } from '../../i18n';
import { useProductApi } from './product-context';
import { ProductState, ObservationNotice, dataAllowed } from './exploration-ui';
import { TimeRangeControl, useExplorationTime } from './exploration-time';
const MetricTrend = lazy(() =>
  import('./MetricTrend').then((module) => ({ default: module.MetricTrend })),
);

export function ResourceMetricsPanel({
  resourceId,
  sourceId,
}: {
  readonly resourceId: string;
  readonly sourceId: string;
}) {
  const api = useProductApi();
  const { t, formatDate, formatNumber } = useLocale();
  const time = useExplorationTime();
  const [revision, setRevision] = useState(0);
  const [state, setState] = useState<{
    metrics: ProductMetricHistoryResult | null;
    health: ProductHealthHistoryResult | null;
  } | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    setState(null);
    void Promise.allSettled([
      api.listMetricHistory({ resourceId, sourceId, ...time.range, signal: controller.signal }),
      api.listHealthHistory({ resourceId, sourceId, ...time.range, signal: controller.signal }),
    ]).then(([metrics, health]) => {
      if (controller.signal.aborted) return;
      setState({
        metrics: metrics.status === 'fulfilled' ? metrics.value : null,
        health: health.status === 'fulfilled' ? health.value : null,
      });
    });
    return () => {
      controller.abort();
    };
  }, [api, resourceId, sourceId, time.range, revision]);
  const metrics = state?.metrics ?? null;
  return (
    <section aria-labelledby="resource-history-title">
      <h2 id="resource-history-title">{t('insight.resourceHistory')}</h2>
      <TimeRangeControl minutes={time.minutes} select={time.select} />
      {state === null ? (
        <ProductState text={t('product.loading')} />
      ) : (
        <>
          {state.metrics === null || state.health === null ? (
            <Button
              onClick={() => {
                setRevision((current) => current + 1);
              }}
            >
              {t('explore.retry')}
            </Button>
          ) : null}
          {metrics === null ? (
            <p role="status">{t('product.source.unavailable')}</p>
          ) : (
            <Suspense fallback={<ProductState text={t('product.loading')} />}>
              <div className="trend-grid">
                {(
                  [
                    'cpu.usage.cores',
                    'memory.usage.bytes',
                    'restart.count',
                    'network.receive.bytes_total',
                    'network.send.bytes_total',
                  ] as const
                ).map((metric) => (
                  <MetricTrend
                    key={metric}
                    formatDate={formatDate}
                    formatNumber={formatNumber}
                    history={metrics}
                    metric={metric}
                    title={t(
                      metric === 'cpu.usage.cores'
                        ? 'product.trend.cpu'
                        : metric === 'memory.usage.bytes'
                          ? 'product.trend.memory'
                          : metric === 'restart.count'
                            ? 'insight.restarts'
                            : metric === 'network.receive.bytes_total'
                              ? 'insight.receive'
                              : 'insight.send',
                    )}
                    unit={t(
                      metric === 'cpu.usage.cores'
                        ? 'product.metric.cores'
                        : metric === 'restart.count'
                          ? 'insight.count'
                          : 'insight.bytes',
                    )}
                    t={t}
                    minutes={time.minutes}
                  />
                ))}
              </div>
            </Suspense>
          )}
          <h3>{t('insight.healthHistory')}</h3>
          {state.health === null ? (
            <p role="status">{t('product.source.unavailable')}</p>
          ) : (
            <>
              <ObservationNotice data={state.health} />
              {dataAllowed(state.health.availability) ? (
                state.health.items.length === 0 ? (
                  <p>{t('insight.empty')}</p>
                ) : (
                  <div
                    className="trend-table-wrap ui-focus-ring"
                    tabIndex={0}
                    role="region"
                    aria-label={t('insight.healthHistory')}
                  >
                    <table>
                      <caption>{t('insight.healthHistory')}</caption>
                      <thead>
                        <tr>
                          <th scope="col">{t('insight.time')}</th>
                          <th scope="col">{t('insight.health')}</th>
                          <th scope="col">{t('insight.availability')}</th>
                          <th scope="col">{t('insight.reason')}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {state.health.items.map((item) => (
                          <tr key={item.id}>
                            <td>
                              <time dateTime={item.observed_at}>
                                {formatDate(Date.parse(item.observed_at), {
                                  dateStyle: 'short',
                                  timeStyle: 'short',
                                })}
                              </time>
                            </td>
                            <td>{t(`product.health.${item.state}`)}</td>
                            <td>
                              {t(
                                `product.source.${item.availability === 'permission_denied' ? 'permissionDenied' : item.availability}`,
                              )}
                            </td>
                            <td>
                              <TechnicalText>{item.reason_code ?? '—'}</TechnicalText>
                              {item.completeness === 'partial' ? (
                                <span>{t('product.observation.partial')}</span>
                              ) : null}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )
              ) : null}
            </>
          )}
        </>
      )}
    </section>
  );
}
