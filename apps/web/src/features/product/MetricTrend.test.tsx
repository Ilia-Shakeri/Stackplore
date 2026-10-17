import type { ProductMetricHistoryResult, ProductMetricSample } from '@stackplore/api-client';
import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { MetricTrend } from './MetricTrend';
import { LocaleProvider, useLocale } from '../../i18n';

const resource = `r1_${'A'.repeat(43)}`;
const secondResource = `r1_${'B'.repeat(43)}`;
const sample = (minute: number, value: number, resourceId = resource): ProductMetricSample => ({
  id: `sample_${String(minute)}_${resourceId}`,
  resource_id: resourceId,
  name: 'cpu.usage.cores',
  unit: 'cores',
  value,
  sampled_at: new Date(Date.UTC(2026, 7, 16, 8, minute)).toISOString(),
  interval_seconds: 60,
  synthetic: true,
});
const history = (items: ProductMetricSample[]): ProductMetricHistoryResult => ({
  availability: 'available',
  completeness: 'complete',
  observed_at: '2026-08-16T09:00:00Z',
  items,
});
function Harness({ data }: { readonly data: ProductMetricHistoryResult }) {
  const locale = useLocale();
  return (
    <MetricTrend
      history={data}
      metric="cpu.usage.cores"
      title="CPU"
      unit="cores"
      minutes={60}
      {...locale}
    />
  );
}
function show(data: ProductMetricHistoryResult) {
  return render(
    <LocaleProvider defaultLocale="en" storageKey="metric-trend-test">
      <Harness data={data} />
    </LocaleProvider>,
  );
}

describe('metric trend evidence', () => {
  it('spaces uneven observations by actual elapsed time', () => {
    const { container } = show(history([sample(0, 1), sample(1, 2), sample(10, 3)]));
    const points = container
      .querySelector('polyline')
      ?.getAttribute('points')
      ?.split(' ')
      .map((point) => Number(point.split(',')[0]));
    expect(points).toEqual([0, 32, 320]);
    expect(screen.getAllByText('3 cores')).toHaveLength(2);
  });
  it('keeps resources separate at both aligned and different observation times', () => {
    const { container } = show(
      history([
        sample(0, 2),
        sample(0, 5, secondResource),
        sample(10, 3),
        sample(11, 7, secondResource),
      ]),
    );
    expect(container.querySelectorAll('polyline')).toHaveLength(2);
    expect(screen.getByText('2 resource series')).not.toBeNull();
    expect(screen.queryByText('10 cores')).toBeNull();
    const table = screen.getByRole('table', { hidden: true });
    expect(within(table).getAllByRole('row', { hidden: true })).toHaveLength(5);
    expect(within(table).getAllByText(resource)).toHaveLength(2);
    expect(within(table).getAllByText(secondResource)).toHaveLength(2);
  });
  it('does not plot denied data and keeps partial evidence visibly distinct', () => {
    const { container, rerender } = show({
      ...history([]),
      availability: 'permission_denied',
      reason_code: 'provider.denied',
    });
    expect(container.querySelector('polyline')).toBeNull();
    expect(screen.getByText('Access denied')).not.toBeNull();
    rerender(
      <LocaleProvider defaultLocale="en" storageKey="metric-trend-test">
        <Harness
          data={{
            ...history([sample(0, 1)]),
            completeness: 'partial',
            reason_code: 'metric.partial',
          }}
        />
      </LocaleProvider>,
    );
    expect(screen.getByText('Partial observation')).not.toBeNull();
    expect(container.querySelector('circle')).not.toBeNull();
  });
});
