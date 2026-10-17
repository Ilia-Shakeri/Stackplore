import type { ProductAlert, ProductAlertPage } from '@stackplore/api-client';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

afterEach(() => {
  window.history.replaceState(null, '', '/');
});

import {
  ProductAuditPage,
  ProductAlertsPage,
  ProductLogsPage,
  ProductOverviewPage,
  ProductResourceDetailPage,
  ProductSystemSettingsPage,
} from './ProductPages';
import { createProductApi, type ProductApi } from './product-api';
import { ProductProvider } from './product-context';
import { AuthProvider } from '../auth/auth-context';
import { LocaleProvider } from '../../i18n';
import { createAuthenticatedAuthStore } from '../../test/auth';

const resourceId = `r1_${'A'.repeat(43)}`;
const observedAt = '2026-08-16T08:00:00Z';

const productApi: ProductApi = {
  ...createProductApi({
    request: () => Promise.reject(new Error('Unexpected product test request.')),
  }),
  listSourceSettings: vi.fn(),
  previewSourceUpdate: vi.fn(),
  updateSource: vi.fn(),
  acknowledgeAlert: vi.fn(),
  createAlertRule: vi.fn(),
  deleteAlertRule: vi.fn(),
  listAlertRules: vi.fn().mockResolvedValue({ items: [], page: { has_more: false, limit: 100 } }),
  replaceAlertRule: vi.fn(),
  testNotification: vi.fn(),
  getSystemReadiness: vi.fn().mockResolvedValue({
    checks: [
      { name: 'configuration', state: 'ready' },
      { name: 'database', state: 'ready' },
      { name: 'service', state: 'ready' },
    ],
    state: 'ready',
  }),
  listAlerts: vi.fn().mockResolvedValue({ items: [], page: { has_more: false, limit: 100 } }),
  listAudit: vi.fn().mockResolvedValue({ items: [], page: { has_more: false, limit: 50 } }),
  listEnvironments: vi.fn().mockResolvedValue({
    availability: 'available',
    completeness: 'complete',
    observed_at: observedAt,
    items: [],
    page: { has_more: false, limit: 50 },
  }),
  getResource: vi.fn().mockResolvedValue({
    environment_id: 'env_demo_shop',
    health: 'degraded',
    id: resourceId,
    kind: 'demo.gateway',
    name: 'Gateway',
    observed_at: observedAt,
    provider: 'demo',
    scope_kind: 'project',
    scope_name: 'demo-shop',
    source_id: 'source_demo',
    synthetic: true,
  }),
  getResourceHealth: vi.fn().mockResolvedValue({
    observed_at: observedAt,
    reason_codes: ['demo.health.latency'],
    resource_id: resourceId,
    state: 'degraded',
    synthetic: true,
  }),
  getNotificationSettings: vi.fn().mockResolvedValue({
    adapter: 'generic_webhook',
    matrix: { adapter: 'matrix', enabled: false },
    configuration_source: 'operator_environment',
    delivery_semantics: 'at_least_once',
    enabled: false,
  }),
  listEvents: vi.fn().mockResolvedValue({ items: [], page: { has_more: false, limit: 100 } }),
  listLogs: vi.fn().mockResolvedValue({ items: [], page: { has_more: false, limit: 100 } }),
  listMetricHistory: vi.fn().mockResolvedValue({
    availability: 'available',
    completeness: 'complete',
    items: [],
    observed_at: observedAt,
  }),
  listMetrics: vi.fn().mockResolvedValue({
    availability: 'available',
    completeness: 'complete',
    items: [],
    observed_at: observedAt,
  }),
  listNotifications: vi.fn().mockResolvedValue({ items: [], page: { has_more: false, limit: 50 } }),
  listResources: vi.fn().mockResolvedValue({
    items: [],
    page: { has_more: false, limit: 50 },
  }),
  listSources: vi.fn().mockResolvedValue([]),
  listTopology: vi.fn().mockResolvedValue({ items: [], page: { has_more: false, limit: 100 } }),
};

