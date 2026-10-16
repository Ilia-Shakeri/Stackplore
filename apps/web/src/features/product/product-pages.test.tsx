import type { ProductAlert, ProductAlertPage } from '@stackplore/api-client';
import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import {
  ProductAuditPage,
  ProductAlertsPage,
  ProductLogsPage,
  ProductOverviewPage,
  ProductResourceDetailPage,
  ProductSystemSettingsPage,
} from './ProductPages';
import type { ProductApi } from './product-api';
import { ProductProvider } from './product-context';
import { AuthProvider } from '../auth/auth-context';
import { LocaleProvider } from '../../i18n';
import { createAuthenticatedAuthStore } from '../../test/auth';

const resourceId = `r1_${'A'.repeat(43)}`;
const observedAt = '2026-08-16T08:00:00Z';

const productApi: ProductApi = {
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
      expect(screen.getByText('Resolved')).not.toBeNull();
      expect(screen.queryByText('Acknowledged')).toBeNull();
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

  it('lets an administrator acknowledge a persisted firing alert', async () => {
    const user = userEvent.setup();
    const firing = {
      id: 'alert_live',
      rule_id: 'rule_live',
      severity: 'warning' as const,
      started_at: observedAt,
      state: 'firing' as const,
      summary_code: 'alert.resource_unhealthy.matched',
      synthetic: false,
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
    expect(await screen.findByText('Acknowledged')).not.toBeNull();
  });

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
    expect(listAlerts).toHaveBeenLastCalledWith({ cursor: 'older_alerts' });
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
