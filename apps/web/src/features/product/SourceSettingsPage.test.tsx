import { ApiProblemError, type SourceSettingsRecord } from '@stackplore/api-client';
import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import axe from 'axe-core';
import { describe, expect, it, vi } from 'vitest';

import { SourceSettingsPage } from './SourceSettingsPage';
import { createProductApi, type ProductApi } from './product-api';
import { ProductProvider } from './product-context';
import { SourceSettingsUncertainError } from './source-settings-api';
import { App } from '../../app/App';
import { createAppRouter } from '../../app/router';
import { AuthProvider } from '../auth/auth-context';
import { LocaleProvider } from '../../i18n';
import { createAuthenticatedAuthStore } from '../../test/auth';
import { createTestUsersApi } from '../../test/users';

const source: SourceSettingsRecord = {
  id: 'source_docker',
  provider: 'docker',
  name: 'Local Docker',
  synthetic: false,
  enabled: true,
  version: 1,
  profile_available: true,
};
const page = (items: SourceSettingsRecord[]) => ({
  items,
  page: { limit: 50, has_more: false as const },
});
const rejected = (code: string, status = 409) =>
  new ApiProblemError({
    code,
    status,
    type: `urn:stackplore:problem:${code}`,
    title: 'Request rejected',
    request_id: 'request_source_test',
  });

function createSettingsApi(overrides: Partial<ProductApi> = {}) {
  let previews = 0;
  const api: ProductApi = {
    ...createProductApi({
      request: () => Promise.reject(new Error('Unexpected source-page transport call.')),
    }),
    listSourceSettings: vi.fn<ProductApi['listSourceSettings']>().mockResolvedValue(page([source])),
    previewSourceUpdate: vi
      .fn<ProductApi['previewSourceUpdate']>()
      .mockImplementation((target, enabled) => {
        previews++;
        return Promise.resolve({
          source: target,
          requested_enabled: enabled,
          impact: enabled ? 'collection.start' : 'collection.stop',
          confirmation_token: `${(previews === 1 ? 'A' : 'B').repeat(42)}A`,
          expires_at: '2099-01-02T03:04:05Z',
        });
      }),
    updateSource: vi.fn<ProductApi['updateSource']>().mockImplementation((preview) =>
      Promise.resolve({
        ...preview.source,
        enabled: preview.requested_enabled,
        version: preview.source.version + 1,
      }),
    ),
    ...overrides,
  };
  return api;
}

function renderSettings(
  api: ProductApi,
  locale: 'en' | 'fa' = 'en',
  role: 'administrator' | 'viewer' = 'administrator',
) {
  return render(
    <LocaleProvider defaultLocale={locale}>
      <AuthProvider store={createAuthenticatedAuthStore(role)}>
        <ProductProvider api={api}>
          <SourceSettingsPage />
        </ProductProvider>
      </AuthProvider>
    </LocaleProvider>,
  );
}

async function confirmChange(enabled = false) {
  const user = userEvent.setup();
  const dialog = await screen.findByRole('dialog', {
    name: enabled ? 'Enable source collection' : 'Disable source collection',
  });
  await user.click(
    within(dialog).getByRole('checkbox', {
      name: enabled
        ? 'I confirm enabling collection for Local Docker.'
        : 'I confirm disabling collection for Local Docker.',
    }),
  );
  await user.type(within(dialog).getByLabelText(/^Current password/u), 'invented-password');
  await user.click(within(dialog).getByRole('button', { name: 'Confirm change' }));
}