describe('product resource detail', () => {
  it('keeps verified resource identity when optional health cannot be read', async () => {
    const api: ProductApi = {
      ...productApi,
      getResourceHealth: vi.fn().mockRejectedValue(new Error('Health unavailable.')),
    };
    render(
      <LocaleProvider defaultLocale="en">
        <ProductProvider api={api}>
          <ProductResourceDetailPage
            id={resourceId}
            kind="demo.gateway"
            provider="demo"
            source="source_demo"
          />
        </ProductProvider>
      </LocaleProvider>,
    );
    expect(await screen.findByRole('heading', { level: 2, name: 'Gateway' })).not.toBeNull();
    expect(
      screen.getByText('Health is unavailable. Resource metadata remains visible.'),
    ).not.toBeNull();
    expect(screen.getByRole('link', { name: 'Resource logs' }).getAttribute('href')).toBe(
      `/logs?source=source_demo&resource_id=${resourceId}`,
    );
    expect(screen.getByText('Unknown')).not.toBeNull();
  });
  it('renders connected identity and health data', async () => {
    render(
      <LocaleProvider defaultLocale="en" storageKey="stackplore.product-test.locale">
        <ProductProvider api={productApi}>
          <ProductResourceDetailPage
            id={resourceId}
            kind="demo.gateway"
            provider="demo"
            source="source_demo"
          />
        </ProductProvider>
      </LocaleProvider>,
    );

    expect(await screen.findByRole('heading', { level: 2, name: 'Gateway' })).not.toBeNull();
    expect(screen.getByText('Degraded')).not.toBeNull();
    expect(screen.getByText('demo.health.latency')).not.toBeNull();
    expect(screen.getByText('project:demo-shop').getAttribute('dir')).toBe('ltr');
  });
});

