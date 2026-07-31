export const DEFAULT_LOCALE_STORAGE_KEY = 'stackplore.locale';

export const supportedLocales = ['en', 'fa'] as const;

export type Locale = (typeof supportedLocales)[number];
export type Direction = 'ltr' | 'rtl';

interface LocaleDefinition {
  direction: Direction;
  intlLocale: string;
}

export const localeDefinitions = {
  en: {
    direction: 'ltr',
    intlLocale: 'en-US',
  },
  fa: {
    direction: 'rtl',
    intlLocale: 'fa-IR-u-ca-persian-nu-arabext',
  },
} as const satisfies Record<Locale, LocaleDefinition>;

export function isLocale(value: unknown): value is Locale {
  return supportedLocales.some((locale) => locale === value);
}
