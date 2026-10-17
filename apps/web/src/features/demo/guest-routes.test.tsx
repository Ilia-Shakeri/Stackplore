import type { ApiClient } from '@stackplore/api-client';
import { createMemoryHistory } from '@tanstack/react-router';
import { act, render, screen, waitFor } from '@testing-library/react';
import { beforeAll, describe, expect, it, vi } from 'vitest';
import { App } from '../../app/App';
import { createAppRouter } from '../../app/router';
import { createAuthApi } from '../auth/auth-api';
import { AuthStore } from '../auth/auth-store';
import { createProductApi } from '../product/product-api';
import { createTestUsersApi } from '../../test/users';

beforeAll(async () => {
  await Promise.all([
    import('../product/ExplorationResourcesPage'),
    import('../product/ProductPages'),
    import('../product/ExplorationAlertsPage'),
  ]);
});
const observedAt = '2026-08-16T08:00:00Z';
const metadata = { availability: 'available', completeness: 'complete', observed_at: observedAt };
const resourceId = `r1_${'A'.repeat(43)}`;

async function renderGuest(path: string) {
  const request = vi.fn<ApiClient['request']>().mockImplementation((input) => {
    let data: unknown;
    if (input.path === '/setup/status')
      data = {
        state: 'guest_demo',
        fixture_version: 'demo-v2',
        scenario_step: 0,
        observed_at: observedAt,
      };
    else if (input.path === '/sources')
      data = [
        {
          ...metadata,
          id: 'source_demo',
          name: 'Demo',
          provider: 'demo',
          synthetic: true,
          fixture_version: 'demo-v2',
          resource_count: 9,
          health: { healthy: 7, degraded: 1, unhealthy: 1, unknown: 0 },
        },
      ];
    else if (input.path === '/resources')
      data = {
        ...metadata,
        items: [
          {
            id: resourceId,
            provider: 'demo',
            source_id: 'source_demo',
            environment_id: 'env_demo_shop',
            kind: 'demo.service',
            scope_kind: 'project',
            scope_name: 'demo-shop',
            name: 'API',
            health: 'healthy',
            synthetic: true,
            observed_at: observedAt,
          },
        ],
        page: { has_more: false, limit: 100 },
      };
    else if (input.path === '/metrics/current' || input.path === '/metrics/history')
      data = { ...metadata, items: [] };
    else if (input.path === '/alerts')
      data = {
        items: [
          {
            id: 'alert_demo_checkout',
            rule_id: 'rule_demo_checkout_health',
            severity: 'error',
            state: 'firing',
            summary_code: 'demo.alert.checkout_unhealthy',
            synthetic: true,
            started_at: observedAt,
            updated_at: observedAt,
          },
        ],
        page: { has_more: false, limit: 100 },
      };
    else data = { ...metadata, items: [], page: { has_more: false, limit: 100 } };
    return Promise.resolve({ data, status: 200, requestId: 'request_guest_route' });
  });
  const client = { request: request as ApiClient['request'] };
  const history = createMemoryHistory({ initialEntries: [path] });
  const store = new AuthStore(createAuthApi(client));
  const router = createAppRouter(
    history,
    store,
    createTestUsersApi(),
    createProductApi(client, store),
  );
  await act(async () => {
    render(<App router={router} />);
    await store.ensureLoaded();
    await router.load();
  });
  return { request, history };
}

describe('explicit guest deployment routes', () => {
  it('opens inventory without a login, session cookie context or administrator actions', async () => {
    const { request } = await renderGuest('/resources?source=source_demo');
    expect(await screen.findByRole('link', { name: /API/u })).not.toBeNull();
    expect(screen.getByRole('complementary', { name: 'Demo mode' })).not.toBeNull();
    expect(screen.getByText('Guest')).not.toBeNull();
    expect(screen.queryByRole('button', { name: 'Open user menu' })).toBeNull();
    expect(screen.queryByRole('link', { name: 'Audit' })).toBeNull();
    expect(request.mock.calls.some(([input]) => input.path === '/auth/session')).toBe(false);
    expect(
      request.mock.calls.every(([input]) => input.method === undefined || input.method === 'GET'),
    ).toBe(true);
  });
  it.each(['/login', '/setup', '/settings/users', '/audit'])(
    'keeps %s outside guest administration',
    async (path) => {
      const { history, request } = await renderGuest(path);
      await waitFor(() => {
        expect(history.location.pathname).toBe('/overview');
      });
      expect(await screen.findByRole('complementary', { name: 'Demo mode' })).not.toBeNull();
      expect(
        request.mock.calls.some(
          ([input]) =>
            input.path === '/auth/session' || input.path === '/users' || input.path === '/audit',
        ),
      ).toBe(false);
    },
  );
  it('shows firing alert metadata without an acknowledgement action', async () => {
    const { request } = await renderGuest('/alerts?source=source_demo');
    expect(await screen.findByText('Checkout health check is failing')).not.toBeNull();
    expect(screen.queryByRole('button', { name: 'Acknowledge' })).toBeNull();
    expect(
      request.mock.calls.every(([input]) => input.method === undefined || input.method === 'GET'),
    ).toBe(true);
  });
});
