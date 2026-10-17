import { ThemeProvider, TechnicalText } from '@stackplore/ui';
import { act, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { LocaleProvider, useLocale } from '../../i18n';
import {
  FORMAT_PREFERENCES_STORAGE_KEY,
  defaultFormatPreferences,
} from '../../i18n/format-preferences';
import { PreferencesPage } from './PreferencesPage';

function OtherView() {
  const { formatDate, formatNumber, tPlural } = useLocale();
  return (
    <aside aria-label="Other view">
      <output data-testid="other-number">{formatNumber(4567)}</output>
      <output data-testid="other-date">
        {formatDate(Date.UTC(2026, 7, 16), { year: 'numeric', timeZone: 'UTC' })}
      </output>
      <output data-testid="other-plural">{tPlural('route.about.languageCount', 2)}</output>
      <TechnicalText>resource-1234</TechnicalText>
    </aside>
  );
}

function renderPreferences() {
  return render(
    <LocaleProvider>
      <ThemeProvider>
        <PreferencesPage />
        <OtherView />
      </ThemeProvider>
    </LocaleProvider>,
  );
}

describe('local preferences', () => {
  it('updates central rendering, direction and theme without a session or network write', async () => {
    const user = userEvent.setup();
    const fetch = vi.spyOn(globalThis, 'fetch');
    const view = renderPreferences();
    await user.selectOptions(screen.getByLabelText('Calendar'), 'persian');
    expect(screen.getByTestId('other-date').textContent).toContain('1405');
    await user.selectOptions(screen.getByLabelText('Digits'), 'arabext');
    expect(screen.getByTestId('other-number').textContent).toMatch(/[۰-۹]/u);
    expect(screen.getByTestId('other-plural').textContent).toContain('۲');
    await user.selectOptions(screen.getByLabelText('Language'), 'fa');
    expect(document.documentElement.dir).toBe('rtl');
    await user.selectOptions(screen.getByLabelText('پوسته'), 'dark');
    expect(document.documentElement.dataset.theme).toBe('dark');
    await user.selectOptions(screen.getByLabelText('رقم‌ها'), 'latn');
    expect(screen.getByTestId('other-number').textContent).toMatch(/4.*567/u);
    expect(screen.getByTestId('other-plural').textContent).toContain('2');
    expect(screen.getByText('resource-1234').getAttribute('dir')).toBe('ltr');
    expect(JSON.parse(localStorage.getItem(FORMAT_PREFERENCES_STORAGE_KEY) ?? '{}')).toEqual({
      version: 1,
      calendar: 'persian',
      digits: 'latn',
      hourCycle: 'locale',
    });
    expect(fetch).not.toHaveBeenCalled();
    view.unmount();
    renderPreferences();
    expect(screen.getByLabelText<HTMLSelectElement>('تقویم').value).toBe('persian');
    expect(screen.getByTestId('other-number').textContent).toMatch(/4.*567/u);
  });

  it('shares valid preferences across tabs, resets invalid/cleared storage safely and preserves technical identity', async () => {
    renderPreferences();
    await act(() =>
      window.dispatchEvent(
        new StorageEvent('storage', {
          key: FORMAT_PREFERENCES_STORAGE_KEY,
          newValue: JSON.stringify({ ...defaultFormatPreferences, digits: 'arabext' }),
        }),
      ),
    );
    expect(screen.getByTestId('other-number').textContent).toMatch(/[۰-۹]/u);
    await act(() =>
      window.dispatchEvent(
        new StorageEvent('storage', {
          key: FORMAT_PREFERENCES_STORAGE_KEY,
          newValue: '{"version":999}',
        }),
      ),
    );
    expect(screen.getByTestId('other-number').textContent).toBe('4,567');
    await act(() =>
      window.dispatchEvent(
        new StorageEvent('storage', {
          key: FORMAT_PREFERENCES_STORAGE_KEY,
          newValue: JSON.stringify({ ...defaultFormatPreferences, calendar: 'persian' }),
        }),
      ),
    );
    expect(screen.getByTestId('other-date').textContent).toContain('1405');
    await act(() => window.dispatchEvent(new StorageEvent('storage', { key: null })));
    expect(screen.getByTestId('other-date').textContent).toBe('2026');
    expect(screen.getByText('resource-1234').textContent).toBe('resource-1234');
  });

  it('reports an actual storage failure while keeping the selected display usable', async () => {
    renderPreferences();
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('Storage denied');
    });
    await act(() =>
      fireEvent.change(screen.getByLabelText('Digits'), { target: { value: 'arabext' } }),
    );
    expect(screen.getByTestId('other-number').textContent).toMatch(/[۰-۹]/u);
    expect(screen.getByText(/this browser could not save the preference/u)).not.toBeNull();
  });
});
