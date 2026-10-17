import type {
  ProductEnvironmentPage,
  ProductMetricResult,
  ProductResourcePage,
} from '@stackplore/api-client';
import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { LocaleProvider } from '../../i18n';
import { OverviewInsights } from './OverviewInsights';
const resource = `r1_${'A'.repeat(43)}`;
const at = '2026-10-17T08:00:00Z';
describe('overview evidence panels', () => {
  it('localizes timeline severity for Persian readers', () => {
    render(
      <LocaleProvider defaultLocale="fa" storageKey="overview-insights-fa-test">
        <OverviewInsights
          environments={{
            availability: 'available',
            completeness: 'complete',
            observed_at: at,
            items: [],
            page: { limit: 10, has_more: false },
          }}
          events={{
            availability: 'available',
            completeness: 'complete',
            observed_at: at,
            items: [
              {
                id: 'event_overview',
                kind: 'system.alert',
                severity: 'critical',
                reason_code: 'alert.resource_unhealthy',
                summary: 'Synthetic event',
                occurred_at: at,
                observed_at: at,
                synthetic: true,
              },
            ],
            page: { limit: 10, has_more: false },
          }}
          metrics={{
            availability: 'available',
            completeness: 'complete',
            observed_at: at,
            items: [],
          }}
          resources={null}
        />
      </LocaleProvider>,
    );
    expect(screen.getByText('بحرانی')).not.toBeNull();
    expect(screen.queryByText('critical')).toBeNull();
  });

  it('shows bounded rankings, environment navigation, unavailable optional reads and exact resource links', () => {
    const environments: ProductEnvironmentPage = {
      availability: 'available',
      completeness: 'complete',
      observed_at: at,
      items: [
        {
          id: 'env_shop',
          source_id: 'source_demo',
          provider: 'demo',
          name: 'Shop',
          availability: 'available',
          completeness: 'complete',
          observed_at: at,
          synthetic: true,
        },
      ],
      page: { limit: 100, has_more: true, next_cursor: 'next' },
    };
    const metrics: ProductMetricResult = {
      availability: 'available',
      completeness: 'complete',
      observed_at: at,
      items: [
        {
          id: 'metric_one',
          resource_id: resource,
          name: 'cpu.usage.cores',
          unit: 'cores',
          value: 2,
          sampled_at: at,
          interval_seconds: 60,
          synthetic: true,
        },
      ],
    };
    const resources: ProductResourcePage = {
      availability: 'available',
      completeness: 'complete',
      observed_at: at,
      items: [
        {
          id: resource,
          source_id: 'source_demo',
          provider: 'demo',
          environment_id: 'env_shop',
          kind: 'demo.gateway',
          name: 'Gateway',
          scope_kind: 'project',
          scope_name: 'shop',
          health: 'healthy',
          observed_at: at,
          synthetic: true,
        },
      ],
      page: { limit: 100, has_more: false },
    };
    render(
      <LocaleProvider defaultLocale="en" storageKey="overview-insights-test">
        <OverviewInsights
          environments={environments}
          events={null}
          metrics={metrics}
          resources={resources}
          source="source_demo"
        />
      </LocaleProvider>,
    );
    expect(screen.getByRole('heading', { name: 'Top loaded consumers' })).not.toBeNull();
    expect(screen.getByText(/Partial reads cannot prove/)).not.toBeNull();
    expect(screen.getByRole('link', { name: 'Gateway' }).getAttribute('href')).toBe(
      `/resources/source_demo/demo/demo.gateway/${resource}?source=source_demo`,
    );
    expect(screen.getByRole('heading', { name: 'Shop' })).not.toBeNull();
    expect(
      screen
        .getAllByRole('link', { name: 'Resources' })
        .some((link) => link.getAttribute('href')?.includes('environment_id=env_shop')),
    ).toBe(true);
    expect(screen.getByText('Unavailable')).not.toBeNull();
  });
});
