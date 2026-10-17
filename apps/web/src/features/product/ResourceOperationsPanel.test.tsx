import type { ProductResource } from '@stackplore/api-client';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { LocaleProvider } from '../../i18n';
import { createAuthenticatedAuthStore } from '../../test/auth';
import { AuthProvider } from '../auth/auth-context';
import type { ProductApi } from './product-api';
import { ProductProvider } from './product-context';
import { OperationUncertainError } from './operation-api';
import { ResourceOperationsPanel } from './ResourceOperationsPanel';

const resource: ProductResource = {
  id: `r1_${'A'.repeat(43)}`,
  source_id: 'source_cluster',
  provider: 'kubernetes',
  environment_id: 'env_cluster',
  kind: 'kubernetes.deployment',
  scope_kind: 'namespace',
  scope_name: 'shop',
  name: 'API',
  synthetic: false,
  observed_at: '2026-01-02T03:04:05Z',
  operation_version: 'kv_before',
};
const preview = {
  source_id: resource.source_id,
  resource_id: resource.id,
  action: 'kubernetes.workload_scale' as const,
  expected_version: 'kv_before',
  parameters: { replicas: 0 },
  target_name: 'API',
  effect_code: 'operation.kubernetes_scale',
  risk_code: 'operation.capacity_change',
  current_replicas: 3,
  confirmation_token: 'A'.repeat(43),
  expires_at: '2099-01-02T03:04:05Z',
};
function mount(
  api: Partial<ProductApi>,
  locale: 'en' | 'fa' = 'en',
  role: 'administrator' | 'viewer' = 'administrator',
  target = resource,
) {
  return render(
    <AuthProvider store={createAuthenticatedAuthStore(role)}>
      <LocaleProvider defaultLocale={locale}>
        <ProductProvider api={api as ProductApi}>
          <ResourceOperationsPanel resource={target} />
        </ProductProvider>
      </LocaleProvider>
    </AuthProvider>,
  );
}

describe('guarded resource actions', () => {
  it.each([
    [
      'en',
      'Guarded actions',
      'Change replicas',
      'Check impact',
      'Review this change',
      'I checked this target and accept this impact.',
      'Current password',
      'Confirm this change',
      'The response was lost.',
    ],
    [
      'fa',
      'عملیات با تأیید',
      'تغییر تعداد نمونه‌ها',
      'بررسی اثر',
      'بازبینی این تغییر',
      'این هدف را بررسی کردم و اثر تغییر را می‌پذیرم.',
      'گذرواژه فعلی',
      'تأیید این تغییر',
      'پاسخ نرسید.',
    ],
  ] as const)(
    'requires impact and acknowledgment and never resends lost reply in %s',
    async (locale, title, scale, check, review, ack, password, confirm, unknown) => {
      const user = userEvent.setup();
      const previewOperation = vi.fn().mockResolvedValue(preview);
      const executeOperation = vi.fn().mockRejectedValue(new OperationUncertainError());
      const listOperations = vi
        .fn()
        .mockResolvedValue({ data: [], page: { limit: 25, has_more: false } });
      mount({ previewOperation, executeOperation, listOperations }, locale);
      expect(screen.getByRole('region', { name: title })).not.toBeNull();
      await user.selectOptions(screen.getByRole('combobox'), 'kubernetes.workload_scale');
      expect(screen.getByRole('option', { name: scale })).not.toBeNull();
      const count = screen.getByRole('spinbutton');
      await user.clear(count);
      await user.type(count, '0');
      await user.click(screen.getByRole('button', { name: check }));
      const heading = await screen.findByRole('heading', { name: review });
      expect(document.activeElement).toBe(heading);
      expect(previewOperation).toHaveBeenCalledWith(
        expect.objectContaining({ parameters: { replicas: 0 } }),
        expect.any(AbortSignal),
      );
      expect(screen.getByRole<HTMLButtonElement>('button', { name: confirm }).disabled).toBe(true);
      await user.click(screen.getByRole('checkbox', { name: ack }));
      await user.type(screen.getByLabelText(password), 'fixture-password');
      await user.click(screen.getByRole('button', { name: confirm }));
      await waitFor(() => {
        expect(screen.getByRole('status').textContent).toContain(unknown);
      });
      expect(executeOperation).toHaveBeenCalledTimes(1);
      expect(screen.queryByLabelText(password)).toBeNull();
      expect(listOperations.mock.calls.length).toBeGreaterThanOrEqual(2);
    },
  );
  it('hides all write controls for viewer/synthetic/unknown-version and forbids DaemonSet scale', () => {
    const listOperations = vi
      .fn()
      .mockResolvedValue({ data: [], page: { limit: 25, has_more: false } });
    const viewer = mount({ listOperations }, 'en', 'viewer');
    expect(screen.queryByRole('region')).toBeNull();
    viewer.unmount();
    const synthetic = mount({ listOperations }, 'en', 'administrator', {
      ...resource,
      synthetic: true,
    });
    expect(screen.queryByRole('region')).toBeNull();
    synthetic.unmount();
    const daemon = mount({ listOperations }, 'en', 'administrator', {
      ...resource,
      kind: 'kubernetes.daemonset',
    });
    expect(screen.queryByRole('option', { name: 'Change replicas' })).toBeNull();
    daemon.unmount();
    expect(listOperations).toHaveBeenCalledTimes(1);
  });
});
