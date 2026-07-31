import { Select, SelectGroup, SelectOption } from '@stackplore/ui';

import { isLocale, useLocale } from '../../i18n';

export function LocaleControl() {
  const { locale, setLocale, t } = useLocale();

  return (
    <Select
      className="locale-control"
      aria-label={t('shell.locale.label')}
      value={locale}
      onChange={(event) => {
        const value = event.currentTarget.value;
        if (isLocale(value)) {
          setLocale(value);
        }
      }}
    >
      <SelectGroup label={t('shell.locale.label')}>
        <SelectOption value="en" lang="en" dir="ltr">
          {t('shell.locale.option.en')}
        </SelectOption>
        <SelectOption value="fa" lang="fa" dir="rtl">
          {t('shell.locale.option.fa')}
        </SelectOption>
      </SelectGroup>
    </Select>
  );
}
