import { render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { LocaleProvider } from '../../i18n';
import { ProductProvider } from './product-context';
import { createProductApi } from './product-api';
import { ResourceMetricsPanel } from './ResourceMetricsPanel';
const resource = `r1_${'A'.repeat(43)}`;
const at = '2026-10-17T08:00:00Z';
describe('resource metrics and retained health', () => {
  it('keeps a semantic health table when optional metric history fails', async () => {
    const api = {
      ...createProductApi({ request: () => Promise.reject(new Error('unexpected')) }),
      listSources: vi.fn().mockResolvedValue([]),
      listMetricHistory: vi.fn().mockRejectedValue(new Error('unavailable')),
      listHealthHistory: vi.fn().mockResolvedValue({
        availability: 'available',
        completeness: 'partial',
        observed_at: at,
        reason_code: 'source.health_history_limited',
        items: [
          {
            id: 'health_one',
            resource_id: resource,
            state: 'unknown',
            availability: 'available',
            completeness: 'partial',
            reason_code: 'source.partial',
            observed_at: at,
            synthetic: true,
          },
        ],
      }),
    };
    render(
      <LocaleProvider defaultLocale="en" storageKey="resource-history-test">
        <ProductProvider api={api} sourceId="source_demo">
          <ResourceMetricsPanel resourceId={resource} sourceId="source_demo" />
        </ProductProvider>
      </LocaleProvider>,
    );
    const table = await screen.findByRole('table', { name: 'Retained health observations' });
    expect(within(table).getAllByRole('row')).toHaveLength(2);
    expect(within(table).getByText('Unknown')).not.toBeNull();
    expect(screen.getByText('Unavailable')).not.toBeNull();
    expect(api.listHealthHistory.mock.calls[0]?.[0]).toMatchObject({
      resourceId: resource,
      sourceId: 'source_demo',
    });
  });
});
