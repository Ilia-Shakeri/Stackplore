import type { AlertRuleRecord } from '@stackplore/api-client';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { ProductAlertRulesSettingsPage } from './AlertRulesSettingsPage';
import type { ProductApi } from './product-api';
import { ProductProvider } from './product-context';
import { LocaleProvider } from '../../i18n';

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
