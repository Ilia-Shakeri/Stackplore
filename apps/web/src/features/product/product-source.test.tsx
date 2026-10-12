import type { ApiClient } from '@stackplore/api-client';
import { createMemoryHistory } from '@tanstack/react-router';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { createProductApi } from './product-api';
import { App } from '../../app/App';
import { createAppRouter } from '../../app/router';
import { createAuthenticatedAuthStore } from '../../test/auth';
import { createTestUsersApi } from '../../test/users';

const observedAt = '2026-08-16T08:00:00Z';
const dockerId = `r1_${'B'.repeat(43)}`;
const metadata = { availability: 'available', completeness: 'complete', observed_at: observedAt };
type TestRequest = (input: Parameters<ApiClient['request']>[0]) => Promise<{
  readonly data: unknown;
  readonly status: number;
  readonly requestId: string;
}>;

function renderProductRoute(path: string) {
  const sources = ['source_demo', 'source_docker'].map((id) => ({
    ...metadata,
    id,
    name: id === 'source_demo' ? 'Demo source' : 'Docker source',
    provider: id === 'source_demo' ? 'demo' : 'docker',
    synthetic: id === 'source_demo',
    resource_count: 1,
    health: { healthy: 1, degraded: 0, unhealthy: 0, unknown: 0 },
  }));
  const request = vi.fn<TestRequest>().mockImplementation((input) => {
    const sourceId = input.query?.source_id ?? 'source_demo';
    const docker = sourceId === 'source_docker';
    const resource = {
      id: docker ? dockerId : `r1_${'A'.repeat(43)}`,
      source_id: sourceId,
      environment_id: 'env_test',
      provider: docker ? 'docker' : 'demo',
      kind: docker ? 'docker.container' : 'demo.gateway',
      scope_kind: 'project',
      name: docker ? 'Worker' : 'Gateway',
      synthetic: !docker,
      observed_at: observedAt,
    };
    let data: unknown;
    if (input.path === '/sources') data = sources;
    else if (input.path === '/resources')
      data = { ...metadata, items: [resource], page: { has_more: false, limit: 50 } };
    else if (input.path.endsWith('/health'))
      data = {
        resource_id: resource.id,
        state: 'healthy',
        observed_at: observedAt,
        reason_codes: [],
        synthetic: !docker,
      };
    else if (input.path.startsWith('/resources/')) data = resource;
    else data = { ...metadata, items: [], page: { has_more: false, limit: 100 } };
    return Promise.resolve({ data, status: 200, requestId: 'request_source_test' });
  });
  const history = createMemoryHistory({ initialEntries: [path] });
  const router = createAppRouter(
    history,
    createAuthenticatedAuthStore(),
    createTestUsersApi(),
    createProductApi({ request: request as ApiClient['request'] }),
  );
  render(<App router={router} />);
  return { history, request, router };
}

describe('source-bound product routes', () => {
  it('switches source before each request and follows URL history', async () => {
    const user = userEvent.setup();
    const { history, request } = renderProductRoute('/resources?source=source_demo');
    expect(await screen.findByRole('link', { name: /Gateway/u })).not.toBeNull();
    await user.selectOptions(
      screen.getByRole('combobox', { name: 'Infrastructure source' }),
      'source_docker',
    );
    const worker = await screen.findByRole('link', { name: /Worker/u });
    expect(worker.getAttribute('href')).toBe(
      `/resources/source_docker/docker/docker.container/${dockerId}?source=source_docker`,
    );
    expect(history.location.search).toBe('?source=source_docker');
    expect(
      request.mock.calls.filter(([input]) => input.path === '/resources').at(-1)?.[0].query
        ?.source_id,
    ).toBe('source_docker');

    const query = screen.getByRole('textbox', { name: 'Search resources' });
    await user.type(query, 'worker');
    const form = query.closest('form');
    if (form === null) throw new Error('Resource search form is missing.');
    await user.click(within(form).getByRole('button', { name: 'Search' }));
    await waitFor(() => {
      expect(history.location.search).toBe('?source=source_docker&q=worker');
    });

    act(() => {
      history.push('/resources?source=source_demo&q=gateway');
    });
    expect(await screen.findByRole('link', { name: /Gateway/u })).not.toBeNull();
    expect(screen.getByRole<HTMLInputElement>('textbox', { name: 'Search resources' }).value).toBe(
      'gateway',
    );
    act(() => {
      history.back();
    });
    expect(await screen.findByRole('link', { name: /Worker/u })).not.toBeNull();
    expect(screen.getByRole<HTMLInputElement>('textbox', { name: 'Search resources' }).value).toBe(
      'worker',
    );
    await waitFor(() => {
      expect(
        screen.getByRole<HTMLSelectElement>('combobox', { name: 'Infrastructure source' }).value,
      ).toBe('source_docker');
    });
  });

  it('loads a direct resource route from its exact source and keeps that source in navigation', async () => {
    const user = userEvent.setup();
    const { request, history } = renderProductRoute(
      `/resources/source_docker/docker/docker.container/${dockerId}?source=source_demo`,
    );
    expect(await screen.findByRole('heading', { level: 2, name: 'Worker' })).not.toBeNull();
    for (const [input] of request.mock.calls.filter(([input]) =>
      input.path.startsWith('/resources/'),
    )) {
      expect(input.query?.source_id).toBe('source_docker');
    }
    await user.click(screen.getByRole('link', { name: 'Logs' }));
    expect(await screen.findByRole('heading', { level: 1, name: 'Logs' })).not.toBeNull();
    await waitFor(() => {
      expect(
        request.mock.calls.find(([input]) => input.path === '/logs')?.[0].query?.source_id,
      ).toBe('source_docker');
    });
    expect(history.location.search).toBe('?source=source_docker');
  });
});