describe('operator source settings', () => {
  it.each([false, true])(
    'requires exact preview, explicit confirmation and password for enabled=%s',
    async (enabled) => {
      const user = userEvent.setup();
      const initial = { ...source, enabled: !enabled };
      const listSourceSettings = vi
        .fn<ProductApi['listSourceSettings']>()
        .mockResolvedValueOnce(page([initial]))
        .mockResolvedValueOnce(page([{ ...source, enabled, version: 2 }]));
      const api = createSettingsApi({ listSourceSettings });
      renderSettings(api);
      await user.click(
        await screen.findByRole('button', {
          name: enabled
            ? 'Enable collection for Local Docker'
            : 'Disable collection for Local Docker',
        }),
      );
      const dialog = await screen.findByRole('dialog', {
        name: enabled ? 'Enable source collection' : 'Disable source collection',
      });
      const apply = within(dialog).getByRole<HTMLButtonElement>('button', {
        name: 'Confirm change',
      });
      expect(apply.disabled).toBe(true);
      expect(within(dialog).getByText('source_docker').getAttribute('dir')).toBe('ltr');
      expect(
        within(dialog).getByText(
          enabled
            ? 'Allow new collection and reads from this operator profile.'
            : 'Stop new collection and reads from this profile. Existing observations remain stored.',
        ),
      ).not.toBeNull();
      expect(api.updateSource).not.toHaveBeenCalled();
      expect(dialog.textContent).not.toContain('A'.repeat(43));
      await confirmChange(enabled);
      expect(
        await screen.findByText('Collection setting updated for Local Docker.'),
      ).not.toBeNull();
      expect(
        await screen.findByRole('button', {
          name: enabled
            ? 'Disable collection for Local Docker'
            : 'Enable collection for Local Docker',
        }),
      ).not.toBeNull();
      const previewCall = vi.mocked(api.previewSourceUpdate).mock.calls[0];
      expect(previewCall?.[0]).toEqual(initial);
      expect(previewCall?.[1]).toBe(enabled);
      expect(previewCall?.[2]).toBeInstanceOf(AbortSignal);
      expect(api.updateSource).toHaveBeenCalledTimes(1);
      expect(api.updateSource).toHaveBeenCalledWith(
        {
          requested_enabled: enabled,
          source: initial,
          impact: enabled ? 'collection.start' : 'collection.stop',
          confirmation_token: 'A'.repeat(43),
          expires_at: '2099-01-02T03:04:05Z',
        },
        'invented-password',
      );
      expect(screen.queryByRole('dialog')).toBeNull();
    },
  );

  it('keeps viewer route denied without reading source settings', async () => {
    const api = createSettingsApi();
    const history = createMemoryHistory({ initialEntries: ['/settings/connections'] });
    const router = createAppRouter(
      history,
      createAuthenticatedAuthStore('viewer'),
      createTestUsersApi(),
      api,
    );
    render(<App router={router} />);
    expect(await screen.findByRole('heading', { level: 1, name: 'Connections' })).not.toBeNull();
    expect(screen.getByRole('alert').textContent).toContain(
      'Permission or recent authentication was denied',
    );
    expect(api.listSourceSettings).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: /collection for/u })).toBeNull();
  });

  it('cannot enable missing operator profiles, but can review disabling an enabled orphan', async () => {
    const user = userEvent.setup();
    const missingOff = {
      ...source,
      id: 'source_missing',
      name: 'Missing profile',
      enabled: false,
      profile_available: false,
    };
    const missingOn = {
      ...source,
      id: 'source_orphan',
      name: 'Orphan profile',
      enabled: true,
      profile_available: false,
    };
    const api = createSettingsApi({
      listSourceSettings: vi
        .fn<ProductApi['listSourceSettings']>()
        .mockResolvedValue(page([missingOff, missingOn])),
    });
    renderSettings(api);
    expect(
      (
        await screen.findByRole<HTMLButtonElement>('button', {
          name: 'Enable collection for Missing profile',
        })
      ).disabled,
    ).toBe(true);
    const disable = screen.getByRole<HTMLButtonElement>('button', {
      name: 'Disable collection for Orphan profile',
    });
    expect(disable.disabled).toBe(false);
    await user.click(disable);
    expect(await screen.findByRole('dialog', { name: 'Disable source collection' })).not.toBeNull();
    const previewCall = vi.mocked(api.previewSourceUpdate).mock.calls[0];
    expect(previewCall?.[0]).toEqual(missingOn);
    expect(previewCall?.[1]).toBe(false);
    expect(previewCall?.[2]).toBeInstanceOf(AbortSignal);
  });

  it.each([
    ['source.version_conflict', 'This profile changed.'],
    ['source.confirmation_rejected', 'This confirmation expired or was already used.'],
  ])('spends local confirmation after %s and requires a fresh review', async (code, message) => {
    const user = userEvent.setup();
    const fresh = { ...source, version: 2 };
    const updateSource = vi
      .fn<ProductApi['updateSource']>()
      .mockRejectedValueOnce(rejected(code))
      .mockResolvedValueOnce({ ...fresh, enabled: false, version: 3 });
    const listSourceSettings = vi
      .fn<ProductApi['listSourceSettings']>()
      .mockResolvedValueOnce(page([source]))
      .mockResolvedValueOnce(page([fresh]))
      .mockResolvedValueOnce(page([{ ...fresh, enabled: false, version: 3 }]));
    const api = createSettingsApi({ updateSource, listSourceSettings });
    renderSettings(api);
    await user.click(
      await screen.findByRole('button', { name: 'Disable collection for Local Docker' }),
    );
    await confirmChange();
    const alert = await screen.findByRole('alert');
    expect(alert.textContent).toContain(message);
    const dialog = screen.getByRole('dialog');
    const password = within(dialog).getByLabelText<HTMLInputElement>(/^Current password/u);
    expect(password.value).toBe('');
    expect(password.disabled).toBe(true);
    expect(within(dialog).queryByRole('button', { name: 'Confirm change' })).toBeNull();
    expect(updateSource).toHaveBeenCalledTimes(1);
    await user.click(within(dialog).getByRole('button', { name: 'Reload and review again' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull();
    });
    await user.click(
      await screen.findByRole('button', { name: 'Disable collection for Local Docker' }),
    );
    await confirmChange();
    expect(await screen.findByText('Collection setting updated for Local Docker.')).not.toBeNull();
    expect(updateSource).toHaveBeenCalledTimes(2);
    expect(updateSource).toHaveBeenLastCalledWith(
      {
        source: fresh,
        requested_enabled: false,
        impact: 'collection.stop',
        confirmation_token: `${'B'.repeat(42)}A`,
        expires_at: '2099-01-02T03:04:05Z',
      },
      'invented-password',
    );
  });

  it('shows uncertain result without claiming success or retrying', async () => {
    const user = userEvent.setup();
    const updateSource = vi
      .fn<ProductApi['updateSource']>()
      .mockRejectedValue(new SourceSettingsUncertainError());
    const api = createSettingsApi({ updateSource });
    renderSettings(api);
    await user.click(
      await screen.findByRole('button', { name: 'Disable collection for Local Docker' }),
    );
    await confirmChange();
    expect((await screen.findByRole('alert')).textContent).toContain(
      'The update result is unknown.',
    );
    expect(screen.queryByText('Collection setting updated for Local Docker.')).toBeNull();
    expect(updateSource).toHaveBeenCalledTimes(1);
  });

  it('blocks locally expired confirmation before requesting a write', async () => {
    const user = userEvent.setup();
    const api = createSettingsApi({
      previewSourceUpdate: vi.fn<ProductApi['previewSourceUpdate']>().mockResolvedValue({
        source,
        requested_enabled: false,
        impact: 'collection.stop',
        confirmation_token: 'A'.repeat(43),
        expires_at: '2000-01-02T03:04:05Z',
      }),
    });
    renderSettings(api);
    await user.click(
      await screen.findByRole('button', { name: 'Disable collection for Local Docker' }),
    );
    const dialog = await screen.findByRole('dialog');
    expect((await within(dialog).findByRole('alert')).textContent).toContain(
      'This confirmation expired',
    );
    expect(within(dialog).queryByRole('button', { name: 'Confirm change' })).toBeNull();
    expect(api.updateSource).not.toHaveBeenCalled();
  });

  it('keeps a pending update locked when confirmation expires', async () => {
    const user = userEvent.setup();
    let finishUpdate: (value: SourceSettingsRecord) => void = () => {
      throw new Error('Source write did not start.');
    };
    const updateSource = vi.fn<ProductApi['updateSource']>().mockImplementation(
      () =>
        new Promise<SourceSettingsRecord>((resolve) => {
          finishUpdate = resolve;
        }),
    );
    const api = createSettingsApi({ updateSource });
    renderSettings(api);
    await user.click(
      await screen.findByRole('button', { name: 'Disable collection for Local Docker' }),
    );
    await confirmChange();
    vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2100-01-02T03:04:05Z'));
    const dialog = screen.getByRole('dialog');
    expect((await within(dialog).findByRole('alert')).textContent).toContain(
      'This confirmation expired',
    );
    expect(within(dialog).queryByRole('button', { name: 'Reload and review again' })).toBeNull();
    expect(within(dialog).getByRole<HTMLButtonElement>('button', { name: 'Cancel' }).disabled).toBe(
      true,
    );
    await user.keyboard('{Escape}');
    expect(screen.getByRole('dialog')).toBe(dialog);
    expect(updateSource).toHaveBeenCalledTimes(1);
    finishUpdate({ ...source, enabled: false, version: 2 });
    expect(await screen.findByText('Collection setting updated for Local Docker.')).not.toBeNull();
  });

  it('keeps stale rows disabled until a failed list refresh succeeds', async () => {
    const user = userEvent.setup();
    const api = createSettingsApi({
      listSourceSettings: vi
        .fn<ProductApi['listSourceSettings']>()
        .mockResolvedValueOnce(page([source]))
        .mockRejectedValueOnce(rejected('temporarily_unavailable', 503))
        .mockResolvedValueOnce(page([{ ...source, version: 2 }])),
    });
    renderSettings(api);
    await screen.findByRole('heading', { name: 'Local Docker', level: 2 });
    await user.click(screen.getByRole('button', { name: 'Reload profiles' }));
    expect((await screen.findByRole('alert')).textContent).toContain('unavailable');
    const trigger = screen.getByRole<HTMLButtonElement>('button', {
      name: 'Disable collection for Local Docker',
    });
    expect(trigger.disabled).toBe(true);
    await user.click(screen.getByRole('button', { name: 'Reload profiles' }));
    await waitFor(() => {
      expect(trigger.disabled).toBe(false);
    });
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('requires profile reload when a profile disappears before preview', async () => {
    const user = userEvent.setup();
    const api = createSettingsApi({
      listSourceSettings: vi
        .fn<ProductApi['listSourceSettings']>()
        .mockResolvedValue(page([{ ...source, enabled: false }])),
      previewSourceUpdate: vi
        .fn<ProductApi['previewSourceUpdate']>()
        .mockRejectedValue(rejected('source.profile_unavailable', 422)),
    });
    renderSettings(api);
    await user.click(
      await screen.findByRole('button', { name: 'Enable collection for Local Docker' }),
    );
    const dialog = await screen.findByRole('dialog');
    expect((await within(dialog).findByRole('alert')).textContent).toContain('profile');
    expect(within(dialog).queryByRole('button', { name: 'Confirm change' })).toBeNull();
    expect(within(dialog).getByRole('button', { name: 'Reload and review again' })).not.toBeNull();
    expect(api.updateSource).not.toHaveBeenCalled();
  });

  it('keeps pages bounded and loads previous cursor state', async () => {
    const user = userEvent.setup();
    const other = { ...source, id: 'source_other', name: 'Other Docker' };
    const first = {
      items: [source],
      page: { limit: 50, has_more: true as const, next_cursor: 'next_profiles' },
    };
    const listSourceSettings = vi
      .fn<ProductApi['listSourceSettings']>()
      .mockResolvedValueOnce(first)
      .mockResolvedValueOnce(page([other]))
      .mockResolvedValueOnce(first);
    const api = createSettingsApi({ listSourceSettings });
    renderSettings(api);
    expect(await screen.findByRole('heading', { name: 'Local Docker', level: 2 })).not.toBeNull();
    await user.click(screen.getByRole('button', { name: 'Next profiles' }));
    expect(await screen.findByRole('heading', { name: 'Other Docker', level: 2 })).not.toBeNull();
    expect(screen.queryByRole('heading', { name: 'Local Docker', level: 2 })).toBeNull();
    expect(listSourceSettings.mock.calls[1]?.[0]?.cursor).toBe('next_profiles');
    expect(listSourceSettings.mock.calls[1]?.[0]?.signal).toBeInstanceOf(AbortSignal);
    await user.click(screen.getByRole('button', { name: 'Previous profiles' }));
    expect(await screen.findByRole('heading', { name: 'Local Docker', level: 2 })).not.toBeNull();
    expect(listSourceSettings.mock.calls[2]?.[0]?.cursor).toBeUndefined();
    expect(listSourceSettings.mock.calls[2]?.[0]?.signal).toBeInstanceOf(AbortSignal);
  });

  it('restores row focus on keyboard cancellation', async () => {
    const user = userEvent.setup();
    const api = createSettingsApi();
    renderSettings(api);
    const trigger = await screen.findByRole('button', {
      name: 'Disable collection for Local Docker',
    });
    await user.click(trigger);
    expect(await screen.findByRole('dialog')).not.toBeNull();
    await user.keyboard('{Escape}');
    await waitFor(() => {
      expect(document.activeElement).toBe(trigger);
    });
    expect(api.updateSource).not.toHaveBeenCalled();
  });

  it.each(['en', 'fa'] as const)(
    'keeps %s source review named, isolated and accessible',
    async (locale) => {
      const user = userEvent.setup();
      const api = createSettingsApi();
      renderSettings(api, locale);
      await user.click(
        await screen.findByRole('button', {
          name:
            locale === 'en'
              ? 'Disable collection for Local Docker'
              : 'غیرفعال‌سازی جمع‌آوری برای Local Docker',
        }),
      );
      const dialog = await screen.findByRole('dialog');
      expect(within(dialog).getByText('source_docker').getAttribute('dir')).toBe('ltr');
      expect(document.documentElement.dir).toBe(locale === 'fa' ? 'rtl' : 'ltr');
      expect(
        within(dialog)
          .getByLabelText(locale === 'en' ? /^Current password/u : /^گذرواژهٔ فعلی/u)
          .getAttribute('dir'),
      ).toBe('ltr');
      const result = await axe.run(document.body, {
        rules: { 'color-contrast': { enabled: false } },
      });
      expect(result.violations).toEqual([]);
    },
  );
});
