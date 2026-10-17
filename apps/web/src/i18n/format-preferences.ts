export const FORMAT_PREFERENCES_STORAGE_KEY = 'stackplore.format-preferences';

export interface FormatPreferences {
  readonly version: 1;
  readonly calendar: 'locale' | 'gregory' | 'persian';
  readonly digits: 'locale' | 'latn' | 'arabext';
  readonly hourCycle: 'locale' | 'h12' | 'h23';
}

export const defaultFormatPreferences: FormatPreferences = Object.freeze({
  version: 1,
  calendar: 'locale',
  digits: 'locale',
  hourCycle: 'locale',
});

export function isFormatPreferences(value: unknown): value is FormatPreferences {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return false;
  const record = value as Record<string, unknown>;
  return (
    Object.keys(record).length === 4 &&
    ['version', 'calendar', 'digits', 'hourCycle'].every((key) => Object.hasOwn(record, key)) &&
    record.version === 1 &&
    (record.calendar === 'locale' ||
      record.calendar === 'gregory' ||
      record.calendar === 'persian') &&
    (record.digits === 'locale' || record.digits === 'latn' || record.digits === 'arabext') &&
    (record.hourCycle === 'locale' || record.hourCycle === 'h12' || record.hourCycle === 'h23')
  );
}

export function parseFormatPreferences(stored: string | null): FormatPreferences {
  if (stored === null || stored.length > 256) return defaultFormatPreferences;
  try {
    const value: unknown = JSON.parse(stored);
    return isFormatPreferences(value) ? value : defaultFormatPreferences;
  } catch {
    return defaultFormatPreferences;
  }
}

export function readFormatPreferences(storageKey: string): FormatPreferences {
  if (typeof window === 'undefined') return defaultFormatPreferences;
  try {
    return parseFormatPreferences(window.localStorage.getItem(storageKey));
  } catch {
    return defaultFormatPreferences;
  }
}

export function numberPreferenceOptions(preferences: FormatPreferences): Intl.NumberFormatOptions {
  return preferences.digits === 'locale' ? {} : { numberingSystem: preferences.digits };
}

export function datePreferenceOptions(preferences: FormatPreferences): Intl.DateTimeFormatOptions {
  return {
    ...(preferences.digits === 'locale' ? {} : { numberingSystem: preferences.digits }),
    ...(preferences.calendar === 'locale' ? {} : { calendar: preferences.calendar }),
    ...(preferences.hourCycle === 'locale' ? {} : { hourCycle: preferences.hourCycle }),
  };
}
