import type { ApiClientOptions } from '@stackplore/api-client';
import { createMemoryHistory } from '@tanstack/react-router';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { App } from '../App';
import { createBrowserApiClient } from '../api-client';
import { createAppRouter } from '../router';
import { createProductApi } from '../../features/product/product-api';
import { TopologyGraph } from '../../features/product/TopologyGraph';
import { productViewSearch } from '../../features/product/product-view-search';
import {
  environmentDestination,
  environmentNavigationSearch,
  readEnvironment,
} from '../../features/product/environment-navigation';
import { createAuthenticatedAuthStore } from '../../test/auth';
import { LocaleProvider } from '../../i18n';

const observedAt = '2026-08-16T08:00:00Z';
const observation = {
  availability: 'available',
  completeness: 'complete',
  observed_at: observedAt,
};
const resourceId = `r1_${'A'.repeat(43)}`;
const environment = (index: number, source = 'source_demo') => ({
  ...observation,
  id: `env_${String(index)}`,
  source_id: source,
  provider: 'demo',
  name: `Environment ${String(index)}`,
  synthetic: true,
});
const page = (items: readonly unknown[] = [], cursor?: string) => ({
  ...observation,
  items,
  page: {
    limit: 100,
    has_more: cursor !== undefined,
    ...(cursor === undefined ? {} : { next_cursor: cursor }),
  },
});
const json = (data: unknown) =>
  new Response(JSON.stringify(data), {
    headers: { 'Content-Type': 'application/json', 'X-Request-Id': 'req_environment_fixture' },
  });
type ReadEnvironments = (url: URL, signal?: AbortSignal | null) => Response | Promise<Response>;

function renderEnvironments(
  entry = '/about?source=source_demo',
  read: ReadEnvironments = () => json(page([environment(0), environment(1)])),
) {
  const requests: URL[] = [];
  const signals: (AbortSignal | null | undefined)[] = [];
  const fetch: NonNullable<ApiClientOptions['fetch']> = async (input, init) => {
    const target =
      typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
    const url = new URL(target, 'http://localhost');
    requests.push(url);
    if (url.pathname === '/api/v1/environments') {
      signals.push(init?.signal);
      return read(url, init?.signal);
    }
    if (url.pathname === '/api/v1/sources')
      return json(
        ['source_demo', 'source_other'].map((id) => ({
          ...observation,
          id,
          provider: 'demo',
          name: id,
          synthetic: true,
          resource_count: 0,
          health: { healthy: 0, degraded: 0, unhealthy: 0, unknown: 0 },
        })),
      );
    if (url.pathname.startsWith('/api/v1/metrics/')) return json({ ...observation, items: [] });
    if (url.pathname === '/api/v1/resources') {
      const source = url.searchParams.get('source_id') ?? 'source_demo';
      const selected = url.searchParams.get('environment_id');
      return json(
        page(
          [0, 1]
            .filter((index) => selected === null || selected === `env_${String(index)}`)
            .map((index) => ({
              id: index === 0 ? resourceId : `r1_${'B'.repeat(43)}`,
              provider: 'demo',
              source_id: source,
              environment_id: `env_${String(index)}`,
              kind: 'demo.service',
              scope_kind: 'project',
              name: `Service ${String(index)}`,
              synthetic: true,
              observed_at: observedAt,
            })),
        ),
      );
    }
    return json(page());
  };
  const auth = createAuthenticatedAuthStore();
  const api = createProductApi(createBrowserApiClient({ fetch }), auth);
  const router = createAppRouter(
    createMemoryHistory({ initialEntries: [entry] }),
    auth,
    undefined,
    api,
  );
  const rendered = render(<App router={router} />);
  return { router, requests, signals, ...rendered };
}

function parameters(router: ReturnType<typeof createAppRouter>) {
  return new URLSearchParams(router.state.location.searchStr);
}