describe('product observation state', () => {
  it('lets a slow log read finish while stream changes queue behind it', async () => {
    const streams: EventTarget[] = [];
    class Stream extends EventTarget {
      constructor() {
        super();
        streams.push(this);
      }
      close() {}
    }
    vi.stubGlobal('EventSource', Stream);
    let finish!: () => void;
    const response: Awaited<ReturnType<ProductApi['listLogs']>> = {
      availability: 'available',
      completeness: 'complete',
      observed_at: observedAt,
      page: { has_more: false, limit: 100 },
      items: [
        {
          id: 'slow_log',
          resource_id: resourceId,
          stream: 'stdout',
          sequence: 1,
          severity: 'info',
          timestamp: observedAt,
          message: 'slow target finished',
          redacted: false,
          truncated: false,
          synthetic: false,
        },
      ],
    };
    const listLogs = vi
      .fn<ProductApi['listLogs']>()
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finish = () => {
              resolve(response);
            };
          }),
      )
      .mockResolvedValue(response);
    const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-17T08:00:00Z'));
    try {
      render(
        <LocaleProvider defaultLocale="en">
          <ProductProvider
            api={{ ...productApi, listLogs }}
            sourceId="source_docker"
            onSourceChange={() => {}}
          >
            <ProductLogsPage />
          </ProductProvider>
        </LocaleProvider>,
      );
      await waitFor(() => {
        expect(listLogs).toHaveBeenCalledTimes(1);
      });
      const firstSignal = listLogs.mock.calls[0]?.[0]?.signal;
      clock.mockReturnValue(Date.parse('2026-10-17T08:00:08Z'));
      act(() => {
        for (let index = 0; index < 4; index++)
          streams[0]?.dispatchEvent(new Event('logs.snapshot.v1'));
      });
      expect(firstSignal?.aborted).toBe(false);
      expect(listLogs).toHaveBeenCalledTimes(1);
      await act(async () => {
        finish();
        await Promise.resolve();
      });
      expect(await screen.findByText('slow target finished')).not.toBeNull();
      await waitFor(() => {
        expect(listLogs).toHaveBeenCalledTimes(2);
      });
    } finally {
      clock.mockRestore();
      vi.unstubAllGlobals();
    }
  });
  it('polls an exact log target and advances its rolling window without a source snapshot', async () => {
    let poll: (() => void) | undefined;
    const realTimer = window.setTimeout.bind(window);
    const timer = vi.spyOn(window, 'setTimeout').mockImplementation((handler, delay, ...args) => {
      if (delay === 5000 && typeof handler === 'function')
        poll = () => {
          handler(...args);
        };
      return realTimer(handler, delay, ...args) as unknown as ReturnType<typeof window.setTimeout>;
    });
    const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-17T08:00:00Z'));
    try {
      const listLogs = vi.fn<ProductApi['listLogs']>().mockResolvedValue({
        availability: 'available',
        completeness: 'complete',
        observed_at: observedAt,
        page: { has_more: false, limit: 100 },
        items: [
          {
            id: 'target_line',
            resource_id: resourceId,
            stream: 'stdout',
            sequence: 1,
            severity: 'info',
            timestamp: observedAt,
            message: 'target outside snapshot budget',
            redacted: false,
            truncated: false,
            synthetic: false,
          },
        ],
      });
      const api: ProductApi = { ...productApi, listLogs };
      render(
        <LocaleProvider defaultLocale="en">
          <ProductProvider
            api={api}
            sourceId="source_docker"
            onSourceChange={() => {}}
            viewSearch={`resource_id=${resourceId}&container=worker&tail=live&minutes=360`}
          >
            <ProductLogsPage />
          </ProductProvider>
        </LocaleProvider>,
      );
      expect(await screen.findByText('target outside snapshot budget')).not.toBeNull();
      expect(listLogs.mock.calls.at(-1)?.[0]).toMatchObject({
        resourceId,
        container: 'worker',
        from: '2026-10-17T02:00:00.000Z',
        to: '2026-10-17T08:00:00.000Z',
      });
      await waitFor(() => {
        expect(poll).toBeTypeOf('function');
      });
      clock.mockReturnValue(Date.parse('2026-10-17T08:00:05Z'));
      act(() => {
        poll?.();
      });
      await waitFor(() => {
        expect(listLogs.mock.calls.at(-1)?.[0]).toMatchObject({
          resourceId,
          container: 'worker',
          from: '2026-10-17T02:00:05.000Z',
          to: '2026-10-17T08:00:05.000Z',
        });
      });
      const current = listLogs.mock.calls.length;
      const user = userEvent.setup();
      await user.click(screen.getByRole('button', { name: 'Pause live tail' }));
      const pausedCalls = listLogs.mock.calls.length;
      expect(pausedCalls).toBeGreaterThanOrEqual(current);
      clock.mockReturnValue(Date.parse('2026-10-17T08:00:10Z'));
      expect(screen.getByText('target outside snapshot budget')).not.toBeNull();
    } finally {
      timer.mockRestore();
      clock.mockRestore();
    }
  });
  it('anchors demo log windows and range changes to the observed fixture clock', async () => {
    const listLogs = vi.fn<ProductApi['listLogs']>().mockImplementation((options) =>
      Promise.resolve({
        availability: 'available',
        completeness: 'complete',
        observed_at: observedAt,
        page: { has_more: false, limit: 100 },
        items:
          Date.parse(options?.from ?? '') <= Date.parse(observedAt) &&
          Date.parse(options?.to ?? '') > Date.parse(observedAt)
            ? [
                {
                  id: 'demo_clock_log',
                  resource_id: resourceId,
                  stream: 'stdout',
                  sequence: 1,
                  severity: 'info',
                  timestamp: observedAt,
                  message: 'fixture clock line',
                  redacted: false,
                  truncated: false,
                  synthetic: true,
                },
              ]
            : [],
      }),
    );
    const api: ProductApi = {
      ...productApi,
      listLogs,
      listSources: vi.fn().mockResolvedValue([
        {
          id: 'source_demo',
          name: 'Demo',
          provider: 'demo',
          synthetic: true,
          availability: 'available',
          completeness: 'complete',
          observed_at: observedAt,
          resource_count: 1,
          health: { healthy: 1, degraded: 0, unhealthy: 0, unknown: 0 },
        },
      ]),
    };
    const user = userEvent.setup();
    render(
      <LocaleProvider defaultLocale="en">
        <ProductProvider api={api} sourceId="source_demo" onSourceChange={() => {}}>
          <ProductLogsPage />
        </ProductProvider>
      </LocaleProvider>,
    );
    expect(await screen.findByText('fixture clock line')).not.toBeNull();
    expect(listLogs.mock.calls.at(-1)?.[0]).toMatchObject({
      from: '2026-08-16T07:00:00.001Z',
      to: '2026-08-16T08:00:00.001Z',
    });
    await user.selectOptions(screen.getByRole('combobox', { name: 'Time range' }), '360');
    await waitFor(() => {
      expect(listLogs.mock.calls.at(-1)?.[0]).toMatchObject({
        from: '2026-08-16T02:00:00.001Z',
        to: '2026-08-16T08:00:00.001Z',
      });
    });
    expect(screen.getByText('fixture clock line')).not.toBeNull();
  });
  it('pauses live tail, copies selected redacted lines, and downloads filtered server logs', async () => {
    const streams: { readonly close: ReturnType<typeof vi.fn> }[] = [];
    class Stream extends EventTarget {
      readonly close = vi.fn();
      constructor() {
        super();
        streams.push(this);
      }
    }
    vi.stubGlobal('EventSource', Stream);
    const createUrl = vi.fn().mockReturnValue('blob:loaded-log-lines');
    const revokeUrl = vi.fn();
    class LogURL extends URL {
      static override createObjectURL = createUrl;
      static override revokeObjectURL = revokeUrl;
    }
    vi.stubGlobal('URL', LogURL);
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => undefined);
    try {
      const user = userEvent.setup();
      const copied = vi.spyOn(navigator.clipboard, 'writeText');
      const api: ProductApi = {
        ...productApi,
        downloadLogs: vi
          .fn()
          .mockResolvedValue({ text: 'safe filtered log download', limited: false }),
        listLogs: vi.fn().mockResolvedValue({
          availability: 'stale',
          completeness: 'partial',
          reason_code: 'logs.partial',
          observed_at: observedAt,
          page: { has_more: false, limit: 100 },
          items: [
            {
              id: 'log_loaded',
              resource_id: resourceId,
              stream: 'stdout',
              sequence: 1,
              severity: 'warning',
              timestamp: observedAt,
              message: 'safe [redacted] output',
              redacted: true,
              truncated: false,
              synthetic: true,
            },
          ],
        }),
      };
      render(
        <LocaleProvider defaultLocale="en">
          <ProductProvider api={api}>
            <ProductLogsPage />
          </ProductProvider>
        </LocaleProvider>,
      );
      expect(await screen.findByText('safe [redacted] output')).not.toBeNull();
      expect(screen.getByText('Sensitive text removed')).not.toBeNull();
      expect(screen.getByText('Stale Partial observation')).not.toBeNull();
      await user.click(screen.getByRole('checkbox', { name: 'Select log line 1' }));
      await user.click(screen.getByRole('button', { name: 'Copy selected lines' }));
      expect(copied).toHaveBeenCalledWith(`${observedAt} warning safe [redacted] output`);
      await user.click(screen.getByRole('button', { name: 'Download filtered logs' }));
      expect(createUrl).toHaveBeenCalledOnce();
      await waitFor(() => {
        expect(revokeUrl).toHaveBeenCalledWith('blob:loaded-log-lines');
      });
      expect(click).toHaveBeenCalledOnce();
      await user.click(screen.getByRole('button', { name: 'Pause live tail' }));
      expect(streams[0]?.close).toHaveBeenCalledOnce();
      expect(screen.getByText('safe [redacted] output')).not.toBeNull();
      await user.click(screen.getByRole('button', { name: 'Resume live tail' }));
      expect(streams).toHaveLength(2);
      await user.click(screen.getByRole('checkbox', { name: 'Wrap lines' }));
      expect(
        screen.getByLabelText('Observed resource logs').classList.contains('log-list--no-wrap'),
      ).toBe(true);
    } finally {
      click.mockRestore();
      vi.unstubAllGlobals();
    }
  });

  it.each(['available', 'permission_denied', 'unsupported', 'unavailable'] as const)(
    'shows missing %s metrics without invented zero values',
    async (availability) => {
      const api: ProductApi = {
        ...productApi,
        listMetrics: vi.fn().mockResolvedValue({
          availability,
          completeness: 'complete',
          items: [],
          observed_at: observedAt,
          ...(availability === 'available' ? {} : { reason_code: 'metric.not_available' }),
        }),
      };
      render(
        <LocaleProvider defaultLocale="en">
          <ProductProvider api={api}>
            <ProductOverviewPage />
          </ProductProvider>
        </LocaleProvider>,
      );
      const summary = await screen.findByRole('region', { name: 'Current source metrics' });
      for (const name of ['CPU use', 'Memory use', 'Restarts']) {
        const value = within(summary).getByText(name).nextElementSibling?.textContent;
        expect(value).toBe(
          availability === 'permission_denied'
            ? 'Access denied'
            : availability === 'unsupported'
              ? 'Unsupported'
              : 'Unavailable',
        );
      }
    },
  );

  it('keeps measured zero and marks stale partial metrics', async () => {
    const api: ProductApi = {
      ...productApi,
      listMetrics: vi.fn().mockResolvedValue({
        availability: 'stale',
        completeness: 'partial',
        observed_at: observedAt,
        reason_code: 'metric.partial',
        items: [
          {
            id: 'metric_cpu',
            resource_id: resourceId,
            name: 'cpu.usage.cores',
            unit: 'cores',
            value: 0,
            sampled_at: observedAt,
            interval_seconds: 30,
            synthetic: true,
          },
        ],
      }),
    };
    render(
      <LocaleProvider defaultLocale="en">
        <ProductProvider api={api}>
          <ProductOverviewPage />
        </ProductProvider>
      </LocaleProvider>,
    );
    const summary = await screen.findByRole('region', { name: 'Current source metrics' });
    expect(within(summary).getByText('CPU use').nextElementSibling?.textContent).toBe('0 cores');
    expect(within(summary).getByRole('status').textContent).toBe('Stale Partial observation');
    expect(within(summary).getByText('Memory use').nextElementSibling?.textContent).toBe(
      'Unavailable',
    );
  });

  it('isolates raw log text in Persian', async () => {
    const api: ProductApi = {
      ...productApi,
      listLogs: vi.fn().mockResolvedValue({
        availability: 'available',
        completeness: 'complete',
        observed_at: observedAt,
        page: { has_more: false, limit: 100 },
        items: [
          {
            id: 'log_gateway',
            resource_id: resourceId,
            stream: 'stdout',
            sequence: 1,
            severity: 'info',
            timestamp: observedAt,
            message: 'GET /health: 200 (ok)',
            redacted: false,
            truncated: false,
            synthetic: true,
          },
        ],
      }),
    };
    render(
      <LocaleProvider defaultLocale="fa">
        <ProductProvider api={api}>
          <ProductLogsPage />
        </ProductProvider>
      </LocaleProvider>,
    );
    const line = await screen.findByText('GET /health: 200 (ok)');
    expect(document.documentElement.dir).toBe('rtl');
    expect(line.getAttribute('dir')).toBe('ltr');
    expect(line.hasAttribute('data-ui-technical')).toBe(true);
  });

  it('shows unsupported logs instead of a false empty list', async () => {
    const api: ProductApi = {
      ...productApi,
      listLogs: vi.fn().mockResolvedValue({
        availability: 'unsupported',
        completeness: 'complete',
        items: [],
        observed_at: observedAt,
        page: { has_more: false, limit: 100 },
        reason_code: 'kubernetes.log.not_implemented',
      }),
    };
    render(
      <LocaleProvider defaultLocale="en" storageKey="stackplore.product-state-test.locale">
        <ProductProvider api={api}>
          <ProductLogsPage />
        </ProductProvider>
      </LocaleProvider>,
    );

    expect(await screen.findByText('Unsupported')).not.toBeNull();
    expect(screen.queryByLabelText('Observed resource logs')).toBeNull();
  });
});

