import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { LocaleProvider, type Locale } from '../../i18n';
import { createAuthenticatedAuthStore } from '../../test/auth';
import { AuthProvider } from '../auth/auth-context';
import { createProductApi, type ProductApi } from '../product/product-api';
import { ProductProvider } from '../product/product-context';
import { ApplicationSettingsPage } from './ApplicationSettingsPage';
import { SettingsUncertainError } from './settings-api';
import {
  cleanupPreviewFixture,
  cleanupResultFixture,
  settingsFixture,
  settingsPreviewFixture,
} from './settings-test-fixtures';

function renderSettings(
  role: 'administrator' | 'viewer' = 'administrator',
  overrides: Partial<ProductApi> = {},
  locale: Locale = 'en',
) {
  const api: ProductApi = {
    ...createProductApi({
      request: () => Promise.reject(new Error('Unexpected settings transport call.')),
    }),
    getApplicationSettings: vi
      .fn<ProductApi['getApplicationSettings']>()
      .mockResolvedValue(settingsFixture),
    previewApplicationSettings: vi
      .fn<ProductApi['previewApplicationSettings']>()
      .mockResolvedValue(settingsPreviewFixture),
    updateApplicationSettings: vi.fn<ProductApi['updateApplicationSettings']>().mockResolvedValue({
      ...settingsFixture,
      version: 2,
      values: settingsPreviewFixture.requested,
    }),
    previewCleanup: vi.fn<ProductApi['previewCleanup']>().mockResolvedValue(cleanupPreviewFixture),
    cleanupData: vi.fn<ProductApi['cleanupData']>().mockResolvedValue(cleanupResultFixture),
    ...overrides,
  };
  render(
    <LocaleProvider defaultLocale={locale}>
      <AuthProvider store={createAuthenticatedAuthStore(role)}>
        <ProductProvider api={api}>
          <ApplicationSettingsPage />
        </ProductProvider>
      </AuthProvider>
    </LocaleProvider>,
  );
  return api;
}

async function confirmDialog(user: ReturnType<typeof userEvent.setup>, dialog: HTMLElement) {
  for (const checkbox of within(dialog).getAllByRole('checkbox')) await user.click(checkbox);
  await user.type(within(dialog).getByLabelText(/^Current password/u), 'fixture-password-123');
}