const labels = {
  en: {
    open: /^Browse environments/u,
    title: 'Choose an environment',
    select: 'Environment',
    reload: 'Reload environments',
    more: 'Load next page',
  },
  fa: {
    open: /^مرور محیط‌ها/u,
    title: 'انتخاب محیط',
    select: 'محیط',
    reload: 'بارگذاری دوبارهٔ محیط‌ها',
    more: 'بارگیری صفحه بعد',
  },
} as const;

describe('environment selection', () => {
  it.each(['en', 'fa'] as const)(
    'keeps keyboard focus in the %s dialog and restores it on Escape',
    async (locale) => {
      localStorage.setItem('stackplore.locale', locale);
      const user = userEvent.setup();
      const { requests } = renderEnvironments();
      const trigger = await screen.findByRole('button', { name: labels[locale].open });
      expect(requests.filter((url) => url.pathname.endsWith('/environments'))).toHaveLength(0);
      trigger.focus();
      await user.keyboard('{Enter}');
      const dialog = await screen.findByRole('dialog', { name: labels[locale].title });
      await within(dialog).findByRole('combobox', { name: labels[locale].select });
      expect(dialog.contains(document.activeElement)).toBe(true);
      for (let index = 0; index < 4; index++) {
        await user.tab();
        expect(dialog.contains(document.activeElement)).toBe(true);
      }
      await user.keyboard('{Escape}');
      await waitFor(() => {
        expect(screen.queryByRole('dialog')).toBeNull();
      });
      expect(document.activeElement).toBe(trigger);
      expect(document.documentElement.dir).toBe(locale === 'fa' ? 'rtl' : 'ltr');
    },
  );

  it.each(['en', 'fa'] as const)(
    'loads bounded environment pages and retries initial and later failures in %s',
    async (locale) => {
      localStorage.setItem('stackplore.locale', locale);
      const user = userEvent.setup();
      let firstAttempts = 0;
      let nextAttempts = 0;
      const { requests } = renderEnvironments(undefined, (url) => {
        if (url.searchParams.has('cursor')) {
          if (++nextAttempts === 1) throw new TypeError('Fixture page unavailable.');
          return json(page([environment(1)]));
        }
        if (++firstAttempts === 1) throw new TypeError('Fixture page unavailable.');
        return json(page([environment(0)], 'next_public_page'));
      });
      await user.click(await screen.findByRole('button', { name: labels[locale].open }));
      const dialog = await screen.findByRole('dialog', { name: labels[locale].title });
      await user.click(await within(dialog).findByRole('button', { name: labels[locale].reload }));
      await within(dialog).findByRole('option', { name: 'Environment 0' });
      await user.click(within(dialog).getByRole('button', { name: labels[locale].more }));
      await within(dialog).findByRole('alert');
      await user.click(within(dialog).getByRole('button', { name: labels[locale].more }));
      await within(dialog).findByRole('option', { name: 'Environment 1' });
      expect(within(dialog).getAllByRole('option')).toHaveLength(3);
      expect(within(dialog).queryByRole('button', { name: labels[locale].more })).toBeNull();
      expect(firstAttempts).toBe(2);
      expect(nextAttempts).toBe(2);
      for (const url of requests.filter((request) => request.pathname.endsWith('/environments'))) {
        expect(url.searchParams.get('limit')).toBe('100');
        expect(url.searchParams.get('source_id')).toBe('source_demo');
      }
    },
  );

  it('selects a source-bound environment from About and opens resources', async () => {
    const user = userEvent.setup();
    const { router, requests } = renderEnvironments('/about');
    await user.click(await screen.findByRole('button', { name: labels.en.open }));
    const dialog = await screen.findByRole('dialog', { name: labels.en.title });
    await user.selectOptions(await within(dialog).findByRole('combobox'), 'source_demo/env_1');
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/resources');
    });
    expect(router.state.location.search).toMatchObject({
      source: 'source_demo',
      environment_id: 'env_1',
    });
    await waitFor(() => {
      expect(
        requests.some(
          (url) =>
            url.pathname.endsWith('/resources') &&
            url.searchParams.get('environment_id') === 'env_1' &&
            url.searchParams.get('source_id') === 'source_demo',
        ),
      ).toBe(true);
    });
    expect(screen.queryByRole('dialog')).toBeNull();
    await waitFor(() => {
      expect(document.activeElement).toBe(screen.getByRole('main'));
    });
  });

  it.each(['/resources', '/search', '/events'])(
    'changes environment on %s without leaving the route or losing its search',
    async (path) => {
      const user = userEvent.setup();
      const { router } = renderEnvironments(
        `${path}?source=source_demo&q=gateway&event_q=ready&minutes=360&resource_id=${resourceId}&rule_cursor=cursor_public`,
      );
      await user.click(await screen.findByRole('button', { name: labels.en.open }));
      const dialog = await screen.findByRole('dialog', { name: labels.en.title });
      await user.selectOptions(await within(dialog).findByRole('combobox'), 'source_demo/env_1');
      await waitFor(() => {
        expect(parameters(router).get('environment_id')).toBe('env_1');
      });
      expect(router.state.location.pathname).toBe(path);
      expect(router.state.location.search.q).toBe('gateway');
      expect(parameters(router).get('event_q')).toBe('ready');
      expect(productViewSearch(router.state.location.search).minutes).toBe('360');
      expect(parameters(router).has('resource_id')).toBe(false);
      expect(parameters(router).has('rule_cursor')).toBe(false);
    },
  );

  it('shows current denial without selectable stale choices and reloads after recovery', async () => {
    const user = userEvent.setup();
    let denied = true;
    renderEnvironments(undefined, () =>
      json(
        denied
          ? {
              ...page(),
              availability: 'permission_denied',
              reason_code: 'provider.permission_denied',
            }
          : page([environment(0)]),
      ),
    );
    await user.click(await screen.findByRole('button', { name: labels.en.open }));
    const dialog = await screen.findByRole('dialog', { name: labels.en.title });
    const retry = await within(dialog).findByRole('button', { name: labels.en.reload });
    expect(within(dialog).queryByRole('combobox')).toBeNull();
    denied = false;
    await user.click(retry);
    await within(dialog).findByRole('option', { name: 'Environment 0' });
  });

  it('cancels old source reads and ignores their late result', async () => {
    const user = userEvent.setup();
    let finishOld: (response: Response) => void = () => {
      throw new Error('Fixture read missing.');
    };
    const { router, signals } = renderEnvironments(undefined, (url) =>
      url.searchParams.get('source_id') === 'source_demo'
        ? new Promise<Response>((resolve) => {
            finishOld = resolve;
          })
        : json(page([environment(2, 'source_other')])),
    );
    await user.click(await screen.findByRole('button', { name: labels.en.open }));
    await waitFor(() => {
      expect(signals).toHaveLength(1);
    });
    await act(async () => {
      await router.navigate({ to: '/about', search: { source: 'source_other' } });
    });
    expect(signals[0]?.aborted).toBe(true);
    const dialog = screen.getByRole('dialog', { name: labels.en.title });
    await within(dialog).findByRole('option', { name: 'Environment 2' });
    await act(async () => {
      finishOld(json(page([environment(0)])));
      await Promise.resolve();
    });
    expect(within(dialog).queryByRole('option', { name: 'Environment 0' })).toBeNull();
  });

  it('aborts a closed dialog read and opens a fresh query next time', async () => {
    const user = userEvent.setup();
    let finishOld: (response: Response) => void = () => {
      throw new Error('Fixture read missing.');
    };
    let reads = 0;
    const { signals } = renderEnvironments(undefined, () =>
      ++reads === 1
        ? new Promise<Response>((resolve) => {
            finishOld = resolve;
          })
        : json(page([environment(1)])),
    );
    const trigger = await screen.findByRole('button', { name: labels.en.open });
    await user.click(trigger);
    await waitFor(() => {
      expect(signals).toHaveLength(1);
    });
    await user.keyboard('{Escape}');
    expect(signals[0]?.aborted).toBe(true);
    await user.click(trigger);
    const dialog = await screen.findByRole('dialog', { name: labels.en.title });
    await within(dialog).findByRole('option', { name: 'Environment 1' });
    await act(async () => {
      finishOld(json(page([environment(0)])));
      await Promise.resolve();
    });
    expect(within(dialog).queryByRole('option', { name: 'Environment 0' })).toBeNull();
  });

  it('clears source-bound environment, target and cursors on source change', async () => {
    const user = userEvent.setup();
    const { router, requests } = renderEnvironments(
      `/resources?source=source_demo&environment_id=env_0&topology_environment=env_1&topology_selected=${resourceId}&resource_id=${resourceId}&container=api&rule_cursor=cursor_public`,
    );
    const source = await screen.findByRole('combobox', { name: 'Infrastructure source' });
    await within(source).findByRole('option', { name: 'source_other' });
    await user.selectOptions(source, 'source_other');
    await waitFor(() => {
      expect(router.state.location.search.source).toBe('source_other');
    });
    for (const key of [
      'environment_id',
      'topology_environment',
      'topology_selected',
      'resource_id',
      'container',
      'rule_cursor',
    ])
      expect(parameters(router).has(key)).toBe(false);
    await waitFor(() => {
      expect(
        requests.some(
          (url) =>
            url.pathname.endsWith('/resources') &&
            url.searchParams.get('source_id') === 'source_other' &&
            !url.searchParams.has('environment_id'),
        ),
      ).toBe(true);
    });
  });

  it('keeps environment only on supported navigation and palette destinations', async () => {
    const user = userEvent.setup();
    const { router } = renderEnvironments('/resources?source=source_demo&environment_id=env_1');
    await screen.findByRole('button', { name: labels.en.open });
    for (const path of [
      '/resources',
      '/topology',
      '/events',
      '/overview',
      '/logs',
      '/alerts',
      '/about',
    ]) {
      const link = screen
        .getAllByRole('link')
        .find(
          (item) => new URL(item.getAttribute('href') ?? '', 'http://localhost').pathname === path,
        );
      expect(link).toBeDefined();
      const url = new URL(link?.getAttribute('href') ?? '', 'http://localhost');
      expect(url.searchParams.get('source')).toBe('source_demo');
      expect(url.searchParams.get('environment_id')).toBe(
        ['/resources', '/topology', '/events'].includes(path) ? 'env_1' : null,
      );
    }
    for (const path of ['/search', '/overview']) {
      await user.click(screen.getByRole('button', { name: 'Open command palette' }));
      const dialog = await screen.findByRole('dialog', { name: 'Go to a page' });
      await user.type(within(dialog).getByRole('combobox'), path);
      await user.keyboard('{Enter}');
      await waitFor(() => {
        expect(router.state.location.pathname).toBe(path);
      });
      expect(router.state.location.search.source).toBe('source_demo');
      expect(parameters(router).get('environment_id')).toBe(path === '/search' ? 'env_1' : null);
      await waitFor(() => {
        expect(screen.queryByRole('dialog')).toBeNull();
      });
    }
  });

  it.each(['environment_id', 'topology_environment'])(
    'applies the %s topology filter and writes only the canonical parameter',
    async (parameter) => {
      const user = userEvent.setup();
      const { router, requests } = renderEnvironments(
        `/topology?source=source_demo&${parameter}=env_1`,
      );
      const filter = await screen.findByRole<HTMLSelectElement>('combobox', {
        name: 'Environment',
      });
      expect(filter.value).toBe('env_1');
      await waitFor(() => {
        expect(
          requests.some(
            (url) =>
              url.pathname.endsWith('/resources') &&
              url.searchParams.get('environment_id') === 'env_1',
          ),
        ).toBe(true);
      });
      await user.selectOptions(filter, '');
      await waitFor(() => {
        expect(parameters(router).has('environment_id')).toBe(false);
      });
      expect(parameters(router).has('topology_environment')).toBe(false);
      const clearedFilter = await screen.findByRole('combobox', { name: 'Environment' });
      await within(clearedFilter).findByRole('option', { name: 'env_0' });
      await user.selectOptions(clearedFilter, 'env_0');
      await waitFor(() => {
        expect(parameters(router).get('environment_id')).toBe('env_0');
      });
      expect(parameters(router).has('topology_environment')).toBe(false);
    },
  );

  it('bounds loaded environment choices at 1,000 and stops requesting extra pages', async () => {
    const user = userEvent.setup();
    let pages = 0;
    renderEnvironments(undefined, () => {
      const index = pages++;
      return json(
        page(
          Array.from({ length: 100 }, (_, offset) => environment(index * 100 + offset)),
          `page_${String(index + 1)}`,
        ),
      );
    });
    await user.click(await screen.findByRole('button', { name: labels.en.open }));
    const dialog = await screen.findByRole('dialog', { name: labels.en.title });
    const select = await within(dialog).findByRole<HTMLSelectElement>('combobox', {
      name: labels.en.select,
    });
    const more = within(dialog).getByRole('button', { name: labels.en.more });
    expect(select.options.length).toBe(101);
    expect(select.options.item(1)?.textContent).toBe('Environment 0');
    for (let index = 1; index < 10; index++) {
      await user.click(more);
      await waitFor(
        () => {
          expect(select.options.length).toBe((index + 1) * 100 + 1);
          expect(select.options.item(index * 100 + 1)?.textContent).toBe(
            `Environment ${String(index * 100)}`,
          );
        },
        { container: select },
      );
    }
    expect(select.options.length).toBe(1001);
    expect(select.options.item(1000)?.textContent).toBe('Environment 999');
    expect(pages).toBe(10);
    expect(within(dialog).queryByRole('button', { name: labels.en.more })).toBeNull();
    expect(within(dialog).getByRole('status').textContent).toContain('1,000');
  });

  it('clears the legacy topology environment in a standalone view and writes the canonical selection', async () => {
    const initial = window.location.href;
    window.history.replaceState(null, '', '/topology?topology_environment=env_1');
    try {
      const user = userEvent.setup();
      render(
        <LocaleProvider>
          <TopologyGraph
            relationships={[]}
            resources={[
              {
                id: resourceId,
                provider: 'demo',
                source_id: 'source_demo',
                environment_id: 'env_1',
                kind: 'demo.service',
                scope_kind: 'project',
                name: 'Public service',
                observed_at: observedAt,
                synthetic: true,
              },
            ]}
          />
        </LocaleProvider>,
      );
      const filter = screen.getByRole<HTMLSelectElement>('combobox', { name: 'Environment' });
      expect(filter.value).toBe('env_1');
      await user.selectOptions(filter, '');
      expect(new URLSearchParams(window.location.search).has('environment_id')).toBe(false);
      expect(new URLSearchParams(window.location.search).has('topology_environment')).toBe(false);
      await user.selectOptions(filter, 'env_1');
      expect(new URLSearchParams(window.location.search).get('environment_id')).toBe('env_1');
      expect(new URLSearchParams(window.location.search).has('topology_environment')).toBe(false);
    } finally {
      window.history.replaceState(null, '', initial);
    }
  });

  it('validates environment identifiers and scopes only the four supported routes', () => {
    expect(readEnvironment('?environment_id=env_0&topology_environment=env_1')).toBe('env_0');
    expect(readEnvironment('?topology_environment=env_1')).toBe('env_1');
    expect(readEnvironment(`?environment_id=${'x'.repeat(129)}`)).toBeUndefined();
    for (const path of ['/resources', '/search', '/events', '/topology']) {
      expect(environmentDestination(path)).toBe(path);
      expect(environmentNavigationSearch(path, 'source_demo', 'env_1')).toEqual({
        source: 'source_demo',
        environment_id: 'env_1',
      });
    }
    for (const path of ['/overview', '/logs', '/alerts', '/resources/detail', '/about']) {
      expect(environmentDestination(path)).toBe('/resources');
      expect(environmentNavigationSearch(path, 'source_demo', 'env_1')).toEqual({
        source: 'source_demo',
      });
    }
  });
});
