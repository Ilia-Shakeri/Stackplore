import type { ApiClient } from '@stackplore/api-client';
import { createMemoryHistory } from '@tanstack/react-router';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeAll, describe, expect, it, vi } from 'vitest';

import { createProductApi } from './product-api';
import { App } from '../../app/App';
import { createAppRouter } from '../../app/router';
import { createAuthenticatedAuthStore } from '../../test/auth';
import { createTestUsersApi } from '../../test/users';

beforeAll(async () => {
  await Promise.all([import('./ExplorationResourcesPage'), import('./ExplorationLogsPage')]);
});

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
  it.each(['trace', 'unknown'])('keeps log severity %s in links and history', async (severity) => {
    const user = userEvent.setup();
    const { history } = renderProductRoute('/logs?source=source_docker&tail=paused');
    const field = await screen.findByRole<HTMLSelectElement>('combobox', { name: 'Severity' });
    await user.selectOptions(field, severity);
    await waitFor(() => {
      expect(new URLSearchParams(history.location.search).get('severity')).toBe(severity);
    });
    act(() => {
      history.push('/logs?source=source_docker&tail=paused&severity=warning');
    });
    await waitFor(() => {
      expect(field.value).toBe('warning');
    });
    act(() => {
      history.back();
    });
    await waitFor(() => {
      expect(field.value).toBe(severity);
    });
  });
  it('preserves batched tail and time parameters in the real route history', async () => {
    const user = userEvent.setup();
    const { history } = renderProductRoute(`/logs?source=source_docker&resource_id=${dockerId}`);
    await user.click(await screen.findByRole('button', { name: 'Pause live tail' }));
    await waitFor(() => {
      const params = new URLSearchParams(history.location.search);
      expect(params.get('tail')).toBe('paused');
      expect(params.get('source')).toBe('source_docker');
      expect(params.get('resource_id')).toBe(dockerId);
      expect(Number.isFinite(Date.parse(params.get('from') ?? ''))).toBe(true);
      expect(Number.isFinite(Date.parse(params.get('to') ?? ''))).toBe(true);
    });
    await user.selectOptions(screen.getByRole('combobox', { name: 'Time range' }), '360');
    await waitFor(() => {
      const params = new URLSearchParams(history.location.search);
      expect(params.get('minutes')?.replaceAll('"', '')).toBe('360');
      expect(params.get('tail')).toBe('paused');
      expect(Date.parse(params.get('to') ?? '') - Date.parse(params.get('from') ?? '')).toBe(
        360 * 60000,
      );
    });
    await user.click(screen.getByRole('button', { name: 'Resume live tail' }));
    await waitFor(() => {
      const params = new URLSearchParams(history.location.search);
      expect(params.get('tail')).toBe('live');
      expect(params.has('from')).toBe(false);
      expect(params.has('to')).toBe(false);
      expect(params.get('resource_id')).toBe(dockerId);
    });
  });
  it('loads validated inventory filters from a deep link and restores them through history', async () => {
    const user = userEvent.setup();
    const { history, request } = renderProductRoute(
      '/resources?source=source_docker&provider=docker&health=unknown&availability=available&kind=docker.container&scope_kind=project&scope_name=shop&environment_id=env_test&sort=observed_at&direction=desc',
    );
    expect(await screen.findByRole('link', { name: /Worker/u })).not.toBeNull();
    const latestResourceQuery = () =>
      request.mock.calls.filter(([input]) => input.path === '/resources').at(-1)?.[0].query;
    expect(latestResourceQuery()).toMatchObject({
      source_id: 'source_docker',
      provider: 'docker',
      health: 'unknown',
      availability: 'available',
      kind: 'docker.container',
      scope_kind: 'project',
      scope_name: 'shop',
      environment_id: 'env_test',
      sort: 'observed_at',
      direction: 'desc',
    });
    await user.selectOptions(screen.getByRole('combobox', { name: 'Health' }), 'healthy');
    await waitFor(() => {
      expect(new URLSearchParams(history.location.search).get('health')).toBe('healthy');
      expect(latestResourceQuery()?.health).toBe('healthy');
    });
    act(() => {
      history.push('/resources?source=source_demo&health=degraded&sort=kind');
    });
    await waitFor(() => {
      expect(screen.getByRole<HTMLSelectElement>('combobox', { name: 'Health' }).value).toBe(
        'degraded',
      );
      expect(latestResourceQuery()).toMatchObject({
        health: 'degraded',
        sort: 'kind',
        source_id: 'source_demo',
      });
    });
    act(() => {
      history.back();
    });
    await waitFor(() => {
      expect(screen.getByRole<HTMLSelectElement>('combobox', { name: 'Health' }).value).toBe(
        'healthy',
      );
      expect(latestResourceQuery()).toMatchObject({
        health: 'healthy',
        sort: 'observed_at',
        source_id: 'source_docker',
      });
    });
  });

  it('keeps resource, container and exact time bounds in a log deep link', async () => {
    const from = '2026-08-16T07:00:00Z';
    const to = '2026-08-16T08:00:00Z';
    const { request } = renderProductRoute(
      `/logs?source=source_docker&resource_id=${dockerId}&container=worker&previous=true&from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
    );
    expect(await screen.findByRole('heading', { name: 'Logs', level: 1 })).not.toBeNull();
    await waitFor(() => {
      expect(request.mock.calls.find(([input]) => input.path === '/logs')?.[0].query).toMatchObject(
        {
          resource_id: dockerId,
          source_id: 'source_docker',
          container: 'worker',
          previous: true,
          from,
          to,
        },
      );
    });
    expect(screen.getByRole<HTMLInputElement>('combobox', { name: 'Container name' }).value).toBe(
      'worker',
    );
    expect(
      screen.getByRole<HTMLInputElement>('checkbox', { name: 'Previous container' }).checked,
    ).toBe(true);
  });

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
