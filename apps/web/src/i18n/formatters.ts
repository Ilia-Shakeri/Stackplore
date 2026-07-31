import { findMessage, type PluralMessageKey, type TextMessageKey } from './catalog';
import { localeDefinitions, type Locale } from './locale';

export type MessageValues = Readonly<Record<string, number | string>>;

const placeholderPattern = /\{([A-Za-z][A-Za-z0-9_]*)\}/g;

function formatValue(locale: Locale, value: number | string): string {
  return typeof value === 'number' ? formatLocaleNumber(locale, value) : value;
}

function interpolate(locale: Locale, template: string, values: MessageValues): string {
  return template.replace(placeholderPattern, (placeholder, name: string) => {
    const value = values[name];
    return value === undefined ? placeholder : formatValue(locale, value);
  });
}

export function formatLocaleNumber(
  locale: Locale,
  value: number | bigint,
  options?: Intl.NumberFormatOptions,
): string {
  return new Intl.NumberFormat(localeDefinitions[locale].intlLocale, options).format(value);
}

export function formatLocaleDate(
  locale: Locale,
  value: Date | number,
  options?: Intl.DateTimeFormatOptions,
): string {
  return new Intl.DateTimeFormat(localeDefinitions[locale].intlLocale, options).format(value);
}

export function translateText(
  locale: Locale,
  key: TextMessageKey,
  values: MessageValues = {},
): string {
  const message = findMessage(locale, key);
  return typeof message === 'string' ? interpolate(locale, message, values) : `[${key}]`;
}

export function translatePlural(
  locale: Locale,
  key: PluralMessageKey,
  count: number,
  values: MessageValues = {},
): string {
  const message = findMessage(locale, key);
  if (message === undefined || typeof message === 'string') {
    return `[${key}]`;
  }

  const category = new Intl.PluralRules(localeDefinitions[locale].intlLocale).select(count);
  const template = message[category] ?? message.other;
  return interpolate(locale, template, { ...values, count });
}
