import { Tabs, TabsContent, TabsList, TabsTrigger, TechnicalText, useToast } from '@stackplore/ui';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { act } from 'react';
import { describe, expect, it, vi } from 'vitest';

import { AppErrorBoundary } from '../app/AppErrorBoundary';
import { AppProviders } from '../app/AppProviders';
import {
  LocaleProvider,
  formatLocaleDate,
  formatLocaleNumber,
  translatePlural,
  useLocale,
} from './index';

function LocaleHarness() {
  const { direction, formatDate, formatNumber, locale, setLocale, tPlural } = useLocale();
  const fixedDate = Date.UTC(2025, 2, 20, 12);

  return (
    <>
      <output data-testid="locale">{locale}</output>
      <output data-testid="direction">{direction}</output>
      <output data-testid="number">{formatNumber(1234.5)}</output>
      <output data-testid="date">
        {formatDate(fixedDate, { dateStyle: 'long', timeZone: 'UTC' })}
      </output>
      <output data-testid="plural">{tPlural('route.about.languageCount', 2)}</output>
      <button
        type="button"
        onClick={() => {
          setLocale(locale === 'en' ? 'fa' : 'en');
        }}
      >
        Switch
      </button>
    </>
  );
}

function ToastHarness() {
  const { notify } = useToast();

  return (
    <button
      type="button"
      onClick={() => {
        notify({ duration: 60_000, title: 'Test notice' });
      }}
    >
      Send
    </button>
  );
}

describe('locale state and formatting', () => {
  it('starts in English and updates document metadata', () => {
    render(
      <LocaleProvider>
        <LocaleHarness />
      </LocaleProvider>,
    );

    expect(screen.getByTestId('locale').textContent).toBe('en');
    expect(screen.getByTestId('direction').textContent).toBe('ltr');
    expect(document.documentElement.lang).toBe('en');
    expect(document.documentElement.dir).toBe('ltr');
    expect(document.documentElement.dataset.locale).toBe('en');
  });

  it('reads Persian, switches live, and persists the new locale', () => {
    globalThis.localStorage.setItem('stackplore.locale', 'fa');

    render(
      <LocaleProvider>
        <LocaleHarness />
      </LocaleProvider>,
    );

    expect(screen.getByTestId('locale').textContent).toBe('fa');
    expect(screen.getByTestId('direction').textContent).toBe('rtl');
    expect(screen.getByTestId('number').textContent).toMatch(/[۰-۹]/u);
    expect(screen.getByTestId('date').textContent).toMatch(/[۰-۹]/u);
    expect(screen.getByTestId('plural').textContent).toContain('۲');
    expect(document.documentElement.lang).toBe('fa');
    expect(document.documentElement.dir).toBe('rtl');

    fireEvent.click(screen.getByRole('button', { name: 'Switch' }));

    expect(screen.getByTestId('locale').textContent).toBe('en');
    expect(globalThis.localStorage.getItem('stackplore.locale')).toBe('en');
    expect(document.documentElement.dir).toBe('ltr');
  });

  it('ignores invalid storage and syncs a valid cross-tab change', () => {
    globalThis.localStorage.setItem('test.locale', 'invalid');

    render(
      <LocaleProvider storageKey="test.locale">
        <LocaleHarness />
      </LocaleProvider>,
    );

    expect(screen.getByTestId('locale').textContent).toBe('en');

    act(() => {
      globalThis.dispatchEvent(
        new StorageEvent('storage', {
          key: 'test.locale',
          newValue: 'fa',
        }),
      );
    });

    expect(screen.getByTestId('locale').textContent).toBe('fa');
    expect(document.documentElement.dir).toBe('rtl');
  });

  it('formats numbers, dates, and plural copy with explicit locale rules', () => {
    const fixedDate = Date.UTC(2025, 2, 20, 12);
    const englishDate = formatLocaleDate('en', fixedDate, {
      dateStyle: 'long',
      timeZone: 'UTC',
    });
    const persianDate = formatLocaleDate('fa', fixedDate, {
      dateStyle: 'long',
      timeZone: 'UTC',
    });

    expect(formatLocaleNumber('en', 1234.5)).toBe('1,234.5');
    expect(formatLocaleNumber('fa', 1234.5)).toMatch(/[۰-۹]/u);
    expect(persianDate).not.toBe(englishDate);
    expect(persianDate).toMatch(/[۰-۹]/u);
    expect(translatePlural('en', 'route.about.languageCount', 1)).toContain('1');
    expect(translatePlural('en', 'route.about.languageCount', 2)).toContain('languages');
    expect(translatePlural('fa', 'route.about.languageCount', 2)).toContain('۲');
  });
});

describe('direction-aware primitives', () => {
  it('moves through horizontal tabs in RTL visual order', async () => {
    const user = userEvent.setup();

    render(
      <LocaleProvider defaultLocale="fa">
        <Tabs defaultValue="first">
          <TabsList aria-label="Views">
            <TabsTrigger value="first">First</TabsTrigger>
            <TabsTrigger value="second">Second</TabsTrigger>
          </TabsList>
          <TabsContent value="first">First view</TabsContent>
          <TabsContent value="second">Second view</TabsContent>
        </Tabs>
      </LocaleProvider>,
    );

    const first = screen.getByRole('tab', { name: 'First' });
    first.focus();
    await user.keyboard('{ArrowLeft}');

    expect(screen.getByRole('tab', { name: 'Second' }).getAttribute('data-state')).toBe('active');
  });

  it('isolates technical text from the surrounding direction', () => {
    render(
      <LocaleProvider defaultLocale="fa">
        <p>
          منبع <TechnicalText>service/edge:1.2</TechnicalText>
        </p>
      </LocaleProvider>,
    );

    const technical = screen.getByText('service/edge:1.2');
    expect(technical.getAttribute('dir')).toBe('ltr');
    expect(technical.hasAttribute('data-ui-technical')).toBe(true);
  });

  it('uses the inline-end swipe direction and Persian live-region label', async () => {
    render(
      <LocaleProvider defaultLocale="fa">
        <AppProviders>
          <ToastHarness />
        </AppProviders>
      </LocaleProvider>,
    );

    expect(
      document.querySelector('.ui-toast__viewport')?.getAttribute('data-swipe-direction'),
    ).toBe('left');

    fireEvent.click(screen.getByRole('button', { name: 'Send' }));

    await waitFor(() => {
      expect(document.querySelector('[aria-live]')?.textContent).toContain('اعلان');
    });
  });

  it('keeps fatal fallback copy in the active language', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);

    function BrokenContent(): never {
      throw new Error('Render failure');
    }

    render(
      <LocaleProvider defaultLocale="fa">
        <AppErrorBoundary>
          <BrokenContent />
        </AppErrorBoundary>
      </LocaleProvider>,
    );

    expect(
      screen.getByRole('heading', { level: 1, name: 'ادامهٔ کار در فضای کاری ممکن نیست' }),
    ).not.toBeNull();
    expect(document.documentElement.dir).toBe('rtl');
  });
});
