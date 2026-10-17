import { describe, expect, it } from 'vitest';

import {
  defaultFormatPreferences,
  parseFormatPreferences,
  type FormatPreferences,
} from './format-preferences';
import { formatLocaleDate, formatLocaleNumber, translatePlural, translateText } from './formatters';

describe('bounded versioned local formatting preferences', () => {
  it.each([
    null,
    '',
    '{',
    'null',
    '[]',
    JSON.stringify({ ...defaultFormatPreferences, version: 2 }),
    JSON.stringify({ ...defaultFormatPreferences, calendar: 'unreviewed' }),
    JSON.stringify({ ...defaultFormatPreferences, digits: 'arbitrary' }),
    JSON.stringify({ ...defaultFormatPreferences, hourCycle: 'h24' }),
    JSON.stringify({ ...defaultFormatPreferences, endpoint: 'unreviewed' }),
    ' '.repeat(257),
  ])('falls back safely from %s', (stored) => {
    expect(parseFormatPreferences(stored)).toEqual(defaultFormatPreferences);
  });

  it('applies digits to numbers, dates, text interpolation and plurals', () => {
    const preferences: FormatPreferences = {
      ...defaultFormatPreferences,
      digits: 'latn',
      calendar: 'gregory',
      hourCycle: 'h23',
    };
    expect(formatLocaleNumber('fa', 1234.5, undefined, preferences)).toMatch(/1.*234/u);
    expect(
      formatLocaleDate(
        'fa',
        Date.UTC(2026, 7, 16, 18, 23),
        { dateStyle: 'long', timeStyle: 'short', timeZone: 'UTC' },
        preferences,
      ),
    ).toContain('2026');
    expect(translateText('fa', 'preferences.preview.count', { count: 12 }, preferences)).toContain(
      '12',
    );
    expect(translatePlural('fa', 'route.about.languageCount', 2, undefined, preferences)).toContain(
      '2',
    );
    expect(translatePlural('fa', 'route.about.languageCount', 2)).toContain('۲');
  });

  it('changes calendar and hour cycle while retaining the original instant', () => {
    const instant = Date.UTC(2026, 7, 16, 18, 23);
    expect(
      formatLocaleDate(
        'en',
        instant,
        { year: 'numeric', timeZone: 'UTC' },
        { ...defaultFormatPreferences, calendar: 'persian' },
      ),
    ).toContain('1405');
    expect(
      formatLocaleDate(
        'en',
        instant,
        { hour: 'numeric', minute: '2-digit', timeZone: 'UTC' },
        { ...defaultFormatPreferences, hourCycle: 'h23' },
      ),
    ).toBe('18:23');
    expect(
      formatLocaleDate(
        'en',
        instant,
        { hour: 'numeric', minute: '2-digit', timeZone: 'UTC' },
        { ...defaultFormatPreferences, hourCycle: 'h12' },
      ),
    ).toContain('6:23');
    expect(new Date(instant).toISOString()).toBe('2026-08-16T18:23:00.000Z');
  });
});
