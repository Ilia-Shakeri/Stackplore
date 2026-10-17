import type { AlertRuleRecord } from '@stackplore/api-client';
import { createMemoryHistory } from '@tanstack/react-router';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ProductAlertRulesSettingsPage } from './AlertRulesSettingsPage';
import { createProductApi, type ProductApi } from './product-api';
import { ProductProvider } from './product-context';
import { LocaleProvider } from '../../i18n';
import { App } from '../../app/App';
import { createAppRouter } from '../../app/router';
import { createAuthenticatedAuthStore } from '../../test/auth';
import { createTestUsersApi } from '../../test/users';

const observedAt = '2026-08-20T08:00:00Z';
const rule: AlertRuleRecord = {
  created_at: observedAt,
  display_name: 'Provider lost',
  enabled: true,
  evaluation_interval_seconds: 30,
  id: 'rule_provider_lost',
  kind: 'provider_connection_lost',
  pending_for_seconds: 60,
  severity: 'critical',
  source_id: 'source_demo',
  updated_at: observedAt,
  version: 1,
};

function productApi(overrides: Partial<ProductApi> = {}): ProductApi {
  return {
    ...createProductApi({
      request: () => Promise.reject(new Error('Unexpected rule test request.')),
    }),
    listSourceSettings: vi.fn(),
    previewSourceUpdate: vi.fn(),
    updateSource: vi.fn(),
    acknowledgeAlert: vi.fn(),
    createAlertRule: vi.fn(),
    deleteAlertRule: vi.fn(),
    getResource: vi.fn(),
    getResourceHealth: vi.fn(),
    getNotificationSettings: vi.fn().mockResolvedValue({
      adapter: 'generic_webhook',
      matrix: { adapter: 'matrix', enabled: false },
      configuration_source: 'operator_environment',
      delivery_semantics: 'at_least_once',
      enabled: false,
    }),
    getSystemReadiness: vi.fn(),
    listAlertRules: vi
      .fn()
      .mockResolvedValue({ items: [rule], page: { has_more: false, limit: 100 } }),
    listAlerts: vi.fn(),
    listAudit: vi.fn(),
    listEnvironments: vi.fn(),
    listEvents: vi.fn(),
    listLogs: vi.fn(),
    listMetricHistory: vi.fn(),
    listMetrics: vi.fn(),
    listNotifications: vi.fn(),
    listResources: vi.fn(),
    listSources: vi.fn(),
    listTopology: vi.fn(),
    replaceAlertRule: vi.fn(),
    testNotification: vi.fn(),
    ...overrides,
  };
}