describe('product audit history', () => {
  it('renders safe actor, target, and request facts', async () => {
    const api: ProductApi = {
      ...productApi,
      listAudit: vi.fn().mockResolvedValue({
        items: [
          {
            action: 'auth.login',
            actor: { id: 'user_admin', kind: 'user' },
            id: 'audit_login_01',
            occurred_at: observedAt,
            outcome: 'success',
            request_id: 'request_login_01',
            summary_code: 'auth.login_succeeded',
            target: { id: 'user_admin', kind: 'auth.account' },
          },
        ],
        page: { has_more: false, limit: 50 },
      }),
    };
    render(
      <LocaleProvider defaultLocale="en" storageKey="stackplore.audit-test.locale">
        <ProductProvider api={api}>
          <ProductAuditPage />
        </ProductProvider>
      </LocaleProvider>,
    );

    expect(await screen.findByText('auth.login')).not.toBeNull();
    expect(screen.getByText('user:user_admin').getAttribute('dir')).toBe('ltr');
    expect(screen.getByText('request_login_01').getAttribute('dir')).toBe('ltr');
    expect(screen.getByText('Succeeded')).not.toBeNull();
  });
});

describe('product alert history', () => {
  const alertFixture = (
    id: string,
    summaryCode = 'alert.resource_unhealthy.matched',
  ): ProductAlert => ({
    id,
    rule_id: `rule_${id}`,
    severity: 'warning',
    started_at: observedAt,
    updated_at: observedAt,
    state: 'firing',
    summary_code: summaryCode,
    synthetic: false,
  });
  const deferred = <T,>() => {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>((accept) => {
      resolve = accept;
    });
    return { promise, resolve };
  };
  const stubAlertStreams = () => {
    const streams: EventTarget[] = [];
    class Stream extends EventTarget {
      constructor() {
        super();
        streams.push(this);
      }
      close() {}
    }
    vi.stubGlobal('EventSource', Stream);
    return streams;
  };

  it('keeps refreshed alerts and a newer resolution when acknowledgement replies late', async () => {
    const streams = stubAlertStreams();
    try {
      const firing = alertFixture('alert_live');
      const fresh = alertFixture('alert_fresh', 'alert.provider_connection_lost.matched');
      const ack = deferred<ProductAlert>();
      const listAlerts = vi
        .fn<ProductApi['listAlerts']>()
        .mockResolvedValueOnce({ items: [firing], page: { has_more: false, limit: 100 } })
        .mockResolvedValueOnce({
          items: [{ ...firing, state: 'resolved', resolved_at: observedAt }, fresh],
          page: { has_more: false, limit: 100 },
        });
      const api: ProductApi = {
        ...productApi,
        listAlerts,
        acknowledgeAlert: vi.fn().mockReturnValue(ack.promise),
      };
      const user = userEvent.setup();
      const { container } = render(
        <LocaleProvider defaultLocale="en">
          <AuthProvider store={createAuthenticatedAuthStore()}>
            <ProductProvider api={api}>
              <ProductAlertsPage />
            </ProductProvider>
          </AuthProvider>
        </LocaleProvider>,
      );
      await user.click(await screen.findByRole('button', { name: 'Acknowledge' }));
      act(() => {
        streams[0]?.dispatchEvent(new Event('alerts.changed.v1'));
      });
      expect(await screen.findByText('Provider connection lost')).not.toBeNull();
      await act(async () => {
        ack.resolve({ ...firing, state: 'acknowledged', acknowledged_at: observedAt });
        await ack.promise;
      });
      expect(container.querySelector('#alert-alert_fresh')).not.toBeNull();
      const alertList = container.querySelector('.alert-list');
      if (!(alertList instanceof HTMLElement)) throw new Error('Alert list is missing.');
      expect(within(alertList).getByText('Resolved')).not.toBeNull();
      expect(within(alertList).queryByText('Acknowledged')).toBeNull();
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it.each(['reset', 'source_change'] as const)(
    'discards an older-page reply after %s',
    async (change) => {
      const streams = stubAlertStreams();
      try {
        const older = deferred<ProductAlertPage>();
        const listAlerts = vi
          .fn<ProductApi['listAlerts']>()
          .mockResolvedValueOnce({
            items: [alertFixture('alert_first')],
            page: { has_more: true, limit: 100, next_cursor: 'older_alerts' },
          })
          .mockReturnValueOnce(older.promise)
          .mockResolvedValueOnce({
            items: [alertFixture('alert_current', 'alert.provider_connection_lost.matched')],
            page: { has_more: false, limit: 100 },
          });
        const api: ProductApi = { ...productApi, listAlerts };
        const store = createAuthenticatedAuthStore();
        const view = (sourceId: string) => (
          <LocaleProvider defaultLocale="en">
            <AuthProvider store={store}>
              <ProductProvider api={api} sourceId={sourceId} onSourceChange={() => {}}>
                <ProductAlertsPage />
              </ProductProvider>
            </AuthProvider>
          </LocaleProvider>
        );
        const user = userEvent.setup();
        const { container, rerender } = render(view('source_demo'));
        await user.click(await screen.findByRole('button', { name: 'Load older alerts' }));
        if (change === 'reset')
          act(() => {
            streams[0]?.dispatchEvent(new Event('stream.reset.v1'));
          });
        else rerender(view('source_cluster'));
        expect(await screen.findByText('Provider connection lost')).not.toBeNull();
        await act(async () => {
          older.resolve({
            items: [alertFixture('alert_old')],
            page: { has_more: false, limit: 100 },
          });
          await older.promise;
        });
        expect(container.querySelector('#alert-alert_current')).not.toBeNull();
        expect(container.querySelector('#alert-alert_first')).toBeNull();
        expect(container.querySelector('#alert-alert_old')).toBeNull();
      } finally {
        vi.unstubAllGlobals();
      }
    },
  );
  it.each([
    ['en', 'Resource is unhealthy', 'Infrastructure alert'],
    ['fa', 'منبع ناسالم است', 'هشدار زیرساخت'],
  ] as const)('names each real predicate in %s', async (locale, unhealthy, fallback) => {
    const kinds = [
      'resource_unavailable',
      'resource_unhealthy',
      'restart_threshold',
      'cpu_threshold',
      'memory_threshold',
      'workload_unavailable_replicas',
      'event_severity_match',
      'provider_connection_lost',
    ];
    const items = kinds.flatMap((kind) =>
      ['matched', 'clear'].map((suffix) => ({
        id: `alert_${kind}_${suffix}`,
        rule_id: `rule_${kind}`,
        severity: 'warning',
        started_at: observedAt,
        updated_at: observedAt,
        state: suffix === 'clear' ? 'resolved' : 'firing',
        ...(suffix === 'clear' ? { resolved_at: observedAt } : {}),
        summary_code: `alert.${kind}.${suffix}`,
        synthetic: false,
      })),
    );
    const api: ProductApi = {
      ...productApi,
      listAlerts: vi.fn().mockResolvedValue({ items, page: { has_more: false, limit: 100 } }),
    };
    const { container } = render(
      <LocaleProvider defaultLocale={locale}>
        <AuthProvider store={createAuthenticatedAuthStore()}>
          <ProductProvider api={api}>
            <ProductAlertsPage />
          </ProductProvider>
        </AuthProvider>
      </LocaleProvider>,
    );
    expect(await screen.findByText(unhealthy)).not.toBeNull();
    expect(container.querySelectorAll('.alert-list article')).toHaveLength(16);
    expect(screen.queryByText(fallback)).toBeNull();
    expect(container.textContent).not.toContain('product.alert.');
  });

  it.each([false, true])(
    'lets an administrator acknowledge a persisted firing alert (synthetic: %s)',
    async (synthetic) => {
      const user = userEvent.setup();
      const firing = {
        id: 'alert_live',
        rule_id: 'rule_live',
        severity: 'warning' as const,
        started_at: observedAt,
        state: 'firing' as const,
        summary_code: 'alert.resource_unhealthy.matched',
        synthetic,
        updated_at: observedAt,
      };
      const acknowledgeAlert = vi.fn().mockResolvedValue({
        ...firing,
        acknowledged_at: observedAt,
        state: 'acknowledged',
      });
      const api: ProductApi = {
        ...productApi,
        acknowledgeAlert,
        listAlerts: vi.fn().mockResolvedValue({
          items: [firing],
          page: { has_more: false, limit: 100 },
        }),
      };
      render(
        <LocaleProvider defaultLocale="en" storageKey="stackplore.alert-ack-test.locale">
          <AuthProvider store={createAuthenticatedAuthStore()}>
            <ProductProvider api={api}>
              <ProductAlertsPage />
            </ProductProvider>
          </AuthProvider>
        </LocaleProvider>,
      );

      await user.click(await screen.findByRole('button', { name: 'Acknowledge' }));
      expect(acknowledgeAlert).toHaveBeenCalledWith('alert_live');
      await waitFor(() => {
        expect(document.querySelector('.alert-state--acknowledged')?.textContent).toBe(
          'Acknowledged',
        );
      });
    },
  );

  it('loads older cursor pages without dropping current alerts', async () => {
    const user = userEvent.setup();
    const listAlerts = vi
      .fn()
      .mockResolvedValueOnce({
        items: [
          {
            id: 'alert_new',
            rule_id: 'rule_new',
            severity: 'critical',
            started_at: observedAt,
            state: 'firing',
            summary_code: 'demo.alert.checkout_unhealthy',
            synthetic: true,
            updated_at: observedAt,
          },
        ],
        page: { has_more: true, limit: 100, next_cursor: 'older_alerts' },
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: 'alert_old',
            rule_id: 'rule_old',
            severity: 'warning',
            started_at: observedAt,
            state: 'resolved',
            summary_code: 'alert.provider_connection_lost.matched',
            synthetic: false,
            updated_at: observedAt,
          },
        ],
        page: { has_more: false, limit: 100 },
      });
    const api: ProductApi = { ...productApi, listAlerts };
    render(
      <LocaleProvider defaultLocale="en" storageKey="stackplore.alert-page-test.locale">
        <AuthProvider store={createAuthenticatedAuthStore()}>
          <ProductProvider api={api}>
            <ProductAlertsPage />
          </ProductProvider>
        </AuthProvider>
      </LocaleProvider>,
    );

    expect(await screen.findByText('Checkout health check is failing')).not.toBeNull();
    await user.click(screen.getByRole('button', { name: 'Load older alerts' }));
    expect(await screen.findByText('Provider connection lost')).not.toBeNull();
    expect(screen.getByText('Checkout health check is failing')).not.toBeNull();
    expect(listAlerts).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: 'older_alerts' }),
    );
  });
});

describe('system settings readiness', () => {
  it('shows each real readiness check without a false healthy rollup', async () => {
    const api: ProductApi = {
      ...productApi,
      getSystemReadiness: vi.fn().mockResolvedValue({
        checks: [
          { name: 'configuration', state: 'ready' },
          { name: 'database', state: 'not_ready' },
          { name: 'service', state: 'ready' },
        ],
        state: 'not_ready',
      }),
    };
    render(
      <LocaleProvider defaultLocale="en" storageKey="stackplore.readiness-test.locale">
        <ProductProvider api={api}>
          <ProductSystemSettingsPage toolbar={<nav aria-label="Settings sections" />} />
        </ProductProvider>
      </LocaleProvider>,
    );

    expect(
      await screen.findByRole('heading', { level: 2, name: 'System readiness' }),
    ).not.toBeNull();
    expect(screen.getByText('Database')).not.toBeNull();
    expect(screen.getAllByText('Not ready')).toHaveLength(2);
  });
});
