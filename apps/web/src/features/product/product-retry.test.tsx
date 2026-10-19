import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ProductAuditPage, ProductNotificationsPage } from './ProductHistoryPages';
import { ProductResourcesPage } from './ExplorationResourcesPage';
import { ProductEventsPage } from './ExplorationEventsPage';
import { ProductLogsPage } from './ExplorationLogsPage';
import { ProductTopologyPage } from './ExplorationTopologyPage';
import { ProductOverviewPage } from './ProductPages';
import { ProductAlertsPage } from './ExplorationAlertsPage';
import { ProductResourceDetailPage } from './ExplorationResourceDetailPage';
import { createProductApi, type ProductApi } from './product-api';
import { ProductProvider } from './product-context';
import { AuthProvider } from '../auth/auth-context';
import { LocaleProvider } from '../../i18n';
import { createAuthenticatedAuthStore } from '../../test/auth';

const at = '2026-08-16T08:00:00Z';
const resourceId = `r1_${'A'.repeat(43)}`;
const plainPage = { items: [], page: { has_more: false, limit: 100 } };
const observation = {
  availability: 'available' as const,
  completeness: 'complete' as const,
  observed_at: at,
  items: [],
};
const observedPage = { ...observation, ...plainPage };
const resource = {
  id: resourceId,
  source_id: 'source_demo',
  provider: 'demo' as const,
  kind: 'demo.gateway',
  environment_id: 'env_demo',
  scope_kind: 'project' as const,
  name: 'Gateway',
  synthetic: true,
  observed_at: at,
};
const cases = [
  { name: 'audit', Component: ProductAuditPage, method: 'listAudit', value: plainPage },
  {
    name: 'notifications',
    Component: ProductNotificationsPage,
    method: 'listNotifications',
    value: plainPage,
  },
  {
    name: 'resources',
    Component: ProductResourcesPage,
    method: 'listResources',
    value: observedPage,
  },
  { name: 'events', Component: ProductEventsPage, method: 'listEvents', value: observedPage },
  { name: 'logs', Component: ProductLogsPage, method: 'listLogs', value: observedPage },
  { name: 'topology', Component: ProductTopologyPage, method: 'listTopology', value: observedPage },
  { name: 'overview', Component: ProductOverviewPage, method: 'listSources', value: [] },
  { name: 'alerts', Component: ProductAlertsPage, method: 'listAlerts', value: plainPage },
  {
    name: 'resource detail',
    Component: () => (
      <ProductResourceDetailPage
        id={resourceId}
        source="source_demo"
        provider="demo"
        kind="demo.gateway"
      />
    ),
    method: 'getResource',
    value: resource,
  },
] as const;

describe('failed product reads', () => {
  it('filters topology resources by the URL environment before reading each page', async () => {
    const listResources = vi.fn().mockResolvedValue(observedPage);
    const listTopology = vi.fn().mockResolvedValue(observedPage);
    const api = {
      ...createProductApi({
        request: () => Promise.reject(new Error('Unexpected topology request.')),
      }),
      listResources,
      listTopology,
    };
    const subject = (environment: string) => (
      <LocaleProvider>
        <ProductProvider api={api} viewSearch={`environment_id=${environment}`}>
          <ProductTopologyPage />
        </ProductProvider>
      </LocaleProvider>
    );
    const view = render(subject('env_first'));
    await waitFor(() => {
      expect(listResources).toHaveBeenCalledWith(
        expect.objectContaining({ environmentId: 'env_first', limit: 100 }),
      );
    });
    const signal = listResources.mock.calls[0]?.[0] as { signal: AbortSignal } | undefined;
    view.rerender(subject('env_second'));
    await waitFor(() => {
      expect(listResources).toHaveBeenLastCalledWith(
        expect.objectContaining({ environmentId: 'env_second', limit: 100 }),
      );
    });
    expect(signal?.signal.aborted).toBe(true);
    expect(listTopology.mock.calls[0]?.[0]).not.toHaveProperty('environmentId');
  });
  it.each(cases)(
    'retries $name without a document reload',
    async ({ Component, method, value }) => {
      let failed = true;
      const load = vi
        .fn()
        .mockImplementation(() =>
          failed ? Promise.reject(new Error('Read unavailable.')) : Promise.resolve(value),
        );
      const api: ProductApi = {
        ...createProductApi({
          request: () => Promise.reject(new Error('Unexpected retry request.')),
        }),
        listSources: vi.fn().mockResolvedValue([]),
        listResources: vi.fn().mockResolvedValue(observedPage),
        listEnvironments: vi.fn().mockResolvedValue(observedPage),
        listEvents: vi.fn().mockResolvedValue(observedPage),
        listAlerts: vi.fn().mockResolvedValue(plainPage),
        listMetrics: vi.fn().mockResolvedValue(observation),
        listMetricHistory: vi.fn().mockResolvedValue(observation),
        listHealthHistory: vi.fn().mockResolvedValue(observation),
        getResourceHealth: vi.fn().mockRejectedValue(new Error('Health unavailable.')),
        [method]: load,
      };
      const user = userEvent.setup();
      render(
        <LocaleProvider>
          <AuthProvider store={createAuthenticatedAuthStore()}>
            <ProductProvider api={api}>
              <Component />
            </ProductProvider>
          </AuthProvider>
        </LocaleProvider>,
      );
      const retry = await screen.findByRole('button', { name: 'Try again' });
      const before = load.mock.calls.length;
      failed = false;
      await user.click(retry);
      await waitFor(() => {
        expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
      });
      expect(load.mock.calls.length).toBeGreaterThan(before);
    },
  );
});
