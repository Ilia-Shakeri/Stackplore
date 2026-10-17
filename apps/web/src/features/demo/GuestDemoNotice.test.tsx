import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { GuestDemoNotice } from './GuestDemoNotice';
import { AuthProvider } from '../auth/auth-context';
import { AuthStore } from '../auth/auth-store';
import { createAuthenticatedAuthStore, createTestAuthApi } from '../../test/auth';
import { LocaleProvider } from '../../i18n';

describe('guest demonstration notice', () => {
  it.each([
    ['en', 'Demo mode', 'Read only · synthetic data'],
    ['fa', 'حالت نمونه', 'فقط‌خواندنی · دادهٔ ساختگی'],
  ] as const)('identifies guest synthetic data in %s', (locale, name, readOnly) => {
    const store = new AuthStore(createTestAuthApi(), {
      snapshot: {
        status: 'guest_demo',
        fixtureVersion: 'demo-v2',
        scenarioStep: 0,
        observedAt: '2026-08-16T08:00:00Z',
      },
    });
    render(
      <LocaleProvider defaultLocale={locale}>
        <AuthProvider store={store}>
          <GuestDemoNotice />
        </AuthProvider>
      </LocaleProvider>,
    );
    expect(screen.getByRole('complementary', { name })).not.toBeNull();
    expect(screen.getByText(readOnly)).not.toBeNull();
    expect(screen.getByText('demo-v2').getAttribute('dir')).toBe('ltr');
    expect(document.documentElement.dir).toBe(locale === 'fa' ? 'rtl' : 'ltr');
  });
  it('keeps a normal authenticated installation distinct from a guest deployment', () => {
    render(
      <LocaleProvider defaultLocale="en">
        <AuthProvider store={createAuthenticatedAuthStore()}>
          <GuestDemoNotice />
        </AuthProvider>
      </LocaleProvider>,
    );
    expect(screen.queryByRole('complementary', { name: 'Demo mode' })).toBeNull();
  });
});