describe('application settings and destructive reviews', () => {
  it('shows effective settings and gates to a viewer with no mutation controls', async () => {
    renderSettings('viewer');
    expect(await screen.findByRole('heading', { name: 'Application policy' })).not.toBeNull();
    expect(screen.getByText('You can inspect these settings.', { exact: false })).not.toBeNull();
    expect(screen.getByText('1.0.2').getAttribute('dir')).toBe('ltr');
    expect(screen.queryByRole('button', { name: 'Review changes' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Generate configuration export' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Review cleanup' })).toBeNull();
    expect(screen.queryByRole('spinbutton')).toBeNull();
  });

  it('reviews the full ongoing retention effect, freezes scope and submits once with a cleared password', async () => {
    const user = userEvent.setup();
    let finish: ((value: typeof settingsFixture) => void) | undefined;
    const update = vi.fn<ProductApi['updateApplicationSettings']>().mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    renderSettings('administrator', { updateApplicationSettings: update });
    const input = await screen.findByLabelText('Metric retention (hours)');
    fireEvent.change(input, { target: { value: '1' } });
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    const dialog = await screen.findByRole('dialog', { name: 'Review application changes' });
    expect(await within(dialog).findByText('At least 10,001')).not.toBeNull();
    expect(within(dialog).getByText(/not a limit on the total history/u)).not.toBeNull();
    expect(
      within(dialog).getByRole<HTMLButtonElement>('button', { name: 'Confirm action' }).disabled,
    ).toBe(true);
    await confirmDialog(user, dialog);
    const password = within(dialog).getByLabelText<HTMLInputElement>(/^Current password/u);
    const submit = within(dialog).getByRole<HTMLButtonElement>('button', {
      name: 'Confirm action',
    });
    await user.click(submit);
    expect(password.value).toBe('');
    expect(update).toHaveBeenCalledTimes(1);
    expect(update).toHaveBeenCalledWith(settingsPreviewFixture, 'fixture-password-123', true);
    await user.click(submit);
    expect(update).toHaveBeenCalledTimes(1);
    await act(async () => {
      finish?.({ ...settingsFixture, version: 2, values: settingsPreviewFixture.requested });
      await Promise.resolve();
    });
    expect(await screen.findByText('Application settings updated.')).not.toBeNull();
    expect(update).toHaveBeenCalledTimes(1);
  });

  it('keeps an uncertain action spent and waits for an explicit reload', async () => {
    const user = userEvent.setup();
    const update = vi.fn().mockRejectedValue(new SettingsUncertainError());
    const getSettings = vi.fn().mockResolvedValue(settingsFixture);
    renderSettings('administrator', {
      getApplicationSettings: getSettings,
      updateApplicationSettings: update,
    });
    fireEvent.change(await screen.findByLabelText('Metric retention (hours)'), {
      target: { value: '1' },
    });
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    const dialog = await screen.findByRole('dialog', { name: 'Review application changes' });
    await within(dialog).findByText('At least 10,001');
    await confirmDialog(user, dialog);
    await user.click(within(dialog).getByRole('button', { name: 'Confirm action' }));
    expect(await within(dialog).findByRole('alert')).toHaveProperty(
      'textContent',
      expect.stringContaining('result is unknown'),
    );
    expect(
      within(dialog).getByRole<HTMLButtonElement>('button', { name: 'Confirm action' }).disabled,
    ).toBe(true);
    expect(update).toHaveBeenCalledTimes(1);
    expect(getSettings).toHaveBeenCalledTimes(1);
    const close = within(dialog).getAllByRole('button', { name: 'Close review' });
    await user.click(close[close.length - 1] as HTMLElement);
    await user.click(screen.getByRole('button', { name: 'Reload settings' }));
    await waitFor(() => {
      expect(getSettings).toHaveBeenCalledTimes(2);
    });
    expect(update).toHaveBeenCalledTimes(1);
  });

  it('reports a bounded cleanup batch with remaining work and does not run a second batch', async () => {
    const user = userEvent.setup();
    const cleanupData = vi
      .fn()
      .mockResolvedValue({ ...cleanupResultFixture, removed_rows: 1000, remaining_rows: true });
    renderSettings('administrator', { cleanupData });
    await screen.findByRole('heading', { name: 'Application policy' });
    await user.click(screen.getByRole('button', { name: 'Review cleanup' }));
    const dialog = await screen.findByRole('dialog', { name: 'Review history cleanup' });
    await within(dialog).findByText('25');
    expect(within(dialog).getByText(/Verify a usable backup/u)).not.toBeNull();
    await confirmDialog(user, dialog);
    await user.click(within(dialog).getByRole('button', { name: 'Confirm action' }));
    expect(await screen.findByText('Removed 1,000 eligible history rows.')).not.toBeNull();
    expect(screen.getByText(/More eligible rows remain/u)).not.toBeNull();
    expect(cleanupData).toHaveBeenCalledTimes(1);
  });

  it('rejects invalid numeric input before a preview request', async () => {
    const user = userEvent.setup();
    const preview = vi.fn();
    renderSettings('administrator', { previewApplicationSettings: preview });
    fireEvent.change(await screen.findByLabelText('Metric retention (hours)'), {
      target: { value: '0' },
    });
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    expect(screen.getByRole('alert').textContent).toContain('valid whole numbers');
    expect(preview).not.toHaveBeenCalled();
  });

  it('prevents an expired preview from requesting a fresh password or committing', async () => {
    const user = userEvent.setup();
    const preview = vi
      .fn()
      .mockResolvedValue({ ...settingsPreviewFixture, expires_at: '2000-08-16T08:00:00Z' });
    const update = vi.fn();
    renderSettings('administrator', {
      previewApplicationSettings: preview,
      updateApplicationSettings: update,
    });
    fireEvent.change(await screen.findByLabelText('Metric retention (hours)'), {
      target: { value: '1' },
    });
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    const dialog = await screen.findByRole('dialog', { name: 'Review application changes' });
    expect(await within(dialog).findByRole('alert')).toHaveProperty(
      'textContent',
      expect.stringContaining('preview expired'),
    );
    expect(within(dialog).getByLabelText<HTMLInputElement>(/^Current password/u).disabled).toBe(
      true,
    );
    expect(update).not.toHaveBeenCalled();
  });

  it('uses Persian readable labels and isolates application version in RTL', async () => {
    renderSettings('viewer', {}, 'fa');
    expect(await screen.findByRole('heading', { name: 'سیاست برنامه' })).not.toBeNull();
    expect(document.documentElement.dir).toBe('rtl');
    expect(screen.getByText('1.0.2').getAttribute('dir')).toBe('ltr');
    expect(screen.getByText('روزهای نگهداری')).not.toBeNull();
  });
});
