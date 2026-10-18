import { render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AuthProvider } from '../auth/auth-context';
import { LocaleProvider } from '../../i18n';
import { createAuthenticatedAuthStore } from '../../test/auth';
import { ProductAlertsPage } from './ExplorationAlertsPage';
import { ProductLogsPage } from './ExplorationLogsPage';
import { ProductNotificationsPage } from './ProductHistoryPages';
import { createProductApi, type ProductApi } from './product-api';
import { ProductProvider } from './product-context';

const observedAt = '2026-08-16T08:00:00Z';
const severities = ['trace', 'debug', 'info', 'warning', 'error', 'critical', 'unknown'] as const;
const labels = {
  en: ['Trace', 'Debug', 'Information', 'Warning', 'Error', 'Critical', 'Unknown'],
  fa: ['ردیابی', 'اشکال‌زدایی', 'اطلاعات', 'هشدار', 'خطا', 'بحرانی', 'نامشخص'],
} as const;

function productApi(): ProductApi {
  return {
    ...createProductApi({
      request: () => Promise.reject(new Error('Unexpected severity test request.')),
    }),
    listSources: vi.fn().mockResolvedValue([]),
    listLogs: vi.fn().mockResolvedValue({
      availability: 'available',
      completeness: 'complete',
      observed_at: observedAt,
      items: severities.map((severity, index) => ({
        id: `log_${severity}`,
        resource_id: `r1_${'A'.repeat(43)}`,
        stream: 'stdout',
        sequence: index,
        severity,
        timestamp: observedAt,
        message: `Line ${String(index)}`,
        redacted: false,
        truncated: false,
        synthetic: true,
      })),
      page: { has_more: false, limit: 100 },
    }),
    listAlerts: vi.fn().mockResolvedValue({
      items: [
        {
          id: 'alert_test',
          rule_id: 'rule_test',
          severity: 'critical',
          started_at: observedAt,
          updated_at: observedAt,
          state: 'firing',
          summary_code: 'alert.resource_unhealthy.matched',
          synthetic: true,
        },
      ],
      page: { has_more: false, limit: 100 },
    }),
    listNotifications: vi.fn().mockResolvedValue({
      items: [
        {
          id: 'notification_test',
          kind: 'alert.firing',
          severity: 'critical',
          summary_code: 'alert.resource_unhealthy.matched',
          created_at: observedAt,
          synthetic: true,
        },
      ],
      page: { has_more: false, limit: 100 },
    }),
  };
}

afterEach(() => {
  window.history.replaceState(null, '', '/');
});

describe('localized observation severity', () => {
  it.each(['en', 'fa'] as const)(
    'labels every log level in %s while keeping raw option values',
    async (locale) => {
      window.history.replaceState(null, '', '/logs?tail=paused');
      const { container } = render(
        <LocaleProvider defaultLocale={locale}>
          <ProductProvider api={productApi()}>
            <ProductLogsPage />
          </ProductProvider>
        </LocaleProvider>,
      );
      await screen.findByText('Line 0');
      const chips = container.querySelectorAll('.severity-chip');
      expect([...chips].map((chip) => chip.textContent)).toEqual(labels[locale]);
      const select = screen.getByRole('combobox', { name: locale === 'en' ? 'Severity' : 'شدت' });
      expect(
        within(select)
          .getAllByRole<HTMLOptionElement>('option')
          .slice(1)
          .map((option) => option.value),
      ).toEqual(severities);
      expect(
        within(select)
          .getAllByRole('option')
          .slice(1)
          .map((option) => option.textContent),
      ).toEqual(labels[locale]);
    },
  );

  it.each(['en', 'fa'] as const)('labels alert and notification severity in %s', async (locale) => {
    const { container } = render(
      <LocaleProvider defaultLocale={locale}>
        <AuthProvider store={createAuthenticatedAuthStore()}>
          <ProductProvider api={productApi()}>
            <ProductAlertsPage />
            <ProductNotificationsPage />
          </ProductProvider>
        </AuthProvider>
      </LocaleProvider>,
    );
    expect(await screen.findAllByText(labels[locale][5])).toHaveLength(2);
    expect(container.querySelector('.alert-list .severity-chip')?.textContent).toBe(
      labels[locale][5],
    );
    expect(container.querySelector('.notification-list .severity-chip')?.textContent).toBe(
      labels[locale][5],
    );
    expect(screen.queryByText('critical')).toBeNull();
  });
});
