import { act, render, screen } from '@testing-library/react';
import axe from 'axe-core';
import { describe, expect, it, vi } from 'vitest';

import { LocaleProvider } from '../i18n';
import { PublicShellStatus } from './PublicShellStatus';

describe('public shell connection state', () => {
  it.each(['en', 'fa'] as const)(
    'announces network loss in %s without blocking controls',
    async (locale) => {
      const connection = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
      const { container } = render(
        <LocaleProvider defaultLocale={locale}>
          <PublicShellStatus />
        </LocaleProvider>,
      );
      expect(screen.queryByRole('status')).toBeNull();
      connection.mockReturnValue(false);
      act(() => {
        window.dispatchEvent(new Event('offline'));
      });
      expect(screen.getByRole('status').textContent).toContain(
        locale === 'en' ? 'Network offline' : 'اتصال شبکه قطع است',
      );
      expect(document.documentElement.dir).toBe(locale === 'en' ? 'ltr' : 'rtl');
      const result = await axe.run(container, { rules: { 'color-contrast': { enabled: false } } });
      expect(result.violations).toEqual([]);
      connection.mockReturnValue(true);
      act(() => {
        window.dispatchEvent(new Event('online'));
      });
      expect(screen.queryByRole('status')).toBeNull();
    },
  );
});
