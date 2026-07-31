import { Select, SelectGroup, SelectOption, useTheme } from '@stackplore/ui';

import { useLocale } from '../../i18n';

export function ThemeControl() {
  const { setTheme, theme } = useTheme();
  const { t } = useLocale();

  return (
    <Select
      className="theme-control"
      aria-label={t('shell.theme.label')}
      value={theme}
      onChange={(event) => {
        const value = event.currentTarget.value;
        if (value === 'light' || value === 'dark' || value === 'system') {
          setTheme(value);
        }
      }}
    >
      <SelectGroup label={t('shell.theme.label')}>
        <SelectOption value="light">{t('shell.theme.light')}</SelectOption>
        <SelectOption value="dark">{t('shell.theme.dark')}</SelectOption>
        <SelectOption value="system">{t('shell.theme.system')}</SelectOption>
      </SelectGroup>
    </Select>
  );
}