describe('alert rule settings', () => {
  afterEach(() => {
    window.history.replaceState(null, '', '/');
  });

  it('keeps rule bookmarks and back navigation bound to the router history', async () => {
    const second = { ...rule, id: 'rule_second', display_name: 'Second rule' };
    const listAlertRules = vi.fn<ProductApi['listAlertRules']>().mockImplementation((options) =>
      Promise.resolve({
        items: [options?.cursor === 'cGFnZTI' ? second : rule],
        page: { has_more: false, limit: 100 },
      }),
    );
    const history = createMemoryHistory({
      initialEntries: ['/settings/notifications?rule_cursor=cGFnZTI'],
    });
    const router = createAppRouter(
      history,
      createAuthenticatedAuthStore(),
      createTestUsersApi(),
      productApi({ listAlertRules }),
    );
    await router.load();
    render(<App router={router} />);
    await waitFor(() => {
      expect(listAlertRules).toHaveBeenCalled();
    });
    expect(listAlertRules).toHaveBeenLastCalledWith(expect.objectContaining({ cursor: 'cGFnZTI' }));
    expect(await screen.findByRole('heading', { name: 'Second rule' })).not.toBeNull();
    act(() => {
      history.push('/settings/notifications');
    });
    expect(await screen.findByRole('heading', { name: 'Provider lost' })).not.toBeNull();
    act(() => {
      history.back();
    });
    expect(await screen.findByRole('heading', { name: 'Second rule' })).not.toBeNull();
    expect(history.location.search).toBe('?rule_cursor=cGFnZTI');
  });

  it('accepts fractional metric thresholds through native form validation', async () => {
    const user = userEvent.setup();
    render(
      <LocaleProvider>
        <ProductProvider api={productApi()}>
          <ProductAlertRulesSettingsPage />
        </ProductProvider>
      </LocaleProvider>,
    );
    await user.selectOptions(screen.getByLabelText('Rule kind'), 'cpu_threshold');
    const threshold = screen.getByLabelText<HTMLInputElement>('Threshold value');
    await user.clear(threshold);
    await user.type(threshold, '0.5');
    expect(threshold.validity.stepMismatch).toBe(false);
    expect(threshold.checkValidity()).toBe(true);
  });

  it('shows one rule page and navigates through URL-backed cursors', async () => {
    const secondRule = { ...rule, id: 'rule_second', display_name: 'Second rule' };
    const listAlertRules = vi
      .fn()
      .mockResolvedValueOnce({
        items: [rule],
        page: { has_more: true, limit: 100, next_cursor: 'rule_page_two' },
      })
      .mockResolvedValueOnce({ items: [secondRule], page: { has_more: false, limit: 100 } })
      .mockResolvedValueOnce({
        items: [rule],
        page: { has_more: true, limit: 100, next_cursor: 'rule_page_two' },
      });
    const user = userEvent.setup();
    render(
      <LocaleProvider>
        <ProductProvider api={productApi({ listAlertRules })}>
          <ProductAlertRulesSettingsPage />
        </ProductProvider>
      </LocaleProvider>,
    );
    expect(await screen.findByRole('heading', { name: 'Provider lost' })).not.toBeNull();
    await user.click(screen.getByRole('button', { name: 'Next rules' }));
    expect(await screen.findByRole('heading', { name: 'Second rule' })).not.toBeNull();
    expect(screen.queryByRole('heading', { name: 'Provider lost' })).toBeNull();
    expect(new URLSearchParams(window.location.search).get('rule_cursor')).toBe('rule_page_two');
    expect(listAlertRules).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ cursor: 'rule_page_two', limit: 100 }),
    );
    expect(screen.getByRole<HTMLButtonElement>('button', { name: 'Next rules' }).disabled).toBe(
      true,
    );
    await user.click(screen.getByRole('button', { name: 'Previous rules' }));
    expect(await screen.findByRole('heading', { name: 'Provider lost' })).not.toBeNull();
    expect(new URLSearchParams(window.location.search).has('rule_cursor')).toBe(false);
  });

  it('reads a bookmarked page, reacts to browser navigation and ignores an old response', async () => {
    window.history.replaceState(null, '', '/settings/alerts?rule_cursor=bookmark_page');
    let resolveOld:
      | ((value: { items: AlertRuleRecord[]; page: { has_more: false; limit: number } }) => void)
      | undefined;
    const listAlertRules = vi
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveOld = resolve;
          }),
      )
      .mockResolvedValueOnce({ items: [rule], page: { has_more: false, limit: 100 } });
    render(
      <LocaleProvider>
        <ProductProvider api={productApi({ listAlertRules })}>
          <ProductAlertRulesSettingsPage />
        </ProductProvider>
      </LocaleProvider>,
    );
    await waitFor(() => {
      expect(listAlertRules).toHaveBeenCalledWith(
        expect.objectContaining({ cursor: 'bookmark_page' }),
      );
    });
    act(() => {
      window.history.replaceState(null, '', '/settings/alerts');
      window.dispatchEvent(new PopStateEvent('popstate'));
    });
    expect(await screen.findByRole('heading', { name: 'Provider lost' })).not.toBeNull();
    act(() => {
      resolveOld?.({
        items: [{ ...rule, display_name: 'Old page' }],
        page: { has_more: false, limit: 100 },
      });
    });
    expect(screen.queryByRole('heading', { name: 'Old page' })).toBeNull();
    expect((listAlertRules.mock.calls[0]?.[0] as { signal: AbortSignal }).signal.aborted).toBe(
      true,
    );
  });

  it('keeps a failed next page honest and allows retry without losing the list', async () => {
    const listAlertRules = vi
      .fn()
      .mockResolvedValueOnce({
        items: [rule],
        page: { has_more: true, limit: 100, next_cursor: 'next_page' },
      })
      .mockRejectedValueOnce(new Error('Unavailable'))
      .mockResolvedValueOnce({ items: [], page: { has_more: false, limit: 100 } });
    const user = userEvent.setup();
    render(
      <LocaleProvider>
        <ProductProvider api={productApi({ listAlertRules })}>
          <ProductAlertRulesSettingsPage />
        </ProductProvider>
      </LocaleProvider>,
    );
    expect(await screen.findByRole('heading', { name: 'Provider lost' })).not.toBeNull();
    await user.click(screen.getByRole('button', { name: 'Next rules' }));
    await user.click(await screen.findByRole('button', { name: 'Reload rules' }));
    await waitFor(() => {
      expect(screen.queryByRole('heading', { name: 'Provider lost' })).toBeNull();
    });
    expect(listAlertRules).toHaveBeenNthCalledWith(
      3,
      expect.objectContaining({ cursor: 'next_page' }),
    );
  });

  it('shows secret-free outbound delivery state', async () => {
    const api = productApi({
      getNotificationSettings: vi.fn().mockResolvedValue({
        adapter: 'generic_webhook',
        matrix: { adapter: 'matrix', enabled: false },
        configuration_source: 'operator_environment',
        delivery_semantics: 'at_least_once',
        destination_reference_id: 'webhook_primary',
        enabled: true,
      }),
    });
    render(
      <LocaleProvider>
        <ProductProvider api={api}>
          <ProductAlertRulesSettingsPage />
        </ProductProvider>
      </LocaleProvider>,
    );

    expect(await screen.findByText('webhook_primary')).not.toBeNull();
    expect(screen.getByRole('heading', { name: 'Outbound delivery' })).not.toBeNull();
    expect(
      screen.getByText(
        'The full destination URL and signing key stay outside the browser and API.',
      ),
    ).not.toBeNull();
  });

  it('requires the current password and queues one enabled test delivery', async () => {
    const testNotification = vi.fn().mockResolvedValue({
      destination_reference_id: 'webhook_primary',
      notification_id: 'notification_test_01',
      state: 'queued',
    });
    const api = productApi({
      getNotificationSettings: vi.fn().mockResolvedValue({
        adapter: 'generic_webhook',
        matrix: { adapter: 'matrix', enabled: false },
        configuration_source: 'operator_environment',
        delivery_semantics: 'at_least_once',
        destination_reference_id: 'webhook_primary',
        enabled: true,
      }),
      testNotification,
    });
    const user = userEvent.setup();
    render(
      <LocaleProvider>
        <ProductProvider api={api}>
          <ProductAlertRulesSettingsPage />
        </ProductProvider>
      </LocaleProvider>,
    );

    const password = await screen.findByLabelText('Your current password');
    await user.type(password, 'admin-password');
    await user.click(screen.getByRole('button', { name: 'Send test' }));

    expect(await screen.findByText(/Test delivery queued: notification_test_01/u)).not.toBeNull();
    expect(testNotification).toHaveBeenCalledWith('admin-password');
    expect((password as HTMLInputElement).value).toBe('');
  });

  it('does not offer a test when outbound delivery is disabled', async () => {
    render(
      <LocaleProvider>
        <ProductProvider api={productApi()}>
          <ProductAlertRulesSettingsPage />
        </ProductProvider>
      </LocaleProvider>,
    );

    expect(await screen.findAllByText('Not configured')).toHaveLength(2);
    expect(screen.queryByRole('button', { name: 'Send test' })).toBeNull();
  });

  it('selects Matrix when only that lane is configured', async () => {
    const testNotification = vi.fn().mockResolvedValue({
      destination_reference_id: 'matrix_primary',
      notification_id: 'notification_matrix_test',
      state: 'queued',
    });
    const api = productApi({
      getNotificationSettings: vi.fn().mockResolvedValue({
        adapter: 'generic_webhook',
        enabled: false,
        matrix: { adapter: 'matrix', enabled: true, destination_reference_id: 'matrix_primary' },
        configuration_source: 'operator_environment',
        delivery_semantics: 'at_least_once',
      }),
      testNotification,
    });
    const user = userEvent.setup();
    render(
      <LocaleProvider>
        <ProductProvider api={api}>
          <ProductAlertRulesSettingsPage />
        </ProductProvider>
      </LocaleProvider>,
    );
    expect(await screen.findByText('matrix_primary')).not.toBeNull();
    expect(screen.getByLabelText<HTMLSelectElement>('Test destination').value).toBe('matrix');
    await user.type(screen.getByLabelText('Your current password'), 'admin-password');
    await user.click(screen.getByRole('button', { name: 'Send test' }));
    await waitFor(() => {
      expect(testNotification).toHaveBeenCalledWith('admin-password', 'matrix');
    });
  });

  it('lists and disables a persisted rule at its current version', async () => {
    const replaceAlertRule = vi.fn().mockResolvedValue({ ...rule, enabled: false, version: 2 });
    const api = productApi({ replaceAlertRule });
    const user = userEvent.setup();
    render(
      <LocaleProvider>
        <ProductProvider api={api}>
          <ProductAlertRulesSettingsPage />
        </ProductProvider>
      </LocaleProvider>,
    );

    const heading = await screen.findByRole('heading', { name: 'Provider lost' });
    const card = heading.closest('article');
    if (card === null) throw new Error('Alert rule card is missing.');
    await user.click(screen.getByRole('button', { name: 'Disable' }));

    await waitFor(() => {
      expect(within(card).getByText('Disabled')).not.toBeNull();
    });
    expect(replaceAlertRule).toHaveBeenCalledWith(
      'rule_provider_lost',
      1,
      expect.objectContaining({ enabled: false, kind: 'provider_connection_lost' }),
    );
  });
});
