import { Button, Select, SelectOption, useTheme } from '@stackplore/ui';
import { useState, type ReactNode } from 'react';

import { defaultFormatPreferences, isLocale, useLocale, type FormatPreferences } from '../../i18n';
import './settings.css';

const previewTime = Date.UTC(2026, 7, 16, 18, 23);

export function PreferencesPage({ toolbar }: { readonly toolbar?: ReactNode }) {
  const {
    locale,
    setLocale,
    formatPreferences,
    setFormatPreferences,
    formatDate,
    formatNumber,
    t,
  } = useLocale();
  const { theme, setTheme } = useTheme();
  const [storageUnavailable, setStorageUnavailable] = useState(false);
  const changeFormats = (next: FormatPreferences) => {
    setStorageUnavailable(!setFormatPreferences(next));
  };

  return (
    <div className="route-page settings-page">
      <header className="route-page__header">
        <p className="route-page__eyebrow">{t('preferences.eyebrow')}</p>
        <h1>{t('route.preferences.title')}</h1>
        <p>{t('preferences.description')}</p>
      </header>
      {toolbar}
      <section className="settings-panel" aria-labelledby="preferences-display-title">
        <h2 id="preferences-display-title">{t('route.preferences.title')}</h2>
        <p>{t('preferences.local')}</p>
        <div className="settings-fields">
          <label className="settings-field">
            <span>{t('preferences.language')}</span>
            <Select
              value={locale}
              onChange={(event) => {
                const next = event.currentTarget.value;
                if (isLocale(next)) setLocale(next);
              }}
            >
              <SelectOption value="en">{t('shell.locale.option.en')}</SelectOption>
              <SelectOption value="fa">{t('shell.locale.option.fa')}</SelectOption>
            </Select>
          </label>
          <label className="settings-field">
            <span>{t('preferences.theme')}</span>
            <Select
              value={theme}
              onChange={(event) => {
                const next = event.currentTarget.value;
                if (next === 'light' || next === 'dark' || next === 'system') setTheme(next);
              }}
            >
              <SelectOption value="system">{t('shell.theme.system')}</SelectOption>
              <SelectOption value="light">{t('shell.theme.light')}</SelectOption>
              <SelectOption value="dark">{t('shell.theme.dark')}</SelectOption>
            </Select>
          </label>
          <label className="settings-field">
            <span>{t('preferences.calendar')}</span>
            <Select
              value={formatPreferences.calendar}
              onChange={(event) => {
                const calendar = event.currentTarget.value;
                if (calendar === 'locale' || calendar === 'gregory' || calendar === 'persian')
                  changeFormats({ ...formatPreferences, calendar });
              }}
            >
              <SelectOption value="locale">{t('preferences.calendar.locale')}</SelectOption>
              <SelectOption value="gregory">{t('preferences.calendar.gregory')}</SelectOption>
              <SelectOption value="persian">{t('preferences.calendar.persian')}</SelectOption>
            </Select>
          </label>
          <label className="settings-field">
            <span>{t('preferences.digits')}</span>
            <Select
              value={formatPreferences.digits}
              onChange={(event) => {
                const digits = event.currentTarget.value;
                if (digits === 'locale' || digits === 'latn' || digits === 'arabext')
                  changeFormats({ ...formatPreferences, digits });
              }}
            >
              <SelectOption value="locale">{t('preferences.digits.locale')}</SelectOption>
              <SelectOption value="latn">{t('preferences.digits.latn')}</SelectOption>
              <SelectOption value="arabext">{t('preferences.digits.arabext')}</SelectOption>
            </Select>
          </label>
          <label className="settings-field">
            <span>{t('preferences.hourCycle')}</span>
            <Select
              value={formatPreferences.hourCycle}
              onChange={(event) => {
                const hourCycle = event.currentTarget.value;
                if (hourCycle === 'locale' || hourCycle === 'h12' || hourCycle === 'h23')
                  changeFormats({ ...formatPreferences, hourCycle });
              }}
            >
              <SelectOption value="locale">{t('preferences.hourCycle.locale')}</SelectOption>
              <SelectOption value="h12">{t('preferences.hourCycle.h12')}</SelectOption>
              <SelectOption value="h23">{t('preferences.hourCycle.h23')}</SelectOption>
            </Select>
          </label>
        </div>
        <Button
          variant="secondary"
          onClick={() => {
            changeFormats(defaultFormatPreferences);
          }}
        >
          {t('preferences.reset')}
        </Button>
        {storageUnavailable ? <p role="status">{t('preferences.storageUnavailable')}</p> : null}
      </section>
      <section className="settings-panel" aria-labelledby="preferences-preview-title">
        <h2 id="preferences-preview-title">{t('preferences.preview')}</h2>
        <dl className="settings-facts" aria-live="polite">
          <div>
            <dt>{t('preferences.preview.date')}</dt>
            <dd>
              <time dateTime="2026-08-16T18:23:00Z">
                {formatDate(previewTime, {
                  dateStyle: 'long',
                  timeStyle: 'short',
                  timeZone: 'UTC',
                })}
              </time>
            </dd>
          </div>
          <div>
            <dt>{t('preferences.preview.number')}</dt>
            <dd>{formatNumber(12345.67)}</dd>
          </div>
        </dl>
        <p>{t('preferences.preview.count', { count: 12 })}</p>
        <p className="settings-note">{t('preferences.preview.zone')}</p>
      </section>
    </div>
  );
}
