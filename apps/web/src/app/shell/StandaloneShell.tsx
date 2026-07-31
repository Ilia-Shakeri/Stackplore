import { Outlet } from '@tanstack/react-router';

import { BrandLogo } from './BrandLogo';
import { LocaleControl } from './LocaleControl';
import { ThemeControl } from './ThemeControl';
import { useLocale } from '../../i18n';

export function StandaloneShell() {
  const { t } = useLocale();

  return (
    <div className="standalone-shell min-h-dvh bg-canvas text-ink">
      <a className="skip-link" href="#main-content">
        {t('shell.skipToMain')}
      </a>
      <header className="standalone-shell__header">
        <BrandLogo />
        <div className="standalone-shell__actions">
          <LocaleControl />
          <ThemeControl />
        </div>
      </header>
      <main id="main-content" className="standalone-shell__main" tabIndex={-1}>
        <Outlet />
      </main>
    </div>
  );
}
