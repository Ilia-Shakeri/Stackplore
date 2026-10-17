import { DirectionProvider } from '@stackplore/ui';
import {
  createContext,
  useCallback,
  useContext,
  useLayoutEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

import type { PluralMessageKey, TextMessageKey } from './catalog';
import {
  FORMAT_PREFERENCES_STORAGE_KEY,
  defaultFormatPreferences,
  isFormatPreferences,
  parseFormatPreferences,
  readFormatPreferences,
  type FormatPreferences,
} from './format-preferences';
import {
  formatLocaleDate,
  formatLocaleNumber,
  translatePlural,
  translateText,
  type MessageValues,
} from './formatters';
import {
  DEFAULT_LOCALE_STORAGE_KEY,
  isLocale,
  localeDefinitions,
  type Direction,
  type Locale,
} from './locale';

interface LocaleContextValue {
  direction: Direction;
  formatDate: (value: Date | number, options?: Intl.DateTimeFormatOptions) => string;
  formatNumber: (value: number | bigint, options?: Intl.NumberFormatOptions) => string;
  locale: Locale;
  formatPreferences: FormatPreferences;
  setFormatPreferences: (preferences: FormatPreferences) => boolean;
  setLocale: (locale: Locale) => void;
  t: (key: TextMessageKey, values?: MessageValues) => string;
  tPlural: (key: PluralMessageKey, count: number, values?: MessageValues) => string;
}

export interface LocaleProviderProps {
  children: ReactNode;
  defaultLocale?: Locale;
  storageKey?: string;
  formatStorageKey?: string;
}

const LocaleContext = createContext<LocaleContextValue | null>(null);

function applyDocumentLocale(locale: Locale) {
  const root = document.documentElement;
  root.lang = locale;
  root.dir = localeDefinitions[locale].direction;
  root.dataset.locale = locale;
}

function readStoredLocale(storageKey: string, fallback: Locale): Locale {
  if (typeof window === 'undefined') {
    return fallback;
  }

  try {
    const storedLocale = window.localStorage.getItem(storageKey);
    if (isLocale(storedLocale)) {
      return storedLocale;
    }
  } catch {
    return fallback;
  }

  const bootLocale = document.documentElement.dataset.locale;
  return isLocale(bootLocale) ? bootLocale : fallback;
}

export function LocaleProvider({
  children,
  defaultLocale = 'en',
  storageKey = DEFAULT_LOCALE_STORAGE_KEY,
  formatStorageKey = FORMAT_PREFERENCES_STORAGE_KEY,
}: LocaleProviderProps) {
  const [locale, setLocaleState] = useState<Locale>(() =>
    readStoredLocale(storageKey, defaultLocale),
  );
  const direction = localeDefinitions[locale].direction;
  const [formatPreferences, setFormatPreferencesState] = useState(() =>
    readFormatPreferences(formatStorageKey),
  );

  useLayoutEffect(() => {
    const synchronize = (event: StorageEvent) => {
      if (event.key === formatStorageKey || event.key === null) {
        setFormatPreferencesState(
          event.key === null ? defaultFormatPreferences : parseFormatPreferences(event.newValue),
        );
      }
    };
    window.addEventListener('storage', synchronize);
    return () => {
      window.removeEventListener('storage', synchronize);
    };
  }, [formatStorageKey]);

  const setFormatPreferences = useCallback(
    (preferences: FormatPreferences) => {
      if (!isFormatPreferences(preferences)) return false;
      setFormatPreferencesState(preferences);
      try {
        window.localStorage.setItem(formatStorageKey, JSON.stringify(preferences));
        return true;
      } catch {
        return false;
      }
    },
    [formatStorageKey],
  );

  useLayoutEffect(() => {
    applyDocumentLocale(locale);

    let description = document.querySelector<HTMLMetaElement>('meta[name="description"]');
    if (description === null) {
      description = document.createElement('meta');
      description.name = 'description';
      document.head.append(description);
    }
    description.content = translateText(locale, 'app.meta.description');
  }, [locale]);

  useLayoutEffect(() => {
    const syncStoredLocale = (event: StorageEvent) => {
      if (event.key !== storageKey) {
        return;
      }
      setLocaleState(isLocale(event.newValue) ? event.newValue : defaultLocale);
    };

    window.addEventListener('storage', syncStoredLocale);
    return () => {
      window.removeEventListener('storage', syncStoredLocale);
    };
  }, [defaultLocale, storageKey]);

  const setLocale = useCallback(
    (nextLocale: Locale) => {
      applyDocumentLocale(nextLocale);
      setLocaleState(nextLocale);
      try {
        window.localStorage.setItem(storageKey, nextLocale);
      } catch {
        return;
      }
    },
    [storageKey],
  );

  const t = useCallback(
    (key: TextMessageKey, values?: MessageValues) =>
      translateText(locale, key, values, formatPreferences),
    [formatPreferences, locale],
  );
  const tPlural = useCallback(
    (key: PluralMessageKey, count: number, values?: MessageValues) =>
      translatePlural(locale, key, count, values, formatPreferences),
    [formatPreferences, locale],
  );
  const formatNumber = useCallback(
    (value: number | bigint, options?: Intl.NumberFormatOptions) =>
      formatLocaleNumber(locale, value, options, formatPreferences),
    [formatPreferences, locale],
  );
  const formatDate = useCallback(
    (value: Date | number, options?: Intl.DateTimeFormatOptions) =>
      formatLocaleDate(locale, value, options, formatPreferences),
    [formatPreferences, locale],
  );

  const value = useMemo(
    () => ({
      direction,
      formatDate,
      formatNumber,
      locale,
      formatPreferences,
      setFormatPreferences,
      setLocale,
      t,
      tPlural,
    }),
    [
      direction,
      formatDate,
      formatNumber,
      formatPreferences,
      locale,
      setFormatPreferences,
      setLocale,
      t,
      tPlural,
    ],
  );

  return (
    <LocaleContext value={value}>
      <DirectionProvider direction={direction}>{children}</DirectionProvider>
    </LocaleContext>
  );
}

export function useLocale() {
  const value = useContext(LocaleContext);
  if (value === null) {
    throw new Error('useLocale must be used within LocaleProvider.');
  }
  return value;
}
