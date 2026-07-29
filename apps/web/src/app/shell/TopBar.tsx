import { useState } from 'react';
import { useRouterState } from '@tanstack/react-router';

import { Breadcrumbs } from './Breadcrumbs';
import { MobileNav } from './MobileNav';
import { ThemeControl } from './ThemeControl';
import { getPageTitle } from './navigation';

export function TopBar() {
  const [mobileNavigationOpen, setMobileNavigationOpen] = useState(false);
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const pageTitle = getPageTitle(pathname);

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
        <ThemeControl />
      </div>
    </header>
  );
}
