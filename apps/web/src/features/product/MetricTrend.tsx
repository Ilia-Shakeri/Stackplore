import type { ProductMetricHistoryResult } from '@stackplore/api-client';
import { TechnicalText } from '@stackplore/ui';
import { useMemo } from 'react';
import type { useLocale } from '../../i18n';
import { dataAllowed, availabilityLabel } from './exploration-ui';

interface TrendPoint {
  readonly time: number;
  readonly timestamp: string;
  readonly value: number;
  readonly resourceId: string;
  readonly id: string;
}

export function MetricTrend({
  divisor = 1,
  formatDate,
  formatNumber,
  history,
  metric,
  t,
  title,
  unit,
  minutes,
}: {
  readonly divisor?: number;
  readonly formatDate: ReturnType<typeof useLocale>['formatDate'];
  readonly formatNumber: ReturnType<typeof useLocale>['formatNumber'];
  readonly history: ProductMetricHistoryResult;
  readonly metric: ProductMetricHistoryResult['items'][number]['name'];
  readonly t: ReturnType<typeof useLocale>['t'];
  readonly title: string;
  readonly unit: string;
  readonly minutes: number;
}) {
  const points = useMemo(
    () =>
      (dataAllowed(history.availability) ? history.items : [])
        .filter((sample) => sample.name === metric)
        .map((sample): TrendPoint => ({
          time: Date.parse(sample.sampled_at),
          timestamp: sample.sampled_at,
          value: sample.value / divisor,
          resourceId: sample.resource_id,
          id: sample.id,
        }))
        .sort(
          (left, right) =>
            left.time - right.time ||
            left.resourceId.localeCompare(right.resourceId) ||
            left.id.localeCompare(right.id),
        ),
    [divisor, history, metric],
  );
  const byResource = new Map<string, TrendPoint[]>();
  for (const point of points) {
    const series = byResource.get(point.resourceId) ?? [];
    series.push(point);
    byResource.set(point.resourceId, series);
  }
  const start = points.at(0)?.time ?? 0;
  const end = points.at(-1)?.time ?? start;
  const maximum = Math.max(1, ...points.map((point) => point.value));
  const x = (time: number) => (end === start ? 160 : ((time - start) / (end - start)) * 320);
  const y = (value: number) => 88 - (value / maximum) * 80;
  const latest = points.at(-1);
  return (
    <section className="trend-card">
      <header>
        <div>
          <h2>{title}</h2>
          <p>
            {dataAllowed(history.availability)
              ? t(
                  minutes === 1440
                    ? 'explore.range.day'
                    : minutes === 360
                      ? 'explore.range.sixHours'
                      : 'explore.range.hour',
                )
              : availabilityLabel(history.availability, t)}
          </p>
          {history.availability === 'stale' ? <p>{t('product.source.stale')}</p> : null}
          {history.completeness === 'partial' ? <p>{t('product.observation.partial')}</p> : null}
        </div>
        <strong>
          {latest === undefined
            ? t('product.trend.empty')
            : byResource.size === 1
              ? `${formatNumber(latest.value, { maximumFractionDigits: 2 })} ${unit}`
              : t('trend.seriesCount', { count: byResource.size })}
        </strong>
      </header>
      {points.length === 0 ? null : (
        <>
          <p>{t('trend.perResource')}</p>
          <svg viewBox="0 0 320 96" role="img" aria-label={title} preserveAspectRatio="none">
            <title>{title}</title>
            <line x1="0" x2="320" y1="88" y2="88" />
            {[...byResource].map(([resourceId, series]) => (
              <g key={resourceId} data-resource-id={resourceId}>
                <title>{resourceId}</title>
                <polyline
                  points={series
                    .map((point) => `${x(point.time).toFixed(2)},${y(point.value).toFixed(2)}`)
                    .join(' ')}
                />
                {series.length === 1 ? (
                  <circle
                    cx={x(series[0]?.time ?? start)}
                    cy={y(series[0]?.value ?? 0)}
                    r="2"
                    fill="currentColor"
                  />
                ) : null}
              </g>
            ))}
          </svg>
        </>
      )}
      <details>
        <summary>{t('product.trend.table')}</summary>
        <div className="trend-table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col">{t('trend.resource')}</th>
                <th scope="col">{t('product.trend.time')}</th>
                <th scope="col">{t('product.trend.value')}</th>
              </tr>
            </thead>
            <tbody>
              {points.map((point) => (
                <tr key={point.id}>
                  <td>
                    <TechnicalText>{point.resourceId}</TechnicalText>
                  </td>
                  <td>
                    <time dateTime={point.timestamp}>
                      {formatDate(point.time, { dateStyle: 'short', timeStyle: 'medium' })}
                    </time>
                  </td>
                  <td>{`${formatNumber(point.value, { maximumFractionDigits: 2 })} ${unit}`}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
    </section>
  );
}
