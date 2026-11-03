import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { HelpPage } from './HelpPage';
import { ProductProvider } from '../features/product/product-context';
import { createProductApi } from '../features/product/product-api';
import { LocaleProvider } from '../i18n';

describe('task help', () => {
  it.each([
    ['en', 'Access and privacy'],
    ['fa', 'دسترسی و حریم خصوصی'],
  ] as const)('opens one URL-backed topic in %s without a network request', (locale, heading) => {
    const request = vi.fn().mockRejectedValue(new Error('Help must not fetch data.'));
    render(
      <LocaleProvider defaultLocale={locale}>
        <ProductProvider api={createProductApi({ request })} viewSearch="help_topic=security">
          <HelpPage />
        </ProductProvider>
      </LocaleProvider>,
    );
    expect(screen.getByRole('heading', { level: 2, name: heading })).not.toBeNull();
    expect(screen.getAllByRole('article')).toHaveLength(1);
    expect(request).not.toHaveBeenCalled();
    const support = screen.getByRole('link', {
      name: locale === 'en' ? 'Report an issue on GitHub' : 'گزارش خطا در GitHub',
    });
    expect(support.getAttribute('rel')).toBe('noopener noreferrer');
  });
});
