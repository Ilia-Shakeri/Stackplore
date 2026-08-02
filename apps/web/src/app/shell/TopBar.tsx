import { useState } from 'react';
import { useRouterState } from '@tanstack/react-router';

import { Breadcrumbs } from './Breadcrumbs';
import { LocaleControl } from './LocaleControl';
import { MobileNav } from './MobileNav';
import { ThemeControl } from './ThemeControl';
import { UserMenu } from '../../features/auth/UserMenu';
import { getPageTitle } from './navigation';
import { useLocale } from '../../i18n';

export function TopBar() {
  const [mobileNavigationOpen, setMobileNavigationOpen] = useState(false);
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { t } = useLocale();
  const pageTitle = getPageTitle(pathname, t);

  return (
    <header className="app-topbar">
      <div className="app-topbar__mobile-leading">
        <MobileNav
          open={mobileNavigationOpen}
          pathname={pathname}
          onOpenChange={setMobileNavigationOpen}
        />
        <span className="app-topbar__title">{pageTitle}</span>
      </div>
      <Breadcrumbs />
      <div className="app-topbar__actions">
        <LocaleControl />
        <ThemeControl />
        <UserMenu />
      </div>
    </header>
  );
}
